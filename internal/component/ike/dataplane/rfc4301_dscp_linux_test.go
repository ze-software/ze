// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- XFRM netlink backend
// Related: outer_header.go -- OuterDSCP; xfrm_linux.go -- xfrmDontEncapDSCP, the projection under test
// RFC: rfc/short/rfc4301.md -- Section 5.1.2.1 tunnel header DSCP

//go:build linux

package dataplane

import (
	"errors"
	"testing"
)

// VALIDATES: the outer DSCP treatments Linux XFRM can write reach the state: copy
// leaves XFRM_SA_XFLAG_DONT_ENCAP_DSCP clear, a fixed 0 sets it.
// PREVENTS: a configured mapping to 0 being installed as a copy, which carries the
// inner class into a domain where it is not appropriate.
// RFC requirement: RFC4301-5.1.2.1-1 positive -- an outbound tunnel-mode SA mapping the outer DSCP to 0 is built with DontEncapDSCP, and one copying it without.
func TestXFRMStateCarriesTheOuterDSCP(t *testing.T) {
	zero, err := FixedOuterDSCP(0)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		dscp OuterDSCP
		want bool
	}{{OuterDSCPCopy, false}, {zero, true}} {
		p := dfTunnelSA(DFCopy)
		p.OuterDSCP = tc.dscp
		state, err := xfrmStateFromParams(p)
		if err != nil {
			t.Fatalf("DSCP %s: xfrmStateFromParams: %v", tc.dscp, err)
		}
		if state.DontEncapDSCP != tc.want {
			t.Errorf("DSCP %s: DontEncapDSCP = %t, want %t", tc.dscp, state.DontEncapDSCP, tc.want)
		}
	}
}

// VALIDATES: an outer DSCP the kernel cannot write is refused, and so is a mapping on
// an SA that builds no outer header.
// PREVENTS: dscp-map 46 installed as copy or as 0 while the config says 46.
// RFC requirement: RFC4301-5.1.2.1-1 negative -- a fixed outer DSCP other than 0, or any fixed value on a transport-mode or inbound SA, is refused on Linux XFRM with ErrSAOptionRefused and never with ErrNotSupported, which the engine would keep as a degraded Child SA.
func TestXFRMStateRefusesOuterDSCPItCannotWrite(t *testing.T) {
	ef, err := FixedOuterDSCP(46)
	if err != nil {
		t.Fatal(err)
	}
	zero, err := FixedOuterDSCP(0)
	if err != nil {
		t.Fatal(err)
	}
	transport := dfTunnelSA(DFCopy)
	transport.Mode, transport.OuterDSCP = ModeTransport, zero
	inbound := dfTunnelSA(DFCopy)
	inbound.Dir, inbound.OuterDSCP = SADirIn, zero
	fixed := dfTunnelSA(DFCopy)
	fixed.OuterDSCP = ef
	for name, p := range map[string]SAParams{"46": fixed, "transport": transport, "inbound": inbound} {
		if _, err := xfrmStateFromParams(p); !errors.Is(err, ErrSAOptionRefused) || errors.Is(err, ErrNotSupported) {
			t.Errorf("%s: err = %v, want ErrSAOptionRefused and not ErrNotSupported", name, err)
		}
	}
	if _, err := FixedOuterDSCP(64); err == nil {
		t.Error("FixedOuterDSCP(64) accepted a value past the six DSCP bits")
	}
}

// VALIDATES: Linux XFRM answers config verify from the predicates its install path
// uses: a fixed outer DSCP other than 0 and DF set are refused with ErrSAOptionRefused,
// copy, a fixed 0, DF clear and the ICMP error processing are accepted.
// PREVENTS: a commit accepting dscp-map 46 on Linux that the install then refuses after
// IKE negotiated the Child SA.
// RFC requirement: RFC4301-5.1.2.1-1 negative -- Linux XFRM's config-verify answer refuses a fixed outer DSCP other than 0 with ErrSAOptionRefused, and accepts copy and a fixed 0.
func TestXFRMSAOptionsSupportedRefusesOuterDSCPItCannotWrite(t *testing.T) {
	b := &xfrmBackend{}
	ef, err := FixedOuterDSCP(46)
	if err != nil {
		t.Fatal(err)
	}
	zero, err := FixedOuterDSCP(0)
	if err != nil {
		t.Fatal(err)
	}
	err = b.SAOptionsSupported(SAOptions{OuterDSCP: ef, ICMPErrorCheck: true})
	if !errors.Is(err, ErrSAOptionRefused) || errors.Is(err, ErrNotSupported) {
		t.Errorf("dscp-map 46: err = %v, want ErrSAOptionRefused and not ErrNotSupported", err)
	}
	if err := b.SAOptionsSupported(SAOptions{DF: DFSet}); !errors.Is(err, ErrSAOptionRefused) {
		t.Errorf("df-bit set: err = %v, want ErrSAOptionRefused", err)
	}
	for name, o := range map[string]SAOptions{
		"defaults":               {ICMPErrorCheck: true},
		"dscp 0":                 {OuterDSCP: zero, ICMPErrorCheck: true},
		"df clear":               {DF: DFClear, ICMPErrorCheck: true},
		"icmp-error-check false": {},
	} {
		if err := b.SAOptionsSupported(o); err != nil {
			t.Errorf("%s: refused an option Linux XFRM applies: %v", name, err)
		}
	}
}
