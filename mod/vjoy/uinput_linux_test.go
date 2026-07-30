package vjoy

import "testing"

// // // // // // // // // //

// The kernel code pool must stay strictly increasing (compaction-preserving),
// skip the BTN_GAMEPAD block (gamepad auto-classification) and KEY_MAX (dead
// in SDL), and match maxButtons exactly.
func TestButtonCodePool(t *testing.T) {
	if _, ok := buttonCode(-1); ok {
		t.Fatal("negative index accepted")
	}
	if _, ok := buttonCode(maxButtons); ok {
		t.Fatalf("index %d beyond the pool accepted", maxButtons)
	}

	prev := uint16(0)
	for i := 0; i < maxButtons; i++ {
		code, ok := buttonCode(i)
		if !ok {
			t.Fatalf("index %d: pool smaller than maxButtons=%d", i, maxButtons)
		}
		if code <= prev && i > 0 {
			t.Fatalf("index %d: code 0x%x not strictly increasing after 0x%x", i, code, prev)
		}
		if code >= 0x130 && code <= 0x13F {
			t.Fatalf("index %d: code 0x%x is in the BTN_GAMEPAD block", i, code)
		}
		if code == 0x2FF {
			t.Fatalf("index %d: code KEY_MAX is dead in SDL", i)
		}
		prev = code
	}

	for index, want := range map[int]uint16{0: 0x120, 15: 0x12F, 16: 0x2C0, maxButtons - 1: 0x2FE} {
		if code, _ := buttonCode(index); code != want {
			t.Fatalf("buttonCode(%d) = 0x%x, want 0x%x", index, code, want)
		}
	}
}

// The gamepad node must advertise every button code up to the highest
// assigned number so consumer compaction preserves pinned numbering.
func TestPlanDeviceAdvertisesUpToHighest(t *testing.T) {
	spec := SpecObj{Name: "t", Controls: []ControlObj{
		{Handle: 1, Kind: KindButton, Usage: 1},
		{Handle: 2, Kind: KindButton, Usage: 2},
		{Handle: 3, Kind: KindButton, Usage: 3},
		{Handle: 6, Kind: KindButton, Usage: 6},
		{Handle: 7, Kind: KindButton, Usage: 7},
		{Handle: 16, Kind: KindButton, Usage: 16},
	}}
	dev, gamepad, _, hasRel, err := planDevice(spec)
	if err != nil || hasRel {
		t.Fatalf("plan: err=%v hasRel=%v", err, hasRel)
	}

	if len(gamepad.keyCodes) != 16 {
		t.Fatalf("advertised %d codes, want 16 (up to the highest pin)", len(gamepad.keyCodes))
	}
	for i, code := range gamepad.keyCodes {
		if want := uint16(0x120 + i); code != want {
			t.Fatalf("advertised[%d] = 0x%x, want 0x%x (gap-free)", i, code, want)
		}
	}

	for handle, want := range map[uint16]uint16{1: 0x120, 2: 0x121, 3: 0x122, 6: 0x125, 7: 0x126, 16: 0x12F} {
		if got := dev.bindings[handle].code; got != want {
			t.Fatalf("pin %d bound to 0x%x, want 0x%x", handle, got, want)
		}
	}
}

// Crossing into BTN_TRIGGER_HAPPY keeps numbering intact and never touches
// the gamepad block.
func TestPlanDeviceCrossesGapCleanly(t *testing.T) {
	spec := SpecObj{Name: "t", Controls: []ControlObj{
		{Handle: 29, Kind: KindButton, Usage: 29},
	}}
	dev, gamepad, _, _, err := planDevice(spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(gamepad.keyCodes) != 29 {
		t.Fatalf("advertised %d codes, want 29", len(gamepad.keyCodes))
	}
	for _, code := range gamepad.keyCodes {
		if code >= 0x130 && code <= 0x13F {
			t.Fatalf("gamepad-block code 0x%x advertised", code)
		}
	}
	if got := dev.bindings[29].code; got != 0x2C0+12 {
		t.Fatalf("pin 29 bound to 0x%x, want 0x%x", got, 0x2C0+12)
	}
}

// A zero-button device still advertises BTN_TRIGGER once for classification.
func TestPlanDeviceZeroButtons(t *testing.T) {
	spec := SpecObj{Name: "t", Controls: []ControlObj{
		{Handle: 1, Kind: KindAxis, Usage: UsageX, Min: -1, Max: 1},
	}}
	_, gamepad, _, _, err := planDevice(spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(gamepad.keyCodes) != 1 || gamepad.keyCodes[0] != btnTrigger {
		t.Fatalf("keyCodes = %#v, want exactly [BTN_TRIGGER]", gamepad.keyCodes)
	}
}
