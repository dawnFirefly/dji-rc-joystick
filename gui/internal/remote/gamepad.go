package remote

import (
	"errors"

	"github.com/voluminor/dji-rc-joystick/gui/internal/gamepad"
	"github.com/voluminor/dji-rc-joystick/mod/rc"
)

// // // // // // // // // //

func (obj *Obj) observeGamepad(snapshot gamepad.SnapshotObj) {
	obj.mu.Lock()
	obj.snapshot.Gamepad = snapshot
	obj.notifyLocked()
	obj.mu.Unlock()
}

// //

// StartGamepad starts emulation with the persisted mapping and active profile.
func (obj *Obj) StartGamepad(mapping rc.MappingObj) error {
	obj.gamepadMu.Lock()
	defer obj.gamepadMu.Unlock()

	obj.mu.RLock()
	phase := obj.snapshot.Phase
	profile := cloneProfile(obj.snapshot.Profile)
	obj.mu.RUnlock()
	if phase != PhaseConnected {
		return errors.New("remote must be connected before starting emulation")
	}
	return obj.gamepad.Start(mapping, profile)
}

// StopGamepad stops emulation without disconnecting the remote.
func (obj *Obj) StopGamepad() {
	obj.gamepadMu.Lock()
	defer obj.gamepadMu.Unlock()
	obj.gamepad.Stop()
}
