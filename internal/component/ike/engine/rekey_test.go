package engine

import (
	"net"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/ike/crypto"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

func TestChildSARekeyInitiator(t *testing.T) {
	sa := testSA()
	dp := &mockDP{}
	log := slogutil.DiscardLogger()
	espGroup := testESPGroup()

	old, err := createFirstChildSA(sa, espGroup, "10.0.0.1", "10.0.0.2", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	oldInSPI := old.InboundSPI

	newChild, err := rekeyChildSA(sa, old, espGroup, dp, log)
	if err != nil {
		t.Fatalf("rekeyChildSA: %v", err)
	}

	if newChild.InboundSPI == oldInSPI {
		t.Error("new child SA should have different inbound SPI")
	}
	if newChild.Keys == nil {
		t.Fatal("new child SA keys are nil")
	}
	if len(newChild.Keys.EncryptKeyI) == 0 {
		t.Error("new child initiator encrypt key is empty")
	}

	// 2 SAs from first child + 2 SAs from rekey = 4 installed
	if len(dp.sas) != 4 {
		t.Errorf("total installed SAs = %d, want 4", len(dp.sas))
	}
	// 2 SAs removed from old child
	if len(dp.removed) != 2 {
		t.Errorf("removed SAs = %d, want 2", len(dp.removed))
	}
}

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

func TestIKESARekey(t *testing.T) {
	sa := testSA()
	sa.SKKeys = &crypto.SKKeys{
		SK_d:  make([]byte, 32),
		SK_ai: make([]byte, 32),
		SK_ar: make([]byte, 32),
		SK_ei: make([]byte, 16),
		SK_er: make([]byte, 16),
		SK_pi: make([]byte, 32),
		SK_pr: make([]byte, 32),
	}
	sa.Proposal = crypto.IKEProposal{
		PRF:        crypto.PRFTransform{ID: crypto.PRF_HMAC_SHA2_256, KeyLength: 32, OutputLength: 32},
		Encryption: crypto.EncryptionTransform{ID: crypto.ENCR_AES_CBC, KeyLength: 128},
		Integrity:  crypto.IntegrityTransform{ID: crypto.AUTH_HMAC_SHA2_256_128, KeyLength: 32, TruncatedLength: 16},
		DHGroup:    crypto.DHGroupTransform{ID: crypto.DH_ECP_256},
	}
	log := slogutil.DiscardLogger()

	ikeGroup := testIKEGroup()
	newSA, err := rekeyIKESA(sa, ikeGroup, log)
	if err != nil {
		t.Fatalf("rekeyIKESA: %v", err)
	}
	if newSA == nil {
		t.Fatal("new SA is nil")
	}
	if newSA.InitiatorSPI == sa.InitiatorSPI {
		t.Error("new SA should have different initiator SPI")
	}
	if newSA.SKKeys == nil {
		t.Fatal("new SA SKKeys are nil")
	}
	if len(newSA.SKKeys.SK_d) == 0 {
		t.Error("new SA SK_d is empty")
	}
	if newSA.State != StateEstablished {
		t.Errorf("new SA state = %v, want established", newSA.State)
	}
	if newSA.PeerName != sa.PeerName {
		t.Errorf("new SA peer = %q, want %q", newSA.PeerName, sa.PeerName)
	}
	newSA.SKKeys.Clear()
}

func TestRekeyPreservesAddresses(t *testing.T) {
	sa := testSA()
	dp := &mockDP{}
	log := slogutil.DiscardLogger()
	espGroup := testESPGroup()

	old, err := createFirstChildSA(sa, espGroup, "10.0.0.1", "10.0.0.2", 42, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}

	newChild, err := rekeyChildSA(sa, old, espGroup, dp, log)
	if err != nil {
		t.Fatalf("rekeyChildSA: %v", err)
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
