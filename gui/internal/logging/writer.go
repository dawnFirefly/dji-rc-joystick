package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/voluminor/dji-rc-joystick/target"
	targetsettings "github.com/voluminor/dji-rc-joystick/target/settings"
	"gopkg.in/natefinch/lumberjack.v2"
)

// // // // // // // // // //

const (
	decimalMegabyte = 1_000_000
	logFileSuffix   = ".log"
)

type levelWriterObj struct {
	writer zerolog.LevelWriter
	level  zerolog.Level
}

type sinkObj struct {
	writer io.Writer
	closer io.Closer
	level  zerolog.Level
	path   string
}

func (obj levelWriterObj) Write(data []byte) (int, error) {
	return obj.writer.Write(data)
}

func (obj levelWriterObj) WriteLevel(level zerolog.Level, data []byte) (int, error) {
	if level < obj.level {
		return len(data), nil
	}

	return obj.writer.WriteLevel(level, data)
}

func parseLevel(value string) (zerolog.Level, error) {
	level, err := zerolog.ParseLevel(value)
	if err != nil {
		return zerolog.NoLevel, fmt.Errorf("parse log level %q: %w", value, err)
	}

	return level, nil
}

func resolveDirectory(configPath string, directory string) (string, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return "", fmt.Errorf("log directory must not be empty")
	}
	if !filepath.IsAbs(directory) {
		configPath, err := filepath.Abs(configPath)
		if err != nil {
			return "", fmt.Errorf("resolve config path: %w", err)
		}
		directory = filepath.Join(filepath.Dir(configPath), directory)
	}

	resolved, err := filepath.Abs(directory)
	if err != nil {
		return "", fmt.Errorf("resolve log directory: %w", err)
	}

	return filepath.Clean(resolved), nil
}

func ensureDirectory(path string) error {
	info, err := os.Stat(path)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("log directory path %q is not a directory", path)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("inspect log directory: %w", err)
	}

	if err = os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}
	if err = os.Chmod(path, 0o700); err != nil {
		return fmt.Errorf("secure log directory: %w", err)
	}

	return nil
}

func prepareFile(path string) error {
	info, err := os.Lstat(path)
	if err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("log path %q is not a regular file", path)
	}
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect log file: %w", err)
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	if err = file.Chmod(0o600); err != nil {
		return fmt.Errorf("secure log file: %w", err)
	}
	info, err = file.Stat()
	if err != nil {
		return fmt.Errorf("inspect opened log file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("opened log path %q is not a regular file", path)
	}

	return nil
}

func ceilUnits(value uint64, unit uint64) int {
	if value == 0 {
		return 0
	}

	return int(((value - 1) / unit) + 1)
}

func formatConsoleMessage(value any) string {
	if value == nil {
		return ""
	}
	text := fmt.Sprint(value)
	if text == "" {
		return ""
	}
	quoted := strconv.Quote(text)
	return quoted[1 : len(quoted)-1]
}

func consoleSink(config targetsettings.LoggingConsoleObj, output io.Writer) (sinkObj, error) {
	level, err := parseLevel(config.Level.String())
	if err != nil {
		return sinkObj{}, err
	}

	writer := zerolog.NewConsoleWriter(func(obj *zerolog.ConsoleWriter) {
		obj.Out = output
		obj.TimeFormat = time.RFC3339
		obj.NoColor = false
		obj.FormatMessage = formatConsoleMessage
	})

	return sinkObj{
		writer: levelWriterObj{
			writer: zerolog.LevelWriterAdapter{Writer: writer},
			level:  level,
		},
		level: level,
	}, nil
}

func fileSink(config targetsettings.LoggingFileObj, configPath string) (sinkObj, error) {
	level, err := parseLevel(config.Level.String())
	if err != nil {
		return sinkObj{}, err
	}

	directory, err := resolveDirectory(configPath, config.Dir)
	if err != nil {
		return sinkObj{}, err
	}
	if err = ensureDirectory(directory); err != nil {
		return sinkObj{}, err
	}

	path := filepath.Join(directory, target.Name+logFileSuffix)
	if err = prepareFile(path); err != nil {
		return sinkObj{}, err
	}

	writer := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    ceilUnits(uint64(config.MaxSize), decimalMegabyte),
		MaxBackups: int(config.MaxBackups),
		MaxAge:     ceilUnits(uint64(config.MaxAge), uint64(24*time.Hour)),
		Compress:   config.Compress,
	}

	return sinkObj{
		writer: levelWriterObj{
			writer: zerolog.LevelWriterAdapter{Writer: writer},
			level:  level,
		},
		closer: writer,
		level:  level,
		path:   path,
	}, nil
}
