package vjoy

import (
	"runtime"
	"strings"
	"testing"
)

// // // // // // // // // //

func validSpec() SpecObj {
	return SpecObj{
		Name:      "Test Device",
		VendorID:  1,
		ProductID: 2,
		Controls: []ControlObj{
			{Handle: 10, Kind: KindAxis, Usage: UsageX, Min: -100, Max: 100},
			{Handle: 11, Kind: KindButton},
			{Handle: 12, Kind: KindRelative, Usage: UsageWheel},
		},
	}
}

// //

func TestNewRejectsBadSpecs(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*SpecObj)
	}{
		{"empty name", func(s *SpecObj) { s.Name = "" }},
		{"long name", func(s *SpecObj) { s.Name = strings.Repeat("x", maxDeviceNameBytes+1) }},
		{"invalid name", func(s *SpecObj) { s.Name = "bad\x00name" }},
		{"too many controls", func(s *SpecObj) { s.Controls = make([]ControlObj, maxControls+1) }},
		{"duplicate handle", func(s *SpecObj) { s.Controls[1].Handle = s.Controls[0].Handle }},
		{"empty axis range", func(s *SpecObj) { s.Controls[0].Max = s.Controls[0].Min }},
		{"unknown kind", func(s *SpecObj) { s.Controls[0].Kind = ControlKind(255) }},
		{"non-wheel relative usage", func(s *SpecObj) { s.Controls[2].Usage = UsageAuto }},
		{"duplicate axis usage", func(s *SpecObj) {
			s.Controls = append(s.Controls, ControlObj{Handle: 20, Kind: KindAxis, Usage: UsageX, Min: -1, Max: 1})
		}},
		{"auto axis collides with explicit", func(s *SpecObj) {
			// UsageAuto resolves to the first axis (UsageX), which handle 10 already claims.
			s.Controls = append(s.Controls, ControlObj{Handle: 21, Kind: KindAxis, Usage: UsageAuto, Min: -1, Max: 1})
		}},
		{"duplicate pinned button", func(s *SpecObj) {
			s.Controls = append(s.Controls,
				ControlObj{Handle: 22, Kind: KindButton, Usage: 7},
				ControlObj{Handle: 23, Kind: KindButton, Usage: 7})
		}},
		{"pinned button out of range", func(s *SpecObj) {
			s.Controls = append(s.Controls, ControlObj{Handle: 24, Kind: KindButton, Usage: maxButtons + 1})
		}},
		{"non-axis usage on an axis", func(s *SpecObj) {
			s.Controls = append(s.Controls, ControlObj{Handle: 25, Kind: KindAxis, Usage: UsageWheel, Min: -1, Max: 1})
		}},
	}

	for _, tc := range cases {
		spec := validSpec()
		tc.mutate(&spec)
		if _, err := New(spec); err == nil {
			t.Errorf("%s: error expected", tc.name)
		}
	}
}

func TestActionButtonN(t *testing.T) {
	a, ok := Action("button3")
	if !ok || a.Kind != KindButton || a.Usage != 3 || a.Press != 1 || a.Continuous {
		t.Fatalf("button3 = %+v ok=%v, want pinned button 3", a, ok)
	}
	if ActionButtonN(3) != "button3" {
		t.Fatalf("ActionButtonN(3) = %q", ActionButtonN(3))
	}
	if a, ok = Action(ActionButton); !ok || a.Usage != 0 {
		t.Fatalf("plain button = %+v ok=%v, want auto (Usage 0)", a, ok)
	}
	if a, ok = Action(ActionButtonN(maxButtons)); !ok || a.Usage != maxButtons {
		t.Fatalf("cap boundary %s = %+v ok=%v, want accepted", ActionButtonN(maxButtons), a, ok)
	}
	for _, bad := range []string{"button0", ActionButtonN(maxButtons + 1), "buttonx", "button+1", "button 1", "button-1"} {
		if _, ok = Action(bad); ok {
			t.Fatalf("%q accepted as an action", bad)
		}
	}
}

func TestAssignButtonsOverflow(t *testing.T) {
	controls := make([]ControlObj, 0, maxButtons+1)
	for i := 0; i < maxButtons; i++ {
		controls = append(controls, ControlObj{Handle: uint16(i), Kind: KindButton})
	}
	if _, count, err := assignButtons(controls); err != nil || count != maxButtons {
		t.Fatalf("maxButtons autos: count=%d err=%v", count, err)
	}
	controls = append(controls, ControlObj{Handle: maxButtons, Kind: KindButton})
	if _, _, err := assignButtons(controls); err == nil {
		t.Fatal("maxButtons+1 autos accepted")
	}
}

func TestAssignButtons(t *testing.T) {
	controls := []ControlObj{
		{Handle: 0, Kind: KindButton},           // auto -> 1
		{Handle: 1, Kind: KindButton, Usage: 2}, // pinned 2
		{Handle: 2, Kind: KindButton},           // auto -> 3 (2 is taken)
		{Handle: 3, Kind: KindButton, Usage: 8}, // pinned 8
		{Handle: 4, Kind: KindAxis, Usage: UsageX, Min: -1, Max: 1},
	}
	numbers, count, err := assignButtons(controls)
	if err != nil {
		t.Fatal(err)
	}
	want := map[uint16]int{0: 1, 1: 2, 2: 3, 3: 8}
	for handle, n := range want {
		if numbers[handle] != n {
			t.Fatalf("handle %d -> button %d, want %d", handle, numbers[handle], n)
		}
	}
	if count != 8 {
		t.Fatalf("required buttons = %d, want 8 (highest pin)", count)
	}
}

func TestNewOnPlatform(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("live device test is Linux-only")
	}

	device, err := New(validSpec())
	if err != nil {
		if strings.Contains(err.Error(), "/dev/uinput") {
			t.Skipf("uinput unavailable: %v", err)
		}
		t.Fatal(err)
	}
	defer func() { _ = device.Close() }()

	for _, handle := range []uint16{10, 11, 12} {
		if err := device.Set(handle, 1); err != nil {
			t.Fatalf("set handle %d: %v", handle, err)
		}
	}
	if err := device.Sync(); err != nil {
		t.Fatal(err)
	}
	if err := device.Set(999, 0); err == nil {
		t.Fatal("unknown handle not rejected")
	}
}
