// Design: docs/architecture/bgp/structural-forwarding.md -- receive validation and cache changes
// Related: rib_validation.go -- removePending, applyDecision
// Related: rib_eligibility.go -- routeEligible, routePresent, noteValidationChange
//
// VALIDATES: a withdrawal of a route still pending validation cancels its
// pending work at the one pending record, and every withdrawal under the gate
// wakes the ValidationChange relay that carries it after any queued announce.
// PREVENTS: a verdict that arrives after the withdrawal installing the
// canceled generation, and a withdrawal of an eligible route that the live
// forward deferred never reaching the destination.
package adj_rib_in

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// pendingWithdrawalKey is the path the retained-receive helpers carry:
// 203.0.113.0/24, ADD-PATH identifier 1, from 192.0.2.1.
var pendingWithdrawalKey = ribevents.ValidationRoute{Peer: netip.MustParseAddr("192.0.2.1"),
	Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("203.0.113.0/24"), PathID: 1}

// pendingWithdrawalBody withdraws pendingWithdrawalKey in ADD-PATH framing.
var pendingWithdrawalBody = []byte{0, 8, 0, 0, 0, 1, 24, 203, 0, 113, 0, 0}

func newPendingWithdrawalManager(t *testing.T) (*AdjRIBInManager, *notifyTestBus) {
	t.Helper()
	t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
	bus := &notifyTestBus{}
	r := newTestManager(t)
	r.ingestTracked = true
	r.validationBus = bus
	_, _, err := r.handleCommand("request bgp adj-rib-in enable-validation", nil, "")
	require.NoError(t, err)
	return r, bus
}

// TestPendingWithdrawalCancelsTheVerdict proves the withdrawal of a pending
// route cancels it. Method: receive the route under the gate, so it is pending;
// withdraw it; then deliver the Valid verdict for that generation. The route
// must be neither present nor eligible afterwards, so the ValidationChange
// relay the withdrawal woke answers an explicit withdrawal and the verdict
// announces nothing.
func TestPendingWithdrawalCancelsTheVerdict(t *testing.T) {
	r, bus := newPendingWithdrawalManager(t)

	retainedReceive(t, r, 40, rfc4271Announce(1, 0, 0, 0, 1, 24, 203, 0, 113))
	require.True(t, r.routePresent(pendingWithdrawalKey), "the route waits in the pending state")
	require.False(t, r.routeEligible(pendingWithdrawalKey, 0), "a pending route is not eligible")

	bus.routes = nil
	retainedReceive(t, r, 41, pendingWithdrawalBody)
	require.Contains(t, bus.routes, pendingWithdrawalKey, "the withdrawal wakes the relay for its key")
	require.False(t, r.routePresent(pendingWithdrawalKey), "the withdrawal cancels the pending route")

	retainedDecision(t, r, "a", 1, 40)
	require.False(t, r.routePresent(pendingWithdrawalKey), "a verdict for the canceled generation installs nothing")
	require.False(t, r.routeEligible(pendingWithdrawalKey, 0), "a verdict for the canceled generation makes nothing eligible")
	require.False(t, r.routeEligible(pendingWithdrawalKey, 40))
}

// TestAnnouncedWithdrawalWakesTheRelay proves the withdrawal of an installed,
// eligible route is handed to the ValidationChange relay. Method: receive the
// route, accept it, and withdraw it. While the route was eligible, the live
// forward of the withdrawal left it to this relay
// (validationSelection.section), so the store MUST name the key and answer it
// as no longer eligible.
func TestAnnouncedWithdrawalWakesTheRelay(t *testing.T) {
	r, bus := newPendingWithdrawalManager(t)

	retainedReceive(t, r, 40, rfc4271Announce(1, 0, 0, 0, 1, 24, 203, 0, 113))
	retainedDecision(t, r, "a", 1, 40)
	require.True(t, r.routeEligible(pendingWithdrawalKey, 0), "the accepted route is eligible")

	bus.routes = nil
	retainedReceive(t, r, 41, pendingWithdrawalBody)
	require.Contains(t, bus.routes, pendingWithdrawalKey, "the withdrawal wakes the relay for its key")
	require.False(t, r.routePresent(pendingWithdrawalKey), "the withdrawal removes the installed route")
	require.False(t, r.routeEligible(pendingWithdrawalKey, 0))
}
