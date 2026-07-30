package logging

import (
	"bytes"
	"errors"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/rs/zerolog"
)

// // // // // // // // // //

const maxStandardLogLineBytes = 16 << 10

var (
	standardRedirectMu sync.Mutex
	activeStandard     *standardRedirectObj
)

type standardWriterObj struct {
	log zerolog.Logger

	mu       sync.Mutex
	pending  []byte
	dropping bool
}

type standardRedirectObj struct {
	writer *standardWriterObj
	output io.Writer
	flags  int
	prefix string

	restoreOnce sync.Once
}

func newStandardWriter(logger zerolog.Logger) *standardWriterObj {
	return &standardWriterObj{
		log:     logger,
		pending: make([]byte, 0, 1024),
	}
}

func standardLevel(message string) zerolog.Level {
	normalized := strings.ToLower(strings.TrimSpace(message))
	switch {
	case strings.Contains(normalized, "error"),
		strings.Contains(normalized, "failed"),
		strings.Contains(normalized, "could not"),
		strings.Contains(normalized, "unable to"),
		strings.Contains(normalized, "fatal"),
		strings.Contains(normalized, "panic"),
		strings.HasPrefix(normalized, "cause:"),
		strings.HasPrefix(normalized, "at:"):
		return zerolog.ErrorLevel
	case strings.Contains(normalized, "warn"),
		strings.Contains(normalized, "hint"),
		strings.Contains(normalized, "deprecated"),
		strings.HasPrefix(normalized, "***"):
		return zerolog.WarnLevel
	default:
		return zerolog.InfoLevel
	}
}

func (obj *standardWriterObj) emitRecord(line []byte, incomplete bool) {
	message := strings.TrimSpace(string(line))
	if message == "" {
		return
	}
	event := obj.log.WithLevel(standardLevel(message)).
		Str("source", "standard_log")
	if incomplete {
		event = event.Bool("incomplete", true)
	}
	event.Msg(message)
}

func (obj *standardWriterObj) emitOversized() {
	obj.log.Warn().
		Str("source", "standard_log").
		Str("reason", "line_too_long").
		Int("maximum_bytes", maxStandardLogLineBytes).
		Msg("Standard log line exceeded limit")
}

func (obj *standardWriterObj) Write(data []byte) (int, error) {
	size := len(data)
	obj.mu.Lock()
	defer obj.mu.Unlock()

	for len(data) > 0 {
		lineEnd := bytes.IndexByte(data, '\n')
		complete := lineEnd >= 0
		part := data
		if complete {
			part = data[:lineEnd]
			data = data[lineEnd+1:]
		} else {
			data = nil
		}

		if obj.dropping {
			if complete {
				obj.dropping = false
			}
			continue
		}
		if len(obj.pending)+len(part) > maxStandardLogLineBytes {
			obj.pending = obj.pending[:0]
			obj.emitOversized()
			obj.dropping = !complete
			continue
		}

		obj.pending = append(obj.pending, part...)
		if complete {
			obj.emitRecord(obj.pending, false)
			obj.pending = obj.pending[:0]
		}
	}

	return size, nil
}

func (obj *standardWriterObj) Flush() {
	obj.mu.Lock()
	defer obj.mu.Unlock()

	if len(obj.pending) > 0 {
		obj.emitRecord(obj.pending, true)
		obj.pending = obj.pending[:0]
	}
	obj.dropping = false
}

func (obj *standardRedirectObj) restore() {
	obj.restoreOnce.Do(func() {
		if current, ok := log.Writer().(*standardWriterObj); ok &&
			current == obj.writer {
			log.SetOutput(obj.output)
			log.SetFlags(obj.flags)
			log.SetPrefix(obj.prefix)
		}
		obj.writer.Flush()
	})
}

// RedirectStandard routes the process standard logger through structured sinks.
func (obj *Obj) RedirectStandard(component string) error {
	component = strings.TrimSpace(component)
	if component == "" {
		return errors.New("standard logger component must not be empty")
	}

	standardRedirectMu.Lock()
	defer standardRedirectMu.Unlock()
	obj.redirectMu.Lock()
	defer obj.redirectMu.Unlock()
	if obj.closed {
		return errors.New("logging outputs are closed")
	}
	if obj.standard != nil {
		return errors.New("standard logger is already redirected")
	}
	if activeStandard != nil {
		return errors.New("standard logger is redirected by another logging owner")
	}

	writer := newStandardWriter(obj.logger.With().
		Str("component", component).
		Logger())
	redirect := &standardRedirectObj{
		writer: writer,
		output: log.Writer(),
		flags:  log.Flags(),
		prefix: log.Prefix(),
	}
	log.SetFlags(0)
	log.SetPrefix("")
	log.SetOutput(writer)
	obj.standard = redirect
	activeStandard = redirect
	return nil
}

func (obj *Obj) detachStandard() {
	standardRedirectMu.Lock()
	defer standardRedirectMu.Unlock()
	obj.redirectMu.Lock()
	obj.closed = true
	redirect := obj.standard
	obj.standard = nil
	obj.redirectMu.Unlock()

	if redirect != nil {
		if activeStandard == redirect {
			activeStandard = nil
		}
		redirect.restore()
	}
}
