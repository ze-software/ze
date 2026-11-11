package rib

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

func TestValidationReceiverRestoresUngatedSelection(t *testing.T) {
	if ribevents.ValidationEnabled() {
		t.Fatal("test requires no receive validation owner")
	}
	// This outer cleanup protects later tests if the fixture regression fails;
	// the teardown assertions below run before it can clear the lookup.
	t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
	route := ribevents.ValidationRoute{
		Peer:   netip.MustParseAddr("192.0.2.1"),
		Family: family.IPv4Unicast,
		Prefix: netip.MustParsePrefix("203.0.113.0/24"),
	}
	if !ribevents.RouteEligible(route, 10) {
		t.Fatal("ungated route is ineligible before the fixture starts")
	}
	t.Run("receiver", func(t *testing.T) {
		startValidationReceiver(t, newTestEventBus())
		if !ribevents.ValidationEnabled() {
			t.Fatal("fixture did not enable receive validation")
		}
		if ribevents.RouteEligible(route, 10) {
			t.Fatal("active receive validation accepted an unreceived route")
		}
	})
	if ribevents.ValidationEnabled() {
		t.Error("fixture left receive validation enabled after its owner joined")
	}
	if !ribevents.RouteEligible(route, 10) {
		t.Error("fixture left an ordinary route ineligible after teardown")
	}
}
