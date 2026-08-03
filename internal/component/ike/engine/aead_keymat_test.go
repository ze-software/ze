package engine

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// failingDP refuses every state install with a fixed error, which reproduces a
// platform whose kernel rejects the ESP state.
type failingDP struct {
	mockDP
	err error
}

func (f *failingDP) InstallSA(dataplane.SAParams) error { return f.err }

// gcmESPGroup returns an ESP group that proposes AES-GCM-256 and names a hash beside
// it. parseESPProposal now refuses that spelling, so no operator config reaches the
// engine in this shape. A Go struct still expresses it, and the engine must key the
// Child SA as though the hash were absent. RFC 7296 Section 3.3 makes the integrity
// transform NONE for an AEAD cipher.
func gcmESPGroup() ipsec.ESPGroup {
	return ipsec.ESPGroup{
		Name:     "esp-gcm",
		Lifetime: 3600,
		Proposals: []ipsec.ESPProposal{{
			Number:     1,
			Encryption: ipsec.EncryptionAES256GCM,
			Hash:       ipsec.HashSHA256,
		}},
	}
}

// VALIDATES: an AEAD Child SA reads EncryptKeyI at KEYMAT offset 0 and EncryptKeyR at
// offset 36, whatever hash the ESP proposal names. RFC 4106 Section 8.1 gives 32
// octets of AES key plus four octets of salt, and RFC 7296 Section 3.3 makes the
// integrity transform NONE, so KEYMAT is 72 octets.
// PREVENTS: the silent half-working tunnel. A hash beside an AEAD cipher once added
// two 32 octet integrity keys, so ze sliced EncryptKeyR at offset 68 while strongSwan
// read it at 36. The handshake completed and one direction never decrypted.
func TestChildSAAEADKeyOffsetsIgnoreConfiguredHash(t *testing.T) {
	sa := testSA()
	for i := range sa.LocalNonce {
		sa.LocalNonce[i] = byte(i)
	}
	for i := range sa.RemoteNonce {
		sa.RemoteNonce[i] = byte(i + 100)
	}
	dp := &mockDP{}
	log := slogutil.DiscardLogger()

	child, err := createFirstChildSA(sa, gcmESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	defer child.Clear()

	const encKeyLen = 36
	seed := append(append([]byte(nil), sa.initiatorNonce()...), sa.responderNonce()...)
	keymat, err := crypto.PRFPlus(crypto.PRF_HMAC_SHA2_256, sa.SKKeys.SK_d, seed, 2*encKeyLen)
	if err != nil {
		t.Fatalf("PRFPlus: %v", err)
	}

	if !bytes.Equal(child.Keys.EncryptKeyI, keymat[0:encKeyLen]) {
		t.Errorf("EncryptKeyI = %x, want KEYMAT[0:36] = %x",
			child.Keys.EncryptKeyI, keymat[0:encKeyLen])
	}
	if !bytes.Equal(child.Keys.EncryptKeyR, keymat[encKeyLen:2*encKeyLen]) {
		t.Errorf("EncryptKeyR = %x, want KEYMAT[36:72] = %x",
			child.Keys.EncryptKeyR, keymat[encKeyLen:2*encKeyLen])
	}
	if len(child.Keys.IntegKeyI) != 0 || len(child.Keys.IntegKeyR) != 0 {
		t.Errorf("integrity keys are %d and %d octets, want 0 for an AEAD cipher",
			len(child.Keys.IntegKeyI), len(child.Keys.IntegKeyR))
	}

	// SAParams.EncKey carries the cipher key followed by the salt for an AEAD cipher,
	// which is the contract its doc comment states. Each installed SA therefore holds
	// one of the two derived keys unchanged, salt included.
	if len(dp.sas) != 2 {
		t.Fatalf("installed %d SAs, want 2", len(dp.sas))
	}
	for _, installed := range dp.sas {
		if !bytes.Equal(installed.EncKey, child.Keys.EncryptKeyI) &&
			!bytes.Equal(installed.EncKey, child.Keys.EncryptKeyR) {
			t.Errorf("SPI %d: EncKey = %x, want one of the two derived keys unchanged",
				installed.SPI, installed.EncKey)
		}
		if len(installed.AuthKey) != 0 {
			t.Errorf("SPI %d: AuthKey is %d octets, want 0 for an AEAD cipher",
				installed.SPI, len(installed.AuthKey))
		}
	}
}

// VALIDATES: an AES-GCM-256 Child SA installs a 36 octet key in each direction. RFC
// 4106 Section 8.1 requires 32 octets of AES key and four octets of salt.
// PREVENTS: the dataplane install that the Linux kernel refuses with EPROTONOSUPPORT,
// and the interop break where strongSwan reads the responder key at offset 36.
func TestChildSAAEADInstallsSaltedKeys(t *testing.T) {
	sa := testSA()
	dp := &mockDP{}
	log := slogutil.DiscardLogger()

	child, err := createFirstChildSA(sa, gcmESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	defer child.Clear()

	if len(child.Keys.EncryptKeyI) != 36 {
		t.Errorf("EncryptKeyI length = %d, want 36 (256-bit AES key plus 4-byte salt)",
			len(child.Keys.EncryptKeyI))
	}
	if len(child.Keys.EncryptKeyR) != 36 {
		t.Errorf("EncryptKeyR length = %d, want 36", len(child.Keys.EncryptKeyR))
	}

	if len(dp.sas) != 2 {
		t.Fatalf("installed %d SAs, want 2", len(dp.sas))
	}
	for _, installed := range dp.sas {
		if !installed.IsAEAD {
			t.Errorf("SPI %d: IsAEAD = false, want true for aes256gcm", installed.SPI)
		}
		if len(installed.EncKey) != 36 {
			t.Errorf("SPI %d: EncKey length = %d, want 36", installed.SPI, len(installed.EncKey))
		}
	}
}

// gcmWireProposal builds a wire proposal that offers AES-GCM-256 for the given
// protocol, exactly as a peer writes one on the wire.
func gcmWireProposal(protocolID uint8) wire.Proposal {
	return wire.Proposal{
		Number:     1,
		ProtocolID: protocolID,
		Transforms: []wire.Transform{{
			Type:  wire.TransformTypeENCR,
			ID:    uint16(crypto.ENCR_AES_GCM_16),
			Attrs: []wire.TransformAttr{{Type: wire.AttrTypeKeyLength, Value: 256}},
		}},
	}
}

// VALIDATES: the readers of a wire SA payload fill IsAEAD from the Transform ID they
// read. wireProposalsToESP and wireProposalsToIKE both go through
// crypto.NewEncryptionTransform.
// PREVENTS: the zero-value trap of ai/rules/evidence.md. Both readers once
// set the ID and the key length alone, so every proposal they built reported "not
// AEAD" whatever cipher the peer named.
func TestWireProposalReadersFillAEAD(t *testing.T) {
	espProps := wireProposalsToESP([]wire.Proposal{gcmWireProposal(wire.ProtocolESP)})
	if len(espProps) != 1 {
		t.Fatalf("wireProposalsToESP returned %d proposals, want 1", len(espProps))
	}
	if !espProps[0].Encryption.IsAEAD {
		t.Error("ESP: IsAEAD = false, want true for ENCR_AES_GCM_16")
	}
	if espProps[0].Encryption.KeyLength != 256 {
		t.Errorf("ESP: KeyLength = %d, want 256", espProps[0].Encryption.KeyLength)
	}

	ikeProps := wireProposalsToIKE([]wire.Proposal{gcmWireProposal(wire.ProtocolIKE)})
	if len(ikeProps) != 1 {
		t.Fatalf("wireProposalsToIKE returned %d proposals, want 1", len(ikeProps))
	}
	if !ikeProps[0].Encryption.IsAEAD {
		t.Error("IKE: IsAEAD = false, want true for ENCR_AES_GCM_16")
	}
	if ikeProps[0].Encryption.KeyLength != 256 {
		t.Errorf("IKE: KeyLength = %d, want 256", ikeProps[0].Encryption.KeyLength)
	}
}

// VALIDATES: a non-AEAD cipher read off the wire reports IsAEAD false.
// PREVENTS: a predicate that answers true for every cipher, which would over-key
// AES-CBC by four octets.
func TestWireProposalReaderCBCIsNotAEAD(t *testing.T) {
	prop := wire.Proposal{
		Number:     1,
		ProtocolID: wire.ProtocolESP,
		Transforms: []wire.Transform{{
			Type:  wire.TransformTypeENCR,
			ID:    uint16(crypto.ENCR_AES_CBC),
			Attrs: []wire.TransformAttr{{Type: wire.AttrTypeKeyLength, Value: 256}},
		}},
	}
	props := wireProposalsToESP([]wire.Proposal{prop})
	if len(props) != 1 {
		t.Fatalf("wireProposalsToESP returned %d proposals, want 1", len(props))
	}
	if props[0].Encryption.IsAEAD {
		t.Error("IsAEAD = true, want false for ENCR_AES_CBC")
	}
}

// VALIDATES: an install failure the platform reports as unsupported leaves the Child
// SA marked as carrying no ESP, so the tunnel reports degraded rather than up.
// PREVENTS: the silent dataplane loss where ze_ipsec_tunnel_up reads 1 for a tunnel
// that forwards no encrypted traffic.
func TestChildSADataplaneMissingIsRecorded(t *testing.T) {
	sa := testSA()
	dp := &failingDP{err: dataplane.ErrNotSupported}
	log := slogutil.DiscardLogger()

	child, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	defer child.Clear()

	if child.ESPInstalled {
		t.Error("ESPInstalled = true, want false after an unsupported install")
	}
}

// VALIDATES: a successful install marks the Child SA as carrying ESP.
// PREVENTS: a degraded reading for a tunnel whose dataplane is working.
func TestChildSADataplaneInstalledIsRecorded(t *testing.T) {
	sa := testSA()
	dp := &mockDP{}
	log := slogutil.DiscardLogger()

	child, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", 1, dp, log)
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}
	defer child.Clear()

	if !child.ESPInstalled {
		t.Error("ESPInstalled = false, want true after a successful install")
	}
}
