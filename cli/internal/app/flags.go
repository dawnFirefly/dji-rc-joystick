package app

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/voluminor/dji-rc-joystick/cli/internal/output"
)

// // // // // // // // // //

func helpResult(command string, color trackedStringObj) parsedObj {
	if color.value == "" {
		color.value = string(output.ColorAuto)
	}
	return parsedObj{
		command: "help",
		help:    helpOptionsObj{command: command, color: color},
	}
}

func parseRun(args []string) (parsedObj, error) {
	flags := newFlagSet("run")
	options := runOptionsObj{}
	addRuntimeFlags(flags, &options.runtime)
	flags.StringVar(&options.config, "config", "", "JSON or YAML file whose mapping section overrides the built-in mapping")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return helpResult("run", options.runtime.color), nil
		}
		return parsedObj{}, err
	}
	if err := requireNoArgs(flags); err != nil {
		return parsedObj{}, err
	}
	if err := validateRuntime(&options.runtime); err != nil {
		return parsedObj{}, err
	}
	options.config = strings.TrimSpace(options.config)
	return parsedObj{command: "run", run: options}, nil
}

func parseMonitor(args []string) (parsedObj, error) {
	flags := newFlagSet("monitor")
	options := runtimeOptionsObj{}
	addRuntimeFlags(flags, &options)
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return helpResult("monitor", options.color), nil
		}
		return parsedObj{}, err
	}
	if err := requireNoArgs(flags); err != nil {
		return parsedObj{}, err
	}
	if err := validateRuntime(&options); err != nil {
		return parsedObj{}, err
	}
	return parsedObj{command: "monitor", monitor: options}, nil
}

func parseEvents(args []string) (parsedObj, error) {
	flags := newFlagSet("events")
	options := eventOptionsObj{}
	addRuntimeFlags(flags, &options.runtime)
	flags.Var(&options.events, "event", "event type to include: state|status|info (repeatable)")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return helpResult("events", options.runtime.color), nil
		}
		return parsedObj{}, err
	}
	if err := requireNoArgs(flags); err != nil {
		return parsedObj{}, err
	}
	if err := validateRuntime(&options.runtime); err != nil {
		return parsedObj{}, err
	}
	for _, event := range options.events.values {
		switch event {
		case "state", "status", "info":
		default:
			return parsedObj{}, fmt.Errorf("unknown event type %q (allowed: state|status|info)", event)
		}
	}
	return parsedObj{command: "events", events: options}, nil
}

func parseInspect(args []string) (parsedObj, error) {
	flags := newFlagSet("inspect")
	options := inspectOptionsObj{format: formatText}
	addRuntimeFlags(flags, &options.runtime)
	flags.StringVar(&options.format, "format", formatText, "output format: text|ndjson")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return helpResult("inspect", options.runtime.color), nil
		}
		return parsedObj{}, err
	}
	if err := requireNoArgs(flags); err != nil {
		return parsedObj{}, err
	}
	if err := validateRuntime(&options.runtime); err != nil {
		return parsedObj{}, err
	}
	options.format = strings.ToLower(strings.TrimSpace(options.format))
	if options.format != formatText && options.format != formatNDJSON {
		return parsedObj{}, fmt.Errorf("unknown inspect format %q (allowed: text|ndjson)", options.format)
	}
	return parsedObj{command: "inspect", inspect: options}, nil
}

func parseDiscover(args []string) (parsedObj, error) {
	flags := newFlagSet("discover")
	options := discoverOptionsObj{format: formatText}
	options.color.value = string(output.ColorAuto)
	flags.IntVar(&options.port, "port", 0, "remote discovery TCP port")
	flags.DurationVar(&options.timeout, "timeout", 0, "maximum total scan duration")
	flags.StringVar(&options.format, "format", formatText, "output format: text|json")
	flags.Var(&options.color, "color", "color mode: auto|always|never")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return helpResult("discover", options.color), nil
		}
		return parsedObj{}, err
	}
	if err := requireNoArgs(flags); err != nil {
		return parsedObj{}, err
	}
	if options.port < 0 || options.port > 65535 {
		return parsedObj{}, errors.New("--port must be zero or between 1 and 65535")
	}
	if options.timeout < 0 {
		return parsedObj{}, errors.New("--timeout cannot be negative")
	}
	options.format = strings.ToLower(strings.TrimSpace(options.format))
	if options.format != formatText && options.format != formatJSON {
		return parsedObj{}, fmt.Errorf("unknown discover format %q (allowed: text|json)", options.format)
	}
	options.color.value = strings.ToLower(strings.TrimSpace(options.color.value))
	if _, err := output.ParseColorMode(options.color.value); err != nil {
		return parsedObj{}, err
	}
	return parsedObj{command: "discover", discover: options}, nil
}

func parseVersion(args []string) (parsedObj, error) {
	flags := newFlagSet("version")
	options := versionOptionsObj{format: formatText}
	options.color.value = string(output.ColorAuto)
	flags.StringVar(&options.format, "format", formatText, "output format: text|json")
	flags.Var(&options.color, "color", "color mode: auto|always|never")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return helpResult("version", options.color), nil
		}
		return parsedObj{}, err
	}
	if err := requireNoArgs(flags); err != nil {
		return parsedObj{}, err
	}
	options.format = strings.ToLower(strings.TrimSpace(options.format))
	if options.format != formatText && options.format != formatJSON {
		return parsedObj{}, fmt.Errorf("unknown version format %q (allowed: text|json)", options.format)
	}
	options.color.value = strings.ToLower(strings.TrimSpace(options.color.value))
	if _, err := output.ParseColorMode(options.color.value); err != nil {
		return parsedObj{}, err
	}
	return parsedObj{command: "version", version: options}, nil
}

func parseInfo(args []string) (parsedObj, error) {
	flags := newFlagSet("info")
	options := infoOptionsObj{format: formatText}
	options.color.value = string(output.ColorAuto)
	flags.StringVar(&options.format, "format", formatText, "output format: text|json")
	flags.Var(&options.color, "color", "color mode: auto|always|never")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return helpResult("info", options.color), nil
		}
		return parsedObj{}, err
	}
	if err := requireNoArgs(flags); err != nil {
		return parsedObj{}, err
	}
	options.format = strings.ToLower(strings.TrimSpace(options.format))
	if options.format != formatText && options.format != formatJSON {
		return parsedObj{}, fmt.Errorf("unknown info format %q (allowed: text|json)", options.format)
	}
	options.color.value = strings.ToLower(strings.TrimSpace(options.color.value))
	if _, err := output.ParseColorMode(options.color.value); err != nil {
		return parsedObj{}, err
	}
	return parsedObj{command: "info", info: options}, nil
}

func parseHelp(args []string) (parsedObj, error) {
	options := helpOptionsObj{}
	options.color.value = string(output.ColorAuto)

	for index := 0; index < len(args); index++ {
		value := args[index]
		switch {
		case value == "-h" || value == "--help" || value == "-help":
			continue
		case value == "--color" || value == "-color":
			index++
			if index >= len(args) {
				return parsedObj{}, errors.New("flag needs an argument: --color")
			}
			if err := options.color.Set(args[index]); err != nil {
				return parsedObj{}, err
			}
		case strings.HasPrefix(value, "--color="):
			if err := options.color.Set(strings.TrimPrefix(value, "--color=")); err != nil {
				return parsedObj{}, err
			}
		case strings.HasPrefix(value, "-color="):
			if err := options.color.Set(strings.TrimPrefix(value, "-color=")); err != nil {
				return parsedObj{}, err
			}
		case strings.HasPrefix(value, "-"):
			return parsedObj{}, fmt.Errorf("unknown help flag %q", value)
		default:
			if options.command != "" {
				return parsedObj{}, errors.New("help accepts at most one command name")
			}
			options.command = canonicalCommand(value)
			if !knownCommand(options.command) {
				return parsedObj{}, fmt.Errorf("unknown command %q", value)
			}
		}
	}

	options.color.value = strings.ToLower(strings.TrimSpace(options.color.value))
	if _, err := output.ParseColorMode(options.color.value); err != nil {
		return parsedObj{}, err
	}
	return parsedObj{command: "help", help: options}, nil
}

func canonicalCommand(command string) string {
	switch command {
	case "i":
		return "info"
	case "h":
		return "help"
	default:
		return command
	}
}

func parse(args []string) (parsedObj, error) {
	if len(args) == 0 {
		return parseInfo(nil)
	}
	command := canonicalCommand(args[0])
	commandArgs := args[1:]
	switch command {
	case "run":
		return parseRun(commandArgs)
	case "monitor":
		return parseMonitor(commandArgs)
	case "events":
		return parseEvents(commandArgs)
	case "inspect":
		return parseInspect(commandArgs)
	case "discover":
		return parseDiscover(commandArgs)
	case "version":
		return parseVersion(commandArgs)
	case "info", "-i", "--info":
		return parseInfo(commandArgs)
	case "help", "-h", "--help", "-help":
		return parseHelp(commandArgs)
	default:
		return parsedObj{}, fmt.Errorf("unknown command %q", args[0])
	}
}
