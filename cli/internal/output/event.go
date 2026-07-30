package output

import (
	"encoding/hex"

	"github.com/voluminor/dji-rc-joystick/mod/rc"
)

// // // // // // // // // //

// SchemaVersion is the current JSON and NDJSON wire-format version.
const SchemaVersion = 1

// RemoteObj identifies the remote associated with an event.
type RemoteObj struct {
	IP       string `json:"ip,omitempty"`
	DeviceID string `json:"device_id,omitempty"`
	Profile  string `json:"profile,omitempty"`
}

// AxesObj contains every named controller axis.
type AxesObj struct {
	LeftH      int16 `json:"left_h"`
	LeftV      int16 `json:"left_v"`
	RightH     int16 `json:"right_h"`
	RightV     int16 `json:"right_v"`
	LeftWheel  int16 `json:"left_wheel"`
	RightWheel int16 `json:"right_wheel"`
}

// StateObj is the machine-readable decoded controller state.
type StateObj struct {
	Axes    AxesObj  `json:"axes"`
	Mode    string   `json:"mode"`
	Buttons []string `json:"buttons"`
}

// StatusObj is the machine-readable aggregate connection status.
type StatusObj struct {
	Phase          string  `json:"phase"`
	ConnectedLinks int     `json:"connected_links"`
	TargetLinks    int     `json:"target_links"`
	RateHz         float64 `json:"rate_hz"`
	Errors         uint64  `json:"errors"`
}

// BatteryObj is an optional machine-readable battery reading.
type BatteryObj struct {
	Percent   int `json:"percent"`
	VoltageMV int `json:"voltage_mv"`
}

// InfoObj is the machine-readable remote information snapshot.
type InfoObj struct {
	Name              string      `json:"name,omitempty"`
	Model             string      `json:"model,omitempty"`
	Firmware          string      `json:"firmware,omitempty"`
	SelfTestObserved  bool        `json:"self_test_observed"`
	SelfTestOK        bool        `json:"self_test_ok"`
	TelemetryCommands int         `json:"telemetry_commands"`
	Battery           *BatteryObj `json:"battery,omitempty"`
}

// RawObj is one raw channel snapshot used by inspect.
type RawObj struct {
	PayloadHex string  `json:"payload_hex"`
	Channels   []int16 `json:"channels"`
}

// PacketObj is one bounded DUML packet diagnostic used by inspect.
type PacketObj struct {
	Sender      uint8  `json:"sender"`
	Receiver    uint8  `json:"receiver"`
	Sequence    uint16 `json:"sequence"`
	CommandType uint8  `json:"command_type"`
	AckType     uint8  `json:"ack_type"`
	Response    bool   `json:"response"`
	CommandSet  uint8  `json:"command_set"`
	CommandID   uint8  `json:"command_id"`
	PayloadHex  string `json:"payload_hex"`
}

// EventObj is one NDJSON v1 envelope.
type EventObj struct {
	Schema int       `json:"schema"`
	Type   string    `json:"type"`
	Seq    uint64    `json:"seq"`
	Time   string    `json:"time"`
	Remote RemoteObj `json:"remote"`

	State  *StateObj  `json:"state,omitempty"`
	Status *StatusObj `json:"status,omitempty"`
	Info   *InfoObj   `json:"info,omitempty"`
	Raw    *RawObj    `json:"raw,omitempty"`
	Packet *PacketObj `json:"packet,omitempty"`
}

// StateEvent converts a decoded state into an unstamped event.
func StateEvent(remote RemoteObj, state rc.StateObj) EventObj {
	buttons := make([]string, 0, rc.ButtonCount)
	for button := rc.ButtonID(0); button < rc.ButtonCount; button++ {
		if state.Pressed(button) {
			buttons = append(buttons, button.String())
		}
	}
	return EventObj{
		Type: "state", Remote: remote,
		State: &StateObj{
			Axes: AxesObj{
				LeftH: state.Axes[rc.AxisLeftH], LeftV: state.Axes[rc.AxisLeftV],
				RightH: state.Axes[rc.AxisRightH], RightV: state.Axes[rc.AxisRightV],
				LeftWheel: state.Axes[rc.AxisLeftWheel], RightWheel: state.Axes[rc.AxisRightWheel],
			},
			Mode: state.Mode.String(), Buttons: buttons,
		},
	}
}

// StatusEvent converts aggregate status into an unstamped event.
func StatusEvent(remote RemoteObj, status rc.StatusObj) EventObj {
	return EventObj{
		Type: "status", Remote: remote,
		Status: &StatusObj{
			Phase: status.Phase.String(), ConnectedLinks: status.LinksOnline,
			TargetLinks: status.LinksTarget, RateHz: status.Rate, Errors: status.Errors,
		},
	}
}

// InfoEvent converts remote information into an unstamped event.
func InfoEvent(remote RemoteObj, info rc.InfoObj) EventObj {
	payload := &InfoObj{
		Name: info.Name, Model: info.Model, Firmware: info.Firmware,
		SelfTestObserved: info.SelfTestOn, SelfTestOK: info.SelfTestOK,
		TelemetryCommands: len(info.Telemetry),
	}
	if info.Battery.Decoded {
		payload.Battery = &BatteryObj{
			Percent: info.Battery.Percent, VoltageMV: info.Battery.VoltageMV,
		}
	}
	return EventObj{Type: "info", Remote: remote, Info: payload}
}

// RawEvent converts a raw snapshot into an unstamped inspect event.
func RawEvent(remote RemoteObj, raw rc.RawObj) EventObj {
	channels := append([]int16(nil), raw.Channels[:raw.Count]...)
	return EventObj{
		Type: "raw", Remote: remote,
		Raw: &RawObj{PayloadHex: hex.EncodeToString(raw.Payload[:raw.Len]), Channels: channels},
	}
}

// PacketEvent converts a packet into an unstamped inspect event.
func PacketEvent(remote RemoteObj, packet rc.PacketObj) EventObj {
	return EventObj{
		Type: "packet", Remote: remote,
		Packet: &PacketObj{
			Sender: packet.Sender, Receiver: packet.Receiver, Sequence: packet.Seq,
			CommandType: packet.CmdType, AckType: packet.AckType(), Response: packet.IsResponse(),
			CommandSet: packet.CmdSet, CommandID: packet.CmdID,
			PayloadHex: hex.EncodeToString(packet.Payload[:packet.Len]),
		},
	}
}
