package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/voluminor/dji-rc-joystick/mod/rc"
)

// // // // // // // // // //

// WriteRawText writes one human-readable raw-channel record.
func WriteRawText(dst io.Writer, raw rc.RawObj) error {
	var builder strings.Builder
	builder.WriteString("raw payload=")
	_, _ = fmt.Fprintf(&builder, "% X", raw.Payload[:raw.Len])
	builder.WriteString(" channels=")
	for index := 0; index < raw.Count; index++ {
		if index > 0 {
			builder.WriteByte(' ')
		}
		_, _ = fmt.Fprintf(&builder, "[%d]=%+d", index, raw.Channels[index])
	}
	_, err := fmt.Fprintln(dst, builder.String())
	return err
}

// WritePacketText writes one human-readable DUML packet record.
func WritePacketText(dst io.Writer, packet rc.PacketObj) error {
	_, err := fmt.Fprintf(dst,
		"packet sender=%d receiver=%d seq=%d type=0x%02X ack=%d response=%t cmd=%02X:%02X payload=% X\n",
		packet.Sender, packet.Receiver, packet.Seq, packet.CmdType,
		packet.AckType(), packet.IsResponse(), packet.CmdSet, packet.CmdID,
		packet.Payload[:packet.Len])
	return err
}
