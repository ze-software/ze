package rs

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestRouteServerDeclaresPeerUpBarrier asserts this plugin's registration keeps
// the peer-up barrier declaration.
//
// VALIDATES: registry.RequiresPeerUpBarrier("bgp-rs") is true once the package
// init() has registered the plugin.
// PREVENTS: the declaration being dropped, which would silently restore the
// window it closes. handleState is what makes a peer a live forward target (Up
// plus the peer-up cut, one critical section in server_handlers.go); an UPDATE
// the engine takes delivery of before that lands at or below the peer's cut and
// is therefore left to the announce-only Adj-RIB-In replay, so its withdrawals
// never reach the peer. The declaration is what holds that peer's initial-sync
// End-of-RIB until this plugin has processed the peer-up event, which is what
// lets a peer treat the End-of-RIB as "I am a registered forward target".
func TestRouteServerDeclaresPeerUpBarrier(t *testing.T) {
	if !registry.RequiresPeerUpBarrier("bgp-rs") {
		t.Fatal("bgp-rs must declare PeerUpBarrier: without it the engine does not hold a " +
			"peer's end-of-rib until this plugin has registered the peer as a forward target")
	}
}

// TestRouteServerClaimsPeerUpReplay pins the sibling declaration in the same
// registration, so a future edit to the Registration literal cannot quietly
// drop one while keeping the other.
//
// VALIDATES: bgp-rs declares the peer-up replay claim.
// PREVENTS: bgp-adj-rib-in self-replaying alongside this plugin's explicit
// replay, which announced a route learned just before establishment twice.
func TestRouteServerClaimsPeerUpReplay(t *testing.T) {
	claims := registry.ClaimsFor("bgp-rs")
	if !slices.Contains(claims, ClaimPeerUpReplay) {
		t.Fatalf("bgp-rs must claim %q; claims are %v", ClaimPeerUpReplay, claims)
	}
}
