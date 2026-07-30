package app

import (
	"context"
	"os"
	"sync"

	"github.com/voluminor/dji-rc-joystick/cli/internal/output"
	"github.com/voluminor/dji-rc-joystick/cli/internal/session"
	"github.com/voluminor/dji-rc-joystick/mod/rc"
)

// // // // // // // // // //

type asyncErrorObj struct {
	once   sync.Once
	cancel context.CancelFunc
	errors chan error
}

func newAsyncError(cancel context.CancelFunc) *asyncErrorObj {
	return &asyncErrorObj{cancel: cancel, errors: make(chan error, 1)}
}

func (obj *asyncErrorObj) report(err error) {
	if err == nil {
		return
	}
	obj.once.Do(func() {
		obj.errors <- err
		obj.cancel()
	})
}

type waitResultObj struct {
	signal os.Signal
	err    error
}

func waitRuntime(sessionObj *session.Obj, signals <-chan os.Signal, async *asyncErrorObj) waitResultObj {
	for {
		select {
		case signal, open := <-signals:
			if !open {
				signals = nil
				continue
			}
			return waitResultObj{signal: signal}
		case err := <-sessionObj.Errors():
			return waitResultObj{err: err}
		case err := <-async.errors:
			return waitResultObj{err: err}
		}
	}
}

func newRuntimeLogger(options runtimeOptionsObj, dependencies DependenciesObj) (*output.LoggerObj, error) {
	level, err := output.ParseLevel(options.logLevel.value)
	if err != nil {
		return nil, err
	}
	style, err := styleFor(
		options.color.value, dependencies.StderrTTY,
		dependencies.NoColor && !options.color.set,
	)
	if err != nil {
		return nil, err
	}
	jsonFormat := options.logFormat == logFormatJSON
	if jsonFormat {
		style = output.NewStyle(false)
	}
	return output.NewLogger(dependencies.Stderr, dependencies.Now, style, level, jsonFormat), nil
}

func newSessionOptions(options runtimeOptionsObj) session.OptionsObj {
	keepalive, _ := parseKeepalive(options.keepalive)
	return session.OptionsObj{
		IP: options.ip, Port: options.port, ProfileID: options.profile,
		Keepalive: keepalive, Handshake: options.handshake,
		Connections: options.connections, QueryInfo: options.queryInfo,
		ConnectTimeout: options.connectTimeout, DisconnectTimeout: options.disconnectTimeout,
	}
}

func remoteSnapshot(sessionObj *session.Obj) output.RemoteObj {
	_, status := sessionObj.Snapshot()
	return output.RemoteObj{
		IP: status.IP, DeviceID: status.DeviceID, Profile: sessionObj.Profile().ID,
	}
}

func bindLinkDiagnostics(
	sessionObj *session.Obj,
	options runtimeOptionsObj,
	logger *output.LoggerObj,
	async *asyncErrorObj,
) {
	if !options.debug {
		return
	}
	sessionObj.OnLink(func(event rc.LinkEventObj) {
		fields := map[string]any{
			"index": event.Index, "online": event.Online, "operation": event.Operation,
		}
		if event.Error != nil {
			fields["error"] = event.Error.Error()
		}
		async.report(logger.Log(output.LevelDebug, "Reader link event", fields))
	})
}
