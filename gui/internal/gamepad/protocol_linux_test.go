//go:build linux

package gamepad

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/voluminor/dji-rc-joystick/mod/rc"
	"github.com/voluminor/dji-rc-joystick/mod/vjoy"

	"github.com/rs/zerolog"
)

// // // // // // // // // //

func appendFrame(
	t *testing.T,
	writer *bufio.Writer,
	values []frameValueObj,
) {
	t.Helper()
	if err := writer.WriteByte(commandFrame); err != nil {
		t.Fatal(err)
	}
	var countData [2]byte
	binary.LittleEndian.PutUint16(countData[:], uint16(len(values)))
	if _, err := writer.Write(countData[:]); err != nil {
		t.Fatal(err)
	}
	var valueData [6]byte
	for _, value := range values {
		binary.LittleEndian.PutUint16(valueData[:2], value.handle)
		binary.LittleEndian.PutUint32(valueData[2:], uint32(value.value))
		if _, err := writer.Write(valueData[:]); err != nil {
			t.Fatal(err)
		}
	}
}

// //

func TestHelperProtocolAppliesBoundedFrames(t *testing.T) {
	device := newCaptureDevice()
	spec := rc.DefaultMapping().DeviceSpec(rc.DefaultProfile())
	var input bytes.Buffer
	inputWriter := bufio.NewWriter(&input)
	if err := writeSpec(inputWriter, spec); err != nil {
		t.Fatal(err)
	}
	appendFrame(t, inputWriter, []frameValueObj{
		{handle: 0, value: 123},
		{handle: 100, value: int32(rc.ModeNormal)},
	})
	if err := inputWriter.WriteByte(commandClose); err != nil {
		t.Fatal(err)
	}
	if err := inputWriter.Flush(); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	var creates atomic.Int64
	err := serveHelper(
		&input,
		&output,
		func(createdSpec vjoy.SpecObj) (vjoy.Interface, error) {
			creates.Add(1)
			if createdSpec.Name != spec.Name ||
				len(createdSpec.Controls) != len(spec.Controls) {
				t.Fatalf("helper specification = %#v, want %#v", createdSpec, spec)
			}
			return device, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := creates.Load(); got != 1 {
		t.Fatalf("device creates = %d, want 1", got)
	}
	reader := bufio.NewReader(&output)
	for response := 0; response < 3; response++ {
		if err = readResponse(reader); err != nil {
			t.Fatalf("response %d: %v", response, err)
		}
	}
	waitFrame(t, device, func(frame map[uint16]int32) bool {
		return frame[0] == 123 && frame[100] == int32(rc.ModeNormal)
	})
	if got := device.closeCount(); got != 1 {
		t.Fatalf("device closes = %d, want 1", got)
	}
}

func TestHelperProtocolRejectsOversizedSpecification(t *testing.T) {
	var input bytes.Buffer
	var size [4]byte
	binary.LittleEndian.PutUint32(size[:], maxSpecBytes+1)
	input.Write(size[:])

	var output bytes.Buffer
	var creates atomic.Int64
	err := serveHelper(
		&input,
		&output,
		func(vjoy.SpecObj) (vjoy.Interface, error) {
			creates.Add(1)
			return newCaptureDevice(), nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if creates.Load() != 0 {
		t.Fatal("helper created a device for an oversized specification")
	}
	if responseErr := readResponse(bufio.NewReader(&output)); responseErr == nil {
		t.Fatal("helper accepted an oversized specification")
	}
}

func TestHelperProtocolRejectsOversizedFrame(t *testing.T) {
	spec := rc.DefaultMapping().DeviceSpec(rc.DefaultProfile())
	var input bytes.Buffer
	writer := bufio.NewWriter(&input)
	if err := writeSpec(writer, spec); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteByte(commandFrame); err != nil {
		t.Fatal(err)
	}
	var countData [2]byte
	binary.LittleEndian.PutUint16(countData[:], maxFrameControls+1)
	if _, err := writer.Write(countData[:]); err != nil {
		t.Fatal(err)
	}
	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}

	device := newCaptureDevice()
	var output bytes.Buffer
	if err := serveHelper(
		&input,
		&output,
		func(vjoy.SpecObj) (vjoy.Interface, error) {
			return device, nil
		},
	); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(&output)
	if err := readResponse(reader); err != nil {
		t.Fatalf("creation response: %v", err)
	}
	frameErr := readResponse(reader)
	if frameErr == nil || !strings.Contains(frameErr.Error(), "maximum") {
		t.Fatalf("oversized frame response = %v", frameErr)
	}
	if got := device.closeCount(); got != 1 {
		t.Fatalf("device closes = %d, want 1", got)
	}
}

func TestHelperProtocolClosesDeviceOnInputEOF(t *testing.T) {
	spec := rc.DefaultMapping().DeviceSpec(rc.DefaultProfile())
	var input bytes.Buffer
	writer := bufio.NewWriter(&input)
	if err := writeSpec(writer, spec); err != nil {
		t.Fatal(err)
	}

	device := newCaptureDevice()
	var output bytes.Buffer
	if err := serveHelper(
		&input,
		&output,
		func(vjoy.SpecObj) (vjoy.Interface, error) {
			return device, nil
		},
	); err != nil {
		t.Fatal(err)
	}
	if err := readResponse(bufio.NewReader(&output)); err != nil {
		t.Fatalf("creation response: %v", err)
	}
	if got := device.closeCount(); got != 1 {
		t.Fatalf("device closes after EOF = %d, want 1", got)
	}
}

func TestHelperLogWriterForwardsStructuredRecords(t *testing.T) {
	var output bytes.Buffer
	logger := zerolog.New(&output).Level(zerolog.TraceLevel)
	writer := newHelperLogWriter(logger)
	writer.setPID(321)

	record := []byte(
		`{"level":"debug","component":"gamepad_helper",` +
			`"controls":8,"message":"Device created"}` + "\n",
	)
	split := len(record) / 2
	if size, err := writer.Write(record[:split]); err != nil || size != split {
		t.Fatalf("first Write() = %d, %v", size, err)
	}
	if size, err := writer.Write(record[split:]); err != nil || size != len(record)-split {
		t.Fatalf("second Write() = %d, %v", size, err)
	}

	var forwarded map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &forwarded); err != nil {
		t.Fatalf("parse forwarded record: %v", err)
	}
	if forwarded["level"] != "debug" ||
		forwarded["source"] != "gamepad_helper" ||
		forwarded["helper_pid"] != float64(321) ||
		forwarded["message"] != "Device created" {
		t.Fatalf("forwarded record = %#v", forwarded)
	}
	fields, ok := forwarded["helper_fields"].(map[string]any)
	if !ok || fields["controls"] != float64(8) {
		t.Fatalf("forwarded helper fields = %#v", forwarded["helper_fields"])
	}
	if tail := writer.String(); tail != "" {
		t.Fatalf("structured helper record was duplicated in diagnostic tail: %q", tail)
	}
}

func TestHelperLogWriterBoundsMalformedOutput(t *testing.T) {
	var output bytes.Buffer
	logger := zerolog.New(&output).Level(zerolog.TraceLevel)
	writer := newHelperLogWriter(logger)

	longLine := strings.Repeat("x", maxHelperLogLineBytes+1) + "\n"
	if size, err := writer.Write([]byte(longLine)); err != nil || size != len(longLine) {
		t.Fatalf("Write() = %d, %v", size, err)
	}
	if _, err := writer.Write([]byte("incomplete")); err != nil {
		t.Fatal(err)
	}
	writer.Flush()

	logText := output.String()
	if !strings.Contains(logText, `"reason":"line_too_long"`) ||
		!strings.Contains(logText, `"reason":"incomplete_line"`) {
		t.Fatalf("bounded warnings are missing: %s", logText)
	}
	if len(writer.String()) > maxHelperStderrBytes {
		t.Fatalf("diagnostic tail = %d bytes, maximum %d", len(writer.String()), maxHelperStderrBytes)
	}
}
