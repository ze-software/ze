package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// dpdProbeLink returns an established initiator SA and the peer SA that must read
// what it writes. It also returns the session that owns the exchange, and the
// loopback transport pair. The initiator is the role that raises a liveness probe.
func dpdProbeLink(t *testing.T) (ini, peer *SA, ps *PeerSession, peerTr, myTr *transport.UDPTransport) {
	t.Helper()
	ini, peer, ps = establishPSK(t)
	peerTr, myTr = rtxPeerLink(t)
	ini.PeerCfg.RemoteAddress = "127.0.0.1"
	peer.PeerCfg.RemoteAddress = "127.0.0.1"
	return ini, peer, ps, peerTr, myTr
}

// dpdBareHeader builds the unprotected probe of a bare IKE header. This is the exact
// datagram Ze wrote before the fix, and the tests keep it as the counter-example.
func dpdBareHeader(t *testing.T, sa *SA, msgID uint32) []byte {
	t.Helper()
	msg := wire.Message{
		Header: wire.Header{
			InitiatorSPI: sa.InitiatorSPI,
			ResponderSPI: sa.ResponderSPI,
			MajorVersion: 2,
			ExchangeType: wire.ExchangeInformational,
			Flags:        wire.FlagInitiator,
			MessageID:    msgID,
		},
	}
	buf := make([]byte, 512)
	n, err := msg.CheckedWriteTo(buf, 0)
	if err != nil {
		t.Fatalf("build the bare-header probe at id %d: %v", msgID, err)
	}
	return buf[:n]
}

// dpdSendProbe raises one liveness probe and returns the datagram it wrote, with the
// message id the probe carries. It fails the test when nothing reaches the peer.
func dpdSendProbe(t *testing.T, ini *SA, myTr, peerTr *transport.UDPTransport, dpd *dpdState) (raw []byte, msgID uint32) {
	t.Helper()
	msgID = ini.NextMsgID
	sendDPD(ini, myTr, dpd, slogutil.DiscardLogger())
	raw = rtxRecv(t, peerTr)
	if raw == nil {
		t.Fatal("the liveness probe never reached the peer")
	}
	return raw, msgID
}

// VALIDATES: the liveness probe Ze sends is an INFORMATIONAL request that carries an
// Encrypted payload, and the peer authenticates it under the negotiated keys.
// PREVENTS: the bare 28-byte header Ze wrote before this fix. A conforming peer drops
// it, so every probe went unanswered and Dead Peer Detection killed a healthy tunnel.
//
// RFC requirement: RFC7296-1.4-5 positive -- sendDPD (dpd.go) builds the probe through
// buildEncryptedMessageEx, so the datagram holds an SK payload. decryptAndParse under
// the peer SA accepts it, which is the cryptographic protection Section 1.4 requires.
//
// RFC requirement: RFC7296-1.4-5 negative -- the protection is keyed to this pair of SAs. An
// unrelated IKE SA cannot read the probe, and the bare header of the old builder is
// refused by the same peer. The positive assertion is therefore a real check.
func TestDpdProbeIsEncrypted(t *testing.T) {
	ini, peer, _, peerTr, myTr := dpdProbeLink(t)

	probe, probeID := dpdSendProbe(t, ini, myTr, peerTr, winDueDPD())
	msg := parseMsg(t, probe)

	if msg.Header.ExchangeType != wire.ExchangeInformational {
		t.Errorf("probe exchange = %d, want INFORMATIONAL", msg.Header.ExchangeType)
	}
	if msg.Header.Flags&wire.FlagResponse != 0 {
		t.Error("the probe carries the Response flag, so it is not a request")
	}
	if msg.Header.MessageID != probeID {
		t.Errorf("probe Message ID = %d, want %d", msg.Header.MessageID, probeID)
	}
	if msg.Header.InitiatorSPI != ini.InitiatorSPI || msg.Header.ResponderSPI != ini.ResponderSPI {
		t.Error("the probe carries the SPIs of another IKE SA")
	}

	// Positive. The datagram holds an Encrypted payload and the peer authenticates it.
	sawSK := false
	for i := range msg.Payloads {
		if _, ok := msg.Payloads[i].Payload.(*wire.PayloadSK); ok {
			sawSK = true
		}
	}
	if !sawSK {
		t.Fatal("the probe carries no Encrypted payload, so it is not protected")
	}
	if _, err := decryptAndParse(peer, msg, probe); err != nil {
		t.Fatalf("the peer could not authenticate the probe: %v", err)
	}

	// Negative. The bare header of the old builder is refused by the same peer.
	bare := dpdBareHeader(t, ini, probeID)
	if _, err := decryptAndParse(peer, parseMsg(t, bare), bare); err == nil {
		t.Error("a bare IKE header authenticated as an INFORMATIONAL request")
	}

	// Negative. An unrelated IKE SA holds other keys and cannot read the probe.
	_, other, _ := establishPSK(t)
	if _, err := decryptAndParse(other, parseMsg(t, probe), probe); err == nil {
		t.Error("the probe authenticated under an unrelated IKE SA")
	}
}

// VALIDATES: the probe decrypts to an empty inner payload chain, which is the shape
// RFC 7296 Section 1.4 gives the liveness check.
// PREVENTS: a probe that smuggles a payload into what the RFC calls an empty request.
func TestDpdProbeDecryptsToEmptyChain(t *testing.T) {
	ini, peer, _, peerTr, myTr := dpdProbeLink(t)

	probe, _ := dpdSendProbe(t, ini, myTr, peerTr, winDueDPD())

	inner := lcyDecrypt(t, peer, probe)
	if len(inner) != 0 {
		t.Errorf("the probe carries %d inner payloads, want none", len(inner))
	}
}

// VALIDATES: a probe and its answer complete one exchange. The answer clears the wait
// and frees the request window of RFC 7296 Section 2.3.
// PREVENTS: a probe the receive half cannot correlate, which would leave the wait
// standing until the Dead Peer Detection timeout declared a live peer dead.
func TestDpdRoundTripCreditsLiveness(t *testing.T) {
	log := slogutil.DiscardLogger()
	ini, peer, ps, peerTr, myTr := dpdProbeLink(t)

	dpd := winDueDPD()
	probe, _ := dpdSendProbe(t, ini, myTr, peerTr, dpd)
	probeID := parseMsg(t, probe).Header.MessageID
	// A round trip starts with a probe the peer accepts, so the exchange begins with
	// the peer reading what Ze wrote.
	lcyDecrypt(t, peer, probe)
	if !dpd.awaitingReply() {
		t.Fatal("the probe left no outstanding wait")
	}
	if !dpd.matchesProbe(probeID) {
		t.Fatalf("the outstanding wait does not name message id %d", probeID)
	}

	answer := winInformationalAnswer(t, peer, probeID)
	out := ps.handleOwnedInbound(ini, transport.Packet{Data: answer}, myTr, nil, log)
	if !out.dpdResp || out.dpdRespMsgID != probeID {
		t.Fatalf("the answer was read as %+v, want a response at id %d", out, probeID)
	}

	handleDPDResponse(dpd, log, "ze")
	if dpd.awaitingReply() {
		t.Error("the answered probe still waits for a reply")
	}
	if ini.requestOutstanding {
		t.Error("the answered probe still holds the request window")
	}
}

// VALIDATES: a probe whose build fails writes nothing and frees the request window it
// reserved.
// PREVENTS: a failed build that leaves the window held. The next request on the SA
// would then be refused, and a rekey and a Delete would be refused with it.
func TestDpdBuildFailureReleasesWindow(t *testing.T) {
	ini, _, _, peerTr, myTr := dpdProbeLink(t)
	remote := ini.remoteUDPAddr()
	if remote == nil {
		t.Fatal("the initiator has no resolvable peer address")
	}

	// An encryption key of an invalid length fails the cipher, and that is the one
	// failure a probe meets after the window is reserved.
	ini.SKKeys.SK_ei = make([]byte, 7)

	dpd := winDueDPD()
	sendDPD(ini, myTr, dpd, slogutil.DiscardLogger())

	rtxExpectSilence(t, peerTr, myTr, remote, "a probe whose build failed")
	if ini.requestOutstanding {
		t.Error("the failed probe still holds the request window")
	}
	if dpd.awaitingReply() {
		t.Error("the failed probe waits for an answer to a datagram it never sent")
	}
}
