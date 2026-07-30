//go:build linux

package gamepad

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/voluminor/dji-rc-joystick/mod/vjoy"

	"github.com/rs/zerolog"
)

// // // // // // // // // //

const (
	helperArgument = "--virtual-gamepad-helper"

	maxSpecBytes     = 64 << 10
	maxFrameControls = 256
	maxErrorBytes    = 4 << 10
	helperExitWait   = 2 * time.Second

	commandFrame byte = 1
	commandClose byte = 2

	responseOK    byte = 0
	responseError byte = 1
)

type frameValueObj struct {
	handle uint16
	value  int32
}

type elevatedDeviceObj struct {
	command       *exec.Cmd
	processCancel context.CancelFunc
	input         *os.File
	output        *os.File
	writer        *bufio.Writer
	reader        *bufio.Reader
	pending       []frameValueObj
	log           zerolog.Logger
	logWriter     *helperLogWriterObj
	closed        bool
}

// //

func writeSpec(writer *bufio.Writer, spec vjoy.SpecObj) error {
	data, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("encode gamepad specification: %w", err)
	}
	if len(data) > maxSpecBytes {
		return fmt.Errorf("gamepad specification exceeds %d bytes", maxSpecBytes)
	}

	var size [4]byte
	binary.LittleEndian.PutUint32(size[:], uint32(len(data)))
	if _, err = writer.Write(size[:]); err != nil {
		return fmt.Errorf("write gamepad specification size: %w", err)
	}
	if _, err = writer.Write(data); err != nil {
		return fmt.Errorf("write gamepad specification: %w", err)
	}
	if err = writer.Flush(); err != nil {
		return fmt.Errorf("flush gamepad specification: %w", err)
	}
	return nil
}

func readSpec(reader *bufio.Reader) (vjoy.SpecObj, error) {
	var size [4]byte
	if _, err := io.ReadFull(reader, size[:]); err != nil {
		return vjoy.SpecObj{}, fmt.Errorf("read gamepad specification size: %w", err)
	}
	length := binary.LittleEndian.Uint32(size[:])
	if length == 0 || length > maxSpecBytes {
		return vjoy.SpecObj{}, fmt.Errorf(
			"gamepad specification size %d is outside 1..%d",
			length,
			maxSpecBytes,
		)
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(reader, data); err != nil {
		return vjoy.SpecObj{}, fmt.Errorf("read gamepad specification: %w", err)
	}

	var spec vjoy.SpecObj
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&spec); err != nil {
		return vjoy.SpecObj{}, fmt.Errorf("decode gamepad specification: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return vjoy.SpecObj{}, errors.New(
			"gamepad specification contains trailing data",
		)
	}
	return spec, nil
}

func writeResponse(writer *bufio.Writer, err error) error {
	if err == nil {
		if writeErr := writer.WriteByte(responseOK); writeErr != nil {
			return writeErr
		}
		return writer.Flush()
	}

	message := []byte(err.Error())
	if len(message) > maxErrorBytes {
		message = message[:maxErrorBytes]
	}
	if writeErr := writer.WriteByte(responseError); writeErr != nil {
		return writeErr
	}
	var size [2]byte
	binary.LittleEndian.PutUint16(size[:], uint16(len(message)))
	if _, writeErr := writer.Write(size[:]); writeErr != nil {
		return writeErr
	}
	if _, writeErr := writer.Write(message); writeErr != nil {
		return writeErr
	}
	return writer.Flush()
}

func readResponse(reader *bufio.Reader) error {
	status, err := reader.ReadByte()
	if err != nil {
		return fmt.Errorf("read helper response: %w", err)
	}
	switch status {
	case responseOK:
		return nil
	case responseError:
		var size [2]byte
		if _, err = io.ReadFull(reader, size[:]); err != nil {
			return fmt.Errorf("read helper error size: %w", err)
		}
		length := int(binary.LittleEndian.Uint16(size[:]))
		if length > maxErrorBytes {
			return fmt.Errorf("helper error exceeds %d bytes", maxErrorBytes)
		}
		message := make([]byte, length)
		if _, err = io.ReadFull(reader, message); err != nil {
			return fmt.Errorf("read helper error: %w", err)
		}
		return errors.New(string(message))
	default:
		return fmt.Errorf("helper returned unknown status %d", status)
	}
}

func (obj *elevatedDeviceObj) wait() error {
	waitC := make(chan error, 1)
	go func() {
		waitC <- obj.command.Wait()
	}()

	select {
	case err := <-waitC:
		obj.processCancel()
		obj.logWriter.Flush()
		return err
	case <-time.After(helperExitWait):
		obj.processCancel()
		select {
		case err := <-waitC:
			obj.logWriter.Flush()
			return err
		case <-time.After(helperExitWait):
			obj.logWriter.Flush()
			return errors.New("gamepad helper did not exit")
		}
	}
}

func (obj *elevatedDeviceObj) setProtocolDeadline() error {
	deadline := time.Now().Add(helperExitWait)
	if err := obj.input.SetWriteDeadline(deadline); err != nil {
		return fmt.Errorf("set gamepad helper write deadline: %w", err)
	}
	if err := obj.output.SetReadDeadline(deadline); err != nil {
		return fmt.Errorf("set gamepad helper read deadline: %w", err)
	}
	return nil
}

// //

func (obj *elevatedDeviceObj) Set(handle uint16, value int32) error {
	if obj.closed {
		return errors.New("gamepad helper is closed")
	}
	if len(obj.pending) >= maxFrameControls {
		return fmt.Errorf("gamepad frame exceeds %d controls", maxFrameControls)
	}
	obj.pending = append(obj.pending, frameValueObj{handle: handle, value: value})
	return nil
}

func (obj *elevatedDeviceObj) Sync() error {
	if obj.closed {
		return errors.New("gamepad helper is closed")
	}
	if err := obj.setProtocolDeadline(); err != nil {
		return err
	}
	if err := obj.writer.WriteByte(commandFrame); err != nil {
		return fmt.Errorf("write gamepad frame command: %w", err)
	}

	var header [2]byte
	binary.LittleEndian.PutUint16(header[:], uint16(len(obj.pending)))
	if _, err := obj.writer.Write(header[:]); err != nil {
		return fmt.Errorf("write gamepad frame size: %w", err)
	}
	var valueData [6]byte
	for _, value := range obj.pending {
		binary.LittleEndian.PutUint16(valueData[:2], value.handle)
		binary.LittleEndian.PutUint32(valueData[2:], uint32(value.value))
		if _, err := obj.writer.Write(valueData[:]); err != nil {
			return fmt.Errorf("write gamepad frame value: %w", err)
		}
	}
	obj.pending = obj.pending[:0]
	if err := obj.writer.Flush(); err != nil {
		return fmt.Errorf("flush gamepad frame: %w", err)
	}
	return readResponse(obj.reader)
}

func (obj *elevatedDeviceObj) Close() error {
	if obj.closed {
		return nil
	}
	obj.closed = true
	obj.pending = nil

	var resultErr error
	if err := obj.setProtocolDeadline(); err != nil {
		resultErr = err
	} else if err = obj.writer.WriteByte(commandClose); err == nil {
		if err = obj.writer.Flush(); err == nil {
			resultErr = readResponse(obj.reader)
		} else {
			resultErr = fmt.Errorf("flush gamepad close command: %w", err)
		}
	} else {
		resultErr = fmt.Errorf("write gamepad close command: %w", err)
	}
	if err := obj.input.Close(); resultErr == nil && err != nil {
		resultErr = fmt.Errorf("close gamepad helper input: %w", err)
	}
	if err := obj.wait(); resultErr == nil && err != nil {
		resultErr = fmt.Errorf("wait for gamepad helper: %w", err)
	}
	event := obj.log.Debug().
		Int("helper_pid", obj.command.Process.Pid)
	if resultErr != nil {
		event = obj.log.Error().
			Int("helper_pid", obj.command.Process.Pid).
			Str("error", helperErrorText(resultErr))
	}
	event.Msg("Privileged gamepad helper stopped")
	return resultErr
}
