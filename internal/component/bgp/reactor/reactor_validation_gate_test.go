// Design: docs/architecture/plugin/rib-storage-design.md -- validation eligibility between receive stores
// Related: reactor_validation_gate.go -- subscribeValidationGate

package reactor

import (
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
)

// TestSubscribeValidationGateAppliesAnnouncement proves the daemon applies a
// bgp-rib/validation-gate announcement to its own receive gate, so the
// forward path refuses routes while a forked store has validation enabled.
//
// VALIDATES: an enabled announcement on the bus turns ValidationEnabled on
// and RouteEligible to false in the daemon; a disabled one lifts it; release
// removes the subscription.
// PREVENTS: the forward path forwarding every route as eligible because the
// store's lookup, registered only in its own process, is absent here.
// Method: subscribe on a fake bus, emit each state through the typed handle,
// and read the gate.
func TestSubscribeValidationGateAppliesAnnouncement(t *testing.T) {
	t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
	ribevents.RegisterValidationLookup(nil, nil)
	bus := newFakeEventBus()
	r := New(&Config{ListenAddr: "127.0.0.1:0", Standalone: true})
	r.SetEventBus(bus)
	r.subscribeValidationGate()
	t.Cleanup(r.releaseEventSubscriptions)

	if _, err := ribevents.ValidationGate.Emit(bus, ribevents.ValidationGateState{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if !ribevents.ValidationEnabled() {
		t.Fatal("the announced gate must turn validation on in the daemon")
	}
	if ribevents.RouteEligible(ribevents.ValidationRoute{}, 0) {
		t.Fatal("the daemon must refuse a route the gate cannot judge here")
	}
	if _, err := ribevents.ValidationGate.Emit(bus, ribevents.ValidationGateState{Enabled: false}); err != nil {
		t.Fatal(err)
	}
	if ribevents.ValidationEnabled() {
		t.Fatal("the disabled gate must leave no validation in the daemon")
	}
	r.releaseEventSubscriptions()
	if bus.activeSubscriptions() != 0 {
		t.Fatal("release must remove the validation-gate subscription")
	}
}
