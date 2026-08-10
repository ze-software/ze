// VALIDATES: the VPP selector and algorithm translators. vppRange turns a CIDR into
// the address RANGE an ipsec_spd_entry_v2 takes. It tags IPv4 against IPv6 and
// refuses a nil prefix. vppCryptoAlg and vppIntegAlg REFUSE an algorithm name they
// cannot name to VPP.
// PREVENTS: a policy programmed into VPP with the wrong address family, a range
// that covers one address instead of a prefix, a nil selector reaching VPP as
// 0.0.0.0, or an unknown cipher reaching VPP as a cipher the operator never chose.

//go:build ze_vpp

package dataplane

import (
	"errors"
	"net"
	"testing"

	"go.fd.io/govpp/binapi/ip_types"
	"go.fd.io/govpp/binapi/ipsec_types"
)

func TestVPPRangeIPv4(t *testing.T) {
	_, cidr, err := net.ParseCIDR("192.0.2.0/24")
	if err != nil {
		t.Fatalf("ParseCIDR: %v", err)
	}
	start, stop, err := vppRange(cidr)
	if err != nil {
		t.Fatalf("vppRange: %v", err)
	}
	if start.Af != ip_types.ADDRESS_IP4 || stop.Af != ip_types.ADDRESS_IP4 {
		t.Errorf("Af = %v/%v, want ADDRESS_IP4", start.Af, stop.Af)
	}
	if got := start.Un.GetIP4(); got != (ip_types.IP4Address{192, 0, 2, 0}) {
		t.Errorf("start = %v, want 192.0.2.0", got)
	}
	if got := stop.Un.GetIP4(); got != (ip_types.IP4Address{192, 0, 2, 255}) {
		t.Errorf("stop = %v, want 192.0.2.255", got)
	}
}

func TestVPPRangeIPv6(t *testing.T) {
	_, cidr, err := net.ParseCIDR("2001:db8::/120")
	if err != nil {
		t.Fatalf("ParseCIDR: %v", err)
	}
	start, stop, err := vppRange(cidr)
	if err != nil {
		t.Fatalf("vppRange: %v", err)
	}
	if start.Af != ip_types.ADDRESS_IP6 || stop.Af != ip_types.ADDRESS_IP6 {
		t.Errorf("Af = %v/%v, want ADDRESS_IP6", start.Af, stop.Af)
	}
	first, last := start.Un.GetIP6(), stop.Un.GetIP6()
	if first[0] != 0x20 || first[1] != 0x01 || first[15] != 0x00 {
		t.Errorf("start = %v, want 2001:db8::", first)
	}
	if last[15] != 0xff {
		t.Errorf("stop = %v, want 2001:db8::ff", last)
	}
}

// A single address reaches VPP as a range whose start and stop are equal. That is the
// /32 the IKE engine installs for a host selector (engine/child.go, ipToFullNet).
func TestVPPRangeHostPrefix(t *testing.T) {
	_, cidr, err := net.ParseCIDR("198.51.100.7/32")
	if err != nil {
		t.Fatalf("ParseCIDR: %v", err)
	}
	start, stop, err := vppRange(cidr)
	if err != nil {
		t.Fatalf("vppRange: %v", err)
	}
	if start.Un.GetIP4() != stop.Un.GetIP4() {
		t.Errorf("start %v != stop %v for a /32", start.Un.GetIP4(), stop.Un.GetIP4())
	}
}

// A nil selector is REFUSED. The zero ip_types.Address is 0.0.0.0, so a nil that
// translated silently would install a policy matching one address nobody asked for.
func TestVPPRangeNilRefused(t *testing.T) {
	if _, _, err := vppRange(nil); !errors.Is(err, ErrNotSupported) {
		t.Errorf("vppRange(nil) error = %v, want ErrNotSupported", err)
	}
}

// An unknown cipher is REFUSED. Both of these used to return a default, and this
// test used to assert that default: AES_CBC_128 for an unknown non-AEAD name and
// AES_GCM_128 for an unknown AEAD one. A default here programs a cipher the operator
// never configured, which is the fault "3des" reaching VPP as AES_CTR_128 was, so the
// old assertions pinned the defect they were written beside the fix for.
func TestVPPCryptoAlgUnknownRefused(t *testing.T) {
	if _, err := vppCryptoAlg("no-such-aead", true); !errors.Is(err, ErrNotSupported) {
		t.Errorf("unknown AEAD cipher error = %v, want ErrNotSupported", err)
	}
	if _, err := vppCryptoAlg("no-such-cbc", false); !errors.Is(err, ErrNotSupported) {
		t.Errorf("unknown cipher error = %v, want ErrNotSupported", err)
	}
	// A known value still resolves correctly.
	got, err := vppCryptoAlg("aes256", false)
	if err != nil {
		t.Fatalf("vppCryptoAlg(aes256): %v", err)
	}
	if got != ipsec_types.IPSEC_API_CRYPTO_ALG_AES_CBC_256 {
		t.Errorf("aes256 = %d, want 3", got)
	}
}

// An unknown integrity algorithm is REFUSED. SHA_256_128 used to stand as the
// default, so an SA authenticated with an algorithm the peer never negotiated and
// every packet failed its integrity check.
func TestVPPIntegAlgUnknownRefused(t *testing.T) {
	// An AEAD cipher authenticates in the cipher, so the name beside it is not read.
	got, err := vppIntegAlg("anything", true)
	if err != nil {
		t.Fatalf("vppIntegAlg(AEAD): %v", err)
	}
	if got != ipsec_types.IPSEC_API_INTEG_ALG_NONE {
		t.Errorf("AEAD integ = %d, want 0 (NONE)", got)
	}
	if _, err := vppIntegAlg("no-such-hash", false); !errors.Is(err, ErrNotSupported) {
		t.Errorf("unknown integrity algorithm error = %v, want ErrNotSupported", err)
	}
	got, err = vppIntegAlg("sha512", false)
	if err != nil {
		t.Fatalf("vppIntegAlg(sha512): %v", err)
	}
	if got != ipsec_types.IPSEC_API_INTEG_ALG_SHA_512_256 {
		t.Errorf("sha512 = %d, want 6", got)
	}
}
