// Design: docs/architecture/plugin/rib-storage-design.md -- receive validation gate
// Related: rib_eligibility.go -- noteValidationChange, validationRouteKey
// RFC: rfc/short/rfc8212.md -- Section 3 default import rejection
//
// VALIDATES: a ValidationChange for a route of a family keyed by its NLRI
// identity (VPN here) carries the route's native NLRI, so the key a subscriber
// receives resolves, through the selection query, to the route the store holds.
// PREVENTS: an eligibility change of such a route announced under a key with no
// NLRI, which no subscriber can look up, so dependent selection never
// re-evaluates the route.
package adj_rib_in

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// nlriKeyVPN is IPv4 VPN, a family the store keys by NLRI identity.
var nlriKeyVPN = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}

// nlriKeyVPNWire is one VPN NLRI: 112 bits, label 1 with bottom of stack,
// RD 0:65000:1, 10.1.0.0/24.
var nlriKeyVPNWire = []byte{0x70, 0x00, 0x00, 0x11, 0x00, 0x00, 0xfd, 0xe8, 0x00, 0x00, 0x00, 0x01, 0x0a, 0x01, 0x00}

// nlriKeyVPNWithdrawal withdraws nlriKeyVPNWire: the RFC 8277 Section 2.4
// Compatibility field 0x800000 in place of the label stack.
var nlriKeyVPNWithdrawal = []byte{0x70, 0x80, 0x00, 0x00, 0x00, 0x00, 0xfd, 0xe8, 0x00, 0x00, 0x00, 0x01, 0x0a, 0x01, 0x00}

// storeNLRIKeyVPN stores nlriKeyVPNWire for source the way a received UPDATE
// does, admitted at policy epoch 5, and returns its store key.
func storeNLRIKeyVPN(t *testing.T, r *AdjRIBInManager, source netip.Addr) compactRouteKey {
	t.Helper()
	r.mu.Lock()
	r.installComplexNLRIs(source, nlriKeyVPN, nlriKeyVPNWire, false, "40010100", "0a000001", 100, 5)
	key, err := r.routeKeyFromNLRI(nlriKeyVPN, nlriKeyVPNWire, 0, false)
	r.mu.Unlock()
	require.NoError(t, err)
	_, stored := r.ribIn[source].Get(key)
	require.True(t, stored, "the VPN route is stored under its NLRI identity")
	r.peerUp[source] = true
	return key
}

// emittedFor returns the notified keys of family fam, and fails when none was.
func emittedFor(t *testing.T, bus *notifyTestBus, fam family.Family) []ribevents.ValidationRoute {
	t.Helper()
	var routes []ribevents.ValidationRoute
	for _, route := range bus.routes {
		if route.Family == fam {
			routes = append(routes, route)
		}
	}
	require.NotEmpty(t, routes, "a ValidationChange names the %s route", fam)
	return routes
}

// requireResolves fails unless every notified key carries the route's native
// NLRI and names, through the selection query's own key function, key.
func requireResolves(t *testing.T, routes []ribevents.ValidationRoute, key compactRouteKey) {
	t.Helper()
	for _, route := range routes {
		require.Equal(t, string(nlriKeyVPNWire), route.NLRI, "the key carries the route's native NLRI")
		resolved, named := validationRouteKey(route)
		require.True(t, named, "the notified key names a route")
		require.Equal(t, key, resolved, "the notified key resolves to the stored route")
	}
}

// nlriKeySourcePolicy runs one source-policy operation for source.
func nlriKeySourcePolicy(t *testing.T, r *AdjRIBInManager, op rpc.SourcePolicyOperation, source netip.Addr, epoch, next uint64) {
	t.Helper()
	input := rpc.SourcePolicyInput{TransactionID: "nlri-key", Operation: op,
		Source: rpc.SourcePolicyIdentity{Peer: source.String(), PeerOwner: 1, SessionGeneration: 3,
			PolicyEpoch: epoch, ReceiveFrontier: 9, TemplateGeneration: 1},
		NextPolicyEpoch: next, DeadlineMS: time.Now().Add(time.Minute).UnixMilli()}
	require.NoError(t, input.Validate(time.Now()))
	require.NoError(t, r.handleSourcePolicy(context.Background(), input))
}

// TestValidationChangeVPNSourcePolicyKeyResolves proves an RFC 8212 denial
// and its abort reach the subscriber under a key that finds the VPN route.
// Method: store a VPN route, apply a source-policy denial, then abort it; after
// each step the notified key must resolve to the stored route, and the
// selection query asked with that very key must answer the new eligibility.
func TestValidationChangeVPNSourcePolicyKeyResolves(t *testing.T) {
	r, bus := newPendingWithdrawalManager(t)
	source := netip.MustParseAddr("10.0.0.1")
	key := storeNLRIKeyVPN(t, r, source)

	nlriKeySourcePolicy(t, r, rpc.SourcePolicyPrepare, source, 5, 0)
	bus.routes = nil
	nlriKeySourcePolicy(t, r, rpc.SourcePolicyApply, source, 5, 6)
	denied := emittedFor(t, bus, nlriKeyVPN)
	requireResolves(t, denied, key)
	require.True(t, r.routePresent(denied[0]), "the denied route stays stored for abort")
	require.False(t, r.routeEligible(denied[0], 0), "the subscriber reads the denial")

	bus.routes = nil
	nlriKeySourcePolicy(t, r, rpc.SourcePolicyAbort, source, 5, 7)
	restored := emittedFor(t, bus, nlriKeyVPN)
	requireResolves(t, restored, key)
	require.True(t, r.routeEligible(restored[0], 0), "the subscriber reads the restored route")
}

// TestValidationChangeVPNWithdrawalKeyResolves proves the withdrawal of a VPN
// route and the end of its source's session name the route they removed.
// Method: store the route, withdraw it with RFC 8277 Compatibility framing and
// check the key; store it again and drop the source's routes as a session end
// does, and check that key too. Each key must have named the stored route.
func TestValidationChangeVPNWithdrawalKeyResolves(t *testing.T) {
	r, bus := newPendingWithdrawalManager(t)
	source := netip.MustParseAddr("10.0.0.1")
	key := storeNLRIKeyVPN(t, r, source)

	bus.routes = nil
	r.mu.Lock()
	r.removeComplexNLRIs(source, nlriKeyVPN, nlriKeyVPNWithdrawal, false)
	r.unlockValidation()
	withdrawn := emittedFor(t, bus, nlriKeyVPN)
	requireResolves(t, withdrawn, key)
	require.False(t, r.routePresent(withdrawn[0]), "the withdrawn route is gone")

	key = storeNLRIKeyVPN(t, r, source)
	bus.routes = nil
	r.mu.Lock()
	r.removePeerInstalled(source)
	r.unlockValidation()
	dropped := emittedFor(t, bus, nlriKeyVPN)
	requireResolves(t, dropped, key)
	require.False(t, r.routePresent(dropped[0]), "the session end removed the route")
}
