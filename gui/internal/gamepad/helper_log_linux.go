//go:build linux

package gamepad

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"

	"github.com/rs/zerolog"
)

// // // // // // // // // //

const maxHelperLogLineBytes = 16 << 10

type helperLogWriterObj struct {
	log zerolog.Logger

	mu       sync.Mutex
	pid      int
	pending  []byte
	tail     []byte
	dropping bool
}

func newHelperLogWriter(logger zerolog.Logger) *helperLogWriterObj {
	return &helperLogWriterObj{
		log:     logger,
		pending: make([]byte, 0, 1024),
		tail:    make([]byte, 0, maxHelperStderrBytes),
	}
}

func (obj *helperLogWriterObj) appendTail(data []byte) {
	if len(data) >= maxHelperStderrBytes {
		obj.tail = append(obj.tail[:0], data[len(data)-maxHelperStderrBytes:]...)
		return
	}
	overflow := len(obj.tail) + len(data) - maxHelperStderrBytes
	if overflow > 0 {
		copy(obj.tail, obj.tail[overflow:])
		obj.tail = obj.tail[:len(obj.tail)-overflow]
	}
	obj.tail = append(obj.tail, data...)
}

func (obj *helperLogWriterObj) emitUnstructured(line []byte, reason string) {
	text := strings.TrimSpace(string(line))
	if len(line) > 0 {
		obj.appendTail(line)
		obj.appendTail([]byte{'\n'})
	}
	event := obj.log.Warn().
		Str("source", "gamepad_helper").
		Int("helper_pid", obj.pid).
		Str("reason", reason)
	if text != "" {
		event = event.Str("helper_output", text)
	}
	event.Msg("Unstructured privileged helper output")
}

func (obj *helperLogWriterObj) emitRecord(line []byte) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return
	}

	var fields map[string]any
	if err := json.Unmarshal(line, &fields); err != nil {
		obj.emitUnstructured(line, "invalid_json")
		return
	}
	levelText, levelOK := fields[zerolog.LevelFieldName].(string)
	message, messageOK := fields[zerolog.MessageFieldName].(string)
	level, err := zerolog.ParseLevel(levelText)
	if err != nil || !levelOK || !messageOK {
		obj.emitUnstructured(line, "invalid_record")
		return
	}
	delete(fields, zerolog.LevelFieldName)
	delete(fields, zerolog.MessageFieldName)

	pid := obj.pid
	if pid == 0 {
		if value, ok := fields["pid"].(float64); ok &&
			value >= 1 &&
			value <= (1<<31)-1 {
			pid = int(value)
		}
	}
	obj.log.WithLevel(level).
		Str("source", "gamepad_helper").
		Int("helper_pid", pid).
		Interface("helper_fields", fields).
		Msg(message)
}

func (obj *helperLogWriterObj) Write(data []byte) (int, error) {
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
		if len(obj.pending)+len(part) > maxHelperLogLineBytes {
			obj.pending = obj.pending[:0]
			obj.emitUnstructured(nil, "line_too_long")
			obj.dropping = !complete
			continue
		}

		obj.pending = append(obj.pending, part...)
		if complete {
			obj.emitRecord(obj.pending)
			obj.pending = obj.pending[:0]
		}
	}

	return size, nil
}

func (obj *helperLogWriterObj) setPID(pid int) {
	obj.mu.Lock()
	obj.pid = pid
	obj.mu.Unlock()
}

func (obj *helperLogWriterObj) Flush() {
	obj.mu.Lock()
	defer obj.mu.Unlock()

	if len(obj.pending) > 0 {
		obj.emitUnstructured(obj.pending, "incomplete_line")
		obj.pending = obj.pending[:0]
	}
	obj.dropping = false
}

func (obj *helperLogWriterObj) String() string {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	return strings.TrimSpace(string(obj.tail))
}
