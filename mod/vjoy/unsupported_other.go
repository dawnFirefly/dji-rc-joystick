//go:build !linux && !darwin && (!windows || 386)

package vjoy

import "errors"

// // // // // // // // // //

func newDevice(_ SpecObj) (Interface, error) {
	return nil, errors.New("vjoy: virtual joystick is not supported on this platform")
}
