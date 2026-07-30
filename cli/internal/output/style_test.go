package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// // // // // // // // // //

func TestLoggerJSONIsVersionedAndHasNoANSI(t *testing.T) {
	var destination bytes.Buffer
	now := func() time.Time {
		return time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)
	}
	logger := NewLogger(&destination, now, NewStyle(true), LevelDebug, true)
	if err := logger.Log(LevelInfo, "Remote online", map[string]any{"links": 2}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(destination.String(), "\x1b[") {
		t.Fatal("JSON log contains ANSI escapes")
	}
	var record struct {
		Schema  int            `json:"schema"`
		Type    string         `json:"type"`
		Level   string         `json:"level"`
		Message string         `json:"message"`
		Fields  map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(destination.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record.Schema != SchemaVersion || record.Type != "log" ||
		record.Level != "info" || record.Message != "Remote online" {
		t.Fatalf("unexpected JSON log: %+v", record)
	}
}

func TestLoggerTextEscapesRecordBreakingControlCharacters(t *testing.T) {
	var destination bytes.Buffer
	logger := NewLogger(
		&destination,
		time.Now,
		NewStyle(false),
		LevelDebug,
		false,
	)

	if err := logger.Log(
		LevelInfo,
		"first\n\x1b[31msecond",
		map[string]any{"detail": "third\r\nfourth"},
	); err != nil {
		t.Fatal(err)
	}

	text := destination.String()
	if strings.Count(text, "\n") != 1 {
		t.Fatalf("text log was split into multiple records: %q", text)
	}
	if strings.Contains(text, "\x1b") {
		t.Fatalf("text log contains an injected ANSI escape: %q", text)
	}
	if !strings.Contains(text, `first\n\x1b[31msecond`) ||
		!strings.Contains(text, `detail=third\r\nfourth`) {
		t.Fatalf("escaped diagnostic content is missing: %q", text)
	}
}

func TestStylePolicyIsLocalToPalette(t *testing.T) {
	colored := NewStyle(true).Green("ready")
	plain := NewStyle(false).Green("ready")
	if !strings.Contains(colored, "\x1b[") {
		t.Fatalf("enabled style is not colored: %q", colored)
	}
	if plain != "ready" {
		t.Fatalf("disabled style = %q, want plain text", plain)
	}
}
