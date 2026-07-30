package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// // // // // // // // // //

func TestParseCommandContracts(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		command string
		helpFor string
	}{
		{name: "default info", command: "info"},
		{name: "info", args: []string{"info"}, command: "info"},
		{name: "info short command", args: []string{"i"}, command: "info"},
		{name: "info long flag", args: []string{"--info"}, command: "info"},
		{name: "info short flag", args: []string{"-i"}, command: "info"},
		{name: "run", args: []string{"run", "--ip", "192.0.2.1"}, command: "run"},
		{name: "monitor", args: []string{"monitor"}, command: "monitor"},
		{name: "events", args: []string{"events", "--event", "state", "--event", "status"}, command: "events"},
		{name: "inspect", args: []string{"inspect", "--format", "ndjson"}, command: "inspect"},
		{name: "discover", args: []string{"discover", "--format", "json"}, command: "discover"},
		{name: "version", args: []string{"version"}, command: "version"},
		{name: "help command", args: []string{"help", "run"}, command: "help", helpFor: "run"},
		{name: "help short command", args: []string{"h", "run"}, command: "help", helpFor: "run"},
		{name: "help long flag", args: []string{"--help"}, command: "help"},
		{name: "help short flag", args: []string{"-h"}, command: "help"},
		{name: "flag help", args: []string{"inspect", "--help"}, command: "help", helpFor: "inspect"},
		{name: "short flag help", args: []string{"inspect", "-h"}, command: "help", helpFor: "inspect"},
		{name: "alias help", args: []string{"help", "i"}, command: "help", helpFor: "info"},
		{name: "help color after command", args: []string{"help", "run", "--color", "always"}, command: "help", helpFor: "run"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := parse(test.args)
			if err != nil {
				t.Fatal(err)
			}
			if parsed.command != test.command || parsed.help.command != test.helpFor {
				t.Fatalf("parsed command=%q help=%q", parsed.command, parsed.help.command)
			}
		})
	}
}

func TestParseRejectsLegacyAliasesAndPositionals(t *testing.T) {
	tests := [][]string{
		{"--list"},
		{"run", "192.0.2.1"},
		{"run", "--test"},
		{"run", "--raw"},
		{"run", "--info"},
	}
	for _, args := range tests {
		if _, err := parse(args); err == nil {
			t.Fatalf("legacy or positional arguments accepted: %v", args)
		}
	}
}

func TestCommandMetadataIsComplete(t *testing.T) {
	for _, command := range commandOrder {
		if commandSummary[command] == "" || commandPage(command).usage == "" {
			t.Fatalf("incomplete metadata for %q", command)
		}
		if !knownCommand(command) {
			t.Fatalf("command %q is missing from lookup", command)
		}
	}
}

func TestPublicFlagsAreInHelpAndREADME(t *testing.T) {
	runtimeFlags := []string{
		"ip", "port", "profile", "keepalive", "connections", "handshake",
		"query-info", "connect-timeout", "disconnect-timeout", "debug", "quiet",
		"log-level", "log-format", "color",
	}
	expected := map[string][]string{
		"run":      append(append([]string(nil), runtimeFlags...), "config"),
		"monitor":  runtimeFlags,
		"events":   append(append([]string(nil), runtimeFlags...), "event"),
		"inspect":  append(append([]string(nil), runtimeFlags...), "format"),
		"discover": {"port", "timeout", "format", "color"},
		"info":     {"format", "color"},
		"version":  {"format", "color"},
		"help":     {"color"},
	}

	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	for command, flags := range expected {
		help := commandHelp(command)
		for _, name := range flags {
			needle := "--" + name
			if !strings.Contains(help, needle) {
				t.Errorf("%s help does not document %s", command, needle)
			}
			if !strings.Contains(string(readme), needle) {
				t.Errorf("README does not document %s", needle)
			}
		}
	}
}
