package windows

import (
	"fmt"

	guiremote "github.com/voluminor/dji-rc-joystick/gui/internal/remote"
	guiwidgets "github.com/voluminor/dji-rc-joystick/gui/internal/widgets"
	"github.com/voluminor/dji-rc-joystick/mod/rc"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// // // // // // // // // //

const (
	leftControl = iota
	rightControl
	controlCount

	axisScale = 32767.0
)

type homeControlsObj struct {
	root *fyne.Container

	stickTitles [controlCount]*widget.Label
	sticks      [controlCount]*guiwidgets.StickObj
	stickValues [controlCount]*widget.Label
	dialTitles  [controlCount]*widget.Label
	dials       [controlCount]*guiwidgets.BarObj
	dialValues  [controlCount]*widget.Label
	buttonTitle *widget.Label
	buttons     [rc.ButtonCount]*guiwidgets.ButtonObj
}

// //

func newControlTitle(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Alignment = fyne.TextAlignCenter
	label.SizeName = theme.SizeNameSubHeadingText
	label.TextStyle = fyne.TextStyle{Bold: true}
	return label
}

func newControlValue(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Alignment = fyne.TextAlignCenter
	label.TextStyle = fyne.TextStyle{Monospace: true}
	return label
}

func stickColumn(
	title *widget.Label,
	stick *guiwidgets.StickObj,
	value *widget.Label,
) fyne.CanvasObject {
	return container.NewVBox(title, container.NewCenter(stick), value)
}

func dialColumn(
	title *widget.Label,
	dial *guiwidgets.BarObj,
	value *widget.Label,
) fyne.CanvasObject {
	return container.NewVBox(title, dial, value)
}

func localizedControlButton(
	button rc.ButtonID,
	lg func(string, string) string,
) string {
	switch button {
	case rc.ButtonRTH:
		return lg("home.controls.button.rth", "RTH")
	case rc.ButtonRecord:
		return lg("home.controls.button.record", "Record")
	case rc.ButtonCameraHalf:
		return lg("home.controls.button.camera_half", "Half-press")
	case rc.ButtonCameraFull:
		return lg("home.controls.button.camera_full", "Shutter")
	case rc.ButtonC1:
		return lg("home.controls.button.c1", "C1")
	case rc.ButtonC2:
		return lg("home.controls.button.c2", "C2")
	default:
		return button.String()
	}
}

// //

func newHomeControls() *homeControlsObj {
	obj := new(homeControlsObj)
	for index := range controlCount {
		obj.stickTitles[index] = newControlTitle("")
		obj.sticks[index] = guiwidgets.NewStick()
		obj.stickValues[index] = newControlValue("X: 0  Y: 0")
		obj.dialTitles[index] = newControlTitle("")
		obj.dials[index] = guiwidgets.NewBar()
		obj.dialValues[index] = newControlValue("0")
	}
	for button := rc.ButtonID(0); button < rc.ButtonCount; button++ {
		obj.buttons[button] = guiwidgets.NewButton(button.String())
	}
	obj.buttonTitle = newControlTitle("")

	sticks := container.NewGridWithColumns(
		controlCount,
		stickColumn(
			obj.stickTitles[leftControl],
			obj.sticks[leftControl],
			obj.stickValues[leftControl],
		),
		stickColumn(
			obj.stickTitles[rightControl],
			obj.sticks[rightControl],
			obj.stickValues[rightControl],
		),
	)
	dials := container.NewGridWithColumns(
		controlCount,
		dialColumn(
			obj.dialTitles[leftControl],
			obj.dials[leftControl],
			obj.dialValues[leftControl],
		),
		dialColumn(
			obj.dialTitles[rightControl],
			obj.dials[rightControl],
			obj.dialValues[rightControl],
		),
	)
	obj.root = container.NewVBox(
		container.NewGridWithColumns(2, obj.buttons[rc.ButtonRecord], container.NewGridWithColumns(1, obj.buttons[rc.ButtonCameraHalf], obj.buttons[rc.ButtonCameraFull])),
		dials,
		widget.NewSeparator(),
		sticks,
		widget.NewSeparator(),
		obj.buttonTitle,
		container.NewGridWithColumns(3, obj.buttons[rc.ButtonC1], obj.buttons[rc.ButtonRTH], obj.buttons[rc.ButtonC2]),
	)
	return obj
}

func (obj *homeControlsObj) refreshLanguage(
	lg func(string, string) string,
) {
	obj.stickTitles[leftControl].SetText(
		lg("home.controls.left_stick", "Left stick"),
	)
	obj.stickTitles[rightControl].SetText(
		lg("home.controls.right_stick", "Right stick"),
	)
	obj.dialTitles[leftControl].SetText(
		lg("home.controls.left_dial", "Left dial"),
	)
	obj.dialTitles[rightControl].SetText(
		lg("home.controls.right_dial", "Right dial"),
	)
	obj.buttonTitle.SetText(lg("home.controls.buttons", "Buttons"))
	for button := rc.ButtonID(0); button < rc.ButtonCount; button++ {
		obj.buttons[button].SetText(localizedControlButton(button, lg))
	}
}

func (obj *homeControlsObj) update(snapshot guiremote.SnapshotObj) {
	state := snapshot.State
	obj.sticks[leftControl].SetPosition(
		float64(state.Axes[rc.AxisLeftH])/axisScale,
		float64(state.Axes[rc.AxisLeftV])/axisScale,
	)
	obj.sticks[rightControl].SetPosition(
		float64(state.Axes[rc.AxisRightH])/axisScale,
		float64(state.Axes[rc.AxisRightV])/axisScale,
	)
	obj.stickValues[leftControl].SetText(fmt.Sprintf(
		"X: %d  Y: %d",
		state.Axes[rc.AxisLeftH],
		state.Axes[rc.AxisLeftV],
	))
	obj.stickValues[rightControl].SetText(fmt.Sprintf(
		"X: %d  Y: %d",
		state.Axes[rc.AxisRightH],
		state.Axes[rc.AxisRightV],
	))

	obj.dials[leftControl].SetValue(
		float64(state.Axes[rc.AxisLeftWheel]) / axisScale,
	)
	obj.dials[rightControl].SetValue(
		float64(state.Axes[rc.AxisRightWheel]) / axisScale,
	)
	obj.dialValues[leftControl].SetText(fmt.Sprintf(
		"%d",
		state.Axes[rc.AxisLeftWheel],
	))
	obj.dialValues[rightControl].SetText(fmt.Sprintf(
		"%d",
		state.Axes[rc.AxisRightWheel],
	))

	capabilities := snapshot.Profile.Capabilities()
	for button := rc.ButtonID(0); button < rc.ButtonCount; button++ {
		buttonState := guiwidgets.ButtonInactive
		if !capabilities.Buttons[button] {
			buttonState = guiwidgets.ButtonUnavailable
		} else if state.Pressed(button) {
			buttonState = guiwidgets.ButtonActive
		}
		obj.buttons[button].SetState(buttonState)
	}
}
