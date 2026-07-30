package remote

import (
	"context"
	"fmt"

	"github.com/voluminor/dji-rc-joystick/mod/rc"
)

// // // // // // // // // //

func (obj *Obj) observeProgress(operation *operationObj, progress rc.DiscoverProgressObj) {
	obj.mu.Lock()
	if !obj.activeLocked(operation) || obj.snapshot.Phase != PhaseSearching {
		obj.mu.Unlock()
		return
	}
	obj.snapshot.Progress = progress
	obj.notifyLocked()
	obj.mu.Unlock()

	obj.log.Trace().
		Bool("known", progress.Known).
		Str("network", progress.Network).
		Int("probed", progress.Probed).
		Int("total", progress.Total).
		Int("found", progress.Found).
		Msg("Remote discovery progressed")
}

func (obj *Obj) finishSearch(
	operation *operationObj,
	remotes []rc.RemoteObj,
	err error,
) {
	applied := false
	acceptedFound := false
	obj.mu.Lock()
	if obj.activeLocked(operation) {
		applied = true
		obj.operation = nil
		obj.snapshot.Phase = PhaseIdle
		obj.snapshot.Progress = rc.DiscoverProgressObj{}
		acceptedFound = operation.acceptFound &&
			errorsIsCancellation(err) &&
			len(remotes) > 0
		if err == nil || acceptedFound {
			obj.snapshot.SearchRevision++
			obj.snapshot.SearchCount = len(remotes)
			if len(remotes) > 0 {
				obj.snapshot.Remotes = append([]rc.RemoteObj(nil), remotes...)
			}
		}
		obj.setErrorLocked(err)
		obj.notifyLocked()
	}
	obj.mu.Unlock()
	if applied {
		event := obj.log.Info()
		message := "Remote discovery finished"
		if err != nil && !errorsIsCancellation(err) {
			event = obj.log.Error().Err(err)
			message = "Remote discovery failed"
		} else if errorsIsCancellation(err) && !acceptedFound {
			message = "Remote discovery canceled"
		}
		event.
			Int("remotes", len(remotes)).
			Bool("accepted_partial", acceptedFound).
			Msg(message)
	}
	close(operation.done)
}

func (obj *Obj) runSearch(ctx context.Context, operation *operationObj) {
	remotes, err := obj.dependencies.discover(ctx, &rc.DiscoverConfigObj{
		Port:        rc.DefaultPort,
		CollectInfo: true,
		OnProgress: func(progress rc.DiscoverProgressObj) {
			obj.observeProgress(operation, progress)
		},
	})
	if err != nil && !errorsIsCancellation(err) {
		err = fmt.Errorf("discover remotes: %w", err)
	}
	obj.finishSearch(operation, remotes, err)
}

// //

// Search starts explicit discovery on the standard DJI RC port.
func (obj *Obj) Search() error {
	obj.mu.Lock()
	if obj.operation != nil {
		obj.mu.Unlock()
		return errOperationActive
	}

	operation, ctx := obj.beginOperationLocked()
	obj.snapshot.Phase = PhaseSearching
	obj.snapshot.Progress = rc.DiscoverProgressObj{}
	obj.snapshot.Error = ""
	obj.notifyLocked()
	obj.mu.Unlock()

	obj.log.Info().
		Uint64("generation", operation.generation).
		Msg("Remote discovery started")
	go obj.runSearch(ctx, operation)
	return nil
}

// AcceptFound stops discovery and keeps the remotes accumulated so far.
func (obj *Obj) AcceptFound() error {
	obj.mu.Lock()
	operation := obj.operation
	if operation == nil ||
		obj.snapshot.Phase != PhaseSearching ||
		obj.snapshot.Progress.Found == 0 {
		obj.mu.Unlock()
		return errNoSearchResults
	}
	operation.acceptFound = true
	operation.cancel()
	done := operation.done
	obj.mu.Unlock()

	obj.log.Info().
		Uint64("generation", operation.generation).
		Msg("Partial remote discovery results accepted")
	<-done
	return nil
}
