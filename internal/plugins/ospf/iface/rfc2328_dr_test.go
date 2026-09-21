// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- RFC 2328 Appendix A.1 AllDRouters
// membership follows the DR/BDR election, and section 9.5.1 periodic Hellos on NBMA reach
// every eligible neighbor.

package iface

import (
	"net/netip"
	"slices"
	"testing"
	"time"
)

// electWith runs the DR/BDR election on a broadcast interface that has one two-way
// neighbor with peerPriority. The election is driven directly, without the ISM goroutine,
// so a break in the producer cannot wedge Stop behind the interface lock.
func electWith(t *testing.T, selfPriority, peerPriority uint8, sender *fakeSender) *Interface {
	t.Helper()
	cfg := baseConfig(t)
	cfg.Priority = selfPriority
	ifc := New(cfg, sender, NopMetrics())
	peer := rid(t, "10.0.0.2")
	ifc.neighbors[peer] = Neighbor{RouterID: peer, Address: netip.MustParseAddr("10.0.0.2"), Priority: peerPriority, TwoWay: true}
	ifc.state = StateWaiting
	ifc.runElectionLocked()
	return ifc
}

// RFC requirement: RFC2328-A.1-2 positive — when the election makes this router the Backup
// Designated Router, the interface joins AllDRouters (224.0.0.6) on its own name, so packets
// sent to that address are received (runElectionLocked calls sender.JoinAllDRouters).
func TestOSPFElectedBDRJoinsAllDRouters(t *testing.T) {
	sender := &fakeSender{}
	ifc := electWith(t, 1, 1, sender)
	if ifc.state != StateBackup {
		t.Fatalf("state = %v, want Backup (peer 10.0.0.2 wins DR by Router ID)", ifc.state)
	}
	if !slices.Equal(sender.joined, []string{"eth0"}) {
		t.Fatalf("JoinAllDRouters calls = %v, want [eth0] once the router is BDR", sender.joined)
	}
	if len(sender.left) != 0 {
		t.Fatalf("LeaveAllDRouters calls = %v, want none while BDR", sender.left)
	}
}

// RFC requirement: RFC2328-A.1-2 negative — a router the election leaves as DROther never
// joins AllDRouters, and a router that loses its BDR role leaves the group, so only the
// Designated Router and the Backup receive packets sent to 224.0.0.6 (runElectionLocked).
func TestOSPFDROtherDoesNotJoinAllDRouters(t *testing.T) {
	sender := &fakeSender{}
	ifc := electWith(t, 0, 1, sender)
	if ifc.state != StateDROther {
		t.Fatalf("state = %v, want DROther for a priority-0 router", ifc.state)
	}
	if len(sender.joined) != 0 {
		t.Fatalf("JoinAllDRouters calls = %v, want none for DROther", sender.joined)
	}

	// The role is lost: a BDR re-elected out of the role leaves the group.
	sender = &fakeSender{}
	ifc = electWith(t, 1, 1, sender)
	if ifc.state != StateBackup {
		t.Fatalf("state = %v, want Backup before the demotion", ifc.state)
	}
	ifc.cfg.Priority = 0
	ifc.runElectionLocked()
	if ifc.state != StateDROther {
		t.Fatalf("state = %v, want DROther after the priority drop", ifc.state)
	}
	if !slices.Equal(sender.left, []string{"eth0"}) {
		t.Fatalf("LeaveAllDRouters calls = %v, want [eth0] once the router is no longer BDR", sender.left)
	}
}

// RFC requirement: RFC2328-9.5.1-1 positive — on an NBMA interface an eligible router (priority
// above 0) lists every eligible configured neighbor as a periodic Hello target, heard or not,
// whatever its own DR state (helloTargetsLocked).
func TestOSPFNBMAEligibleRouterHellosEveryEligibleNeighbor(t *testing.T) {
	cfg := nbmaConfig(t)
	heard := netip.MustParseAddr("10.0.0.2")
	silent := netip.MustParseAddr("10.0.0.3")
	cfg.NBMANeighbors = []NBMANeighbor{{Address: heard, Priority: 1}, {Address: silent, Priority: 5}}
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	ifc.neighbors[rid(t, "10.0.0.2")] = Neighbor{RouterID: rid(t, "10.0.0.2"), Address: heard}
	ifc.state = StateDROther
	got := ifc.helloTargetsLocked(time.Unix(1000, 0))
	if !slices.Contains(got, heard) || !slices.Contains(got, silent) {
		t.Fatalf("Hello targets = %v, want both eligible neighbors %s and %s", got, heard, silent)
	}
}

// RFC requirement: RFC2328-9.5.1-1 negative — the periodic Hello set of an eligible router that
// is not DR or BDR holds only eligible neighbors: a priority-0 neighbor, heard or silent, is
// not a target (helloTargetsLocked eligibility gate).
func TestOSPFNBMAPeriodicHelloSkipsIneligibleNeighbor(t *testing.T) {
	cfg := nbmaConfig(t)
	eligible := netip.MustParseAddr("10.0.0.2")
	ineligible := netip.MustParseAddr("10.0.0.9")
	cfg.NBMANeighbors = []NBMANeighbor{{Address: eligible, Priority: 1}, {Address: ineligible, Priority: 0}}
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	ifc.neighbors[rid(t, "10.0.0.9")] = Neighbor{RouterID: rid(t, "10.0.0.9"), Address: ineligible}
	ifc.state = StateDROther
	got := ifc.helloTargetsLocked(time.Unix(1000, 0))
	if slices.Contains(got, ineligible) {
		t.Fatalf("Hello targets = %v, want no priority-0 neighbor from a DROther router", got)
	}
	if !slices.Equal(got, []netip.Addr{eligible}) {
		t.Fatalf("Hello targets = %v, want exactly [%s]", got, eligible)
	}
}
