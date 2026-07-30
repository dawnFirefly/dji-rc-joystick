package widgets

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// // // // // // // // // //

type barRendererObj struct {
	bar     *BarObj
	track   *canvas.Rectangle
	fill    *canvas.Rectangle
	center  *canvas.Line
	objects []fyne.CanvasObject
}

// BarObj visualizes a normalized value around a centered zero point.
type BarObj struct {
	widget.BaseWidget

	value float32
}

// //

func (obj *barRendererObj) Destroy() {
}

func (obj *barRendererObj) Layout(size fyne.Size) {
	obj.track.Move(fyne.NewPos(0, 0))
	obj.track.Resize(size)

	centerX := size.Width / 2
	obj.center.Position1 = fyne.NewPos(centerX, 0)
	obj.center.Position2 = fyne.NewPos(centerX, size.Height)

	width := float32Abs(obj.bar.value) * centerX
	x := centerX
	if obj.bar.value < 0 {
		x -= width
	}
	obj.fill.Move(fyne.NewPos(x, 0))
	obj.fill.Resize(fyne.NewSize(width, size.Height))
	obj.fill.Hidden = width == 0
}

func (obj *barRendererObj) MinSize() fyne.Size {
	themeObj := obj.bar.Theme()
	textSize := themeObj.Size(theme.SizeNameText)
	padding := themeObj.Size(theme.SizeNameInnerPadding)
	return fyne.NewSize(textSize*10, textSize+2*padding)
}

func (obj *barRendererObj) Objects() []fyne.CanvasObject {
	return obj.objects
}

func (obj *barRendererObj) Refresh() {
	themeObj := obj.bar.Theme()
	variant := currentVariant()
	borderWidth := themeObj.Size(theme.SizeNameInputBorder)
	radius := themeObj.Size(theme.SizeNameInputRadius)

	obj.track.FillColor = themeObj.Color(theme.ColorNameInputBackground, variant)
	obj.track.StrokeColor = themeObj.Color(theme.ColorNameInputBorder, variant)
	obj.track.StrokeWidth = borderWidth
	obj.track.CornerRadius = radius
	obj.fill.FillColor = themeObj.Color(theme.ColorNamePrimary, variant)
	obj.fill.StrokeColor = color.Transparent
	obj.fill.StrokeWidth = 0
	obj.fill.CornerRadius = radius
	obj.center.StrokeColor = themeObj.Color(theme.ColorNameForeground, variant)
	obj.center.StrokeWidth = borderWidth

	obj.Layout(obj.bar.Size())
	refreshObjects(obj.objects)
}

// //

// CreateRenderer creates the theme-aware centered bar renderer.
func (obj *BarObj) CreateRenderer() fyne.WidgetRenderer {
	obj.ExtendBaseWidget(obj)
	renderer := &barRendererObj{
		bar:    obj,
		track:  canvas.NewRectangle(color.Transparent),
		fill:   canvas.NewRectangle(color.Transparent),
		center: canvas.NewLine(color.Transparent),
	}
	renderer.objects = []fyne.CanvasObject{
		renderer.track,
		renderer.fill,
		renderer.center,
	}
	renderer.Refresh()
	return renderer
}

// MinSize returns the renderer's theme-scaled minimum size.
func (obj *BarObj) MinSize() fyne.Size {
	obj.ExtendBaseWidget(obj)
	return obj.BaseWidget.MinSize()
}

// SetValue clamps and displays a normalized value in the range -1..1.
func (obj *BarObj) SetValue(value float64) {
	resolved := normalize(value)
	if obj.value == resolved {
		return
	}
	obj.value = resolved
	obj.Refresh()
}

// NewBar creates an empty centered bar.
func NewBar() *BarObj {
	obj := new(BarObj)
	obj.ExtendBaseWidget(obj)
	return obj
}
