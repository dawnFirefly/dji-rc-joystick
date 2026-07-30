package logging

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/voluminor/dji-rc-joystick/target"
	targetsettings "github.com/voluminor/dji-rc-joystick/target/settings"
)

// // // // // // // // // //

var timeFormatOnce sync.Once

// Obj owns the configured logging outputs.
type Obj struct {
	logger   zerolog.Logger
	filePath string
	closers  []io.Closer

	redirectMu sync.Mutex
	standard   *standardRedirectObj
	closed     bool

	closeOnce sync.Once
	closeErr  error
}

func configureTimeFormat() {
	timeFormatOnce.Do(func() {
		zerolog.TimeFieldFormat = time.RFC3339Nano
	})
}

func lowestLevel(current zerolog.Level, candidate zerolog.Level) zerolog.Level {
	if candidate < current {
		return candidate
	}

	return current
}

func newWithConsole(
	config targetsettings.LoggingObj,
	configPath string,
	consoleOutput io.Writer,
) (*Obj, error) {
	if !config.Console.Enabled && !config.File.Enabled {
		return nil, errors.New("at least one logging output must be enabled")
	}
	if config.Console.Enabled && consoleOutput == nil {
		return nil, errors.New("console output must not be nil")
	}

	configureTimeFormat()

	writers := make([]io.Writer, 0, 2)
	closers := make([]io.Closer, 0, 1)
	level := zerolog.Disabled
	filePath := ""

	if config.Console.Enabled {
		sink, err := consoleSink(config.Console, consoleOutput)
		if err != nil {
			return nil, fmt.Errorf("configure console logging: %w", err)
		}
		writers = append(writers, sink.writer)
		level = lowestLevel(level, sink.level)
	}

	if config.File.Enabled {
		sink, err := fileSink(config.File, configPath)
		if err != nil {
			return nil, fmt.Errorf("configure file logging: %w", err)
		}
		writers = append(writers, sink.writer)
		closers = append(closers, sink.closer)
		level = lowestLevel(level, sink.level)
		filePath = sink.path
	}

	writer := zerolog.SyncWriter(zerolog.MultiLevelWriter(writers...))
	logger := zerolog.New(writer).
		Level(level).
		With().
		Timestamp().
		Str("service", target.Name).
		Logger()

	return &Obj{
		logger:   logger,
		filePath: filePath,
		closers:  closers,
	}, nil
}

// New configures independent console and rotating-file logging outputs.
func New(config targetsettings.LoggingObj, configPath string) (*Obj, error) {
	return newWithConsole(config, configPath, os.Stderr)
}

// Logger returns the configured structured logger.
func (obj *Obj) Logger() zerolog.Logger {
	return obj.logger
}

// FilePath returns the active log file path, or an empty string when disabled.
func (obj *Obj) FilePath() string {
	return obj.filePath
}

// Close flushes and closes owned logging outputs.
func (obj *Obj) Close() error {
	obj.closeOnce.Do(func() {
		obj.detachStandard()

		errorArr := make([]error, 0, len(obj.closers))
		for _, closer := range obj.closers {
			if err := closer.Close(); err != nil {
				errorArr = append(errorArr, err)
			}
		}
		obj.closeErr = errors.Join(errorArr...)
	})

	return obj.closeErr
}
