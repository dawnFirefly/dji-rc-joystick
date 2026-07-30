package windows

import (
	"context"
	"time"

	guiremote "github.com/voluminor/dji-rc-joystick/gui/internal/remote"

	"fyne.io/fyne/v2"
)

// // // // // // // // // //

func (obj *homeObj) hideModal() {
	if obj.modal == nil {
		return
	}
	obj.owner.log.Debug().
		Str("phase", obj.modal.phase.String()).
		Msg("GUI progress dialog closed")
	obj.modal.hide()
	obj.modal = nil
}

func (obj *homeObj) showSearchModal(snapshot guiremote.SnapshotObj) {
	if obj.modal == nil || obj.modal.phase != guiremote.PhaseSearching {
		obj.hideModal()
		obj.modal = newSearchDialog(obj)
	}
	obj.modal.updateSearch(snapshot, obj.owner.lg)
}

func (obj *homeObj) showConnectionModal(
	phase guiremote.PhaseID,
	snapshot guiremote.SnapshotObj,
) {
	if obj.modal != nil && obj.modal.phase == phase {
		return
	}
	obj.hideModal()
	obj.modal = newConnectionDialog(obj, phase, snapshot)
}

func (obj *homeObj) applySnapshot(snapshot guiremote.SnapshotObj) {
	if obj.closed.Load() {
		return
	}
	previousPhase := obj.lastSnapshot.Phase
	obj.lastSnapshot = snapshot
	if snapshot.Phase != previousPhase {
		obj.owner.log.Debug().
			Str("from", previousPhase.String()).
			Str("to", snapshot.Phase.String()).
			Msg("GUI remote phase rendered")
	}

	switch snapshot.Phase {
	case guiremote.PhaseSearching:
		obj.showDisconnected()
		obj.showSearchModal(snapshot)
	case guiremote.PhaseConnecting:
		obj.showDisconnected()
		obj.showConnectionModal(guiremote.PhaseConnecting, snapshot)
	case guiremote.PhaseConnected:
		obj.wasConnected = true
		obj.showConnected()
		obj.disconnectButton.Enable()
		obj.hideModal()
	case guiremote.PhaseReconnecting:
		obj.showConnected()
		obj.showConnectionModal(guiremote.PhaseReconnecting, snapshot)
	case guiremote.PhaseDisconnecting:
		if obj.wasConnected {
			obj.showConnected()
		} else {
			obj.showDisconnected()
		}
		obj.disconnectButton.Disable()
		if obj.modal != nil {
			obj.modal.setCancelling(obj.owner.lg)
		}
	default:
		obj.hideModal()
		obj.showDisconnected()
		obj.disconnectButton.Enable()
		obj.wasConnected = false
	}

	if obj.wasConnected {
		obj.updateConnectionSummary(snapshot)
		obj.info.update(snapshot, obj.owner.lg)
		obj.controls.update(snapshot)
	}
	obj.gamepad.update(snapshot)
	obj.handleResults(snapshot)
	obj.handleError(snapshot)
}

func (obj *homeObj) refreshLoop(ctx context.Context) {
	defer close(obj.updatesDone)
	ticker := time.NewTicker(homeRefreshInterval)
	defer ticker.Stop()
	dirty := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-obj.remote.Updates():
			dirty = true
		case <-ticker.C:
			if !dirty {
				continue
			}
			dirty = false
			snapshot := obj.remote.Snapshot()
			fyne.Do(func() {
				obj.applySnapshot(snapshot)
			})
		}
	}
}

// //

func (obj *homeObj) start() {
	ctx, cancel := context.WithCancel(context.Background())
	obj.cancelUpdates = cancel
	go obj.refreshLoop(ctx)
}

func (obj *homeObj) stop() {
	if !obj.closed.CompareAndSwap(false, true) {
		return
	}
	if obj.cancelUpdates != nil {
		obj.cancelUpdates()
		<-obj.updatesDone
	}
}
