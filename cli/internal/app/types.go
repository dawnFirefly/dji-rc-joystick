package app

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/voluminor/dji-rc-joystick/cli/internal/output"
	"github.com/voluminor/dji-rc-joystick/mod/rc"
	"github.com/voluminor/dji-rc-joystick/mod/vjoy"
)

// // // // // // // // // //

const (
	exitSuccess = 0
	exitRuntime = 1
	exitUsage   = 2
	exitSIGINT  = 130
	exitSIGTERM = 143
)

// DependenciesObj supplies process resources and test seams to Execute.
type DependenciesObj struct {
	// Program is the basename used to invoke the executable.
	Program string
	// Stdout receives command results and machine-readable event data.
	Stdout io.WriteCloser
	// Stderr receives progress, logs, and final errors.
	Stderr io.Writer
	// StdoutTTY reports whether stdout is an interactive terminal.
	StdoutTTY bool
	// StderrTTY reports whether stderr is an interactive terminal.
	StderrTTY bool
	// NoColor reports whether NO_COLOR is present in the environment.
	NoColor bool
	// Now supplies wall-clock timestamps.
	Now func() time.Time
	// Signals delivers process signals to long-running commands.
	Signals <-chan os.Signal
	// Discover performs one bounded remote scan.
	Discover func(context.Context, *rc.DiscoverConfigObj) ([]rc.RemoteObj, error)
	// CreateDevice creates the platform virtual joystick.
	CreateDevice func(vjoy.SpecObj) (vjoy.Interface, error)
}

type resultObj struct {
	code   int
	err    error
	logger *output.LoggerObj
}
