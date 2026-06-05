// Design: docs/architecture/plugin/rib-storage-design.md — compact keys for adj-rib-in maps

package adj_rib_in

import (
	"net/netip"

	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

// compactRouteKey is a value-type map key for ribIn seqmap entries.
// Replaces string RouteKey ("family:prefix" or "family:prefix:pathID")
// with a zero-allocation comparable struct.
type compactRouteKey struct {
	Fam    family.Family
	Prefix netip.Prefix
	PathID uint32
}

// compactPendingKey is a value-type map key for pending/earlyDecisions.
// Replaces string pendingKey ("peerAddr|routeKey") with a zero-allocation
// comparable struct.
type compactPendingKey struct {
	PeerAddr netip.Addr
	Route    compactRouteKey
}

// routeKeyFromStrings constructs a compactRouteKey from parsed string values.
// Used on cold paths (command handlers) where string arguments are available.
func routeKeyFromStrings(fam family.Family, prefix string, pathID uint32) compactRouteKey {
	pfx, _ := netip.ParsePrefix(prefix)
	return compactRouteKey{Fam: fam, Prefix: pfx, PathID: pathID}
}

// pendingKeyFromStrings constructs a compactPendingKey from string values.
func pendingKeyFromStrings(peerAddr string, rk compactRouteKey) compactPendingKey {
	addr, _ := netip.ParseAddr(peerAddr)
	return compactPendingKey{PeerAddr: addr, Route: rk}
}

// routeKeyFromWire constructs a compactRouteKey from wire-parsed prefix.
func routeKeyFromWire(fam family.Family, prefix netip.Prefix, pathID uint32) compactRouteKey {
	return compactRouteKey{Fam: fam, Prefix: prefix, PathID: pathID}
}
