package windows

import (
	"fmt"
	"strconv"

	guiremote "github.com/voluminor/dji-rc-joystick/gui/internal/remote"
	"github.com/voluminor/dji-rc-joystick/mod/rc"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// // // // // // // // // //

type infoRowObj struct {
	key      string
	fallback string
	title    *widget.Label
	value    *widget.Label
}

type homeInfoObj struct {
	root *fyne.Container
	rows map[string]*infoRowObj
}

// //

func newInfoRow(key, fallback string) *infoRowObj {
	value := widget.NewLabel("")
	value.Wrapping = fyne.TextWrapBreak
	return &infoRowObj{
		key:      key,
		fallback: fallback,
		title:    widget.NewLabel(fallback),
		value:    value,
	}
}

func newHomeInfo() *homeInfoObj {
	rows := []*infoRowObj{
		newInfoRow("home.info.ip", "IP address:"),
		newInfoRow("home.info.model", "Model:"),
		newInfoRow("home.info.firmware", "Firmware:"),
		newInfoRow("home.info.device_id", "Device ID:"),
		newInfoRow("home.info.profile", "Profile:"),
		newInfoRow("home.info.battery", "Battery:"),
		newInfoRow("home.info.links", "Links:"),
		newInfoRow("home.info.rate", "Frame rate:"),
		newInfoRow("home.info.errors", "Errors:"),
		newInfoRow("home.info.mode", "Flight mode:"),
	}

	rowMap := make(map[string]*infoRowObj, len(rows))
	objects := make([]fyne.CanvasObject, 0, 2*len(rows))
	for _, row := range rows {
		rowMap[row.key] = row
		objects = append(objects, row.title, row.value)
	}
	return &homeInfoObj{
		root: container.NewGridWithColumns(2, objects...),
		rows: rowMap,
	}
}

func (obj *homeInfoObj) set(key, value string) {
	label := obj.rows[key].value
	if label.Text != value {
		label.SetText(value)
	}
}

func localizedPhase(phase guiremote.PhaseID, lg func(string, string) string) string {
	switch phase {
	case guiremote.PhaseSearching:
		return lg("home.status.searching", "Searching")
	case guiremote.PhaseConnecting:
		return lg("home.status.connecting", "Connecting")
	case guiremote.PhaseConnected:
		return lg("home.status.connected", "Connected")
	case guiremote.PhaseReconnecting:
		return lg("home.status.reconnecting", "Reconnecting")
	case guiremote.PhaseDisconnecting:
		return lg("home.status.disconnecting", "Disconnecting")
	default:
		return lg("home.status.disconnected", "Disconnected")
	}
}

func localizedMode(mode rc.ModeID, lg func(string, string) string) string {
	switch mode {
	case rc.ModeSport:
		return lg("home.mode.sport", "Sport")
	case rc.ModeNormal:
		return lg("home.mode.normal", "Normal")
	case rc.ModeCinema:
		return lg("home.mode.cinema", "Cine")
	default:
		return lg("home.value.unknown", "Unknown")
	}
}

func formatProfile(profile rc.ProfileObj, unknown string) string {
	switch {
	case profile.ID != "" && profile.Name != "":
		return profile.ID + " — " + profile.Name
	case profile.ID != "":
		return profile.ID
	case profile.Name != "":
		return profile.Name
	default:
		return unknown
	}
}

func (obj *homeInfoObj) refreshLanguage(lg func(string, string) string) {
	for _, row := range obj.rows {
		row.title.SetText(lg(row.key, row.fallback))
	}
}

func (obj *homeInfoObj) update(
	snapshot guiremote.SnapshotObj,
	lg func(string, string) string,
) {
	unknown := lg("home.value.unknown", "Unknown")
	model := snapshot.Info.Model
	if model == "" {
		model = snapshot.Profile.Name
	}
	if model == "" {
		model = unknown
	}
	firmware := snapshot.Info.Firmware
	if firmware == "" {
		firmware = unknown
	}
	deviceID := snapshot.Status.DeviceID
	if deviceID == "" {
		deviceID = snapshot.Info.Name
	}
	if deviceID == "" {
		deviceID = unknown
	}
	battery := unknown
	if snapshot.Info.Battery.Decoded {
		battery = fmt.Sprintf(
			"%d%% / %d mV",
			snapshot.Info.Battery.Percent,
			snapshot.Info.Battery.VoltageMV,
		)
	}

	obj.set("home.info.ip", snapshot.Status.IP)
	obj.set("home.info.model", model)
	obj.set("home.info.firmware", firmware)
	obj.set("home.info.device_id", deviceID)
	obj.set("home.info.profile", formatProfile(snapshot.Profile, unknown))
	obj.set("home.info.battery", battery)
	obj.set(
		"home.info.links",
		fmt.Sprintf("%d / %d", snapshot.Status.LinksOnline, snapshot.Status.LinksTarget),
	)
	obj.set("home.info.rate", fmt.Sprintf("%.1f Hz", snapshot.Status.Rate))
	obj.set("home.info.errors", strconv.FormatUint(snapshot.Status.Errors, 10))
	obj.set("home.info.mode", localizedMode(snapshot.State.Mode, lg))
}
