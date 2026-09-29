package l2tp

// Design: docs/architecture/wire/l2tp.md -- unrecognized mandatory AVP handling
//
// RFC 2661 Section 4.1: "If the M bit is set on an unrecognized AVP within a
// message associated with a particular session, the session associated with
// this message MUST be terminated. If the M bit is set on an unrecognized AVP
// within a message associated with the overall tunnel, the entire tunnel (and
// all sessions within) MUST be terminated."
//
// These tests exercise the two shapes the vendor-AVP tests do not reach: an
// IETF AVP (vendor 0) whose attribute type Ze does not recognize, and a
// tunnel-scoped HELLO on an established tunnel.
// Related: avp.go (AVPIterator.Next), session_fsm.go, tunnel_fsm.go.

import (
	"log/slog"
	"testing"
	"time"
)

// unknownIETFAttr is an IETF attribute type RFC 2661 does not define.
const unknownIETFAttr AVPType = 200

// icrqWithUnknownIETFAVP returns an ICRQ body that ends with an IETF AVP of
// the undefined attribute type unknownIETFAttr, its M bit set to mandatory.
func icrqWithUnknownIETFAVP(mandatory bool) []byte {
	buf := make([]byte, 128)
	off := WriteAVPUint16(buf, 0, true, AVPMessageType, uint16(MsgICRQ))
	off += WriteAVPUint16(buf, off, true, AVPAssignedSessionID, 500)
	off += WriteAVPUint32(buf, off, true, AVPCallSerialNumber, 1001)
	off += WriteAVPBytes(buf, off, mandatory, 0, unknownIETFAttr, []byte{0x01})
	return buf[:off]
}

// RFC requirement: RFC2661-4.1-3 positive -- an ICRQ carrying an IETF AVP of
// an attribute type RFC 2661 does not define, with M=1, terminates the session
// it would open: handleICRQ answers with one CDN, no session exists, and the
// tunnel stays established.
// RFC requirement: RFC2661-4.1-3 negative -- the same AVP with M=0 is ignored:
// handleICRQ answers with one ICRP and the session exists.
//
// VALIDATES: an unrecognized mandatory IETF AVP ends the session only.
// PREVENTS: the parser skipping an unrecognized mandatory IETF AVP as though
// its M bit were clear.
func TestUnrecognizedMandatoryIETFAVPInICRQTerminatesSession(t *testing.T) {
	tun := newEstablishedTunnel(t, 0)
	out := tun.handleICRQ(icrqWithUnknownIETFAVP(true), time.Now(), slog.Default())
	if len(out) != 1 {
		t.Fatalf("unrecognized M=1 IETF AVP: %d datagrams, want one CDN", len(out))
	}
	if mt := sentMessageType(t, out[0].bytes); mt != MsgCDN {
		t.Fatalf("unrecognized M=1 IETF AVP: answered with message type %d, want CDN", mt)
	}
	if tun.sessionCount() != 0 {
		t.Fatalf("unrecognized M=1 IETF AVP: session created")
	}
	if tun.state != L2TPTunnelEstablished {
		t.Fatalf("unrecognized M=1 IETF AVP in a session message closed the tunnel: %s", tun.state)
	}

	tun = newEstablishedTunnel(t, 0)
	out = tun.handleICRQ(icrqWithUnknownIETFAVP(false), time.Now(), slog.Default())
	if len(out) != 1 {
		t.Fatalf("unrecognized M=0 IETF AVP: %d datagrams, want one ICRP", len(out))
	}
	if mt := sentMessageType(t, out[0].bytes); mt != MsgICRP {
		t.Fatalf("unrecognized M=0 IETF AVP: answered with message type %d, want ICRP", mt)
	}
	if tun.sessionCount() != 1 {
		t.Fatalf("unrecognized M=0 IETF AVP: %d sessions, want 1", tun.sessionCount())
	}
}

// helloWith returns a HELLO body whose Message Type AVP is followed by one AVP
// written by extra.
func helloWith(extra func(buf []byte, off int) int) []byte {
	buf := make([]byte, 64)
	off := WriteAVPUint16(buf, 0, true, AVPMessageType, uint16(MsgHello))
	off += extra(buf, off)
	return buf[:off]
}

// helloAVPCases are the three unrecognized AVP shapes a HELLO can carry: a
// vendor AVP, an IETF AVP of an undefined type, and a defined IETF AVP with a
// reserved header bit set. Each writer takes the M bit.
var helloAVPCases = []struct {
	name  string
	write func(mandatory bool) func(buf []byte, off int) int
}{
	{"vendor", func(m bool) func([]byte, int) int {
		return func(buf []byte, off int) int { return WriteAVPBytes(buf, off, m, 9999, AVPType(1), []byte{0x01}) }
	}},
	{"undefined IETF type", func(m bool) func([]byte, int) int {
		return func(buf []byte, off int) int { return WriteAVPBytes(buf, off, m, 0, unknownIETFAttr, []byte{0x01}) }
	}},
	{"reserved bit", func(m bool) func([]byte, int) int {
		return func(buf []byte, off int) int {
			n := WriteAVPBytes(buf, off, m, 0, AVPFirmwareRevision, []byte{0x01, 0x02})
			buf[off] |= 0x04 // a reserved bit of the AVP header
			return n
		}
	}},
}

// RFC requirement: RFC2661-4.1-4 positive -- a HELLO (a message associated
// with the overall tunnel) carrying an unrecognized AVP with M=1, whether a
// vendor AVP, an undefined IETF type or an AVP with a reserved bit set, on an
// established tunnel with one session: handleMessage sends one StopCCN, the
// session is gone and the tunnel is closed.
// RFC requirement: RFC2661-4.1-4 negative -- the same three AVPs with M=0:
// handleMessage sends nothing, and the tunnel stays established with its
// session.
// RFC requirement: RFC2661-4.1-1 positive -- a HELLO AVP with a reserved bit
// set to 1 is treated as unrecognized: with M=1 it clears the tunnel, with M=0
// it is ignored, exactly as the unrecognized vendor and IETF AVPs are.
//
// VALIDATES: HELLO bodies go through the AVP iterator and the M-bit rule.
// PREVENTS: handleMessage acknowledging a HELLO without reading its body.
func TestUnrecognizedMandatoryAVPInHelloClearsTunnel(t *testing.T) {
	now := time.Now()
	for _, tc := range helloAVPCases {
		for _, mandatory := range []bool{true, false} {
			tun := newEstablishedTunnel(t, 0)
			tun.handleICRQ(buildICRQ(500, 1001), now, slog.Default())
			if tun.sessionCount() != 1 {
				t.Fatalf("%s: setup: %d sessions, want 1", tc.name, tun.sessionCount())
			}
			// The ICRP fills the one-message congestion window; acknowledge it
			// so a StopCCN leaves at once rather than queueing behind it.
			ackAll(t, tun, now)

			out := tun.handleMessage(RecvEntry{
				MessageType:          uint16(MsgHello),
				Payload:              helloWith(tc.write(mandatory)),
				MessageTypeMandatory: true,
			}, now, TunnelDefaults{}, nil)
			if !mandatory {
				if len(out) != 0 || tun.sessionCount() != 1 || tun.state != L2TPTunnelEstablished {
					t.Fatalf("%s M=0: %d datagrams, %d sessions, tunnel %s; want none, 1, established",
						tc.name, len(out), tun.sessionCount(), tun.state)
				}
				continue
			}
			if len(out) != 1 {
				t.Fatalf("%s M=1: %d datagrams, want one StopCCN", tc.name, len(out))
			}
			if mt := sentMessageType(t, out[0].bytes); mt != MsgStopCCN {
				t.Fatalf("%s M=1: answered with message type %d, want StopCCN", tc.name, mt)
			}
			if tun.sessionCount() != 0 {
				t.Fatalf("%s M=1: %d sessions survived", tc.name, tun.sessionCount())
			}
			if tun.state != L2TPTunnelClosed {
				t.Fatalf("%s M=1: tunnel %s, want closed", tc.name, tun.state)
			}
		}
	}
}
