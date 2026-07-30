package gamepad

import (
	"errors"

	"github.com/voluminor/dji-rc-joystick/mod/rc"
)

// // // // // // // // // //

var (
	// ErrAuthorizationCanceled reports a dismissed Linux authorization prompt.
	ErrAuthorizationCanceled = errors.New("gamepad authorization was canceled")
	errOperationActive       = errors.New("gamepad operation is already active")
)

// PhaseID identifies the virtual-gamepad lifecycle phase.
type PhaseID uint8

const (
	PhaseStopped PhaseID = iota
	PhaseStarting
	PhaseRunning
	PhaseWaiting
	PhaseStopping
	PhaseError
)

// SnapshotObj is an immutable view of the virtual-gamepad runtime.
type SnapshotObj struct {
	Revision      uint64
	ErrorRevision uint64
	Phase         PhaseID
	Error         string
}

// Interface controls one independently managed virtual gamepad.
type Interface interface {
	Start(rc.MappingObj, rc.ProfileObj) error
	Stop()
	SetOnline(bool)
	Feed(rc.StateObj)
	Snapshot() SnapshotObj
}

// //

// String returns the stable lowercase phase name.
func (phase PhaseID) String() string {
	switch phase {
	case PhaseStopped:
		return "stopped"
	case PhaseStarting:
		return "starting"
	case PhaseRunning:
		return "running"
	case PhaseWaiting:
		return "waiting"
	case PhaseStopping:
		return "stopping"
	case PhaseError:
		return "error"
	default:
		return "invalid"
	}
}
