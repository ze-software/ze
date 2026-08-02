package engine

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// rekeyPair builds a Child SA and the replacement a make-before-break rekey would
// put beside it. newRekeyedChild is the production builder, so the two share exactly
// what production makes them share.
func rekeyPair(t *testing.T) (old, replacement *ChildSA) {
	t.Helper()
	_, tsLocal, err := net.ParseCIDR("0.0.0.0/0")
	if err != nil {
		t.Fatalf("parsing the traffic selector: %v", err)
	}
	_, tsRemote, err := net.ParseCIDR("0.0.0.0/0")
	if err != nil {
		t.Fatalf("parsing the traffic selector: %v", err)
	}
	old = &ChildSA{
		InboundSPI:  0x1111,
		OutboundSPI: 0x2222,
		LocalAddr:   net.ParseIP("172.28.0.2"),
		RemoteAddr:  net.ParseIP("172.28.0.3"),
		IfID:        7,
		ReqID:       7,
		TSLocal:     tsLocal,
		TSRemote:    tsRemote,
		Mode:        modeTunnel,
	}
	replacement = newRekeyedChild(old, 0x3333, 0x4444, &crypto.ChildSAKeys{}, true)
	return old, replacement
}

// VALIDATES: a Child SA retired by a make-before-break rekey gives up its STATES and
// keeps the POLICIES the replacement still answers to.
// PREVENTS: the tunnel dying at the moment a rekey SUCCEEDS. Both pairs share one
// pair of kernel policies, because newRekeyedChild inherits the selector and the
// backend upserts it, so removing the retired pair's policy removes the live pair's.
// MEASURED against strongSwan as "no ESP traffic after the rekey" (interop 05).
func TestRetiredChildKeepsThePolicyTheReplacementUses(t *testing.T) {
	old, replacement := rekeyPair(t)
	if !samePolicySelector(old, replacement) {
		t.Fatal("newRekeyedChild produced a replacement with a DIFFERENT policy selector; " +
			"this test's premise is gone and the sharing it guards no longer happens")
	}

	dp := &bypassDP{}
	removeChildSAExcept(old, replacement, dp, slogutil.DiscardLogger())

	// Both removal APIs are checked. removeChildSA* now goes through the owner-aware
	// RemovePolicyParams (dp.removed), and reading only the three-argument recorder
	// would have made this assertion vacuous the moment the call moved.
	if len(dp.removed) != 0 || len(dp.removedPolicies) != 0 {
		t.Errorf("retiring the superseded pair removed %d+%d policies (%+v %+v); the replacement "+
			"answers to them and the tunnel stops forwarding",
			len(dp.removed), len(dp.removedPolicies), dp.removed, dp.removedPolicies)
	}
	// The states are keyed by SPI and are never shared, so both must go.
	if len(dp.removedSAs) != 2 {
		t.Errorf("removed %d states, want 2 (inbound + outbound) for the retired pair", len(dp.removedSAs))
	}
}

// VALIDATES: the last Child SA on a selector DOES take the policies with it.
// PREVENTS: the fix above turning into a leak. If nothing ever removed the policies,
// a torn-down tunnel would leave its SPD entries behind and the next flow matching
// that selector would be captured by a policy resolving to no state.
func TestLastChildOnASelectorRemovesThePolicies(t *testing.T) {
	old, _ := rekeyPair(t)

	dp := &bypassDP{}
	removeChildSA(old, dp, slogutil.DiscardLogger())

	if len(dp.removed) != 2 {
		t.Fatalf("removed %d policies, want 2 (in + out) when nothing else uses the selector", len(dp.removed))
	}
	dirs := map[dataplane.SADir]bool{}
	for _, p := range dp.removed {
		dirs[p.Dir] = true
	}
	if !dirs[dataplane.SADirIn] || !dirs[dataplane.SADirOut] {
		t.Errorf("policy removals covered %v, want both SADirIn and SADirOut", dp.removed)
	}
	// The removal must go through the owner-aware API. The three-argument RemovePolicy
	// carries no owner, so a removal taking that route could delete a policy another
	// peer installed (dataplane.SPParams.Owner).
	if len(dp.removedPolicies) != 0 {
		t.Errorf("%d removals took the ownerless three-argument RemovePolicy route (%+v); "+
			"that route cannot refuse a foreign delete", len(dp.removedPolicies), dp.removedPolicies)
	}
	for _, p := range dp.removed {
		if p.Owner != old.Owner {
			t.Errorf("removal for %v carries owner %q, want %q: an unowned delete cannot be refused",
				p.Dir, p.Owner, old.Owner)
		}
	}
}

// VALIDATES: the sharing test is a real comparison, not a constant true.
// PREVENTS: samePolicySelector degenerating into "always share", which would turn
// the fix above into a policy leak for every genuinely distinct Child SA.
func TestPolicySelectorComparisonDiscriminates(t *testing.T) {
	old, replacement := rekeyPair(t)

	if samePolicySelector(old, nil) {
		t.Error("a nil survivor must never be reported as sharing a selector; that is the " +
			"whole-tunnel teardown case and the policies have to go")
	}

	_, other, err := net.ParseCIDR("10.99.0.0/24")
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	narrowed := *replacement
	narrowed.TSRemote = other
	if samePolicySelector(old, &narrowed) {
		t.Error("two Child SAs with DIFFERENT remote traffic selectors were reported as " +
			"sharing one policy; the retired pair's own policy would be left installed")
	}

	differentIfID := *replacement
	differentIfID.IfID = old.IfID + 1
	if samePolicySelector(old, &differentIfID) {
		t.Error("two Child SAs bound to different XFRM if_ids were reported as sharing one " +
			"policy; if_id is part of the selector the kernel matches on")
	}
}
