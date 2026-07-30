package app

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/voluminor/dji-rc-joystick/cli/internal/output"
	"github.com/voluminor/dji-rc-joystick/mod/rc"
)

// // // // // // // // // //

const (
	formatText   = "text"
	formatJSON   = "json"
	formatNDJSON = "ndjson"

	logFormatText = "text"
	logFormatJSON = "json"
)

type trackedStringObj struct {
	value string
	set   bool
}

func (obj *trackedStringObj) String() string {
	return obj.value
}

func (obj *trackedStringObj) Set(value string) error {
	obj.value, obj.set = value, true
	return nil
}

type stringListObj struct {
	values []string
}

func (obj *stringListObj) String() string {
	return strings.Join(obj.values, ",")
}

func (obj *stringListObj) Set(value string) error {
	obj.values = append(obj.values, value)
	return nil
}

type runtimeOptionsObj struct {
	ip                string
	port              int
	profile           string
	keepalive         string
	connections       int
	handshake         bool
	queryInfo         bool
	connectTimeout    time.Duration
	disconnectTimeout time.Duration
	debug             bool
	quiet             bool
	logLevel          trackedStringObj
	logFormat         string
	color             trackedStringObj
}

type runOptionsObj struct {
	runtime runtimeOptionsObj
	config  string
}

type eventOptionsObj struct {
	runtime runtimeOptionsObj
	events  stringListObj
}

type inspectOptionsObj struct {
	runtime runtimeOptionsObj
	format  string
}

type discoverOptionsObj struct {
	port    int
	timeout time.Duration
	format  string
	color   trackedStringObj
}

type versionOptionsObj struct {
	format string
	color  trackedStringObj
}

type infoOptionsObj struct {
	format string
	color  trackedStringObj
}

type helpOptionsObj struct {
	command string
	color   trackedStringObj
}

type parsedObj struct {
	command  string
	help     helpOptionsObj
	run      runOptionsObj
	monitor  runtimeOptionsObj
	events   eventOptionsObj
	inspect  inspectOptionsObj
	discover discoverOptionsObj
	version  versionOptionsObj
	info     infoOptionsObj
}

func newFlagSet(name string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	return flags
}

func addRuntimeFlags(flags *flag.FlagSet, options *runtimeOptionsObj) {
	flags.StringVar(&options.ip, "ip", "", "remote IP address")
	flags.IntVar(&options.port, "port", 0, "remote TCP port (0: profile default)")
	flags.StringVar(&options.profile, "profile", "", "remote profile ID (empty: auto-detect)")
	flags.StringVar(&options.keepalive, "keepalive", "", "keepalive mode: version|heartbeat|off")
	flags.IntVar(&options.connections, "connections", 0, "reader links (0: profile default)")
	flags.BoolVar(&options.handshake, "handshake", false, "answer recurring remote poll packets")
	flags.BoolVar(&options.queryInfo, "query-info", true, "query model and firmware once")
	flags.DurationVar(&options.connectTimeout, "connect-timeout", 0, "maximum wait for the first live link")
	flags.DurationVar(&options.disconnectTimeout, "disconnect-timeout", 0, "maximum complete outage after first connection")
	flags.BoolVar(&options.debug, "debug", false, "enable debug logs and per-link diagnostics")
	flags.BoolVar(&options.quiet, "quiet", false, "show errors only")
	options.logLevel.value = "info"
	flags.Var(&options.logLevel, "log-level", "diagnostic level: debug|info|warn|error")
	flags.StringVar(&options.logFormat, "log-format", logFormatText, "diagnostic format: text|json")
	options.color.value = string(output.ColorAuto)
	flags.Var(&options.color, "color", "color mode: auto|always|never")
}

func parseKeepalive(value string) (rc.KeepaliveMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return rc.KeepaliveDefault, nil
	case "version":
		return rc.KeepaliveVersion, nil
	case "heartbeat":
		return rc.KeepaliveHeartbeat, nil
	case "off":
		return rc.KeepaliveOff, nil
	default:
		return 0, fmt.Errorf("unknown keepalive mode %q (allowed: version|heartbeat|off)", value)
	}
}

func profileIDs() string {
	profiles := rc.Profiles()
	ids := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		ids = append(ids, profile.ID)
	}
	return strings.Join(ids, "|")
}

func validateRuntime(options *runtimeOptionsObj) error {
	options.ip = strings.TrimSpace(options.ip)
	options.profile = strings.TrimSpace(options.profile)
	options.keepalive = strings.ToLower(strings.TrimSpace(options.keepalive))
	options.logFormat = strings.ToLower(strings.TrimSpace(options.logFormat))
	options.color.value = strings.ToLower(strings.TrimSpace(options.color.value))

	if options.ip != "" && net.ParseIP(options.ip) == nil {
		return fmt.Errorf("invalid IP address %q", options.ip)
	}
	if options.port < 0 || options.port > 65535 {
		return errors.New("--port must be zero or between 1 and 65535")
	}
	if options.connections < 0 || options.connections > rc.MaxConnections {
		return fmt.Errorf("--connections must be zero or between 1 and %d", rc.MaxConnections)
	}
	if options.connectTimeout < 0 {
		return errors.New("--connect-timeout cannot be negative")
	}
	if options.disconnectTimeout < 0 {
		return errors.New("--disconnect-timeout cannot be negative")
	}
	if options.profile != "" {
		if _, found := rc.ProfileByID(options.profile); !found {
			return fmt.Errorf("unknown profile %q (available: %s)", options.profile, profileIDs())
		}
	}
	if _, err := parseKeepalive(options.keepalive); err != nil {
		return err
	}
	if _, err := output.ParseColorMode(options.color.value); err != nil {
		return err
	}
	if options.logFormat != logFormatText && options.logFormat != logFormatJSON {
		return fmt.Errorf("unknown log format %q (allowed: text|json)", options.logFormat)
	}

	shorthands := 0
	if options.debug {
		shorthands++
	}
	if options.quiet {
		shorthands++
	}
	if options.logLevel.set {
		shorthands++
	}
	if shorthands > 1 {
		return errors.New("--debug, --quiet, and an explicit --log-level are mutually exclusive")
	}
	if options.debug {
		options.logLevel.value = "debug"
	}
	if options.quiet {
		options.logLevel.value = "error"
	}
	if _, err := output.ParseLevel(options.logLevel.value); err != nil {
		return err
	}
	return nil
}

func requireNoArgs(flags *flag.FlagSet) error {
	if flags.NArg() != 0 {
		return fmt.Errorf("%s accepts no positional arguments", flags.Name())
	}
	return nil
}
