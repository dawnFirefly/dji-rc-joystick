//go:build darwin

package vjoy

import "errors"

// // // // // // // // // //

// macOS virtual HID creation requires a signed DriverKit extension with
// Apple-granted entitlements and user approval; no regular user-space API or
// supported legacy-kext shortcut can provide this backend.

func newDevice(_ SpecObj) (Interface, error) {
	return nil, errors.New(
		"vjoy: virtual joystick is not supported on macOS " +
			"(it requires a DriverKit HID system extension with Apple-granted entitlements)")
}
