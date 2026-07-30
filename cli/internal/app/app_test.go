package app

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/voluminor/dji-rc-joystick/mod/rc"
)

// // // // // // // // // //

type appBufferObj struct {
	bytes.Buffer
	closed bool
}

func (obj *appBufferObj) Close() error {
	obj.closed = true
	return nil
}

type brokenOutputObj struct{}

func (brokenOutputObj) Write([]byte) (int, error) {
	return 0, syscall.EPIPE
}

func (brokenOutputObj) Close() error {
	return nil
}

func testDependencies() (DependenciesObj, *appBufferObj, *bytes.Buffer) {
	stdout := &appBufferObj{}
	stderr := &bytes.Buffer{}
	return DependenciesObj{
		Program: "renamed-rc", Stdout: stdout, Stderr: stderr,
	}, stdout, stderr
}

// //

func TestExecuteDefaultInfoAndVersion(t *testing.T) {
	dependencies, stdout, stderr := testDependencies()
	if code := Execute(nil, dependencies); code != exitSuccess {
		t.Fatalf("info exit code = %d", code)
	}
	if !strings.Contains(stdout.String(), "Supported remotes") ||
		!strings.Contains(stdout.String(), "Dependencies") ||
		!strings.Contains(stdout.String(), `Run "--help" for detailed usage.`) ||
		stderr.Len() != 0 {
		t.Fatalf("unexpected info output: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	dependencies, stdout, stderr = testDependencies()
	if code := Execute([]string{"version", "--format", "json"}, dependencies); code != exitSuccess {
		t.Fatalf("version exit code = %d, stderr=%q", code, stderr.String())
	}
	var version versionResultObj
	if err := json.Unmarshal(stdout.Bytes(), &version); err != nil {
		t.Fatal(err)
	}
	if version.Schema != 1 || version.Name == "" || version.Version == "" || version.Hash == "" {
		t.Fatalf("incomplete version document: %+v", version)
	}
}

func TestCommandHelpFlagUsesStableHelp(t *testing.T) {
	dependencies, stdout, stderr := testDependencies()
	if code := Execute([]string{"events", "--help"}, dependencies); code != exitSuccess {
		t.Fatalf("help exit code = %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "renamed-rc events") ||
		!strings.Contains(stdout.String(), "--event") {
		t.Fatalf("unexpected events help: %q", stdout.String())
	}
}

func TestInfoAliasesAndJSON(t *testing.T) {
	for _, alias := range []string{"i", "info", "-i", "--info"} {
		t.Run(alias, func(t *testing.T) {
			dependencies, stdout, stderr := testDependencies()
			if code := Execute([]string{alias, "--format", "json"}, dependencies); code != exitSuccess {
				t.Fatalf("exit code = %d, stderr=%q", code, stderr.String())
			}
			if strings.Contains(stdout.String(), "\x1b[") ||
				strings.Contains(stdout.String(), `Run "--help"`) {
				t.Fatalf("JSON info was polluted: %q", stdout.String())
			}
			var result infoResultObj
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Schema != 1 || result.Name == "" || result.Hash == "" ||
				len(result.Profiles) == 0 || len(result.Dependencies) == 0 {
				t.Fatalf("incomplete info document: %+v", result)
			}
			if _, ok := result.Dependencies["github.com/fatih/color"]; !ok {
				t.Fatalf("CLI dependency metadata is missing github.com/fatih/color: %+v", result.Dependencies)
			}
			if _, ok := result.Dependencies["fyne.io/fyne/v2"]; ok {
				t.Fatalf("CLI dependency metadata contains GUI dependency fyne.io/fyne/v2: %+v", result.Dependencies)
			}
		})
	}
}

func TestHelpAliasesUseRichTTYColor(t *testing.T) {
	for _, alias := range []string{"h", "help", "-h", "--help"} {
		t.Run(alias, func(t *testing.T) {
			dependencies, stdout, stderr := testDependencies()
			dependencies.StdoutTTY = true
			if code := Execute([]string{alias}, dependencies); code != exitSuccess {
				t.Fatalf("exit code = %d, stderr=%q", code, stderr.String())
			}
			if count := strings.Count(stdout.String(), "\x1b["); count < 12 {
				t.Fatalf("help uses too little semantic color: %d escapes in %q", count, stdout.String())
			}
			for _, text := range []string{"Commands", "info", "help", "renamed-rc"} {
				if !strings.Contains(stdout.String(), text) {
					t.Fatalf("help misses %q: %q", text, stdout.String())
				}
			}
		})
	}
}

func TestDefaultInfoUsesRichTTYColor(t *testing.T) {
	dependencies, stdout, stderr := testDependencies()
	dependencies.StdoutTTY = true
	if code := Execute(nil, dependencies); code != exitSuccess {
		t.Fatalf("exit code = %d, stderr=%q", code, stderr.String())
	}
	if count := strings.Count(stdout.String(), "\x1b["); count < 12 {
		t.Fatalf("info uses too little semantic color: %d escapes in %q", count, stdout.String())
	}
}

func TestHelpAndInfoAutoColorStayPlainWhenRedirected(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"--info"}} {
		dependencies, stdout, stderr := testDependencies()
		if code := Execute(args, dependencies); code != exitSuccess {
			t.Fatalf("%v exit code = %d, stderr=%q", args, code, stderr.String())
		}
		if strings.Contains(stdout.String(), "\x1b[") {
			t.Fatalf("%v redirected text contains ANSI: %q", args, stdout.String())
		}
	}
}

func TestHelpColorPolicyHonorsNoColorAndExplicitOverride(t *testing.T) {
	dependencies, stdout, stderr := testDependencies()
	dependencies.StdoutTTY = true
	dependencies.NoColor = true
	if code := Execute([]string{"--help"}, dependencies); code != exitSuccess {
		t.Fatalf("auto exit code = %d, stderr=%q", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("implicit help color ignored NO_COLOR: %q", stdout.String())
	}

	dependencies, stdout, stderr = testDependencies()
	dependencies.StdoutTTY = true
	dependencies.NoColor = true
	if code := Execute([]string{"--help", "--color", "always"}, dependencies); code != exitSuccess {
		t.Fatalf("always exit code = %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("explicit help color did not override NO_COLOR: %q", stdout.String())
	}
}

func TestProgramNameIsSanitizedAndUsedByHelp(t *testing.T) {
	dependencies, stdout, stderr := testDependencies()
	dependencies.Program = "/tmp/custom\x1b[31m-name"
	if code := Execute([]string{"--help"}, dependencies); code != exitSuccess {
		t.Fatalf("exit code = %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "custom?[31m-name") ||
		strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("program name was not safely rendered: %q", stdout.String())
	}
}

func TestUsageErrorIsColoredOnlyOnTerminal(t *testing.T) {
	dependencies, _, stderr := testDependencies()
	dependencies.StderrTTY = true
	if code := Execute([]string{"unknown"}, dependencies); code != exitUsage {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stderr.String(), "\x1b[") {
		t.Fatalf("terminal error is not colored: %q", stderr.String())
	}

	dependencies, _, stderr = testDependencies()
	if code := Execute([]string{"unknown"}, dependencies); code != exitUsage {
		t.Fatalf("redirected exit code = %d", code)
	}
	if strings.Contains(stderr.String(), "\x1b[") {
		t.Fatalf("redirected error contains ANSI: %q", stderr.String())
	}
}

func TestExecuteUsageErrorsUseCodeTwo(t *testing.T) {
	tests := [][]string{
		{"unknown"},
		{"monitor"},
		{"events", "--port", "70000"},
		{"events", "--debug", "--log-level", "info"},
		{"events", "--event", "link"},
		{"discover", "--timeout", "-1s"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			dependencies, stdout, stderr := testDependencies()
			if args[0] == "monitor" {
				dependencies.StdoutTTY = false
			}
			if code := Execute(args, dependencies); code != exitUsage {
				t.Fatalf("exit code = %d, want %d", code, exitUsage)
			}
			if stdout.Len() != 0 || !strings.Contains(stderr.String(), "ERROR") {
				t.Fatalf("unexpected streams: stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}

func TestRunRejectsInvalidMappingBeforeRuntime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"ui":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	dependencies, stdout, stderr := testDependencies()
	code := Execute([]string{"run", "--config", path}, dependencies)
	if code != exitUsage {
		t.Fatalf("exit code = %d, stderr=%q", code, stderr.String())
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "no root mapping") {
		t.Fatalf("unexpected streams: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestDiscoverJSONHasNoProgressOrANSI(t *testing.T) {
	dependencies, stdout, stderr := testDependencies()
	dependencies.StdoutTTY = true
	dependencies.StderrTTY = true
	dependencies.Discover = func(_ context.Context, config *rc.DiscoverConfigObj) ([]rc.RemoteObj, error) {
		if config.OnProgress != nil {
			t.Fatal("JSON discovery installed a progress callback")
		}
		return []rc.RemoteObj{
			{IP: "192.168.0.20", Model: "B"},
			{IP: "192.168.0.10", Model: "A", Firmware: "1.2.3"},
		}, nil
	}

	if code := Execute([]string{"discover", "--format", "json"}, dependencies); code != exitSuccess {
		t.Fatalf("exit code = %d, stderr=%q", code, stderr.String())
	}
	if stderr.Len() != 0 || strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("machine output was polluted: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	var result discoverResultObj
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Remotes) != 2 || result.Remotes[0].IP != "192.168.0.10" {
		t.Fatalf("unexpected discovery result: %+v", result)
	}
}

func TestDiscoverTextShowsTTYProgress(t *testing.T) {
	dependencies, stdout, stderr := testDependencies()
	dependencies.StdoutTTY = true
	dependencies.StderrTTY = true
	dependencies.Now = func() time.Time {
		return time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)
	}
	dependencies.Discover = func(_ context.Context, config *rc.DiscoverConfigObj) ([]rc.RemoteObj, error) {
		config.OnProgress(rc.DiscoverProgressObj{
			Known: true, Probed: 1, Total: 1, Found: 1,
		})
		return []rc.RemoteObj{{IP: "192.168.0.100"}}, nil
	}

	if code := Execute([]string{"discover"}, dependencies); code != exitSuccess {
		t.Fatalf("exit code = %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "known addresses") ||
		!strings.Contains(stderr.String(), "\r\x1b[2K") {
		t.Fatalf("dynamic progress missing: %q", stderr.String())
	}
	if !strings.Contains(stdout.String(), "192.168.0.100") {
		t.Fatalf("discovery result missing: %q", stdout.String())
	}
}

func TestLongRunningCommandsReturnSignalCodes(t *testing.T) {
	tests := []struct {
		signal syscall.Signal
		code   int
	}{
		{signal: syscall.SIGINT, code: exitSIGINT},
		{signal: syscall.SIGTERM, code: exitSIGTERM},
	}
	for _, test := range tests {
		t.Run(test.signal.String(), func(t *testing.T) {
			dependencies, _, stderr := testDependencies()
			signals := make(chan os.Signal, 1)
			signals <- test.signal
			dependencies.Signals = signals
			code := Execute([]string{
				"events", "--ip", "127.0.0.1", "--port", "1", "--event", "status",
			}, dependencies)
			if code != test.code {
				t.Fatalf("exit code = %d, want %d, stderr=%q", code, test.code, stderr.String())
			}
		})
	}
}

func TestRuntimeFailureUsesJSONLog(t *testing.T) {
	dependencies, stdout, stderr := testDependencies()
	code := Execute([]string{
		"events", "--ip", "127.0.0.1", "--port", "1",
		"--connect-timeout", "30ms", "--log-format", "json",
	}, dependencies)
	if code != exitRuntime {
		t.Fatalf("exit code = %d, stderr=%q", code, stderr.String())
	}
	if strings.Contains(stderr.String(), "\x1b[") {
		t.Fatalf("JSON diagnostics contain ANSI escapes: %q", stderr.String())
	}
	var record struct {
		Schema int    `json:"schema"`
		Type   string `json:"type"`
		Level  string `json:"level"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record.Schema != 1 || record.Type != "log" || record.Level != "error" {
		t.Fatalf("unexpected final JSON log: %+v", record)
	}
	if !stdout.closed {
		t.Fatal("event command did not close its stdout destination")
	}
}

func TestBrokenPipeIsNormalCompletion(t *testing.T) {
	var stderr bytes.Buffer
	code := Execute([]string{"version"}, DependenciesObj{
		Stdout: brokenOutputObj{}, Stderr: &stderr,
	})
	if code != exitSuccess || stderr.Len() != 0 {
		t.Fatalf("EPIPE result: code=%d stderr=%q", code, stderr.String())
	}
}

func TestExplicitColorOverridesNoColor(t *testing.T) {
	dependencies, stdout, _ := testDependencies()
	dependencies.StdoutTTY = true
	dependencies.NoColor = true
	if code := Execute([]string{"version"}, dependencies); code != exitSuccess {
		t.Fatalf("default color exit code = %d", code)
	}
	if strings.Contains(stdout.String(), "\x1b[") {
		t.Fatal("implicit auto color ignored NO_COLOR")
	}

	dependencies, stdout, _ = testDependencies()
	dependencies.StdoutTTY = true
	dependencies.NoColor = true
	if code := Execute([]string{"version", "--color", "auto"}, dependencies); code != exitSuccess {
		t.Fatalf("explicit color exit code = %d", code)
	}
	if !strings.Contains(stdout.String(), "\x1b[") {
		t.Fatal("explicit color did not override NO_COLOR")
	}
}

func TestUseRunViewHonorsDiagnosticModes(t *testing.T) {
	base := runtimeOptionsObj{logFormat: logFormatText}
	dependencies := DependenciesObj{StderrTTY: true}
	if !useRunView(base, dependencies) {
		t.Fatal("normal terminal run did not enable the compact view")
	}

	debug := base
	debug.debug = true
	if useRunView(debug, dependencies) {
		t.Fatal("debug run enabled a view that would conflict with link logs")
	}

	quiet := base
	quiet.quiet = true
	if useRunView(quiet, dependencies) {
		t.Fatal("quiet run enabled diagnostic output")
	}

	jsonLogs := base
	jsonLogs.logFormat = logFormatJSON
	if useRunView(jsonLogs, dependencies) {
		t.Fatal("JSON logging enabled terminal control output")
	}
}
