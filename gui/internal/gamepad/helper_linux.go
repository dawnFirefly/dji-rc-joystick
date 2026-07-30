//go:build linux

package gamepad

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/voluminor/dji-rc-joystick/mod/vjoy"

	"github.com/rs/zerolog"
)

// // // // // // // // // //

type helperCreateFunc func(vjoy.SpecObj) (vjoy.Interface, error)

func helperLogger(outputs []io.Writer) zerolog.Logger {
	output := io.Discard
	if len(outputs) > 0 && outputs[0] != nil {
		output = outputs[0]
	}
	return zerolog.New(output).
		Level(zerolog.TraceLevel).
		With().
		Str("component", "gamepad_helper").
		Int("pid", os.Getpid()).
		Timestamp().
		Logger()
}

func helperErrorText(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	if len(text) > maxErrorBytes {
		return text[:maxErrorBytes]
	}
	return text
}

func readFrame(reader *bufio.Reader) ([]frameValueObj, error) {
	var countData [2]byte
	if _, err := io.ReadFull(reader, countData[:]); err != nil {
		return nil, fmt.Errorf("read gamepad frame size: %w", err)
	}
	count := int(binary.LittleEndian.Uint16(countData[:]))
	if count > maxFrameControls {
		return nil, fmt.Errorf(
			"gamepad frame contains %d controls, maximum is %d",
			count,
			maxFrameControls,
		)
	}

	values := make([]frameValueObj, count)
	var valueData [6]byte
	for index := range values {
		if _, err := io.ReadFull(reader, valueData[:]); err != nil {
			return nil, fmt.Errorf("read gamepad frame value %d: %w", index, err)
		}
		values[index] = frameValueObj{
			handle: binary.LittleEndian.Uint16(valueData[:2]),
			value:  int32(binary.LittleEndian.Uint32(valueData[2:])),
		}
	}
	return values, nil
}

func applyFrame(device vjoy.Interface, values []frameValueObj) error {
	for _, value := range values {
		if err := device.Set(value.handle, value.value); err != nil {
			return err
		}
	}
	return device.Sync()
}

func serveHelper(
	input io.Reader,
	output io.Writer,
	create helperCreateFunc,
	loggers ...zerolog.Logger,
) (resultErr error) {
	logger := zerolog.Nop()
	if len(loggers) > 0 {
		logger = loggers[0]
	}
	reader := bufio.NewReader(input)
	writer := bufio.NewWriter(output)

	spec, err := readSpec(reader)
	if err != nil {
		logger.Error().
			Str("error", helperErrorText(err)).
			Msg("Privileged helper rejected gamepad specification")
		_ = writeResponse(writer, err)
		return nil
	}
	logger.Debug().
		Str("name", spec.Name).
		Int("controls", len(spec.Controls)).
		Uint16("vendor_id", spec.VendorID).
		Uint16("product_id", spec.ProductID).
		Msg("Privileged helper received gamepad specification")
	device, err := create(spec)
	if writeErr := writeResponse(writer, err); writeErr != nil {
		if device != nil {
			_ = device.Close()
		}
		return fmt.Errorf("write gamepad creation response: %w", writeErr)
	}
	if err != nil {
		logger.Error().
			Str("error", helperErrorText(err)).
			Msg("Privileged helper could not create virtual gamepad")
		if device != nil {
			_ = device.Close()
		}
		return nil
	}
	defer func() {
		if device != nil {
			if err = device.Close(); resultErr == nil && err != nil {
				resultErr = fmt.Errorf("close virtual gamepad: %w", err)
				logger.Error().
					Str("error", helperErrorText(resultErr)).
					Msg("Privileged helper could not close virtual gamepad")
			}
		}
	}()
	logger.Info().Msg("Privileged helper created virtual gamepad")

	for {
		command, readErr := reader.ReadByte()
		if errors.Is(readErr, io.EOF) {
			return nil
		}
		if readErr != nil {
			return fmt.Errorf("read gamepad helper command: %w", readErr)
		}

		switch command {
		case commandFrame:
			values, frameErr := readFrame(reader)
			if frameErr == nil {
				frameErr = applyFrame(device, values)
			}
			if writeErr := writeResponse(writer, frameErr); writeErr != nil {
				return fmt.Errorf("write gamepad frame response: %w", writeErr)
			}
			if frameErr != nil {
				logger.Error().
					Str("error", helperErrorText(frameErr)).
					Msg("Privileged helper rejected gamepad frame")
				return nil
			}

		case commandClose:
			logger.Debug().Msg("Privileged helper received close command")
			closeErr := device.Close()
			device = nil
			if writeErr := writeResponse(writer, closeErr); writeErr != nil {
				return fmt.Errorf("write gamepad close response: %w", writeErr)
			}
			if closeErr != nil {
				logger.Error().
					Str("error", helperErrorText(closeErr)).
					Msg("Privileged helper could not close virtual gamepad")
			}
			return nil

		default:
			protocolErr := fmt.Errorf(
				"unknown gamepad helper command %d",
				command,
			)
			logger.Error().
				Str("error", helperErrorText(protocolErr)).
				Msg("Privileged helper rejected protocol command")
			_ = writeResponse(writer, protocolErr)
			return nil
		}
	}
}

// //

// RunHelper serves the bounded privileged protocol when its exact mode is set.
func RunHelper(
	args []string,
	input io.Reader,
	output io.Writer,
	logOutputs ...io.Writer,
) (bool, error) {
	if len(args) != 1 || args[0] != helperArgument {
		return false, nil
	}
	logger := helperLogger(logOutputs)
	logger.Debug().Msg("Privileged gamepad helper entered service mode")
	if os.Geteuid() != 0 {
		err := errors.New("virtual gamepad helper requires root")
		logger.Error().
			Str("error", helperErrorText(err)).
			Msg("Privileged gamepad helper rejected startup")
		return true, err
	}
	err := serveHelper(input, output, vjoy.New, logger)
	if err != nil {
		logger.Error().
			Str("error", helperErrorText(err)).
			Msg("Privileged gamepad helper failed")
	}
	logger.Debug().Msg("Privileged gamepad helper exited service mode")
	return true, err
}
