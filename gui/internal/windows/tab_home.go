package windows

import (
	"fyne.io/fyne/v2"
)

// // // // // // // // // //

func (obj *Obj) renderHome() fyne.CanvasObject {
	obj.home.refreshLanguage()
	return obj.home.root
}
