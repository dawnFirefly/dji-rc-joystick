package data

import (
	"fmt"
	"sync"

	"github.com/voluminor/dji-rc-joystick/mod/data/language"
	datasettings "github.com/voluminor/dji-rc-joystick/mod/data/settings"
	targetsettings "github.com/voluminor/dji-rc-joystick/target/settings"
)

// // // // // // // // // //

// Obj coordinates persistent settings and the active language.
type Obj struct {
	language *language.Obj
	settings *datasettings.Obj
	saveMu   sync.Mutex
}

// New loads settings from args and initializes the configured language.
func New(args []string) (*Obj, error) {
	settingsObj, err := datasettings.New(args)
	if err != nil {
		return nil, err
	}

	config := settingsObj.Snapshot()
	languageObj, err := language.New(config.UI.Language)
	if err != nil {
		return nil, fmt.Errorf("initialize language: %w", err)
	}

	return &Obj{
		language: languageObj,
		settings: settingsObj,
	}, nil
}

// Language returns the facade's active translation service.
func (obj *Obj) Language() *language.Obj {
	return obj.language
}

// ConfigPath returns the active settings file path.
func (obj *Obj) ConfigPath() string {
	return obj.settings.Path()
}

// Logging returns an independent snapshot of the active logging settings.
func (obj *Obj) Logging() targetsettings.LoggingObj {
	return obj.settings.Snapshot().Logging
}

// UI returns an independent snapshot of the active UI settings.
func (obj *Obj) UI() targetsettings.UiObj {
	return obj.settings.Snapshot().UI
}

// Mapping returns an independent snapshot of the active control bindings.
func (obj *Obj) Mapping() targetsettings.MappingObj {
	return obj.settings.Snapshot().Mapping
}

// SaveUI validates, persists, and activates a complete UI settings snapshot.
func (obj *Obj) SaveUI(ui targetsettings.UiObj) error {
	obj.saveMu.Lock()
	defer obj.saveMu.Unlock()

	if !obj.language.Supports(ui.Language) {
		return fmt.Errorf("unsupported language: %s", ui.Language)
	}

	config := obj.settings.Snapshot()
	config.UI = ui
	if err := obj.settings.Save(config); err != nil {
		return err
	}
	if err := obj.language.Set(ui.Language); err != nil {
		return fmt.Errorf("apply language: %w", err)
	}

	return nil
}

// SaveMapping validates, persists, and activates complete control bindings.
func (obj *Obj) SaveMapping(mapping targetsettings.MappingObj) error {
	if err := datasettings.ControlMapping(mapping).Validate(); err != nil {
		return fmt.Errorf("validate control mapping: %w", err)
	}

	obj.saveMu.Lock()
	defer obj.saveMu.Unlock()

	config := obj.settings.Snapshot()
	config.Mapping = mapping
	return obj.settings.Save(config)
}
