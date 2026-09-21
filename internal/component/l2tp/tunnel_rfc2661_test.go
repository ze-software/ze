package l2tp

// Design: tagged proofs of RFC 2661 obligations the tunnel and session FSMs
// already meet: the Tunnel ID and Session ID a header carries before and
// after the peer assigns one, the Challenge and Challenge Response exchange,
// the capability masks an outgoing call is checked against, and the clean-up
// a StopCCN or a CDN performs. Each tag names the exact assertion its body
// makes; a claim wider than the body is a lie the discrimination record
// exists to refuse.
//
// VALIDATES: the tunnel and session FSMs meet the RFC 2661 obligations each
// tag names, on both polarities where the FSM holds a refusing branch.
// PREVENTS: a header stamped with the wrong Tunnel ID or Session ID, an
// unanswered Challenge, an outgoing call the peer's capabilities do not
// admit, and a StopCCN or CDN that is answered with anything but a ZLB.
// Related: message_rfc2661_test.go (header and body codecs), tunnel_fsm.go,
// tunnel_initiator.go, session_fsm.go, session_initiator.go, reliable.go.

import (
	"log/slog"
	"net/netip"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// rfc2661PeerAddr is the peer address every tunnel in this file is created
// with; RFC 2661 Section 8.1 fixes it for the tunnel's life.
var rfc2661PeerAddr = netip.MustParseAddrPort("10.0.0.2:1701")

// rfc2661PeerTID is the Tunnel ID the peer assigns in every SCCRP a dial
// test answers with.
const rfc2661PeerTID uint16 = 777

// controlWire parses one datagram a tunnel emitted and returns its header
// and its AVP body.
func controlWire(t *testing.T, pkt []byte) (MessageHeader, []byte) {
	t.Helper()
	hdr, err := ParseMessageHeader(pkt)
	require.NoError(t, err)
	require.True(t, hdr.IsControl, "not a control message")
	return hdr, pkt[hdr.PayloadOff:hdr.Length]
}

// wireMessageType reads the Message Type AVP that opens body. A ZLB has no
// body and reports 0.
func wireMessageType(t *testing.T, body []byte) MessageType {
	t.Helper()
	if len(body) == 0 {
		return 0
	}
	it := NewAVPIterator(body)
	_, attrType, _, value, ok := it.Next()
	require.True(t, ok, "body carries no AVP: %v", it.Err())
	require.Equal(t, AVPMessageType, attrType, "first AVP is not Message Type")
	mt, err := readAVPUint16(value)
	require.NoError(t, err)
	return MessageType(mt)
}

// findWire returns the header and body of the first datagram of the wanted
// message type among outs, and fails when none is.
func findWire(t *testing.T, outs []sendRequest, want MessageType) (MessageHeader, []byte) {
	t.Helper()
	for _, o := range outs {
		hdr, body := controlWire(t, o.bytes)
		if wireMessageType(t, body) == want {
			return hdr, body
		}
	}
	t.Fatalf("no %d message among %d datagrams", want, len(outs))
	return MessageHeader{}, nil
}

// avpU16 returns the 16-bit value of the first AVP of attrType in body, and
// fails when the AVP is absent.
func avpU16(t *testing.T, body []byte, attrType AVPType) uint16 {
	t.Helper()
	it := NewAVPIterator(body)
	for {
		vendorID, at, _, value, ok := it.Next()
		if !ok {
			t.Fatalf("AVP %d absent from body", attrType)
		}
		if vendorID == 0 && at == attrType {
			v, err := readAVPUint16(value)
			require.NoError(t, err)
			return v
		}
	}
}

// avpBytes returns the value of the first AVP of attrType in body, or nil.
func avpBytes(body []byte, attrType AVPType) []byte {
	it := NewAVPIterator(body)
	for {
		vendorID, at, _, value, ok := it.Next()
		if !ok {
			return nil
		}
		if vendorID == 0 && at == attrType {
			return value
		}
	}
}

// keep clones the bytes of every datagram so they survive the engine's slab
// reuse: a sent slab returns to the pool once the peer acknowledges it.
func keep(outs []sendRequest) []sendRequest {
	kept := make([]sendRequest, len(outs))
	for i, o := range outs {
		kept[i] = sendRequest{to: o.to, bytes: append([]byte(nil), o.bytes...)}
	}
	return kept
}

// ackAll delivers a ZLB from the peer acknowledging every message the tunnel
// has sent, so the next message is not held back by the send window.
func ackAll(t *testing.T, tun *L2TPTunnel, now time.Time) {
	t.Helper()
	zlb := buildZLBAck(tun.localTID, tun.engine.nextRecvSeq, tun.engine.nextSendSeq)
	require.Empty(t, deliver(t, tun, zlb, now, TunnelDefaults{}), "a ZLB produced a reply")
}

// deliver hands one full datagram from the peer to the tunnel, exactly as
// the reactor does after locating the tunnel. An SCCRQ is parsed first, since
// the reactor validates one before it allocates the tunnel.
func deliver(t *testing.T, tun *L2TPTunnel, pkt []byte, now time.Time, defaults TunnelDefaults) []sendRequest {
	t.Helper()
	hdr, body := controlWire(t, pkt)
	var info *sccrqInfo
	if wireMessageType(t, body) == MsgSCCRQ {
		parsed, err := parseSCCRQ(body)
		require.NoError(t, err)
		info = &parsed
	}
	return keep(tun.Process(hdr, body, now, defaults, info))
}

// wrapControl frames body in a control header addressed to tid/sid with the
// given sequence numbers.
func wrapControl(body []byte, tid, sid, ns, nr uint16) []byte {
	pkt := make([]byte, ControlHeaderLen+len(body))
	WriteControlHeader(pkt, 0, uint16(ControlHeaderLen+len(body)), tid, sid, ns, nr) //nolint:gosec // test bodies are small
	copy(pkt[ControlHeaderLen:], body)
	return pkt
}

// dialUntilEstablished dials a tunnel with the given secret, answers its SCCRQ
// with an SCCRP assigning rfc2661PeerTID, and returns the tunnel, its
// defaults, and every datagram it emitted (SCCRQ then SCCCN). With a secret,
// the SCCRP carries the peer's Challenge and a valid response to ours.
func dialUntilEstablished(t *testing.T, now time.Time, secret string) (*L2TPTunnel, TunnelDefaults, []sendRequest) {
	t.Helper()
	defaults := initiatorDefaults(secret)
	tun := newTunnel(100, 0, rfc2661PeerAddr, ReliableConfig{RecvWindow: 8}, slog.Default(), now)
	outs := keep(tun.initiate(now, defaults, nil))
	require.Len(t, outs, 1, "the dial emits one SCCRQ")
	var challenge, response []byte
	if secret != "" {
		challenge = []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
		resp := ChallengeResponse(ChapIDSCCRP, []byte(secret), tun.ourChallenge)
		response = resp[:]
	}
	outs = append(outs, deliver(t, tun, buildSCCRPWire(t, rfc2661PeerTID, challenge, response), now, defaults)...)
	require.Equal(t, L2TPTunnelEstablished, tun.state)
	ackAll(t, tun, now)
	return tun, defaults, outs
}

// RFC requirement: RFC2661-4.4.3-7 positive -- the SCCRQ a dialed tunnel
// emits, before any Assigned Tunnel ID arrived, carries Tunnel ID 0 in its
// control header.
// RFC requirement: RFC2661-5.3-2 positive -- that SCCRQ carries Tunnel ID 0
// in the header and identifies the tunnel by its Assigned Tunnel ID AVP,
// which holds the local TID.
// RFC requirement: RFC2661-4.4.3-6 positive -- once the SCCRP assigned Tunnel
// ID 777, the SCCCN, the next HELLO and the StopCCN each carry 777 in the
// header.
// TestRFC2661HeaderTunnelIDBeforeAndAfterAssignment walks a dial from SCCRQ
// to StopCCN and reads the Tunnel ID of each header the tunnel stamps.
func TestRFC2661HeaderTunnelIDBeforeAndAfterAssignment(t *testing.T) {
	now := time.Now()
	tun, _, outs := dialUntilEstablished(t, now, "")

	sccrqHdr, sccrqBody := findWire(t, outs, MsgSCCRQ)
	require.EqualValues(t, 0, sccrqHdr.TunnelID, "SCCRQ header Tunnel ID")
	require.EqualValues(t, 100, avpU16(t, sccrqBody, AVPAssignedTunnelID), "SCCRQ Assigned Tunnel ID AVP")

	scccnHdr, _ := findWire(t, outs, MsgSCCCN)
	require.EqualValues(t, 777, scccnHdr.TunnelID, "SCCCN header Tunnel ID")

	helloHdr, _ := findWire(t, keep(tun.handleHelloTimer(now)), MsgHello)
	require.EqualValues(t, 777, helloHdr.TunnelID, "HELLO header Tunnel ID")

	stopHdr, _ := findWire(t, tun.teardownStopCCN(now, ResultCodeValue{Result: resultGeneralError}, l2tpevents.TerminateCauseAdminReset), MsgStopCCN)
	require.EqualValues(t, 777, stopHdr.TunnelID, "StopCCN header Tunnel ID")
}

// RFC requirement: RFC2661-4.4.3-7 negative -- a StopCCN emitted before any
// Assigned Tunnel ID arrived (the SCCRP omitted the AVP) carries Tunnel ID 0,
// never the local TID.
// RFC requirement: RFC2661-5.3-2 negative -- the same StopCCN identifies the
// tunnel by its Assigned Tunnel ID AVP, not by a non-zero header Tunnel ID.
// RFC requirement: RFC2661-4.4.3-6 negative -- after the peer assigned Tunnel
// ID 777, no datagram the tunnel emits carries Tunnel ID 0 or the local TID.
// TestRFC2661HeaderTunnelIDNeverWrong drives the two failure shapes: a
// refusal before assignment, and every message after assignment.
func TestRFC2661HeaderTunnelIDNeverWrong(t *testing.T) {
	now := time.Now()
	tun, defaults := dialedTunnel(t, now)
	outs := deliver(t, tun, buildSCCRPWithout(t, AVPAssignedTunnelID, 100, 777), now, defaults)
	require.Equal(t, L2TPTunnelClosed, tun.state, "an SCCRP without Assigned Tunnel ID tears the tunnel down")
	stopHdr, stopBody := findWire(t, outs, MsgStopCCN)
	require.EqualValues(t, 0, stopHdr.TunnelID, "StopCCN before assignment: header Tunnel ID")
	require.EqualValues(t, 100, avpU16(t, stopBody, AVPAssignedTunnelID), "StopCCN before assignment: Assigned Tunnel ID AVP")
	for _, o := range outs {
		hdr, _ := controlWire(t, o.bytes)
		require.EqualValues(t, 0, hdr.TunnelID, "every datagram before assignment carries Tunnel ID 0")
	}

	tun, _, outs = dialUntilEstablished(t, now, "")
	after := outs[1:] // the SCCRQ precedes the assignment
	after = append(after, keep(tun.handleHelloTimer(now))...)
	ackAll(t, tun, now)
	after = append(after, keep(tun.teardownStopCCN(now, ResultCodeValue{Result: resultGeneralError}, l2tpevents.TerminateCauseAdminReset))...)
	require.GreaterOrEqual(t, len(after), 3, "SCCCN, HELLO and StopCCN")
	for _, o := range after {
		hdr, body := controlWire(t, o.bytes)
		require.NotEqualValues(t, 0, hdr.TunnelID, "message %d after assignment carries Tunnel ID 0", wireMessageType(t, body))
		require.NotEqualValues(t, tun.localTID, hdr.TunnelID, "message %d after assignment carries the local TID", wireMessageType(t, body))
	}
}

// RFC requirement: RFC2661-4.4.3-8 positive -- the Assigned Tunnel ID AVP of
// the StopCCN equals the Assigned Tunnel ID AVP the SCCRQ first sent.
// RFC requirement: RFC2661-4.4.3-8 negative -- that AVP is not the Tunnel ID
// the peer assigned, even though the header now carries the peer's.
// TestRFC2661StopCCNRepeatsFirstAssignedTunnelID compares the two AVPs on a
// tunnel whose peer assigned 777.
func TestRFC2661StopCCNRepeatsFirstAssignedTunnelID(t *testing.T) {
	now := time.Now()
	tun, _, outs := dialUntilEstablished(t, now, "")
	_, sccrqBody := findWire(t, outs, MsgSCCRQ)
	first := avpU16(t, sccrqBody, AVPAssignedTunnelID)

	stopHdr, stopBody := findWire(t, tun.teardownStopCCN(now, ResultCodeValue{Result: resultGeneralError}, l2tpevents.TerminateCauseAdminReset), MsgStopCCN)
	got := avpU16(t, stopBody, AVPAssignedTunnelID)
	require.Equal(t, first, got, "StopCCN Assigned Tunnel ID AVP differs from the SCCRQ's")
	require.EqualValues(t, 777, stopHdr.TunnelID, "the header carries the peer's TID")
	require.NotEqualValues(t, 777, got, "the AVP carries the peer's TID instead of ours")
}

// RFC requirement: RFC2661-4.4.3-9 positive -- an SCCRP carrying a Challenge
// AVP is answered by an SCCCN carrying a Challenge Response AVP whose value
// is the CHAP-MD5 response over the shared secret with the SCCCN ID.
// RFC requirement: RFC2661-5.1.1-1 positive -- the Challenge received in the
// SCCRP is answered in the following SCCCN.
// RFC requirement: RFC2661-5.1.1-3 positive -- with one shared secret
// configured, the peer's SCCRP response verifies and the tunnel establishes.
// TestRFC2661InitiatorAnswersChallenge dials with a shared secret and reads
// the SCCCN the valid SCCRP produces.
func TestRFC2661InitiatorAnswersChallenge(t *testing.T) {
	now := time.Now()
	const secret = "s3cret"
	tun, _, outs := dialUntilEstablished(t, now, secret)
	require.Equal(t, L2TPTunnelEstablished, tun.state)
	_, scccnBody := findWire(t, outs, MsgSCCCN)
	got := avpBytes(scccnBody, AVPChallengeResponse)
	require.NotNil(t, got, "SCCCN carries no Challenge Response AVP")
	peerChallenge := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	want := ChallengeResponse(ChapIDSCCCN, []byte(secret), peerChallenge)
	require.Equal(t, want[:], got, "SCCCN Challenge Response value")
}

// RFC requirement: RFC2661-5.1.1-3 negative -- an SCCRP carrying a Challenge
// when no shared secret is configured is refused with a StopCCN, Result Code
// 4, and the tunnel closes instead of establishing.
// TestRFC2661InitiatorWithoutSecretRefusesChallenge dials without a secret
// and hands the tunnel a challenging SCCRP.
func TestRFC2661InitiatorWithoutSecretRefusesChallenge(t *testing.T) {
	now := time.Now()
	tun, defaults := dialedTunnel(t, now)
	challenge := []byte{9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9}
	outs := deliver(t, tun, buildSCCRPWire(t, rfc2661PeerTID, challenge, nil), now, defaults)
	require.Equal(t, L2TPTunnelClosed, tun.state)
	_, stopBody := findWire(t, outs, MsgStopCCN)
	info, err := parseStopCCN(stopBody)
	require.NoError(t, err)
	require.Equal(t, resultNotAuthorized, info.Result)
}

// answeringTunnelWithChallenge feeds an SCCRQ to an idle tunnel configured
// with secret and returns the tunnel, its defaults, and the SCCRP it emitted.
func answeringTunnelWithChallenge(t *testing.T, now time.Time, secret string) (*L2TPTunnel, TunnelDefaults, []byte) {
	t.Helper()
	defaults := initiatorDefaults(secret)
	tun := newTunnel(100, 42, rfc2661PeerAddr, ReliableConfig{RecvWindow: 8}, slog.Default(), now)
	outs := deliver(t, tun, buildSCCRQ(t, 42, "peer"), now, defaults)
	require.Equal(t, L2TPTunnelWaitCtlConn, tun.state)
	_, sccrpBody := findWire(t, outs, MsgSCCRP)
	return tun, defaults, sccrpBody
}

// RFC requirement: RFC2661-5.1.1-2 positive -- an SCCCN whose Challenge
// Response matches the expected CHAP-MD5 value establishes the tunnel.
// RFC requirement: RFC2661-5.1.1-2 negative -- an SCCCN whose Challenge
// Response does not match is refused with a StopCCN, Result Code 4, and the
// tunnel closes.
// RFC requirement: RFC2661-4.4.3-9 negative -- an SCCCN that omits the
// Challenge Response AVP after Ze sent a Challenge in the SCCRP is refused the
// same way.
// RFC requirement: RFC2661-5.1.1-1 negative -- the Challenge sent in the SCCRP
// is not answered by that SCCCN, so the tunnel does not establish.
// TestRFC2661AnsweringSideVerifiesChallengeResponse runs three SCCCNs against
// three tunnels that each sent a Challenge.
func TestRFC2661AnsweringSideVerifiesChallengeResponse(t *testing.T) {
	now := time.Now()
	const secret = "s3cret"

	tun, defaults, sccrpBody := answeringTunnelWithChallenge(t, now, secret)
	ourChallenge := avpBytes(sccrpBody, AVPChallenge)
	require.Len(t, ourChallenge, 16, "SCCRP carries no 16-byte Challenge")
	good := ChallengeResponse(ChapIDSCCCN, []byte(secret), ourChallenge)
	outs := deliver(t, tun, buildSCCCN(t, 100, 1, 1, good[:]), now, defaults)
	require.Equal(t, L2TPTunnelEstablished, tun.state, "a matching response establishes")
	for _, o := range outs {
		_, body := controlWire(t, o.bytes)
		require.Empty(t, body, "only a ZLB follows a valid SCCCN")
	}

	refused := func(name string, response []byte) {
		t.Helper()
		tun, defaults, sccrpBody := answeringTunnelWithChallenge(t, now, secret)
		if response != nil {
			ourChallenge := avpBytes(sccrpBody, AVPChallenge)
			wrong := ChallengeResponse(ChapIDSCCCN, []byte("other"), ourChallenge)
			response = wrong[:]
		}
		outs := deliver(t, tun, buildSCCCN(t, 100, 1, 1, response), now, defaults)
		require.Equal(t, L2TPTunnelClosed, tun.state, "%s: tunnel not closed", name)
		_, stopBody := findWire(t, outs, MsgStopCCN)
		info, err := parseStopCCN(stopBody)
		require.NoError(t, err)
		require.Equal(t, resultNotAuthorized, info.Result, "%s: StopCCN Result Code", name)
	}
	refused("mismatched response", []byte{1})
	refused("missing response", nil)
}

// RFC requirement: RFC2661-4.4.4-2 positive -- the ICRQ and the OCRQ a tunnel
// emits before any Assigned Session ID arrived carry Session ID 0 in the
// header.
// RFC requirement: RFC2661-5.3-1 positive -- those requests carry Session ID 0
// in the header and identify the session by their Assigned Session ID AVP,
// which holds the local SID.
// RFC requirement: RFC2661-4.4.4-1 positive -- once the ICRP assigned Session
// ID 55, the ICCN carries 55 in the header.
// TestRFC2661HeaderSessionIDBeforeAndAfterAssignment places a call on an
// established tunnel and reads the Session ID of each header.
func TestRFC2661HeaderSessionIDBeforeAndAfterAssignment(t *testing.T) {
	now := time.Now()
	logger := slog.Default()
	tun := newEstablishedTunnel(t, 4)

	sid, outs := tun.placeIncomingCall(now, callParams{callSerial: 1, framingType: 1}, logger)
	require.NotZero(t, sid)
	icrqHdr, icrqBody := findWire(t, keep(outs), MsgICRQ)
	require.EqualValues(t, 0, icrqHdr.SessionID, "ICRQ header Session ID")
	require.Equal(t, sid, avpU16(t, icrqBody, AVPAssignedSessionID), "ICRQ Assigned Session ID AVP")
	ackAll(t, tun, now)

	osid, outs := tun.placeOutgoingCall(now, callParams{callSerial: 2, bearerType: 1, framingType: 1}, logger)
	require.NotZero(t, osid)
	ocrqHdr, ocrqBody := findWire(t, outs, MsgOCRQ)
	require.EqualValues(t, 0, ocrqHdr.SessionID, "OCRQ header Session ID")
	require.Equal(t, osid, avpU16(t, ocrqBody, AVPAssignedSessionID), "OCRQ Assigned Session ID AVP")

	icrp := wrapControl(bodyOf(MsgICRP, u16AVP(AVPAssignedSessionID, 55)), tun.localTID, sid, 0, 2)
	outs = deliver(t, tun, icrp, now, TunnelDefaults{})
	iccnHdr, _ := findWire(t, outs, MsgICCN)
	require.EqualValues(t, 55, iccnHdr.SessionID, "ICCN header Session ID")
}

// RFC requirement: RFC2661-4.4.4-2 negative -- the ICRQ header never carries
// the local SID the tunnel allocated for the session.
// RFC requirement: RFC2661-5.3-1 negative -- the ICRQ is identified by its
// Assigned Session ID AVP alone: the header Session ID is not that value.
// RFC requirement: RFC2661-4.4.4-1 negative -- an ICRP whose header Session
// ID is not the one Ze assigned reaches no session: no ICCN follows and the
// session stays in wait-reply.
// TestRFC2661HeaderSessionIDNeverWrong checks the request header against the
// allocated SID and delivers a misaddressed reply.
func TestRFC2661HeaderSessionIDNeverWrong(t *testing.T) {
	now := time.Now()
	tun := newEstablishedTunnel(t, 4)
	sid, outs := tun.placeIncomingCall(now, callParams{callSerial: 1, framingType: 1}, slog.Default())
	require.NotZero(t, sid)
	icrqHdr, _ := findWire(t, outs, MsgICRQ)
	require.NotEqual(t, sid, icrqHdr.SessionID, "ICRQ header carries the local SID")

	icrp := wrapControl(bodyOf(MsgICRP, u16AVP(AVPAssignedSessionID, 55)), tun.localTID, sid+1, 0, 1)
	outs = deliver(t, tun, icrp, now, TunnelDefaults{})
	for _, o := range outs {
		_, body := controlWire(t, o.bytes)
		require.Empty(t, body, "a misaddressed ICRP produced %d", wireMessageType(t, body))
	}
	sess := tun.lookupSession(sid)
	require.NotNil(t, sess)
	require.Equal(t, L2TPSessionWaitReply, sess.state)
}

// RFC requirement: RFC2661-6.9-1 positive -- a tunnel whose peer advertised a
// Bearer Capabilities mask accepts an outgoing call and emits the OCRQ.
// RFC requirement: RFC2661-6.9-1 negative -- a tunnel whose peer advertised no
// bearer capability refuses the outgoing call: no session, no OCRQ.
// RFC requirement: RFC2661-4.4.4-3 positive -- the OCRQ Bearer Type carries
// the requested bit when the peer's Bearer Capabilities advertised it.
// RFC requirement: RFC2661-4.4.4-3 negative -- a Bearer Type bit the peer did
// not advertise refuses the call: no session, no OCRQ.
// RFC requirement: RFC2661-4.4.4-4 positive -- the OCRQ Framing Type carries
// the requested bit when the peer's Framing Capabilities advertised it.
// RFC requirement: RFC2661-4.4.4-4 negative -- a Framing Type bit the peer did
// not advertise refuses the outgoing call: no session, no OCRQ.
// RFC requirement: RFC2661-4.4.3-1 positive -- an incoming call and an
// outgoing call whose Framing Type the peer advertised are placed.
// RFC requirement: RFC2661-4.4.3-1 negative -- an incoming call and an
// outgoing call whose Framing Type the peer did not advertise are refused:
// no session, no request.
// TestRFC2661CallCapabilitiesChecked places calls on tunnels whose peers
// advertised analog-only and synchronous-only capabilities.
func TestRFC2661CallCapabilitiesChecked(t *testing.T) {
	now := time.Now()
	logger := slog.Default()
	tun := newEstablishedTunnel(t, 8)
	tun.peerBearer = 0x1  // analog only
	tun.peerFraming = 0x1 // synchronous only

	sid, outs := tun.placeOutgoingCall(now, callParams{callSerial: 1, bearerType: 0x1, framingType: 0x1}, logger)
	require.NotZero(t, sid, "an advertised bearer and framing are accepted")
	_, ocrqBody := findWire(t, keep(outs), MsgOCRQ)
	require.EqualValues(t, 0x1, avpBytes(ocrqBody, AVPBearerType)[3], "OCRQ Bearer Type")
	require.EqualValues(t, 0x1, avpBytes(ocrqBody, AVPFramingType)[3], "OCRQ Framing Type")
	ackAll(t, tun, now)
	isid, outs := tun.placeIncomingCall(now, callParams{callSerial: 2, framingType: 0x1}, logger)
	require.NotZero(t, isid, "an advertised framing is accepted for an incoming call")
	findWire(t, outs, MsgICRQ)
	require.Equal(t, 2, tun.sessionCount())

	refused := func(name string, sid uint16, outs []sendRequest) {
		t.Helper()
		require.Zero(t, sid, "%s: call placed", name)
		require.Empty(t, outs, "%s: request emitted", name)
		require.Equal(t, 2, tun.sessionCount(), "%s: session allocated", name)
	}
	sid, outs = tun.placeOutgoingCall(now, callParams{callSerial: 3, bearerType: 0x2, framingType: 0x1}, logger)
	refused("digital bearer on an analog-only peer", sid, outs)
	sid, outs = tun.placeOutgoingCall(now, callParams{callSerial: 4, bearerType: 0x1, framingType: 0x2}, logger)
	refused("asynchronous framing on a synchronous-only peer (outgoing)", sid, outs)
	sid, outs = tun.placeIncomingCall(now, callParams{callSerial: 5, framingType: 0x2}, logger)
	refused("asynchronous framing on a synchronous-only peer (incoming)", sid, outs)

	none := newEstablishedTunnel(t, 8)
	none.peerBearer = 0
	sid, outs = none.placeOutgoingCall(now, callParams{callSerial: 6, framingType: 0x1}, logger)
	require.Zero(t, sid, "no bearer capability received: call placed")
	require.Empty(t, outs, "no bearer capability received: OCRQ emitted")
	require.Equal(t, 0, none.sessionCount())
}

// RFC requirement: RFC2661-4.4.3-2 positive -- the SCCRQ and the SCCRP Ze
// emits each carry a Bearer Capabilities AVP.
// TestRFC2661BearerCapabilitiesEmitted reads the AVP off both bodies.
func TestRFC2661BearerCapabilitiesEmitted(t *testing.T) {
	for _, wb := range writtenBodies() {
		if wb.name != "SCCRQ" && wb.name != "SCCRP" {
			continue
		}
		require.NotNil(t, avpBytes(wb.body, AVPBearerCapabilities), "%s carries no Bearer Capabilities AVP", wb.name)
	}
}

// RFC requirement: RFC2661-5.0-1 positive -- an established tunnel places an
// incoming call and emits the ICRQ.
// RFC requirement: RFC2661-5.0-1 negative -- a tunnel still in wait-ctl-reply
// refuses both an incoming and an outgoing call: no session, no request.
// TestRFC2661CallNeedsEstablishedTunnel places calls before and after the
// control connection establishes.
func TestRFC2661CallNeedsEstablishedTunnel(t *testing.T) {
	now := time.Now()
	logger := slog.Default()
	tun, _ := dialedTunnel(t, now)
	tun.peerBearer = 0x3
	tun.peerFraming = 0x3
	sid, outs := tun.placeIncomingCall(now, callParams{callSerial: 1, framingType: 1}, logger)
	require.Zero(t, sid)
	require.Empty(t, outs)
	sid, outs = tun.placeOutgoingCall(now, callParams{callSerial: 2, bearerType: 1, framingType: 1}, logger)
	require.Zero(t, sid)
	require.Empty(t, outs)
	require.Equal(t, 0, tun.sessionCount())

	est := newEstablishedTunnel(t, 4)
	sid, outs = est.placeIncomingCall(now, callParams{callSerial: 1, framingType: 1}, logger)
	require.NotZero(t, sid)
	findWire(t, outs, MsgICRQ)
}

// RFC requirement: RFC2661-5.7-1 positive -- a StopCCN received on an
// established tunnel is answered by a ZLB ACK whose Nr acknowledges it, and a
// retransmission of the same StopCCN after the tunnel closed is answered by a
// ZLB ACK again.
// RFC requirement: RFC2661-5.7-1 negative -- neither StopCCN is answered by
// anything other than the ZLB: no StopCCN, no other control message.
// TestRFC2661StopCCNAcknowledged delivers one StopCCN twice.
func TestRFC2661StopCCNAcknowledged(t *testing.T) {
	now := time.Now()
	tun := newEstablishedTunnel(t, 4)
	stop := buildStopCCN(t, tun.localTID, 0, 0, tun.remoteTID, resultGeneralError)
	for round := range 2 {
		outs := deliver(t, tun, stop, now, TunnelDefaults{})
		require.Len(t, outs, 1, "round %d: one ZLB", round)
		hdr, body := controlWire(t, outs[0].bytes)
		require.Empty(t, body, "round %d: reply is not a ZLB", round)
		require.EqualValues(t, 1, hdr.Nr, "round %d: ZLB Nr does not acknowledge the StopCCN", round)
		require.Equal(t, L2TPTunnelClosed, tun.state)
	}
}

// RFC requirement: RFC2661-7.2.1-2 positive -- a local termination of an
// initiated tunnel emits a StopCCN and leaves the tunnel closed with its
// session cleared.
// RFC requirement: RFC2661-7.2.1-2 negative -- after the termination the
// tunnel accepts no call and emits no HELLO.
// RFC requirement: RFC2661-8.1-1 positive -- every datagram the tunnel emitted
// over its life (SCCRQ, SCCCN, ICRQ, StopCCN) is addressed to the peer address
// fixed at creation.
// TestRFC2661LocalTerminationSendsStopCCN terminates a dialed tunnel that
// carries one session.
func TestRFC2661LocalTerminationSendsStopCCN(t *testing.T) {
	now := time.Now()
	logger := slog.Default()
	tun, _, outs := dialUntilEstablished(t, now, "")
	sid, more := tun.placeIncomingCall(now, callParams{callSerial: 1, framingType: 1}, logger)
	require.NotZero(t, sid)
	outs = append(outs, keep(more)...)
	ackAll(t, tun, now)

	more = tun.teardownStopCCN(now, ResultCodeValue{Result: resultGeneralError}, l2tpevents.TerminateCauseAdminReset)
	more = keep(more)
	outs = append(outs, more...)
	findWire(t, more, MsgStopCCN)
	require.Equal(t, L2TPTunnelClosed, tun.state)
	require.Equal(t, 0, tun.sessionCount())

	sid, more = tun.placeIncomingCall(now, callParams{callSerial: 2, framingType: 1}, logger)
	require.Zero(t, sid)
	require.Empty(t, more)
	require.Empty(t, tun.handleHelloTimer(now))

	require.Len(t, outs, 4, "SCCRQ, SCCCN, ICRQ, StopCCN")
	for _, o := range outs {
		require.Equal(t, rfc2661PeerAddr, o.to)
	}
}

// RFC requirement: RFC2661-7.2.1-3 positive -- a StopCCN received by the
// tunnel originator closes the tunnel and clears its session.
// RFC requirement: RFC2661-7.2.1-3 negative -- the originator answers with
// nothing but a ZLB, and accepts no call afterwards.
// TestRFC2661OriginatorCleansUpOnStopCCN delivers a StopCCN to a dialed
// tunnel that carries one session.
func TestRFC2661OriginatorCleansUpOnStopCCN(t *testing.T) {
	now := time.Now()
	logger := slog.Default()
	tun, defaults, _ := dialUntilEstablished(t, now, "")
	sid, _ := tun.placeIncomingCall(now, callParams{callSerial: 1, framingType: 1}, logger)
	require.NotZero(t, sid)

	// Ze sent SCCRQ, SCCCN and ICRQ (Ns 0..2); the peer sent the SCCRP (Ns 0).
	outs := deliver(t, tun, buildStopCCN(t, tun.localTID, 1, 3, 777, resultGeneralError), now, defaults)
	require.Equal(t, L2TPTunnelClosed, tun.state)
	require.Equal(t, 0, tun.sessionCount())
	require.Len(t, outs, 1)
	_, body := controlWire(t, outs[0].bytes)
	require.Empty(t, body, "the reply is not a ZLB")

	sid, more := tun.placeIncomingCall(now, callParams{callSerial: 2, framingType: 1}, logger)
	require.Zero(t, sid)
	require.Empty(t, more)
}

// RFC requirement: RFC2661-5.8-10 positive -- an in-order control message
// received while nothing is outstanding is acknowledged: the engine reports a
// ZLB is owed and builds one whose Nr is the message's Ns plus one.
// RFC requirement: RFC2661-5.8-10 negative -- with the send window exhausted
// and a message queued behind it, an in-order control message is still
// acknowledged the same way: the ZLB is never withheld.
// TestRFC2661AckNotWithheldUnderFlowControl drives the engine at both window
// states.
func TestRFC2661AckNotWithheldUnderFlowControl(t *testing.T) {
	now := time.Now()
	e := NewReliableEngine(ReliableConfig{
		LocalTunnelID: 100, PeerTunnelID: 200,
		RTimeout: time.Second, RTimeoutCap: 16 * time.Second, MaxRetransmit: 3,
		RecvWindow: 4, InitialPeerRWS: 1,
	})
	hello := messageTypeAVP(uint16(MsgHello))

	res := e.OnReceive(MessageHeader{IsControl: true, TunnelID: 100, Ns: 0, Nr: 0}, hello, now)
	require.Equal(t, ClassDelivered, res.Class)
	require.True(t, e.NeedsZLB(), "idle engine: ZLB not owed")
	buf := make([]byte, ControlHeaderLen)
	e.BuildZLB(buf, 0)
	require.EqualValues(t, 1, parseSent(t, buf).Nr)

	// Exhaust the window (peer RWS 1): one message outstanding, one queued.
	sent := mustEnqueue(t, e, 0, hello, now)
	require.NotEmpty(t, sent)
	queued, err := e.Enqueue(0, hello, now, false)
	require.NoError(t, err)
	require.Nil(t, queued, "second message must queue behind the closed window")

	res = e.OnReceive(MessageHeader{IsControl: true, TunnelID: 100, Ns: 1, Nr: 0}, hello, now)
	require.Equal(t, ClassDelivered, res.Class)
	require.True(t, e.NeedsZLB(), "closed window: ZLB withheld")
	e.BuildZLB(buf, 0)
	require.EqualValues(t, 2, parseSent(t, buf).Nr)
}

// RFC requirement: RFC2661-4.1-5 positive -- an SCCRQ carrying an
// unrecognized vendor AVP with M=0, and one carrying an unrecognized IETF
// attribute type with M=0, each parse to the same fields as the SCCRQ without
// it.
// RFC requirement: RFC2661-4.1-5 negative -- the same vendor AVP with M=1 is
// not ignored: parseSCCRQ refuses the body.
// TestRFC2661UnrecognizedOptionalAVPIgnored parses the three bodies.
func TestRFC2661UnrecognizedOptionalAVPIgnored(t *testing.T) {
	want, err := parseSCCRQ(buildSCCRQ(t, 42, "peer")[ControlHeaderLen:])
	require.NoError(t, err)

	got, err := parseSCCRQ(buildSCCRQWithVendorAVP(t, false, 42, "peer")[ControlHeaderLen:])
	require.NoError(t, err)
	require.Equal(t, want, got, "an M=0 vendor AVP changed the parse")

	plain := buildSCCRQ(t, 42, "peer")[ControlHeaderLen:]
	unknown := make([]byte, len(plain)+AVPHeaderLen+2)
	copy(unknown, plain)
	WriteAVPUint16(unknown, len(plain), false, AVPType(250), 7)
	got, err = parseSCCRQ(unknown)
	require.NoError(t, err)
	require.Equal(t, want, got, "an M=0 unknown IETF AVP changed the parse")

	_, err = parseSCCRQ(buildSCCRQWithVendorAVP(t, true, 42, "peer")[ControlHeaderLen:])
	require.ErrorIs(t, err, errSCCRQUnknownMandatoryVendorAVP)
}

// RFC requirement: RFC2661-4.4.2-1 positive -- the Result Code AVP of a
// StopCCN carries its error message as the UTF-8 bytes of the string Ze
// wrote, and the message read back is valid UTF-8.
// TestRFC2661ResultCodeMessageUTF8 tears a tunnel down with a message that
// holds a multi-byte character.
func TestRFC2661ResultCodeMessageUTF8(t *testing.T) {
	now := time.Now()
	tun, _, _ := dialUntilEstablished(t, now, "")
	const msg = "clé refusée"
	outs := tun.teardownStopCCN(now, ResultCodeValue{
		Result: resultProtocolError, ErrorPresent: true, Error: errorValueOutOfRange,
		Message: msg, MessagePresent: true,
	}, l2tpevents.TerminateCauseNASError)
	_, stopBody := findWire(t, outs, MsgStopCCN)
	info, err := parseStopCCN(stopBody)
	require.NoError(t, err)
	require.Equal(t, msg, info.Message)
	require.True(t, utf8.ValidString(info.Message))
	require.Equal(t, []byte(msg), avpBytes(stopBody, AVPResultCode)[4:], "Result Code AVP bytes after the two codes")
}

// lnsSessionEstablished establishes one LNS-side session on tun from an ICRQ
// and an ICCN and returns its local SID.
func lnsSessionEstablished(t *testing.T, tun *L2TPTunnel, now time.Time, remoteSID uint16) uint16 {
	t.Helper()
	ns := tun.engine.nextRecvSeq
	nr := tun.engine.nextSendSeq
	outs := deliver(t, tun, wrapControl(buildICRQ(remoteSID, 1), tun.localTID, 0, ns, nr), now, TunnelDefaults{})
	_, icrpBody := findWire(t, outs, MsgICRP)
	sid := avpU16(t, icrpBody, AVPAssignedSessionID)
	deliver(t, tun, wrapControl(buildICCN(64000, 1), tun.localTID, sid, ns+1, nr+1), now, TunnelDefaults{})
	sess := tun.lookupSession(sid)
	require.NotNil(t, sess)
	require.Equal(t, L2TPSessionEstablished, sess.state)
	return sid
}

// RFC requirement: RFC2661-6.12-1 positive -- a CDN for an established session
// removes the session and queues its kernel teardown and its session-down
// event.
// RFC requirement: RFC2661-6.12-1 negative -- the CDN is answered by nothing
// but a ZLB: no CDN and no other control message goes back.
// RFC requirement: RFC2661-7.5.1-1 positive -- a CDN received for an
// established LAC session removes the session and queues the kernel teardown
// that releases the bridged call.
// RFC requirement: RFC2661-7.5.1-1 negative -- a CDN whose header names no
// session removes nothing and queues nothing.
// TestRFC2661CDNCleansUpSilently delivers a CDN to an LNS session, to a LAC
// session, and to a Session ID no session holds.
func TestRFC2661CDNCleansUpSilently(t *testing.T) {
	now := time.Now()
	lns := newEstablishedTunnel(t, 4)
	sid := lnsSessionEstablished(t, lns, now, 9)
	cdn := wrapControl(bodyOf(MsgCDN, resultCodeAVP(), u16AVP(AVPAssignedSessionID, 9)), lns.localTID, sid, 2, 1)
	outs := deliver(t, lns, cdn, now, TunnelDefaults{})
	require.Nil(t, lns.lookupSession(sid), "session survived the CDN")
	require.Len(t, lns.pendingKernelTeardowns, 1)
	require.Len(t, lns.pendingSessionDowns, 1)
	require.Len(t, outs, 1)
	_, body := controlWire(t, outs[0].bytes)
	require.Empty(t, body, "the CDN was answered with %d", wireMessageType(t, body))

	lac := newEstablishedTunnel(t, 4)
	lsid, _ := lac.placeIncomingCall(now, callParams{callSerial: 1, framingType: 1}, slog.Default())
	deliver(t, lac, wrapControl(bodyOf(MsgICRP, u16AVP(AVPAssignedSessionID, 55)), lac.localTID, lsid, 0, 1), now, TunnelDefaults{})
	require.Equal(t, L2TPSessionEstablished, lac.lookupSession(lsid).state)
	deliver(t, lac, wrapControl(bodyOf(MsgCDN, resultCodeAVP(), u16AVP(AVPAssignedSessionID, 55)), lac.localTID, lsid, 1, 2), now, TunnelDefaults{})
	require.Nil(t, lac.lookupSession(lsid), "LAC session survived the CDN")
	require.Len(t, lac.pendingKernelTeardowns, 1)

	other := newEstablishedTunnel(t, 4)
	osid := lnsSessionEstablished(t, other, now, 9)
	deliver(t, other, wrapControl(bodyOf(MsgCDN, resultCodeAVP(), u16AVP(AVPAssignedSessionID, 9)), other.localTID, osid+1, 2, 1), now, TunnelDefaults{})
	require.NotNil(t, other.lookupSession(osid), "a CDN naming no session removed one")
	require.Empty(t, other.pendingKernelTeardowns)
	require.Empty(t, other.pendingSessionDowns)
}

// RFC requirement: RFC2661-7.5.1-2 positive -- tearing down an established
// LAC session emits a CDN to the LNS whose header carries the session's
// remote SID and whose Result Code AVP carries the given code.
// TestRFC2661LACSendsCDNOnDisconnect tears down the session the ICRP
// established.
func TestRFC2661LACSendsCDNOnDisconnect(t *testing.T) {
	now := time.Now()
	logger := slog.Default()
	lac := newEstablishedTunnel(t, 4)
	lsid, _ := lac.placeIncomingCall(now, callParams{callSerial: 1, framingType: 1}, logger)
	deliver(t, lac, wrapControl(bodyOf(MsgICRP, u16AVP(AVPAssignedSessionID, 55)), lac.localTID, lsid, 0, 1), now, TunnelDefaults{})
	sess := lac.lookupSession(lsid)
	require.Equal(t, L2TPSessionEstablished, sess.state)

	outs := lac.teardownSession(sess, cdnResultGeneralError, l2tpevents.TerminateCauseLostCarrier, now, logger)
	hdr, body := findWire(t, outs, MsgCDN)
	require.EqualValues(t, 55, hdr.SessionID)
	require.Equal(t, lac.peerAddr, outs[0].to)
	rc := avpBytes(body, AVPResultCode)
	require.NotNil(t, rc)
	require.EqualValues(t, cdnResultGeneralError, uint16(rc[0])<<8|uint16(rc[1]))
	require.Nil(t, lac.lookupSession(lsid))
}

// RFC requirement: RFC2661-6.14-1 positive -- an SLI on an established session
// updates the session's ACCM, and a second SLI updates it again.
// RFC requirement: RFC2661-6.14-1 negative -- an SLI for a Session ID no
// session holds changes no session's ACCM.
// TestRFC2661SLIUpdatesACCM delivers two SLIs to a LAC session and one to
// nobody.
func TestRFC2661SLIUpdatesACCM(t *testing.T) {
	now := time.Now()
	lac := newEstablishedTunnel(t, 4)
	lsid, _ := lac.placeIncomingCall(now, callParams{callSerial: 1, framingType: 1}, slog.Default())
	deliver(t, lac, wrapControl(bodyOf(MsgICRP, u16AVP(AVPAssignedSessionID, 55)), lac.localTID, lsid, 0, 1), now, TunnelDefaults{})
	sess := lac.lookupSession(lsid)
	require.Equal(t, L2TPSessionEstablished, sess.state)

	sli := func(v ACCMValue) func(buf []byte, off int) int {
		return func(buf []byte, off int) int { return writeAVPACCM(buf, off, v) }
	}
	first := ACCMValue{SendACCM: 0x000A0000, RecvACCM: 0xFFFFFFFF}
	deliver(t, lac, wrapControl(bodyOf(MsgSLI, sli(first)), lac.localTID, lsid, 1, 2), now, TunnelDefaults{})
	require.Equal(t, first, sess.accm)
	second := ACCMValue{SendACCM: 0, RecvACCM: 0x00000001}
	deliver(t, lac, wrapControl(bodyOf(MsgSLI, sli(second)), lac.localTID, lsid, 2, 2), now, TunnelDefaults{})
	require.Equal(t, second, sess.accm)

	third := ACCMValue{SendACCM: 0x12345678, RecvACCM: 0x9ABCDEF0}
	deliver(t, lac, wrapControl(bodyOf(MsgSLI, sli(third)), lac.localTID, lsid+1, 3, 2), now, TunnelDefaults{})
	require.Equal(t, second, sess.accm, "an SLI for another Session ID changed this session")
}

// RFC requirement: RFC2661-7.5-1 positive -- an OCRQ on an established tunnel
// with room for a session is answered by an OCRP whose header carries the
// peer's Assigned Session ID and whose body carries the local one.
// RFC requirement: RFC2661-7.5-1 negative -- an OCRQ the tunnel cannot serve
// (max sessions reached) is answered by a CDN, Result Code 4, and no OCRP.
// TestRFC2661OCRQAnsweredWithOCRP delivers two OCRQs to a tunnel that admits
// one session.
func TestRFC2661OCRQAnsweredWithOCRP(t *testing.T) {
	now := time.Now()
	tun := newEstablishedTunnel(t, 1)
	ocrq := func(remoteSID uint16, serial uint32) []byte {
		return bodyOf(MsgOCRQ,
			u16AVP(AVPAssignedSessionID, remoteSID), u32AVP(AVPCallSerialNumber, serial),
			u32AVP(AVPMinimumBPS, 9600), u32AVP(AVPMaximumBPS, 64000),
			u32AVP(AVPBearerType, 1), u32AVP(AVPFramingType, 1), strAVP(AVPCalledNumber, "5551000"))
	}
	outs := deliver(t, tun, wrapControl(ocrq(9, 1), tun.localTID, 0, 0, 0), now, TunnelDefaults{})
	hdr, body := findWire(t, outs, MsgOCRP)
	require.EqualValues(t, 9, hdr.SessionID)
	local := avpU16(t, body, AVPAssignedSessionID)
	require.NotNil(t, tun.lookupSession(local))

	outs = deliver(t, tun, wrapControl(ocrq(10, 2), tun.localTID, 0, 1, 1), now, TunnelDefaults{})
	for _, o := range outs {
		_, body := controlWire(t, o.bytes)
		require.NotEqual(t, MsgOCRP, wireMessageType(t, body), "a refused OCRQ was answered with an OCRP")
	}
	hdr, body = findWire(t, outs, MsgCDN)
	require.EqualValues(t, 10, hdr.SessionID)
	rc := avpBytes(body, AVPResultCode)
	require.EqualValues(t, cdnResultNoResources, uint16(rc[0])<<8|uint16(rc[1]))
	require.Equal(t, 1, tun.sessionCount())
}

// RFC requirement: RFC2661-8.2-1 positive -- the listener Ze opens is a UDP
// socket: an SCCRQ sent to it over UDP is answered by an SCCRP over UDP.
// TestRFC2661UDPEncapsulationOffered drives one exchange over the loopback
// UDP socket the reactor binds.
func TestRFC2661UDPEncapsulationOffered(t *testing.T) {
	ln, _, _, stop := buildLogReactor(t)
	defer stop()
	require.Equal(t, "udp", ln.conn.LocalAddr().Network())
	client := newClient(t, ln)
	defer client.Close()
	client.Send(t, buildSCCRQ(t, 42, "peer-udp"))
	_, body := controlWire(t, readDatagram(t, client))
	require.Equal(t, MsgSCCRP, wireMessageType(t, body))
}
