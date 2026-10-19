// VALIDATES: RFC 4301 Section 4.2, "A compliant implementation MUST NOT allow
// instantiation of an ESP SA that employs both NULL encryption and no integrity
// algorithm", on the ze_vpp backend, the second path by which Ze instantiates an ESP
// SA (vppBackend.InstallSA). The XFRM path is proven in rfc4301_esp_null_linux_test.go.
// PREVENTS: an ESP SA with NULL encryption and no integrity transform reaching VPP's
// SAD through ipsec_sad_entry_add_del_v3, and integrity-only ESP (NULL encryption
// with an integrity algorithm, RFC 4303 Section 1) being refused on this backend.

//go:build ze_vpp

package dataplane

import (
	"errors"
	"testing"

	"go.fd.io/govpp/binapi/ipsec"
	"go.fd.io/govpp/binapi/ipsec_types"
)

// TestRFC4301VPPESPNullEncryptionWithoutIntegrityIsRefused proves the VPP backend
// refuses an ESP SA that employs NULL encryption and no integrity algorithm, and sends
// VPP no SAD entry for it. vppCryptoAlg names the null cipher (integrity-only ESP, RFC
// 4303 Section 1), so the refusal comes from vppIntegAlg, which names no VPP algorithm
// for an empty or "none" integrity beside a non-AEAD cipher. Method: a control install
// of the well-formed test SA sends one message, then each NULL variant without
// integrity goes through InstallSA on a fresh capturing backend.
func TestRFC4301VPPESPNullEncryptionWithoutIntegrityIsRefused(t *testing.T) {
	// RFC requirement: RFC4301-4.2-1 negative -- on the ze_vpp backend, InstallSA of an ESP SA with NULL encryption and no integrity algorithm ("" or "none") returns ErrNotSupported and sends VPP no SAD entry.
	control, controlCh := newCapturingBackend()
	if err := control.InstallSA(testSAParams()); err != nil {
		t.Fatalf("control: InstallSA of the well-formed test SA: %v", err)
	}
	if len(controlCh.sent) != 1 {
		t.Fatalf("control: sent %d messages for the well-formed test SA, want 1", len(controlCh.sent))
	}

	for _, auth := range []string{"", "none"} {
		b, ch := newCapturingBackend()
		p := testSAParams()
		p.EncAlgo = "null"
		p.EncKey = nil
		p.AuthAlgo = auth
		err := b.InstallSA(p)
		if !errors.Is(err, ErrNotSupported) {
			t.Errorf("InstallSA(null encryption, integrity %q) error = %v, want ErrNotSupported", auth, err)
		}
		if len(ch.sent) != 0 {
			t.Errorf("InstallSA(null encryption, integrity %q) sent %d messages to VPP, want none", auth, len(ch.sent))
		}
	}
}

// TestRFC4303VPPIntegrityOnlyESPIsInstalled proves the VPP backend installs an
// integrity-only ESP SA: NULL encryption beside sha256 reaches VPP's SAD as one
// ipsec_sad_entry_add_del_v3 naming VPP's null cipher (IPSEC_API_CRYPTO_ALG_NONE) and
// HMAC-SHA-256-128. Method: InstallSA on a capturing backend, then the one captured
// message is read back.
func TestRFC4303VPPIntegrityOnlyESPIsInstalled(t *testing.T) {
	// RFC requirement: RFC4303-1-2 positive -- on the ze_vpp backend, InstallSA of an ESP SA with NULL encryption and sha256 integrity sends VPP one SAD entry whose crypto algorithm is IPSEC_API_CRYPTO_ALG_NONE and whose integrity algorithm is IPSEC_API_INTEG_ALG_SHA_256_128.
	b, ch := newCapturingBackend()
	p := testSAParams()
	p.EncAlgo = "null"
	p.EncKey = nil
	p.AuthAlgo = "sha256"
	if err := b.InstallSA(p); err != nil {
		t.Fatalf("InstallSA(null encryption, integrity sha256): %v", err)
	}
	if len(ch.sent) != 1 {
		t.Fatalf("InstallSA(null encryption, integrity sha256) sent %d messages to VPP, want 1", len(ch.sent))
	}
	req, ok := ch.sent[0].(*ipsec.IpsecSadEntryAddDelV3)
	if !ok {
		t.Fatalf("sent %T, want *ipsec.IpsecSadEntryAddDelV3", ch.sent[0])
	}
	if req.Entry.CryptoAlgorithm != ipsec_types.IPSEC_API_CRYPTO_ALG_NONE {
		t.Errorf("CryptoAlgorithm = %v, want NONE (the null cipher)", req.Entry.CryptoAlgorithm)
	}
	if req.Entry.IntegrityAlgorithm != ipsec_types.IPSEC_API_INTEG_ALG_SHA_256_128 {
		t.Errorf("IntegrityAlgorithm = %v, want SHA_256_128", req.Entry.IntegrityAlgorithm)
	}
}
