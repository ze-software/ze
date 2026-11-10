// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the outer header DF bit
// Related: rekey.go -- newRekeyedChild, the producer of the replacement Child SA
// Related: create_child.go -- newChildSA, the producer of a CREATE_CHILD_SA Child SA
// RFC: rfc/short/rfc4301.md -- Section 8.1 DF bit, Section 5.1.2.1 tunnel header DSCP

package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// VALIDATES: a rekeyed Child SA keeps the DF treatment, the outer DSCP and the ICMP
// error processing choice of the SA it replaces, so the replacement's states carry
// what the operator configured.
// PREVENTS: the first rekey silently returning the tunnel to copy/copy, or turning a
// peer's `icmp-error-check false` back on.
// RFC requirement: RFC4301-8.1-2 positive -- newRekeyedChild carries the retired Child SA's DFBit onto its replacement.
// RFC requirement: RFC4301-5.1.2.1-1 positive -- newRekeyedChild carries the retired Child SA's OuterDSCP onto its replacement.
// RFC requirement: RFC4301-6.2-1 positive -- newRekeyedChild carries the retired Child SA's SkipICMPErrorCheck onto its replacement.
func TestRekeyKeepsTheOuterHeaderTreatment(t *testing.T) {
	ef, err := dataplane.FixedOuterDSCP(46)
	if err != nil {
		t.Fatal(err)
	}
	old := &ChildSA{Mode: modeTunnel, DFBit: dataplane.DFClear, OuterDSCP: ef, SkipICMPErrorCheck: true}
	replacement := newRekeyedChild(old, 0x1001, 0x1002, nil, true, nil, modeTunnel)
	if replacement.DFBit != dataplane.DFClear {
		t.Errorf("the replacement carries DF %s, want clear", replacement.DFBit)
	}
	if replacement.OuterDSCP != ef {
		t.Errorf("the replacement carries outer DSCP %s, want 46", replacement.OuterDSCP)
	}
	// The zero value is the check ON, so the retired SA carries the non-default value.
	if !replacement.SkipICMPErrorCheck {
		t.Errorf("the replacement turned the ICMP error payload check back on, which the retired SA's peer turned off")
	}
}

// VALIDATES: a Child SA built by CREATE_CHILD_SA (newChildSA, the builder both
// directions call) takes the peer's DF treatment, outer DSCP and ICMP error processing
// choice from the IKE SA's peer config. Method: every option set to its non-default
// value, so a dropped field reads back as its default and is red.
// PREVENTS: a second Child SA on the same peer installed with copy/copy and the ICMP
// check on, whatever the operator configured for the first.
// RFC requirement: RFC4301-6.2-1 positive -- newChildSA carries the peer's icmp-error-check false onto a CREATE_CHILD_SA Child SA as SkipICMPErrorCheck.
func TestCreatedChildTakesThePeerSAOptions(t *testing.T) {
	ef, err := dataplane.FixedOuterDSCP(46)
	if err != nil {
		t.Fatal(err)
	}
	sa := testSA()
	sa.PeerCfg.DFBit = dataplane.DFClear
	sa.PeerCfg.OuterDSCP = ef
	sa.PeerCfg.SkipICMPErrorCheck = true
	created, err := newChildSA(sa, "10.0.0.1", "10.0.0.2", &childSpec{mode: modeTunnel})
	if err != nil {
		t.Fatalf("newChildSA: %v", err)
	}
	if created.DFBit != dataplane.DFClear {
		t.Errorf("the created Child SA carries DF %s, want clear", created.DFBit)
	}
	if created.OuterDSCP != ef {
		t.Errorf("the created Child SA carries outer DSCP %s, want 46", created.OuterDSCP)
	}
	if !created.SkipICMPErrorCheck {
		t.Errorf("the created Child SA carries the ICMP error payload check, which the peer turned off")
	}
}
