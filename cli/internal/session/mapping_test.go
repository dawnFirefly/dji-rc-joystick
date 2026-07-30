package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voluminor/dji-rc-joystick/mod/rc"
)

// // // // // // // // // //

func writeConfig(t *testing.T, extension, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings"+extension)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// //

func TestLoadMappingIgnoresInvalidJSONUI(t *testing.T) {
	path := writeConfig(t, ".json", `{"ui":"invalid","mapping":{}}`)
	mapping, err := LoadMapping(path)
	if err != nil {
		t.Fatal(err)
	}
	if mapping.Buttons[rc.ButtonRTH] != "button10" {
		t.Fatalf("RTH mapping = %q, want built-in button10", mapping.Buttons[rc.ButtonRTH])
	}
}

func TestLoadMappingIgnoresInvalidYAMLUIAndAppliesOverride(t *testing.T) {
	path := writeConfig(t, ".yaml", "ui: invalid\nmapping:\n  buttons:\n    rth: button11\n")
	mapping, err := LoadMapping(path)
	if err != nil {
		t.Fatal(err)
	}
	if mapping.Buttons[rc.ButtonRTH] != "button11" {
		t.Fatalf("RTH mapping = %q, want button11", mapping.Buttons[rc.ButtonRTH])
	}
}

func TestLoadMappingRejectsMissingDuplicateAndUnknownMapping(t *testing.T) {
	tests := []struct {
		name      string
		extension string
		content   string
	}{
		{name: "missing JSON", extension: ".json", content: `{"ui":{}}`},
		{name: "duplicate JSON", extension: ".json", content: `{"mapping":{},"mapping":{}}`},
		{name: "unknown JSON field", extension: ".json", content: `{"mapping":{"unknown":true}}`},
		{name: "missing YAML", extension: ".yaml", content: "ui: {}\n"},
		{name: "duplicate YAML", extension: ".yaml", content: "mapping: {}\nmapping: {}\n"},
		{name: "unknown YAML field", extension: ".yaml", content: "mapping:\n  unknown: true\n"},
		{name: "unknown action", extension: ".yaml", content: "mapping:\n  buttons:\n    rth: invalid_action\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := LoadMapping(writeConfig(t, test.extension, test.content)); err == nil {
				t.Fatal("invalid mapping config was accepted")
			}
		})
	}
}

func TestLoadMappingEnforcesFilePolicy(t *testing.T) {
	if _, err := LoadMapping(writeConfig(t, ".toml", "mapping = {}")); err == nil {
		t.Fatal("unsupported extension was accepted")
	}
	if _, err := LoadMapping(t.TempDir()); err == nil {
		t.Fatal("directory was accepted as a config")
	}
	oversized := writeConfig(t, ".json", strings.Repeat(" ", maxConfigSize+1))
	if _, err := LoadMapping(oversized); err == nil {
		t.Fatal("oversized config was accepted")
	}
}
