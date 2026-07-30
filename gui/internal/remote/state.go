package remote

import (
	"github.com/voluminor/dji-rc-joystick/gui/internal/gamepad"
	"github.com/voluminor/dji-rc-joystick/mod/rc"
)

// // // // // // // // // //

// PhaseID identifies the GUI-managed remote lifecycle phase.
type PhaseID uint8

const (
	PhaseIdle PhaseID = iota
	PhaseSearching
	PhaseConnecting
	PhaseConnected
	PhaseReconnecting
	PhaseDisconnecting
)

// SnapshotObj is an immutable view of the latest runtime state.
type SnapshotObj struct {
	Revision       uint64
	ErrorRevision  uint64
	SearchRevision uint64
	SearchCount    int
	Phase          PhaseID
	Progress       rc.DiscoverProgressObj
	Remotes        []rc.RemoteObj
	TargetIP       string
	TargetPort     int
	Status         rc.StatusObj
	State          rc.StateObj
	Info           rc.InfoObj
	Profile        rc.ProfileObj
	Gamepad        gamepad.SnapshotObj
	Error          string
}

// //

func displayInfo(info rc.InfoObj) rc.InfoObj {
	info.Telemetry = nil
	return info
}

func cloneProfile(profile rc.ProfileObj) rc.ProfileObj {
	profile.DeviceIDs = append([]string(nil), profile.DeviceIDs...)
	profile.AbsentButtons = append([]rc.ButtonID(nil), profile.AbsentButtons...)
	return profile
}

func cloneSnapshot(snapshot SnapshotObj) SnapshotObj {
	snapshot.Remotes = append([]rc.RemoteObj(nil), snapshot.Remotes...)
	snapshot.Profile = cloneProfile(snapshot.Profile)
	return snapshot
}

// //

// String returns the stable lowercase phase name.
func (phase PhaseID) String() string {
	switch phase {
	case PhaseIdle:
		return "idle"
	case PhaseSearching:
		return "searching"
	case PhaseConnecting:
		return "connecting"
	case PhaseConnected:
		return "connected"
	case PhaseReconnecting:
		return "reconnecting"
	case PhaseDisconnecting:
		return "disconnecting"
	default:
		return "invalid"
	}
}
