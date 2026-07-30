package windows

import (
	"net"
	"strconv"

	guiremote "github.com/voluminor/dji-rc-joystick/gui/internal/remote"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// // // // // // // // // //

func formatRemoteEndpoint(snapshot guiremote.SnapshotObj) string {
	ip := snapshot.Status.IP
	if ip == "" {
		ip = snapshot.TargetIP
	}
	if ip == "" {
		return ""
	}
	if snapshot.TargetPort < 1 {
		return ip
	}
	return net.JoinHostPort(ip, strconv.Itoa(snapshot.TargetPort))
}

func connectionImportance(phase guiremote.PhaseID) widget.Importance {
	switch phase {
	case guiremote.PhaseConnected:
		return widget.SuccessImportance
	case guiremote.PhaseReconnecting:
		return widget.WarningImportance
	case guiremote.PhaseDisconnecting:
		return widget.DangerImportance
	default:
		return widget.MediumImportance
	}
}

// //

func (obj *homeObj) buildConnected() *fyne.Container {
	obj.connectionStatus = widget.NewLabel("")
	obj.connectionStatus.TextStyle = fyne.TextStyle{Bold: true}
	obj.connectionEndpoint = widget.NewLabel("")
	obj.connectionEndpoint.Importance = widget.LowImportance
	obj.connectionEndpoint.TextStyle = fyne.TextStyle{Monospace: true}

	toolbar := container.NewBorder(
		nil,
		nil,
		container.NewHBox(obj.connectionStatus, obj.connectionEndpoint),
		obj.disconnectButton,
	)
	columns := container.NewGridWithColumns(
		2,
		obj.info.root,
		obj.controls.root,
	)
	controllerBlock := widget.NewCard(
		"",
		"",
		container.NewVBox(toolbar, widget.NewSeparator(), columns),
	)
	return container.NewVBox(controllerBlock, obj.gamepad.root)
}

func (obj *homeObj) updateConnectionSummary(snapshot guiremote.SnapshotObj) {
	status := localizedPhase(snapshot.Phase, obj.owner.lg)
	importance := connectionImportance(snapshot.Phase)
	if obj.connectionStatus.Text != status ||
		obj.connectionStatus.Importance != importance {
		obj.connectionStatus.Importance = importance
		obj.connectionStatus.SetText(status)
	}

	endpoint := formatRemoteEndpoint(snapshot)
	if obj.connectionEndpoint.Text != endpoint {
		obj.connectionEndpoint.SetText(endpoint)
	}
}
