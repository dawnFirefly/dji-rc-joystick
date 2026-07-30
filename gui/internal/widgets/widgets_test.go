package widgets

import (
	"image/color"
	"math"
	"testing"

	apptheme "github.com/voluminor/dji-rc-joystick/gui/internal/theme"
	targetsettings "github.com/voluminor/dji-rc-joystick/target/settings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	fynetest "fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

// // // // // // // // // //

func newTestApp(t *testing.T) fyne.App {
	t.Helper()
	application := fynetest.NewApp()
	t.Cleanup(application.Quit)
	return application
}

func colorsEqual(first, second color.Color) bool {
	firstR, firstG, firstB, firstA := first.RGBA()
	secondR, secondG, secondB, secondA := second.RGBA()
	return firstR == secondR &&
		firstG == secondG &&
		firstB == secondB &&
		firstA == secondA
}

func distance(first, second fyne.Position) float64 {
	return math.Hypot(
		float64(first.X-second.X),
		float64(first.Y-second.Y),
	)
}

// //

func TestNormalizeClampsInvalidValues(t *testing.T) {
	tests := map[float64]float32{
		math.Inf(-1): 0,
		-2:           -1,
		-0.5:         -0.5,
		0:            0,
		0.5:          0.5,
		2:            1,
		math.Inf(1):  0,
	}
	for value, want := range tests {
		if got := normalize(value); got != want {
			t.Fatalf("normalize(%v) = %v, want %v", value, got, want)
		}
	}
	if got := normalize(math.NaN()); got != 0 {
		t.Fatalf("normalize(NaN) = %v, want 0", got)
	}
}

func TestBarGeometryUsesCenteredZero(t *testing.T) {
	newTestApp(t)
	size := fyne.NewSize(200, 24)

	negative := NewBar()
	negative.SetValue(-0.5)
	negativeRenderer := negative.CreateRenderer().(*barRendererObj)
	negativeRenderer.Layout(size)
	if negativeRenderer.fill.Position().X != 50 ||
		negativeRenderer.fill.Size().Width != 50 {
		t.Fatalf(
			"negative fill = position %.1f width %.1f",
			negativeRenderer.fill.Position().X,
			negativeRenderer.fill.Size().Width,
		)
	}

	positive := NewBar()
	positive.SetValue(0.5)
	positiveRenderer := positive.CreateRenderer().(*barRendererObj)
	positiveRenderer.Layout(size)
	if positiveRenderer.fill.Position().X != 100 ||
		positiveRenderer.fill.Size().Width != 50 {
		t.Fatalf(
			"positive fill = position %.1f width %.1f",
			positiveRenderer.fill.Position().X,
			positiveRenderer.fill.Size().Width,
		)
	}

	zero := NewBar()
	zeroRenderer := zero.CreateRenderer().(*barRendererObj)
	zeroRenderer.Layout(size)
	if !zeroRenderer.fill.Hidden {
		t.Fatal("zero fill is visible")
	}
}

func TestStickGeometryUsesCartesianCoordinates(t *testing.T) {
	newTestApp(t)
	size := fyne.NewSquareSize(200)

	horizontal := NewStick()
	horizontal.SetPosition(1, 0)
	horizontalRenderer := horizontal.CreateRenderer().(*stickRendererObj)
	horizontalRenderer.Layout(size)
	if horizontalRenderer.ray.Position2.X <= horizontalRenderer.ray.Position1.X ||
		horizontalRenderer.ray.Position2.Y != horizontalRenderer.ray.Position1.Y {
		t.Fatalf(
			"horizontal ray = %+v -> %+v",
			horizontalRenderer.ray.Position1,
			horizontalRenderer.ray.Position2,
		)
	}

	vertical := NewStick()
	vertical.SetPosition(0, 1)
	verticalRenderer := vertical.CreateRenderer().(*stickRendererObj)
	verticalRenderer.Layout(size)
	if verticalRenderer.ray.Position2.Y >= verticalRenderer.ray.Position1.Y ||
		verticalRenderer.ray.Position2.X != verticalRenderer.ray.Position1.X {
		t.Fatalf(
			"vertical ray = %+v -> %+v",
			verticalRenderer.ray.Position1,
			verticalRenderer.ray.Position2,
		)
	}
}

func TestStickClampsDiagonalToCircle(t *testing.T) {
	newTestApp(t)
	stick := NewStick()
	stick.SetPosition(1, 1)
	if math.Abs(math.Hypot(float64(stick.x), float64(stick.y))-1) > 0.0001 {
		t.Fatalf("normalized diagonal = %v/%v", stick.x, stick.y)
	}

	renderer := stick.CreateRenderer().(*stickRendererObj)
	renderer.Layout(fyne.NewSquareSize(200))
	radius := float64(renderer.outer.Size().Width / 2)
	if delta := math.Abs(distance(
		renderer.ray.Position1,
		renderer.ray.Position2,
	) - radius); delta > 0.001 {
		t.Fatalf("ray radius delta = %v", delta)
	}
}

func TestButtonStatesUseThemeColors(t *testing.T) {
	newTestApp(t)
	button := NewButton("REC")
	renderer := button.CreateRenderer().(*buttonRendererObj)
	themeObj := button.Theme()
	variant := currentVariant()

	if !colorsEqual(
		renderer.background.FillColor,
		themeObj.Color(theme.ColorNameButton, variant),
	) {
		t.Fatal("inactive button does not use the button palette")
	}

	button.SetState(ButtonActive)
	renderer.Refresh()
	if !colorsEqual(
		renderer.background.FillColor,
		themeObj.Color(theme.ColorNameSuccess, variant),
	) {
		t.Fatal("active button does not use the success palette")
	}

	button.SetState(ButtonUnavailable)
	renderer.Refresh()
	if !colorsEqual(
		renderer.background.FillColor,
		themeObj.Color(theme.ColorNameDisabledButton, variant),
	) {
		t.Fatal("unavailable button does not use the disabled palette")
	}

	button.SetState(ButtonStateID(255))
	if button.state != ButtonUnavailable {
		t.Fatalf("invalid button state = %d", button.state)
	}
}

func TestButtonDoesNotAcceptInput(t *testing.T) {
	newTestApp(t)
	var candidate any = NewButton("C1")
	if _, ok := candidate.(fyne.Tappable); ok {
		t.Fatal("button implements fyne.Tappable")
	}
	if _, ok := candidate.(fyne.Focusable); ok {
		t.Fatal("button implements fyne.Focusable")
	}
	if _, ok := candidate.(desktop.Hoverable); ok {
		t.Fatal("button implements desktop.Hoverable")
	}
	if _, ok := candidate.(desktop.Mouseable); ok {
		t.Fatal("button implements desktop.Mouseable")
	}
	if _, ok := candidate.(desktop.Cursorable); ok {
		t.Fatal("button implements desktop.Cursorable")
	}
}

func TestButtonRendererRefreshesTheme(t *testing.T) {
	application := newTestApp(t)
	themeObj := apptheme.New(targetsettings.UiObj{TextSize: 1})
	application.Settings().SetTheme(themeObj)
	button := NewButton("RTH")
	renderer := button.CreateRenderer().(*buttonRendererObj)
	lightColor := renderer.background.FillColor

	themeObj.SetDark(true)
	renderer.Refresh()
	if colorsEqual(lightColor, renderer.background.FillColor) {
		t.Fatal("button color did not change with the theme")
	}
}
