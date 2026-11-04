package streamgate

import (
	"encoding/binary"
	"io"
	"net/netip"

	"tailscale.com/net/packet"
	"tailscale.com/types/ipproto"
)

type synEvidence struct {
	syn, reply bool
}

// readSYNEvidence consumes the pinned SDK capture format without recording
// frames. Its caller owns the stream and bounds its lifetime.
func readSYNEvidence(r io.Reader, controller, target netip.Addr, atTarget bool, observe func(synEvidence)) synEvidence {
	if !readSYNHeader(r) {
		return synEvidence{}
	}
	return readSYNRecords(r, controller, target, atTarget, observe)
}

func readSYNHeader(r io.Reader) bool {
	var header [24]byte
	le := binary.LittleEndian
	return readFull(r, header[:]) && le.Uint32(header[:4]) == 0xa1b2c3d4 &&
		le.Uint16(header[4:6]) == 2 && le.Uint16(header[6:8]) == 4 && le.Uint32(header[20:24]) == 147
}

func readSYNRecords(r io.Reader, controller, target netip.Addr, atTarget bool, observe func(synEvidence)) synEvidence {
	var evidence synEvidence
	if !controller.Is4() || !target.Is4() || controller == target {
		return evidence
	}
	le := binary.LittleEndian
	var header [16]byte
	synPath, replyPath := packet.SynthesizedToPeer, packet.FromPeer
	if atTarget {
		synPath, replyPath = replyPath, synPath
	}
	var frame [65535]byte
	var sourcePort uint16
	for range 256 {
		if !readFull(r, header[:16]) {
			break
		}
		n := le.Uint32(header[8:12])
		if n < 4 || n > uint32(len(frame)) || n != le.Uint32(header[12:16]) || !readFull(r, frame[:n]) {
			break
		}
		path := packet.CapturePath(le.Uint16(frame[:2]))
		// This signed overlay flow has neither SNAT nor DNAT metadata.
		if frame[2] != 0 || frame[3] != 0 {
			continue
		}
		var p packet.Parsed
		p.Decode(frame[4:n])
		if p.IPVersion != 4 || p.IPProto != ipproto.TCP {
			continue
		}
		flags := p.TCPFlags &^ packet.TCPECNBits
		if !evidence.syn && path == synPath && flags == packet.TCPSyn &&
			p.Src.Addr() == controller && p.Src.Port() != 0 && p.Dst == netip.AddrPortFrom(target, 47476) {
			evidence.syn, sourcePort = true, p.Src.Port()
			if observe != nil {
				observe(evidence)
			}
		}
		if evidence.syn && path == replyPath && flags == packet.TCPSynAck &&
			p.Src == netip.AddrPortFrom(target, 47476) && p.Dst == netip.AddrPortFrom(controller, sourcePort) {
			evidence.reply = true
			if observe != nil {
				observe(evidence)
			}
			break
		}
	}
	return evidence
}

func readFull(r io.Reader, data []byte) bool {
	_, err := io.ReadFull(r, data)
	return err == nil
}
