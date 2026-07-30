package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/voluminor/dji-rc-joystick/cli/internal/output"
	"github.com/voluminor/dji-rc-joystick/cli/internal/session"
	"github.com/voluminor/dji-rc-joystick/mod/rc"
)

// // // // // // // // // //

func useRunView(options runtimeOptionsObj, dependencies DependenciesObj) bool {
	return dependencies.StderrTTY &&
		options.logFormat == logFormatText &&
		!options.debug &&
		!options.quiet
}

// //

func executeRun(options runOptionsObj, dependencies DependenciesObj) resultObj {
	logger, err := newRuntimeLogger(options.runtime, dependencies)
	if err != nil {
		return resultObj{code: exitUsage, err: err}
	}

	mapping, err := session.DefaultMapping()
	if options.config != "" {
		mapping, err = session.LoadMapping(options.config)
	}
	if err != nil {
		return resultObj{code: exitUsage, err: fmt.Errorf("load control mapping: %w", err), logger: logger}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	async := newAsyncError(cancel)

	var view *output.RunViewObj
	if useRunView(options.runtime, dependencies) {
		style, styleErr := styleFor(
			options.runtime.color.value, true,
			dependencies.NoColor && !options.runtime.color.set,
		)
		if styleErr != nil {
			return resultObj{code: exitUsage, err: styleErr, logger: logger}
		}
		view = output.NewRunView(dependencies.Stderr, style, cancel)
	}

	sessionOptions := newSessionOptions(options.runtime)
	sessionOptions.Mapping = &mapping
	sessionOptions.CreateDevice = dependencies.CreateDevice
	sessionOptions.OnDevice = func(event session.DeviceEventObj) {
		if view != nil {
			async.report(view.Close())
		}
		switch event.State {
		case "created":
			async.report(logger.Log(output.LevelInfo, "Virtual joystick created", map[string]any{"name": event.Name}))
			if view != nil {
				view.UpdateDevice("ready")
			}
		case "removed":
			async.report(logger.Log(output.LevelWarn, "Virtual joystick removed", map[string]any{"reason": event.Reason}))
			if view != nil {
				view.UpdateDevice("offline")
			}
		}
		if view != nil {
			async.report(view.Err())
		}
	}

	sessionObj, err := session.New(sessionOptions)
	if err != nil {
		return resultObj{code: exitUsage, err: fmt.Errorf("configure session: %w", err), logger: logger}
	}

	announcedOnline := false
	sessionObj.OnStatus(func(status rc.StatusObj) {
		if view != nil {
			view.UpdateStatus(status, sessionObj.Profile().ID)
			async.report(view.Err())
			return
		}
		if announcedOnline || status.LinksOnline == 0 {
			return
		}
		announcedOnline = true
		async.report(logger.Log(output.LevelInfo, "Remote online", map[string]any{
			"ip": status.IP, "links": status.LinksOnline, "target": status.LinksTarget,
		}))
	})
	bindLinkDiagnostics(sessionObj, options.runtime, logger, async)

	async.report(logger.Log(output.LevelInfo, "Waiting for a DJI remote", nil))
	if err = sessionObj.Start(ctx); err != nil {
		if view != nil {
			_ = view.Close()
		}
		return resultObj{code: exitRuntime, err: fmt.Errorf("start session: %w", err), logger: logger}
	}

	wait := waitRuntime(sessionObj, dependencies.Signals, async)
	cancel()
	stopErr := sessionObj.Stop()
	if wait.err == nil {
		wait.err = stopErr
	}
	if view != nil {
		if viewErr := view.Close(); wait.err == nil {
			wait.err = viewErr
		}
	}
	if wait.err == nil {
		wait.err = logger.Log(output.LevelInfo, "Stopped", nil)
	}
	if wait.err != nil {
		return resultObj{code: exitRuntime, err: wait.err, logger: logger}
	}
	if wait.signal != nil {
		return resultObj{code: signalExitCode(wait.signal), logger: logger}
	}
	return resultObj{code: exitRuntime, err: errors.New("run stopped unexpectedly"), logger: logger}
}

func executeMonitor(options runtimeOptionsObj, dependencies DependenciesObj) resultObj {
	if !dependencies.StdoutTTY {
		return resultObj{
			code: exitUsage,
			err:  errors.New("monitor requires an interactive stdout terminal; use events for pipelines"),
		}
	}
	logger, err := newRuntimeLogger(options, dependencies)
	if err != nil {
		return resultObj{code: exitUsage, err: err}
	}
	style, err := styleFor(options.color.value, true, dependencies.NoColor && !options.color.set)
	if err != nil {
		return resultObj{code: exitUsage, err: err, logger: logger}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	async := newAsyncError(cancel)
	dashboard := output.NewDashboard(dependencies.Stdout, style, dependencies.Now)
	dashboardDone := make(chan struct{})
	go func() {
		defer close(dashboardDone)
		async.report(dashboard.Run(ctx))
	}()

	sessionObj, err := session.New(newSessionOptions(options))
	if err != nil {
		cancel()
		<-dashboardDone
		return resultObj{code: exitUsage, err: fmt.Errorf("configure session: %w", err), logger: logger}
	}
	sessionObj.OnStatus(func(status rc.StatusObj) {
		dashboard.UpdateStatus(status, sessionObj.Profile().ID)
	})
	sessionObj.OnState(dashboard.UpdateState)
	sessionObj.OnInfo(dashboard.UpdateInfo)
	bindLinkDiagnostics(sessionObj, options, logger, async)

	if err = sessionObj.Start(ctx); err != nil {
		cancel()
		<-dashboardDone
		return resultObj{code: exitRuntime, err: fmt.Errorf("start session: %w", err), logger: logger}
	}
	wait := waitRuntime(sessionObj, dependencies.Signals, async)
	cancel()
	stopErr := sessionObj.Stop()
	<-dashboardDone
	if wait.err == nil {
		wait.err = stopErr
	}
	if wait.err == nil {
		wait.err = logger.Err()
	}
	if wait.err != nil {
		return resultObj{code: exitRuntime, err: wait.err, logger: logger}
	}
	if wait.signal != nil {
		return resultObj{code: signalExitCode(wait.signal), logger: logger}
	}
	return resultObj{code: exitRuntime, err: errors.New("monitor stopped unexpectedly"), logger: logger}
}
