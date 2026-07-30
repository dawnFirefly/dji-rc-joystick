package remote

import (
	"context"

	"github.com/voluminor/dji-rc-joystick/gui/internal/gamepad"
	"github.com/voluminor/dji-rc-joystick/mod/rc"

	"github.com/rs/zerolog"
)

// // // // // // // // // //

type controllerInterface interface {
	Start(context.Context) error
	Stop()
	OnState(func(rc.StateObj))
	OnStatus(func(rc.StatusObj))
	OnInfo(func(rc.InfoObj))
	Profile() rc.ProfileObj
}

type dependenciesObj struct {
	discover      func(context.Context, *rc.DiscoverConfigObj) ([]rc.RemoteObj, error)
	newController func(rc.ConfigObj) (controllerInterface, error)
	newGamepad    func(func(gamepad.SnapshotObj)) gamepad.Interface
}

// //

func defaultDependencies(logger zerolog.Logger) dependenciesObj {
	return dependenciesObj{
		discover: rc.Discover,
		newController: func(config rc.ConfigObj) (controllerInterface, error) {
			return rc.New(config)
		},
		newGamepad: func(onChange func(gamepad.SnapshotObj)) gamepad.Interface {
			gamepadLogger := logger.With().
				Str("subsystem", "gamepad").
				Logger()
			return gamepad.New(onChange, gamepadLogger)
		},
	}
}
