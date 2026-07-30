package app

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unicode"

	"github.com/voluminor/dji-rc-joystick/cli/internal/output"
	"github.com/voluminor/dji-rc-joystick/mod/rc"
	"github.com/voluminor/dji-rc-joystick/mod/vjoy"
	"github.com/voluminor/dji-rc-joystick/target"
)

// // // // // // // // // //

func normalizeProgramName(value string) string {
	name := filepath.Base(value)
	if name == "" || name == "." || name == string(filepath.Separator) {
		return target.Name
	}
	runes := []rune(name)
	if len(runes) > 64 {
		runes = runes[:64]
	}
	for index, value := range runes {
		if !unicode.IsPrint(value) {
			runes[index] = '?'
		}
	}
	return string(runes)
}

func normalizeDependencies(dependencies DependenciesObj) DependenciesObj {
	dependencies.Program = normalizeProgramName(dependencies.Program)
	if dependencies.Stdout == nil {
		dependencies.Stdout = nopWriteCloserObj{Writer: io.Discard}
	}
	if dependencies.Stderr == nil {
		dependencies.Stderr = io.Discard
	}
	if dependencies.Now == nil {
		dependencies.Now = time.Now
	}
	if dependencies.Discover == nil {
		dependencies.Discover = rc.Discover
	}
	if dependencies.CreateDevice == nil {
		dependencies.CreateDevice = vjoy.New
	}
	return dependencies
}

type nopWriteCloserObj struct {
	io.Writer
}

func (obj nopWriteCloserObj) Close() error {
	return nil
}

func styleFor(modeName string, terminal, noColor bool) (output.StyleObj, error) {
	mode, err := output.ParseColorMode(modeName)
	if err != nil {
		return output.StyleObj{}, err
	}
	enabled := false
	switch mode {
	case output.ColorAlways:
		enabled = true
	case output.ColorAuto:
		enabled = terminal && !noColor
	}
	return output.NewStyle(enabled), nil
}

func defaultErrorLogger(dependencies DependenciesObj) *output.LoggerObj {
	style, _ := styleFor(string(output.ColorAuto), dependencies.StderrTTY, dependencies.NoColor)
	return output.NewLogger(
		dependencies.Stderr, dependencies.Now, style, output.LevelError, false,
	)
}

func dispatch(parsed parsedObj, dependencies DependenciesObj) resultObj {
	switch parsed.command {
	case "help":
		if err := writeHelp(dependencies.Stdout, parsed.help, dependencies); err != nil {
			return resultObj{code: exitRuntime, err: err}
		}
		return resultObj{code: exitSuccess}
	case "info":
		return executeInfo(parsed.info, dependencies)
	case "version":
		return executeVersion(parsed.version, dependencies)
	case "discover":
		return executeDiscover(parsed.discover, dependencies)
	case "run":
		return executeRun(parsed.run, dependencies)
	case "monitor":
		return executeMonitor(parsed.monitor, dependencies)
	case "events":
		return executeEvents(parsed.events, dependencies)
	case "inspect":
		return executeInspect(parsed.inspect, dependencies)
	default:
		return resultObj{code: exitUsage, err: errors.New("internal command dispatch failure")}
	}
}

// Execute parses one command, renders any final error, and returns its exit code.
func Execute(args []string, dependencies DependenciesObj) int {
	dependencies = normalizeDependencies(dependencies)
	parsed, err := parse(args)
	if err != nil {
		logger := defaultErrorLogger(dependencies)
		_ = logger.FinalError(err.Error())
		return exitUsage
	}

	result := dispatch(parsed, dependencies)
	if result.err == nil {
		return result.code
	}
	if output.IsBrokenPipe(result.err) {
		return exitSuccess
	}
	logger := result.logger
	if logger == nil {
		logger = defaultErrorLogger(dependencies)
	}
	if logErr := logger.FinalError(result.err.Error()); logErr != nil {
		return exitRuntime
	}
	return result.code
}

func signalExitCode(signal os.Signal) int {
	if signal == nil {
		return exitSuccess
	}
	if signal == syscall.SIGTERM {
		return exitSIGTERM
	}
	return exitSIGINT
}

func contextWithTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout > 0 {
		return context.WithTimeout(parent, timeout)
	}
	return context.WithCancel(parent)
}
