// VALIDATES: RFC 4301 Section 4.2, "A compliant implementation MUST NOT allow
// instantiation of an ESP SA that employs both NULL encryption and no integrity
// algorithm", on the ze_vpp backend, the second path by which Ze instantiates an ESP
// SA (vppBackend.InstallSA). The XFRM path is proven in rfc4301_esp_null_linux_test.go.
// PREVENTS: an ESP SA with NULL encryption and no integrity transform reaching VPP's
// SAD through ipsec_sad_entry_add_del_v3.

//go:build ze_vpp

package dataplane

import (
	"errors"
	"testing"
)

// TestRFC4301VPPESPNullEncryptionWithoutIntegrityIsRefused proves the VPP backend
// refuses an ESP SA that employs NULL encryption and no integrity algorithm, and sends
// VPP no SAD entry for it. VPP's cipher mapping (vppCryptoAlg) names no NULL cipher, so
// this backend refuses NULL encryption with or without integrity; the case with an
// integrity algorithm is included so the refusal is shown to come from the cipher, not
// from the empty integrity name alone. Method: a control install of the well-formed
// test SA sends one message, then each NULL variant goes through InstallSA on a fresh
// capturing backend.
func TestRFC4301VPPESPNullEncryptionWithoutIntegrityIsRefused(t *testing.T) {
	// RFC requirement: RFC4301-4.2-1 negative -- on the ze_vpp backend, InstallSA of an ESP SA with NULL encryption and no integrity algorithm ("" or "none") returns ErrNotSupported and sends VPP no SAD entry; NULL encryption with sha256 is refused the same way, because vppCryptoAlg names no NULL cipher.
	control, controlCh := newCapturingBackend()
	if err := control.InstallSA(testSAParams()); err != nil {
		t.Fatalf("control: InstallSA of the well-formed test SA: %v", err)
	}
	if len(controlCh.sent) != 1 {
		t.Fatalf("control: sent %d messages for the well-formed test SA, want 1", len(controlCh.sent))
	}

	for _, auth := range []string{"", "none", "sha256"} {
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
		if _, cipherErr := vppCryptoAlg(p.EncAlgo, p.IsAEAD); cipherErr == nil {
			t.Errorf("vppCryptoAlg(%q) named a VPP cipher: NULL encryption would reach the SAD", p.EncAlgo)
		}
	}
}
