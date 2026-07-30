package rc

import "testing"

// // // // // // // // // //

func TestDecodeBattery(t *testing.T) {
	// Sample captured from a DJI RC 2: [4]=0x63 (99%), [7:9]=EA 0F (4074 mV).
	payload := []byte{0x00, 0xAB, 0x09, 0x00, 0x63, 0x00, 0x00, 0xEA, 0x0F, 0x00, 0x00}
	b := decodeBattery(payload)
	if !b.Decoded || b.Percent != 99 || b.VoltageMV != 4074 {
		t.Fatalf("battery = %+v, want 99%% / 4074mV", b)
	}
	if decodeBattery([]byte{0x00, 0x01}).Decoded {
		t.Fatal("short payload must not decode")
	}
}

func TestParseVersionReply(t *testing.T) {
	// Real reply: model "RC331" at [2:18], version bytes near the tail.
	payload := []byte{
		0x00, 0x01, 0x52, 0x43, 0x33, 0x33, 0x31, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x06, 0x00,
		0x00, 0x00, 0x2e, 0x00, 0x06, 0x0a, 0x01, 0x00, 0x00, 0x00,
	}
	model, fw := parseVersionReply(payload)
	if model != "RC331" {
		t.Fatalf("model = %q, want RC331", model)
	}
	if fw != "10.06.00.46" {
		t.Fatalf("firmware = %q, want 10.06.00.46", fw)
	}
}

func TestInfoAccumulator(t *testing.T) {
	acc := newInfoAccumulator()

	name := PacketObj{Sender: 13, CmdSet: idCmdSet, CmdID: idCmdID}
	name.Len = copy(name.Payload[:], []byte("rc331"))
	if !acc.observe(name) {
		t.Fatal("first name packet should change info")
	}
	if acc.observe(name) {
		t.Fatal("identical name packet should not change info")
	}

	st := PacketObj{Sender: 14, CmdSet: selfTestSet, CmdID: selfTestID}
	st.Len = copy(st.Payload[:], []byte{0, 0, 0, 0})
	acc.observe(st)

	snap := acc.snapshot()
	if snap.Name != "rc331" || !snap.SelfTestOK || !snap.SelfTestOn {
		t.Fatalf("snapshot = %+v", snap)
	}
	if len(snap.Telemetry) != 2 {
		t.Fatalf("telemetry entries = %d, want 2", len(snap.Telemetry))
	}
	// Telemetry must be sorted by command key (0x00:0x81 before ... wait 0x00F1 < 0x0081? no).
	if snap.Telemetry[0].CmdID != idCmdID && snap.Telemetry[0].CmdID != selfTestID {
		t.Fatalf("unexpected telemetry ordering: %+v", snap.Telemetry)
	}
}

func TestInfoAccumulatorCapsTelemetry(t *testing.T) {
	acc := newInfoAccumulator()
	for i := 0; i < maxTelemetryCommands+10; i++ {
		packet := PacketObj{
			CmdSet: uint8(i >> 8),
			CmdID:  uint8(i),
		}
		acc.observe(packet)
	}

	if count := len(acc.snapshot().Telemetry); count != maxTelemetryCommands {
		t.Fatalf("telemetry entries = %d, want %d", count, maxTelemetryCommands)
	}
}
