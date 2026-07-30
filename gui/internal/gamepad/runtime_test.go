package gamepad

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
	"github.com/voluminor/dji-rc-joystick/mod/vjoy"

	"github.com/rs/zerolog"
)

// // // // // // // // // //

type captureDeviceObj struct {
	mu        sync.Mutex
	current   map[uint16]int32
	frames    []map[uint16]int32
	closed    int
	blockNext bool
	entered   chan struct{}
	release   chan struct{}
}

func newCaptureDevice() *captureDeviceObj {
	return &captureDeviceObj{
		current: make(map[uint16]int32),
		entered: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
}

func (obj *captureDeviceObj) Set(handle uint16, value int32) error {
	obj.mu.Lock()
	obj.current[handle] = value
	obj.mu.Unlock()
	return nil
}

func (obj *captureDeviceObj) Sync() error {
	obj.mu.Lock()
	frame := make(map[uint16]int32, len(obj.current))
	for handle, value := range obj.current {
		frame[handle] = value
	}
	obj.frames = append(obj.frames, frame)
	block := obj.blockNext
	obj.blockNext = false
	obj.mu.Unlock()

	if block {
		obj.entered <- struct{}{}
		<-obj.release
	}
	return nil
}

func (obj *captureDeviceObj) Close() error {
	obj.mu.Lock()
	obj.closed++
	obj.mu.Unlock()
	return nil
}

func (obj *captureDeviceObj) setBlockNext() {
	obj.mu.Lock()
	obj.blockNext = true
	obj.mu.Unlock()
}

func (obj *captureDeviceObj) snapshotFrames() []map[uint16]int32 {
	obj.mu.Lock()
	defer obj.mu.Unlock()

	frames := make([]map[uint16]int32, len(obj.frames))
	for index, frame := range obj.frames {
		frames[index] = make(map[uint16]int32, len(frame))
		for handle, value := range frame {
			frames[index][handle] = value
		}
	}
	return frames
}

func (obj *captureDeviceObj) closeCount() int {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	return obj.closed
}

func findControlHandle(
	t *testing.T,
	spec vjoy.SpecObj,
	kind vjoy.ControlKind,
	usage vjoy.Usage,
) uint16 {
	t.Helper()
	for _, control := range spec.Controls {
		if control.Kind == kind && control.Usage == usage {
			return control.Handle
		}
	}
	t.Fatalf("control kind %d usage %d is missing", kind, usage)
	return 0
}

func waitPhase(t *testing.T, obj *Obj, want PhaseID) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if obj.Snapshot().Phase == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("phase = %s, want %s", obj.Snapshot().Phase, want)
}

func waitFrame(
	t *testing.T,
	device *captureDeviceObj,
	match func(map[uint16]int32) bool,
) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		for _, frame := range device.snapshotFrames() {
			if match(frame) {
				return
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("matching frame was not emitted: %#v", device.snapshotFrames())
}

// //

func TestRuntimeKeepsDeviceAcrossRemoteReconnect(t *testing.T) {
	device := newCaptureDevice()
	var creates atomic.Int64
	var spec vjoy.SpecObj
	obj := newWithDependencies(
		dependenciesObj{
			createDevice: func(
				_ context.Context,
				createdSpec vjoy.SpecObj,
			) (vjoy.Interface, error) {
				creates.Add(1)
				spec = createdSpec
				return device, nil
			},
		},
		nil,
	)
	obj.SetOnline(true)
	if err := obj.Start(rc.DefaultMapping(), rc.DefaultProfile()); err != nil {
		t.Fatal(err)
	}
	waitPhase(t, obj, PhaseRunning)

	leftHandle := findControlHandle(t, spec, vjoy.KindAxis, vjoy.UsageX)
	modeHandle := findControlHandle(t, spec, vjoy.KindAxis, vjoy.UsageSlider0)
	state := rc.StateObj{Mode: rc.ModeSport}
	state.Axes[rc.AxisLeftH] = 12345
	state.Buttons = 1 << uint(rc.ButtonRTH)
	obj.Feed(state)
	waitFrame(t, device, func(frame map[uint16]int32) bool {
		return frame[leftHandle] == 12345 && frame[modeHandle] == int32(rc.ModeSport)
	})

	obj.SetOnline(false)
	waitPhase(t, obj, PhaseWaiting)
	waitFrame(t, device, func(frame map[uint16]int32) bool {
		if frame[leftHandle] != 0 ||
			frame[modeHandle] != int32(rc.ModeNormal) {
			return false
		}
		for handle, value := range frame {
			if handle >= 200 && value != 0 {
				return false
			}
		}
		return true
	})

	obj.SetOnline(true)
	waitPhase(t, obj, PhaseRunning)
	state.Axes[rc.AxisLeftH] = -23456
	state.Mode = rc.ModeCinema
	obj.Feed(state)
	waitFrame(t, device, func(frame map[uint16]int32) bool {
		return frame[leftHandle] == -23456 &&
			frame[modeHandle] == int32(rc.ModeCinema)
	})
	if got := creates.Load(); got != 1 {
		t.Fatalf("device creates = %d, want 1", got)
	}

	obj.Stop()
	if obj.Snapshot().Phase != PhaseStopped {
		t.Fatalf("phase after stop = %s", obj.Snapshot().Phase)
	}
	if got := device.closeCount(); got != 1 {
		t.Fatalf("device closes = %d, want 1", got)
	}
}

func TestRuntimeLogsCompletedLinkTransitionsWithoutAnnouncements(t *testing.T) {
	device := newCaptureDevice()
	var output bytes.Buffer
	logger := zerolog.New(zerolog.SyncWriter(&output)).
		Level(zerolog.InfoLevel)
	obj := newWithDependencies(
		dependenciesObj{
			createDevice: func(
				context.Context,
				vjoy.SpecObj,
			) (vjoy.Interface, error) {
				return device, nil
			},
		},
		nil,
		logger,
	)
	obj.SetOnline(true)
	if err := obj.Start(rc.DefaultMapping(), rc.DefaultProfile()); err != nil {
		t.Fatal(err)
	}
	waitPhase(t, obj, PhaseRunning)

	obj.SetOnline(false)
	waitPhase(t, obj, PhaseWaiting)
	obj.SetOnline(true)
	waitPhase(t, obj, PhaseRunning)
	obj.Stop()

	logText := output.String()
	for _, duplicate := range []string{
		"Virtual gamepad input is offline",
		"Virtual gamepad input is online",
	} {
		if strings.Contains(logText, duplicate) {
			t.Fatalf("link transition was announced before completion: %s", logText)
		}
	}
	for _, completed := range []string{
		"Virtual gamepad neutralized after remote link loss",
		"Virtual gamepad resumed after remote reconnect",
		"Virtual gamepad is stopping",
		"Virtual gamepad stopped",
	} {
		if count := strings.Count(logText, completed); count != 1 {
			t.Fatalf("%q records = %d, want 1: %s", completed, count, logText)
		}
	}
}

func TestRuntimeFeedIsBoundedAndKeepsNewestState(t *testing.T) {
	device := newCaptureDevice()
	var spec vjoy.SpecObj
	obj := newWithDependencies(
		dependenciesObj{
			createDevice: func(
				_ context.Context,
				createdSpec vjoy.SpecObj,
			) (vjoy.Interface, error) {
				spec = createdSpec
				return device, nil
			},
		},
		nil,
	)
	obj.SetOnline(true)
	if err := obj.Start(rc.DefaultMapping(), rc.DefaultProfile()); err != nil {
		t.Fatal(err)
	}
	waitPhase(t, obj, PhaseRunning)

	device.setBlockNext()
	first := rc.StateObj{Mode: rc.ModeNormal}
	first.Axes[rc.AxisLeftH] = 1
	obj.Feed(first)
	select {
	case <-device.entered:
	case <-time.After(time.Second):
		t.Fatal("device did not enter the blocked frame")
	}

	feedDone := make(chan struct{})
	go func() {
		for value := int16(2); value < 2000; value++ {
			state := rc.StateObj{Mode: rc.ModeNormal}
			state.Axes[rc.AxisLeftH] = value
			obj.Feed(state)
		}
		close(feedDone)
	}()
	select {
	case <-feedDone:
	case <-time.After(time.Second):
		t.Fatal("Feed blocked while the virtual device was busy")
	}
	close(device.release)

	leftHandle := findControlHandle(t, spec, vjoy.KindAxis, vjoy.UsageX)
	waitFrame(t, device, func(frame map[uint16]int32) bool {
		return frame[leftHandle] == 1999
	})
	obj.Stop()
}

func TestRuntimeReportsCreationFailure(t *testing.T) {
	createErr := errors.New("device unavailable")
	obj := newWithDependencies(
		dependenciesObj{
			createDevice: func(
				context.Context,
				vjoy.SpecObj,
			) (vjoy.Interface, error) {
				return nil, createErr
			},
		},
		nil,
	)
	if err := obj.Start(rc.DefaultMapping(), rc.DefaultProfile()); err != nil {
		t.Fatal(err)
	}
	waitPhase(t, obj, PhaseError)
	snapshot := obj.Snapshot()
	if snapshot.ErrorRevision != 1 ||
		!strings.Contains(snapshot.Error, createErr.Error()) {
		t.Fatalf("creation failure snapshot = %#v", snapshot)
	}
}

func TestRuntimeTreatsCanceledAuthorizationAsStopped(t *testing.T) {
	obj := newWithDependencies(
		dependenciesObj{
			createDevice: func(
				context.Context,
				vjoy.SpecObj,
			) (vjoy.Interface, error) {
				return nil, ErrAuthorizationCanceled
			},
		},
		nil,
	)
	if err := obj.Start(rc.DefaultMapping(), rc.DefaultProfile()); err != nil {
		t.Fatal(err)
	}
	waitPhase(t, obj, PhaseStopped)
	if snapshot := obj.Snapshot(); snapshot.Error != "" {
		t.Fatalf("authorization cancellation error = %q", snapshot.Error)
	}
}

func TestRuntimeStopCancelsDeviceCreation(t *testing.T) {
	entered := make(chan struct{})
	obj := newWithDependencies(
		dependenciesObj{
			createDevice: func(
				ctx context.Context,
				_ vjoy.SpecObj,
			) (vjoy.Interface, error) {
				close(entered)
				<-ctx.Done()
				return nil, ctx.Err()
			},
		},
		nil,
	)
	if err := obj.Start(rc.DefaultMapping(), rc.DefaultProfile()); err != nil {
		t.Fatal(err)
	}
	<-entered
	obj.Stop()
	if obj.Snapshot().Phase != PhaseStopped {
		t.Fatalf("phase after canceled creation = %s", obj.Snapshot().Phase)
	}
}
