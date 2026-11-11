// Design: docs/architecture/plugin/rib-storage-design.md -- validation publication ownership.
// Related: rpki_reload_test.go -- startRPKIReloadReceiver owns the fixture lifecycle.

package rpki

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// TestRPKIReloadReceiverRestoresUngatedSelection drives the real receiver and
// its public gate command, then checks that fixture teardown releases the test
// process after the receiver joins. Production's exit refusal stays unchanged.
func TestRPKIReloadReceiverRestoresUngatedSelection(t *testing.T) {
	if ribevents.ValidationEnabled() {
		t.Fatal("test requires no receive validation owner")
	}
	// Protect later tests if the teardown assertions fail. This cleanup runs
	// after those assertions and cannot make them pass.
	t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
	route := ribevents.ValidationRoute{
		Peer:   netip.MustParseAddr("192.0.2.1"),
		Family: family.IPv4Unicast,
		Prefix: netip.MustParsePrefix("192.0.2.0/24"),
	}
	if !ribevents.RouteEligible(route, 10) {
		t.Fatal("ungated route is ineligible before the fixture starts")
	}
	t.Run("receiver", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		t.Cleanup(cancel)
		adj := startRPKIReloadReceiver(t, ctx)
		out, err := adj.ExecuteCommand(ctx, "enable", "request bgp adj-rib-in enable-validation", nil, "")
		if err != nil {
			t.Fatal(err)
		}
		if out.Status != "done" {
			t.Fatalf("enable-validation status=%q, want done", out.Status)
		}
		if !ribevents.ValidationEnabled() {
			t.Fatal("fixture did not enable receive validation")
		}
		received, _ := receiveRPKIReloadRoute(t, adj)
		if received != route {
			t.Fatalf("fixture received route %+v, want %+v", received, route)
		}
		if ribevents.RouteEligible(route, 10) {
			t.Fatal("active receive validation accepted a route with no verdict")
		}
	})
	if ribevents.ValidationEnabled() {
		t.Error("fixture left receive validation enabled after its owner joined")
	}
	if !ribevents.RouteEligible(route, 10) {
		t.Error("fixture left an ordinary route ineligible after teardown")
	}
}
