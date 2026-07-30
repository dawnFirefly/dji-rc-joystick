package rc

// // // // // // // // // //

// AxisID identifies a semantic controller axis.
type AxisID int

const (
	// AxisLeftH is the left stick's horizontal axis.
	AxisLeftH AxisID = iota
	// AxisLeftV is the left stick's vertical axis.
	AxisLeftV
	// AxisRightH is the right stick's horizontal axis.
	AxisRightH
	// AxisRightV is the right stick's vertical axis.
	AxisRightV
	// AxisLeftWheel is the left dial.
	AxisLeftWheel
	// AxisRightWheel is the right dial.
	AxisRightWheel

	// AxisCount is the number of semantic axes.
	AxisCount
)

// ButtonID identifies a semantic controller button.
type ButtonID int

const (
	// ButtonRTH is the return-to-home or flight-pause button.
	ButtonRTH ButtonID = iota
	// ButtonRecord is the video-record button.
	ButtonRecord
	// ButtonCameraHalf is the camera half-press.
	ButtonCameraHalf
	// ButtonCameraFull is the camera full-press or shutter.
	ButtonCameraFull
	// ButtonC1 is the first customizable button.
	ButtonC1
	// ButtonC2 is the second customizable button.
	ButtonC2

	// ButtonCount is the number of semantic buttons.
	ButtonCount
)

// ModeID is the flight mode switch position; raw values match the wire.
type ModeID int

const (
	// ModeSport is the Sport switch position.
	ModeSport ModeID = iota
	// ModeNormal is the Normal switch position.
	ModeNormal
	// ModeCinema is the Cinema switch position.
	ModeCinema
	// ModeUnknown means the mode was absent or invalid.
	ModeUnknown
)

// PhaseID is the pipeline connection phase.
type PhaseID int

const (
	// PhaseScanning means local-network discovery is running.
	PhaseScanning PhaseID = iota
	// PhaseConnecting means no valid controller stream is currently active.
	PhaseConnecting
	// PhaseConnected means controller frames are arriving within the liveness window.
	PhaseConnected
	// PhaseDisconnected is the initial and stopped state.
	PhaseDisconnected
)

// //

// String returns the stable lowercase mode name.
func (m ModeID) String() string {
	switch m {
	case ModeSport:
		return "sport"
	case ModeNormal:
		return "normal"
	case ModeCinema:
		return "cinema"
	default:
		return "unknown"
	}
}

// String returns the stable lowercase phase name.
func (p PhaseID) String() string {
	switch p {
	case PhaseScanning:
		return "scanning"
	case PhaseConnecting:
		return "connecting"
	case PhaseConnected:
		return "connected"
	case PhaseDisconnected:
		return "disconnected"
	default:
		return "invalid"
	}
}

// String returns the short button label used in diagnostics.
func (b ButtonID) String() string {
	switch b {
	case ButtonRTH:
		return "RTH"
	case ButtonRecord:
		return "REC"
	case ButtonCameraHalf:
		return "CAMh"
	case ButtonCameraFull:
		return "CAMf"
	case ButtonC1:
		return "C1"
	case ButtonC2:
		return "C2"
	default:
		return "BTN?"
	}
}

// String returns the short axis label used in diagnostics.
func (a AxisID) String() string {
	switch a {
	case AxisLeftH:
		return "LH"
	case AxisLeftV:
		return "LV"
	case AxisRightH:
		return "RH"
	case AxisRightV:
		return "RV"
	case AxisLeftWheel:
		return "LW"
	case AxisRightWheel:
		return "RW"
	default:
		return "AX?"
	}
}

// //

// StateObj is one decoded controller snapshot. Plain value: cheap to copy,
// allocation-free on the hot path.
type StateObj struct {
	// Axes contains values scaled to -32767..32767.
	Axes [AxisCount]int16
	// Buttons is a bitset indexed by ButtonID.
	Buttons uint32
	// Mode is the decoded flight-mode switch position.
	Mode ModeID
}

// Pressed reports whether the given button is held.
func (s StateObj) Pressed(b ButtonID) bool {
	if b < 0 || b >= ButtonCount {
		return false
	}
	return s.Buttons&(1<<uint(b)) != 0
}

// StatusObj describes pipeline health.
type StatusObj struct {
	// Phase is the merged connection state.
	Phase PhaseID
	// IP is the selected remote address.
	IP string
	// DeviceID is the announced identity, such as "rc331".
	DeviceID string
	// LinksOnline is the number of connected reader links.
	LinksOnline int
	// LinksTarget is the number of reader links maintained by the coordinator.
	LinksTarget int
	// Rate is the average decoded-frame rate since Start.
	Rate float64
	// Errors counts reported discovery and sink failures in the current run.
	Errors uint64
}

// LinkEventObj describes one reader-link transport transition or failure.
// Operation is one of "connect", "read", "write", or "disconnect".
type LinkEventObj struct {
	// Index identifies the reader within the current run.
	Index int
	// Online reports whether the reader is connected after this event.
	Online bool
	// Operation identifies the transport operation that produced the event.
	Operation string
	// Error is the transport failure, or nil for a clean transition.
	Error error
}

// RawObj carries undecoded channel data for mapping unknown remotes.
type RawObj struct {
	// Payload contains the captured RC-channel payload.
	Payload [rawPayloadMax]byte
	// Len is the number of valid bytes in Payload.
	Len int
	// Channels contains little-endian channel words decoded as signed values.
	Channels [maxChannels]int16
	// Count is the number of valid channel values.
	Count int
}

// PacketObj is one raw DUML packet header, exposed for protocol diagnostics
// and acknowledged automatically by the pipeline when the sender asks for it.
type PacketObj struct {
	// Sender is the DUML sender identifier.
	Sender uint8
	// Receiver is the DUML receiver identifier.
	Receiver uint8
	// Seq is the DUML sequence number.
	Seq uint16
	// CmdType contains the response flag and requested acknowledgement type.
	CmdType uint8
	// CmdSet is the DUML command-set identifier.
	CmdSet uint8
	// CmdID is the command identifier within CmdSet.
	CmdID uint8
	// Payload contains a bounded diagnostic prefix of the packet payload.
	Payload [packetSnippetMax]byte
	// Len is the number of valid bytes in Payload.
	Len int
}

// IsResponse reports whether the packet is a reply rather than a request.
func (p PacketObj) IsResponse() bool {
	return p.CmdType&0x80 != 0
}

// AckType returns the acknowledgment the sender expects (0: none).
func (p PacketObj) AckType() uint8 {
	return (p.CmdType >> 5) & 0x03
}
