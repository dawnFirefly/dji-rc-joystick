package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	targetsettings "github.com/voluminor/dji-rc-joystick/target/settings"
)

// // // // // // // // // //

func render(config *targetsettings.ConfigObj, path string) ([]byte, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return config.RenderJSON(false)
	case ".yaml", ".yml":
		return config.RenderYAML(false)
	default:
		return nil, fmt.Errorf("unsupported config file extension: %s", filepath.Ext(path))
	}
}

func ensureDirectory(path string) error {
	info, err := os.Stat(path)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("config directory path %q is not a directory", path)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("inspect config directory: %w", err)
	}

	if err = os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err = os.Chmod(path, 0o700); err != nil {
		return fmt.Errorf("secure config directory: %w", err)
	}

	return nil
}

func writeTemporary(file *os.File, data []byte) error {
	if err := file.Chmod(0o600); err != nil {
		return fmt.Errorf("secure temporary config file: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("write temporary config file: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync temporary config file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary config file: %w", err)
	}

	return nil
}

func saveFile(path string, config *targetsettings.ConfigObj) error {
	data, err := render(config, path)
	if err != nil {
		return err
	}
	if len(data) > maxFileBytes {
		return fmt.Errorf("rendered config exceeds %d bytes", maxFileBytes)
	}

	directory := filepath.Dir(path)
	if err = ensureDirectory(directory); err != nil {
		return err
	}

	file, err := os.CreateTemp(directory, ".settings-*")
	if err != nil {
		return fmt.Errorf("create temporary config file: %w", err)
	}
	tempPath := file.Name()
	committed := false
	defer func() {
		if !committed {
			_ = file.Close()
			_ = os.Remove(tempPath)
		}
	}()

	if err = writeTemporary(file, data); err != nil {
		return err
	}
	if err = os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace config file: %w", err)
	}

	committed = true
	return nil
}
