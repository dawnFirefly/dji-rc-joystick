package widgets

import (
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// // // // // // // // // //

func normalize(value float64) float32 {
	switch {
	case math.IsNaN(value), math.IsInf(value, 0):
		return 0
	case value < -1:
		return -1
	case value > 1:
		return 1
	default:
		return float32(value)
	}
}

func float32Abs(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}

func currentVariant() fyne.ThemeVariant {
	application := fyne.CurrentApp()
	if application == nil {
		return theme.VariantLight
	}
	return application.Settings().ThemeVariant()
}

func refreshObjects(objects []fyne.CanvasObject) {
	for _, object := range objects {
		object.Refresh()
	}
}
