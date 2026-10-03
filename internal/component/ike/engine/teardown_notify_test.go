package engine

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// tdnSent drives sendIKESATeardown on an established loopback SA and returns the decrypted
// payload chain the peer received, plus the exchange type of the datagram.
func tdnSent(t *testing.T, notifyType uint16) ([]wire.PayloadEntry, uint8) {
	t.Helper()
	log := slogutil.DiscardLogger()
	local, peer, _, peerTr, myTr := lcyLoopback(t)

	sendIKESATeardown(local, myTr, notifyType, log)

	raw := rtxRecv(t, peerTr)
	if raw == nil {
		t.Fatal("the teardown sent nothing at all; the peer keeps an SA ze has deleted")
	}
	return lcyDecrypt(t, peer, raw), parseMsg(t, raw).Header.ExchangeType
}

// tdnAbandonedAuth runs a real PSK handshake that ends in a teardown, and returns the
// initiator SA, the responder SA, the Message ID the IKE_AUTH REQUEST carried, and the raw
// teardown datagram the peer received.
//
// The refusal is genuine rather than arranged. The initiator asks for transport mode and
// its configuration makes tunnel mode unacceptable. The responder is configured for tunnel
// mode, so it answers WITHOUT a USE_TRANSPORT_MODE notification, which RFC 7296 Section
// 1.3.1 makes the decline: "If the responder declines the request, the Child SA will be
// established in tunnel mode. If this is unacceptable to the initiator, the initiator MUST
// delete the SA." handleAuthResponse (fsm.go) therefore takes its teardown arm.
//
// Driving the real handshake is the point. sendIKESATeardown reads sa.NextMsgID, and only
// the handshake leaves that counter where production leaves it: on the id of the IKE_AUTH
// request, not on a free one. A test that arranges an established SA cannot see the defect
// this reproduces, because an established SA's counter is already free.
func tdnAbandonedAuth(t *testing.T) (resp *SA, authMsgID uint32, teardown []byte) {
	t.Helper()
	log := slogutil.DiscardLogger()
	peerTr, myTr := rtxPeerLink(t)

	ikeGroup := testIKEGroup()
	espGroup := testESPGroup()
	iniPeer, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "teardown-msgid-psk")
	// rtxPeerLink points remoteUDPAddr at peerTr through the ze.test.ike.port seam, so the
	// teardown has somewhere to land. Both ends use the same literal, which also keeps the
	// single-address traffic selectors of RFC 7296 Section 2.23.1 satisfiable.
	iniPeer.LocalAddress, iniPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"
	respPeer.LocalAddress, respPeer.RemoteAddress = "127.0.0.1", "127.0.0.1"
	iniPeer.Mode = dataplane.ModeTransport
	iniPeer.TransportRequired = true

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
	handleSAInitRequest(resp, parseMsg(t, saInitReq), saInitReq, nil, nil, log)
	handleSAInitResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, nil, nil, log)

	// The id the IKE_AUTH request spends. RFC 7296 Section 2.2 fixes it at 1.
	authMsgID = parseMsg(t, ini.LastSentMsg).Header.MessageID

	ps := &PeerSession{peerName: "ze", peerCfg: respPeer, ikeGroup: ikeGroup, espGroup: espGroup}
	ps.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, nil, nil, log)
	if resp.State != StateEstablished {
		t.Fatalf("the responder did not establish (state %v), so there is no refusal to test", resp.State)
	}

	handleAuthResponse(ini, parseMsg(t, resp.LastSentMsg), resp.LastSentMsg, table, myTr, log)
	if ini.State != StateDead {
		t.Fatalf("the initiator accepted a declined transport-mode response (state %v); "+
			"RFC 7296 Section 1.3.1 requires it to delete the SA", ini.State)
	}

	teardown = rtxRecv(t, peerTr)
	if teardown == nil {
		t.Fatal("the abandoned handshake sent nothing at all; the peer keeps an SA ze has deleted")
	}
	return resp, authMsgID, teardown
}

// tdnNotifies returns every Notify payload in a decrypted chain.
func tdnNotifies(inner []wire.PayloadEntry) []*wire.PayloadNotify {
	var out []*wire.PayloadNotify
	for i := range inner {
		if n, ok := inner[i].Payload.(*wire.PayloadNotify); ok {
			out = append(out, n)
		}
	}
	return out
}

// VALIDATES: an authenticated IKE SA that ze abandons because the IKE_AUTH response was
// unacceptable is torn down ON THE WIRE, in an INFORMATIONAL exchange carrying both the
// error notification and an IKE Delete payload.
//
// PREVENTS: the defect this test exists for. handleAuthResponse set State to StateDead and
// returned, sending nothing. The peer kept both SAs and went on encrypting to a node that
// had none, while ze's own log line claimed the SA was being deleted.
//
// The Delete payload is the load-bearing half. RFC 7296 Section 2.21.2
// (rfc/full/rfc7296.txt:3317-3321) closes the set of notifications that delete an IKE SA
// WITHOUT one: UNSUPPORTED_CRITICAL_PAYLOAD, INVALID_SYNTAX and AUTHENTICATION_FAILED.
// TS_UNACCEPTABLE is in none of them, so a bare notify would not end the peer's SA.
func TestTdnTeardownCarriesBothDeleteAndNotify(t *testing.T) {
	inner, exchange := tdnSent(t, wire.NotifyTSUnacceptable)

	if exchange != wire.ExchangeInformational {
		t.Errorf("the teardown used exchange type %d, want INFORMATIONAL (%d); RFC 7296 "+
			"Section 2.21.2 puts it in a separate INFORMATIONAL exchange",
			exchange, wire.ExchangeInformational)
	}

	dels := lcyDeletes(inner)
	if len(dels) != 1 {
		t.Fatalf("the teardown carries %d Delete payloads, want exactly 1; without one the "+
			"peer's IKE SA is not deleted at all", len(dels))
	}
	if dels[0].ProtocolID != wire.ProtocolIKE {
		t.Errorf("the teardown Delete names protocol %d, want IKE (%d)", dels[0].ProtocolID, wire.ProtocolIKE)
	}
	// RFC 7296 Section 3.11: an IKE Delete carries no SPIs.
	if dels[0].NumSPIs != 0 {
		t.Errorf("the IKE Delete names %d SPIs, want none", dels[0].NumSPIs)
	}

	notifies := tdnNotifies(inner)
	if len(notifies) != 1 {
		t.Fatalf("the teardown carries %d Notify payloads, want exactly 1", len(notifies))
	}
	if notifies[0].NotifyMsgType != wire.NotifyTSUnacceptable {
		t.Errorf("the teardown reports notify %d, want TS_UNACCEPTABLE (%d)",
			notifies[0].NotifyMsgType, wire.NotifyTSUnacceptable)
	}
}

// VALIDATES: a teardown that is nobody's error carries the Delete alone.
//
// This is the discriminator for the test above: it proves the Notify payload is a decision
// about the CAUSE and not a payload the sender attaches unconditionally. RFC 7296 Section
// 1.3.1 lets the responder decline transport mode, so an initiator that finds tunnel mode
// unacceptable deletes the SA without accusing the peer of anything.
func TestTdnTeardownWithoutACauseSendsDeleteAlone(t *testing.T) {
	inner, _ := tdnSent(t, 0)

	if dels := lcyDeletes(inner); len(dels) != 1 {
		t.Fatalf("the teardown carries %d Delete payloads, want exactly 1", len(dels))
	}
	if notifies := tdnNotifies(inner); len(notifies) != 0 {
		t.Errorf("a teardown with no error cause still carried %d Notify payloads", len(notifies))
	}
}

// VALIDATES: the teardown INFORMATIONAL carries a Message ID no earlier request on the SA
// has spent, so the peer PROCESSES it (RFC 7296 Section 2.2: a request takes a new id).
//
// PREVENTS: the teardown never reaching the peer at all. sendIKESATeardown built at
// sa.NextMsgID, and at both of its call sites in handleAuthResponse that field still held
// the id of the IKE_AUTH REQUEST: handleSAInitResponse sets it to 1, and the only advance
// past it runs on the success path AFTER both teardown arms. Ze's own responder had cached
// its IKE_AUTH response under that id (finishResponderEstablish -> cacheResponse), so
// classifyInbound read the teardown as inboundRetransmit and REPLAYED the cached IKE_AUTH
// response. The Delete was never processed, and the peer kept the SA the teardown exists to
// end -- the exact failure TestTdnTeardownCarriesBothDeleteAndNotify was written to stop,
// still live because that test asserted the payloads and never the id.
//
// The second assertion is the one that measures the defect. Comparing the id against the
// IKE_AUTH's only proves they differ; running the real peer's classifier proves the peer
// acts on the Delete rather than answering from its cache.
func TestTdnTeardownSpendsAFreeMessageID(t *testing.T) {
	resp, authMsgID, teardown := tdnAbandonedAuth(t)

	hdr := parseMsg(t, teardown).Header
	if hdr.MessageID == authMsgID {
		t.Errorf("the teardown carries Message ID %d, the same id as the IKE_AUTH request; "+
			"RFC 7296 Section 2.2 gives every request a new one", hdr.MessageID)
	}
	if hdr.Flags&wire.FlagResponse != 0 {
		t.Error("the teardown is marked as a response; it is a new request (RFC 7296 Section 2.21.2)")
	}

	// The peer's own window logic, on the responder SA that answered the IKE_AUTH. Its
	// cached response still sits under authMsgID, which is what made the reuse invisible.
	if resp.lastResponseID != authMsgID || !resp.lastResponseSet {
		t.Fatalf("the responder cached its IKE_AUTH response under id %d (set=%v), want %d; "+
			"without that cache this test cannot see the retransmit trap",
			resp.lastResponseID, resp.lastResponseSet, authMsgID)
	}
	if got := classifyInbound(resp, hdr.MessageID, false, nil); got != inboundNewRequest {
		t.Errorf("the peer classified the teardown as %d, want inboundNewRequest (%d); "+
			"inboundRetransmit (%d) means it replays its cached IKE_AUTH response and never "+
			"processes the Delete", got, inboundNewRequest, inboundRetransmit)
	}
}

// VALIDATES: the teardown that a real abandoned handshake sends carries the IKE Delete.
//
// This is the end-to-end half of TestTdnTeardownWithoutACauseSendsDeleteAlone, which drives
// sendIKESATeardown directly. RFC 7296 Section 1.3.1's decline is nobody's protocol error,
// so the Delete travels alone.
func TestTdnAbandonedAuthSendsAnIKEDelete(t *testing.T) {
	resp, _, teardown := tdnAbandonedAuth(t)

	inner := lcyDecrypt(t, resp, teardown)
	dels := lcyDeletes(inner)
	if len(dels) != 1 {
		t.Fatalf("the teardown carries %d Delete payloads, want exactly 1; without one the "+
			"peer's IKE SA is not deleted at all", len(dels))
	}
	if dels[0].ProtocolID != wire.ProtocolIKE {
		t.Errorf("the teardown Delete names protocol %d, want IKE (%d)", dels[0].ProtocolID, wire.ProtocolIKE)
	}
	if notifies := tdnNotifies(inner); len(notifies) != 0 {
		t.Errorf("a declined transport-mode teardown carried %d Notify payloads, want none; "+
			"the peer broke nothing", len(notifies))
	}
}

// VALIDATES: the initiator loop stops as soon as the handshake is abandoned, instead of
// retransmitting the request of an SA that no longer exists.
//
// PREVENTS: the second half of the same defect. handleAuthResponse marks a failed SA
// StateDead on the dispatch goroutine, and runInitiator looped on
// `sa.State != StateEstablished` alone. A refused IKE_AUTH response therefore left the loop
// resending the IKE_AUTH request up to maxRetransmissions times before giving up, and the
// reconnect waited out the whole retransmit schedule rather than the reconnect backoff.
func TestTdnInitiatorLoopStopsWhenTheHandshakeIsAbandoned(t *testing.T) {
	log := slogutil.DiscardLogger()
	_, myTr := rtxPeerLink(t)

	peer := testPeer()
	peer.LocalAddress = "127.0.0.1"
	peer.RemoteAddress = "127.0.0.1"
	ps := &PeerSession{
		peerName: "ze",
		peerCfg:  peer,
		ikeGroup: testIKEGroup(),
		espGroup: testESPGroup(),
		stopCh:   make(chan struct{}),
	}

	var mu sync.Mutex
	waits := 0
	old := afterFunc
	// The stub runs on the session goroutine. It rewinds the deadline so the next pass
	// retransmits without real elapsed time, and it kills the SA on the FIRST wait, which
	// is what a refused IKE_AUTH response does from the dispatch goroutine.
	afterFunc = func(time.Duration) <-chan time.Time {
		mu.Lock()
		waits++
		mu.Unlock()
		if cur := ps.getSA(); cur != nil {
			cur.RetransmitTime = time.Now().Add(-time.Millisecond)
			cur.State = StateDead
		}
		ch := make(chan time.Time, 1)
		ch <- time.Now()
		return ch
	}
	t.Cleanup(func() { afterFunc = old })

	var err error
	done := make(chan struct{})
	go func() {
		err = ps.runInitiator(peer, testIKEGroup(), NewSATable(), myTr, nil, log)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(rtxArrive):
		close(ps.stopCh)
		<-done
		t.Fatal("the abandoned handshake never reached its verdict")
	}

	if !errors.Is(err, errSADead) {
		t.Errorf("the abandoned handshake ended with %v, want errSADead; errTimeout means the "+
			"loop ran the whole retransmit schedule for an SA that was already deleted", err)
	}

	mu.Lock()
	got := waits
	mu.Unlock()
	// One wait is the loop noticing the deadline. Anything approaching maxRetransmissions
	// means the death was ignored.
	if got >= maxRetransmissions {
		t.Errorf("the loop waited %d times after the SA died, with maxRetransmissions=%d; "+
			"it is still retransmitting an abandoned handshake", got, maxRetransmissions)
	}
}
