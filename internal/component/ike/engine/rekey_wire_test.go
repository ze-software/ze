package engine

import (
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// espSAPayload builds the wire SA payload a peer sends in a CREATE_CHILD_SA rekey. It
// holds one ESP proposal with the SPI and the transforms of the test ESP group.
//
// The transforms are part of the fixture. The initiator checks an accepted offer against
// the proposals it sent (RFC 7296 Section 3.3.6, verifyAcceptedOffer). A proposal with no
// transform names no suite, so a peer never sends one.
func espSAPayload(spi uint32) *wire.PayloadSA {
	return &wire.PayloadSA{Proposals: buildWireESPProposals(testESPGroup(), spi)}
}

func testNonce(seed byte) []byte {
	n := make([]byte, nonceLen)
	for i := range n {
		n[i] = seed + byte(i)
	}
	return n
}

// VALIDATES: RFC 7296 §2.3 message-ID window classification and response caching
// (AC-6): a request at ExpectedMsgID is new, a repeat is a retransmit, other IDs
// are invalid, and a response matches only its outstanding request.
// PREVENTS: reprocessing replayed requests or accepting out-of-window messages.
func TestClassifyInboundMessageIDWindow(t *testing.T) {
	sa := &SA{ExpectedMsgID: 5}

	if got := classifyInbound(sa, 5, false, nil); got != inboundNewRequest {
		t.Fatalf("request at ExpectedMsgID: got %v, want inboundNewRequest", got)
	}
	cacheResponse(sa, 5, []byte("resp"))
	if sa.ExpectedMsgID != 6 {
		t.Fatalf("ExpectedMsgID after cache = %d, want 6", sa.ExpectedMsgID)
	}
	if got := classifyInbound(sa, 5, false, nil); got != inboundRetransmit {
		t.Fatalf("repeat request: got %v, want inboundRetransmit", got)
	}
	if got := classifyInbound(sa, 9, false, nil); got != inboundInvalid {
		t.Fatalf("out-of-window request: got %v, want inboundInvalid", got)
	}

	p := &pendingRekey{messageID: 7}
	if got := classifyInbound(sa, 7, true, p); got != inboundResponse {
		t.Fatalf("matching response: got %v, want inboundResponse", got)
	}
	if got := classifyInbound(sa, 8, true, p); got != inboundInvalid {
		t.Fatalf("mismatched response: got %v, want inboundInvalid", got)
	}
	if got := classifyInbound(sa, 7, true, nil); got != inboundInvalid {
		t.Fatalf("response with no pending: got %v, want inboundInvalid", got)
	}
}

// VALIDATES: the initiator processing of a CREATE_CHILD_SA response (AC-2):
// keys are derived from our Ni and the peer's Nr, the new Child SA carries our
// proposed inbound SPI and the peer's outbound SPI, it is installed, and the old
// SA is NOT removed here (make-before-break: the caller removes it after).
// PREVENTS: a rekey that swaps to un-negotiated keys or tears down before install.
func TestApplyChildRekeyResponse(t *testing.T) {
	sa := testSA()
	dp := &mockDP{}
	log := slogutil.DiscardLogger()

	old, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	installedBefore := len(dp.sas)

	const peerSPI = 0xAABBCCDD
	const ourSPI = 0x11223344
	pending := &pendingRekey{
		kind:          rekeyChild,
		localNonce:    testNonce(1),
		newInboundSPI: ourSPI,
		oldChild:      old,
	}
	inner := []wire.PayloadEntry{
		{Payload: espSAPayload(peerSPI)},
		{Payload: &wire.PayloadNonce{NonceData: testNonce(2)}},
	}

	child, err := applyChildRekeyResponse(sa, pending, inner, dp, log)
	if err != nil {
		t.Fatalf("applyChildRekeyResponse: %v", err)
	}
	if child.InboundSPI != ourSPI {
		t.Errorf("inbound SPI = %#x, want %#x", child.InboundSPI, ourSPI)
	}
	if child.OutboundSPI != peerSPI {
		t.Errorf("outbound SPI = %#x, want %#x", child.OutboundSPI, peerSPI)
	}
	if child.Keys == nil || len(child.Keys.EncryptKeyI) == 0 {
		t.Error("new child keys not derived")
	}
	if len(dp.sas) != installedBefore+2 {
		t.Errorf("installed SAs = %d, want %d (new child installed)", len(dp.sas), installedBefore+2)
	}
	if len(dp.removed) != 0 {
		t.Errorf("removed SAs = %d, want 0 (make-before-break: caller removes old)", len(dp.removed))
	}
}

// VALIDATES: the responder handling of a peer-initiated CREATE_CHILD_SA rekey
// (AC-3): a replacement Child SA is installed, and the response is an SK-encrypted
// CREATE_CHILD_SA that echoes the request message ID and carries the Response flag.
// PREVENTS: a responder that logs but never replies (the pre-fix stub behavior).
func TestRespondChildRekey(t *testing.T) {
	sa := testSAWithGCMKeys(t)
	sa.ESPGroup = testESPGroup()
	dp := &mockDP{}
	log := slogutil.DiscardLogger()

	old, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	installedBefore := len(dp.sas)

	const reqMsgID = 5
	inner := []wire.PayloadEntry{
		{Payload: espSAPayload(0x01020304)},
		{Payload: &wire.PayloadNonce{NonceData: testNonce(3)}},
	}

	resp, child, err := respondChildRekey(sa, inner, old, reqMsgID, dp, log)
	if err != nil {
		t.Fatalf("respondChildRekey: %v", err)
	}
	if child == nil || len(dp.sas) != installedBefore+2 {
		t.Fatalf("responder did not install replacement child SA")
	}

	var m wire.Message
	if err := m.ReadFrom(resp); err != nil {
		t.Fatalf("response does not parse: %v", err)
	}
	if m.Header.ExchangeType != wire.ExchangeCreateChildSA {
		t.Errorf("response exchange = %d, want CREATE_CHILD_SA", m.Header.ExchangeType)
	}
	if m.Header.MessageID != reqMsgID {
		t.Errorf("response message ID = %d, want %d (echo request)", m.Header.MessageID, reqMsgID)
	}
	if m.Header.Flags&wire.FlagResponse == 0 {
		t.Error("response is missing the Response flag")
	}
}

// VALIDATES: the initiator building a Child SA rekey request (AC-1): a fresh
// CREATE_CHILD_SA is emitted (ExchangeType 36, Initiator flag, not Response), the
// message ID is the SA's NextMsgID which then advances, and the pendingRekey
// tracks the exchange for response correlation and retransmission.
// PREVENTS: the pre-fix local-only key roll that never sent a rekey on the wire.
func TestInitiateChildRekey(t *testing.T) {
	sa := testSAWithGCMKeys(t)
	sa.ESPGroup = testESPGroup()
	dp := &mockDP{}
	log := slogutil.DiscardLogger()

	old, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}

	startMsgID := sa.NextMsgID
	msg, pending, err := initiateChildRekey(sa, old)
	if err != nil {
		t.Fatalf("initiateChildRekey: %v", err)
	}
	if pending.messageID != startMsgID {
		t.Errorf("pending message ID = %d, want %d", pending.messageID, startMsgID)
	}
	if sa.NextMsgID != startMsgID+1 {
		t.Errorf("NextMsgID = %d, want %d (advanced)", sa.NextMsgID, startMsgID+1)
	}
	if pending.oldChild != old {
		t.Error("pending does not reference the old child")
	}
	if pending.newInboundSPI == 0 {
		t.Error("pending has no proposed inbound SPI")
	}

	var m wire.Message
	if err := m.ReadFrom(msg); err != nil {
		t.Fatalf("request does not parse: %v", err)
	}
	if m.Header.ExchangeType != wire.ExchangeCreateChildSA {
		t.Errorf("request exchange = %d, want CREATE_CHILD_SA", m.Header.ExchangeType)
	}
	if m.Header.Flags&wire.FlagInitiator == 0 {
		t.Error("request is missing the Initiator flag")
	}
	if m.Header.Flags&wire.FlagResponse != 0 {
		t.Error("request must not carry the Response flag")
	}
}

// VALIDATES: the initiator building an IKE SA rekey request (AC-4): a fresh
// CREATE_CHILD_SA (SA + Ni + KEi, KE mandatory) is emitted, a real DH half is held
// in the pendingRekey for the response, and the message ID advances.
// PREVENTS: the pre-fix self-DH local roll (rekey.go "Simulate DH with self").
func TestInitiateIKERekey(t *testing.T) {
	sa := testSAWithGCMKeys(t)
	ikeGroup := testIKEGroup()

	start := sa.NextMsgID
	msg, pending, err := initiateIKERekey(sa, ikeGroup)
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}
	defer pending.clear()

	if pending.kind != rekeyIKE {
		t.Errorf("pending kind = %v, want rekeyIKE", pending.kind)
	}
	if pending.dh == nil {
		t.Error("pending holds no DH exchange for the response")
	}
	if pending.newInitiatorSPI == ([8]byte{}) {
		t.Error("pending has no new IKE SPI")
	}
	if sa.NextMsgID != start+1 {
		t.Errorf("NextMsgID = %d, want %d", sa.NextMsgID, start+1)
	}

	var m wire.Message
	if err := m.ReadFrom(msg); err != nil {
		t.Fatalf("request does not parse: %v", err)
	}
	if m.Header.ExchangeType != wire.ExchangeCreateChildSA {
		t.Errorf("request exchange = %d, want CREATE_CHILD_SA", m.Header.ExchangeType)
	}
	if m.Header.Flags&wire.FlagInitiator == 0 {
		t.Error("request is missing the Initiator flag")
	}
}

// VALIDATES: the initiator processing an IKE SA rekey response (AC-4, AC-7): DH
// completes from the peer's KEr, a new IKE SA is derived carrying our new SPI and
// the peer's, with fresh SK keys and message-ID counters reset to 0.
// PREVENTS: a rekeyed IKE SA that reuses old keys/SPIs or stale message IDs.
func TestApplyIKERekeyResponse(t *testing.T) {
	sa := testSAWithGCMKeys(t)
	ikeGroup := testIKEGroup()
	log := slogutil.DiscardLogger()

	_, pending, err := initiateIKERekey(sa, ikeGroup)
	if err != nil {
		t.Fatalf("initiateIKERekey: %v", err)
	}

	// Peer response: SA (8-byte responder SPI), Nr, KEr (a real DH pubkey of the
	// same group so the shared-secret computation succeeds).
	dh2, err := crypto.NewDHExchange(crypto.DHGroupID(ikeGroup.Proposals[0].DHGroup))
	if err != nil {
		t.Fatalf("NewDHExchange: %v", err)
	}
	defer dh2.Clear()

	respSPI := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	// The accepted offer carries the transforms of the IKE group we proposed. The
	// initiator checks it against its own proposals (RFC 7296 Section 3.3.6), so a
	// proposal with no transform is not something a peer sends.
	respProps := buildWireIKEProposals(ikeGroup)
	for i := range respProps {
		respProps[i].SPISize = 8
		respProps[i].SPI = respSPI
	}
	inner := []wire.PayloadEntry{
		{Payload: &wire.PayloadSA{Proposals: respProps}},
		{Payload: &wire.PayloadNonce{NonceData: testNonce(7)}},
		{Payload: &wire.PayloadKE{DHGroup: uint16(ikeGroup.Proposals[0].DHGroup), KeyExchangeData: dh2.PublicKey}},
	}

	newSA, err := applyIKERekeyResponse(sa, pending, inner, log)
	if err != nil {
		t.Fatalf("applyIKERekeyResponse: %v", err)
	}
	if newSA.InitiatorSPI != pending.newInitiatorSPI {
		t.Error("new SA does not carry our proposed initiator SPI")
	}
	var wantResp [8]byte
	copy(wantResp[:], respSPI)
	if newSA.ResponderSPI != wantResp {
		t.Errorf("new SA responder SPI = %x, want %x", newSA.ResponderSPI, wantResp)
	}
	if newSA.SKKeys == nil || len(newSA.SKKeys.SK_ei) == 0 {
		t.Error("new SA SK keys not derived")
	}
	if newSA.State != StateEstablished {
		t.Errorf("new SA state = %v, want established", newSA.State)
	}
	if newSA.NextMsgID != 0 || newSA.ExpectedMsgID != 0 {
		t.Errorf("message IDs not reset: next=%d expected=%d", newSA.NextMsgID, newSA.ExpectedMsgID)
	}
	pending.clear()
	newSA.SKKeys.Clear()
}

// VALIDATES: simultaneous-rekey collision resolution (AC-5, §2.8.1): when we have
// initiated a Child SA rekey and the peer's competing request carries the LOWER nonce,
// the peer closes its own exchange, so we ignore its request and keep ours.
// PREVENTS: both peers installing duplicate replacement SAs on a rekey race.
func TestChildRekeyCollisionWeWin(t *testing.T) {
	log := slogutil.DiscardLogger()
	sa := testSA()
	ps := &PeerSession{peerName: "test-peer"}
	localNonce := make([]byte, nonceLen)
	localNonce[0] = 0xff // higher than the peer nonce below, so our exchange survives
	ps.pendingRekey = &pendingRekey{kind: rekeyChild, localNonce: localNonce, messageID: 1}

	peerNi := make([]byte, nonceLen) // all zero, so the peer holds the lowest nonce
	inner := []wire.PayloadEntry{
		{Payload: &wire.PayloadNotify{ProtocolID: wire.ProtocolESP, SPISize: 4, NotifyMsgType: wire.NotifyRekeySA, SPI: []byte{0, 0, 0, 1}}},
		{Payload: &wire.PayloadNonce{NonceData: peerNi}},
		{Payload: espSAPayload(0x1234)},
	}
	msg := &wire.Message{Header: wire.Header{MessageID: 5}}

	out := ps.handleCreateChildSAOwned(sa, msg, inner, false, nil, nil, log)
	if out.newChild != nil || out.newSA != nil {
		t.Error("winner must not install anything from the peer's losing request")
	}
	if ps.pendingRekey == nil {
		t.Error("winner must keep its own pending rekey")
	}
}

// VALIDATES: a malformed peer rekey request (REKEY_SA but no Nonce) does NOT make
// us abandon our own in-flight rekey via a bogus collision resolution.
// PREVENTS: an empty peer nonce reaching localNonceIsLower and deciding a collision
// the peer never opened.
func TestChildRekeyCollisionMalformedKeepsPending(t *testing.T) {
	log := slogutil.DiscardLogger()
	sa := testSA()
	ps := &PeerSession{peerName: "test-peer"}
	ps.pendingRekey = &pendingRekey{kind: rekeyChild, localNonce: make([]byte, nonceLen), messageID: 3}

	inner := []wire.PayloadEntry{
		{Payload: &wire.PayloadNotify{ProtocolID: wire.ProtocolESP, SPISize: 4, NotifyMsgType: wire.NotifyRekeySA, SPI: []byte{0, 0, 0, 1}}},
		{Payload: espSAPayload(0x1234)}, // no Nonce payload -> malformed
	}
	msg := &wire.Message{Header: wire.Header{MessageID: 5}}

	// ps.childSA is nil, so the responder path returns early after the guarded
	// collision check; the point is that our pendingRekey survives.
	out := ps.handleCreateChildSAOwned(sa, msg, inner, false, nil, nil, log)
	if out.newChild != nil {
		t.Error("no child should be installed from a malformed request")
	}
	if ps.pendingRekey == nil {
		t.Error("our in-flight rekey must survive a malformed peer request")
	}
}

// VALIDATES: a rekey request whose response never arrives is retransmitted and,
// once retransmissions are exhausted, tears the SA down (AC-8) rather than running
// on soon-to-expire keys.
// PREVENTS: a silently stuck rekey leaving a half-negotiated tunnel.
func TestRekeyRetransmitExhaustionTeardown(t *testing.T) {
	log := slogutil.DiscardLogger()
	sa := testSA()
	ps := &PeerSession{peerName: "test-peer"}

	// Not yet exhausted: an overdue request retransmits and bumps the counter.
	ps.pendingRekey = &pendingRekey{kind: rekeyChild, sentAt: time.Now().Add(-time.Hour), sentMsg: []byte("req")}
	if err := ps.serviceRekeyRetransmit(sa, nil, time.Now(), &mockDP{}, nil, log); err != nil {
		t.Fatalf("retransmit should not error: %v", err)
	}
	if ps.pendingRekey == nil || ps.pendingRekey.retransmits != 1 {
		t.Fatalf("retransmit count not advanced")
	}

	// Exhausted: tears down (errTimeout) and clears the pending rekey.
	ps.pendingRekey = &pendingRekey{kind: rekeyChild, sentAt: time.Now().Add(-time.Hour), retransmits: maxRetransmissions}
	if err := ps.serviceRekeyRetransmit(sa, nil, time.Now(), &mockDP{}, nil, log); !errors.Is(err, errTimeout) {
		t.Fatalf("expected errTimeout on exhaustion, got %v", err)
	}
	if ps.pendingRekey != nil {
		t.Error("pending rekey must be cleared on teardown")
	}
}
