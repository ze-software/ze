package engine

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

func testIKEGroup() ipsec.IKEGroup {
	return ipsec.IKEGroup{
		Name: "test-ike",
		Proposals: []ipsec.IKEProposal{
			{
				Number:     1,
				Encryption: ipsec.EncryptionAES256,
				Hash:       ipsec.HashSHA256,
				DHGroup:    14,
			},
		},
	}
}

func testPeer() ipsec.SiteToSitePeer {
	return ipsec.SiteToSitePeer{
		Name:           "test-peer",
		IKEGroup:       "test-ike",
		ESPGroup:       "test-esp",
		ConnectionType: ipsec.ConnectionInitiate,
		RemoteAddress:  "192.0.2.1",
		Auth: ipsec.AuthConfig{
			Mode: ipsec.AuthPreSharedSecret,
			PSK:  "test-secret",
		},
	}
}

func TestFSMInitiatorInit(t *testing.T) {
	peer := testPeer()
	ikeGroup := testIKEGroup()

	sa, err := newInitiatorSA("test-peer", peer, ikeGroup, ipsec.ESPGroup{})
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}

	if sa.State != StateIdle {
		t.Fatalf("expected StateIdle, got %s", sa.State)
	}
	if !sa.IsInitiator {
		t.Fatal("expected IsInitiator=true")
	}
	if sa.InitiatorSPI == [8]byte{} {
		t.Fatal("expected non-zero initiator SPI")
	}
	if len(sa.LocalNonce) != nonceLen {
		t.Fatalf("expected nonce len %d, got %d", nonceLen, len(sa.LocalNonce))
	}
	if sa.LocalDH == nil {
		t.Fatal("expected non-nil DH exchange")
	}
}

func TestFSMInitiatorSAInitRequest(t *testing.T) {
	peer := testPeer()
	ikeGroup := testIKEGroup()

	sa, err := newInitiatorSA("test-peer", peer, ikeGroup, ipsec.ESPGroup{})
	if err != nil {
		t.Fatalf("newInitiatorSA: %v", err)
	}

	msg := buildSAInitRequest(sa, ikeGroup)
	if len(msg) < 28 {
		t.Fatalf("message too short: %d bytes", len(msg))
	}

	// Verify header fields.
	if msg[18] != 34 { // ExchangeIKESAInit
		t.Fatalf("expected exchange type 34, got %d", msg[18])
	}
	if msg[19]&0x08 == 0 { // FlagInitiator
		t.Fatal("expected FlagInitiator set")
	}
}

func TestRetransmitBackoff(t *testing.T) {
	tests := []struct {
		attempt int
		minD    time.Duration
		maxD    time.Duration
	}{
		{0, retransmitBase, retransmitBase},
		{1, retransmitBase, retransmitBase},
		{2, 1 * time.Second, 1 * time.Second},
		{3, 2 * time.Second, 2 * time.Second},
		{4, 4 * time.Second, 4 * time.Second},
		{8, retransmitMax, retransmitMax},  // capped
		{20, retransmitMax, retransmitMax}, // capped
	}

	for _, tc := range tests {
		d := retransmitBackoff(tc.attempt)
		if d < tc.minD || d > tc.maxD {
			t.Errorf("attempt %d: got %v, want [%v, %v]", tc.attempt, d, tc.minD, tc.maxD)
		}
	}
}

func TestMessageIDTracking(t *testing.T) {
	sa := &SA{
		NextMsgID:     0,
		ExpectedMsgID: 0,
	}

	if sa.NextMsgID != 0 {
		t.Fatalf("expected initial NextMsgID=0, got %d", sa.NextMsgID)
	}

	sa.NextMsgID = 1
	if sa.NextMsgID != 1 {
		t.Fatalf("expected NextMsgID=1, got %d", sa.NextMsgID)
	}
}

func TestBuildIKEProposals(t *testing.T) {
	ikeGroup := testIKEGroup()
	proposals := buildIKEProposals(ikeGroup)

	if len(proposals) != 1 {
		t.Fatalf("expected 1 proposal, got %d", len(proposals))
	}

	p := proposals[0]
	if p.Number != 1 {
		t.Fatalf("expected proposal number 1, got %d", p.Number)
	}
	if p.Encryption.ID != crypto.ENCR_AES_CBC {
		t.Fatalf("expected ENCR_AES_CBC, got %d", p.Encryption.ID)
	}
	if p.PRF.ID != crypto.PRF_HMAC_SHA2_256 {
		t.Fatalf("expected PRF_HMAC_SHA2_256, got %d", p.PRF.ID)
	}
	if p.Integrity.ID != crypto.AUTH_HMAC_SHA2_256_128 {
		t.Fatalf("expected AUTH_HMAC_SHA2_256_128, got %d", p.Integrity.ID)
	}
	if p.DHGroup.ID != crypto.DH_MODP_2048 {
		t.Fatalf("expected DH_MODP_2048, got %d", p.DHGroup.ID)
	}
}

func TestNoProposalChosenNotify(t *testing.T) {
	// Build two incompatible proposal sets.
	local := []crypto.IKEProposal{{
		Encryption: crypto.EncryptionTransform{ID: crypto.ENCR_AES_CBC, KeyLength: 256},
		PRF:        crypto.PRFTransform{ID: crypto.PRF_HMAC_SHA2_256},
		Integrity:  crypto.IntegrityTransform{ID: crypto.AUTH_HMAC_SHA2_256_128},
		DHGroup:    crypto.DHGroupTransform{ID: crypto.DH_MODP_2048},
	}}
	remote := []crypto.IKEProposal{{
		Encryption: crypto.EncryptionTransform{ID: crypto.ENCR_AES_GCM_16, KeyLength: 128},
		PRF:        crypto.PRFTransform{ID: crypto.PRF_HMAC_SHA2_512},
		Integrity:  crypto.IntegrityTransform{ID: crypto.AUTH_HMAC_SHA2_512_256},
		DHGroup:    crypto.DHGroupTransform{ID: crypto.DH_ECP_256},
	}}

	_, err := crypto.NegotiateIKE(remote, local)
	if err == nil {
		t.Fatal("expected NO_PROPOSAL_CHOSEN error")
	}
}

func TestParseHashAlgoNotify(t *testing.T) {
	// SHA2-256 (2), SHA2-384 (3), SHA2-512 (4).
	data := []byte{0, 2, 0, 3, 0, 4}
	algos := parseHashAlgoNotify(data)
	if len(algos) != 3 {
		t.Fatalf("expected 3 algos, got %d", len(algos))
	}
	if algos[0] != 2 || algos[1] != 3 || algos[2] != 4 {
		t.Fatalf("unexpected algos: %v", algos)
	}
}

func TestParseHashAlgoNotifyShort(t *testing.T) {
	algos := parseHashAlgoNotify([]byte{0})
	if algos != nil {
		t.Fatalf("expected nil for short data, got %v", algos)
	}
}
