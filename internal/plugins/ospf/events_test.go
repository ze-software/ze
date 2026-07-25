// Design: plan/learned/958-ospf-4-component-config.md -- OSPF event namespace tests
//
// VALIDATES: OSPF event namespace registers neighbor, interface, SPF, and LSDB
// change event types without collision.
package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/core/events"
)

func TestOSPFEventNamespace(t *testing.T) {
	for _, et := range []string{EventNeighborUp, EventNeighborDown, EventSPFRun, EventLSDBChange, EventInterfaceState, EventDRChange, EventNeighborChange} {
		if !events.IsValidEvent(Namespace, et) {
			t.Errorf("event %q not valid in namespace %q", et, Namespace)
		}
	}
	if events.IsValidEvent(Namespace, "not-an-ospf-event") {
		t.Error("unexpected OSPF event type reported valid")
	}
	if NeighborUp.EventType() != EventNeighborUp {
		t.Errorf("NeighborUp.EventType() = %q, want %q", NeighborUp.EventType(), EventNeighborUp)
	}
	if NeighborDown.EventType() != EventNeighborDown {
		t.Errorf("NeighborDown.EventType() = %q, want %q", NeighborDown.EventType(), EventNeighborDown)
	}
	if spfRun.EventType() != EventSPFRun {
		t.Errorf("spfRun.EventType() = %q, want %q", spfRun.EventType(), EventSPFRun)
	}
	if lsdbChange.EventType() != EventLSDBChange {
		t.Errorf("lsdbChange.EventType() = %q, want %q", lsdbChange.EventType(), EventLSDBChange)
	}
	if InterfaceState.EventType() != EventInterfaceState {
		t.Errorf("InterfaceState.EventType() = %q, want %q", InterfaceState.EventType(), EventInterfaceState)
	}
	if DRChange.EventType() != EventDRChange {
		t.Errorf("DRChange.EventType() = %q, want %q", DRChange.EventType(), EventDRChange)
	}
	if NeighborChange.EventType() != EventNeighborChange {
		t.Errorf("NeighborChange.EventType() = %q, want %q", NeighborChange.EventType(), EventNeighborChange)
	}
}
