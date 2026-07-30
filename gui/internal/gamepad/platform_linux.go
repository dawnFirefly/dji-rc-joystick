//go:build linux

package gamepad

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/voluminor/dji-rc-joystick/mod/vjoy"

	"github.com/rs/zerolog"
)

// // // // // // // // // //

const maxHelperStderrBytes = 16 << 10

var pkexecPaths = [...]string{"/usr/bin/pkexec", "/bin/pkexec"}

type stringerInterface interface {
	String() string
}

func findPkexec() (string, error) {
	for _, path := range pkexecPaths {
		info, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return "", fmt.Errorf("inspect Polkit executable %s: %w", path, err)
		}
		if !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
			continue
		}
		return path, nil
	}
	return "", errors.New(
		"pkexec is not installed; grant write access to /dev/uinput with udev",
	)
}

func executablePath() (string, error) {
	path, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve application executable: %w", err)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve application executable links: %w", err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve absolute application executable: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("inspect application executable: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("application executable %q is not a regular file", path)
	}
	return path, nil
}

func authorizationCanceled(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 126
}

func elevatedStartError(
	ctx context.Context,
	responseErr error,
	waitErr error,
	stderr stringerInterface,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if authorizationCanceled(waitErr) {
		return ErrAuthorizationCanceled
	}
	message := stderr.String()
	switch {
	case waitErr == nil:
		return responseErr
	case message != "":
		return fmt.Errorf(
			"authorized gamepad helper failed: %w: %s",
			waitErr,
			message,
		)
	default:
		return fmt.Errorf(
			"authorized gamepad helper failed: %w (protocol: %v)",
			waitErr,
			responseErr,
		)
	}
}

func newElevatedDevice(
	ctx context.Context,
	spec vjoy.SpecObj,
	logger zerolog.Logger,
) (vjoy.Interface, error) {
	pkexec, err := findPkexec()
	if err != nil {
		return nil, err
	}
	executable, err := executablePath()
	if err != nil {
		return nil, err
	}

	processCtx, processCancel := context.WithCancel(context.Background())
	command := exec.CommandContext(
		processCtx,
		pkexec,
		"--disable-internal-agent",
		executable,
		helperArgument,
	)
	command.WaitDelay = helperExitWait

	inputPipe, err := command.StdinPipe()
	if err != nil {
		processCancel()
		return nil, fmt.Errorf("open gamepad helper input: %w", err)
	}
	input, ok := inputPipe.(*os.File)
	if !ok {
		_ = inputPipe.Close()
		processCancel()
		return nil, errors.New("gamepad helper input is not an OS pipe")
	}
	outputPipe, err := command.StdoutPipe()
	if err != nil {
		_ = input.Close()
		processCancel()
		return nil, fmt.Errorf("open gamepad helper output: %w", err)
	}
	output, ok := outputPipe.(*os.File)
	if !ok {
		_ = input.Close()
		_ = outputPipe.Close()
		processCancel()
		return nil, errors.New("gamepad helper output is not an OS pipe")
	}
	stderr := newHelperLogWriter(logger)
	command.Stderr = stderr

	logger.Info().
		Str("authorization", "polkit").
		Msg("Virtual gamepad authorization requested")
	if err = command.Start(); err != nil {
		_ = input.Close()
		_ = output.Close()
		processCancel()
		return nil, fmt.Errorf("start Polkit authorization: %w", err)
	}
	stderr.setPID(command.Process.Pid)
	logger.Debug().
		Int("helper_pid", command.Process.Pid).
		Msg("Privileged gamepad helper process started")

	device := &elevatedDeviceObj{
		command:       command,
		processCancel: processCancel,
		input:         input,
		output:        output,
		writer:        bufio.NewWriter(input),
		reader:        bufio.NewReader(output),
		pending:       make([]frameValueObj, 0, 16),
		log:           logger,
		logWriter:     stderr,
	}

	authenticationDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = input.Close()
			processCancel()
		case <-authenticationDone:
		}
	}()

	if err = writeSpec(device.writer, spec); err == nil {
		err = readResponse(device.reader)
	}
	close(authenticationDone)
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		logger.Info().
			Int("helper_pid", command.Process.Pid).
			Msg("Virtual gamepad authorization completed")
		return device, nil
	}

	_ = input.Close()
	waitErr := device.wait()
	startErr := elevatedStartError(ctx, err, waitErr, stderr)
	if errors.Is(startErr, ErrAuthorizationCanceled) {
		logger.Warn().Msg("Virtual gamepad authorization canceled")
	} else if startErr != nil && !errors.Is(startErr, context.Canceled) {
		logger.Error().
			Str("error", helperErrorText(startErr)).
			Msg("Privileged gamepad helper startup failed")
	}
	return nil, startErr
}

func createDevice(
	ctx context.Context,
	spec vjoy.SpecObj,
	logger zerolog.Logger,
) (vjoy.Interface, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	device, err := vjoy.New(spec)
	if err == nil {
		logger.Info().
			Str("access", "direct").
			Msg("Virtual gamepad device created")
		return device, nil
	}
	if !errors.Is(err, os.ErrPermission) || os.Geteuid() == 0 {
		return nil, err
	}
	logger.Warn().
		Str("access", "polkit").
		Msg("Direct virtual gamepad access was denied")
	return newElevatedDevice(ctx, spec, logger)
}
