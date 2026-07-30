package session

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/voluminor/dji-rc-joystick/mod/rc"
	"github.com/voluminor/dji-rc-joystick/mod/vjoy"
)

// // // // // // // // // //

type testDeviceObj struct {
	mu         sync.Mutex
	closes     int
	setStarted chan struct{}
	releaseSet chan struct{}
	startOnce  sync.Once
}

func (obj *testDeviceObj) Set(uint16, int32) error {
	if obj.setStarted != nil {
		obj.startOnce.Do(func() { close(obj.setStarted) })
		<-obj.releaseSet
	}
	return nil
}

func (obj *testDeviceObj) Sync() error {
	return nil
}

func (obj *testDeviceObj) Close() error {
	obj.mu.Lock()
	obj.closes++
	obj.mu.Unlock()
	return nil
}

func (obj *testDeviceObj) closeCount() int {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	return obj.closes
}

func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition was not met before the deadline")
		}
		time.Sleep(time.Millisecond)
	}
}

// //

func TestDeviceSinkSerializesCloseWithSet(t *testing.T) {
	device := &testDeviceObj{setStarted: make(chan struct{}), releaseSet: make(chan struct{})}
	sink := newDeviceSink()
	if err := sink.install(device); err != nil {
		t.Fatal(err)
	}

	setResultC := make(chan error, 1)
	go func() { setResultC <- sink.Set(1, 1) }()
	<-device.setStarted

	closeResultC := make(chan error, 1)
	go func() { closeResultC <- sink.Close() }()
	time.Sleep(20 * time.Millisecond)
	if device.closeCount() != 0 {
		t.Fatal("device closed while Set was still running")
	}

	close(device.releaseSet)
	if err := <-setResultC; err != nil {
		t.Fatal(err)
	}
	if err := <-closeResultC; err != nil {
		t.Fatal(err)
	}
}

func TestDeviceRunnerRetainsRemovesAndRecreates(t *testing.T) {
	mapping, err := DefaultMapping()
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var devices []*testDeviceObj
	events := make(chan DeviceEventObj, 8)
	sessionObj, err := New(OptionsObj{
		ProfileID: "rc", Mapping: &mapping,
		CreateDevice: func(vjoy.SpecObj) (vjoy.Interface, error) {
			device := &testDeviceObj{}
			mu.Lock()
			devices = append(devices, device)
			mu.Unlock()
			return device, nil
		},
		OnDevice: func(event DeviceEventObj) { events <- event },
	})
	if err != nil {
		t.Fatal(err)
	}
	deviceCount := func() int {
		mu.Lock()
		defer mu.Unlock()
		return len(devices)
	}
	deviceAt := func(index int) *testDeviceObj {
		mu.Lock()
		defer mu.Unlock()
		return devices[index]
	}

	ctx, cancel := context.WithCancel(context.Background())
	resultC := make(chan error, 1)
	sink := newDeviceSink()
	runner := &deviceRunnerObj{session: sessionObj, sink: sink}
	go func() { resultC <- runner.run(ctx) }()

	sessionObj.observeStatus(rc.StatusObj{LinksOnline: 1})
	waitForCondition(t, time.Second, func() bool { return deviceCount() == 1 })
	first := deviceAt(0)

	sessionObj.observeStatus(rc.StatusObj{LinksOnline: 0})
	time.Sleep(50 * time.Millisecond)
	sessionObj.observeStatus(rc.StatusObj{LinksOnline: 1})
	time.Sleep(offlineGrace + 50*time.Millisecond)
	if first.closeCount() != 0 {
		t.Fatal("device was removed during a recovered short outage")
	}

	sessionObj.observeStatus(rc.StatusObj{LinksOnline: 0})
	waitForCondition(t, offlineGrace+time.Second, func() bool { return first.closeCount() == 1 })
	sessionObj.observeStatus(rc.StatusObj{LinksOnline: 1})
	waitForCondition(t, time.Second, func() bool { return deviceCount() == 2 })
	second := deviceAt(1)

	cancel()
	select {
	case err := <-resultC:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("device runner did not stop")
	}
	if first.closeCount() != 1 || second.closeCount() != 1 {
		t.Fatalf("unexpected close counts: first=%d second=%d", first.closeCount(), second.closeCount())
	}
}
