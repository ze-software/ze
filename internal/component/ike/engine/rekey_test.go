package engine

import (
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// test-relax: TestChildSARekeyInitiator removed -- the local-roll rekeyChildSA it
// exercised was replaced by the real CREATE_CHILD_SA wire exchange (spec-ipsec-13).
// Coverage moved to rekey_wire_test.go: TestInitiateChildRekey (request build),
// TestApplyChildRekeyResponse (key derive + install, replaces the SPI/key asserts),
// TestRespondChildRekey (responder install + reply).

// RFC requirement: RFC7296-2.8-1 positive -- on a simultaneous rekey, resolveRekeyCollision
// (rekey.go:418) declares the exchange with the LOWER nonce the winner (§2.8.1), so the peer
// whose nonce compares lower keeps its exchange.
// RFC requirement: RFC7296-2.8-1 negative -- the higher nonce loses and two equal nonces do
// NOT declare the local side the winner, so a collision never resolves to both sides winning.
func TestRekeyCollision(t *testing.T) {
	localNonce := []byte{0x01, 0x02, 0x03}
	remoteNonce := []byte{0x04, 0x05, 0x06}

	if !resolveRekeyCollision(localNonce, remoteNonce) {
		t.Error("local nonce is lower, should win")
	}
	if resolveRekeyCollision(remoteNonce, localNonce) {
		t.Error("local nonce is higher, should lose")
	}

	sameNonce := []byte{0x01, 0x02, 0x03}
	if resolveRekeyCollision(sameNonce, sameNonce) {
		t.Error("equal nonces should not declare local winner")
	}
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

// test-relax: TestIKESARekey removed -- the local-roll rekeyIKESA it exercised
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
