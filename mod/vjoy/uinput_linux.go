//go:build linux

package vjoy

import (
	"errors"
	"fmt"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// // // // // // // // // //

const (
	uinputPath = "/dev/uinput"

	evSyn uint16 = 0x00
	evKey uint16 = 0x01
	evRel uint16 = 0x02
	evAbs uint16 = 0x03

	synReport  uint16 = 0x00
	busVirtual uint16 = 0x06

	// ioctl request codes from linux/uinput.h.
	uiSetEvBit   = 0x40045564 // _IOW('U', 100, int)
	uiSetKeyBit  = 0x40045565 // _IOW('U', 101, int)
	uiSetRelBit  = 0x40045566 // _IOW('U', 102, int)
	uiSetAbsBit  = 0x40045567 // _IOW('U', 103, int)
	uiAbsSetup   = 0x401c5504 // _IOW('U', 4, struct uinput_abs_setup)
	uiDevSetup   = 0x405c5503 // _IOW('U', 3, struct uinput_setup)
	uiDevCreate  = 0x00005501 // _IO('U', 1)
	uiDevDestroy = 0x00005502 // _IO('U', 2)

	// btnTrigger doubles as BTN_JOYSTICK: forcing it on keeps the gamepad node
	// classified as a joystick even with no button controls.
	btnTrigger uint16 = 0x120

	relX      uint16 = 0x00 // REL_X
	relY      uint16 = 0x01 // REL_Y
	relWheel  uint16 = 0x08 // REL_WHEEL
	relHWheel uint16 = 0x06 // REL_HWHEEL

	btnLeft uint16 = 0x110 // BTN_LEFT

	deviceSettleDelay = 200 * time.Millisecond

	// Node indexes inside deviceObj.nodes.
	gamepadNode = 0
	pointerNode = 1
)

// //

// Mirrors of the kernel uinput structs; layouts are arch-independent.
type absInfoObj struct {
	Value      int32
	Minimum    int32
	Maximum    int32
	Fuzz       int32
	Flat       int32
	Resolution int32
}

type absSetupObj struct {
	Code    uint16
	_       uint16
	Absinfo absInfoObj
}

type inputIDObj struct {
	Bustype uint16
	Vendor  uint16
	Product uint16
	Version uint16
}

type devSetupObj struct {
	ID           inputIDObj
	Name         [80]byte
	FFEffectsMax uint32
}

// eventObj mirrors struct input_event; unix.Timeval keeps it arch-correct.
type eventObj struct {
	Time  unix.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

type nodePlanObj struct {
	name     string
	absSetup []absSetupObj
	keyCodes []uint16
	relCodes []uint16
}

type nodeObj struct {
	file *os.File
}

type bindingObj struct {
	node   int
	evType uint16
	code   uint16
	min    int32
	max    int32
}

// deviceObj implements Interface over one or two uinput nodes. A dedicated
// "pointer" node carries EV_REL separately, because mixing EV_REL with the
// EV_ABS + gamepad buttons of the main node breaks libinput/udev classification.
type deviceObj struct {
	nodes    []*nodeObj
	bindings map[uint16]bindingObj
}

// //

func ioctlInt(fd uintptr, req uint, value int) error {
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, uintptr(req), uintptr(value)); errno != 0 {
		return errno
	}
	return nil
}

func ioctlPtr(fd uintptr, req uint, ptr unsafe.Pointer) error {
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, uintptr(req), uintptr(ptr)); errno != 0 {
		return errno
	}
	return nil
}

// absCode maps an axis Usage to a Linux ABS_* code. Sliders map to
// ABS_THROTTLE / ABS_RUDDER, which games recognize as slider axes.
func absCode(u Usage) (uint16, bool) {
	switch u {
	case UsageX:
		return 0x00, true // ABS_X
	case UsageY:
		return 0x01, true // ABS_Y
	case UsageZ:
		return 0x02, true // ABS_Z
	case UsageRX:
		return 0x03, true // ABS_RX
	case UsageRY:
		return 0x04, true // ABS_RY
	case UsageRZ:
		return 0x05, true // ABS_RZ
	case UsageSlider0:
		return 0x06, true // ABS_THROTTLE
	case UsageSlider1:
		return 0x07, true // ABS_RUDDER
	}
	return 0, false
}

func relCode(u Usage) (uint16, bool) {
	switch u {
	case UsageWheel:
		return relWheel, true
	case UsageHWheel:
		return relHWheel, true
	}
	return 0, false
}

// Only pure joystick ranges are used. BTN_GAMEPAD makes SDL remap the device,
// while KEY_MAX is excluded from SDL enumeration. Ascending BTN_JOYSTICK and
// BTN_TRIGGER_HAPPY codes keep pinned button numbers stable.
func buttonCode(index int) (uint16, bool) {
	switch {
	case index < 0:
		return 0, false
	case index < 16:
		return uint16(0x120 + index), true // 0x120 is btnTrigger
	case index < 16+63:
		return uint16(0x2C0 + index - 16), true
	}
	return 0, false
}

func (n *nodeObj) emit(evType, code uint16, value int32) error {
	event := eventObj{Type: evType, Code: code, Value: value}
	buf := (*(*[unsafe.Sizeof(eventObj{})]byte)(unsafe.Pointer(&event)))[:]
	if _, err := n.file.Write(buf); err != nil {
		return fmt.Errorf("vjoy: write event: %w", err)
	}
	return nil
}

func (n *nodeObj) close() error {
	if n.file == nil {
		return nil
	}
	_ = ioctlInt(n.file.Fd(), uiDevDestroy, 0)
	err := n.file.Close()
	n.file = nil
	return err
}

func (n *nodeObj) setup(plan *nodePlanObj, spec SpecObj) error {
	fd := n.file.Fd()

	if len(plan.keyCodes) > 0 {
		if err := ioctlInt(fd, uiSetEvBit, int(evKey)); err != nil {
			return fmt.Errorf("vjoy: enable EV_KEY: %w", err)
		}
		for _, code := range plan.keyCodes {
			if err := ioctlInt(fd, uiSetKeyBit, int(code)); err != nil {
				return fmt.Errorf("vjoy: enable key 0x%x: %w", code, err)
			}
		}
	}
	if len(plan.absSetup) > 0 {
		if err := ioctlInt(fd, uiSetEvBit, int(evAbs)); err != nil {
			return fmt.Errorf("vjoy: enable EV_ABS: %w", err)
		}
		for _, s := range plan.absSetup {
			if err := ioctlInt(fd, uiSetAbsBit, int(s.Code)); err != nil {
				return fmt.Errorf("vjoy: enable abs 0x%x: %w", s.Code, err)
			}
		}
	}
	if len(plan.relCodes) > 0 {
		if err := ioctlInt(fd, uiSetEvBit, int(evRel)); err != nil {
			return fmt.Errorf("vjoy: enable EV_REL: %w", err)
		}
		for _, code := range plan.relCodes {
			if err := ioctlInt(fd, uiSetRelBit, int(code)); err != nil {
				return fmt.Errorf("vjoy: enable rel 0x%x: %w", code, err)
			}
		}
	}
	for i := range plan.absSetup {
		if err := ioctlPtr(fd, uiAbsSetup, unsafe.Pointer(&plan.absSetup[i])); err != nil {
			return fmt.Errorf("vjoy: setup abs 0x%x: %w", plan.absSetup[i].Code, err)
		}
	}

	var dev devSetupObj
	dev.ID = inputIDObj{
		Bustype: busVirtual,
		Vendor:  spec.VendorID,
		Product: spec.ProductID,
		Version: 1,
	}
	copy(dev.Name[:len(dev.Name)-1], plan.name)
	if err := ioctlPtr(fd, uiDevSetup, unsafe.Pointer(&dev)); err != nil {
		return fmt.Errorf("vjoy: device setup: %w", err)
	}
	if err := ioctlInt(fd, uiDevCreate, 0); err != nil {
		return fmt.Errorf("vjoy: device create: %w", err)
	}

	// Give userspace (udev, applications) a moment to pick the node up.
	time.Sleep(deviceSettleDelay)
	return nil
}

func createNode(plan *nodePlanObj, spec SpecObj) (*nodeObj, error) {
	file, err := os.OpenFile(uinputPath, os.O_WRONLY|unix.O_NONBLOCK, 0)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return nil, fmt.Errorf(
				"vjoy: open %s: %w (grant access with a udev rule, e.g. "+
					`KERNEL=="uinput", GROUP="input", MODE="0660", or run as root)`,
				uinputPath, err)
		}
		return nil, fmt.Errorf("vjoy: open %s: %w", uinputPath, err)
	}
	node := &nodeObj{file: file}
	if err := node.setup(plan, spec); err != nil {
		_ = node.close()
		return nil, err
	}
	return node, nil
}

// //

func planDevice(spec SpecObj) (dev *deviceObj, gamepad, pointer *nodePlanObj, hasRel bool, err error) {
	for i := range spec.Controls {
		if spec.Controls[i].Kind == KindRelative {
			hasRel = true
			break
		}
	}

	gamepad = &nodePlanObj{name: spec.Name}
	pointer = &nodePlanObj{name: spec.Name + " Pointer"}

	dev = &deviceObj{
		bindings: make(map[uint16]bindingObj, len(spec.Controls)),
	}

	buttonNums, buttonCount, err := assignButtons(spec.Controls)
	if err != nil {
		return nil, nil, nil, false, err
	}
	if buttonCount == 0 {
		// Force BTN_TRIGGER on so the node registers as a joystick even with
		// zero buttons; with buttons it is advertised by the loop below.
		gamepad.keyCodes = append(gamepad.keyCodes, btnTrigger)
	}
	// Advertise every button code up to the highest assigned number, not just
	// the bound ones: consumers number buttons by compacting the advertised
	// codes, so a gap between pinned numbers would collapse and shift every
	// button after it. Unbound slots exist but never fire — the same contract
	// as a vJoy device sized to the highest pin on Windows.
	for i := 0; i < buttonCount; i++ {
		code, ok := buttonCode(i)
		if !ok {
			return nil, nil, nil, false, fmt.Errorf("vjoy: %d buttons exceed the %d-code joystick pool", buttonCount, maxButtons)
		}
		gamepad.keyCodes = append(gamepad.keyCodes, code)
	}

	autoAxis := 0
	for _, c := range spec.Controls {
		switch c.Kind {
		case KindButton:
			code, _ := buttonCode(buttonNums[c.Handle] - 1) // advertised above
			dev.bindings[c.Handle] = bindingObj{node: gamepadNode, evType: evKey, code: code}

		case KindAxis:
			usage := c.Usage
			if usage == UsageAuto {
				if autoAxis >= len(axisUsageOrder) {
					return nil, nil, nil, false, fmt.Errorf("vjoy: control %d has no free auto axis", c.Handle)
				}
				usage = axisUsageOrder[autoAxis]
				autoAxis++
			}
			code, ok := absCode(usage)
			if !ok {
				return nil, nil, nil, false, fmt.Errorf("vjoy: control %d usage %d is not an axis", c.Handle, c.Usage)
			}
			gamepad.absSetup = append(gamepad.absSetup, absSetupObj{
				Code:    code,
				Absinfo: absInfoObj{Minimum: c.Min, Maximum: c.Max},
			})
			dev.bindings[c.Handle] = bindingObj{node: gamepadNode, evType: evAbs, code: code, min: c.Min, max: c.Max}

		case KindRelative:
			code, ok := relCode(c.Usage)
			if !ok {
				return nil, nil, nil, false, fmt.Errorf("vjoy: control %d usage %d is not a wheel", c.Handle, c.Usage)
			}
			pointer.relCodes = append(pointer.relCodes, code)
			dev.bindings[c.Handle] = bindingObj{node: pointerNode, evType: evRel, code: code}

		default:
			return nil, nil, nil, false, fmt.Errorf("vjoy: control %d has unknown kind %d", c.Handle, c.Kind)
		}
	}

	if hasRel {
		// udev's input_id only tags a relative device ID_INPUT_MOUSE — the cue
		// libinput/Wayland/X use to forward its scroll — when it also advertises
		// REL_X/REL_Y and a mouse button. These codes are never emitted (no
		// binding targets them); their mere presence classifies the node.
		pointer.relCodes = append(pointer.relCodes, relX, relY)
		pointer.keyCodes = append(pointer.keyCodes, btnLeft)
	}
	return dev, gamepad, pointer, hasRel, nil
}

func newDevice(spec SpecObj) (Interface, error) {
	dev, gamepad, pointer, hasRel, err := planDevice(spec)
	if err != nil {
		return nil, err
	}

	gp, err := createNode(gamepad, spec)
	if err != nil {
		return nil, err
	}
	dev.nodes = append(dev.nodes, gp)
	if hasRel {
		pt, err := createNode(pointer, spec)
		if err != nil {
			_ = gp.close()
			return nil, err
		}
		dev.nodes = append(dev.nodes, pt)
	}
	return dev, nil
}

func (d *deviceObj) Set(handle uint16, value int32) error {
	b, ok := d.bindings[handle]
	if !ok {
		return fmt.Errorf("vjoy: unknown control handle %d", handle)
	}
	node := d.nodes[b.node]
	switch b.evType {
	case evAbs:
		if value < b.min {
			value = b.min
		}
		if value > b.max {
			value = b.max
		}
		return node.emit(evAbs, b.code, value)
	case evRel:
		return node.emit(evRel, b.code, value)
	default: // evKey
		pressed := int32(0)
		if value != 0 {
			pressed = 1
		}
		return node.emit(evKey, b.code, pressed)
	}
}

func (d *deviceObj) Sync() error {
	for _, n := range d.nodes {
		if err := n.emit(evSyn, synReport, 0); err != nil {
			return err
		}
	}
	return nil
}

func (d *deviceObj) Close() error {
	var firstErr error
	for _, n := range d.nodes {
		if err := n.close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
