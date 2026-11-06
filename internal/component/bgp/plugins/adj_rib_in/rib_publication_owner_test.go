// Design: docs/architecture/plugin/rib-storage-design.md -- validation publication ownership.
package adj_rib_in

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

func TestValidationOwnerRemovalSettlesBeforeFencing(t *testing.T) {
	r := newTestManager(t)
	key := ribevents.ValidationRoute{Peer: netip.MustParseAddr("192.0.2.1"),
		Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("203.0.113.0/24")}
	r.validationChanges = []ribevents.ValidationRoute{key}
	failure := errors.New("publication refused")
	r.emitValidation = func([]ribevents.ValidationRoute) error { return failure }
	if err := r.removeValidationOwner("removed"); !errors.Is(err, failure) {
		t.Fatalf("failed removal = %v, want publication refusal", err)
	}
	if err := r.admitMutation(true); !errors.Is(err, failure) {
		if err == nil {
			r.mutationMu.Unlock()
		}
		t.Fatalf("paused publication admitted a mutation: %v", err)
	}
	if err := r.validationFailure(); err != nil {
		t.Fatal("refused removal fenced a recoverable owner:", err)
	}
	published := 0
	r.emitValidation = func(routes []ribevents.ValidationRoute) error {
		published += len(routes)
		if len(routes) != 1 || routes[0] != key {
			t.Fatalf("removal did not settle exact owed key: %+v", routes)
		}
		return nil
	}
	if err := r.removeValidationOwner("removed"); err != nil {
		t.Fatal(err)
	}
	if published != 1 {
		t.Fatalf("published %d keys, want one", published)
	}
	if err := r.admitMutation(true); !errors.Is(err, errReceiveStoreRemoved) {
		if err == nil {
			r.mutationMu.Unlock()
		}
		t.Fatalf("successful removal admission = %v", err)
	}
	if err := r.validationFailure(); err != nil {
		t.Fatal("successful removal became a fatal owner failure:", err)
	}
}

// TestValidationOwnerExitFencesOnlyAGate exits an owner the way a daemon stop
// does: the connection closes, no bye arrives, so removeValidationOwner never ran.
//
// VALIDATES: an owner that never enabled validation leaves the process with no
// gate, while one that enabled it leaves a lookup refusing every route, including
// after disable-validation, because its lookup stays registered until exit.
// PREVENTS: a store's stop switching on a refusing gate that makes every later
// forward in the process (a reload's new runtime, the next test) withdraw routes.
func TestValidationOwnerExitFencesOnlyAGate(t *testing.T) {
	t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
	key := ribevents.ValidationRoute{Peer: netip.MustParseAddr("192.0.2.1"),
		Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("203.0.113.0/24")}

	ribevents.RegisterValidationLookup(nil, nil)
	ungated := newTestManager(t)
	ungated.releaseValidationOwner()
	if ribevents.ValidationEnabled() {
		t.Fatal("an owner with no gate enabled one by exiting")
	}
	if !ribevents.RouteEligible(key, 0) {
		t.Fatal("an owner with no gate refused a route by exiting")
	}

	gated := newTestManager(t)
	gated.ingestTracked = true
	if _, _, err := gated.enableValidationCommand(nil); err != nil {
		t.Fatal(err)
	}
	if !ribevents.ValidationEnabled() {
		t.Fatal("enable-validation registered no lookup")
	}
	if _, _, err := gated.disableValidationCommand(); err != nil {
		t.Fatal(err)
	}
	gated.releaseValidationOwner()
	if !ribevents.ValidationEnabled() {
		t.Fatal("exit of an owner that held a gate left no fence")
	}
	if ribevents.RouteEligible(key, 0) {
		t.Fatal("exit of an owner that held a gate reopened eligibility")
	}
}

func TestValidationOwnerTerminalCauseSurvivesTeardown(t *testing.T) {
	t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
	r := newTestManager(t)
	failure := errors.New("withdrawal publication refused")
	r.failValidationOwner(failure)
	if err := r.validationFailure(); !errors.Is(err, failure) {
		t.Fatalf("terminal cause = %v", err)
	}
	if _, _, err := r.enableValidationCommand(nil); !errors.Is(err, failure) {
		t.Fatalf("enable after terminal failure = %v", err)
	}
	if err := r.removeValidationOwner("removed"); !errors.Is(err, failure) {
		t.Fatalf("failed owner acknowledged removal: %v", err)
	}
	r.releaseValidationOwner()
	key := ribevents.ValidationRoute{Peer: netip.MustParseAddr("192.0.2.1"),
		Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("203.0.113.0/24")}
	if ribevents.RouteEligible(key, 0) {
		t.Fatal("teardown reopened eligibility")
	}
}
