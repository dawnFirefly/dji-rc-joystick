package vjoy

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// // // // // // // // // //

// Action names are stable values shared by configuration and producers.
const (
	// ActionUnmapped disables an input binding.
	ActionUnmapped = "unmapped"

	// ActionButton assigns the next free gamepad button.
	ActionButton = "button"
	// ActionWheelUp emits one upward wheel tick.
	ActionWheelUp = "wheel_up"
	// ActionWheelDown emits one downward wheel tick.
	ActionWheelDown = "wheel_down"
	// ActionHWheelLeft emits one leftward wheel tick.
	ActionHWheelLeft = "hwheel_left"
	// ActionHWheelRight emits one rightward wheel tick.
	ActionHWheelRight = "hwheel_right"

	// ActionAxisX drives the X axis.
	ActionAxisX = "axis_x"
	// ActionAxisY drives the Y axis.
	ActionAxisY = "axis_y"
	// ActionAxisZ drives the Z axis.
	ActionAxisZ = "axis_z"
	// ActionAxisRX drives the rotational X axis.
	ActionAxisRX = "axis_rx"
	// ActionAxisRY drives the rotational Y axis.
	ActionAxisRY = "axis_ry"
	// ActionAxisRZ drives the rotational Z axis.
	ActionAxisRZ = "axis_rz"
	// ActionSlider0 drives the first slider axis.
	ActionSlider0 = "axis_slider0"
	// ActionSlider1 drives the second slider axis.
	ActionSlider1 = "axis_slider1"
	// ActionWheel converts analog deflection to vertical scrolling.
	ActionWheel = "wheel"
	// ActionHWheel converts analog deflection to horizontal scrolling.
	ActionHWheel = "hwheel"
)

// //

// ActionObj describes how an output action is realized on the device: the
// control Kind and Usage to declare, and the value emitted when a discrete
// source is active (Press) or the scroll direction for a continuous wheel.
type ActionObj struct {
	// Kind is the target control category.
	Kind ControlKind
	// Usage identifies the target axis, wheel, or button.
	Usage Usage
	// Press is the active discrete value or wheel direction.
	Press int32
	// Continuous reports whether the action requires an analog source.
	Continuous bool
}

var actionTable = map[string]ActionObj{
	ActionUnmapped: {},

	ActionButton:      {Kind: KindButton, Press: 1},
	ActionWheelUp:     {Kind: KindRelative, Usage: UsageWheel, Press: 1},
	ActionWheelDown:   {Kind: KindRelative, Usage: UsageWheel, Press: -1},
	ActionHWheelLeft:  {Kind: KindRelative, Usage: UsageHWheel, Press: -1},
	ActionHWheelRight: {Kind: KindRelative, Usage: UsageHWheel, Press: 1},

	ActionAxisX:   {Kind: KindAxis, Usage: UsageX, Continuous: true},
	ActionAxisY:   {Kind: KindAxis, Usage: UsageY, Continuous: true},
	ActionAxisZ:   {Kind: KindAxis, Usage: UsageZ, Continuous: true},
	ActionAxisRX:  {Kind: KindAxis, Usage: UsageRX, Continuous: true},
	ActionAxisRY:  {Kind: KindAxis, Usage: UsageRY, Continuous: true},
	ActionAxisRZ:  {Kind: KindAxis, Usage: UsageRZ, Continuous: true},
	ActionSlider0: {Kind: KindAxis, Usage: UsageSlider0, Continuous: true},
	ActionSlider1: {Kind: KindAxis, Usage: UsageSlider1, Continuous: true},
	ActionWheel:   {Kind: KindRelative, Usage: UsageWheel, Continuous: true},
	ActionHWheel:  {Kind: KindRelative, Usage: UsageHWheel, Continuous: true},
}

// //

func buttonNumber(name string) (int, bool) {
	digits, found := strings.CutPrefix(name, ActionButton)
	if !found || digits == "" || strings.Trim(digits, "0123456789") != "" {
		return 0, false
	}
	n, err := strconv.Atoi(digits)
	if err != nil || n < 1 || n > maxButtons {
		return 0, false
	}
	return n, true
}

// ActionButtonN formats a pinned-button action. Action accepts values from
// ActionButtonN(1) through ActionButtonN(79).
func ActionButtonN(n int) string {
	return fmt.Sprintf("%s%d", ActionButton, n)
}

// Action returns the realization of a named action, and whether it is known.
// ActionUnmapped is known and returns the zero ActionObj. Besides the fixed
// names, the dynamic "button1".."button79" forms pin a specific gamepad
// button (carried in Usage; 0 means auto-assign).
func Action(name string) (ActionObj, bool) {
	if a, ok := actionTable[name]; ok {
		return a, ok
	}
	if n, ok := buttonNumber(name); ok {
		return ActionObj{Kind: KindButton, Usage: Usage(n), Press: 1}, true
	}
	return ActionObj{}, false
}

// ActionNames returns every fixed action name, sorted. The dynamic
// "button<N>" forms are accepted by Action but not enumerated here.
func ActionNames() []string {
	names := make([]string, 0, len(actionTable))
	for name := range actionTable {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
