package session

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/voluminor/dji-rc-joystick/mod/rc"
	"github.com/voluminor/dji-rc-joystick/mod/vjoy"
)

// // // // // // // // // //

const (
	identityGrace = 6 * time.Second
	offlineGrace  = 2 * time.Second
)

type deviceSinkObj struct {
	mu      sync.Mutex
	device  vjoy.Interface
	errorsC chan error
}

func newDeviceSink() *deviceSinkObj {
	return &deviceSinkObj{errorsC: make(chan error, 1)}
}

func (obj *deviceSinkObj) report(err error) error {
	if err == nil {
		return nil
	}
	select {
	case obj.errorsC <- err:
	default:
	}
	return err
}

func (obj *deviceSinkObj) hasDevice() bool {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	return obj.device != nil
}

func (obj *deviceSinkObj) install(device vjoy.Interface) error {
	if device == nil {
		return errors.New("virtual device is nil")
	}
	obj.mu.Lock()
	defer obj.mu.Unlock()
	if obj.device != nil {
		return errors.New("virtual device is already installed")
	}
	obj.device = device
	return nil
}

func (obj *deviceSinkObj) Set(handle uint16, value int32) error {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	if obj.device == nil {
		return nil
	}
	return obj.report(obj.device.Set(handle, value))
}

func (obj *deviceSinkObj) Sync() error {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	if obj.device == nil {
		return nil
	}
	return obj.report(obj.device.Sync())
}

func (obj *deviceSinkObj) Close() error {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	if obj.device == nil {
		return nil
	}
	device := obj.device
	obj.device = nil
	return device.Close()
}

type deviceRunnerObj struct {
	session *Obj
	sink    *deviceSinkObj
	ready   bool
}

func (obj *deviceRunnerObj) notify(event DeviceEventObj) {
	if obj.session.options.OnDevice != nil {
		obj.session.options.OnDevice(event)
	}
}

func (obj *deviceRunnerObj) deviceName() string {
	_, status := obj.session.controller.Snapshot()
	if status.DeviceID != "" {
		if _, known := rc.ProfileByDeviceID(status.DeviceID); !known {
			return "DJI RC (" + status.DeviceID + ")"
		}
	}
	return obj.session.controller.Profile().Name
}

func (obj *deviceRunnerObj) create() error {
	if obj.sink.hasDevice() {
		return nil
	}
	name := obj.deviceName()
	spec := obj.session.options.Mapping.DeviceSpec(obj.session.controller.Profile())
	spec.Name = name
	device, err := obj.session.options.CreateDevice(spec)
	if err != nil {
		return fmt.Errorf("create virtual joystick: %w", err)
	}
	if err = obj.sink.install(device); err != nil {
		_ = device.Close()
		return fmt.Errorf("install virtual joystick: %w", err)
	}
	obj.notify(DeviceEventObj{State: "created", Name: name})
	return nil
}

func (obj *deviceRunnerObj) run(ctx context.Context) (resultErr error) {
	defer func() {
		if err := obj.sink.Close(); resultErr == nil && err != nil {
			resultErr = fmt.Errorf("close virtual joystick: %w", err)
		}
	}()

	var identityTimer, offlineTimer *time.Timer
	var identityTimerC, offlineTimerC <-chan time.Time
	var offlineSince time.Time
	defer stopTimer(&identityTimer, &identityTimerC)
	defer stopTimer(&offlineTimer, &offlineTimerC)
	obj.ready = obj.session.options.ProfileID != ""

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-obj.sink.errorsC:
			return fmt.Errorf("update virtual joystick: %w", err)
		case <-identityTimerC:
			obj.ready = true
			stopTimer(&identityTimer, &identityTimerC)
			if obj.session.connectionSnapshot().online {
				if err := obj.create(); err != nil {
					return err
				}
			}
		case <-offlineTimerC:
			stopTimer(&offlineTimer, &offlineTimerC)
			state := obj.session.connectionSnapshot()
			if state.online {
				offlineSince = time.Time{}
				continue
			}
			if remaining := offlineGrace - time.Since(state.offlineSince); remaining > 0 {
				offlineSince = state.offlineSince
				resetTimer(&offlineTimer, &offlineTimerC, remaining)
				continue
			}
			if err := obj.sink.Close(); err != nil {
				return fmt.Errorf("remove virtual joystick: %w", err)
			}
			obj.notify(DeviceEventObj{
				State: "removed", Reason: "all remote connections were offline for 2 seconds",
			})
		case <-obj.session.deviceC:
			state := obj.session.connectionSnapshot()
			status := state.status
			if status.DeviceID != "" {
				obj.ready = true
				stopTimer(&identityTimer, &identityTimerC)
			}
			if state.online {
				stopTimer(&offlineTimer, &offlineTimerC)
				offlineSince = time.Time{}
				if obj.ready {
					if err := obj.create(); err != nil {
						return err
					}
				} else if identityTimer == nil {
					resetTimer(&identityTimer, &identityTimerC, identityGrace)
				}
				continue
			}
			if obj.sink.hasDevice() && !state.offlineSince.IsZero() &&
				(offlineTimer == nil || state.offlineSince != offlineSince) {
				offlineSince = state.offlineSince
				remaining := offlineGrace - time.Since(state.offlineSince)
				if remaining <= 0 {
					remaining = time.Nanosecond
				}
				resetTimer(&offlineTimer, &offlineTimerC, remaining)
			}
		}
	}
}

func (obj *Obj) startDevice(ctx context.Context) {
	sink := newDeviceSink()
	obj.controller.AttachSink(sink)
	runner := &deviceRunnerObj{session: obj, sink: sink}
	obj.wg.Add(1)
	go func() {
		defer obj.wg.Done()
		obj.report(runner.run(ctx))
	}()
}
