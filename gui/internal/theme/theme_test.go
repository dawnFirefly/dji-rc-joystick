package theme

import (
	"math"
	"testing"

	targetsettings "github.com/voluminor/dji-rc-joystick/target/settings"

	"fyne.io/fyne/v2/theme"
)

// // // // // // // // // //

func TestObjUpdatesPaletteAndScale(t *testing.T) {
	obj := New(targetsettings.UiObj{DarkMode: true, TextSize: 1.5})
	if !obj.IsDark() {
		t.Fatal("New() did not apply dark mode")
	}
	if scale := obj.TextScale(); scale != 1.5 {
		t.Fatalf("TextScale() = %v, want 1.5", scale)
	}

	base := theme.DefaultTheme().Size(theme.SizeNameText)
	if size := obj.Size(theme.SizeNameText); size != base*1.5 {
		t.Fatalf("Size() = %v, want %v", size, base*1.5)
	}

	obj.SetDark(false)
	obj.SetTextScale(2)
	if obj.IsDark() {
		t.Fatal("SetDark(false) did not select light mode")
	}
	if scale := obj.TextScale(); scale != 2 {
		t.Fatalf("TextScale() = %v, want 2", scale)
	}

	obj.SetTextScale(-1)
	if scale := obj.TextScale(); scale != minTextScale {
		t.Fatalf("negative scale normalized to %v, want %v", scale, minTextScale)
	}
	obj.SetTextScale(100)
	if scale := obj.TextScale(); scale != maxTextScale {
		t.Fatalf("large scale normalized to %v, want %v", scale, maxTextScale)
	}
	obj.SetTextScale(float32(math.NaN()))
	if scale := obj.TextScale(); scale != defaultTextScale {
		t.Fatalf("NaN scale normalized to %v, want %v", scale, defaultTextScale)
	}
}
