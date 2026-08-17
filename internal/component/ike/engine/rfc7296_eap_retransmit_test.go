package engine

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// eaprtxResponderMidExchange drives a real EAP-MSCHAPv2 handshake to the point where the
// responder has answered the first IKE_AUTH and sits in StateEAPInProgress. It returns the
// responder SA, the session that owns it, and the exact IKE_AUTH request bytes the
// initiator sent.
//
// Those bytes are what the peer retransmits when the response does not reach it, and
// RFC 7296 Section 2.1 makes that retransmission bitwise identical to the original.
// Delivering the same slice twice is therefore the real event and not a stand-in for it.
//
// It follows eapHandshakeShapes (rfc7296_eap_auth_producer_test.go) and stops one delivery
// earlier, because the defect it serves lives in the window the EAP exchange holds open.
func eaprtxResponderMidExchange(t *testing.T) (resp *SA, ps *PeerSession, authReq []byte) {
	t.Helper()
	log := slogutil.DiscardLogger()
	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	autLoadPKI(t)
	iniPeer, respPeer := autPeers(ipsec.AuthConfig{
		Mode:          ipsec.AuthEAPMSCHAPv2,
		PSK:           "eap-pass",
		Certificate:   autCertName,
		CACertificate: autCAName,
	})

	table := NewSATable()
	ini, err := newInitiatorSA("ze", iniPeer, ikeGroup, espGroup)
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}
	table.Insert(ini)
	saInitReq := buildSAInitRequest(ini, ikeGroup)
	ini.InitiatorSAInitMsg = saInitReq
	ini.State = StateSAInitSent

	resp, err = newResponderSA("ze", respPeer, ikeGroup, espGroup, ini.InitiatorSPI)
	if err != nil {
		t.Fatalf("newResponderSA: %v", err)
	}
	ps = &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	ps.setSA(resp)
	setActivePeers(map[string]*PeerSession{"ze": ps})
	t.Cleanup(func() { setActivePeers(nil) })

	// Both IKE_SA_INIT deliveries run with no transport, so nothing leaves the host until
	// the caller's link is in place and the IKE_AUTH arrives on it.
	handleSAInitRequest(resp, parseMsg(t, saInitReq), saInitReq, nil, nil, log)
	handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, nil, log)

	authReq = ini.LastSentMsg
	if len(authReq) == 0 {
		t.Fatal("the initiator sent no IKE_AUTH request")
	}
	return resp, ps, authReq
}

// eaprtxPeerAddr returns the address the stand-in peer listens on.
func eaprtxPeerAddr(t *testing.T, peerTr *transport.UDPTransport) *net.UDPAddr {
	t.Helper()
	addr, ok := peerTr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("peer transport local address is not *net.UDPAddr")
	}
	return addr
}

// VALIDATES: a retransmitted IKE_AUTH that arrives while the EAP exchange is still running
// is answered from the cached response, and the EAP round is not run a second time.
//
// PREVENTS: the responder killing a live IKE SA because it re-processed a duplicate. The
// retransmit carries IDi and SAi2 and no EAP payload, so re-processing reaches the
// eapPayload == nil arm of handleResponderEAP (responder_eap.go) and sets StateDead. That
// is what strongSwan met in interop scenario 08: the peer retransmits IKE_AUTH #1 and ze
// answers by tearing the SA down.
//
// RFC requirement: RFC7296-2.1-4 positive -- the duplicate is ignored "except insofar as it
// causes a retransmission of the response". replayCachedResponse (responder.go) returns
// before handleResponderEAP runs, so the EAP state machine never sees the duplicate, and
// the cached bytes go back.
// RFC requirement: RFC7296-2.1-4 negative -- the SAME bytes on FIRST delivery DO drive the
// EAP round and produce a freshly built response, so the quiet second pass comes from the
// duplicate path and not from an inert handler.
// RFC requirement: RFC7296-2.1-3 positive -- the response comes back byte for byte. A
// rebuild cannot match it, because every build draws a fresh random CBC IV (auth.go).
func TestEapRtxResponderReplaysCachedResponseMidEAP(t *testing.T) {
	log := slogutil.DiscardLogger()
	resp, ps, authReq := eaprtxResponderMidExchange(t)
	peerTr, myTr := rtxPeerLink(t)
	peerAddr := eaprtxPeerAddr(t, peerTr)
	pkt := transport.Packet{Data: authReq, RemoteAddr: peerAddr}

	// First delivery. The EAP exchange starts and the responder answers.
	ps.handleResponderInbound(resp, parseMsg(t, authReq), pkt, myTr, log)
	if resp.State != StateEAPInProgress {
		t.Fatalf("state after the first IKE_AUTH = %v, want EAP in progress", resp.State)
	}
	first := rtxRecv(t, peerTr)
	if first == nil {
		t.Fatal("the first IKE_AUTH drew no EAP response")
	}
	if !resp.lastResponseSet {
		t.Fatal("the EAP path cached no response, so no retransmission could ever be answered")
	}

	// Second delivery of the same datagram: the peer's retransmission.
	ps.handleResponderInbound(resp, parseMsg(t, authReq), pkt, myTr, log)
	if resp.State == StateDead {
		t.Fatal("a retransmitted IKE_AUTH killed the IKE SA mid-EAP")
	}
	if resp.State != StateEAPInProgress {
		t.Fatalf("state after the retransmission = %v, want EAP in progress", resp.State)
	}
	replay := rtxRecv(t, peerTr)
	if replay == nil {
		t.Fatal("the retransmitted IKE_AUTH drew no answer")
	}
	if !bytes.Equal(replay, first) {
		t.Fatal("the answer to the retransmission was rebuilt rather than replayed from cache")
	}
}

// VALIDATES: an unprotected datagram at the cached message id draws no response mid-EAP,
// and does not disturb the EAP exchange.
//
// PREVENTS: a mid-EAP replay that answers a forged 28-byte header. The cached IKE_AUTH
// response is several hundred octets. Answering one makes ze an amplifier that a spoofed
// source aims (RFC 7296 Section 2.21.4).
//
// Before replayCachedResponse existed this forgery was worse than an amplifier.
// handleResponderEAP tried to decrypt it, failed, and set StateDead. One 28-byte datagram
// built from an observed header therefore killed a live IKE SA.
//
// The established arm of handleResponderInbound already carried this guard. A guard on
// one replay site with the sibling left open is the failure ai/rules/architecture.md
// names.
//
// RFC requirement: RFC7296-2.1-3 negative -- a datagram at the cached message id that
// carries no Encrypted payload is not a retransmission of the request, and it draws no
// response at all. The responder therefore resends for a genuine duplicate alone.
func TestEapRtxMidEAPReplayRefusesUnprotected(t *testing.T) {
	log := slogutil.DiscardLogger()
	resp, ps, authReq := eaprtxResponderMidExchange(t)
	peerTr, myTr := rtxPeerLink(t)
	peerAddr := eaprtxPeerAddr(t, peerTr)
	pkt := transport.Packet{Data: authReq, RemoteAddr: peerAddr}

	ps.handleResponderInbound(resp, parseMsg(t, authReq), pkt, myTr, log)
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the first IKE_AUTH drew no EAP response")
	}

	// A bare header at the cached message id. It carries no Encrypted payload, so it is
	// a forgery an off-path attacker can build from one observed datagram. Both SPIs and
	// the Message ID travel in the clear in every IKE header.
	forged := &wire.Message{Header: wire.Header{
		InitiatorSPI: resp.InitiatorSPI,
		ResponderSPI: resp.ResponderSPI,
		MajorVersion: 2,
		ExchangeType: wire.ExchangeIKEAuth,
		MessageID:    resp.lastResponseID,
	}}
	ps.handleResponderInbound(resp, forged, pkt, myTr, log)
	if resp.State != StateEAPInProgress {
		t.Fatalf("state after the forgery = %v, want EAP in progress", resp.State)
	}
	rtxExpectSilence(t, peerTr, myTr, peerAddr, "unprotected message at the cached message id")
}

// VALIDATES: the token bucket that bounds the established replay site also bounds the
// mid-EAP one, so a flood of well-formed duplicates cannot draw one response each.
//
// PREVENTS: an amplifier an attacker can run without bound. carriesSKPayload raises the
// cost of a forgery from a 28-byte header to about forty octets. It does not remove the
// amplification, so a rate limit is needed beside it: RFC 7296 Section 2.4 MUST,
// "Implementations MUST limit the rate at which they take actions based on unprotected
// messages" (`RFC7296-2.4-12`).
//
// This carries no RFC requirement tag. The bound is a Section 2.4 rate-limit defense
// rather than a Section 2.1 retransmission obligation. The two Section 2.1 rows are
// proven by the two tests above. Section 2.21.4 is NOT the source: it opens with "A node
// needs to limit the rate ...", which is not RFC 2119 language.
func TestEapRtxMidEAPReplayIsRateLimited(t *testing.T) {
	log := slogutil.DiscardLogger()
	resp, ps, authReq := eaprtxResponderMidExchange(t)
	peerTr, myTr := rtxPeerLink(t)
	peerAddr := eaprtxPeerAddr(t, peerTr)
	pkt := transport.Packet{Data: authReq, RemoteAddr: peerAddr}

	ps.handleResponderInbound(resp, parseMsg(t, authReq), pkt, myTr, log)
	if rtxRecv(t, peerTr) == nil {
		t.Fatal("the first IKE_AUTH drew no EAP response")
	}

	// Nothing is read between deliveries. The count comes from one drain afterwards.
	//
	// Waiting for an answer the limiter is refusing would refill the bucket during the
	// wait. allow() adds elapsed*rate tokens (notify_error.go). A read that blocks for
	// rtxArrive therefore hands back that many more answers, and the assertion would
	// measure the test's own patience.
	//
	// One socket keeps send order on loopback. Every replay is therefore queued ahead of
	// the sentinel imiDrain sends.
	duplicates := cachedReplayBurst + 8
	start := time.Now()
	for range duplicates {
		ps.handleResponderInbound(resp, parseMsg(t, authReq), pkt, myTr, log)
	}
	elapsed := time.Since(start)
	answered := len(imiDrain(t, peerTr, myTr, peerAddr, "mid-EAP replay flood"))

	// The floor keeps the ceiling from passing vacuously. A replay path that answered
	// nothing at all would satisfy any upper bound.
	if answered == 0 {
		t.Fatal("a mid-EAP duplicate drew no answer at all, so the bound below proves nothing")
	}
	// The bucket starts full and refills by elapsed*rate. The loop above runs in-process
	// and does no crypto, so a whole extra token needs a second of wall clock. Name that
	// cause rather than let it read as a missing limiter.
	if answered > cachedReplayBurst && elapsed < time.Second {
		t.Errorf("a mid-EAP replay flood of %d drew %d answers in %v, want at most the burst of %d",
			duplicates, answered, elapsed, cachedReplayBurst)
	}
	if resp.State != StateEAPInProgress {
		t.Fatalf("state after the flood = %v, want EAP in progress", resp.State)
	}
}
