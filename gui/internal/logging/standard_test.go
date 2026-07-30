package logging

import (
	"bytes"
	"log"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

// // // // // // // // // //

func preserveStandardLogger(t *testing.T) {
	t.Helper()
	output := log.Writer()
	flags := log.Flags()
	prefix := log.Prefix()
	t.Cleanup(func() {
		log.SetOutput(output)
		log.SetFlags(flags)
		log.SetPrefix(prefix)
	})
}

func TestStandardWriterBoundsAndClassifiesFragmentedLines(t *testing.T) {
	var output bytes.Buffer
	logger := zerolog.New(&output).Level(zerolog.TraceLevel)
	writer := newStandardWriter(logger)

	if _, err := writer.Write([]byte("Fyne ")); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("hint: use fyne.Do\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("panic: unexpected framework state\n")); err != nil {
		t.Fatal(err)
	}
	longLine := strings.Repeat("x", maxStandardLogLineBytes+1) + "\n"
	if size, err := writer.Write([]byte(longLine)); err != nil || size != len(longLine) {
		t.Fatalf("Write() = %d, %v", size, err)
	}
	writer.Flush()

	logText := output.String()
	if !strings.Contains(logText, `"level":"warn"`) ||
		!strings.Contains(logText, `"message":"Fyne hint: use fyne.Do"`) ||
		!strings.Contains(
			logText,
			`"level":"error","source":"standard_log","message":"panic: unexpected framework state"`,
		) ||
		!strings.Contains(logText, `"reason":"line_too_long"`) {
		t.Fatalf("standard log output = %s", logText)
	}
	if strings.Contains(logText, strings.Repeat("x", 1024)) {
		t.Fatal("oversized standard-log input was retained")
	}
}

func TestRedirectStandardHasOneProcessOwner(t *testing.T) {
	preserveStandardLogger(t)
	config := defaultLogging()
	config.File.Enabled = false
	first, err := newWithConsole(
		config,
		filepath.Join(t.TempDir(), "first.json"),
		&bytes.Buffer{},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = first.Close()
	})
	second, err := newWithConsole(
		config,
		filepath.Join(t.TempDir(), "second.json"),
		&bytes.Buffer{},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = second.Close()
	})

	if err = first.RedirectStandard("fyne"); err != nil {
		t.Fatal(err)
	}
	if err = second.RedirectStandard("fyne"); err == nil {
		t.Fatal("second logging owner redirected the process standard logger")
	}
	if err = first.Close(); err != nil {
		t.Fatal(err)
	}
	if err = second.RedirectStandard("fyne"); err != nil {
		t.Fatalf("RedirectStandard() after release error = %v", err)
	}
	if err = second.Close(); err != nil {
		t.Fatal(err)
	}
}
