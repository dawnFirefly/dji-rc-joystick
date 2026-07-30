package remote

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/voluminor/dji-rc-joystick/gui/internal/gamepad"
	"github.com/voluminor/dji-rc-joystick/mod/rc"
)

// // // // // // // // // //

type fakeGamepadObj struct {
	mu       sync.Mutex
	onChange func(gamepad.SnapshotObj)
	snapshot gamepad.SnapshotObj
	mapping  rc.MappingObj
	profile  rc.ProfileObj
	state    rc.StateObj
	starts   int
	stops    int
	feeds    int
	online   bool

	startEntered chan struct{}
	releaseStart chan struct{}
}

func (obj *fakeGamepadObj) Start(
	mapping rc.MappingObj,
	profile rc.ProfileObj,
) error {
	obj.mu.Lock()
	if obj.snapshot.Phase != gamepad.PhaseStopped {
		obj.mu.Unlock()
		return errors.New("fake gamepad is active")
	}
	obj.mapping = mapping
	obj.profile = profile
	obj.starts++
	if obj.startEntered != nil {
		close(obj.startEntered)
	}
	if obj.releaseStart != nil {
		<-obj.releaseStart
	}
	obj.snapshot.Phase = gamepad.PhaseRunning
	obj.snapshot.Revision++
	snapshot := obj.snapshot
	callback := obj.onChange
	obj.mu.Unlock()
	callback(snapshot)
	return nil
}

func (obj *fakeGamepadObj) Stop() {
	obj.mu.Lock()
	if obj.snapshot.Phase == gamepad.PhaseStopped {
		obj.mu.Unlock()
		return
	}
	obj.stops++
	obj.snapshot.Phase = gamepad.PhaseStopped
	obj.snapshot.Revision++
	snapshot := obj.snapshot
	callback := obj.onChange
	obj.mu.Unlock()
	callback(snapshot)
}

func (obj *fakeGamepadObj) SetOnline(online bool) {
	obj.mu.Lock()
	obj.online = online
	active := obj.snapshot.Phase == gamepad.PhaseRunning ||
		obj.snapshot.Phase == gamepad.PhaseWaiting
	if active {
		if online {
			obj.snapshot.Phase = gamepad.PhaseRunning
		} else {
			obj.snapshot.Phase = gamepad.PhaseWaiting
		}
		obj.snapshot.Revision++
	}
	snapshot := obj.snapshot
	callback := obj.onChange
	obj.mu.Unlock()
	if active {
		callback(snapshot)
	}
}

func (obj *fakeGamepadObj) Feed(state rc.StateObj) {
	obj.mu.Lock()
	if obj.online && obj.snapshot.Phase == gamepad.PhaseRunning {
		obj.state = state
		obj.feeds++
	}
	obj.mu.Unlock()
}

func (obj *fakeGamepadObj) Snapshot() gamepad.SnapshotObj {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	return obj.snapshot
}

func (obj *fakeGamepadObj) values() (
	int,
	int,
	int,
	bool,
	rc.MappingObj,
	rc.ProfileObj,
	rc.StateObj,
) {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	return obj.starts,
		obj.stops,
		obj.feeds,
		obj.online,
		obj.mapping,
		obj.profile,
		obj.state
}

// //

func TestGamepadRequiresConnectedRemote(t *testing.T) {
	obj := newWithDependencies(dependenciesObj{})
	if err := obj.StartGamepad(rc.DefaultMapping()); err == nil {
		t.Fatal("StartGamepad() accepted an idle remote")
	}
}

func TestGamepadSurvivesReconnectAndStopsWithRemote(t *testing.T) {
	profile, ok := rc.ProfileByID("rc2")
	if !ok {
		t.Fatal("DJI RC 2 profile is missing")
	}
	controller := newFakeController(profile)
	virtual := &fakeGamepadObj{
		snapshot: gamepad.SnapshotObj{Phase: gamepad.PhaseStopped},
	}
	obj := newWithDependencies(dependenciesObj{
		newController: func(rc.ConfigObj) (controllerInterface, error) {
			return controller, nil
		},
		newGamepad: func(
			onChange func(gamepad.SnapshotObj),
		) gamepad.Interface {
			virtual.onChange = onChange
			return virtual
		},
	})

	if err := obj.Connect("192.168.7.2", rc.DefaultPort); err != nil {
		t.Fatal(err)
	}
	<-controller.started
	controller.emitStatus(rc.StatusObj{
		Phase: rc.PhaseConnected,
		IP:    "192.168.7.2",
	})

	mapping := rc.DefaultMapping()
	if err := obj.StartGamepad(mapping); err != nil {
		t.Fatal(err)
	}
	state := rc.StateObj{Mode: rc.ModeNormal}
	state.Axes[rc.AxisLeftH] = 123
	controller.emitState(state)

	controller.emitStatus(rc.StatusObj{
		Phase: rc.PhaseConnecting,
		IP:    "192.168.7.2",
	})
	if snapshot := obj.Snapshot(); snapshot.Gamepad.Phase != gamepad.PhaseWaiting {
		t.Fatalf("gamepad phase during reconnect = %s", snapshot.Gamepad.Phase)
	}
	starts, stops, feeds, online, gotMapping, gotProfile, gotState := virtual.values()
	if starts != 1 || stops != 0 || feeds != 1 || online {
		t.Fatalf(
			"gamepad before reconnect: starts=%d stops=%d feeds=%d online=%t",
			starts,
			stops,
			feeds,
			online,
		)
	}
	if gotMapping.Axes[rc.AxisLeftH] != mapping.Axes[rc.AxisLeftH] ||
		gotProfile.ID != profile.ID ||
		gotState.Axes[rc.AxisLeftH] != 123 {
		t.Fatal("gamepad did not receive the active mapping, profile, and state")
	}

	controller.emitStatus(rc.StatusObj{
		Phase: rc.PhaseConnected,
		IP:    "192.168.7.2",
	})
	if snapshot := obj.Snapshot(); snapshot.Gamepad.Phase != gamepad.PhaseRunning {
		t.Fatalf("gamepad phase after reconnect = %s", snapshot.Gamepad.Phase)
	}
	starts, stops, _, online, _, _, _ = virtual.values()
	if starts != 1 || stops != 0 || !online {
		t.Fatalf(
			"gamepad after reconnect: starts=%d stops=%d online=%t",
			starts,
			stops,
			online,
		)
	}

	stopRuntime(t, obj)
	starts, stops, _, online, _, _, _ = virtual.values()
	if starts != 1 || stops != 1 || online {
		t.Fatalf(
			"gamepad after remote stop: starts=%d stops=%d online=%t",
			starts,
			stops,
			online,
		)
	}
}

func TestConcurrentGamepadStartCannotSurviveRemoteStop(t *testing.T) {
	controller := newFakeController(rc.DefaultProfile())
	virtual := &fakeGamepadObj{
		snapshot:     gamepad.SnapshotObj{Phase: gamepad.PhaseStopped},
		startEntered: make(chan struct{}),
		releaseStart: make(chan struct{}),
	}
	obj := newWithDependencies(dependenciesObj{
		newController: func(rc.ConfigObj) (controllerInterface, error) {
			return controller, nil
		},
		newGamepad: func(
			onChange func(gamepad.SnapshotObj),
		) gamepad.Interface {
			virtual.onChange = onChange
			return virtual
		},
	})
	if err := obj.Connect("192.168.7.2", rc.DefaultPort); err != nil {
		t.Fatal(err)
	}
	<-controller.started
	controller.emitStatus(rc.StatusObj{
		Phase: rc.PhaseConnected,
		IP:    "192.168.7.2",
	})

	startResult := make(chan error, 1)
	go func() {
		startResult <- obj.StartGamepad(rc.DefaultMapping())
	}()
	<-virtual.startEntered

	stopDone := make(chan struct{})
	go func() {
		obj.Stop()
		close(stopDone)
	}()
	close(virtual.releaseStart)
	if err := <-startResult; err != nil {
		t.Fatalf("StartGamepad() error = %v", err)
	}
	select {
	case <-stopDone:
	case <-time.After(2 * time.Second):
		t.Fatal("remote stop timed out after concurrent gamepad start")
	}

	snapshot := obj.Snapshot()
	if snapshot.Phase != PhaseIdle ||
		snapshot.Gamepad.Phase != gamepad.PhaseStopped {
		t.Fatalf("snapshot after concurrent start and stop = %+v", snapshot)
	}
	_, stops, _, _, _, _, _ := virtual.values()
	if stops != 1 {
		t.Fatalf("gamepad stops = %d, want 1", stops)
	}
}
