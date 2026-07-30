package windows

import (
	"fyne.io/fyne/v2"
)

// // // // // // // // // //

func (obj *Obj) close() {
	if !obj.closing.CompareAndSwap(false, true) {
		return
	}
	obj.log.Info().Msg("Main window close requested")

	go func() {
		obj.home.stop()
		obj.remote.Stop()
		fyne.Do(func() {
			obj.log.Info().Msg("Main window closed")
			obj.window.SetCloseIntercept(nil)
			obj.window.Close()
		})
	}()
}
