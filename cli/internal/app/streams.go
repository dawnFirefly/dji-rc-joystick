package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/voluminor/dji-rc-joystick/cli/internal/output"
	"github.com/voluminor/dji-rc-joystick/cli/internal/session"
	"github.com/voluminor/dji-rc-joystick/mod/rc"
)

// // // // // // // // // //

func eventFilter(values []string) map[string]bool {
	if len(values) == 0 {
		return map[string]bool{"state": true, "status": true, "info": true}
	}
	filter := make(map[string]bool, len(values))
	for _, value := range values {
		filter[value] = true
	}
	return filter
}

func executeEvents(options eventOptionsObj, dependencies DependenciesObj) resultObj {
	logger, err := newRuntimeLogger(options.runtime, dependencies)
	if err != nil {
		return resultObj{code: exitUsage, err: err}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	async := newAsyncError(cancel)
	writer := output.NewWriter(dependencies.Stdout, dependencies.Now, cancel)

	sessionObj, err := session.New(newSessionOptions(options.runtime))
	if err != nil {
		_ = writer.Close()
		return resultObj{code: exitUsage, err: fmt.Errorf("configure session: %w", err), logger: logger}
	}
	filter := eventFilter(options.events.values)
	if filter["state"] {
		sessionObj.OnState(func(state rc.StateObj) {
			_ = writer.Enqueue(output.StateEvent(remoteSnapshot(sessionObj), state))
		})
	}
	if filter["status"] {
		sessionObj.OnStatus(func(status rc.StatusObj) {
			remote := output.RemoteObj{
				IP: status.IP, DeviceID: status.DeviceID, Profile: sessionObj.Profile().ID,
			}
			_ = writer.Enqueue(output.StatusEvent(remote, status))
		})
	}
	if filter["info"] {
		sessionObj.OnInfo(func(info rc.InfoObj) {
			_ = writer.Enqueue(output.InfoEvent(remoteSnapshot(sessionObj), info))
		})
	}
	bindLinkDiagnostics(sessionObj, options.runtime, logger, async)

	if err = sessionObj.Start(ctx); err != nil {
		_ = writer.Close()
		return resultObj{code: exitRuntime, err: fmt.Errorf("start session: %w", err), logger: logger}
	}

	var wait waitResultObj
	select {
	case signal := <-dependencies.Signals:
		wait.signal = signal
	case err = <-sessionObj.Errors():
		wait.err = err
	case err = <-async.errors:
		wait.err = err
	case <-writer.Failed():
		wait.err = writer.Err()
	}

	cancel()
	stopErr := sessionObj.Stop()
	if wait.err == nil {
		wait.err = stopErr
	}
	if writerErr := writer.Close(); wait.err == nil {
		wait.err = writerErr
	}
	if output.IsBrokenPipe(wait.err) {
		return resultObj{code: exitSuccess, logger: logger}
	}
	if wait.err != nil {
		return resultObj{code: exitRuntime, err: wait.err, logger: logger}
	}
	if wait.signal != nil {
		return resultObj{code: signalExitCode(wait.signal), logger: logger}
	}
	return resultObj{code: exitRuntime, err: errors.New("event stream stopped unexpectedly"), logger: logger}
}

type textStreamObj struct {
	mu    sync.Mutex
	dst   io.Writer
	async *asyncErrorObj
	err   error
}

func (obj *textStreamObj) raw(raw rc.RawObj) {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	if obj.err == nil {
		obj.err = output.WriteRawText(obj.dst, raw)
		obj.async.report(obj.err)
	}
}

func (obj *textStreamObj) packet(packet rc.PacketObj) {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	if obj.err == nil {
		obj.err = output.WritePacketText(obj.dst, packet)
		obj.async.report(obj.err)
	}
}

func (obj *textStreamObj) failure() error {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	return obj.err
}

func executeInspect(options inspectOptionsObj, dependencies DependenciesObj) resultObj {
	logger, err := newRuntimeLogger(options.runtime, dependencies)
	if err != nil {
		return resultObj{code: exitUsage, err: err}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	async := newAsyncError(cancel)

	sessionObj, err := session.New(newSessionOptions(options.runtime))
	if err != nil {
		return resultObj{code: exitUsage, err: fmt.Errorf("configure session: %w", err), logger: logger}
	}

	var writer *output.WriterObj
	var text *textStreamObj
	if options.format == formatNDJSON {
		writer = output.NewWriter(dependencies.Stdout, dependencies.Now, cancel)
		sessionObj.OnRaw(func(raw rc.RawObj) {
			_ = writer.Enqueue(output.RawEvent(remoteSnapshot(sessionObj), raw))
		})
		sessionObj.OnPacket(func(packet rc.PacketObj) {
			_ = writer.Enqueue(output.PacketEvent(remoteSnapshot(sessionObj), packet))
		})
	} else {
		text = &textStreamObj{dst: dependencies.Stdout, async: async}
		sessionObj.OnRaw(text.raw)
		sessionObj.OnPacket(text.packet)
	}
	bindLinkDiagnostics(sessionObj, options.runtime, logger, async)

	if err = sessionObj.Start(ctx); err != nil {
		if writer != nil {
			_ = writer.Close()
		}
		return resultObj{code: exitRuntime, err: fmt.Errorf("start session: %w", err), logger: logger}
	}

	var wait waitResultObj
	if writer != nil {
		select {
		case signal := <-dependencies.Signals:
			wait.signal = signal
		case err = <-sessionObj.Errors():
			wait.err = err
		case err = <-async.errors:
			wait.err = err
		case <-writer.Failed():
			wait.err = writer.Err()
		}
	} else {
		wait = waitRuntime(sessionObj, dependencies.Signals, async)
	}

	cancel()
	stopErr := sessionObj.Stop()
	if wait.err == nil {
		wait.err = stopErr
	}
	if writer != nil {
		if writerErr := writer.Close(); wait.err == nil {
			wait.err = writerErr
		}
	} else if wait.err == nil {
		wait.err = text.failure()
	}
	if output.IsBrokenPipe(wait.err) {
		return resultObj{code: exitSuccess, logger: logger}
	}
	if wait.err != nil {
		return resultObj{code: exitRuntime, err: wait.err, logger: logger}
	}
	if wait.signal != nil {
		return resultObj{code: signalExitCode(wait.signal), logger: logger}
	}
	return resultObj{code: exitRuntime, err: errors.New("inspect stopped unexpectedly"), logger: logger}
}
