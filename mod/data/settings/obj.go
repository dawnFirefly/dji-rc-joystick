package settings

import (
	"errors"
	"strings"
	"sync"

	targetsettings "github.com/voluminor/dji-rc-joystick/target/settings"
)

// // // // // // // // // //

const (
	fileName     = "settings.json"
	maxFileBytes = 1 << 20
)

// Obj owns a validated configuration snapshot and its persistence path.
type Obj struct {
	path   string
	config targetsettings.ConfigObj

	mx sync.RWMutex
}

// SnapshotObj is an independent, mutable settings value.
type SnapshotObj struct {
	// Logging contains console and file sink settings.
	Logging targetsettings.LoggingObj
	// UI contains appearance and language settings.
	UI targetsettings.UiObj
	// Mapping contains DJI-control bindings.
	Mapping targetsettings.MappingObj
}

func validateLogging(logging targetsettings.LoggingObj) error {
	if !logging.Console.Enabled && !logging.File.Enabled {
		return errors.New("at least one logging output must be enabled")
	}
	if strings.TrimSpace(logging.File.Dir) == "" {
		return errors.New("log directory must not be empty")
	}

	return nil
}

func configFromSnapshot(snapshot SnapshotObj) targetsettings.ConfigObj {
	config := targetsettings.New()
	config.Logging = snapshot.Logging
	config.Ui = snapshot.UI
	config.Mapping = snapshot.Mapping
	return config
}

// Path returns the resolved configuration file path.
func (obj *Obj) Path() string {
	return obj.path
}

// Snapshot returns an independent copy of the active settings.
func (obj *Obj) Snapshot() SnapshotObj {
	obj.mx.RLock()
	defer obj.mx.RUnlock()

	return SnapshotObj{
		Logging: obj.config.Logging,
		UI:      obj.config.Ui,
		Mapping: obj.config.Mapping,
	}
}

// Save validates and atomically persists snapshot before making it active.
func (obj *Obj) Save(snapshot SnapshotObj) error {
	config := configFromSnapshot(snapshot)
	if err := validateLogging(config.Logging); err != nil {
		return err
	}
	if err := config.Validate(); err != nil {
		return err
	}

	obj.mx.Lock()
	defer obj.mx.Unlock()

	if err := saveFile(obj.path, &config); err != nil {
		return err
	}

	obj.config = config
	return nil
}
