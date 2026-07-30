//go:build !cgo && !ci

package app

import "errors"

// // // // // // // // // //

// Run reports that the desktop runtime is unavailable without CGO.
func Run(_ []string) error {
	return errors.New("GUI requires CGO; rebuild with CGO_ENABLED=1")
}
