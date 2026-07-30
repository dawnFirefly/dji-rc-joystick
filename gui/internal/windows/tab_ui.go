package windows

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	targetsettings "github.com/voluminor/dji-rc-joystick/target/settings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// // // // // // // // // //

const (
	minTextSize = 0.2
	maxTextSize = 8.0
)

func parseTextSize(value string) (float32, error) {
	number, err := strconv.ParseFloat(strings.TrimSpace(value), 32)
	if err != nil {
		return 0, errors.New("enter a valid number")
	}
	if number < minTextSize || number > maxTextSize {
		return 0, fmt.Errorf("value must be between %.1f and %.1f", minTextSize, maxTextSize)
	}

	return float32(number), nil
}

func formatTextSize(value float32) string {
	return strconv.FormatFloat(float64(value), 'f', 1, 32)
}

func (obj *Obj) applyTheme(ui targetsettings.UiObj) {
	obj.theme.SetDark(ui.DarkMode)
	obj.theme.SetTextScale(ui.TextSize)
	obj.application.Settings().SetTheme(obj.theme)
}

func (obj *Obj) renderUI() fyne.CanvasObject {
	active := obj.data.UI()
	draft := active
	valid := true
	updating := false

	saveButton := widget.NewButtonWithIcon(obj.lg("button.save", "Save"), theme.ConfirmIcon(), nil)
	resetButton := widget.NewButtonWithIcon(obj.lg("button.reset", "Reset"), theme.ContentUndoIcon(), nil)
	saveButton.Importance = widget.HighImportance

	updateButtons := func() {
		dirty := draft != active
		if dirty && valid {
			saveButton.Enable()
		} else {
			saveButton.Disable()
		}
		if dirty {
			resetButton.Enable()
		} else {
			resetButton.Disable()
		}
	}

	// //

	lightName := obj.lg("ui.theme.light", "Light")
	darkName := obj.lg("ui.theme.dark", "Dark")
	themeSelect := widget.NewRadioGroup([]string{lightName, darkName}, func(value string) {
		if updating {
			return
		}
		draft.DarkMode = value == darkName
		obj.log.Debug().
			Bool("dark_mode", draft.DarkMode).
			Msg("GUI theme setting changed")
		updateButtons()
	})
	themeSelect.Horizontal = true
	themeSelect.Required = true

	// //

	languageOptions := obj.data.Language().List()
	languageNames := make([]string, 0, len(languageOptions))
	languageByName := make(map[string]string, len(languageOptions))
	nameByLanguage := make(map[string]string, len(languageOptions))
	for _, option := range languageOptions {
		languageNames = append(languageNames, option.Name)
		languageByName[option.Name] = option.Key
		nameByLanguage[option.Key] = option.Name
	}
	languageSelect := widget.NewSelect(languageNames, func(value string) {
		if updating {
			return
		}

		key, ok := languageByName[value]
		if !ok {
			valid = false
			updateButtons()
			return
		}

		valid = true
		draft.Language = key
		obj.log.Debug().
			Str("language", key).
			Msg("GUI language setting changed")
		updateButtons()
	})

	// //

	sizeSlider := widget.NewSlider(minTextSize, maxTextSize)
	sizeSlider.Step = 0.1
	sizeEntry := widget.NewEntry()
	sizeEntry.SetPlaceHolder(obj.lg("ui.text_size.placeholder", "Number from 0.2 to 8.0"))
	sizeEntry.Validator = func(value string) error {
		_, err := parseTextSize(value)
		return err
	}
	sizeEntry.AlwaysShowValidationError = true

	sizeSlider.OnChanged = func(value float64) {
		if updating {
			return
		}

		updating = true
		draft.TextSize = float32(value)
		valid = true
		sizeEntry.SetText(formatTextSize(draft.TextSize))
		updating = false
		obj.log.Debug().
			Float32("text_size", draft.TextSize).
			Str("source", "slider").
			Msg("GUI text-size setting changed")
		updateButtons()
	}
	sizeEntry.OnChanged = func(value string) {
		if updating {
			return
		}

		textSize, err := parseTextSize(value)
		valid = err == nil
		if err == nil {
			draft.TextSize = textSize
			updating = true
			sizeSlider.SetValue(float64(textSize))
			updating = false
		}
		event := obj.log.Debug().
			Bool("valid", valid).
			Int("length", len(value)).
			Str("source", "entry")
		if valid {
			event = event.Float32("text_size", draft.TextSize)
		}
		event.Msg("GUI text-size setting changed")
		updateButtons()
	}

	// //

	resetControls := func() {
		updating = true
		draft = active
		valid = true
		if active.DarkMode {
			themeSelect.SetSelected(darkName)
		} else {
			themeSelect.SetSelected(lightName)
		}
		languageSelect.SetSelected(nameByLanguage[active.Language])
		sizeSlider.SetValue(float64(active.TextSize))
		sizeEntry.SetText(formatTextSize(active.TextSize))
		updating = false
		updateButtons()
	}

	// //

	resetButton.OnTapped = func() {
		obj.log.Debug().Msg("GUI settings reset button activated")
		resetControls()
	}
	saveButton.OnTapped = func() {
		obj.log.Debug().Msg("GUI settings save button activated")
		if !valid || draft == active {
			return
		}

		if err := obj.data.SaveUI(draft); err != nil {
			obj.logError("save UI settings", err)
			dialog.ShowError(err, obj.window)
			return
		}

		selected := obj.tabs.SelectedIndex()
		active = draft
		obj.applyTheme(active)
		obj.refreshTabs(selected)
		obj.log.Info().
			Bool("dark_mode", active.DarkMode).
			Str("language", active.Language).
			Float32("text_size", active.TextSize).
			Msg("UI settings saved")
		obj.log.Debug().
			Str("dialog", "settings_saved").
			Msg("GUI information dialog opened")
		dialog.ShowInformation(
			obj.lg("dialog.settings.saved.title", "Settings"),
			obj.lg("dialog.settings.saved.message", "Settings saved."),
			obj.window,
		)
	}

	resetControls()

	// //

	form := widget.NewForm(
		widget.NewFormItem(obj.lg("ui.theme", "Theme:"), themeSelect),
		widget.NewFormItem(obj.lg("ui.language", "Language:"), languageSelect),
		widget.NewFormItem(obj.lg("ui.text_size", "Text size:"), sizeEntry),
	)
	content := container.NewVBox(
		form,
		sizeSlider,
		widget.NewSeparator(),
		container.NewHBox(saveButton, resetButton),
	)

	return container.NewVScroll(content)
}
