package windows

import (
	"sync/atomic"

	guiremote "github.com/voluminor/dji-rc-joystick/gui/internal/remote"
	"github.com/voluminor/dji-rc-joystick/gui/internal/theme"
	"github.com/voluminor/dji-rc-joystick/mod/data"
	"github.com/voluminor/dji-rc-joystick/mod/rc"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"github.com/rs/zerolog"
)

// // // // // // // // // //

const maxLoggedErrorBytes = 4 << 10

type remoteInterface interface {
	Search() error
	AcceptFound() error
	Connect(string, int) error
	Stop()
	StartGamepad(rc.MappingObj) error
	StopGamepad()
	Updates() <-chan struct{}
	Snapshot() guiremote.SnapshotObj
}

type Obj struct {
	application fyne.App
	window      fyne.Window
	log         zerolog.Logger
	data        *data.Obj
	theme       *theme.Obj
	tabs        *container.AppTabs
	remote      remoteInterface
	home        *homeObj
	closing     atomic.Bool
}

func boundedErrorText(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	if len(text) > maxLoggedErrorBytes {
		return text[:maxLoggedErrorBytes]
	}
	return text
}

func (obj *Obj) logError(action string, err error) {
	obj.log.Error().
		Str("action", action).
		Str("error", boundedErrorText(err)).
		Msg("GUI action failed")
}

func (obj *Obj) Window() fyne.Window {
	return obj.window
}

func (obj *Obj) lg(id, fallback string) string {
	return obj.data.Language().Get(id, fallback)
}
