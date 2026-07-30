package main

import (
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/mattn/go-isatty"
	"github.com/voluminor/dji-rc-joystick/cli/internal/app"
)

// // // // // // // // // //

func isTerminal(file *os.File) bool {
	fd := file.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

func main() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	signal.Ignore(syscall.SIGPIPE)

	_, noColor := os.LookupEnv("NO_COLOR")
	code := app.Execute(os.Args[1:], app.DependenciesObj{
		Program: filepath.Base(os.Args[0]),
		Stdout:  os.Stdout, Stderr: os.Stderr,
		StdoutTTY: isTerminal(os.Stdout), StderrTTY: isTerminal(os.Stderr),
		NoColor: noColor, Signals: signals,
	})
	signal.Stop(signals)
	os.Exit(code)
}
