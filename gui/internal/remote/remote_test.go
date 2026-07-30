package remote

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/voluminor/dji-rc-joystick/mod/rc"

	"github.com/rs/zerolog"
)

// // // // // // // // // //

type fakeControllerObj struct {
	mu       sync.Mutex
	profile  rc.ProfileObj
	startErr error
	onState  func(rc.StateObj)
	onStatus func(rc.StatusObj)
	onInfo   func(rc.InfoObj)
	started  chan struct{}
	stopped  chan struct{}
	start    sync.Once
	stop     sync.Once
}

// //

func newFakeController(profile rc.ProfileObj) *fakeControllerObj {
	return &fakeControllerObj{
		profile: profile,
		started: make(chan struct{}),
		stopped: make(chan struct{}),
	}
}

func (obj *fakeControllerObj) Start(_ context.Context) error {
	obj.start.Do(func() {
		close(obj.started)
	})
	return obj.startErr
}

func (obj *fakeControllerObj) Stop() {
	obj.stop.Do(func() {
		close(obj.stopped)
	})
}

func (obj *fakeControllerObj) OnState(callback func(rc.StateObj)) {
	obj.mu.Lock()
	obj.onState = callback
	obj.mu.Unlock()
}

func (obj *fakeControllerObj) OnStatus(callback func(rc.StatusObj)) {
	obj.mu.Lock()
	obj.onStatus = callback
	obj.mu.Unlock()
}

func (obj *fakeControllerObj) OnInfo(callback func(rc.InfoObj)) {
	obj.mu.Lock()
	obj.onInfo = callback
	obj.mu.Unlock()
}

func (obj *fakeControllerObj) Profile() rc.ProfileObj {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	return cloneProfile(obj.profile)
}

func (obj *fakeControllerObj) emitState(state rc.StateObj) {
	obj.mu.Lock()
	callback := obj.onState
	obj.mu.Unlock()
	if callback != nil {
		callback(state)
	}
}

func (obj *fakeControllerObj) emitStatus(status rc.StatusObj) {
	obj.mu.Lock()
	callback := obj.onStatus
	obj.mu.Unlock()
	if callback != nil {
		callback(status)
	}
}

func (obj *fakeControllerObj) emitInfo(info rc.InfoObj) {
	obj.mu.Lock()
	callback := obj.onInfo
	obj.mu.Unlock()
	if callback != nil {
		callback(info)
	}
}

func waitSnapshot(t *testing.T, obj *Obj, predicate func(SnapshotObj) bool) SnapshotObj {
	t.Helper()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for {
		snapshot := obj.Snapshot()
		if predicate(snapshot) {
			return snapshot
		}
		select {
		case <-obj.Updates():
		case <-timer.C:
			t.Fatalf("timed out waiting for remote snapshot: %+v", snapshot)
		}
	}
}

func stopRuntime(t *testing.T, obj *Obj) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		obj.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("remote stop timed out")
	}
}

func runtimeWithController(controller *fakeControllerObj) *Obj {
	return newWithDependencies(dependenciesObj{
		discover: rc.Discover,
		newController: func(rc.ConfigObj) (controllerInterface, error) {
			return controller, nil
		},
	})
}

// //

func TestSearchPublishesProgressAndResults(t *testing.T) {
	var captured rc.DiscoverConfigObj
	obj := newWithDependencies(dependenciesObj{
		discover: func(_ context.Context, config *rc.DiscoverConfigObj) ([]rc.RemoteObj, error) {
			captured = *config
			config.OnProgress(rc.DiscoverProgressObj{
				Known:  true,
				Probed: 2,
				Total:  4,
				Found:  1,
			})
			return []rc.RemoteObj{{
				IP:       "192.168.7.2",
				Model:    "DJI RC 2",
				Firmware: "01.02.03.04",
			}}, nil
		},
		newController: func(rc.ConfigObj) (controllerInterface, error) {
			return nil, errors.New("unexpected controller creation")
		},
	})

	if err := obj.Search(); err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	snapshot := waitSnapshot(t, obj, func(snapshot SnapshotObj) bool {
		return snapshot.Phase == PhaseIdle && snapshot.SearchRevision == 1
	})

	if captured.Port != rc.DefaultPort || !captured.CollectInfo {
		t.Fatalf("discovery config = %+v", captured)
	}
	if snapshot.SearchCount != 1 || len(snapshot.Remotes) != 1 {
		t.Fatalf("search result = %+v", snapshot)
	}
	if snapshot.Remotes[0].IP != "192.168.7.2" {
		t.Fatalf("remote IP = %q", snapshot.Remotes[0].IP)
	}
}

func TestSearchCancellationIsNotAnError(t *testing.T) {
	started := make(chan struct{})
	obj := newWithDependencies(dependenciesObj{
		discover: func(ctx context.Context, _ *rc.DiscoverConfigObj) ([]rc.RemoteObj, error) {
			close(started)
			<-ctx.Done()
			return []rc.RemoteObj{{IP: "192.168.7.2"}}, ctx.Err()
		},
	})

	if err := obj.Search(); err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	<-started
	stopRuntime(t, obj)

	snapshot := obj.Snapshot()
	if snapshot.Phase != PhaseIdle ||
		snapshot.SearchRevision != 0 ||
		len(snapshot.Remotes) != 0 ||
		snapshot.ErrorRevision != 0 ||
		snapshot.Error != "" {
		t.Fatalf("snapshot after cancellation = %+v", snapshot)
	}
}

func TestSearchAcceptsPartialResults(t *testing.T) {
	obj := newWithDependencies(dependenciesObj{
		discover: func(ctx context.Context, config *rc.DiscoverConfigObj) ([]rc.RemoteObj, error) {
			config.OnProgress(rc.DiscoverProgressObj{
				Known:  true,
				Probed: 1,
				Total:  4,
				Found:  1,
			})
			<-ctx.Done()
			return []rc.RemoteObj{{
				IP:       "192.168.7.2",
				Model:    "DJI RC 2",
				Firmware: "01.02.03.04",
			}}, ctx.Err()
		},
	})

	if err := obj.Search(); err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	waitSnapshot(t, obj, func(snapshot SnapshotObj) bool {
		return snapshot.Phase == PhaseSearching && snapshot.Progress.Found == 1
	})
	if err := obj.AcceptFound(); err != nil {
		t.Fatalf("AcceptFound() error = %v", err)
	}

	snapshot := obj.Snapshot()
	if snapshot.Phase != PhaseIdle ||
		snapshot.SearchRevision != 1 ||
		snapshot.SearchCount != 1 ||
		len(snapshot.Remotes) != 1 ||
		snapshot.Remotes[0].IP != "192.168.7.2" ||
		snapshot.ErrorRevision != 0 {
		t.Fatalf("accepted partial search = %+v", snapshot)
	}
}

func TestAcceptFoundRequiresActiveResults(t *testing.T) {
	obj := newWithDependencies(dependenciesObj{})
	if !errors.Is(obj.AcceptFound(), errNoSearchResults) {
		t.Fatal("AcceptFound() accepted an idle runtime")
	}
}

func TestEmptySearchPreservesPreviousResults(t *testing.T) {
	calls := 0
	obj := newWithDependencies(dependenciesObj{
		discover: func(context.Context, *rc.DiscoverConfigObj) ([]rc.RemoteObj, error) {
			calls++
			if calls == 1 {
				return []rc.RemoteObj{{IP: "192.168.7.2"}}, nil
			}
			return nil, nil
		},
	})

	if err := obj.Search(); err != nil {
		t.Fatalf("first Search() error = %v", err)
	}
	waitSnapshot(t, obj, func(snapshot SnapshotObj) bool {
		return snapshot.Phase == PhaseIdle && snapshot.SearchRevision == 1
	})

	if err := obj.Search(); err != nil {
		t.Fatalf("second Search() error = %v", err)
	}
	snapshot := waitSnapshot(t, obj, func(snapshot SnapshotObj) bool {
		return snapshot.Phase == PhaseIdle && snapshot.SearchRevision == 2
	})
	if snapshot.SearchCount != 0 || len(snapshot.Remotes) != 1 {
		t.Fatalf("empty search result = %+v", snapshot)
	}
}

func TestConnectionTransitionsAndLiveSnapshots(t *testing.T) {
	controller := newFakeController(rc.ProfileObj{
		ID:        "rc2",
		Name:      "DJI RC 2",
		DeviceIDs: []string{"rc331"},
	})
	obj := runtimeWithController(controller)

	if err := obj.Connect("192.168.7.2", rc.DefaultPort); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	<-controller.started

	controller.emitStatus(rc.StatusObj{
		Phase:       rc.PhaseConnected,
		IP:          "192.168.7.2",
		DeviceID:    "rc331",
		LinksOnline: 1,
		LinksTarget: 5,
		Rate:        30.5,
	})
	controller.emitState(rc.StateObj{
		Axes: [rc.AxisCount]int16{1, 2, 3, 4, 5, 6},
		Mode: rc.ModeNormal,
	})
	controller.emitInfo(rc.InfoObj{
		Name:     "rc331",
		Model:    "DJI RC 2",
		Firmware: "01.02.03.04",
		Battery: rc.BatteryObj{
			Decoded:   true,
			Percent:   75,
			VoltageMV: 4100,
		},
		Telemetry: []rc.TelemetryObj{{CmdSet: 6, CmdID: 30}},
	})

	snapshot := waitSnapshot(t, obj, func(snapshot SnapshotObj) bool {
		return snapshot.Phase == PhaseConnected &&
			snapshot.Info.Battery.Percent == 75 &&
			snapshot.State.Axes[rc.AxisRightV] == 4
	})
	if snapshot.Profile.ID != "rc2" || snapshot.Status.LinksOnline != 1 {
		t.Fatalf("connected snapshot = %+v", snapshot)
	}
	if len(snapshot.Info.Telemetry) != 0 {
		t.Fatalf("GUI snapshot retained %d telemetry entries", len(snapshot.Info.Telemetry))
	}

	controller.emitStatus(rc.StatusObj{
		Phase:       rc.PhaseConnected,
		IP:          "192.168.7.2",
		LinksOnline: 0,
		LinksTarget: 5,
	})
	waitSnapshot(t, obj, func(snapshot SnapshotObj) bool {
		return snapshot.Phase == PhaseConnected && snapshot.Status.LinksOnline == 0
	})

	controller.emitStatus(rc.StatusObj{
		Phase:       rc.PhaseConnecting,
		IP:          "192.168.7.2",
		LinksOnline: 0,
		LinksTarget: 5,
	})
	waitSnapshot(t, obj, func(snapshot SnapshotObj) bool {
		return snapshot.Phase == PhaseReconnecting
	})

	controller.emitStatus(rc.StatusObj{
		Phase:       rc.PhaseConnected,
		IP:          "192.168.7.2",
		LinksOnline: 2,
		LinksTarget: 5,
	})
	waitSnapshot(t, obj, func(snapshot SnapshotObj) bool {
		return snapshot.Phase == PhaseConnected && snapshot.Status.LinksOnline == 2
	})

	stopRuntime(t, obj)
	select {
	case <-controller.stopped:
	default:
		t.Fatal("controller was not stopped")
	}
}

func TestAcceptedStatesEachProduceOneTraceRecord(t *testing.T) {
	controller := newFakeController(rc.DefaultProfile())
	var output bytes.Buffer
	logger := zerolog.New(&output).Level(zerolog.TraceLevel)
	obj := newWithDependencies(dependenciesObj{
		newController: func(rc.ConfigObj) (controllerInterface, error) {
			return controller, nil
		},
	}, logger)

	if err := obj.Connect("192.168.7.2", rc.DefaultPort); err != nil {
		t.Fatal(err)
	}
	<-controller.started

	first := rc.StateObj{
		Axes:    [rc.AxisCount]int16{1, 2, 3, 4, 5, 6},
		Buttons: 1 << uint(rc.ButtonRTH),
		Mode:    rc.ModeSport,
	}
	second := rc.StateObj{
		Axes:    [rc.AxisCount]int16{-1, -2, -3, -4, -5, -6},
		Buttons: 1 << uint(rc.ButtonC2),
		Mode:    rc.ModeCinema,
	}
	controller.emitState(first)
	controller.emitState(second)
	stopRuntime(t, obj)
	controller.emitState(rc.StateObj{Mode: rc.ModeNormal})

	logText := output.String()
	if count := strings.Count(logText, `"message":"Decoded controller state"`); count != 2 {
		t.Fatalf("decoded-state trace records = %d, want 2: %s", count, logText)
	}
	for _, field := range []string{
		`"left_stick_h":1`,
		`"right_wheel":6`,
		`"mode":"sport"`,
		`"rth":true`,
		`"c2":true`,
	} {
		if !strings.Contains(logText, field) {
			t.Fatalf("trace output does not contain %s: %s", field, logText)
		}
	}
}

func TestCancelledControllerCallbacksCannotMutateNewSession(t *testing.T) {
	first := newFakeController(rc.ProfileObj{ID: "first"})
	second := newFakeController(rc.ProfileObj{ID: "second"})
	controllers := []controllerInterface{first, second}
	obj := newWithDependencies(dependenciesObj{
		newController: func(rc.ConfigObj) (controllerInterface, error) {
			controller := controllers[0]
			controllers = controllers[1:]
			return controller, nil
		},
	})

	if err := obj.Connect("192.168.7.2", rc.DefaultPort); err != nil {
		t.Fatalf("first Connect() error = %v", err)
	}
	<-first.started
	stopRuntime(t, obj)

	if err := obj.Connect("192.168.7.3", rc.DefaultPort); err != nil {
		t.Fatalf("second Connect() error = %v", err)
	}
	<-second.started

	first.emitState(rc.StateObj{Axes: [rc.AxisCount]int16{99}})
	second.emitState(rc.StateObj{Axes: [rc.AxisCount]int16{7}})
	snapshot := waitSnapshot(t, obj, func(snapshot SnapshotObj) bool {
		return snapshot.State.Axes[0] == 7
	})
	if snapshot.TargetIP != "192.168.7.3" {
		t.Fatalf("target IP = %q", snapshot.TargetIP)
	}
	stopRuntime(t, obj)
}

func TestConcurrentConnectStopsDiscardedController(t *testing.T) {
	first := newFakeController(rc.ProfileObj{ID: "first"})
	second := newFakeController(rc.ProfileObj{ID: "second"})
	firstFactoryEntered := make(chan struct{})
	releaseFirstFactory := make(chan struct{})
	var calls atomic.Int64
	obj := newWithDependencies(dependenciesObj{
		newController: func(rc.ConfigObj) (controllerInterface, error) {
			if calls.Add(1) == 1 {
				close(firstFactoryEntered)
				<-releaseFirstFactory
				return first, nil
			}
			return second, nil
		},
	})

	firstResult := make(chan error, 1)
	go func() {
		firstResult <- obj.Connect("192.168.7.2", rc.DefaultPort)
	}()
	<-firstFactoryEntered

	if err := obj.Connect("192.168.7.3", rc.DefaultPort); err != nil {
		t.Fatalf("winning Connect() error = %v", err)
	}
	<-second.started
	close(releaseFirstFactory)

	if err := <-firstResult; !errors.Is(err, errOperationActive) {
		t.Fatalf("discarded Connect() error = %v", err)
	}
	select {
	case <-first.stopped:
	default:
		t.Fatal("discarded controller was not stopped")
	}
	stopRuntime(t, obj)
}

func TestStartErrorReturnsToIdle(t *testing.T) {
	controller := newFakeController(rc.ProfileObj{})
	controller.startErr = errors.New("boom")
	obj := runtimeWithController(controller)

	if err := obj.Connect("192.168.7.2", rc.DefaultPort); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	snapshot := waitSnapshot(t, obj, func(snapshot SnapshotObj) bool {
		return snapshot.Phase == PhaseIdle && snapshot.ErrorRevision == 1
	})
	if !strings.Contains(snapshot.Error, "start remote: boom") {
		t.Fatalf("error = %q", snapshot.Error)
	}
	select {
	case <-controller.stopped:
	default:
		t.Fatal("controller was not stopped after start error")
	}
}

func TestSnapshotsAndNotificationsStayBoundedUnderConcurrentState(t *testing.T) {
	controller := newFakeController(rc.ProfileObj{ID: "rc2"})
	obj := runtimeWithController(controller)

	if err := obj.Connect("192.168.7.2", rc.DefaultPort); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	<-controller.started

	var wait sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wait.Add(1)
		go func(offset int) {
			defer wait.Done()
			for value := 0; value < 500; value++ {
				controller.emitState(rc.StateObj{
					Axes: [rc.AxisCount]int16{int16(offset + value)},
				})
				_ = obj.Snapshot()
			}
		}(worker)
	}
	wait.Wait()

	if cap(obj.updates) != 1 || len(obj.updates) > 1 {
		t.Fatalf("updates capacity=%d length=%d", cap(obj.updates), len(obj.updates))
	}
	stopRuntime(t, obj)
}

func TestValidateTarget(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		port int
		ok   bool
	}{
		{name: "valid", ip: " 192.168.7.2 ", port: 40007, ok: true},
		{name: "invalid IP", ip: "controller", port: 40007},
		{name: "zero port", ip: "192.168.7.2", port: 0},
		{name: "large port", ip: "192.168.7.2", port: 65536},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ip, err := validateTarget(test.ip, test.port)
			if test.ok {
				if err != nil || ip != "192.168.7.2" {
					t.Fatalf("validateTarget() = %q, %v", ip, err)
				}
				return
			}
			if err == nil {
				t.Fatal("validateTarget() error = nil")
			}
		})
	}
}
