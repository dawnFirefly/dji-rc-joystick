package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/voluminor/dji-rc-joystick/gui/internal/app"
	"github.com/voluminor/dji-rc-joystick/gui/internal/gamepad"
)

// // // // // // // // // //

func main() {
	handled, err := gamepad.RunHelper(
		os.Args[1:],
		os.Stdin,
		os.Stdout,
		os.Stderr,
	)
	if handled {
		if err != nil {
			os.Exit(1)
		}
		return
	}

	if err = app.Run(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}

		_, _ = fmt.Fprintf(os.Stderr, "Application failed: %q\n", err)
		os.Exit(1)
	}
}
