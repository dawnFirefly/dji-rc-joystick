package session

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/voluminor/dji-rc-joystick/mod/rc"
	"github.com/voluminor/dji-rc-joystick/mod/vjoy"
)

// // // // // // // // // //

// OptionsObj contains validated transport and lifecycle settings.
type OptionsObj struct {
	IP                string
	Port              int
	ProfileID         string
	Keepalive         rc.KeepaliveMode
	Handshake         bool
	Connections       int
	QueryInfo         bool
	ConnectTimeout    time.Duration
	DisconnectTimeout time.Duration

	CreateDevice func(vjoy.SpecObj) (vjoy.Interface, error)
	Mapping      *rc.MappingObj
	OnDevice     func(DeviceEventObj)
}

// DeviceEventObj describes a virtual-device lifecycle transition.
type DeviceEventObj struct {
	State  string
	Name   string
	Reason string
}

// Obj owns one RC run, timeout monitoring, and an optional virtual device.
type Obj struct {
	controller *rc.Obj
	options    OptionsObj
	errorsC    chan error
	connection connectionStateObj
	timeoutC   chan struct{}
	deviceC    chan struct{}

	mu      sync.Mutex
	started bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	err     error
}

// New validates options and constructs a stopped session.
func New(options OptionsObj) (*Obj, error) {
	if options.ConnectTimeout < 0 {
		return nil, errors.New("connect timeout cannot be negative")
	}
	if options.DisconnectTimeout < 0 {
		return nil, errors.New("disconnect timeout cannot be negative")
	}
	if options.Mapping != nil && options.CreateDevice == nil {
		options.CreateDevice = vjoy.New
	}

	controller, err := rc.New(rc.ConfigObj{
		IP: options.IP, Port: options.Port, ProfileID: options.ProfileID,
		Keepalive: options.Keepalive, Handshake: options.Handshake,
		Connections: options.Connections, QueryInfo: options.QueryInfo,
		Mapping: options.Mapping,
	})
	if err != nil {
		return nil, err
	}

	obj := &Obj{
		controller: controller, options: options,
		timeoutC: make(chan struct{}, 1), deviceC: make(chan struct{}, 1),
		errorsC: make(chan error, 1),
	}
	controller.OnStatus(obj.observeStatus)
	return obj, nil
}

func (obj *Obj) report(err error) {
	if err == nil {
		return
	}
	select {
	case obj.errorsC <- err:
	default:
	}
	obj.mu.Lock()
	if obj.err == nil {
		obj.err = err
	}
	cancel := obj.cancel
	obj.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Start begins the RC pipeline and all configured lifecycle monitors.
func (obj *Obj) Start(parent context.Context) error {
	if parent == nil {
		return errors.New("session context is nil")
	}
	obj.mu.Lock()
	if obj.started {
		obj.mu.Unlock()
		return errors.New("session is already started")
	}
	ctx, cancel := context.WithCancel(parent)
	obj.started, obj.cancel = true, cancel
	obj.mu.Unlock()

	if obj.options.Mapping != nil {
		obj.startDevice(ctx)
	}
	if err := obj.controller.Start(ctx); err != nil {
		cancel()
		obj.wg.Wait()
		obj.mu.Lock()
		obj.started, obj.cancel = false, nil
		obj.mu.Unlock()
		return err
	}
	obj.wg.Add(1)
	go obj.watchTimeouts(ctx)
	return nil
}

// Stop cancels the session, waits for workers, and closes its virtual device.
func (obj *Obj) Stop() error {
	obj.mu.Lock()
	if !obj.started {
		err := obj.err
		obj.mu.Unlock()
		return err
	}
	cancel := obj.cancel
	obj.mu.Unlock()

	cancel()
	obj.controller.Stop()
	obj.wg.Wait()

	obj.mu.Lock()
	obj.started = false
	obj.cancel = nil
	err := obj.err
	obj.mu.Unlock()
	return err
}

// Errors reports the first asynchronous timeout or virtual-device failure.
func (obj *Obj) Errors() <-chan error {
	return obj.errorsC
}

// Snapshot returns the latest decoded state and aggregate status.
func (obj *Obj) Snapshot() (rc.StateObj, rc.StatusObj) {
	return obj.controller.Snapshot()
}

// Profile returns an independent copy of the active remote profile.
func (obj *Obj) Profile() rc.ProfileObj {
	return obj.controller.Profile()
}

// OnState registers a serialized decoded-state callback.
func (obj *Obj) OnState(callback func(rc.StateObj)) {
	obj.controller.OnState(callback)
}

// OnStatus registers a serialized aggregate-status callback.
func (obj *Obj) OnStatus(callback func(rc.StatusObj)) {
	obj.controller.OnStatus(callback)
}

// OnInfo registers a serialized remote-information callback.
func (obj *Obj) OnInfo(callback func(rc.InfoObj)) {
	obj.controller.OnInfo(callback)
}

// OnRaw registers a serialized raw-channel callback.
func (obj *Obj) OnRaw(callback func(rc.RawObj)) {
	obj.controller.OnRaw(callback)
}

// OnPacket registers a serialized DUML-packet callback.
func (obj *Obj) OnPacket(callback func(rc.PacketObj)) {
	obj.controller.OnPacket(callback)
}

// OnLink registers a serialized reader-link diagnostic callback.
func (obj *Obj) OnLink(callback func(rc.LinkEventObj)) {
	obj.controller.OnLink(callback)
}
