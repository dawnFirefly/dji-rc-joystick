package session

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/voluminor/dji-rc-joystick/mod/rc"
)

// // // // // // // // // //

func newTimeoutSession(options OptionsObj) (*Obj, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	obj := &Obj{
		options: options, timeoutC: make(chan struct{}, 1),
		errorsC: make(chan error, 1), cancel: cancel,
	}
	obj.wg.Add(1)
	go obj.watchTimeouts(ctx)
	return obj, cancel
}

// //

func TestConnectTimeout(t *testing.T) {
	obj, cancel := newTimeoutSession(OptionsObj{ConnectTimeout: 30 * time.Millisecond})
	defer func() {
		cancel()
		obj.wg.Wait()
	}()

	select {
	case err := <-obj.Errors():
		if !strings.Contains(err.Error(), "did not connect") {
			t.Fatalf("unexpected timeout error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("connect timeout did not fire")
	}
}

func TestDisconnectTimeoutResetsAfterRecovery(t *testing.T) {
	timeout := 80 * time.Millisecond
	obj, cancel := newTimeoutSession(OptionsObj{DisconnectTimeout: timeout})
	defer func() {
		cancel()
		obj.wg.Wait()
	}()

	obj.observeStatus(rc.StatusObj{LinksOnline: 1})
	obj.observeStatus(rc.StatusObj{LinksOnline: 0})
	time.Sleep(50 * time.Millisecond)
	obj.observeStatus(rc.StatusObj{LinksOnline: 1})
	obj.observeStatus(rc.StatusObj{LinksOnline: 0})

	select {
	case err := <-obj.Errors():
		t.Fatalf("disconnect timeout was not reset: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	select {
	case err := <-obj.Errors():
		if !strings.Contains(err.Error(), "stayed offline") {
			t.Fatalf("unexpected timeout error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("disconnect timeout did not fire after the reset")
	}
}

func TestConnectTimeoutRemainsSatisfiedAfterCoalescedShortConnection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	obj := &Obj{
		options:  OptionsObj{ConnectTimeout: 30 * time.Millisecond},
		timeoutC: make(chan struct{}, 1), errorsC: make(chan error, 1), cancel: cancel,
	}
	obj.observeStatus(rc.StatusObj{LinksOnline: 1})
	obj.observeStatus(rc.StatusObj{LinksOnline: 0})
	obj.wg.Add(1)
	go obj.watchTimeouts(ctx)
	defer func() {
		cancel()
		obj.wg.Wait()
	}()

	select {
	case err := <-obj.Errors():
		t.Fatalf("connect timeout fired after a real connection: %v", err)
	case <-time.After(60 * time.Millisecond):
	}
}
