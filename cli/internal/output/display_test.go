package output

import (
	"strings"
	"testing"

	"github.com/voluminor/dji-rc-joystick/mod/rc"
)

// // // // // // // // // //

func TestDashboardDoesNotInventStateBeforeFirstFrame(t *testing.T) {
	dashboard := NewDashboard(nil, NewStyle(false), nil)
	lines := strings.Join(dashboard.lines(), "\n")
	if !strings.Contains(lines, "waiting for first frame") || strings.Contains(lines, "mode=sport") {
		t.Fatalf("unexpected initial dashboard:\n%s", lines)
	}

	state := rc.StateObj{Mode: rc.ModeNormal}
	dashboard.UpdateState(state)
	lines = strings.Join(dashboard.lines(), "\n")
	if !strings.Contains(lines, "mode=normal") || strings.Contains(lines, "waiting for first frame") {
		t.Fatalf("decoded state missing from dashboard:\n%s", lines)
	}
}
