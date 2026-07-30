package rc

import (
	"testing"
	"time"

	"github.com/voluminor/dji-rc-joystick/mod/vjoy"
)

// // // // // // // // // //

type captureObj struct {
	values map[uint16]int32
	syncs  int
}

type blockingSinkObj struct {
	entered chan struct{}
	release chan struct{}
}

func newCapture() *captureObj { return &captureObj{values: map[uint16]int32{}} }

func (c *captureObj) Set(handle uint16, value int32) error { c.values[handle] = value; return nil }
func (c *captureObj) Sync() error                          { c.syncs++; return nil }
func (c *captureObj) Close() error                         { return nil }

func (obj *blockingSinkObj) Set(uint16, int32) error {
	select {
	case obj.entered <- struct{}{}:
	default:
	}
	<-obj.release
	return nil
}

func (obj *blockingSinkObj) Sync() error  { return nil }
func (obj *blockingSinkObj) Close() error { return nil }

// //

func TestDefaultMappingValid(t *testing.T) {
	if err := DefaultMapping().Validate(); err != nil {
		t.Fatalf("default mapping invalid: %v", err)
	}
	if err := DefaultMapping().Feed(nil, StateObj{}); err == nil {
		t.Fatal("Feed() accepted a nil virtual input sink")
	}
}

func TestMappingValidateRejects(t *testing.T) {
	// Unknown action.
	m := DefaultMapping()
	m.Buttons[ButtonRTH] = "nope"
	if m.Validate() == nil {
		t.Fatal("unknown action accepted")
	}

	// Discrete action on an analog axis.
	m = DefaultMapping()
	m.Axes[AxisLeftH] = vjoy.ActionButton
	if m.Validate() == nil {
		t.Fatal("discrete action on an axis accepted")
	}

	// Continuous action on a button.
	m = DefaultMapping()
	m.Buttons[ButtonRTH] = vjoy.ActionAxisX
	if m.Validate() == nil {
		t.Fatal("continuous action on a button accepted")
	}

	// Two analog sources on the same virtual axis.
	m = DefaultMapping()
	m.Axes[AxisRightH] = m.Axes[AxisLeftH]
	if m.Validate() == nil {
		t.Fatal("two axes on one virtual axis accepted")
	}

	// Two buttons pinned to the same gamepad button.
	m = DefaultMapping()
	m.Buttons[ButtonRTH] = vjoy.ActionButtonN(3)
	m.Buttons[ButtonC1] = vjoy.ActionButtonN(3)
	if m.Validate() == nil {
		t.Fatal("duplicate pinned button accepted")
	}

	// A pin beyond the shared cross-platform cap is an unknown action.
	m = DefaultMapping()
	m.Buttons[ButtonRTH] = vjoy.ActionButtonN(999)
	if m.Validate() == nil {
		t.Fatal("pin beyond the shared button cap accepted")
	}

	for i, invalid := range []MappingObj{
		{Axes: map[AxisID]string{AxisCount: vjoy.ActionAxisX}},
		{Invert: map[AxisID]bool{-1: true}},
		{Buttons: map[ButtonID]string{ButtonCount: vjoy.ActionButton}},
	} {
		if invalid.Validate() == nil {
			t.Errorf("mapping with unknown control %d accepted", i)
		}
	}
}

func TestMappingDeviceSpecPinsButtons(t *testing.T) {
	m := DefaultMapping()
	m.Buttons[ButtonC2] = vjoy.ActionButtonN(9)
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, c := range m.DeviceSpec(DefaultProfile()).Controls {
		if c.Handle == buttonHandle(ButtonC2) {
			if c.Kind != vjoy.KindButton || c.Usage != 9 {
				t.Fatalf("C2 control = %+v, want pinned button 9", c)
			}
			return
		}
	}
	t.Fatal("C2 control missing from the device spec")
}

func TestFeedSinkInvert(t *testing.T) {
	m := DefaultMapping()
	m.Axes[AxisLeftWheel] = vjoy.ActionWheel
	m.Invert = map[AxisID]bool{AxisLeftH: true, AxisLeftWheel: true}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	dev := newCapture()

	var state StateObj
	state.Axes[AxisLeftH] = 12345
	state.Axes[AxisLeftWheel] = outMax
	if err := m.Feed(dev, state); err != nil {
		t.Fatal(err)
	}
	if dev.values[axisHandle(AxisLeftH)] != -12345 {
		t.Fatalf("inverted left-h = %d, want -12345", dev.values[axisHandle(AxisLeftH)])
	}
	if want := int32(-outMax) / wheelAxisScale; dev.values[axisHandle(AxisLeftWheel)] != want {
		t.Fatalf("inverted scroll delta = %d, want %d", dev.values[axisHandle(AxisLeftWheel)], want)
	}
}

func TestAttachSinkWaitsForInFlightFeed(t *testing.T) {
	old := &blockingSinkObj{
		entered: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	obj, err := New(ConfigObj{IP: "127.0.0.1", Sink: old})
	if err != nil {
		t.Fatal(err)
	}

	feedDone := make(chan struct{})
	go func() {
		_ = obj.feed(StateObj{})
		close(feedDone)
	}()
	<-old.entered

	attached := make(chan struct{})
	go func() {
		obj.AttachSink(nil)
		close(attached)
	}()

	select {
	case <-attached:
		t.Fatal("AttachSink() returned while the old sink was in use")
	case <-time.After(20 * time.Millisecond):
	}

	close(old.release)
	<-feedDone
	select {
	case <-attached:
	case <-time.After(time.Second):
		t.Fatal("AttachSink() did not finish after the feed completed")
	}
}

func TestMappingDeviceSpecSkipsUnmapped(t *testing.T) {
	m := DefaultMapping()
	m.Buttons[ButtonRTH] = vjoy.ActionUnmapped
	axes, buttons := countControls(m.DeviceSpec(DefaultProfile()))
	if axes != len(universalAxes)+1 {
		t.Fatalf("axes = %d, want %d", axes, len(universalAxes)+1)
	}
	if buttons != len(universalButtons)-1 {
		t.Fatalf("buttons = %d, want %d (RTH unmapped)", buttons, len(universalButtons)-1)
	}
}

func TestFeedSinkAppliesMapping(t *testing.T) {
	m := DefaultMapping()
	dev := newCapture()

	var state StateObj
	state.Axes[AxisLeftH] = 12345
	state.Mode = ModeNormal
	state.Buttons = 1 << uint(ButtonRTH)

	if err := feedSink(dev, &m, state); err != nil {
		t.Fatal(err)
	}
	if dev.values[axisHandle(AxisLeftH)] != 12345 {
		t.Fatalf("left-h axis = %d, want 12345", dev.values[axisHandle(AxisLeftH)])
	}
	if dev.values[handleMode] != int32(ModeNormal) {
		t.Fatalf("mode = %d, want %d", dev.values[handleMode], ModeNormal)
	}
	if dev.values[buttonHandle(ButtonRTH)] != 1 {
		t.Fatalf("RTH button = %d, want 1", dev.values[buttonHandle(ButtonRTH)])
	}
	if dev.syncs != 1 {
		t.Fatalf("syncs = %d, want 1", dev.syncs)
	}
}

func TestFeedSinkWheelToScroll(t *testing.T) {
	// Bind the left wheel to a mouse scroll: a deflected wheel yields a delta.
	m := DefaultMapping()
	m.Axes[AxisLeftWheel] = vjoy.ActionWheel
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	dev := newCapture()

	var state StateObj
	state.Axes[AxisLeftWheel] = outMax // full deflection
	if err := feedSink(dev, &m, state); err != nil {
		t.Fatal(err)
	}
	want := int32(outMax) / wheelAxisScale
	if dev.values[axisHandle(AxisLeftWheel)] != want {
		t.Fatalf("scroll delta = %d, want %d", dev.values[axisHandle(AxisLeftWheel)], want)
	}
}
