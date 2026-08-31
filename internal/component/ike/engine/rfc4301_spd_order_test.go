// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SA/SP installation
// Related: child.go -- childPolicyParams, the producer of every IKE-installed SPD entry
// RFC: rfc/short/rfc4301.md -- Security Policy Database ordering (Section 4.4.1)
package engine

import (
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: RFC4301-4.4.1-4. The operator's policy-priority reaches every SPD entry the
// peer installs, in both directions, so two peers whose selectors overlap are ordered by
// what the operator wrote rather than by installation order.
// PREVENTS: an overlapping pair of peers being resolved by whichever established last.
// Every IKE-installed policy took the single constant dataplane.PriorityChildSA, so the
// kernel saw two entries of equal rank and fell back to insertion order.
// RFC requirement: RFC4301-4.4.1-4 positive -- the operator orders the SPD entries.
func TestSpdOperatorOrdersOverlappingPeers(t *testing.T) {
	// Two peers whose selectors are both 0.0.0.0/0, which is the overlap Section 4.4.1
	// says ordering exists for: "The ordering requirement arises because entries often
	// will overlap due to the presence of (non-trivial) ranges as values for selectors."
	const (
		firstPriority  = 500
		secondPriority = 1500
	)

	first := wideSA("site-a", "10.0.0.2")
	first.PeerCfg.PolicyPriority = firstPriority
	second := wideSA("site-b", "10.0.0.3")
	second.PeerCfg.PolicyPriority = secondPriority

	childA, err := createFirstChildSA(first, testESPGroup(), "10.0.0.1", "10.0.0.2", testChildIfID, nil, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("createFirstChildSA for the first peer: %v", err)
	}
	childB, err := createFirstChildSA(second, testESPGroup(), "10.0.0.1", "10.0.0.3", testChildIfID, nil, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("createFirstChildSA for the second peer: %v", err)
	}

	for _, dir := range bothDirections {
		if got := childPolicyParams(childA, dir).Priority; got != firstPriority {
			t.Errorf("the %s entry of the first peer ranks %d, and the operator wrote %d",
				dirName(dir), got, firstPriority)
		}
		if got := childPolicyParams(childB, dir).Priority; got != secondPriority {
			t.Errorf("the %s entry of the second peer ranks %d, and the operator wrote %d",
				dirName(dir), got, secondPriority)
		}
		// Lower value means higher precedence, in the kernel and in ze (dataplane.go).
		if childPolicyParams(childA, dir).Priority >= childPolicyParams(childB, dir).Priority {
			t.Errorf("the %s entries of the two peers are not in the order the operator wrote",
				dirName(dir))
		}
	}
}

// VALIDATES: RFC4301-4.4.1-4. A peer that states no order takes the documented default
// rank, so an unordered configuration installs exactly what it installed before the leaf
// existed, and a rekey inherits the operator's rank rather than falling back.
// PREVENTS: a zero reading as a valid rank. Priority 0 outranks the IKE control-plane
// bypass, so a peer whose priority went unset would capture the IKE traffic that builds
// it and prevent its own rekey (dataplane.PriorityIKEBypass).
// RFC requirement: RFC4301-4.4.1-4 negative -- an unstated order never reaches the SPD as 0.
func TestSpdUnstatedOrderTakesTheDefaultRank(t *testing.T) {
	sa := wideSA("site-a", "10.0.0.2")
	if sa.PeerCfg.PolicyPriority != 0 {
		t.Fatal("this test needs a peer with no stated priority; the fixture already sets one")
	}

	child, err := createFirstChildSA(sa, testESPGroup(), "10.0.0.1", "10.0.0.2", testChildIfID, nil, slogutil.DiscardLogger())
	if err != nil {
		t.Fatalf("createFirstChildSA: %v", err)
	}

	for _, dir := range bothDirections {
		got := childPolicyParams(child, dir).Priority
		if got == 0 {
			t.Fatalf("the %s entry reached the SPD at rank 0, which outranks the IKE bypass at %d",
				dirName(dir), dataplane.PriorityIKEBypass)
		}
		if got != dataplane.PriorityChildSA {
			t.Errorf("the %s entry ranks %d, and an unordered peer takes %d",
				dirName(dir), got, dataplane.PriorityChildSA)
		}
	}

	// A rekey replaces the states and keeps the entry, so the rank travels with it.
	replacement := newRekeyedChild(child, child.InboundSPI+1, child.OutboundSPI+1, nil, child.LocalIsInitiator, child.Selectors)
	for _, dir := range bothDirections {
		if got := childPolicyParams(replacement, dir).Priority; got != dataplane.PriorityChildSA {
			t.Errorf("the rekeyed %s entry ranks %d, and the retired one ranked %d",
				dirName(dir), got, dataplane.PriorityChildSA)
		}
	}
}

// VALIDATES: RFC4301-4.4.1-4. An operator rank that would capture the IKE control plane
// is refused at commit, so the ordering interface cannot be used to break the SPD it
// orders.
// PREVENTS: a tunnel that cannot rekey itself. A Child SA entry ranked at or above the
// IKE bypass matches the IKE datagrams before the bypass does, so the exchange that
// builds, rekeys and tears down that very SA is handed to the SA (dataplane.go).
// RFC requirement: RFC4301-4.4.1-4 negative -- an order that captures IKE is refused.
func TestSpdOrderCannotCaptureTheIKEControlPlane(t *testing.T) {
	for _, priority := range []uint32{0, 1, dataplane.PriorityIKEBypass} {
		cfg := &ipsec.IPsecConfig{
			Peers: map[string]ipsec.SiteToSitePeer{
				"branch": {Name: "branch", PolicyPriority: priority},
			},
		}
		if err := cfg.ValidatePolicyOrder(); err == nil {
			t.Errorf("policy-priority %d committed, and it outranks the IKE bypass at %d",
				priority, dataplane.PriorityIKEBypass)
		}
	}

	ok := &ipsec.IPsecConfig{
		Peers: map[string]ipsec.SiteToSitePeer{
			"branch": {Name: "branch", PolicyPriority: dataplane.PriorityIKEBypass + 1},
		},
	}
	if err := ok.ValidatePolicyOrder(); err != nil {
		t.Errorf("the first rank below the IKE bypass was refused: %v", err)
	}
}
