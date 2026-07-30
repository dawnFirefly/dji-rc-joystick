package windows

import (
	"fmt"

	guiremote "github.com/voluminor/dji-rc-joystick/gui/internal/remote"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// // // // // // // // // //

type progressDialogObj struct {
	phase        guiremote.PhaseID
	dialog       *dialog.CustomDialog
	stage        *widget.Label
	found        *widget.Label
	progress     *widget.ProgressBar
	infinite     *widget.ProgressBarInfinite
	acceptButton *widget.Button
	cancelButton *widget.Button
}

// //

func newCancelButton(home *homeObj) *widget.Button {
	return widget.NewButtonWithIcon(
		home.owner.lg("button.cancel", "Cancel"),
		theme.CancelIcon(),
		home.cancelOperation,
	)
}

func newSearchDialog(home *homeObj) *progressDialogObj {
	home.owner.log.Debug().
		Str("dialog", "remote_search").
		Msg("GUI progress dialog opened")
	stage := widget.NewLabel("")
	stage.Wrapping = fyne.TextWrapWord
	found := widget.NewLabel("")
	progress := widget.NewProgressBar()
	acceptButton := widget.NewButtonWithIcon(
		home.owner.lg("home.search.accept_found", "Stop searching and continue"),
		theme.ConfirmIcon(),
		home.acceptFound,
	)
	acceptButton.Importance = widget.HighImportance
	acceptButton.Hide()
	cancelButton := newCancelButton(home)
	custom := dialog.NewCustomWithoutButtons(
		home.owner.lg("home.search.title", "Searching for remotes"),
		container.NewVBox(stage, progress, found),
		home.owner.window,
	)
	custom.SetButtons([]fyne.CanvasObject{acceptButton, cancelButton})
	custom.Resize(fyne.NewSize(520, 220))
	custom.Show()

	return &progressDialogObj{
		phase:        guiremote.PhaseSearching,
		dialog:       custom,
		stage:        stage,
		found:        found,
		progress:     progress,
		acceptButton: acceptButton,
		cancelButton: cancelButton,
	}
}

func newConnectionDialog(
	home *homeObj,
	phase guiremote.PhaseID,
	snapshot guiremote.SnapshotObj,
) *progressDialogObj {
	home.owner.log.Debug().
		Str("dialog", "remote_connection").
		Str("phase", phase.String()).
		Msg("GUI progress dialog opened")
	title := home.owner.lg("home.connect.title", "Connecting to remote")
	messageFormat := home.owner.lg(
		"home.connect.message",
		"Connecting to %s:%d.",
	)
	if phase == guiremote.PhaseReconnecting {
		title = home.owner.lg("home.reconnect.title", "Reconnecting to remote")
		messageFormat = home.owner.lg(
			"home.reconnect.message",
			"Connection to %s:%d was lost. Reconnecting.",
		)
	}

	stage := widget.NewLabel(fmt.Sprintf(
		messageFormat,
		snapshot.TargetIP,
		snapshot.TargetPort,
	))
	stage.Wrapping = fyne.TextWrapWord
	infinite := widget.NewProgressBarInfinite()
	cancelButton := newCancelButton(home)
	custom := dialog.NewCustomWithoutButtons(
		title,
		container.NewVBox(stage, infinite),
		home.owner.window,
	)
	custom.SetButtons([]fyne.CanvasObject{cancelButton})
	custom.Resize(fyne.NewSize(520, 200))
	custom.Show()
	infinite.Start()

	return &progressDialogObj{
		phase:        phase,
		dialog:       custom,
		stage:        stage,
		infinite:     infinite,
		cancelButton: cancelButton,
	}
}

func (obj *progressDialogObj) updateSearch(
	progress guiremote.SnapshotObj,
	lg func(string, string) string,
) {
	stage := progress.Progress.Network
	if progress.Progress.Known {
		stage = lg("home.search.known", "Known addresses")
	} else if stage != "" {
		stage = fmt.Sprintf(
			lg("home.search.network", "Network: %s"),
			stage,
		)
	}
	obj.stage.SetText(fmt.Sprintf(
		"%s\n%s",
		stage,
		fmt.Sprintf(
			lg("home.search.progress", "Probed: %d / %d"),
			progress.Progress.Probed,
			progress.Progress.Total,
		),
	))
	obj.found.SetText(fmt.Sprintf(
		lg("home.search.found", "Found: %d"),
		progress.Progress.Found,
	))
	if progress.Progress.Found > 0 {
		obj.acceptButton.Show()
	} else {
		obj.acceptButton.Hide()
	}
	value := 0.0
	if progress.Progress.Total > 0 {
		value = float64(progress.Progress.Probed) / float64(progress.Progress.Total)
	}
	obj.progress.SetValue(value)
}

func (obj *progressDialogObj) setCancelling(lg func(string, string) string) {
	if obj.acceptButton != nil {
		obj.acceptButton.Disable()
	}
	obj.cancelButton.SetText(lg("home.operation.cancelling", "Cancelling..."))
	obj.cancelButton.Disable()
}

func (obj *progressDialogObj) setAccepting(lg func(string, string) string) {
	if obj.acceptButton != nil {
		obj.acceptButton.SetText(lg("home.search.accepting", "Finishing search..."))
		obj.acceptButton.Disable()
	}
	obj.cancelButton.Disable()
}

func (obj *progressDialogObj) hide() {
	if obj.infinite != nil {
		obj.infinite.Stop()
	}
	obj.dialog.Hide()
}
