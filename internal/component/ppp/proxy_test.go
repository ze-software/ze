package ppp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

// buildOptionStream serializes a small option list for proxy LCP tests.
func buildOptionStream(opts []LCPOption) []byte {
	buf := make([]byte, 256)
	n := WriteLCPOptions(buf, 0, opts)
	return buf[:n]
}

func mruOpt(v uint16) LCPOption {
	d := make([]byte, 2)
	binary.BigEndian.PutUint16(d, v)
	return LCPOption{Type: LCPOptMRU, Data: d}
}

func magicOpt(v uint32) LCPOption {
	d := make([]byte, 4)
	binary.BigEndian.PutUint32(d, v)
	return LCPOption{Type: LCPOptMagic, Data: d}
}

func authProtoOpt(proto uint16, extra ...byte) LCPOption {
	d := make([]byte, 2+len(extra))
	binary.BigEndian.PutUint16(d[:2], proto)
	copy(d[2:], extra)
	return LCPOption{Type: LCPOptAuthProto, Data: d}
}

// VALIDATES: EvaluateProxyLCP extracts MRU, AuthProto, and PeerMagic
//
//	from a typical PAP-authenticated LAC<->peer state.
func TestProxyLCPHappyPath(t *testing.T) {
	initial := buildOptionStream([]LCPOption{
		mruOpt(1492),
	})
	lastSent := buildOptionStream([]LCPOption{
		mruOpt(1500),
		authProtoOpt(0xC023), // PAP
	})
	lastRecv := buildOptionStream([]LCPOption{
		mruOpt(1492),
		magicOpt(0xDEADBEEF),
	})

	res, err := EvaluateProxyLCP(initial, lastSent, lastRecv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.MRU != 1492 {
		t.Errorf("MRU = %d, want 1492 (peer's Last-Received MRU)", res.MRU)
	}
	if res.AuthProto != 0xC023 {
		t.Errorf("AuthProto = 0x%04x, want 0xC023 (PAP)", res.AuthProto)
	}
	if res.PeerMagic != 0xDEADBEEF {
		t.Errorf("PeerMagic = 0x%08x, want 0xDEADBEEF", res.PeerMagic)
	}
	if len(res.AuthData) != 0 {
		t.Errorf("AuthData = %x, want empty for PAP", res.AuthData)
	}
}

// VALIDATES: EvaluateProxyLCP captures the algorithm byte for CHAP.
func TestProxyLCPCHAPAlgorithm(t *testing.T) {
	stream := buildOptionStream([]LCPOption{
		authProtoOpt(0xC223, 0x05), // CHAP-MD5
	})
	res, err := EvaluateProxyLCP(stream, stream, stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.AuthProto != 0xC223 {
		t.Errorf("AuthProto = 0x%04x, want 0xC223", res.AuthProto)
	}
	if !bytes.Equal(res.AuthData, []byte{0x05}) {
		t.Errorf("AuthData = %x, want [0x05] (CHAP-MD5)", res.AuthData)
	}
}

// VALIDATES: EvaluateProxyLCP rejects when any of the three slices
//
//	is empty (RFC 2661 §18 requires all three).
func TestProxyLCPMissingAVP(t *testing.T) {
	good := buildOptionStream([]LCPOption{mruOpt(1500)})
	cases := []struct {
		name                        string
		initial, lastSent, lastRecv []byte
	}{
		{"initial empty", nil, good, good},
		{"lastSent empty", good, nil, good},
		{"lastRecv empty", good, good, nil},
		{"all empty", nil, nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := EvaluateProxyLCP(tc.initial, tc.lastSent, tc.lastRecv)
			if !errors.Is(err, errProxyLCPMissing) {
				t.Errorf("err = %v, want errProxyLCPMissing", err)
			}
		})
	}
}

// VALIDATES: Malformed option streams fall back to errProxyLCPInvalid
//
//	(caller MUST run normal LCP).
func TestProxyLCPMalformed(t *testing.T) {
	good := buildOptionStream([]LCPOption{mruOpt(1500)})
	bad := []byte{LCPOptMRU, 0xFF, 0xAA} // length 0xFF exceeds buf

	cases := []struct {
		name                        string
		initial, lastSent, lastRecv []byte
	}{
		{"initial malformed", bad, good, good},
		{"lastSent malformed", good, bad, good},
		{"lastRecv malformed", good, good, bad},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := EvaluateProxyLCP(tc.initial, tc.lastSent, tc.lastRecv)
			if !errors.Is(err, errProxyLCPInvalid) {
				t.Errorf("err = %v, want errProxyLCPInvalid", err)
			}
		})
	}
}

// VALIDATES: When LAC's Last-Sent CONFREQ omits Auth-Protocol,
//
//	AuthProto is zero (no auth negotiated). Phase 6b's auth phase
//	will then accept the session without a wire exchange.
func TestProxyLCPNoAuth(t *testing.T) {
	stream := buildOptionStream([]LCPOption{mruOpt(1500), magicOpt(0x12345678)})
	res, err := EvaluateProxyLCP(stream, stream, stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.AuthProto != 0 {
		t.Errorf("AuthProto = 0x%04x, want 0 (no auth)", res.AuthProto)
	}
}

// VALIDATES: Boundary -- Auth-Protocol option with only 1 byte (less
//
//	than the 2-byte uint16) is treated as no-auth (defensive).
func TestProxyLCPShortAuthProto(t *testing.T) {
	short := LCPOption{Type: LCPOptAuthProto, Data: []byte{0xC0}}
	stream := buildOptionStream([]LCPOption{mruOpt(1500), short})
	res, err := EvaluateProxyLCP(stream, stream, stream)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.AuthProto != 0 {
		t.Errorf("AuthProto = 0x%04x, want 0 for malformed short option", res.AuthProto)
	}
}

// VALIDATES: lookupMRUOption / lookupOptionUint32 helpers handle
//
//	missing or short options as (0, false).
func TestProxyLCPHelpers(t *testing.T) {
	opts := []LCPOption{mruOpt(1500), magicOpt(0xCAFEBABE)}
	if v, ok := lookupMRUOption(opts); !ok || v != 1500 {
		t.Errorf("MRU lookup wrong: v=%d ok=%v", v, ok)
	}
	if v, ok := lookupOptionUint32(opts, LCPOptMagic); !ok || v != 0xCAFEBABE {
		t.Errorf("Magic lookup wrong: v=0x%08x ok=%v", v, ok)
	}
	if _, ok := lookupOption(opts, LCPOptAuthProto); ok {
		t.Errorf("AuthProto absent expected; got present")
	}
	short := []LCPOption{{Type: LCPOptMRU, Data: []byte{0x05}}}
	if _, ok := lookupMRUOption(short); ok {
		t.Errorf("short MRU should fail lookup")
	}
}
