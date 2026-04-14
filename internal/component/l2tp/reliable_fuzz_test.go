package l2tp

import (
	"encoding/binary"
	"testing"
	"time"
)

// FuzzOnReceiveSequence feeds arbitrary (Ns, Nr, flags, payload-size)
// tuples into a fresh engine to surface any panic or infinite loop in
// the classifier. The wire-level field parsing is already fuzzed at
// phase 1 (FuzzParseMessageHeader); this target focuses on the engine's
// state-machine reactions to valid headers with unusual sequence values.
//
// Seed corpus covers wraparound edges (0, 1, 65534, 65535), the
// half-space boundary (32767, 32768), and in-order/duplicate/reorder
// classes.
func FuzzOnReceiveSequence(f *testing.F) {
	seeds := []struct {
		ns, nr      uint16
		payloadLen  int
		nextRecvSet uint16
	}{
		{0, 0, 8, 0},
		{1, 0, 8, 0},
		{0, 1, 0, 0}, // ZLB ack
		{65535, 0, 8, 0},
		{0, 65535, 8, 65534},
		{32767, 0, 8, 0},
		{32768, 0, 8, 0},
		{65535, 65535, 8, 65534},
		{2, 0, 8, 0},   // reorder-queued (nextRecv=0)
		{100, 0, 8, 0}, // beyond window -> discarded
	}
	for _, s := range seeds {
		f.Add(s.ns, s.nr, s.payloadLen, s.nextRecvSet)
	}

	f.Fuzz(func(t *testing.T, ns, nr uint16, payloadLen int, nextRecvSet uint16) {
		if payloadLen < 0 || payloadLen > 1024 {
			return
		}
		e := newTestEngine()
		e.nextRecvSeq = nextRecvSet

		payload := make([]byte, payloadLen)
		// If payload is at least 8 bytes, populate a valid Message Type
		// AVP so makeRecvEntry reads something sensible.
		if payloadLen >= 8 {
			binary.BigEndian.PutUint16(payload[0:2], 0x8008)
			binary.BigEndian.PutUint16(payload[6:8], 1)
		}
		hdr := MessageHeader{
			IsControl:   true,
			HasLength:   true,
			HasSequence: true,
			Version:     2,
			TunnelID:    100,
			Ns:          ns,
			Nr:          nr,
		}
		// Must not panic or loop.
		_ = e.OnReceive(hdr, payload, time.Unix(0, 0))
	})
}
