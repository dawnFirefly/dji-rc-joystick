package theme

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// // // // // // // // // //

var (
	colorDarkBackground          = color.NRGBA{R: 0x17, G: 0x17, B: 0x18, A: 0xff}
	colorDarkButton              = color.NRGBA{R: 0x28, G: 0x29, B: 0x2e, A: 0xff}
	colorDarkDisabled            = color.NRGBA{R: 0x39, G: 0x39, B: 0x3a, A: 0xff}
	colorDarkDisabledButton      = color.NRGBA{R: 0x28, G: 0x29, B: 0x2e, A: 0xff}
	colorDarkError               = color.NRGBA{R: 0xf4, G: 0x43, B: 0x36, A: 0xff}
	colorDarkForeground          = color.NRGBA{R: 0xf3, G: 0xf3, B: 0xf3, A: 0xff}
	colorDarkForegroundOnError   = color.NRGBA{R: 0x17, G: 0x17, B: 0x18, A: 0xff}
	colorDarkForegroundOnSuccess = color.NRGBA{R: 0x17, G: 0x17, B: 0x18, A: 0xff}
	colorDarkForegroundOnWarning = color.NRGBA{R: 0x17, G: 0x17, B: 0x18, A: 0xff}
	colorDarkHeaderBackground    = color.NRGBA{R: 0x1b, G: 0x1b, B: 0x1b, A: 0xff}
	colorDarkHover               = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x0f}
	colorDarkInputBackground     = color.NRGBA{R: 0x20, G: 0x20, B: 0x23, A: 0xff}
	colorDarkInputBorder         = color.NRGBA{R: 0x39, G: 0x39, B: 0x3a, A: 0xff}
	colorDarkMenuBackground      = color.NRGBA{R: 0x28, G: 0x29, B: 0x2e, A: 0xff}
	colorDarkOverlayBackground   = color.NRGBA{R: 0x18, G: 0x1d, B: 0x25, A: 0xff}
	colorDarkPlaceholder         = color.NRGBA{R: 0xb2, G: 0xb2, B: 0xb2, A: 0xff}
	colorDarkPressed             = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x66}
	colorDarkScrollBar           = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x99}
	colorDarkScrollBarBackground = color.NRGBA{R: 0x20, G: 0x20, B: 0x23, A: 0xff}
	colorDarkSeparator           = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff}
	colorDarkShadow              = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x66}
	colorDarkSuccess             = color.NRGBA{R: 0x43, G: 0xf4, B: 0x36, A: 0xff}
	colorDarkWarning             = color.NRGBA{R: 0xff, G: 0x98, B: 0x00, A: 0xff}

	colorLightBackground          = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	colorLightButton              = color.NRGBA{R: 0xf5, G: 0xf5, B: 0xf5, A: 0xff}
	colorLightDisabled            = color.NRGBA{R: 0xe3, G: 0xe3, B: 0xe3, A: 0xff}
	colorLightDisabledButton      = color.NRGBA{R: 0xf5, G: 0xf5, B: 0xf5, A: 0xff}
	colorLightError               = color.NRGBA{R: 0xf4, G: 0x43, B: 0x36, A: 0xff}
	colorLightFocusBlue           = color.NRGBA{R: 0x00, G: 0x6c, B: 0xff, A: 0x2a}
	colorLightForeground          = color.NRGBA{R: 0x56, G: 0x56, B: 0x56, A: 0xff}
	colorLightForegroundOnError   = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	colorLightForegroundOnSuccess = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	colorLightForegroundOnWarning = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	colorLightHeaderBackground    = color.NRGBA{R: 0xf9, G: 0xf9, B: 0xf9, A: 0xff}
	colorLightHover               = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x0f}
	colorLightInputBackground     = color.NRGBA{R: 0xf3, G: 0xf3, B: 0xf3, A: 0xff}
	colorLightInputBorder         = color.NRGBA{R: 0xe3, G: 0xe3, B: 0xe3, A: 0xff}
	colorLightMenuBackground      = color.NRGBA{R: 0xf5, G: 0xf5, B: 0xf5, A: 0xff}
	colorLightOverlayBackground   = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	colorLightPlaceholder         = color.NRGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xff}
	colorLightPressed             = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x19}
	colorLightScrollBar           = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x99}
	colorLightScrollBarBackground = color.NRGBA{R: 0xdb, G: 0xdb, B: 0xdb, A: 0xff}
	colorLightSelectionBlue       = color.NRGBA{R: 0x00, G: 0x6c, B: 0xff, A: 0x40}
	colorLightSeparator           = color.NRGBA{R: 0xe3, G: 0xe3, B: 0xe3, A: 0xff}
	colorLightShadow              = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x33}
	colorLightSuccess             = color.NRGBA{R: 0x43, G: 0xf4, B: 0x36, A: 0xff}
	colorLightWarning             = color.NRGBA{R: 0xff, G: 0x98, B: 0x00, A: 0xff}
)

// Color resolves name from the active palette.
func (obj *Obj) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	if obj.darkMode.Load() {
		switch name {
		case theme.ColorNameBackground:
			return colorDarkBackground
		case theme.ColorNameButton:
			return colorDarkButton
		case theme.ColorNameDisabled:
			return colorDarkDisabled
		case theme.ColorNameDisabledButton:
			return colorDarkDisabledButton
		case theme.ColorNameError:
			return colorDarkError
		case theme.ColorNameForeground:
			return colorDarkForeground
		case theme.ColorNameForegroundOnError:
			return colorDarkForegroundOnError
		case theme.ColorNameForegroundOnSuccess:
			return colorDarkForegroundOnSuccess
		case theme.ColorNameForegroundOnWarning:
			return colorDarkForegroundOnWarning
		case theme.ColorNameHeaderBackground:
			return colorDarkHeaderBackground
		case theme.ColorNameHover:
			return colorDarkHover
		case theme.ColorNameInputBackground:
			return colorDarkInputBackground
		case theme.ColorNameInputBorder:
			return colorDarkInputBorder
		case theme.ColorNameMenuBackground:
			return colorDarkMenuBackground
		case theme.ColorNameOverlayBackground:
			return colorDarkOverlayBackground
		case theme.ColorNamePlaceHolder:
			return colorDarkPlaceholder
		case theme.ColorNamePressed:
			return colorDarkPressed
		case theme.ColorNameScrollBar:
			return colorDarkScrollBar
		case theme.ColorNameScrollBarBackground:
			return colorDarkScrollBarBackground
		case theme.ColorNameSeparator:
			return colorDarkSeparator
		case theme.ColorNameShadow:
			return colorDarkShadow
		case theme.ColorNameSuccess:
			return colorDarkSuccess
		case theme.ColorNameWarning:
			return colorDarkWarning
		default:
			return theme.DefaultTheme().Color(name, theme.VariantDark)
		}
	}

	switch name {
	case theme.ColorNameBackground:
		return colorLightBackground
	case theme.ColorNameButton:
		return colorLightButton
	case theme.ColorNameDisabled:
		return colorLightDisabled
	case theme.ColorNameDisabledButton:
		return colorLightDisabledButton
	case theme.ColorNameError:
		return colorLightError
	case theme.ColorNameFocus:
		return colorLightFocusBlue
	case theme.ColorNameForeground:
		return colorLightForeground
	case theme.ColorNameForegroundOnError:
		return colorLightForegroundOnError
	case theme.ColorNameForegroundOnSuccess:
		return colorLightForegroundOnSuccess
	case theme.ColorNameForegroundOnWarning:
		return colorLightForegroundOnWarning
	case theme.ColorNameHeaderBackground:
		return colorLightHeaderBackground
	case theme.ColorNameHover:
		return colorLightHover
	case theme.ColorNameInputBackground:
		return colorLightInputBackground
	case theme.ColorNameInputBorder:
		return colorLightInputBorder
	case theme.ColorNameMenuBackground:
		return colorLightMenuBackground
	case theme.ColorNameOverlayBackground:
		return colorLightOverlayBackground
	case theme.ColorNamePlaceHolder:
		return colorLightPlaceholder
	case theme.ColorNamePressed:
		return colorLightPressed
	case theme.ColorNameScrollBar:
		return colorLightScrollBar
	case theme.ColorNameScrollBarBackground:
		return colorLightScrollBarBackground
	case theme.ColorNameSelection:
		return colorLightSelectionBlue
	case theme.ColorNameSeparator:
		return colorLightSeparator
	case theme.ColorNameShadow:
		return colorLightShadow
	case theme.ColorNameSuccess:
		return colorLightSuccess
	case theme.ColorNameWarning:
		return colorLightWarning
	default:
		return theme.DefaultTheme().Color(name, theme.VariantLight)
	}
}
