package app

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/voluminor/dji-rc-joystick/cli/internal/output"
	"github.com/voluminor/dji-rc-joystick/target"
)

// // // // // // // // // //

var commandOrder = [...]string{
	"run", "monitor", "events", "inspect", "discover", "info", "version", "help",
}

var commandSummary = map[string]string{
	"run":      "Maintain the remote connection and a virtual joystick.",
	"monitor":  "Show a live terminal dashboard without a virtual joystick.",
	"events":   "Stream decoded state, status, and device information as NDJSON.",
	"inspect":  "Inspect raw channels and bounded DUML packet diagnostics.",
	"discover": "Find reachable DJI remotes on local networks.",
	"info":     "Show build details, supported remotes, and dependencies.",
	"version":  "Print short build version information.",
	"help":     "Show general or command-specific help.",
}

var commandAliases = map[string]string{
	"info": "i, -i, --info",
	"help": "h, -h, --help",
}

var syntaxTokenRX = regexp.MustCompile(`--?[A-Za-z][A-Za-z0-9-]*|<[^>]+>|\[[^]]+\]`)

type helpFlagObj struct {
	syntax       string
	description  string
	values       string
	defaultValue string
}

type helpSectionObj struct {
	title string
	flags []helpFlagObj
}

type helpPageObj struct {
	usage    string
	summary  string
	sections []helpSectionObj
	notes    []string
}

func knownCommand(command string) bool {
	_, found := commandSummary[command]
	return found
}

func shortHash(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	return hash
}

func colorizeSyntax(style output.StyleObj, value string) string {
	return syntaxTokenRX.ReplaceAllStringFunc(value, func(token string) string {
		if strings.HasPrefix(token, "<") || strings.HasPrefix(token, "[") {
			return style.Blue(token)
		}
		return style.Cyan(token)
	})
}

func title(style output.StyleObj) string {
	return style.Bold(style.Magenta(target.Name)) + " " +
		style.Green(target.Version) + " " +
		style.Yellow(shortHash(target.Hash))
}

func connectionFlags() []helpFlagObj {
	return []helpFlagObj{
		{syntax: "--ip <address>", description: "Use a literal remote IP; empty enables discovery.", defaultValue: "discovery"},
		{syntax: "--port <port>", description: "Select the remote TCP port.", defaultValue: "profile policy"},
		{syntax: "--profile <id>", description: "Force a remote profile.", values: profileIDs(), defaultValue: "auto-detect"},
		{syntax: "--keepalive <mode>", description: "Select the stream keepalive.", values: "version | heartbeat | off", defaultValue: "profile policy"},
		{syntax: "--connections <count>", description: "Maintain overlapping reader links.", values: "1..8", defaultValue: "profile policy"},
		{syntax: "--handshake", description: "Answer recurring remote poll packets.", defaultValue: "false"},
		{syntax: "--query-info=<bool>", description: "Query model and firmware once.", values: "true | false", defaultValue: "true"},
		{syntax: "--connect-timeout <duration>", description: "Limit the wait for the first online link.", defaultValue: "0 (unlimited)"},
		{syntax: "--disconnect-timeout <duration>", description: "Limit a complete outage after first connection.", defaultValue: "0 (unlimited)"},
	}
}

func diagnosticFlags() []helpFlagObj {
	return []helpFlagObj{
		{syntax: "--debug", description: "Enable debug logs and per-link diagnostics.", defaultValue: "false"},
		{syntax: "--quiet", description: "Suppress everything except fatal errors.", defaultValue: "false"},
		{syntax: "--log-level <level>", description: "Set the minimum diagnostic severity.", values: "debug | info | warn | error", defaultValue: "info"},
		{syntax: "--log-format <format>", description: "Select stderr diagnostic records.", values: "text | json", defaultValue: "text"},
		{syntax: "--color <mode>", description: "Control ANSI color in text output.", values: "auto | always | never", defaultValue: "auto"},
	}
}

func commandPage(command string) helpPageObj {
	page := helpPageObj{
		usage:   command + " [flags]",
		summary: commandSummary[command],
	}

	switch command {
	case "run":
		page.sections = []helpSectionObj{
			{title: "Connection flags", flags: connectionFlags()},
			{title: "Diagnostic flags", flags: diagnosticFlags()},
			{title: "Run flags", flags: []helpFlagObj{
				{syntax: "--config <path>", description: "Read only the mapping section from JSON or YAML.", defaultValue: "built-in mapping"},
			}},
		}
		page.notes = []string{
			"Stdout is always empty; runtime status, logs, and errors use stderr.",
			"A complete two-second outage removes the virtual joystick; reconnecting recreates it.",
		}
	case "monitor":
		page.sections = []helpSectionObj{
			{title: "Connection flags", flags: connectionFlags()},
			{title: "Diagnostic flags", flags: diagnosticFlags()},
		}
		page.notes = []string{
			"Stdout must be an interactive terminal. Use events for pipelines.",
			"The dashboard refreshes at no more than 20 Hz without throttling RC frames.",
		}
	case "events":
		page.sections = []helpSectionObj{
			{title: "Connection flags", flags: connectionFlags()},
			{title: "Diagnostic flags", flags: diagnosticFlags()},
			{title: "Event flags", flags: []helpFlagObj{
				{syntax: "--event <type>", description: "Include an event type; repeatable.", values: "state | status | info", defaultValue: "all"},
			}},
		}
		page.notes = []string{
			"Stdout is NDJSON v1. Every decoded frame produces one state record.",
			"A slow consumer fails explicitly; a downstream broken pipe exits successfully.",
		}
	case "inspect":
		page.sections = []helpSectionObj{
			{title: "Connection flags", flags: connectionFlags()},
			{title: "Diagnostic flags", flags: diagnosticFlags()},
			{title: "Inspect flags", flags: []helpFlagObj{
				{syntax: "--format <format>", description: "Select raw and packet output.", values: "text | ndjson", defaultValue: "text"},
			}},
		}
		page.notes = []string{"Raw payloads and packet excerpts are bounded by mod/rc."}
	case "discover":
		page.sections = []helpSectionObj{{title: "Discovery flags", flags: []helpFlagObj{
			{syntax: "--port <port>", description: "Select the discovery TCP port.", defaultValue: "40007"},
			{syntax: "--timeout <duration>", description: "Limit the complete network scan.", defaultValue: "0 (natural completion)"},
			{syntax: "--format <format>", description: "Select the result format.", values: "text | json", defaultValue: "text"},
			{syntax: "--color <mode>", description: "Control ANSI color in text output.", values: "auto | always | never", defaultValue: "auto"},
		}}}
		page.notes = []string{"JSON mode never emits progress or ANSI control sequences."}
	case "info":
		page.sections = []helpSectionObj{{title: "Information flags", flags: []helpFlagObj{
			{syntax: "--format <format>", description: "Select the information format.", values: "text | json", defaultValue: "text"},
			{syntax: "--color <mode>", description: "Control ANSI color in text output.", values: "auto | always | never", defaultValue: "auto"},
		}}}
		page.notes = []string{"Aliases: i, -i, --info. Running without arguments also shows info."}
	case "version":
		page.sections = []helpSectionObj{{title: "Version flags", flags: []helpFlagObj{
			{syntax: "--format <format>", description: "Select the version format.", values: "text | json", defaultValue: "text"},
			{syntax: "--color <mode>", description: "Control ANSI color in text output.", values: "auto | always | never", defaultValue: "auto"},
		}}}
	case "help":
		page.usage = "help [command] [--color <mode>]"
		page.sections = []helpSectionObj{{title: "Help flags", flags: []helpFlagObj{
			{syntax: "--color <mode>", description: "Control ANSI color.", values: "auto | always | never", defaultValue: "auto"},
		}}}
		page.notes = []string{"Aliases: h, -h, --help."}
	}
	return page
}

func writeFlagSection(builder *strings.Builder, style output.StyleObj, section helpSectionObj) {
	builder.WriteString("\n")
	builder.WriteString(style.Bold(style.Magenta(section.title)))
	builder.WriteString("\n")

	width := 0
	for _, flag := range section.flags {
		if len(flag.syntax) > width {
			width = len(flag.syntax)
		}
	}
	for _, flag := range section.flags {
		builder.WriteString("  ")
		builder.WriteString(colorizeSyntax(style, flag.syntax))
		builder.WriteString(strings.Repeat(" ", width-len(flag.syntax)+2))
		builder.WriteString(flag.description)
		if flag.values != "" {
			builder.WriteString("  ")
			builder.WriteString(style.Dim("values:"))
			builder.WriteString(" ")
			builder.WriteString(style.Yellow(flag.values))
		}
		if flag.defaultValue != "" {
			builder.WriteString("  ")
			builder.WriteString(style.Dim("default:"))
			builder.WriteString(" ")
			builder.WriteString(style.Yellow(flag.defaultValue))
		}
		builder.WriteString("\n")
	}
}

func renderCommandHelp(program, command string, style output.StyleObj) string {
	page := commandPage(command)
	var builder strings.Builder
	builder.WriteString(title(style))
	builder.WriteString("\n")
	builder.WriteString(page.summary)
	builder.WriteString("\n\n")
	builder.WriteString(style.Bold(style.Magenta("Usage")))
	builder.WriteString("\n  ")
	builder.WriteString(style.Dim("$"))
	builder.WriteString(" ")
	builder.WriteString(style.Cyan(program))
	builder.WriteString(" ")
	usageParts := strings.SplitN(page.usage, " ", 2)
	builder.WriteString(style.Cyan(usageParts[0]))
	if len(usageParts) == 2 {
		builder.WriteString(" ")
		builder.WriteString(colorizeSyntax(style, usageParts[1]))
	}
	builder.WriteString("\n")

	for _, section := range page.sections {
		writeFlagSection(&builder, style, section)
	}
	if len(page.notes) > 0 {
		builder.WriteString("\n")
		builder.WriteString(style.Bold(style.Magenta("Important")))
		builder.WriteString("\n")
		for _, note := range page.notes {
			builder.WriteString("  ")
			builder.WriteString(style.Yellow("•"))
			builder.WriteString(" ")
			builder.WriteString(note)
			builder.WriteString("\n")
		}
	}
	return builder.String()
}

func renderGeneralHelp(program string, style output.StyleObj) string {
	var builder strings.Builder
	builder.WriteString(title(style))
	builder.WriteString("\n")
	builder.WriteString("DJI remote controller bridge for terminals and data pipelines.\n\n")
	builder.WriteString(style.Bold(style.Magenta("Usage")))
	builder.WriteString("\n  ")
	builder.WriteString(style.Dim("$"))
	builder.WriteString(" ")
	builder.WriteString(style.Cyan(program))
	builder.WriteString(" ")
	builder.WriteString(style.Blue("<command>"))
	builder.WriteString(" ")
	builder.WriteString(style.Blue("[flags]"))
	builder.WriteString("\n\n")
	builder.WriteString(style.Bold(style.Magenta("Commands")))
	builder.WriteString("\n")
	for _, command := range commandOrder {
		builder.WriteString("  ")
		builder.WriteString(style.Cyan(fmt.Sprintf("%-9s", command)))
		builder.WriteString(commandSummary[command])
		if aliases := commandAliases[command]; aliases != "" {
			builder.WriteString("  ")
			builder.WriteString(style.Dim("aliases: "))
			builder.WriteString(style.Cyan(aliases))
		}
		builder.WriteString("\n")
	}
	builder.WriteString("\n")
	builder.WriteString(style.Bold(style.Magenta("Detailed help")))
	builder.WriteString("\n  ")
	builder.WriteString(style.Dim("$"))
	builder.WriteString(" ")
	builder.WriteString(style.Cyan(program))
	builder.WriteString(" ")
	builder.WriteString(style.Cyan("help"))
	builder.WriteString(" ")
	builder.WriteString(style.Blue("<command>"))
	builder.WriteString("\n")
	return builder.String()
}

func commandHelp(command string) string {
	return renderCommandHelp(target.Name, command, output.NewStyle(false))
}

func writeHelp(dst io.Writer, options helpOptionsObj, dependencies DependenciesObj) error {
	style, err := styleFor(
		options.color.value,
		dependencies.StdoutTTY,
		dependencies.NoColor && !options.color.set,
	)
	if err != nil {
		return err
	}
	text := renderGeneralHelp(dependencies.Program, style)
	if options.command != "" {
		text = renderCommandHelp(dependencies.Program, options.command, style)
	}
	if _, err = io.WriteString(dst, text); err != nil {
		return fmt.Errorf("write help: %w", err)
	}
	return nil
}
