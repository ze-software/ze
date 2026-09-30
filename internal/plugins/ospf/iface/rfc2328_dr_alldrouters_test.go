// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- RFC 2328 Appendix A.1 AllDRouters
// membership for the Designated Router itself.

package iface

import (
	"slices"
	"testing"
)

// VALIDATES: RFC 2328 Appendix A.1, the Designated Router receives packets sent to AllDRouters.
// PREVENTS: a DR that never joins 224.0.0.6, or keeps the group after losing the role.

// TestOSPFElectedDRJoinsAllDRouters proves the Designated Router joins AllDRouters and leaves
// it when the election takes the role away. RFC 2328 Appendix A.1: "Both the Designated
// Router and Backup Designated Router must be prepared to receive packets destined to this
// address."
//
// Goal: the DR half of the sentence, which the BDR tests leave unasserted.
// Method: electWith drives the election directly on a broadcast interface with one two-way
// neighbor of lower priority, so this router is elected DR; its priority is then dropped to
// 0 and the election re-run, which makes it DROther.
func TestOSPFElectedDRJoinsAllDRouters(t *testing.T) {
	sender := &fakeSender{}
	ifc := electWith(t, 5, 2, sender)
	if ifc.state != StateDR {
		t.Fatalf("state = %v, want DR (priority 5 against the neighbor's 2)", ifc.state)
	}
	// RFC requirement: RFC2328-A.1-2 positive -- when the election makes this router the
	// Designated Router, the interface joins AllDRouters (224.0.0.6) on its own name
	// (runElectionLocked calls sender.JoinAllDRouters).
	if !slices.Equal(sender.joined, []string{"eth0"}) {
		t.Fatalf("JoinAllDRouters calls = %v, want [eth0] once the router is DR", sender.joined)
	}
	if len(sender.left) != 0 {
		t.Fatalf("LeaveAllDRouters calls = %v, want none while DR", sender.left)
	}

	ifc.cfg.Priority = 0
	ifc.runElectionLocked()
	if ifc.state != StateDROther {
		t.Fatalf("state = %v, want DROther after the priority drop", ifc.state)
	}
	// RFC requirement: RFC2328-A.1-2 negative -- a Designated Router the election demotes to
	// DROther leaves AllDRouters, so it no longer receives packets sent to 224.0.0.6.
	if !slices.Equal(sender.left, []string{"eth0"}) {
		t.Fatalf("LeaveAllDRouters calls = %v, want [eth0] once the DR is demoted", sender.left)
	}
}
