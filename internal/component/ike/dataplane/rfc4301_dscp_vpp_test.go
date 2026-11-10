// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- VPP dataplane backend
// Related: vpp.go -- vppTunnelFlags and vppTunnelDSCP, the projection under test
// RFC: rfc/short/rfc4301.md -- Section 5.1.2.1 tunnel header DSCP

//go:build ze_vpp

package dataplane

import (
	"testing"

	"go.fd.io/govpp/binapi/tunnel_types"
)

// VALIDATES: copy reaches VPP as ENCAP_COPY_DSCP, and a fixed value as the tunnel's
// Dscp with no copy flag, so tunnel_build_v4_hdr writes it into every outer header.
// PREVENTS: the old VPP SA, which carried neither and wrote DSCP 0 on every tunnel.
// RFC requirement: RFC4301-5.1.2.1-1 positive -- a VPP tunnel-mode SA copying the outer DSCP is sent with ENCAP_COPY_DSCP, and one mapping it to 46 is sent with Dscp 46 and without ENCAP_COPY_DSCP.
func TestVPPInstallSACarriesTheOuterDSCP(t *testing.T) {
	ef, err := FixedOuterDSCP(46)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		dscp     OuterDSCP
		copyDSCP bool
		value    uint8
	}{{OuterDSCPCopy, true, 0}, {ef, false, 46}} {
		b, ch := newCapturingBackend()
		p := testSAParams()
		p.OuterDSCP = tc.dscp
		if err := b.InstallSA(p); err != nil {
			t.Fatalf("DSCP %s: InstallSA: %v", tc.dscp, err)
		}
		tunnel := sentSAD(t, ch).Entry.Tunnel
		if (tunnel.EncapDecapFlags&tunnel_types.TUNNEL_API_ENCAP_DECAP_FLAG_ENCAP_COPY_DSCP != 0) != tc.copyDSCP {
			t.Errorf("DSCP %s: EncapDecapFlags = %#x, want ENCAP_COPY_DSCP %t", tc.dscp, tunnel.EncapDecapFlags, tc.copyDSCP)
		}
		if uint8(tunnel.Dscp) != tc.value {
			t.Errorf("DSCP %s: Dscp = %d, want %d", tc.dscp, tunnel.Dscp, tc.value)
		}
	}
}
