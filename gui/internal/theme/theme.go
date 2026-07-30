package theme

import (
	"math"
	"sync/atomic"

	targetsettings "github.com/voluminor/dji-rc-joystick/target/settings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// // // // // // // // // //

const (
	minTextScale     = 0.2
	maxTextScale     = 8.0
	defaultTextScale = 1.0
)

// Obj is a Fyne theme whose palette and scale can be changed concurrently.
type Obj struct {
	darkMode atomic.Bool
	textSize atomic.Uint32
}

func normalizeTextScale(scale float32) float32 {
	switch {
	case math.IsNaN(float64(scale)), math.IsInf(float64(scale), 0):
		return defaultTextScale
	case scale < minTextScale:
		return minTextScale
	case scale > maxTextScale:
		return maxTextScale
	default:
		return scale
	}
}

// New creates a thread-safe Fyne theme from UI settings.
func New(settings targetsettings.UiObj) *Obj {
	obj := new(Obj)
	obj.darkMode.Store(settings.DarkMode)
	obj.textSize.Store(math.Float32bits(normalizeTextScale(settings.TextSize)))
	return obj
}

// SetTextScale clamps and applies the multiplier used for Fyne theme sizes.
func (obj *Obj) SetTextScale(scale float32) {
	obj.textSize.Store(math.Float32bits(normalizeTextScale(scale)))
}

// TextScale returns the active Fyne size multiplier.
func (obj *Obj) TextScale() float32 {
	return math.Float32frombits(obj.textSize.Load())
}

// SetDark selects the dark or light color palette.
func (obj *Obj) SetDark(dark bool) {
	obj.darkMode.Store(dark)
}

// IsDark reports whether the dark color palette is active.
func (obj *Obj) IsDark() bool {
	return obj.darkMode.Load()
}

// Size returns the default Fyne size multiplied by TextScale.
func (obj *Obj) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name) * math.Float32frombits(obj.textSize.Load())
}

// Icon delegates icon lookup to the default Fyne theme.
func (obj *Obj) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

// Font delegates font lookup to the default Fyne theme.
func (obj *Obj) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}
