package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/voluminor/dji-rc-joystick/cli/internal/output"
	"github.com/voluminor/dji-rc-joystick/mod/rc"
	"github.com/voluminor/dji-rc-joystick/target"
)

// // // // // // // // // //

type versionResultObj struct {
	Schema  int    `json:"schema"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Hash    string `json:"hash"`
}

type remoteResultObj struct {
	IP       string `json:"ip"`
	Model    string `json:"model,omitempty"`
	Firmware string `json:"firmware,omitempty"`
}

type discoverResultObj struct {
	Schema  int               `json:"schema"`
	Remotes []remoteResultObj `json:"remotes"`
}

type discoverCallObj struct {
	remotes []rc.RemoteObj
	err     error
}

func executeVersion(options versionOptionsObj, dependencies DependenciesObj) resultObj {
	if options.format == formatJSON {
		err := output.WriteJSON(dependencies.Stdout, versionResultObj{
			Schema: output.SchemaVersion, Name: target.Name, Version: target.Version, Hash: target.Hash,
		})
		if err != nil {
			return resultObj{code: exitRuntime, err: fmt.Errorf("write version: %w", err)}
		}
		return resultObj{code: exitSuccess}
	}

	style, err := styleFor(
		options.color.value, dependencies.StdoutTTY,
		dependencies.NoColor && !options.color.set,
	)
	if err != nil {
		return resultObj{code: exitUsage, err: err}
	}
	_, err = fmt.Fprintf(dependencies.Stdout, "%s %s (%s)\n",
		style.Bold(target.Name), style.Cyan(target.Version), target.Hash)
	if err != nil {
		return resultObj{code: exitRuntime, err: fmt.Errorf("write version: %w", err)}
	}
	return resultObj{code: exitSuccess}
}

func executeDiscover(options discoverOptionsObj, dependencies DependenciesObj) resultObj {
	ctx, cancel := contextWithTimeout(context.Background(), options.timeout)
	defer cancel()

	var view *output.DiscoverViewObj
	config := &rc.DiscoverConfigObj{Port: options.port, CollectInfo: true}
	if options.format == formatText && dependencies.StderrTTY {
		style, err := styleFor(
			options.color.value, true,
			dependencies.NoColor && !options.color.set,
		)
		if err != nil {
			return resultObj{code: exitUsage, err: err}
		}
		view = output.NewDiscoverView(dependencies.Stderr, style, dependencies.Now)
		config.OnProgress = func(progress rc.DiscoverProgressObj) {
			view.Update(progress)
			if view.Err() != nil {
				cancel()
			}
		}
	}

	resultC := make(chan discoverCallObj, 1)
	go func() {
		remotes, err := dependencies.Discover(ctx, config)
		resultC <- discoverCallObj{remotes: remotes, err: err}
	}()

	var signal os.Signal
	var result discoverCallObj
	select {
	case result = <-resultC:
	case signal = <-dependencies.Signals:
		cancel()
		result = <-resultC
	}
	if view != nil {
		if err := view.Close(); err != nil {
			result.err = err
		}
	}
	if signal != nil {
		return resultObj{code: signalExitCode(signal)}
	}
	if result.err != nil {
		if errors.Is(result.err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return resultObj{
				code: exitRuntime, err: fmt.Errorf("discovery timed out after %s", options.timeout),
			}
		}
		return resultObj{code: exitRuntime, err: fmt.Errorf("discover remotes: %w", result.err)}
	}

	sort.Slice(result.remotes, func(left, right int) bool {
		return result.remotes[left].IP < result.remotes[right].IP
	})
	remotes := make([]remoteResultObj, 0, len(result.remotes))
	for _, remote := range result.remotes {
		remotes = append(remotes, remoteResultObj{
			IP: remote.IP, Model: remote.Model, Firmware: remote.Firmware,
		})
	}

	if options.format == formatJSON {
		if err := output.WriteJSON(dependencies.Stdout, discoverResultObj{
			Schema: output.SchemaVersion, Remotes: remotes,
		}); err != nil {
			return resultObj{code: exitRuntime, err: fmt.Errorf("write discovery result: %w", err)}
		}
		return resultObj{code: exitSuccess}
	}

	style, err := styleFor(
		options.color.value, dependencies.StdoutTTY,
		dependencies.NoColor && !options.color.set,
	)
	if err != nil {
		return resultObj{code: exitUsage, err: err}
	}
	if len(remotes) == 0 {
		if _, err = fmt.Fprintln(dependencies.Stdout, "No DJI remotes found."); err != nil {
			return resultObj{code: exitRuntime, err: fmt.Errorf("write discovery result: %w", err)}
		}
		return resultObj{code: exitSuccess}
	}
	for _, remote := range remotes {
		model := remote.Model
		if model == "" {
			model = "-"
		}
		firmware := remote.Firmware
		if firmware == "" {
			firmware = "-"
		}
		if _, err = fmt.Fprintf(dependencies.Stdout, "%s  model=%s  firmware=%s\n",
			style.Cyan(remote.IP), model, firmware); err != nil {
			return resultObj{code: exitRuntime, err: fmt.Errorf("write discovery result: %w", err)}
		}
	}
	return resultObj{code: exitSuccess}
}
