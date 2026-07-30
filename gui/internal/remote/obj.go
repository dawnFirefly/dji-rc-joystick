package remote

import (
	"context"
	"errors"
	"sync"

	"github.com/voluminor/dji-rc-joystick/gui/internal/gamepad"
	"github.com/voluminor/dji-rc-joystick/mod/rc"

	"github.com/rs/zerolog"
)

// // // // // // // // // //

var (
	errOperationActive = errors.New("remote operation is already active")
	errNoSearchResults = errors.New("no partial search results are available")
)

type operationObj struct {
	generation  uint64
	cancel      context.CancelFunc
	done        chan struct{}
	everOnline  bool
	acceptFound bool
}

// Obj owns one GUI remote operation and its latest bounded snapshots.
type Obj struct {
	dependencies dependenciesObj
	log          zerolog.Logger
	updates      chan struct{}
	gamepad      gamepad.Interface
	gamepadMu    sync.Mutex

	mu         sync.RWMutex
	generation uint64
	operation  *operationObj
	snapshot   SnapshotObj
}

// //

func newWithDependencies(
	dependencies dependenciesObj,
	loggers ...zerolog.Logger,
) *Obj {
	logger := zerolog.Nop()
	if len(loggers) > 0 {
		logger = loggers[0]
	}
	if dependencies.newGamepad == nil {
		dependencies.newGamepad = func(
			onChange func(gamepad.SnapshotObj),
		) gamepad.Interface {
			gamepadLogger := logger.With().
				Str("subsystem", "gamepad").
				Logger()
			return gamepad.New(onChange, gamepadLogger)
		}
	}
	obj := &Obj{
		dependencies: dependencies,
		log:          logger,
		updates:      make(chan struct{}, 1),
		snapshot: SnapshotObj{
			Phase: PhaseIdle,
		},
	}
	obj.gamepad = dependencies.newGamepad(obj.observeGamepad)
	obj.snapshot.Gamepad = obj.gamepad.Snapshot()
	return obj
}

func (obj *Obj) notifyLocked() {
	obj.snapshot.Revision++
	select {
	case obj.updates <- struct{}{}:
	default:
	}
}

func (obj *Obj) beginOperationLocked() (*operationObj, context.Context) {
	ctx, cancel := context.WithCancel(context.Background())
	obj.generation++
	operation := &operationObj{
		generation: obj.generation,
		cancel:     cancel,
		done:       make(chan struct{}),
	}
	obj.operation = operation
	return operation, ctx
}

func (obj *Obj) activeLocked(operation *operationObj) bool {
	return obj.operation == operation &&
		obj.operation.generation == operation.generation
}

func (obj *Obj) setErrorLocked(err error) {
	if err == nil || errorsIsCancellation(err) {
		return
	}
	obj.snapshot.Error = err.Error()
	obj.snapshot.ErrorRevision++
}

func (obj *Obj) finishOperation(operation *operationObj, err error) {
	obj.gamepadMu.Lock()
	defer obj.gamepadMu.Unlock()

	obj.gamepad.Stop()
	obj.gamepad.SetOnline(false)

	applied := false
	obj.mu.Lock()
	if obj.activeLocked(operation) {
		applied = true
		obj.operation = nil
		obj.snapshot.Phase = PhaseIdle
		obj.snapshot.Progress = rc.DiscoverProgressObj{}
		obj.snapshot.TargetIP = ""
		obj.snapshot.TargetPort = 0
		obj.snapshot.Status = rc.StatusObj{Phase: rc.PhaseDisconnected}
		obj.snapshot.State = rc.StateObj{}
		obj.snapshot.Info = rc.InfoObj{}
		obj.snapshot.Profile = rc.ProfileObj{}
		obj.setErrorLocked(err)
		obj.notifyLocked()
	}
	obj.mu.Unlock()
	if applied {
		event := obj.log.Info()
		message := "Remote connection stopped"
		if err != nil && !errorsIsCancellation(err) {
			event = obj.log.Error().Err(err)
			message = "Remote connection failed"
		}
		event.
			Uint64("generation", operation.generation).
			Msg(message)
	}
	close(operation.done)
}

// //

// New creates an idle GUI remote runtime.
func New(loggers ...zerolog.Logger) *Obj {
	logger := zerolog.Nop()
	if len(loggers) > 0 {
		logger = loggers[0]
	}
	return newWithDependencies(defaultDependencies(logger), logger)
}

// Updates returns a bounded latest-state notification channel.
func (obj *Obj) Updates() <-chan struct{} {
	return obj.updates
}

// Snapshot returns an independent copy of the latest runtime state.
func (obj *Obj) Snapshot() SnapshotObj {
	obj.mu.RLock()
	defer obj.mu.RUnlock()
	return cloneSnapshot(obj.snapshot)
}

// Stop cancels the active operation and waits for all owned workers.
func (obj *Obj) Stop() {
	obj.gamepadMu.Lock()

	obj.mu.Lock()
	operation := obj.operation
	if operation == nil {
		obj.mu.Unlock()
		obj.log.Debug().Msg("Remote stop requested while idle")
		obj.gamepad.Stop()
		obj.gamepad.SetOnline(false)
		obj.gamepadMu.Unlock()
		return
	}
	if obj.snapshot.Phase != PhaseDisconnecting {
		obj.snapshot.Phase = PhaseDisconnecting
		obj.notifyLocked()
	}
	operation.cancel()
	done := operation.done
	obj.mu.Unlock()

	obj.log.Info().
		Uint64("generation", operation.generation).
		Msg("Remote disconnect requested")
	obj.gamepad.Stop()
	obj.gamepad.SetOnline(false)
	obj.gamepadMu.Unlock()
	<-done
}
