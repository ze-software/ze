package engine

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestChildSARekeyInitiator removed -- the local-roll rekeyChildSA it
// exercised was replaced by the real CREATE_CHILD_SA wire exchange (spec-ipsec-13).
// Coverage moved to rekey_wire_test.go: TestInitiateChildRekey (request build),
// TestApplyChildRekeyResponse (key derive + install, replaces the SPI/key asserts),
// TestRespondChildRekey (responder install + reply).

// rfc-test-change-approved: 2026-07-30 Thomas authorized correcting the RFC7296-2.8-1
// collision direction: RFC 7296 section 2.8.1 closes the LOWEST nonce, and the test
// asserted the opposite.

// rfc-test-change-approved: 2026-07-30 Thomas authorized correcting the RFC7296-2.8-1
// collision direction: RFC 7296 section 2.8.1 closes the LOWEST nonce, and the test
// asserted the opposite.
//
// RFC requirement: RFC7296-2.8-1 positive -- localNonceIsLower (rekey.go) states one fact.
// It reports whether our nonce sorts below the peer nonce, octet by octet. RFC 7296 section
// 2.8.1 closes the SA that carries the lowest of the four nonces, so a caller that reads
// true abandons its own exchange.
// RFC requirement: RFC7296-2.8-1 negative -- a higher nonce and an equal nonce both read
// false, so neither one makes us abandon the exchange we started.
func TestRekeyCollision(t *testing.T) {
	localNonce := []byte{0x01, 0x02, 0x03}
	remoteNonce := []byte{0x04, 0x05, 0x06}

	if !localNonceIsLower(localNonce, remoteNonce) {
		t.Error("our nonce sorts below the peer nonce, so the comparison must read true")
	}
	if localNonceIsLower(remoteNonce, localNonce) {
		t.Error("our nonce sorts above the peer nonce, so the comparison must read false")
	}

	// rfc-test-change-approved: 2026-07-30 Thomas authorized correcting the RFC7296-2.8-1
	// collision direction, so this case now states a fact about the comparison.
	sameNonce := []byte{0x01, 0x02, 0x03}
	if localNonceIsLower(sameNonce, sameNonce) {
		t.Error("two equal nonces must not report ours as the lower one")
	}
}

// rfc-test-change-approved: 2026-07-30 Thomas authorized correcting the RFC7296-2.8-1
// collision direction: RFC 7296 section 2.8.1 closes the LOWEST nonce, and the test
// asserted the opposite.
//
// RFC requirement: RFC7296-2.8-1 positive -- the Child SA branch of handleCreateChildSAOwned
// (inbound.go) abandons our own exchange when our nonce is the lower one. RFC 7296 section
// 2.8.1 closes the SA that carries the lowest of the four nonces. The abandoned request frees
// the one request window of section 2.3, so the SA keeps its voice.
// RFC requirement: RFC7296-2.8-1 negative -- the same branch keeps our exchange when our nonce
// is the higher one, and it holds the request window it reserved.
func TestRekeyCollisionLowestNonceAbandons(t *testing.T) {
	log := slogutil.DiscardLogger()

	collide := func(t *testing.T, localFill, peerFill byte) (*SA, *PeerSession) {
		t.Helper()
		sa := testSA()
		if !sa.reserveRequestWindow() {
			t.Fatal("the request window was already held before our rekey")
		}
		ps := &PeerSession{peerName: "test-peer"}
		ps.pendingRekey = &pendingRekey{
			kind:       rekeyChild,
			localNonce: bytes.Repeat([]byte{localFill}, nonceLen),
			messageID:  1,
		}
		inner := []wire.PayloadEntry{
			{Payload: &wire.PayloadNotify{
				ProtocolID:    wire.ProtocolESP,
				SPISize:       4,
				NotifyMsgType: wire.NotifyRekeySA,
				SPI:           []byte{0, 0, 0, 1},
			}},
			{Payload: &wire.PayloadNonce{NonceData: bytes.Repeat([]byte{peerFill}, nonceLen)}},
			{Payload: espSAPayload(0x1234)},
		}
		msg := &wire.Message{Header: wire.Header{MessageID: 5}}
		ps.handleCreateChildSAOwned(sa, msg, inner, false, nil, nil, log)
		return sa, ps
	}

	// Positive. Our nonce is the lower one, so our own exchange is the one that closes.
	sa, ps := collide(t, 0x10, 0xF0)
	if ps.pendingRekey != nil {
		t.Error("our exchange carries the lower nonce, so it must be abandoned")
	}
	if sa.requestOutstanding {
		t.Error("the abandoned exchange still holds the request window")
	}

	// Negative. Our nonce is the higher one, so our own exchange survives.
	sa, ps = collide(t, 0xF0, 0x10)
	if ps.pendingRekey == nil {
		t.Error("our exchange carries the higher nonce, so it must survive")
	}
	if !sa.requestOutstanding {
		t.Error("our surviving exchange released the request window it holds")
	}
}

// rfc-test-change-approved: 2026-07-30 Thomas authorized correcting the RFC7296-2.8-1
// collision direction: RFC 7296 section 2.8.1 closes the LOWEST nonce, and the test
// asserted the opposite.
//
// RFC requirement: RFC7296-2.8-1 positive -- the IKE SA branch of handleCreateChildSAOwned
// (inbound.go) abandons our own exchange when our nonce is the lower one. It frees the request
// window and answers the peer request as it answers an uncontested one.
// RFC requirement: RFC7296-2.8-1 negative -- the same branch keeps our exchange when our nonce
// is the higher one. It writes no answer, and it holds the request window it reserved.
func TestRekeyCollisionIKEBranchLowestNonceAbandons(t *testing.T) {
	log := slogutil.DiscardLogger()
	public := negModpPublic(t)

	// Positive. Our nonce is the lower one, so the peer exchange is the survivor.
	ini, ps, peerTr, myTr := negRekeySession(t)
	negStartIKERekey(t, ini, ps, negNonce(0x10))
	inner := negIKERekeyInner(t, negNonce(0xF0), uint16(crypto.DH_MODP_2048), public)
	ps.handleCreateChildSAOwned(ini, &wire.Message{Header: wire.Header{MessageID: 4}},
		inner, false, myTr, nil, log)
	if ps.pendingRekey != nil {
		t.Error("our exchange carries the lower nonce, so it must be abandoned")
	}
	if ini.requestOutstanding {
		t.Error("the abandoned exchange still holds the request window")
	}
	if ps.pendingIKESwap == nil {
		t.Error("the surviving peer exchange built no new IKE SA")
	}
	if rtxRecv(t, peerTr) == nil {
		t.Error("the surviving peer exchange drew no answer")
	}

	// Negative. Our nonce is the higher one, so we keep our exchange and stay silent.
	ini, ps, peerTr, myTr = negRekeySession(t)
	remote := ini.remoteUDPAddr()
	if remote == nil {
		t.Fatal("the initiator has no resolvable peer address")
	}
	pending := negStartIKERekey(t, ini, ps, negNonce(0xF0))
	inner = negIKERekeyInner(t, negNonce(0x10), uint16(crypto.DH_MODP_2048), public)
	ps.handleCreateChildSAOwned(ini, &wire.Message{Header: wire.Header{MessageID: 4}},
		inner, false, myTr, nil, log)
	if ps.pendingRekey != pending {
		t.Error("our exchange carries the higher nonce, so it must survive")
	}
	if !ini.requestOutstanding {
		t.Error("our surviving exchange released the request window it holds")
	}
	if ps.pendingIKESwap != nil {
		t.Error("the losing peer exchange still built a new IKE SA")
	}
	rtxExpectSilence(t, peerTr, myTr, remote, "peer IKE rekey that lost the collision")
}

// RFC requirement: RFC7296-2.8-2 positive -- SA lifetimes are enforced from the local
// configuration alone: newLifetimeState (rekey.go) derives soft/hard expiry from the
// locally supplied lifetime value, with no peer-negotiated input, so each peer enforces its
// own policy independently.
func TestSALifetimeTime(t *testing.T) {
	lt := newLifetimeState(3600)
	if lt == nil {
		t.Fatal("newLifetimeState returned nil for 3600s")
	}

	now := time.Now()
	if lt.softExpired(now) {
		t.Error("should not be soft-expired immediately")
	}
	if lt.hardExpired(now) {
		t.Error("should not be hard-expired immediately")
	}

	// Soft time should be before hard time (due to jitter).
	if !lt.softTime.Before(lt.hardTime) {
		t.Error("soft time should be before hard time")
	}

	// Jitter should be at most 10% of lifetime.
	maxJitter := time.Duration(3600) * time.Second / 10
	jitter := lt.hardTime.Sub(lt.softTime)
	if jitter > maxJitter {
		t.Errorf("jitter = %v, max allowed = %v", jitter, maxJitter)
	}

	// Force soft expiry.
	lt.softTime = now.Add(-1 * time.Second)
	if !lt.softExpired(now) {
		t.Error("should be soft-expired after soft time")
	}

	// Force hard expiry.
	lt.hardTime = now.Add(-1 * time.Second)
	if !lt.hardExpired(now) {
		t.Error("should be hard-expired after hard time")
	}
}

func TestSALifetimeBytes(t *testing.T) {
	lt := newLifetimeState(3600)
	lt.softBytes = 1000
	lt.byteCount = 999

	now := time.Now()
	lt.softTime = now.Add(1 * time.Hour)
	lt.hardTime = now.Add(2 * time.Hour)

	if lt.softExpired(now) {
		t.Error("should not be soft-expired at 999/1000 bytes")
	}

	lt.byteCount = 1000
	if !lt.softExpired(now) {
		t.Error("should be soft-expired at 1000/1000 bytes")
	}
}

func TestSALifetimeZero(t *testing.T) {
	lt := newLifetimeState(0)
	if lt != nil {
		t.Error("zero lifetime should return nil")
	}
}

// TestIKESARekey removed -- the local-roll rekeyIKESA it exercised
// (self-DH, which never touched the wire and silently desynced the tunnel) was
// replaced by the real CREATE_CHILD_SA IKE SA rekey exchange (spec-ipsec-13).
// Coverage moved to rekey_wire_test.go: TestInitiateIKERekey (request build +
// DH held in pendingRekey) and TestApplyIKERekeyResponse (DH completion, SKEYSEED
// re-derivation, new SA with reset message-ID counters).

// A rekeyed Child SA inherits the old SA's addresses, traffic selectors, and
// interface id (only SPIs and keys change). Now exercised through the real
// CREATE_CHILD_SA response path (applyChildRekeyResponse) rather than the removed
// local-roll rekeyChildSA.
func TestRekeyPreservesAddresses(t *testing.T) {
	sa := testSA()
	dp := &mockDP{}
	log := slogutil.DiscardLogger()
	espGroup := testESPGroup()

	old, err := createFirstChildSA(sa, espGroup, "10.0.0.1", "10.0.0.2", 42, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}

	pending := &pendingRekey{
		kind:          rekeyChild,
		localNonce:    make([]byte, nonceLen),
		newInboundSPI: 0x55667788,
		oldChild:      old,
	}
	inner := []wire.PayloadEntry{
		{Payload: espSAPayload(0x99AABBCC)},
		{Payload: &wire.PayloadNonce{NonceData: make([]byte, nonceLen)}},
	}
	newChild, err := applyChildRekeyResponse(sa, pending, inner, dp, log)
	if err != nil {
		t.Fatalf("applyChildRekeyResponse: %v", err)
	}

	if !newChild.LocalAddr.Equal(net.ParseIP("10.0.0.1")) {
		t.Errorf("local addr = %v, want 10.0.0.1", newChild.LocalAddr)
	}
	if !newChild.RemoteAddr.Equal(net.ParseIP("10.0.0.2")) {
		t.Errorf("remote addr = %v, want 10.0.0.2", newChild.RemoteAddr)
	}
	if newChild.IfID != 42 {
		t.Errorf("ifID = %d, want 42", newChild.IfID)
	}
}

// VALIDATES: a rekeyed Child SA still receives BOTH ESP forms, and still sends the form
// the NAT verdict calls for.
// PREVENTS: a tunnel narrowing back to one ESP form at its first rekey. newRekeyedChild
// copies fields from the old child by hand, so a field it forgets is silently lost and the
// failure appears only after the first rekey interval -- long after any test that watched
// the initial exchange.
func TestRekeyKeepsBothESPFormAcceptance(t *testing.T) {
	sa := testSA()
	sa.NATDetected = true
	sa.floatToNATTPort()
	dp := &mockDP{}
	log := slogutil.DiscardLogger()

	old, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 42, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}

	pending := &pendingRekey{
		kind:          rekeyChild,
		localNonce:    make([]byte, nonceLen),
		newInboundSPI: 0x55667788,
		oldChild:      old,
	}
	inner := []wire.PayloadEntry{
		{Payload: espSAPayload(0x99AABBCC)},
		{Payload: &wire.PayloadNonce{NonceData: make([]byte, nonceLen)}},
	}
	if _, err := applyChildRekeyResponse(sa, pending, inner, dp, log); err != nil {
		t.Fatalf("applyChildRekeyResponse: %v", err)
	}

	// The rekey installs a fresh pair after the original pair, so the replacement SAs are
	// the last two the dataplane was handed.
	if len(dp.sas) < 4 {
		t.Fatalf("installed %d SAs, want the original pair and the rekeyed pair", len(dp.sas))
	}
	var sawInbound bool
	for i := len(dp.sas) - 2; i < len(dp.sas); i++ {
		p := dp.sas[i]
		if p.Dst.String() != "10.0.0.1" {
			// The outbound replacement. A NAT was detected, so RFC 7296 Section 2.23
			// makes encapsulation mandatory on it.
			if !p.UDPEncap {
				t.Errorf("rekeyed outbound spi %d: no UDP encapsulation with a NAT detected", p.SPI)
			}
			continue
		}
		sawInbound = true
		if !p.AcceptBothESPForms {
			t.Errorf("rekeyed inbound spi %d: accepts one ESP form only; the tunnel narrowed at its first rekey", p.SPI)
		}
		if !p.UDPEncap {
			t.Errorf("rekeyed inbound spi %d: lost the encapsulation template a NAT-traversing tunnel needs", p.SPI)
		}
	}
	if !sawInbound {
		t.Fatal("no rekeyed inbound SA was installed; every assertion above was vacuous")
	}
}
