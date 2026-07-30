package widgets

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// // // // // // // // // //

const (
	ButtonInactive ButtonStateID = iota
	ButtonActive
	ButtonUnavailable
)

// ButtonStateID identifies one visual button state.
type ButtonStateID uint8

type buttonRendererObj struct {
	button     *ButtonObj
	background *canvas.Rectangle
	label      *canvas.Text
	objects    []fyne.CanvasObject
}

// ButtonObj displays button state without accepting input.
type ButtonObj struct {
	widget.BaseWidget

	text  string
	state ButtonStateID
}

// //

func normalizeButtonState(state ButtonStateID) ButtonStateID {
	if state > ButtonUnavailable {
		return ButtonUnavailable
	}
	return state
}

func (obj *buttonRendererObj) Destroy() {
}

func (obj *buttonRendererObj) Layout(size fyne.Size) {
	obj.background.Move(fyne.NewPos(0, 0))
	obj.background.Resize(size)

	labelSize := obj.label.MinSize()
	obj.label.Move(fyne.NewPos(
		(size.Width-labelSize.Width)/2,
		(size.Height-labelSize.Height)/2,
	))
	obj.label.Resize(labelSize)
}

func (obj *buttonRendererObj) MinSize() fyne.Size {
	themeObj := obj.button.Theme()
	textSize := themeObj.Size(theme.SizeNameText)
	padding := themeObj.Size(theme.SizeNameInnerPadding)
	labelSize := fyne.MeasureText(
		obj.button.text,
		textSize,
		fyne.TextStyle{Bold: true},
	)
	return labelSize.AddWidthHeight(4*padding, 2*padding)
}

func (obj *buttonRendererObj) Objects() []fyne.CanvasObject {
	return obj.objects
}

func (obj *buttonRendererObj) Refresh() {
	themeObj := obj.button.Theme()
	variant := currentVariant()
	obj.background.CornerRadius = themeObj.Size(theme.SizeNameButtonRadius)
	obj.background.StrokeWidth = 0
	obj.background.StrokeColor = color.Transparent

	switch obj.button.state {
	case ButtonActive:
		obj.background.FillColor = themeObj.Color(theme.ColorNameSuccess, variant)
		obj.background.StrokeColor = themeObj.Color(
			theme.ColorNameForegroundOnSuccess,
			variant,
		)
		obj.background.StrokeWidth = themeObj.Size(theme.SizeNameInputBorder)
		obj.label.Color = themeObj.Color(theme.ColorNameForegroundOnSuccess, variant)
	case ButtonUnavailable:
		obj.background.FillColor = themeObj.Color(theme.ColorNameDisabledButton, variant)
		obj.label.Color = themeObj.Color(theme.ColorNameDisabled, variant)
	default:
		obj.background.FillColor = themeObj.Color(theme.ColorNameButton, variant)
		obj.label.Color = themeObj.Color(theme.ColorNameForeground, variant)
	}

	obj.label.Text = obj.button.text
	obj.label.TextSize = themeObj.Size(theme.SizeNameText)
	obj.label.TextStyle = fyne.TextStyle{Bold: true}
	obj.Layout(obj.button.Size())
	refreshObjects(obj.objects)
}

// //

// CreateRenderer creates a non-interactive button renderer.
func (obj *ButtonObj) CreateRenderer() fyne.WidgetRenderer {
	obj.ExtendBaseWidget(obj)
	renderer := &buttonRendererObj{
		button:     obj,
		background: canvas.NewRectangle(color.Transparent),
		label:      canvas.NewText(obj.text, color.Transparent),
	}
	renderer.label.Alignment = fyne.TextAlignCenter
	renderer.objects = []fyne.CanvasObject{
		renderer.background,
		renderer.label,
	}
	renderer.Refresh()
	return renderer
}

// MinSize returns the renderer's theme-scaled minimum size.
func (obj *ButtonObj) MinSize() fyne.Size {
	obj.ExtendBaseWidget(obj)
	return obj.BaseWidget.MinSize()
}

// SetState changes the visual state without adding input behavior.
func (obj *ButtonObj) SetState(state ButtonStateID) {
	resolved := normalizeButtonState(state)
	if obj.state == resolved {
		return
	}
	obj.state = resolved
	obj.Refresh()
}

// SetText changes the centered button label.
func (obj *ButtonObj) SetText(text string) {
	if obj.text == text {
		return
	}
	obj.text = text
	obj.Refresh()
}

// NewButton creates a non-interactive button indicator.
func NewButton(text string) *ButtonObj {
	obj := &ButtonObj{text: text}
	obj.ExtendBaseWidget(obj)
	return obj
}
