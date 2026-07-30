package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/voluminor/dji-rc-joystick/mod/rc"
)

// // // // // // // // // //

type connectionStateObj struct {
	mu           sync.Mutex
	status       rc.StatusObj
	everOnline   bool
	online       bool
	offlineSince time.Time
}

type connectionSnapshotObj struct {
	status       rc.StatusObj
	everOnline   bool
	online       bool
	offlineSince time.Time
}

func notifyLatest(channel chan struct{}) {
	select {
	case channel <- struct{}{}:
	default:
	}
}

func (obj *Obj) observeStatus(status rc.StatusObj) {
	obj.connection.mu.Lock()
	wasOnline := obj.connection.online
	obj.connection.status = status
	obj.connection.online = status.LinksOnline > 0
	if obj.connection.online {
		obj.connection.everOnline = true
		obj.connection.offlineSince = time.Time{}
	} else if wasOnline {
		obj.connection.offlineSince = time.Now()
	}
	obj.connection.mu.Unlock()
	notifyLatest(obj.timeoutC)
	if obj.options.Mapping != nil {
		notifyLatest(obj.deviceC)
	}
}

func (obj *Obj) connectionSnapshot() connectionSnapshotObj {
	obj.connection.mu.Lock()
	defer obj.connection.mu.Unlock()
	return connectionSnapshotObj{
		status: obj.connection.status, everOnline: obj.connection.everOnline,
		online: obj.connection.online, offlineSince: obj.connection.offlineSince,
	}
}

func stopTimer(timer **time.Timer, timerC *<-chan time.Time) {
	if *timer != nil {
		(*timer).Stop()
		*timer = nil
	}
	*timerC = nil
}

func resetTimer(timer **time.Timer, timerC *<-chan time.Time, duration time.Duration) {
	stopTimer(timer, timerC)
	if duration > 0 {
		*timer = time.NewTimer(duration)
		*timerC = (*timer).C
	}
}

func (obj *Obj) watchTimeouts(ctx context.Context) {
	defer obj.wg.Done()
	var connectTimer, disconnectTimer *time.Timer
	var connectTimerC, disconnectTimerC <-chan time.Time
	if obj.options.ConnectTimeout > 0 {
		connectTimer = time.NewTimer(obj.options.ConnectTimeout)
		connectTimerC = connectTimer.C
	}
	defer stopTimer(&connectTimer, &connectTimerC)
	defer stopTimer(&disconnectTimer, &disconnectTimerC)

	connected := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-connectTimerC:
			state := obj.connectionSnapshot()
			if state.everOnline {
				connected = true
				stopTimer(&connectTimer, &connectTimerC)
				if state.online {
					continue
				}
				if obj.options.DisconnectTimeout == 0 {
					continue
				}
				if remaining := obj.options.DisconnectTimeout - time.Since(state.offlineSince); remaining > 0 {
					resetTimer(&disconnectTimer, &disconnectTimerC, remaining)
					continue
				}
				obj.report(fmt.Errorf("remote stayed offline for %s", obj.options.DisconnectTimeout))
				return
			}
			obj.report(fmt.Errorf("remote did not connect within %s", obj.options.ConnectTimeout))
			return
		case <-disconnectTimerC:
			state := obj.connectionSnapshot()
			if state.online {
				stopTimer(&disconnectTimer, &disconnectTimerC)
				continue
			}
			if remaining := obj.options.DisconnectTimeout - time.Since(state.offlineSince); remaining > 0 {
				resetTimer(&disconnectTimer, &disconnectTimerC, remaining)
				continue
			}
			obj.report(fmt.Errorf("remote stayed offline for %s", obj.options.DisconnectTimeout))
			return
		case <-obj.timeoutC:
			state := obj.connectionSnapshot()
			if state.everOnline {
				connected = true
				stopTimer(&connectTimer, &connectTimerC)
			}
			if state.online {
				stopTimer(&disconnectTimer, &disconnectTimerC)
				continue
			}
			if connected && obj.options.DisconnectTimeout > 0 {
				remaining := obj.options.DisconnectTimeout - time.Since(state.offlineSince)
				if remaining <= 0 {
					obj.report(fmt.Errorf("remote stayed offline for %s", obj.options.DisconnectTimeout))
					return
				}
				resetTimer(&disconnectTimer, &disconnectTimerC, remaining)
			}
		}
	}
}
