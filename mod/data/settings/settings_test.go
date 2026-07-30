package settings

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/voluminor/dji-rc-joystick/target"
	targetsettings "github.com/voluminor/dji-rc-joystick/target/settings"
)

// // // // // // // // // //

func TestNewCreatesDefaults(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("HOME", configRoot)
	t.Setenv("AppData", configRoot)
	t.Setenv("home", configRoot)

	expectedRoot := configRoot
	switch runtime.GOOS {
	case "darwin", "ios":
		expectedRoot = filepath.Join(configRoot, "Library", "Application Support")
	case "plan9":
		expectedRoot = filepath.Join(configRoot, "lib")
	}

	obj, err := New([]string{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	expectedPath := filepath.Join(expectedRoot, target.Name, fileName)
	if obj.Path() != expectedPath {
		t.Fatalf("Path() = %q, want %q", obj.Path(), expectedPath)
	}

	info, err := os.Stat(expectedPath)
	if err != nil {
		t.Fatalf("stat settings: %v", err)
	}
	directoryInfo, err := os.Stat(filepath.Dir(expectedPath))
	if err != nil {
		t.Fatalf("stat settings directory: %v", err)
	}
	if runtime.GOOS != "windows" {
		if permissions := info.Mode().Perm(); permissions != 0o600 {
			t.Fatalf("settings permissions = %o, want 600", permissions)
		}
		if permissions := directoryInfo.Mode().Perm(); permissions != 0o700 {
			t.Fatalf("settings directory permissions = %o, want 700", permissions)
		}
	}

	config := obj.Snapshot()
	if config.UI.Language != "en" {
		t.Fatalf("language = %q, want en", config.UI.Language)
	}
	if !config.UI.DarkMode {
		t.Fatal("dark mode is disabled by default")
	}
	if config.UI.TextSize != 1 {
		t.Fatalf("text size = %v, want 1", config.UI.TextSize)
	}
	if !config.Logging.Console.Enabled || config.Logging.Console.Level != targetsettings.ConsoleLogLevelInfo {
		t.Fatalf("console logging defaults = %#v", config.Logging.Console)
	}
	if !config.Logging.File.Enabled || config.Logging.File.Level != targetsettings.FileLogLevelError {
		t.Fatalf("file logging defaults = %#v", config.Logging.File)
	}
	if config.Logging.File.Dir != "logs" {
		t.Fatalf("log directory = %q, want logs", config.Logging.File.Dir)
	}
}

func TestNewUsesConfigOverride(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "custom.json")
	data := []byte(`{"ui":{"text_size":1.5,"dark_mode":false,"language":"ru-RU"}}`)
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	obj, err := New([]string{"--config", configPath})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	config := obj.Snapshot()
	if config.UI.Language != "ru-RU" {
		t.Fatalf("language = %q, want ru-RU", config.UI.Language)
	}
	if config.UI.DarkMode {
		t.Fatal("dark mode was not loaded")
	}
	if config.UI.TextSize != 1.5 {
		t.Fatalf("text size = %v, want 1.5", config.UI.TextSize)
	}
}

func TestSnapshotIsIndependent(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "settings.json")
	obj, err := New([]string{"--config", configPath})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	snapshot := obj.Snapshot()
	snapshot.UI.Language = "ru-RU"
	snapshot.Mapping.Axes.LeftStickH.Action = "unmapped"
	snapshot.Logging.File.Dir = "other-logs"

	current := obj.Snapshot()
	if current.UI.Language == snapshot.UI.Language {
		t.Fatal("mutating a snapshot changed the active language")
	}
	if current.Mapping.Axes.LeftStickH.Action == snapshot.Mapping.Axes.LeftStickH.Action {
		t.Fatal("mutating a snapshot changed the active mapping")
	}
	if current.Logging.File.Dir == snapshot.Logging.File.Dir {
		t.Fatal("mutating a snapshot changed the active logging settings")
	}
}

func TestNewRejectsLargeFile(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "large.json")
	if err := os.WriteFile(configPath, make([]byte, maxFileBytes+1), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := New([]string{"--config", configPath}); err == nil {
		t.Fatal("New() accepted an oversized config")
	}
}

func TestSaveUpdatesOnlyAfterSuccessfulReplace(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "settings.json")
	obj, err := New([]string{"--config", configPath})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	next := obj.Snapshot()
	next.UI.Language = "ru-RU"
	next.UI.DarkMode = false
	next.UI.TextSize = 1.7
	if err = obj.Save(next); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := targetsettings.ParseFile(configPath)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}
	if loaded.Ui != next.UI {
		t.Fatalf("saved UI = %#v, want %#v", loaded.Ui, next.UI)
	}
	if loaded.Logging != next.Logging {
		t.Fatalf("saved logging = %#v, want %#v", loaded.Logging, next.Logging)
	}

	active := obj.Snapshot()
	failed := active
	failed.UI.TextSize = 2.3
	if err = os.Remove(configPath); err != nil {
		t.Fatalf("remove config: %v", err)
	}
	if err = os.Mkdir(configPath, 0o700); err != nil {
		t.Fatalf("replace config with directory: %v", err)
	}
	if err = obj.Save(failed); err == nil {
		t.Fatal("Save() succeeded with a directory as the target")
	}
	if current := obj.Snapshot(); current.UI != active.UI {
		t.Fatalf("active UI changed after failed save: %#v", current.UI)
	}
}

func TestSaveRejectsInvalidLoggingSettings(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "settings.json")
	obj, err := New([]string{"--config", configPath})
	if err != nil {
		t.Fatal(err)
	}

	snapshot := obj.Snapshot()
	snapshot.Logging.Console.Enabled = false
	snapshot.Logging.File.Enabled = false
	if err = obj.Save(snapshot); err == nil {
		t.Fatal("Save() accepted disabled logging outputs")
	}

	snapshot = obj.Snapshot()
	snapshot.Logging.File.Dir = " "
	if err = obj.Save(snapshot); err == nil {
		t.Fatal("Save() accepted an empty log directory")
	}
}
