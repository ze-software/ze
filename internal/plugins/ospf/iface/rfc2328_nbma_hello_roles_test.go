// VALIDATES: RFC 2328 Section 9.5.1 for the roles the DROther units leave out: the DR and
// the Backup DR send periodic Hellos to every neighbor, ineligible ones included, and an
// ineligible router sends them to the DR and the Backup DR.
// PREVENTS: an eligibility gate that skips priority-0 neighbors in every state, and an
// ineligible router that drops the DR or BDR from its periodic Hellos.
package iface

import (
	"net/netip"
	"slices"
	"testing"
	"time"
)

// RFC requirement: RFC2328-9.5.1-1 positive -- on an NBMA interface with a heard eligible neighbor 10.0.0.2 and a heard priority-0 neighbor 10.0.0.9, the periodic Hello targets of a router in state DR and in state Backup hold both; and a router of priority 0 itself, in state DROther, targets both the DR 10.0.0.2 and the Backup DR 10.0.0.3 (helloTargetsLocked, nbma.go).
func TestRFC2328NBMAHelloTargetsByRole(t *testing.T) {
	// Goal: the DR/BDR clause and the ineligible-router clause of the sentence.
	// Method: helloTargetsLocked on one tick, neighbors marked heard.
	eligible := netip.MustParseAddr("10.0.0.2")
	backup := netip.MustParseAddr("10.0.0.3")
	ineligible := netip.MustParseAddr("10.0.0.9")
	for _, state := range []State{StateDR, StateBackup} {
		t.Run(state.String(), func(t *testing.T) {
			cfg := nbmaConfig(t)
			cfg.NBMANeighbors = []NBMANeighbor{{Address: eligible, Priority: 1}, {Address: ineligible, Priority: 0}}
			ifc := New(cfg, &fakeSender{}, NopMetrics())
			ifc.neighbors[rid(t, "10.0.0.2")] = Neighbor{RouterID: rid(t, "10.0.0.2"), Address: eligible}
			ifc.neighbors[rid(t, "10.0.0.9")] = Neighbor{RouterID: rid(t, "10.0.0.9"), Address: ineligible}
			ifc.state = state
			got := ifc.helloTargetsLocked(time.Unix(1000, 0))
			if !slices.Contains(got, eligible) {
				t.Errorf("Hello targets = %v, want the eligible neighbor %s", got, eligible)
			}
			if !slices.Contains(got, ineligible) {
				t.Errorf("Hello targets = %v, want the priority-0 neighbor %s from the %s", got, ineligible, state)
			}
		})
	}
	t.Run("ineligible router", func(t *testing.T) {
		cfg := nbmaConfig(t)
		cfg.Priority = 0
		cfg.NBMANeighbors = []NBMANeighbor{{Address: eligible, Priority: 1}, {Address: backup, Priority: 1}}
		ifc := New(cfg, &fakeSender{}, NopMetrics())
		ifc.neighbors[rid(t, "10.0.0.2")] = Neighbor{RouterID: rid(t, "10.0.0.2"), Address: eligible}
		ifc.neighbors[rid(t, "10.0.0.3")] = Neighbor{RouterID: rid(t, "10.0.0.3"), Address: backup}
		ifc.state = StateDROther
		got := ifc.helloTargetsLocked(time.Unix(1000, 0))
		for _, want := range []netip.Addr{eligible, backup} {
			if !slices.Contains(got, want) {
				t.Errorf("Hello targets of a priority-0 router = %v, want the DR/BDR %s", got, want)
			}
		}
	})
}
