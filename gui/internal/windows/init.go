package windows

import (
	guiremote "github.com/voluminor/dji-rc-joystick/gui/internal/remote"
	"github.com/voluminor/dji-rc-joystick/gui/internal/theme"
	"github.com/voluminor/dji-rc-joystick/mod/data"
	"github.com/voluminor/dji-rc-joystick/target"

	"fyne.io/fyne/v2"
	"github.com/rs/zerolog"
)

// // // // // // // // // //

func newWithRemote(
	application fyne.App,
	logger zerolog.Logger,
	dataObj *data.Obj,
	themeObj *theme.Obj,
	icon fyne.Resource,
	remoteObj remoteInterface,
) *Obj {
	obj := &Obj{
		application: application,
		log: logger.With().
			Str("component", "window").
			Logger(),
		data:   dataObj,
		theme:  themeObj,
		remote: remoteObj,
	}

	obj.window = application.NewWindow(target.Name)
	obj.window.SetIcon(icon)
	obj.window.Resize(fyne.NewSize(960, 720))

	obj.home = newHome(obj)
	obj.tabs = obj.newTabs()
	obj.window.SetContent(obj.tabs)
	obj.window.SetCloseIntercept(obj.close)
	obj.home.start()

	obj.log.Info().Msg("Main window initialized")
	return obj
}

// //

func New(
	application fyne.App,
	logger zerolog.Logger,
	dataObj *data.Obj,
	themeObj *theme.Obj,
	icon fyne.Resource,
) *Obj {
	remoteLogger := logger.With().
		Str("component", "remote").
		Logger()
	return newWithRemote(
		application,
		logger,
		dataObj,
		themeObj,
		icon,
		guiremote.New(remoteLogger),
	)
}
