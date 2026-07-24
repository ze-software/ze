// VALIDATES: the VPP address/algorithm translators — vppAddress tags IPv4 vs
// IPv6 (Af 0/1) and copies the right byte count, vppPrefix maps a CIDR to
// address+length (and nil to the zero prefix), and vppCryptoAlg/vppIntegAlg fall
// back to their default enum IDs for unrecognized algorithm names.
// PREVENTS: an SA being programmed into VPP with the wrong address family, a
// truncated prefix length, or an unknown cipher silently mapping to no algorithm.

//go:build ze_vpp

package dataplane

import (
	"net"
	"testing"
)

func TestVPPAddress(t *testing.T) {
	v4 := vppAddress(net.ParseIP("192.0.2.1"))
	if v4.Af != 0 {
		t.Errorf("v4 Af = %d, want 0", v4.Af)
	}
	if v4.Un[0] != 192 || v4.Un[1] != 0 || v4.Un[2] != 2 || v4.Un[3] != 1 {
		t.Errorf("v4 Un[:4] = %v, want [192 0 2 1]", v4.Un[:4])
	}

	v6 := vppAddress(net.ParseIP("2001:db8::1"))
	if v6.Af != 1 {
		t.Errorf("v6 Af = %d, want 1", v6.Af)
	}
	if v6.Un[0] != 0x20 || v6.Un[1] != 0x01 || v6.Un[15] != 0x01 {
		t.Errorf("v6 Un = %v, want a 16-byte 2001:db8::1", v6.Un)
	}
}

func TestVPPPrefix(t *testing.T) {
	if got := vppPrefix(nil); got != (vppPfx{}) {
		t.Errorf("vppPrefix(nil) = %+v, want zero", got)
	}

	_, cidr, err := net.ParseCIDR("192.0.2.0/24")
	if err != nil {
		t.Fatalf("ParseCIDR: %v", err)
	}
	pfx := vppPrefix(cidr)
	if pfx.Len != 24 {
		t.Errorf("Len = %d, want 24", pfx.Len)
	}
	if pfx.Address.Af != 0 {
		t.Errorf("Address.Af = %d, want 0 (v4)", pfx.Address.Af)
	}
}

func TestVPPCryptoAlgDefaults(t *testing.T) {
	if got := vppCryptoAlg("no-such-aead", true); got != 7 {
		t.Errorf("AEAD default = %d, want 7 (AES_GCM_128)", got)
	}
	if got := vppCryptoAlg("no-such-cbc", false); got != 1 {
		t.Errorf("non-AEAD default = %d, want 1 (AES_CBC_128)", got)
	}
	// A known value still resolves correctly.
	if got := vppCryptoAlg("aes256", false); got != 3 {
		t.Errorf("aes256 = %d, want 3", got)
	}
}

func TestVPPIntegAlgDefaults(t *testing.T) {
	if got := vppIntegAlg("anything", true); got != 0 {
		t.Errorf("AEAD integ = %d, want 0 (NONE)", got)
	}
	if got := vppIntegAlg("no-such-hash", false); got != 4 {
		t.Errorf("non-AEAD default = %d, want 4 (SHA_256_128)", got)
	}
	if got := vppIntegAlg("sha512", false); got != 6 {
		t.Errorf("sha512 = %d, want 6", got)
	}
}
