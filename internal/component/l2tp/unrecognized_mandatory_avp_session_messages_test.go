package l2tp

// Design: docs/architecture/wire/l2tp.md -- unrecognized mandatory AVP handling
//
// RFC 2661 Section 4.1: "If the M bit is set on an unrecognized AVP within a
// message associated with a particular session, the session associated with
// this message MUST be terminated."
//
// VALIDATES: WEN and SLI are messages associated with a particular session
// (RFC 2661 Sections 6.15 and 6.16), so an unrecognized AVP with M=1 in either
// one, IETF or vendor-specific, terminates that session with a CDN. The refusal
// stays scoped to that session: the tunnel and a sibling session survive, and
// the same AVP with M=0 changes nothing.
// PREVENTS: handleWEN and handleSLI logging "malformed; ignored" for an
// unrecognized mandatory AVP and keeping the session.
// Related: session_fsm.go (handleWEN, handleSLI, parseSingleAVPMessage).

import (
	"log/slog"
	"testing"
	"time"
)

// unknownVendorID is a vendor Ze defines no AVP for.
const unknownVendorID = 9999

// withUnknownAVP appends one AVP Ze does not recognize to body: the undefined
// IETF type unknownIETFAttr when vendorID is 0, else type 1 of vendorID. Its M
// bit is set when mandatory is true.
func withUnknownAVP(body []byte, vendorID uint16, mandatory bool) []byte {
	buf := make([]byte, len(body)+16)
	off := copy(buf, body)
	attr := unknownIETFAttr
	if vendorID != 0 {
		attr = AVPType(1)
	}
	off += WriteAVPBytes(buf, off, mandatory, vendorID, attr, []byte{0x01})
	return buf[:off]
}

// establishSession brings one incoming session up on tun, acknowledges every
// message tun sent, and answers the session's local ID.
func establishSession(t *testing.T, tun *L2TPTunnel, remoteSID uint16, serial uint32, now time.Time) uint16 {
	t.Helper()
	before := make(map[uint16]bool, len(tun.sessions))
	for sid := range tun.sessions {
		before[sid] = true
	}
	tun.handleICRQ(buildICRQ(remoteSID, serial), now, slog.Default())
	var sess *L2TPSession
	for sid, s := range tun.sessions {
		if !before[sid] {
			sess = s
		}
	}
	if sess == nil {
		t.Fatal("setup: the ICRQ created no session")
	}
	tun.handleICCN(sess, buildICCN(10000000, 2), now, slog.Default())
	if sess.state != L2TPSessionEstablished {
		t.Fatalf("setup: session %s, want established", sess.state)
	}
	ackAll(t, tun, now)
	return sess.localSID
}

// sessionMessage is one session-scoped message Ze parses through
// parseSingleAVPMessage, and the handler that receives it.
type sessionMessage struct {
	name   string
	body   []byte
	handle func(tun *L2TPTunnel, sid uint16, payload []byte, now time.Time) []sendRequest
}

func wenAndSLI() []sessionMessage {
	return []sessionMessage{
		{"WEN", buildWEN(CallErrorsValue{}), func(tun *L2TPTunnel, sid uint16, payload []byte, now time.Time) []sendRequest {
			return tun.handleWEN(sid, payload, now, slog.Default())
		}},
		{"SLI", buildSLI(ACCMValue{}), func(tun *L2TPTunnel, sid uint16, payload []byte, now time.Time) []sendRequest {
			return tun.handleSLI(sid, payload, now, slog.Default())
		}},
	}
}

// RFC requirement: RFC2661-4.1-3 positive -- a WEN or an SLI, both messages
// associated with a particular session, carrying an unrecognized AVP with M=1
// (IETF type 200, or vendor 9999 type 1) terminates that session: exactly one
// datagram, a CDN, and the session is gone from the tunnel.
func TestRFC2661UnrecognizedMandatoryAVPInWENOrSLITerminatesSession(t *testing.T) {
	now := time.Now()
	for _, msg := range wenAndSLI() {
		for _, vendorID := range []uint16{0, unknownVendorID} {
			tun := newEstablishedTunnel(t, 0)
			sid := establishSession(t, tun, 500, 1001, now)

			out := msg.handle(tun, sid, withUnknownAVP(msg.body, vendorID, true), now)
			if len(out) != 1 {
				t.Fatalf("%s, vendor %d, unrecognized M=1 AVP: %d datagrams, want one CDN", msg.name, vendorID, len(out))
			}
			if mt := sentMessageType(t, out[0].bytes); mt != MsgCDN {
				t.Fatalf("%s, vendor %d, unrecognized M=1 AVP: answered with message type %d, want CDN", msg.name, vendorID, mt)
			}
			if tun.lookupSession(sid) != nil {
				t.Fatalf("%s, vendor %d, unrecognized M=1 AVP: the session survived", msg.name, vendorID)
			}
		}
	}
}

// sessionOf answers the one session tun holds.
func sessionOf(t *testing.T, tun *L2TPTunnel) *L2TPSession {
	t.Helper()
	if len(tun.sessions) != 1 {
		t.Fatalf("setup: %d sessions, want 1", len(tun.sessions))
	}
	for _, s := range tun.sessions {
		return s
	}
	return nil
}

// RFC requirement: RFC2661-4.1-3 positive -- every other session-scoped parser
// Ze runs (OCRQ, ICRP, OCRP, ICCN, OCCN) answers an unrecognized M=1 AVP, IETF
// or vendor-specific, with exactly one CDN and leaves no session behind.
func TestRFC2661UnrecognizedMandatoryAVPInEverySessionParserTerminatesSession(t *testing.T) {
	now := time.Now()
	logger := slog.Default()
	for _, tc := range []struct {
		name string
		run  func(t *testing.T, tun *L2TPTunnel, vendorID uint16) []sendRequest
	}{
		{"OCRQ", func(t *testing.T, tun *L2TPTunnel, vendorID uint16) []sendRequest {
			return tun.handleOCRQ(withUnknownAVP(buildOCRQ(500, 1001), vendorID, true), now, logger)
		}},
		{"ICRP", func(t *testing.T, tun *L2TPTunnel, vendorID uint16) []sendRequest {
			tun.placeIncomingCall(now, callParams{callSerial: 1, framingType: 1}, logger)
			ackAll(t, tun, now)
			return tun.handleICRP(sessionOf(t, tun), withUnknownAVP(icrpBody(9), vendorID, true), now, logger)
		}},
		{"OCRP", func(t *testing.T, tun *L2TPTunnel, vendorID uint16) []sendRequest {
			tun.placeOutgoingCall(now, callParams{callSerial: 7, minBPS: 9600, maxBPS: 128000, bearerType: 1, framingType: 1}, logger)
			ackAll(t, tun, now)
			return tun.handleOCRP(sessionOf(t, tun), withUnknownAVP(ocrpBody(9), vendorID, true), now, logger)
		}},
		{"ICCN", func(t *testing.T, tun *L2TPTunnel, vendorID uint16) []sendRequest {
			tun.handleICRQ(buildICRQ(500, 1001), now, logger)
			ackAll(t, tun, now)
			return tun.handleICCN(sessionOf(t, tun), withUnknownAVP(buildICCN(10000000, 2), vendorID, true), now, logger)
		}},
		{"OCCN", func(t *testing.T, tun *L2TPTunnel, vendorID uint16) []sendRequest {
			// Ze receives an OCCN as the LNS of the outgoing call it placed,
			// once the OCRP moved the session to wait-connect.
			tun.placeOutgoingCall(now, callParams{callSerial: 7, minBPS: 9600, maxBPS: 128000, bearerType: 1, framingType: 1}, logger)
			ackAll(t, tun, now)
			tun.handleOCRP(sessionOf(t, tun), ocrpBody(808), now, logger)
			ackAll(t, tun, now)
			return tun.handleOCCN(sessionOf(t, tun), withUnknownAVP(buildOCCN(10000000, 2), vendorID, true), now, logger)
		}},
	} {
		for _, vendorID := range []uint16{0, unknownVendorID} {
			tun := newEstablishedTunnel(t, 0)
			out := tc.run(t, tun, vendorID)
			if len(out) != 1 {
				t.Fatalf("%s, vendor %d, unrecognized M=1 AVP: %d datagrams, want one CDN", tc.name, vendorID, len(out))
			}
			if mt := sentMessageType(t, out[0].bytes); mt != MsgCDN {
				t.Fatalf("%s, vendor %d, unrecognized M=1 AVP: answered with message type %d, want CDN", tc.name, vendorID, mt)
			}
			if n := len(tun.sessions); n != 0 {
				t.Fatalf("%s, vendor %d, unrecognized M=1 AVP: %d sessions remain, want 0", tc.name, vendorID, n)
			}
		}
	}
}

// RFC requirement: RFC2661-4.1-3 negative -- the termination is the session's
// alone: with two sessions up, the unrecognized M=1 AVP on one session never
// escalates to a StopCCN, the tunnel stays established and the sibling session
// keeps running; the same AVP with M=0 sends nothing and keeps the session.
func TestRFC2661UnrecognizedMandatoryAVPInWENOrSLIEndsOnlyThatSession(t *testing.T) {
	now := time.Now()
	for _, msg := range wenAndSLI() {
		tun := newEstablishedTunnel(t, 0)
		sid := establishSession(t, tun, 500, 1001, now)
		sibling := establishSession(t, tun, 501, 1002, now)

		out := msg.handle(tun, sid, withUnknownAVP(msg.body, 0, false), now)
		if len(out) != 0 {
			t.Fatalf("%s with an unrecognized M=0 AVP: %d datagrams, want none", msg.name, len(out))
		}
		if tun.lookupSession(sid) == nil {
			t.Fatalf("%s with an unrecognized M=0 AVP ended the session", msg.name)
		}

		out = msg.handle(tun, sid, withUnknownAVP(msg.body, 0, true), now)
		if len(out) != 1 {
			t.Fatalf("%s with an unrecognized M=1 AVP: %d datagrams, want one CDN", msg.name, len(out))
		}
		if mt := sentMessageType(t, out[0].bytes); mt != MsgCDN {
			t.Fatalf("%s with an unrecognized M=1 AVP: answered with message type %d, want CDN, never StopCCN", msg.name, mt)
		}
		if tun.state != L2TPTunnelEstablished {
			t.Fatalf("%s with an unrecognized M=1 AVP closed the tunnel: %s", msg.name, tun.state)
		}
		if tun.lookupSession(sibling) == nil {
			t.Fatalf("%s with an unrecognized M=1 AVP on session %d ended sibling session %d", msg.name, sid, sibling)
		}
	}
}
