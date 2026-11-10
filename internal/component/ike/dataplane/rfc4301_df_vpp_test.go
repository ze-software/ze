// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- VPP dataplane backend
// Related: outer_header.go -- DFTreatment; vpp.go -- vppTunnelFlags, the projection under test
// RFC: rfc/short/rfc4301.md -- Section 8.1 DF bit

//go:build ze_vpp

package dataplane

import (
	"errors"
	"net"
	"testing"

	"go.fd.io/govpp/binapi/tunnel_types"
)

// VALIDATES: the DF treatments VPP's ESP encrypt path can express reach the SAD entry
// InstallSA sends. Copy carries ENCAP_COPY_DF, which tunnel_encap_fixup_4o4_w_chksum
// reads; clear carries no DF flag, so tunnel_build_v4_hdr's clear bit stands.
// PREVENTS: the zero value going back to the old VPP behavior, which carried no
// ENCAP_COPY_DF and cleared DF on every tunnel whatever the operator asked.
// RFC requirement: RFC4301-8.1-1 positive -- a VPP tunnel-mode SA asking for DF copy is sent with ENCAP_COPY_DF.
// RFC requirement: RFC4301-8.1-2 positive -- a VPP tunnel-mode SA asking for DF clear is sent without ENCAP_COPY_DF and without ENCAP_SET_DF, and the ECN flags stay on both.
func TestVPPInstallSACarriesTheDFTreatment(t *testing.T) {
	for _, tc := range []struct {
		df     DFTreatment
		copyDF bool
	}{
		{DFCopy, true},
		{DFClear, false},
	} {
		b, ch := newCapturingBackend()
		p := testSAParams()
		p.DF = tc.df
		if err := b.InstallSA(p); err != nil {
			t.Fatalf("DF %s: InstallSA: %v", tc.df, err)
		}
		got := sentSAD(t, ch).Entry.Tunnel.EncapDecapFlags
		if (got&tunnel_types.TUNNEL_API_ENCAP_DECAP_FLAG_ENCAP_COPY_DF != 0) != tc.copyDF {
			t.Errorf("DF %s: EncapDecapFlags = %#x, ENCAP_COPY_DF present = %t, want %t",
				tc.df, got, !tc.copyDF, tc.copyDF)
		}
		if got&tunnel_types.TUNNEL_API_ENCAP_DECAP_FLAG_ENCAP_SET_DF != 0 {
			t.Errorf("DF %s: EncapDecapFlags = %#x carries ENCAP_SET_DF, which the ESP path ignores", tc.df, got)
		}
		if got&ecnFullFunctionality != ecnFullFunctionality {
			t.Errorf("DF %s: EncapDecapFlags = %#x lost the ECN full-functionality flags", tc.df, got)
		}
	}
}

// VALIDATES: a DF treatment VPP's IPsec path cannot apply is refused before any message
// is sent.
// PREVENTS: DF set installed as ENCAP_SET_DF, a flag only the ipip plugin reads, which
// would put a clear bit on the wire while the operator's config says set.
// RFC requirement: RFC4301-8.1-2 negative -- a VPP SA asking for DF set, or for a treatment on an IPv6 outer header, is refused with ErrSAOptionRefused, never with ErrNotSupported (which the engine would keep as a degraded Child SA), and sends nothing.
func TestVPPInstallSARefusesDFItCannotExpress(t *testing.T) {
	ipv6Outer := testSAParams()
	ipv6Outer.Src = net.ParseIP("2001:db8::1")
	ipv6Outer.Dst = net.ParseIP("2001:db8::2")
	ipv6Outer.DF = DFClear
	set := testSAParams()
	set.DF = DFSet

	for name, p := range map[string]SAParams{"set": set, "IPv6 outer": ipv6Outer} {
		b, ch := newCapturingBackend()
		if err := b.InstallSA(p); !errors.Is(err, ErrSAOptionRefused) || errors.Is(err, ErrNotSupported) {
			t.Errorf("%s: err = %v, want ErrSAOptionRefused and not ErrNotSupported", name, err)
		}
		if len(ch.sent) != 0 {
			t.Errorf("%s: %d messages were sent for a refused SA", name, len(ch.sent))
		}
	}
}
