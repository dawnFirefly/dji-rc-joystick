package output

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/voluminor/dji-rc-joystick/mod/rc"
)

// // // // // // // // // //

var spinnerFrames = [...]string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// RunViewObj renders a compact aggregate runtime line.
type RunViewObj struct {
	mu      sync.Mutex
	dst     io.Writer
	style   StyleObj
	cancel  func()
	status  rc.StatusObj
	profile string
	device  string
	err     error
	wrote   bool
}

// NewRunView constructs a compact dynamic runtime renderer.
func NewRunView(dst io.Writer, style StyleObj, cancel func()) *RunViewObj {
	if cancel == nil {
		cancel = func() {}
	}
	return &RunViewObj{dst: dst, style: style, cancel: cancel, device: "waiting"}
}

func (obj *RunViewObj) render() {
	if obj.err != nil {
		return
	}
	identity := obj.status.DeviceID
	if identity == "" {
		identity = "unknown"
	}
	line := fmt.Sprintf("\r\x1b[2K%s remote=%s profile=%s ip=%s links=%d/%d rate=%.1fHz errors=%d joystick=%s",
		phaseStyled(obj.style, obj.status.Phase), obj.style.Cyan(identity), obj.profile,
		obj.status.IP, obj.status.LinksOnline, obj.status.LinksTarget,
		obj.status.Rate, obj.status.Errors, obj.device)
	if _, err := io.WriteString(obj.dst, line); err != nil {
		obj.err = fmt.Errorf("render runtime status: %w", err)
		obj.cancel()
		return
	}
	obj.wrote = true
}

// UpdateStatus replaces and renders the aggregate runtime status.
func (obj *RunViewObj) UpdateStatus(status rc.StatusObj, profile string) {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	obj.status, obj.profile = status, profile
	obj.render()
}

// UpdateDevice replaces and renders the virtual-device state.
func (obj *RunViewObj) UpdateDevice(state string) {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	obj.device = state
	obj.render()
}

// Close terminates the compact line and returns its first output error.
func (obj *RunViewObj) Close() error {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	if obj.wrote {
		_, err := fmt.Fprintln(obj.dst)
		if obj.err == nil && err != nil {
			obj.err = fmt.Errorf("finish runtime status: %w", err)
		}
		obj.wrote = false
	}
	return obj.err
}

// Err returns the first compact-renderer failure.
func (obj *RunViewObj) Err() error {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	return obj.err
}

// DiscoverViewObj renders bounded network-scan progress on one terminal line.
type DiscoverViewObj struct {
	mu    sync.Mutex
	dst   io.Writer
	style StyleObj
	now   func() time.Time
	start time.Time
	frame int
	wrote bool
	err   error
}

// NewDiscoverView constructs a dynamic discovery renderer.
func NewDiscoverView(dst io.Writer, style StyleObj, now func() time.Time) *DiscoverViewObj {
	if now == nil {
		now = time.Now
	}
	return &DiscoverViewObj{dst: dst, style: style, now: now, start: now()}
}

// Update renders one discovery-progress snapshot.
func (obj *DiscoverViewObj) Update(progress rc.DiscoverProgressObj) {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	if obj.err != nil {
		return
	}
	stage := progress.Network
	if progress.Known {
		stage = "known addresses"
	}
	if stage == "" {
		stage = "local network"
	}
	frame := spinnerFrames[obj.frame%len(spinnerFrames)]
	obj.frame++
	_, err := fmt.Fprintf(obj.dst, "\r\x1b[2K%s %s  %d/%d probed  %d found  %s",
		obj.style.Cyan(frame), stage, progress.Probed, progress.Total, progress.Found,
		obj.style.Dim(obj.now().Sub(obj.start).Round(100*time.Millisecond)))
	if err != nil {
		obj.err = fmt.Errorf("render discovery progress: %w", err)
		return
	}
	obj.wrote = true
}

// Close clears the progress line and returns its first output error.
func (obj *DiscoverViewObj) Close() error {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	if obj.wrote {
		_, err := io.WriteString(obj.dst, "\r\x1b[2K")
		if obj.err == nil && err != nil {
			obj.err = fmt.Errorf("clear discovery progress: %w", err)
		}
		obj.wrote = false
	}
	return obj.err
}

// Err returns the first discovery-renderer failure.
func (obj *DiscoverViewObj) Err() error {
	obj.mu.Lock()
	defer obj.mu.Unlock()
	return obj.err
}

func phaseStyled(style StyleObj, phase rc.PhaseID) string {
	switch phase {
	case rc.PhaseConnected:
		return style.Green(phase.String())
	case rc.PhaseDisconnected:
		return style.Red(phase.String())
	default:
		return style.Yellow(phase.String())
	}
}
