package rc

import (
	"errors"

	"github.com/voluminor/dji-rc-joystick/mod/vjoy"
)

// // // // // // // // // //

func analogValue(a vjoy.ActionObj, value int32) int32 {
	if a.Kind == vjoy.KindRelative {
		return value / wheelAxisScale
	}
	return value
}

func feedSink(sink vjoy.Interface, mapping *MappingObj, state StateObj) error {
	var firstErr error
	keep := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	for _, axisMap := range universalAxes {
		if a, active, _ := action(mapping.Axes[axisMap.Axis]); active {
			value := int32(state.Axes[axisMap.Axis])
			if mapping.Invert[axisMap.Axis] {
				value = -value
			}
			keep(sink.Set(axisHandle(axisMap.Axis), analogValue(a, value)))
		}
	}
	if a, active, _ := action(mapping.Mode); active && state.Mode != ModeUnknown {
		keep(sink.Set(handleMode, analogValue(a, int32(state.Mode))))
	}
	for _, buttonMap := range universalButtons {
		if a, active, _ := action(mapping.Buttons[buttonMap.Button]); active {
			value := int32(0)
			if state.Pressed(buttonMap.Button) {
				value = a.Press
			}
			keep(sink.Set(buttonHandle(buttonMap.Button), value))
		}
	}
	keep(sink.Sync())
	return firstErr
}

// Feed writes one decoded state through this mapping. The mapping must already
// be validated and remain immutable until the call returns.
func (m MappingObj) Feed(sink vjoy.Interface, state StateObj) error {
	if sink == nil {
		return errors.New("rc: virtual input sink is nil")
	}
	return feedSink(sink, &m, state)
}
