package output

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/voluminor/dji-rc-joystick/mod/rc"
)

// // // // // // // // // //

const dashboardInterval = 50 * time.Millisecond

// DashboardObj renders the latest monitor snapshot at no more than 20 Hz.
type DashboardObj struct {
	mu      sync.Mutex
	dst     io.Writer
	style   StyleObj
	now     func() time.Time
	updates chan struct{}

	status   rc.StatusObj
	state    rc.StateObj
	info     rc.InfoObj
	profile  string
	hasState bool
	dirty    bool
}

// NewDashboard constructs a terminal-only monitor dashboard.
func NewDashboard(dst io.Writer, style StyleObj, now func() time.Time) *DashboardObj {
	if now == nil {
		now = time.Now
	}
	return &DashboardObj{
		dst: dst, style: style, now: now, updates: make(chan struct{}, 1), dirty: true,
	}
}

func (obj *DashboardObj) changed() {
	obj.dirty = true
	select {
	case obj.updates <- struct{}{}:
	default:
	}
}

// UpdateStatus replaces the aggregate status snapshot.
func (obj *DashboardObj) UpdateStatus(status rc.StatusObj, profile string) {
	obj.mu.Lock()
	obj.status, obj.profile = status, profile
	obj.changed()
	obj.mu.Unlock()
}

// UpdateState replaces the decoded-state snapshot.
func (obj *DashboardObj) UpdateState(state rc.StateObj) {
	obj.mu.Lock()
	obj.state = state
	obj.hasState = true
	obj.changed()
	obj.mu.Unlock()
}

// UpdateInfo replaces the remote-information snapshot.
func (obj *DashboardObj) UpdateInfo(info rc.InfoObj) {
	obj.mu.Lock()
	obj.info = info
	obj.changed()
	obj.mu.Unlock()
}

func pressedButtons(state rc.StateObj) string {
	names := make([]string, 0, rc.ButtonCount)
	for button := rc.ButtonID(0); button < rc.ButtonCount; button++ {
		if state.Pressed(button) {
			names = append(names, button.String())
		}
	}
	if len(names) == 0 {
		return "-"
	}
	return strings.Join(names, " ")
}

func (obj *DashboardObj) lines() []string {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	obj.dirty = false

	identity := obj.info.Name
	if identity == "" {
		identity = obj.status.DeviceID
	}
	if identity == "" {
		identity = "unknown"
	}
	model := obj.info.Model
	if model == "" {
		model = "-"
	}
	firmware := obj.info.Firmware
	if firmware == "" {
		firmware = "-"
	}
	battery := "unknown"
	if obj.info.Battery.Decoded {
		battery = fmt.Sprintf("%d%% (%d mV)", obj.info.Battery.Percent, obj.info.Battery.VoltageMV)
	}
	selfTest := "unknown"
	if obj.info.SelfTestOn {
		if obj.info.SelfTestOK {
			selfTest = obj.style.Green("ok")
		} else {
			selfTest = obj.style.Red("failed")
		}
	}

	lines := []string{
		obj.style.Bold("DJI RC monitor") + "  " + obj.style.Dim(obj.now().Format("15:04:05")),
		fmt.Sprintf("Remote: %s  profile=%s  model=%s  firmware=%s  ip=%s",
			obj.style.Cyan(identity), obj.profile, model, firmware, obj.status.IP),
		fmt.Sprintf("Link:   %s  online=%d/%d  rate=%.1f Hz  errors=%d",
			phaseStyled(obj.style, obj.status.Phase), obj.status.LinksOnline,
			obj.status.LinksTarget, obj.status.Rate, obj.status.Errors),
		fmt.Sprintf("Health: battery=%s  self-test=%s", battery, selfTest),
	}
	if !obj.hasState {
		return append(lines, "Axes:   waiting for first frame", "Input:  waiting for first frame")
	}
	return append(lines,
		fmt.Sprintf("Axes:   LH=%+6d LV=%+6d RH=%+6d RV=%+6d LW=%+6d RW=%+6d",
			obj.state.Axes[rc.AxisLeftH], obj.state.Axes[rc.AxisLeftV],
			obj.state.Axes[rc.AxisRightH], obj.state.Axes[rc.AxisRightV],
			obj.state.Axes[rc.AxisLeftWheel], obj.state.Axes[rc.AxisRightWheel]),
		fmt.Sprintf("Input:  mode=%-7s buttons=[%s]", obj.state.Mode, pressedButtons(obj.state)),
	)
}

func (obj *DashboardObj) render(rendered bool, lineCount int) (int, error) {
	lines := obj.lines()
	var builder strings.Builder
	if rendered {
		_, _ = fmt.Fprintf(&builder, "\x1b[%dA", lineCount)
	}
	for _, line := range lines {
		builder.WriteString("\r\x1b[2K")
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	_, err := io.WriteString(obj.dst, builder.String())
	return len(lines), err
}

// Run renders until ctx ends and restores the terminal cursor.
func (obj *DashboardObj) Run(ctx context.Context) error {
	if _, err := io.WriteString(obj.dst, "\x1b[?25l"); err != nil {
		return fmt.Errorf("hide terminal cursor: %w", err)
	}
	defer func() { _, _ = io.WriteString(obj.dst, "\x1b[?25h") }()

	ticker := time.NewTicker(dashboardInterval)
	defer ticker.Stop()
	rendered := false
	lineCount := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-obj.updates:
		case <-ticker.C:
			obj.mu.Lock()
			dirty := obj.dirty
			obj.mu.Unlock()
			if !dirty {
				continue
			}
			var err error
			lineCount, err = obj.render(rendered, lineCount)
			if err != nil {
				return fmt.Errorf("render monitor: %w", err)
			}
			rendered = true
		}
	}
}
