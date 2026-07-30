package output

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
)

// // // // // // // // // //

// ColorModeID controls ANSI color rendering.
type ColorModeID string

const (
	// ColorAuto emits ANSI color only for terminals without NO_COLOR.
	ColorAuto ColorModeID = "auto"
	// ColorAlways emits ANSI color regardless of terminal detection.
	ColorAlways ColorModeID = "always"
	// ColorNever disables ANSI color.
	ColorNever ColorModeID = "never"
)

// LevelID is the minimum diagnostic severity.
type LevelID int

const (
	// LevelDebug includes per-link and other diagnostic detail.
	LevelDebug LevelID = iota
	// LevelInfo includes normal lifecycle records.
	LevelInfo
	// LevelWarn includes recoverable outages and removals.
	LevelWarn
	// LevelError includes only fatal failures.
	LevelError
)

// ParseColorMode validates a color-mode name.
func ParseColorMode(value string) (ColorModeID, error) {
	switch mode := ColorModeID(strings.ToLower(strings.TrimSpace(value))); mode {
	case ColorAuto, ColorAlways, ColorNever:
		return mode, nil
	default:
		return "", fmt.Errorf("unknown color mode %q (allowed: auto|always|never)", value)
	}
}

// ParseLevel validates a diagnostic level name.
func ParseLevel(value string) (LevelID, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return LevelDebug, nil
	case "info", "":
		return LevelInfo, nil
	case "warn":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	default:
		return 0, fmt.Errorf("unknown log level %q (allowed: debug|info|warn|error)", value)
	}
}

func levelName(level LevelID) string {
	switch level {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	default:
		return "error"
	}
}

// StyleObj applies a fixed, independently testable color policy.
type StyleObj struct {
	green   *color.Color
	yellow  *color.Color
	red     *color.Color
	cyan    *color.Color
	blue    *color.Color
	magenta *color.Color
	bold    *color.Color
	dim     *color.Color
}

// NewStyle constructs a color palette without modifying package globals.
func NewStyle(enabled bool) StyleObj {
	newColor := func(attributes ...color.Attribute) *color.Color {
		obj := color.New(attributes...)
		if enabled {
			obj.EnableColor()
		} else {
			obj.DisableColor()
		}
		return obj
	}
	return StyleObj{
		green: newColor(color.FgHiGreen), yellow: newColor(color.FgHiYellow),
		red: newColor(color.FgHiRed), cyan: newColor(color.FgHiCyan),
		blue: newColor(color.FgHiBlue), magenta: newColor(color.FgHiMagenta),
		bold: newColor(color.Bold), dim: newColor(color.Faint),
	}
}

// Green styles successful text.
func (obj StyleObj) Green(value any) string {
	return obj.green.Sprint(value)
}

// Yellow styles warning text.
func (obj StyleObj) Yellow(value any) string {
	return obj.yellow.Sprint(value)
}

// Red styles error text.
func (obj StyleObj) Red(value any) string {
	return obj.red.Sprint(value)
}

// Cyan styles identity and value text.
func (obj StyleObj) Cyan(value any) string {
	return obj.cyan.Sprint(value)
}

// Blue styles placeholders and secondary values.
func (obj StyleObj) Blue(value any) string {
	return obj.blue.Sprint(value)
}

// Magenta styles titles and section headings.
func (obj StyleObj) Magenta(value any) string {
	return obj.magenta.Sprint(value)
}

// Bold styles heading text.
func (obj StyleObj) Bold(value any) string {
	return obj.bold.Sprint(value)
}

// Dim styles secondary text.
func (obj StyleObj) Dim(value any) string {
	return obj.dim.Sprint(value)
}

// LoggerObj renders bounded, line-oriented diagnostics.
type LoggerObj struct {
	mu    sync.Mutex
	dst   io.Writer
	now   func() time.Time
	style StyleObj
	level LevelID
	json  bool
	err   error
}

// NewLogger creates a logger for stderr-style diagnostics.
func NewLogger(dst io.Writer, now func() time.Time, style StyleObj, level LevelID, jsonFormat bool) *LoggerObj {
	if now == nil {
		now = time.Now
	}
	return &LoggerObj{dst: dst, now: now, style: style, level: level, json: jsonFormat}
}

func (obj *LoggerObj) write(level LevelID, message string, fields map[string]any, force bool) error {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	if obj.err != nil {
		return obj.err
	}
	if !force && level < obj.level {
		return nil
	}

	var err error
	if obj.json {
		record := struct {
			Schema  int            `json:"schema"`
			Type    string         `json:"type"`
			Time    string         `json:"time"`
			Level   string         `json:"level"`
			Message string         `json:"message"`
			Fields  map[string]any `json:"fields,omitempty"`
		}{
			Schema: SchemaVersion, Type: "log",
			Time:  obj.now().UTC().Format(time.RFC3339Nano),
			Level: levelName(level), Message: message, Fields: fields,
		}
		err = jsonEncode(obj.dst, record)
	} else {
		label := strings.ToUpper(levelName(level))
		switch level {
		case LevelDebug:
			label = obj.style.Dim(label)
		case LevelInfo:
			label = obj.style.Cyan(label)
		case LevelWarn:
			label = obj.style.Yellow(label)
		case LevelError:
			label = obj.style.Red(label)
		}
		_, err = fmt.Fprintf(obj.dst, "%s %s", label, escapeText(message))
		if err == nil && len(fields) > 0 {
			_, err = fmt.Fprintf(obj.dst, " %s", formatFields(fields))
		}
		if err == nil {
			_, err = fmt.Fprintln(obj.dst)
		}
	}
	if err != nil {
		obj.err = fmt.Errorf("write diagnostic output: %w", err)
	}
	return obj.err
}

// Log writes a diagnostic when its severity passes the configured threshold.
func (obj *LoggerObj) Log(level LevelID, message string, fields map[string]any) error {
	return obj.write(level, message, fields, false)
}

// FinalError writes an error regardless of the configured threshold.
func (obj *LoggerObj) FinalError(message string) error {
	return obj.write(LevelError, message, nil, true)
}

// Err returns the first diagnostic-output failure.
func (obj *LoggerObj) Err() error {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	return obj.err
}
