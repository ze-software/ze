// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- XFRM netlink backend
// Related: outer_header.go -- DFTreatment; xfrm_linux.go -- xfrmNoPMTUDisc, the projection under test
// RFC: rfc/short/rfc4301.md -- Section 8.1 DF bit

//go:build linux

package dataplane

import (
	"errors"
	"net"
	"testing"
)

// dfTunnelSA is an outbound IPv4 tunnel-mode SA, the one shape RFC 4301 Section 8.1
// gives a DF treatment to: "This applies to SAs where both inner and outer headers are
// IPv4."
func dfTunnelSA(df DFTreatment) SAParams {
	return SAParams{
		SPI:      0xC0FFEE81,
		Src:      net.ParseIP("10.0.0.1"),
		Dst:      net.ParseIP("10.0.0.2"),
		Dir:      SADirOut,
		Proto:    ProtoESP,
		Mode:     ModeTunnel,
		ReqID:    7,
		EncAlgo:  "aes256",
		EncKey:   make([]byte, 32),
		AuthAlgo: "sha256",
		AuthKey:  make([]byte, 32),
		DF:       df,
	}
}

// VALIDATES: the two DF treatments Linux XFRM can express reach the state ze hands the
// kernel. Copy leaves XFRM_STATE_NOPMTUDISC clear, so xfrm4_tunnel_encap_add copies the
// inner DF bit; clear sets it, so the outer DF bit is written clear.
// PREVENTS: a configured clear being installed as the kernel default, which copies a
// set inner bit and leaves the tunnel packet unfragmentable on the path.
// RFC requirement: RFC4301-8.1-1 positive -- an outbound IPv4 tunnel-mode SA asking for DF copy is built without XFRM_STATE_NOPMTUDISC, the state under which the kernel copies the inner DF bit.
// RFC requirement: RFC4301-8.1-1 negative -- an outbound IPv4 tunnel-mode SA asking for DF clear is built with XFRM_STATE_NOPMTUDISC rather than as a copy.
// RFC requirement: RFC4301-8.1-2 positive -- an outbound IPv4 tunnel-mode SA asking for DF clear is built with XFRM_STATE_NOPMTUDISC, and one asking for copy without it.
func TestXFRMStateCarriesTheDFTreatment(t *testing.T) {
	for _, tc := range []struct {
		df         DFTreatment
		noPMTUDisc bool
	}{
		{DFCopy, false},
		{DFClear, true},
	} {
		state, err := xfrmStateFromParams(dfTunnelSA(tc.df))
		if err != nil {
			t.Fatalf("DF %s: xfrmStateFromParams: %v", tc.df, err)
		}
		if state.NoPMTUDisc != tc.noPMTUDisc {
			t.Errorf("DF %s: NoPMTUDisc = %t, want %t", tc.df, state.NoPMTUDisc, tc.noPMTUDisc)
		}
	}
}

// VALIDATES: every DF request the kernel cannot apply exactly is refused, rather than
// installed as a state that writes some other bit.
// PREVENTS: DF set installed as a copy (a clear inner bit then reaches the wire clear),
// and a treatment on an SA that builds no IPv4 outer header being accepted and ignored.
// RFC requirement: RFC4301-8.1-2 negative -- a DF treatment Linux XFRM cannot express (set), or one asked of a transport-mode, inbound or IPv6-outer SA, is refused with ErrSAOptionRefused, never with ErrNotSupported (which the engine would keep as a degraded Child SA), instead of installed.
func TestXFRMStateRefusesDFItCannotExpress(t *testing.T) {
	transport := dfTunnelSA(DFClear)
	transport.Mode = ModeTransport
	inbound := dfTunnelSA(DFClear)
	inbound.Dir = SADirIn
	ipv6Outer := dfTunnelSA(DFClear)
	ipv6Outer.Src = net.ParseIP("2001:db8::1")
	ipv6Outer.Dst = net.ParseIP("2001:db8::2")

	for name, p := range map[string]SAParams{
		"set":          dfTunnelSA(DFSet),
		"out of range": dfTunnelSA(DFSet + 1),
		"transport":    transport,
		"inbound":      inbound,
		"IPv6 outer":   ipv6Outer,
	} {
		state, err := xfrmStateFromParams(p)
		if !errors.Is(err, ErrSAOptionRefused) || errors.Is(err, ErrNotSupported) {
			t.Errorf("%s: err = %v, want ErrSAOptionRefused and not ErrNotSupported", name, err)
		}
		if state != nil {
			t.Errorf("%s: a state was built for a refused treatment", name)
		}
	}
}
