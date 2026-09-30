// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- Child SA install into the dataplane
// Related: rfc4301_sad_selector_linux_test.go -- the namespace probe, sender and listeners reused here
// Related: spd_policy.go -- spdPolicyDirection, the operator entry whose order is the kernel priority
// Related: unmatched.go -- unmatchedPolicies, the catch-all ranked last
//
// VALIDATES: against a real Linux XFRM stack, that inbound traffic arriving on an SA is
// checked by an ORDERED search of the SPD Ze installs (RFC 4301 Section 4.4.1): where two
// inbound entries overlap, the one Ze ranks first decides, whichever way it decides.
// PREVENTS: Ze installing its inbound entries with priorities that invert the order the
// configuration states, so a catch-all DISCARD swallows a Child SA's traffic, or an
// operator DISCARD ranked before the Child SA is overridden by the SA's PROTECT entry.
//
// No privilege is needed: sadSelOwnNamespace re-runs the unit in a user and network
// namespace of its own, as its sibling file explains.

//go:build linux

package engine

import (
	"net"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
)

const (
	orderedStatPolBlock = "XfrmInPolBlock"
	// orderedOperatorRank is an operator SPD entry order between the IKE bypass (100)
	// and a negotiated Child SA (dataplane.PriorityChildSA), so the entry is searched
	// before the Child SA's inbound PROTECT entry.
	orderedOperatorRank = 1000
)

// orderedInstall installs each inbound entry through the backend Ze programs, and
// removes it when the test ends.
func orderedInstall(t *testing.T, entries []dataplane.SPParams) {
	t.Helper()
	dp := dataplane.Get()
	for _, p := range entries {
		if p.Dir != dataplane.SADirIn {
			continue
		}
		if err := dp.InstallPolicy(p); err != nil {
			t.Fatalf("install inbound entry %s -> %s: %v", p.Src, p.Dst, err)
		}
		t.Cleanup(func() {
			if err := dp.RemovePolicy(p.Src, p.Dst, p.Dir); err != nil {
				t.Errorf("remove inbound entry %s -> %s: %v", p.Src, p.Dst, err)
			}
		})
	}
}

// orderedInboundPriority reads back the priority and action of the kernel's inbound
// policy whose selector is src -> dst.
func orderedInboundPriority(t *testing.T, src, dst string) (int, netlink.PolicyAction) {
	t.Helper()
	policies, err := netlink.XfrmPolicyList(netlink.FAMILY_V4)
	if err != nil {
		t.Fatalf("list kernel policies: %v", err)
	}
	for i := range policies {
		p := &policies[i]
		if p.Dir != netlink.XFRM_DIR_IN || p.Src == nil || p.Dst == nil {
			continue
		}
		if p.Src.String() == src && p.Dst.String() == dst {
			return p.Priority, p.Action
		}
	}
	t.Fatalf("no inbound kernel policy %s -> %s", src, dst)
	return 0, 0
}

// TestRFC4301OrderedInboundSearchAdmitsOnTheFirstRankedEntry proves the admitting side of
// the ordered search. Method: a tunnel-mode Child SA (UDP from 10.1.0.0/16 to local port
// 5000 of 10.2.0.0/16) and the inbound SPD catch-all DISCARD, which overlaps it. The test
// reads back the inbound priorities Ze installed (Child SA ranked before the catch-all),
// then sends on the SA: an inner packet inside the Child SA's selector is delivered,
// because the first entry the search meets is the PROTECT entry, and an inner packet
// outside it meets only the catch-all and is discarded (XfrmInPolBlock moves by one).
func TestRFC4301OrderedInboundSearchAdmitsOnTheFirstRankedEntry(t *testing.T) {
	// RFC requirement: RFC4301-4.4.1-10 positive -- with Ze's inbound Child SA entry
	// (priority 2000) and the overlapping inbound catch-all DISCARD (priority 2^32-1)
	// installed, an inner packet on the SA inside the Child SA selector is delivered, so
	// the search decides on the first-ranked entry and not the overlapping later one.
	if !sadSelOwnNamespace(t) {
		return
	}
	sadSelNetns(t)
	child := sadSelChild(t, false, "10.2.0.0/16", "10.1.0.0/16")
	catchAll, err := unmatchedPolicies(net.IPv4zero, dataplane.SPActionDiscard)
	if err != nil {
		t.Fatalf("build the catch-all: %v", err)
	}
	orderedInstall(t, catchAll)

	childRank, childAction := orderedInboundPriority(t, "10.1.0.0/16", "10.2.0.0/16")
	if childRank != dataplane.PriorityChildSA || childAction != netlink.XFRM_POLICY_ALLOW {
		t.Fatalf("inbound Child SA entry: priority %d action %d, want %d allow", childRank, childAction, dataplane.PriorityChildSA)
	}
	lastRank, lastAction := orderedInboundPriority(t, "0.0.0.0/0", "0.0.0.0/0")
	if lastRank != dataplane.PriorityUnmatched || lastAction != netlink.XFRM_POLICY_BLOCK {
		t.Fatalf("inbound catch-all: priority %d action %d, want %d block", lastRank, lastAction, dataplane.PriorityUnmatched)
	}

	sender := newSadSelSender(t, child)
	inside := sadSelListen(t, sadSelInnerLocal, sadSelPort)
	before := sadSelStat(t, orderedStatPolBlock)
	sender.send(t, sadSelProtoIPv4, sadSelIPv4(sadSelInnerRemote, sadSelProtoUDP, sadSelUDP(sadSelPort)))
	if !sadSelDelivered(t, inside) {
		t.Fatal("an inner packet inside the Child SA selector was not delivered: the overlapping catch-all decided instead of the first-ranked entry")
	}
	if got := sadSelStat(t, orderedStatPolBlock) - before; got != 0 {
		t.Errorf("%s moved by %d for the in-selector packet, want 0", orderedStatPolBlock, got)
	}

	before = sadSelStat(t, orderedStatPolBlock)
	sender.send(t, sadSelProtoIPv4, sadSelIPv4(sadSelInnerStray, sadSelProtoUDP, sadSelUDP(sadSelPort)))
	if sadSelDelivered(t, inside) {
		t.Error("an inner packet outside every entry but the catch-all was delivered")
	}
	if got := sadSelStat(t, orderedStatPolBlock) - before; got != 1 {
		t.Errorf("%s moved by %d for the out-of-selector packet, want 1 (the catch-all DISCARD)", orderedStatPolBlock, got)
	}
}

// TestRFC4301OrderedInboundSearchRefusesOnTheFirstRankedDiscard proves the refusing side.
// Method: the same Child SA, plus an operator SPD entry DISCARD, inbound, order 1000,
// whose selector (10.1.0.0/16 to 10.2.0.0/16, any protocol and port) covers the Child
// SA's. The test reads back the operator entry's priority (ranked before the Child SA),
// then sends an inner packet inside the Child SA selector on the SA: the Child SA's
// PROTECT entry would admit it, but the search meets the DISCARD first, so it is dropped
// (XfrmInPolBlock moves by one) and never delivered.
func TestRFC4301OrderedInboundSearchRefusesOnTheFirstRankedDiscard(t *testing.T) {
	// RFC requirement: RFC4301-4.4.1-10 negative -- inbound traffic on the SA that the
	// SPD forbids (an operator DISCARD Ze installs at priority 1000, before the Child SA
	// entry at 2000) is dropped (XfrmInPolBlock) and not delivered, even though the later
	// PROTECT entry's template matches the SA it arrived on.
	if !sadSelOwnNamespace(t) {
		return
	}
	sadSelNetns(t)
	child := sadSelChild(t, false, "10.2.0.0/16", "10.1.0.0/16")
	orderedInstall(t, spdPolicyParams(ipsec.SPDPolicy{
		Name:         "deny-peer-lan",
		Action:       dataplane.SPActionDiscard,
		Order:        orderedOperatorRank,
		Direction:    ipsec.SPDDirIn,
		LocalPrefix:  mustCIDRNet(t, "10.2.0.0/16"),
		LocalPort:    ipsec.PortSelector{Form: ipsec.PortAny},
		RemotePrefix: mustCIDRNet(t, "10.1.0.0/16"),
		RemotePort:   ipsec.PortSelector{Form: ipsec.PortAny},
	}))

	rank, action := orderedInboundPriority(t, "10.1.0.0/16", "10.2.0.0/16")
	if rank != orderedOperatorRank || action != netlink.XFRM_POLICY_BLOCK {
		t.Fatalf("inbound entry for the Child SA selector: priority %d action %d, want the operator's %d block", rank, action, orderedOperatorRank)
	}

	sender := newSadSelSender(t, child)
	inside := sadSelListen(t, sadSelInnerLocal, sadSelPort)
	before := sadSelStat(t, orderedStatPolBlock)
	sender.send(t, sadSelProtoIPv4, sadSelIPv4(sadSelInnerRemote, sadSelProtoUDP, sadSelUDP(sadSelPort)))
	if sadSelDelivered(t, inside) {
		t.Error("an inner packet the first-ranked DISCARD forbids was delivered: a later entry decided")
	}
	if got := sadSelStat(t, orderedStatPolBlock) - before; got != 1 {
		t.Errorf("%s moved by %d, want 1 (dropped by the first-ranked DISCARD)", orderedStatPolBlock, got)
	}
}
