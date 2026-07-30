package rc

import (
	"encoding/binary"
	"fmt"
	"sort"
)

// // // // // // // // // //

const (
	statusCmdSet = 0x06 // RC status blob
	statusCmdID  = 0x1E
	selfTestSet  = 0x00 // Component Self Test State
	selfTestID   = 0xF1
	versionSet   = 0x00 // General Version Inquiry
	versionID    = 0x01

	// Field offsets inside the 0x06:0x1E status payload (reverse-engineered on
	// a DJI RC 2; best-effort, hence guarded by length checks).
	statusBatteryPct = 4 // 0x5F..0x64 tracked battery percent while charging
	statusVoltageLo  = 7 // little-endian mV at [7:9], ~4050..4214 observed

	// batteryVoltageTolerance ignores finer voltage jitter so a steady drift of
	// a few millivolts does not re-publish info on every status packet.
	batteryVoltageTolerance = 50 // mV

	maxTelemetryCommands = 256
)

// //

// BatteryObj is the remote's best-effort battery reading. Decoded is false
// when the status blob was too short or absent.
type BatteryObj struct {
	// Decoded reports whether a complete battery payload was observed.
	Decoded bool
	// Percent is the best-effort charge percentage.
	Percent int
	// VoltageMV is the best-effort battery voltage in millivolts.
	VoltageMV int
}

// TelemetryObj is the latest bounded raw payload seen for one DUML command.
type TelemetryObj struct {
	// CmdSet identifies the DUML command set.
	CmdSet uint8
	// CmdID identifies the command within CmdSet.
	CmdID uint8
	// Sender is the latest observed DUML sender.
	Sender uint8
	// Len is the number of valid payload bytes.
	Len int
	// Payload contains the bounded latest payload.
	Payload [rawPayloadMax]byte
	// Count is the number of observations for this command.
	Count uint64
}

// InfoObj is everything the remote reports about itself, collected passively
// from its packet stream plus an optional one-shot version query.
type InfoObj struct {
	// Name is the announced device identity, such as "rc331".
	Name string
	// Model is the model decoded from an optional version reply.
	Model string
	// Firmware is the best-effort version decoded from an optional reply.
	Firmware string
	// SelfTestOK reports an all-zero self-test state.
	SelfTestOK bool
	// SelfTestOn reports whether any self-test packet was observed.
	SelfTestOn bool
	// Battery is the latest best-effort battery reading.
	Battery BatteryObj
	// Telemetry contains up to 256 latest command snapshots sorted by command.
	Telemetry []TelemetryObj
}

// //

func telemetryKey(set, id uint8) uint16 { return uint16(set)<<8 | uint16(id) }

func cloneInfo(info InfoObj) InfoObj {
	info.Telemetry = append([]TelemetryObj(nil), info.Telemetry...)
	return info
}

func decodeBattery(payload []byte) BatteryObj {
	if len(payload) <= statusVoltageLo+1 {
		return BatteryObj{}
	}
	return BatteryObj{
		Decoded:   true,
		Percent:   int(payload[statusBatteryPct]),
		VoltageMV: int(binary.LittleEndian.Uint16(payload[statusVoltageLo : statusVoltageLo+2])),
	}
}

// parseVersionReply extracts the model string and firmware version from a
// General Version Inquiry response. Layout per dji-firmware-tools: [2:18]
// NUL-padded hardware/model string, [22:26] the app (firmware) version dword
// displayed byte-reversed as MM.mm.pp.bb.
func parseVersionReply(payload []byte) (model, firmware string) {
	if len(payload) >= 18 {
		model = trimASCII(payload[2:18])
	}
	if len(payload) >= 26 {
		v := payload[22:26]
		if v[0]|v[1]|v[2]|v[3] != 0 {
			firmware = fmt.Sprintf("%02d.%02d.%02d.%02d", v[3], v[2], v[1], v[0])
		}
	}
	return model, firmware
}

func trimASCII(b []byte) string {
	out := make([]byte, 0, len(b))
	for _, c := range b {
		if c == 0 {
			break
		}
		if c < 0x20 || c > 0x7e {
			return ""
		}
		out = append(out, c)
	}
	return string(out)
}

// //

// infoAccumulatorObj folds the packet stream into an InfoObj. It is touched
// only by the coordinator goroutine, so it needs no locking.
type infoAccumulatorObj struct {
	name       string
	model      string
	firmware   string
	selfTestOK bool
	selfTestOn bool
	battery    BatteryObj
	telemetry  map[uint16]*TelemetryObj
}

func newInfoAccumulator() *infoAccumulatorObj {
	return &infoAccumulatorObj{telemetry: make(map[uint16]*TelemetryObj)}
}

func (a *infoAccumulatorObj) observe(p PacketObj) bool {
	key := telemetryKey(p.CmdSet, p.CmdID)
	entry := a.telemetry[key]
	if entry == nil && len(a.telemetry) < maxTelemetryCommands {
		entry = &TelemetryObj{CmdSet: p.CmdSet, CmdID: p.CmdID}
		a.telemetry[key] = entry
	}

	changed := false
	if entry != nil {
		entry.Sender = p.Sender
		entry.Len = copy(entry.Payload[:], p.Payload[:p.Len])
		entry.Count++
		changed = entry.Count == 1
	}

	switch {
	case p.CmdSet == idCmdSet && p.CmdID == idCmdID:
		if id, ok := packetDeviceID(p); ok && id != a.name {
			a.name = id
			changed = true
		}
	case p.CmdSet == selfTestSet && p.CmdID == selfTestID:
		ok := allZero(p.Payload[:p.Len])
		if !a.selfTestOn || ok != a.selfTestOK {
			a.selfTestOn, a.selfTestOK = true, ok
			changed = true
		}
	case p.CmdSet == statusCmdSet && p.CmdID == statusCmdID:
		b := decodeBattery(p.Payload[:p.Len])
		if b.Decoded && batteryChanged(a.battery, b) {
			a.battery = b
			changed = true
		}
	case p.IsResponse() && p.CmdSet == versionSet && p.CmdID == versionID:
		model, fw := parseVersionReply(p.Payload[:p.Len])
		if model != a.model || fw != a.firmware {
			a.model, a.firmware = model, fw
			changed = true
		}
	}
	return changed
}

func (a *infoAccumulatorObj) snapshot() InfoObj {
	keys := make([]uint16, 0, len(a.telemetry))
	for k := range a.telemetry {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	tele := make([]TelemetryObj, 0, len(keys))
	for _, k := range keys {
		tele = append(tele, *a.telemetry[k])
	}
	return InfoObj{
		Name:       a.name,
		Model:      a.model,
		Firmware:   a.firmware,
		SelfTestOK: a.selfTestOK,
		SelfTestOn: a.selfTestOn,
		Battery:    a.battery,
		Telemetry:  tele,
	}
}

func batteryChanged(old, next BatteryObj) bool {
	if !old.Decoded || old.Percent != next.Percent {
		return true
	}
	delta := old.VoltageMV - next.VoltageMV
	if delta < 0 {
		delta = -delta
	}
	return delta >= batteryVoltageTolerance
}

func allZero(b []byte) bool {
	for _, x := range b {
		if x != 0 {
			return false
		}
	}
	return true
}
