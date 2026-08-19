// RFC 7296 Section 2.3, INVALID_MESSAGE_ID. The sentence makes the sending OPTIONAL and
// the rate limit a MUST, so the obligation these tests prove is the BOUND.
//
// Helpers here start with `imi`, so they cannot collide with the sibling RFC files in
// this package. This file reuses the `rtx` loopback helpers, the `osr` session and
// request builders, and the `win` DPD helper.

package engine

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// imiDrain returns every datagram ze wrote before the sentinel. One UDP socket keeps
// send order on loopback, so the sentinel marks the end of what the preceding action
// produced. This is the counting form of rtxExpectSilence: an empty result is exactly
// the silence that helper proves.
func imiDrain(t *testing.T, peerTr, myTr *transport.UDPTransport, remote *net.UDPAddr, what string) [][]byte {
	t.Helper()
	if err := myTr.Send(rtxSentinel, remote); err != nil {
		t.Fatalf("send sentinel: %v", err)
	}
	var got [][]byte
	for {
		pkt := rtxRecv(t, peerTr)
		if pkt == nil {
			t.Fatalf("%s: the sentinel never arrived", what)
		}
		if bytes.Equal(pkt, rtxSentinel) {
			return got
		}
		got = append(got, bytes.Clone(pkt))
	}
}

// imiNotifyData returns the Notification Data of the single notify the datagram carries,
// decrypted under the counterpart SA. It fails the test when the datagram is not one
// INFORMATIONAL request carrying exactly one Notify of the given type.
func imiNotifyData(t *testing.T, peer *SA, raw []byte, want uint16) []byte {
	t.Helper()
	msg := parseMsg(t, raw)
	inner, err := decryptAndParse(peer, msg, raw)
	if err != nil {
		t.Fatalf("the peer could not authenticate the notification: %v", err)
	}
	var found []byte
	seen := 0
	for i := range inner {
		n, ok := inner[i].Payload.(*wire.PayloadNotify)
		if !ok {
			continue
		}
		seen++
		if n.NotifyMsgType == want {
			found = n.NotificationData
		}
	}
	if seen != 1 {
		t.Fatalf("the notification carries %d Notify payloads, want exactly 1", seen)
	}
	if found == nil {
		t.Fatalf("the notification carries no Notify of type %d", want)
	}
	return found
}

// imiOutOfWindow drives one authenticated request whose Message ID is outside the
// window, and returns the id it carried.
func imiOutOfWindow(t *testing.T, ini, peer *SA, ps *PeerSession, myTr *transport.UDPTransport, offset uint32) uint32 {
	t.Helper()
	badID := ini.ExpectedMsgID + offset
	req := osrRequest(t, peer, badID)
	out := ps.handleOwnedInbound(ini, transport.Packet{Data: req}, myTr, nil, slogutil.DiscardLogger())
	if out.peerAlive {
		t.Fatal("an out-of-window message credited peer liveness; a replay could then mask a dead peer")
	}
	return badID
}

// VALIDATES: the number of INVALID_MESSAGE_ID notifications an SA raises is capped by a
// token bucket, so a peer replaying captured ciphertext at many old Message IDs draws
// far fewer notifications than the requests it sent.
// PREVENTS: an unbounded emitter, which turns every out-of-window datagram into an
// outbound datagram and makes this node an amplifier.
//
// RFC requirement: RFC7296-2.3-9 positive -- RFC 7296 Section 2.3 MUST:
// "notifications of this type MUST be rate limited". The checklist row carries the
// sentence verbatim (rfc/full/rfc7296.txt:1506-1509). sendInvalidMessageID
// (notify_invalid_msgid.go) spends a token from sa.invalidMsgIDLimiter before it builds
// anything, and invalidMsgIDAllowed creates that bucket at invalidMsgIDNotifyRate with a
// burst of invalidMsgIDNotifyBurst.
//
// The cap is asserted against the analytic bound of the bucket. That bound is the burst
// plus the tokens the run refilled. A loaded host lengthens the run and widens the bound
// with it. The count is never compared against a wall-clock guess.
func TestImiRateLimitCapsTheNotification(t *testing.T) {
	ini, peer, ps, peerTr, myTr := osrSession(t)
	remote := ini.remoteUDPAddr()

	const requests = 12
	start := time.Now()
	for i := range uint32(requests) {
		imiOutOfWindow(t, ini, peer, ps, myTr, 1000+i)
		// Free the window between drives so the token bucket is the only bound under
		// test here. TestImiHeldWindowSuppressesTheNotification covers the other guard.
		ini.releaseRequestWindow()
	}
	elapsed := time.Since(start)

	sent := len(imiDrain(t, peerTr, myTr, remote, "a burst of out-of-window requests"))

	// Control. The emitter is not mute. At least one notification did go out, so the cap
	// below is a decision about the rate and not an absent feature.
	if sent == 0 {
		t.Fatal("no INVALID_MESSAGE_ID went out at all, so the cap proves nothing")
	}
	if sent >= requests {
		t.Errorf("%d requests drew %d notifications; the rate limit did not bite", requests, sent)
	}
	// The bucket starts full and refills at invalidMsgIDNotifyRate per second.
	bound := invalidMsgIDNotifyBurst + int(elapsed.Seconds()*invalidMsgIDNotifyRate) + 1
	if sent > bound {
		t.Errorf("%d notifications went out, above the bucket bound of %d over %v", sent, bound, elapsed)
	}
}

// VALIDATES: a datagram at an out-of-window Message ID whose ciphertext does not
// authenticate draws nothing at all, while the same datagram unmodified does draw the
// notification.
// PREVENTS: an emitter placed before the decrypt. Such an emitter lets an off-path
// attacker who read the cleartext SPI pair spend this SA's one request window with one
// forged datagram. It stalls the liveness probe, the Delete and the rekey for the whole
// requestWindowTimeout.
//
// RFC requirement: RFC7296-2.3-9 negative -- the bound the rate limit protects is reachable
// only by an authenticated peer. The out-of-window arm of handleOwnedInbound (inbound.go)
// calls decryptAndParse first. It calls sendInvalidMessageID only when that returns no
// error. An unauthenticated datagram therefore never reaches the emitter, and it never
// spends a token or the request window.
func TestImiUnauthenticatedRequestDrawsNothing(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, peer, ps, peerTr, myTr := osrSession(t)
	remote := ini.remoteUDPAddr()

	badID := ini.ExpectedMsgID + 4242
	good := osrRequest(t, peer, badID)

	// The forgery. A flipped ciphertext bit fails the integrity check, which is what an
	// off-path attacker who can only edit an observed datagram produces.
	trunc := int(ini.Proposal.Integrity.TruncatedLength)
	if trunc == 0 {
		trunc = 16
	}
	forged := bytes.Clone(good)
	forged[len(forged)-trunc-1] ^= 0x01
	if _, err := decryptAndParse(ini, parseMsg(t, forged), forged); err == nil {
		t.Fatal("the forged datagram authenticated, so this test proves nothing")
	}

	before := errorNotifySuppressedCount("invalid-msgid-unauthenticated")
	ps.handleOwnedInbound(ini, transport.Packet{Data: forged}, myTr, nil, log)
	rtxExpectSilence(t, peerTr, myTr, remote, "an unauthenticated out-of-window request")
	if errorNotifySuppressedCount("invalid-msgid-unauthenticated") <= before {
		t.Error("the unauthenticated guard did not record the suppression")
	}
	if ini.requestOutstanding {
		t.Error("a forged datagram took the request window; one such packet would stall the SA")
	}

	// Control. The SAME fixture, unmodified, DOES draw the notification. The silence
	// above is therefore a decision about authentication, not an absent emitter.
	ps.handleOwnedInbound(ini, transport.Packet{Data: good}, myTr, nil, log)
	if len(imiDrain(t, peerTr, myTr, remote, "an authenticated out-of-window request")) != 1 {
		t.Fatal("the authenticated out-of-window request drew no notification")
	}
}

// VALIDATES: the notification is a new INFORMATIONAL REQUEST carrying its own Message
// ID, and its Notification Data is exactly the four octets of the invalid Message ID,
// big-endian.
// PREVENTS: acknowledging the invalid request, and a body of the wrong length or byte
// order, which leaves the peer unable to tell which of its requests was refused.
//
// RFC requirement: RFC7296-2.3-9 positive -- RFC 7296 Section 2.3 says to
// "inform the other side by initiating an INFORMATIONAL exchange with Notification Data containing the four-octet invalid Message ID".
// sendInvalidMessageID (notify_invalid_msgid.go) builds a [4]byte with
// binary.BigEndian.PutUint32. It builds the message with initiatorFlag alone. The
// Response flag is therefore clear, and the message carries sa.NextMsgID rather than the
// invalid id.
//
// RFC requirement: RFC7296-2.3-5 positive -- the same emission is what keeps the neighboring
// MUST NOT intact. The notification "MUST NOT be sent in a response" and
// "the invalid request MUST NOT be acknowledged": this datagram has the Response flag
// clear and its Message ID differs from the invalid one, so it acknowledges nothing.
func TestImiNotificationCarriesTheFourOctetMessageID(t *testing.T) {
	ini, peer, ps, peerTr, myTr := osrSession(t)
	remote := ini.remoteUDPAddr()

	ownID := ini.NextMsgID
	badID := imiOutOfWindow(t, ini, peer, ps, myTr, 7777)

	out := imiDrain(t, peerTr, myTr, remote, "an out-of-window request")
	if len(out) != 1 {
		t.Fatalf("the out-of-window request drew %d datagrams, want exactly 1", len(out))
	}
	hdr := parseMsg(t, out[0]).Header
	if hdr.Flags&wire.FlagResponse != 0 {
		t.Error("the notification carries the Response flag; RFC 7296 Section 2.3 forbids sending it in a response")
	}
	if hdr.ExchangeType != wire.ExchangeInformational {
		t.Errorf("the notification exchange = %d, want INFORMATIONAL", hdr.ExchangeType)
	}
	if hdr.MessageID == badID {
		t.Error("the notification carries the invalid Message ID, so it acknowledges the invalid request")
	}
	if hdr.MessageID != ownID {
		t.Errorf("the notification carries Message ID %d, want %d (this SA's own next request id)",
			hdr.MessageID, ownID)
	}
	if ini.NextMsgID != ownID+1 {
		t.Errorf("NextMsgID = %d after the notification, want %d; the id was not spent", ini.NextMsgID, ownID+1)
	}

	data := imiNotifyData(t, peer, out[0], wire.NotifyInvalidMessageID)
	if len(data) != 4 {
		t.Fatalf("the Notification Data is %d octets, want exactly 4", len(data))
	}
	if got := binary.BigEndian.Uint32(data); got != badID {
		t.Errorf("the Notification Data reads %d big-endian, want the invalid Message ID %d", got, badID)
	}
	// Byte-exact, so a little-endian build of the same value is caught even when the id
	// happens to be palindromic under a looser check.
	var want [4]byte
	binary.BigEndian.PutUint32(want[:], badID)
	if !bytes.Equal(data, want[:]) {
		t.Errorf("the Notification Data is %x, want %x", data, want)
	}
}

// VALIDATES: with the request window already held by this node's own request, an
// authenticated out-of-window request draws nothing, and the holder keeps the window.
// PREVENTS: a courtesy notification displacing this node's DPD probe, Delete or rekey,
// and a second request in flight, which RFC 7296 Section 2.3 forbids at a window of one.
//
// RFC requirement: RFC7296-2.3-9 negative -- the rate limit is not the only bound. The
// emission is skipped whenever reserveRequestWindow (msgid.go) reads false, so a peer
// replaying old ciphertext cannot stall this SA even inside the token budget.
//
// RFC requirement: RFC7296-2.3-8 negative -- the new emitter does not break the one-outstanding
// -request rule it sits beside. The probe that held the window still holds it afterwards,
// and its Message ID is unchanged.
func TestImiHeldWindowSuppressesTheNotification(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, peer, ps, peerTr, myTr := osrSession(t)
	remote := ini.remoteUDPAddr()

	// Our own request takes the window and stays unanswered.
	dpd := winDueDPD()
	sendDPD(ini, myTr, dpd, log)
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the DPD probe never reached the peer")
	}
	if !ini.requestOutstanding {
		t.Fatal("the probe did not take the request window")
	}

	before := errorNotifySuppressedCount("invalid-msgid-window-held")
	imiOutOfWindow(t, ini, peer, ps, myTr, 3131)
	rtxExpectSilence(t, peerTr, myTr, remote, "an out-of-window request while our own request is outstanding")
	if errorNotifySuppressedCount("invalid-msgid-window-held") <= before {
		t.Error("the window guard did not record the suppression")
	}
	if !ini.requestOutstanding {
		t.Error("the notification path released the window our own probe holds")
	}
	if ini.requestMsgID != dpd.probeMsgID {
		t.Errorf("the window now expects an answer at id %d, want the probe's id %d",
			ini.requestMsgID, dpd.probeMsgID)
	}

	// Control. Once the probe is answered and the window frees, the SAME fixture DOES
	// draw the notification. The silence above is a decision about the window.
	ini.answerAuthenticatedResponse(dpd.probeMsgID)
	if ini.requestOutstanding {
		t.Fatal("the window is still held, so the control arm proves nothing")
	}
	imiOutOfWindow(t, ini, peer, ps, myTr, 3132)
	if len(imiDrain(t, peerTr, myTr, remote, "an out-of-window request with the window free")) != 1 {
		t.Fatal("the out-of-window request drew no notification once the window was free")
	}
}
