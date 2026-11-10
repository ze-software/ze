// VALIDATES: the VPP backend refuses the RFC 4301 Section 6.2 ICMP error processing it
// cannot perform, on the SA and on the policy, and names the leaf that turns it off.
// PREVENTS: VPP reporting a Child SA installed with the payload check while its decrypt
// path forwards every ICMP error it decapsulates without reading the payload.

//go:build ze_vpp

package dataplane

import (
	"errors"
	"strings"
	"testing"
)

func TestVPPRefusesICMPErrorProcessing(t *testing.T) {
	sa := SAParams{Dir: SADirIn, Mode: ModeTunnel, ICMPErrorCheck: true}
	err := vppUnsupportedSA(sa)
	if !errors.Is(err, ErrSAOptionRefused) || errors.Is(err, ErrNotSupported) {
		t.Fatalf("vpp accepted an SA asking for the ICMP error payload check: %v", err)
	}
	if !strings.Contains(err.Error(), "icmp-error-check") {
		t.Errorf("the SA refusal names no fix: %v", err)
	}
	sa.ICMPErrorCheck = false
	if err := vppUnsupportedSA(sa); err != nil {
		t.Errorf("vpp refused an SA that asks for no ICMP error check: %v", err)
	}

	if err := vppICMPErrorReturnError(SPParams{Dir: SADirOut, ICMPErrorReturn: true}); !errors.Is(err, ErrSAOptionRefused) || errors.Is(err, ErrNotSupported) {
		t.Errorf("vpp accepted a policy asking for the ICMP error return mapping: %v", err)
	}
	if err := vppICMPErrorReturnError(SPParams{Dir: SADirOut}); err != nil {
		t.Errorf("vpp refused a policy that asks for no ICMP error mapping: %v", err)
	}
}

// VALIDATES: VPP answers config verify from the predicates its install path uses: a peer
// asking for the RFC 4301 Section 6.2 ICMP error processing, the leaf's default, is
// refused with ErrSAOptionRefused naming icmp-error-check; the same peer with the check
// off and a fixed outer DSCP is accepted.
// PREVENTS: a VPP peer committed with the check on, then installed degraded or refused
// only once IKE negotiated its Child SA.
// RFC requirement: RFC4301-6.2-1 negative -- on VPP, whose dataplane cannot perform the ICMP error payload check, a peer configured to perform it is refused at config verify with ErrSAOptionRefused naming icmp-error-check, and a peer configured not to is accepted.
func TestVPPSAOptionsSupportedRefusesICMPErrorProcessing(t *testing.T) {
	b := &vppBackend{}
	err := b.SAOptionsSupported(SAOptions{ICMPErrorCheck: true})
	if !errors.Is(err, ErrSAOptionRefused) || errors.Is(err, ErrNotSupported) {
		t.Fatalf("icmp-error-check true on VPP: err = %v, want ErrSAOptionRefused and not ErrNotSupported", err)
	}
	if !strings.Contains(err.Error(), "icmp-error-check") {
		t.Errorf("the refusal names no fix: %v", err)
	}
	ef, err := FixedOuterDSCP(46)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.SAOptionsSupported(SAOptions{OuterDSCP: ef, DF: DFClear}); err != nil {
		t.Errorf("VPP refused options it applies: %v", err)
	}
	if err := b.SAOptionsSupported(SAOptions{DF: DFSet}); !errors.Is(err, ErrSAOptionRefused) {
		t.Errorf("df-bit set on VPP: err = %v, want ErrSAOptionRefused", err)
	}
}
