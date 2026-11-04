package streamgate

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"
)

func TestSYNEvidenceRequiresMatchingReverseTuple(t *testing.T) {
	controller := netip.MustParseAddr("100.120.0.2")
	target := netip.MustParseAddr("100.120.0.1")
	var capture bytes.Buffer
	le := binary.LittleEndian
	be := binary.BigEndian
	var header [24]byte
	le.PutUint32(header[:4], 0xa1b2c3d4)
	le.PutUint16(header[4:6], 2)
	le.PutUint16(header[6:8], 4)
	le.PutUint32(header[20:], 147)
	capture.Write(header[:])
	write := func(path uint16, src, dst netip.Addr, sport, dport uint16, flags byte) {
		var record [60]byte
		le.PutUint32(record[8:12], 44)
		le.PutUint32(record[12:16], 44)
		le.PutUint16(record[16:18], path)
		ip := record[20:]
		ip[0], ip[9] = 0x45, 6
		be.PutUint16(ip[2:4], 40)
		copy(ip[12:16], src.AsSlice())
		copy(ip[16:20], dst.AsSlice())
		be.PutUint16(ip[20:22], sport)
		be.PutUint16(ip[22:24], dport)
		ip[32], ip[33] = 0x50, flags
		capture.Write(record[:])
	}
	write(3, controller, target, 50000, 47476, 2)
	write(1, target, controller, 47476, 50001, 18)
	if got := readSYNEvidence(bytes.NewReader(capture.Bytes()), controller, target, false); !got.syn || got.reply {
		t.Fatalf("foreign reply admitted: %+v", got)
	}
	write(1, target, controller, 47476, 50000, 18)
	if got := readSYNEvidence(&capture, controller, target, false); !got.syn || !got.reply {
		t.Fatalf("matching handshake missed: %+v", got)
	}
}
