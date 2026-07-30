package windows

import (
	"image/color"
	"testing"

	guiremote "github.com/voluminor/dji-rc-joystick/gui/internal/remote"
	"github.com/voluminor/dji-rc-joystick/mod/rc"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	fynetest "fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

// // // // // // // // // //

func sameColor(first, second color.Color) bool {
	firstR, firstG, firstB, firstA := first.RGBA()
	secondR, secondG, secondB, secondA := second.RGBA()
	return firstR == secondR &&
		firstG == secondG &&
		firstB == secondB &&
		firstA == secondA
}

func renderedButtonColor(button fyne.Widget) color.Color {
	renderer := button.CreateRenderer()
	return renderer.Objects()[0].(*canvas.Rectangle).FillColor
}

// //

func TestHomeControlsMapsStateAndCapabilities(t *testing.T) {
	application := fynetest.NewApp()
	defer application.Quit()
	controls := newHomeControls()
	profile, ok := rc.ProfileByID("rc2")
	if !ok {
		t.Fatal("rc2 profile is missing")
	}

	controls.update(guiremote.SnapshotObj{
		Profile: profile,
		State: rc.StateObj{
			Axes: [rc.AxisCount]int16{
				-100,
				200,
				300,
				-400,
				500,
				-600,
			},
			Buttons: 1<<uint(rc.ButtonCameraFull) |
				1<<uint(rc.ButtonC1),
		},
	})

	if controls.stickValues[leftControl].Text != "X: -100  Y: 200" ||
		controls.stickValues[rightControl].Text != "X: 300  Y: -400" {
		t.Fatalf(
			"stick values = %q / %q",
			controls.stickValues[leftControl].Text,
			controls.stickValues[rightControl].Text,
		)
	}
	if controls.dialValues[leftControl].Text != "500" ||
		controls.dialValues[rightControl].Text != "-600" {
		t.Fatalf(
			"dial values = %q / %q",
			controls.dialValues[leftControl].Text,
			controls.dialValues[rightControl].Text,
		)
	}

	themeObj := controls.buttons[rc.ButtonRTH].Theme()
	variant := fyne.CurrentApp().Settings().ThemeVariant()
	if !sameColor(
		renderedButtonColor(controls.buttons[rc.ButtonRTH]),
		themeObj.Color(theme.ColorNameButton, variant),
	) {
		t.Fatal("released button is not inactive")
	}
	if !sameColor(
		renderedButtonColor(controls.buttons[rc.ButtonCameraFull]),
		themeObj.Color(theme.ColorNameSuccess, variant),
	) {
		t.Fatal("pressed button is not active")
	}
	if !sameColor(
		renderedButtonColor(controls.buttons[rc.ButtonCameraHalf]),
		themeObj.Color(theme.ColorNameDisabledButton, variant),
	) {
		t.Fatal("absent button is not unavailable")
	}
}

func TestHomeControlsRefreshLanguageKeepsWidgets(t *testing.T) {
	application := fynetest.NewApp()
	defer application.Quit()
	controls := newHomeControls()
	root := controls.root
	leftStick := controls.sticks[leftControl]

	controls.refreshLanguage(func(key, fallback string) string {
		if key == "home.controls.left_stick" {
			return "Левый стик"
		}
		if key == "home.controls.button.record" {
			return "Запись"
		}
		return fallback
	})

	if controls.root != root || controls.sticks[leftControl] != leftStick {
		t.Fatal("language refresh replaced control widgets")
	}
	if controls.stickTitles[leftControl].Text != "Левый стик" {
		t.Fatalf(
			"left stick title = %q",
			controls.stickTitles[leftControl].Text,
		)
	}
	renderer := controls.buttons[rc.ButtonRecord].CreateRenderer()
	if text := renderer.Objects()[1].(*canvas.Text).Text; text != "Запись" {
		t.Fatalf("record button text = %q", text)
	}
}
