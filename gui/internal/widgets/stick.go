package widgets

import (
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// // // // // // // // // //

type stickRendererObj struct {
	stick      *StickObj
	outer      *canvas.Circle
	horizontal *canvas.Line
	vertical   *canvas.Line
	ray        *canvas.Line
	center     *canvas.Circle
	endpoint   *canvas.Circle
	objects    []fyne.CanvasObject
}

// StickObj visualizes a normalized two-dimensional stick position.
type StickObj struct {
	widget.BaseWidget

	x float32
	y float32
}

// //

func normalizePosition(x, y float64) (float32, float32) {
	resolvedX := normalize(x)
	resolvedY := normalize(y)
	length := float32(math.Hypot(float64(resolvedX), float64(resolvedY)))
	if length <= 1 {
		return resolvedX, resolvedY
	}
	return resolvedX / length, resolvedY / length
}

func (obj *stickRendererObj) Destroy() {
}

func (obj *stickRendererObj) Layout(size fyne.Size) {
	themeObj := obj.stick.Theme()
	lineWidth := themeObj.Size(theme.SizeNameInputBorder)
	endpointRadius := max(themeObj.Size(theme.SizeNameInnerPadding), 3*lineWidth)
	margin := endpointRadius + lineWidth
	diameter := min(size.Width, size.Height) - 2*margin
	if diameter < 0 {
		diameter = 0
	}

	left := (size.Width - diameter) / 2
	top := (size.Height - diameter) / 2
	center := fyne.NewPos(size.Width/2, size.Height/2)
	radius := diameter / 2
	endpoint := fyne.NewPos(
		center.X+obj.stick.x*radius,
		center.Y-obj.stick.y*radius,
	)

	obj.outer.Move(fyne.NewPos(left, top))
	obj.outer.Resize(fyne.NewSquareSize(diameter))
	obj.horizontal.Position1 = fyne.NewPos(left, center.Y)
	obj.horizontal.Position2 = fyne.NewPos(left+diameter, center.Y)
	obj.vertical.Position1 = fyne.NewPos(center.X, top)
	obj.vertical.Position2 = fyne.NewPos(center.X, top+diameter)
	obj.ray.Position1 = center
	obj.ray.Position2 = endpoint

	centerRadius := 2 * lineWidth
	obj.center.Move(fyne.NewPos(
		center.X-centerRadius,
		center.Y-centerRadius,
	))
	obj.center.Resize(fyne.NewSquareSize(2 * centerRadius))
	obj.endpoint.Move(fyne.NewPos(
		endpoint.X-endpointRadius,
		endpoint.Y-endpointRadius,
	))
	obj.endpoint.Resize(fyne.NewSquareSize(2 * endpointRadius))
}

func (obj *stickRendererObj) MinSize() fyne.Size {
	side := obj.stick.Theme().Size(theme.SizeNameText) * 8
	return fyne.NewSquareSize(side)
}

func (obj *stickRendererObj) Objects() []fyne.CanvasObject {
	return obj.objects
}

func (obj *stickRendererObj) Refresh() {
	themeObj := obj.stick.Theme()
	variant := currentVariant()
	lineWidth := themeObj.Size(theme.SizeNameInputBorder)
	trackColor := themeObj.Color(theme.ColorNameInputBorder, variant)
	accentColor := themeObj.Color(theme.ColorNamePrimary, variant)

	obj.outer.FillColor = themeObj.Color(theme.ColorNameInputBackground, variant)
	obj.outer.StrokeColor = trackColor
	obj.outer.StrokeWidth = lineWidth
	obj.horizontal.StrokeColor = trackColor
	obj.horizontal.StrokeWidth = lineWidth
	obj.vertical.StrokeColor = trackColor
	obj.vertical.StrokeWidth = lineWidth
	obj.ray.StrokeColor = accentColor
	obj.ray.StrokeWidth = 2 * lineWidth
	obj.center.FillColor = themeObj.Color(theme.ColorNameForeground, variant)
	obj.center.StrokeColor = color.Transparent
	obj.endpoint.FillColor = accentColor
	obj.endpoint.StrokeColor = themeObj.Color(
		theme.ColorNameForegroundOnPrimary,
		variant,
	)
	obj.endpoint.StrokeWidth = lineWidth

	obj.Layout(obj.stick.Size())
	refreshObjects(obj.objects)
}

// //

// CreateRenderer creates the circular stick renderer.
func (obj *StickObj) CreateRenderer() fyne.WidgetRenderer {
	obj.ExtendBaseWidget(obj)
	renderer := &stickRendererObj{
		stick:      obj,
		outer:      canvas.NewCircle(color.Transparent),
		horizontal: canvas.NewLine(color.Transparent),
		vertical:   canvas.NewLine(color.Transparent),
		ray:        canvas.NewLine(color.Transparent),
		center:     canvas.NewCircle(color.Transparent),
		endpoint:   canvas.NewCircle(color.Transparent),
	}
	renderer.objects = []fyne.CanvasObject{
		renderer.outer,
		renderer.horizontal,
		renderer.vertical,
		renderer.ray,
		renderer.center,
		renderer.endpoint,
	}
	renderer.Refresh()
	return renderer
}

// MinSize returns a theme-scaled square minimum size.
func (obj *StickObj) MinSize() fyne.Size {
	obj.ExtendBaseWidget(obj)
	return obj.BaseWidget.MinSize()
}

// SetPosition clamps and displays a normalized Cartesian position.
func (obj *StickObj) SetPosition(x, y float64) {
	resolvedX, resolvedY := normalizePosition(x, y)
	if obj.x == resolvedX && obj.y == resolvedY {
		return
	}
	obj.x = resolvedX
	obj.y = resolvedY
	obj.Refresh()
}

// NewStick creates a centered two-dimensional stick indicator.
func NewStick() *StickObj {
	obj := new(StickObj)
	obj.ExtendBaseWidget(obj)
	return obj
}
