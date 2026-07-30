package rc

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// // // // // // // // // //

func refCRC8Table() [256]uint8 {
	var table [256]uint8
	for i := 0; i < 256; i++ {
		crc := uint8(i)
		for bit := 0; bit < 8; bit++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0x8C
			} else {
				crc >>= 1
			}
		}
		table[i] = crc
	}
	return table
}

func refCRC16Table() [256]uint16 {
	var table [256]uint16
	for i := 0; i < 256; i++ {
		crc := uint16(i)
		for bit := 0; bit < 8; bit++ {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0x8408
			} else {
				crc >>= 1
			}
		}
		table[i] = crc
	}
	return table
}

func buildFrame(sender, receiver uint8, seq uint16, cmdType, cmdSet, cmdID uint8, payload []byte) []byte {
	total := dumlHeaderLen + len(payload) + dumlCRCLen
	pkt := make([]byte, 0, total)
	pkt = append(pkt, dumlSOF, byte(total), byte((total>>8)&0x03)|1<<2)
	pkt = append(pkt, crc8(pkt[:3]))
	pkt = append(pkt, sender, receiver, byte(seq), byte(seq>>8), cmdType, cmdSet, cmdID)
	pkt = append(pkt, payload...)
	sum := crc16(pkt)
	pkt = append(pkt, byte(sum), byte(sum>>8))

	frame := make([]byte, 0, 8+len(pkt))
	frame = append(frame, tcpMagic[:]...)
	frame = binary.LittleEndian.AppendUint32(frame, uint32(len(pkt)))
	frame = append(frame, pkt...)
	return frame
}

func buildRCFrame(seq uint16, payload []byte) []byte {
	return buildFrame(6, 2, seq, 0, rcCmdSet, rcCmdID, payload)
}

func buildIDFrame(seq uint16, id string) []byte {
	return buildFrame(13, 2, seq, 0, idCmdSet, idCmdID, []byte(id))
}

func checksumFrame(frame []byte) {
	packet := frame[8:]
	packet[3] = crc8(packet[:3])
	checksum := crc16(packet[:len(packet)-dumlCRCLen])
	binary.LittleEndian.PutUint16(packet[len(packet)-dumlCRCLen:], checksum)
}

func parseChannels(data []byte, channels *[maxChannels]int16, payload *[rawPayloadMax]byte) (int, int, int) {
	var packets [maxParsePackets]PacketObj
	numCh, payloadLen, consumed, _ := parseStream(data, channels, payload, &packets)
	return numCh, payloadLen, consumed
}

// //

func TestCRCTablesMatchPolynomials(t *testing.T) {
	if crc8Table != refCRC8Table() {
		t.Fatal("crc8Table does not match the reference polynomial table")
	}
	if crc16Table != refCRC16Table() {
		t.Fatal("crc16Table does not match the reference polynomial table")
	}
}

func TestBuildKeepalive(t *testing.T) {
	frame := buildVersionQuery(0x1234)

	if !bytes.Equal(frame[:4], tcpMagic[:]) {
		t.Fatal("missing TCP magic")
	}
	if binary.LittleEndian.Uint32(frame[4:8]) != dumlMinLen {
		t.Fatal("wrong transport length")
	}

	pkt := frame[8:]
	if pkt[0] != dumlSOF || pkt[9] != 0x00 || pkt[10] != 0x01 {
		t.Fatal("wrong DUML header or command")
	}
	if pkt[6] != 0x34 || pkt[7] != 0x12 {
		t.Fatal("wrong sequence encoding")
	}
	if pkt[3] != crc8(pkt[:3]) {
		t.Fatal("wrong header CRC-8")
	}
	if binary.LittleEndian.Uint16(pkt[11:13]) != crc16(pkt[:dumlHeaderLen]) {
		t.Fatal("wrong packet CRC-16")
	}
}

func TestParseSingleFrame(t *testing.T) {
	payload := make([]byte, 17)
	payload[0] = 0x20 // ch0: RTH bit
	payload[2] = 0x01 // ch1: normal mode
	frame := buildRCFrame(1, payload)

	var channels [maxChannels]int16
	var raw [rawPayloadMax]byte
	numCh, payloadLen, consumed := parseChannels(frame, &channels, &raw)

	if numCh != 8 {
		t.Fatalf("numCh = %d, want 8", numCh)
	}
	if payloadLen != 17 {
		t.Fatalf("payloadLen = %d, want 17", payloadLen)
	}
	if consumed != len(frame) {
		t.Fatalf("consumed = %d, want %d", consumed, len(frame))
	}
	if channels[0] != 0x20 || channels[1] != 0x01 {
		t.Fatalf("channels = %v", channels[:2])
	}
}

func TestParseKeepsLastFrame(t *testing.T) {
	first := make([]byte, 17)
	first[0] = 1
	second := make([]byte, 17)
	second[0] = 2
	data := append(buildRCFrame(1, first), buildRCFrame(2, second)...)

	var channels [maxChannels]int16
	var raw [rawPayloadMax]byte
	numCh, _, consumed := parseChannels(data, &channels, &raw)

	if numCh != 8 || channels[0] != 2 {
		t.Fatalf("stale frame won: numCh=%d ch0=%d", numCh, channels[0])
	}
	if consumed != len(data) {
		t.Fatalf("consumed = %d, want %d", consumed, len(data))
	}
}

func TestParseGarbagePrefixAndTruncation(t *testing.T) {
	payload := make([]byte, 17)
	payload[0] = 7
	frame := buildRCFrame(1, payload)

	junk := []byte{0x55, 0x00, 0xCC, 0x30, 0x75, 0xFF}
	data := append(append([]byte{}, junk...), frame...)

	var channels [maxChannels]int16
	var raw [rawPayloadMax]byte
	numCh, _, consumed := parseChannels(data, &channels, &raw)
	if numCh != 8 || channels[0] != 7 {
		t.Fatalf("frame after junk not parsed: numCh=%d", numCh)
	}
	if consumed != len(data) {
		t.Fatalf("consumed = %d, want %d", consumed, len(data))
	}

	truncated := frame[:len(frame)-3]
	numCh, _, consumed = parseChannels(truncated, &channels, &raw)
	if numCh != 0 || consumed != 0 {
		t.Fatalf("truncated frame must stay buffered: numCh=%d consumed=%d", numCh, consumed)
	}
}

func TestParseRejectsHugeLength(t *testing.T) {
	// The 0x80000000+ cases have the high bit set: on 32-bit int a narrowed
	// length goes negative and must not bypass the guard or panic.
	for _, badLen := range []uint32{1 << 20, 0x80000000, 0xFFFFFFFF} {
		evil := append(append([]byte{}, tcpMagic[:]...), 0, 0, 0, 0)
		binary.LittleEndian.PutUint32(evil[4:8], badLen)

		payload := make([]byte, 17)
		payload[0] = 9
		data := append(evil, buildRCFrame(1, payload)...)

		var channels [maxChannels]int16
		var raw [rawPayloadMax]byte
		numCh, _, consumed := parseChannels(data, &channels, &raw)
		if numCh != 8 || channels[0] != 9 {
			t.Fatalf("len %#x: valid frame after a rogue length not parsed: numCh=%d", badLen, numCh)
		}
		if consumed != len(data) {
			t.Fatalf("len %#x: consumed = %d, want %d", badLen, consumed, len(data))
		}
	}
}

func TestParseRejectsInvalidChecksumsAndLength(t *testing.T) {
	valid := buildRCFrame(1, centeredPayload())

	for name, mutate := range map[string]func([]byte){
		"header CRC": func(frame []byte) {
			frame[11] ^= 0xFF
		},
		"packet CRC": func(frame []byte) {
			frame[len(frame)-1] ^= 0xFF
		},
		"DUML length": func(frame []byte) {
			frame[9]--
			checksumFrame(frame)
		},
	} {
		t.Run(name, func(t *testing.T) {
			frame := append([]byte(nil), valid...)
			mutate(frame)
			var channels [maxChannels]int16
			var payload [rawPayloadMax]byte
			count, payloadLen, consumed := parseChannels(frame, &channels, &payload)
			if count != 0 || payloadLen != 0 || consumed != len(frame) {
				t.Fatalf("invalid frame decoded: count=%d payload=%d consumed=%d", count, payloadLen, consumed)
			}
		})
	}
}

func TestParseIgnoresOtherCommands(t *testing.T) {
	payload := make([]byte, 17)
	frame := buildRCFrame(1, payload)
	frame[8+10] = 0x42 // overwrite cmd_id
	checksumFrame(frame)

	var channels [maxChannels]int16
	var raw [rawPayloadMax]byte
	numCh, _, consumed := parseChannels(frame, &channels, &raw)
	if numCh != 0 {
		t.Fatalf("non-RC packet decoded: numCh=%d", numCh)
	}
	if consumed != len(frame) {
		t.Fatalf("non-RC frame must still be consumed: %d", consumed)
	}
}

func TestPacketDeviceID(t *testing.T) {
	var packet PacketObj
	packet.CmdSet = idCmdSet
	packet.CmdID = idCmdID
	packet.Len = copy(packet.Payload[:], append([]byte("rc331"), 0, 0, 0))

	if id, ok := packetDeviceID(packet); !ok || id != "rc331" {
		t.Fatalf("device id = %q (ok=%t), want rc331", id, ok)
	}

	packet.CmdID = 0xAE
	if _, ok := packetDeviceID(packet); ok {
		t.Fatal("non-identity packet parsed as device id")
	}

	packet.CmdID = idCmdID
	packet.Len = copy(packet.Payload[:], []byte{0xFF, 0x00})
	if _, ok := packetDeviceID(packet); ok {
		t.Fatal("non-ASCII payload accepted")
	}
}

func TestKeepaliveFrame(t *testing.T) {
	if _, send := keepaliveFrame(KeepaliveOff, 1); send {
		t.Fatal("KeepaliveOff must not send")
	}

	version, send := keepaliveFrame(KeepaliveVersion, 1)
	if !send || version[8+10] != 0x01 {
		t.Fatalf("KeepaliveVersion must send cmd 0x01, got 0x%02X", version[8+10])
	}
	heartbeat, send := keepaliveFrame(KeepaliveHeartbeat, 1)
	if !send || heartbeat[8+10] != 0x0e {
		t.Fatalf("KeepaliveHeartbeat must send cmd 0x0e, got 0x%02X", heartbeat[8+10])
	}
}

func TestParseCollectsPacketHeaders(t *testing.T) {
	rcFrame := buildRCFrame(7, make([]byte, 17))
	// A second, non-RC packet that requests an ack (cmd_type bit 5 set).
	other := buildCommand(6, 2, 42, 0x20, 0x0A, 0x0B)
	data := append(rcFrame, other[:]...)

	var channels [maxChannels]int16
	var raw [rawPayloadMax]byte
	var packets [maxParsePackets]PacketObj
	_, _, _, count := parseStream(data, &channels, &raw, &packets)

	if count != 2 {
		t.Fatalf("packet count = %d, want 2", count)
	}
	if packets[0].CmdSet != rcCmdSet || packets[0].CmdID != rcCmdID || packets[0].Seq != 7 {
		t.Fatalf("first packet header wrong: %+v", packets[0])
	}
	req := packets[1]
	if req.CmdSet != 0x0A || req.CmdID != 0x0B || req.AckType() != 1 || req.IsResponse() {
		t.Fatalf("ack-request header wrong: %+v", req)
	}

	ack := buildResponse(req, nil)
	pkt := ack[8:]
	if pkt[4] != req.Receiver || pkt[5] != req.Sender {
		t.Fatal("ack addressing not mirrored")
	}
	if binary.LittleEndian.Uint16(pkt[6:8]) != req.Seq {
		t.Fatal("ack sequence not echoed")
	}
	if pkt[8]&0x80 == 0 || pkt[9] != req.CmdSet || pkt[10] != req.CmdID {
		t.Fatal("ack command or response flag wrong")
	}
	if pkt[3] != crc8(pkt[:3]) || binary.LittleEndian.Uint16(pkt[11:13]) != crc16(pkt[:dumlHeaderLen]) {
		t.Fatal("ack CRC wrong")
	}
}

func TestBuildResponseWithPayload(t *testing.T) {
	req := PacketObj{Sender: 6, Receiver: 2, Seq: 0x1234, CmdSet: pollHeartbeatSet, CmdID: pollHeartbeatID}
	req.Len = copy(req.Payload[:], []byte{0x04, 0x93, 0x00})

	frame := buildResponse(req, req.Payload[:req.Len])
	if int(binary.LittleEndian.Uint32(frame[4:8])) != dumlHeaderLen+3+dumlCRCLen {
		t.Fatalf("wrong transport length: %d", binary.LittleEndian.Uint32(frame[4:8]))
	}

	pkt := frame[8:]
	if pkt[4] != 2 || pkt[5] != 6 {
		t.Fatal("addressing not mirrored")
	}
	if binary.LittleEndian.Uint16(pkt[6:8]) != 0x1234 || pkt[8]&0x80 == 0 {
		t.Fatal("seq or response flag wrong")
	}
	body := pkt[dumlHeaderLen : dumlHeaderLen+3]
	if body[0] != 0x04 || body[1] != 0x93 || body[2] != 0x00 {
		t.Fatalf("payload not echoed: % X", body)
	}
	crcAt := dumlHeaderLen + 3
	if binary.LittleEndian.Uint16(pkt[crcAt:crcAt+2]) != crc16(pkt[:crcAt]) {
		t.Fatal("payload response CRC wrong")
	}
}

func TestIsPoll(t *testing.T) {
	if !isPoll(PacketObj{CmdSet: 0x00, CmdID: 0x77}) {
		t.Fatal("00:77 must be a poll")
	}
	if !isPoll(PacketObj{CmdSet: 0x18, CmdID: 0x40}) {
		t.Fatal("18:40 must be a poll")
	}
	if isPoll(PacketObj{CmdSet: 0x06, CmdID: 0xAE}) {
		t.Fatal("stick data must not be a poll")
	}
	if isPoll(PacketObj{CmdSet: 0x00, CmdID: 0x77, CmdType: 0x80}) {
		t.Fatal("a response must not be treated as a poll")
	}
}
