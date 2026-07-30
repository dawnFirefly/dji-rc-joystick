package rc

import (
	"fmt"
	"time"
)

// // // // // // // // // //

func normalizePort(port int) (int, error) {
	if port == 0 {
		return DefaultPort, nil
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("rc: port %d is out of range 1..65535", port)
	}
	return port, nil
}

func clampConnections(n int) int {
	if n < 1 {
		return 1
	}
	if n > MaxConnections {
		return MaxConnections
	}
	return n
}

func fanout[T any](cbs []func(T), value T) {
	for _, cb := range cbs {
		cb(value)
	}
}

func (obj *Obj) publishState(state StateObj, raw RawObj) {
	obj.mu.Lock()
	obj.state = state
	stateCbs := obj.stateCbs
	rawCbs := obj.rawCbs
	obj.mu.Unlock()

	fanout(stateCbs, state)
	fanout(rawCbs, raw)
}

func (obj *Obj) publishSyntheticState(state StateObj) {
	obj.mu.Lock()
	obj.state = state
	stateCbs := obj.stateCbs
	obj.mu.Unlock()

	fanout(stateCbs, state)
}

func (obj *Obj) publishPacket(packet PacketObj) {
	obj.mu.Lock()
	packetCbs := obj.packetCbs
	obj.mu.Unlock()

	fanout(packetCbs, packet)
}

func (obj *Obj) publishLink(event LinkEventObj) {
	obj.mu.Lock()
	linkCbs := obj.linkCbs
	obj.mu.Unlock()

	fanout(linkCbs, event)
}

func (obj *Obj) hasLinkCallbacks() bool {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	return len(obj.linkCbs) > 0
}

// Status snapshots are always fresh, while callbacks are rate-limited.
func (obj *Obj) publishStatus(status StatusObj, lastPub *time.Time, force bool) {
	obj.mu.Lock()
	obj.status = status
	statusCbs := obj.statusCbs
	obj.mu.Unlock()

	if !force && time.Since(*lastPub) < statusMinInterval {
		return
	}
	*lastPub = time.Now()
	fanout(statusCbs, status)
}

func (obj *Obj) feed(state StateObj) error {
	obj.sinkMu.RLock()
	defer obj.sinkMu.RUnlock()
	if obj.sink == nil {
		return nil
	}
	return feedSink(obj.sink, &obj.mapping, state)
}

func (obj *Obj) effectiveKeepalive() KeepaliveMode {
	if obj.config.Keepalive != KeepaliveDefault {
		return obj.config.Keepalive
	}
	if keepalive := obj.Profile().Keepalive; keepalive != KeepaliveDefault {
		return keepalive
	}
	return KeepaliveVersion
}

func (obj *Obj) effectiveConnections() int {
	if obj.config.Connections > 0 {
		return clampConnections(obj.config.Connections)
	}
	return clampConnections(obj.Profile().Connections)
}

func (obj *Obj) setProfile(profile ProfileObj) {
	obj.mu.Lock()
	obj.profile = cloneProfile(profile)
	obj.mu.Unlock()
}

func (obj *Obj) publishInfo(info InfoObj) {
	info = cloneInfo(info)

	obj.mu.Lock()
	obj.info = info
	infoCbs := obj.infoCbs
	obj.mu.Unlock()

	for _, cb := range infoCbs {
		cb(cloneInfo(info))
	}
}
