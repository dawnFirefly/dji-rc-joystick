package settings

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/voluminor/dji-rc-joystick/target"
	targetsettings "github.com/voluminor/dji-rc-joystick/target/settings"
)

// // // // // // // // // //

type argumentsObj struct {
	configPath string
	configSet  bool
	filtered   []string
}

func parseArguments(args []string) (argumentsObj, error) {
	result := argumentsObj{
		filtered: make([]string, 0, len(args)),
	}

	for index := 0; index < len(args); index++ {
		arg := strings.TrimSpace(args[index])
		if arg == "--" {
			result.filtered = append(result.filtered, args[index:]...)
			break
		}

		if arg == "-config" || arg == "--config" {
			if result.configSet {
				return argumentsObj{}, errors.New("config path is specified more than once")
			}
			if index+1 >= len(args) {
				return argumentsObj{}, errors.New("config flag requires a value")
			}

			index++
			result.configPath = strings.TrimSpace(args[index])
			result.configSet = true
			continue
		}

		if strings.HasPrefix(arg, "-config=") || strings.HasPrefix(arg, "--config=") {
			if result.configSet {
				return argumentsObj{}, errors.New("config path is specified more than once")
			}

			_, value, _ := strings.Cut(arg, "=")
			result.configPath = strings.TrimSpace(value)
			result.configSet = true
			continue
		}

		result.filtered = append(result.filtered, args[index])
	}

	if result.configSet && result.configPath == "" {
		return argumentsObj{}, errors.New("config path is empty")
	}

	return result, nil
}

func defaultPath() (string, error) {
	rootPath, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}

	return filepath.Join(rootPath, target.Name, fileName), nil
}

func loadFile(path string, args []string) (*targetsettings.ConfigObj, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			config := targetsettings.New()
			if applyErr := targetsettings.ApplyCLI(&config, args); applyErr != nil {
				return nil, false, fmt.Errorf("apply settings arguments: %w", applyErr)
			}
			return &config, false, nil
		}
		return nil, false, fmt.Errorf("open config file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	// //

	info, err := file.Stat()
	if err != nil {
		return nil, false, fmt.Errorf("inspect config file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("config path %q is not a regular file", path)
	}
	if info.Size() > maxFileBytes {
		return nil, false, fmt.Errorf("config file exceeds %d bytes", maxFileBytes)
	}

	// //

	data, err := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
	if err != nil {
		return nil, false, fmt.Errorf("read config file: %w", err)
	}
	if len(data) > maxFileBytes {
		return nil, false, fmt.Errorf("config file exceeds %d bytes", maxFileBytes)
	}

	// //

	formatName := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	if formatName == "yml" {
		formatName = "yaml"
	}
	format, ok := targetsettings.FormatByName(formatName)
	if !ok {
		return nil, false, fmt.Errorf("unsupported config file extension: %s", filepath.Ext(path))
	}

	config, err := format.Parse(data)
	if err != nil {
		return nil, false, fmt.Errorf("parse config file: %w", err)
	}
	if err = targetsettings.ApplyCLI(config, args); err != nil {
		return nil, false, fmt.Errorf("apply settings arguments: %w", err)
	}

	return config, true, nil
}

// New loads defaults, an optional config file, and CLI overrides in that order.
func New(args []string) (*Obj, error) {
	parsedArgs, err := parseArguments(args)
	if err != nil {
		return nil, err
	}

	// //

	path := parsedArgs.configPath
	if !parsedArgs.configSet {
		path, err = defaultPath()
		if err != nil {
			return nil, err
		}
	}

	// //

	config, exists, err := loadFile(path, parsedArgs.filtered)
	if err != nil {
		return nil, err
	}
	if err = validateLogging(config.Logging); err != nil {
		return nil, fmt.Errorf("validate logging settings: %w", err)
	}

	obj := &Obj{
		path:   path,
		config: *config,
	}
	if !exists {
		if err = saveFile(path, config); err != nil {
			return nil, err
		}
	}

	return obj, nil
}
