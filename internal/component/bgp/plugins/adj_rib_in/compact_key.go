// Design: docs/architecture/plugin/rib-storage-design.md — compact keys for adj-rib-in maps
// RFC: rfc/short/rfc4364.md -- Section 4.1, the RD as part of a VPN route's identity
// RFC: rfc/short/rfc8277.md -- Section 2.4, the Compatibility field a withdrawal carries

package adj_rib_in

import (
	"encoding/hex"
	"errors"
	"net/netip"
	"unique"

	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// compactRouteKey is a value-type map key for ribIn seqmap entries, 48 bytes for
// every family. Building one for a plain prefix allocates nothing.
//
// A route is keyed by Prefix when its family names routes by a CIDR: the plain
// unicast and multicast families, and labeled unicast once its label stack is
// stripped. Every other family is keyed by NLRI, the family's registered route
// identity (nlrisplit.GetPrefixKey), and leaves Prefix zero. VPN is one of those,
// because its Route Distinguisher is part of what the route is: one prefix under
// two RDs is two routes.
//
// NLRI is an interned handle rather than a string so that the key of the
// commonest family does not grow: Fam and PathID share the eight bytes the
// alignment of Prefix would otherwise leave as padding, and the handle takes the
// eight a string would have doubled. The interned value lives while a key holds
// it, so a withdrawn route's identity is collected with its last key.
type compactRouteKey struct {
	Fam    family.Family
	PathID uint32
	Prefix netip.Prefix
	NLRI   unique.Handle[string]
}

// compactPendingKey is a value-type map key for pending/earlyDecisions.
// Replaces string pendingKey ("peerAddr|routeKey") with a comparable struct that
// costs no allocation to build.
type compactPendingKey struct {
	PeerAddr netip.Addr
	Route    compactRouteKey
}

var errRouteKey = errors.New("adj-rib-in: NLRI names no route of its family")

// keyedByNLRI is true when the key names its route by the family's route
// identity, not by Prefix. A zero handle is the guard: only routeKeyFromNLRI
// sets one, and it does so for exactly the families it does not key by CIDR.
func (k compactRouteKey) keyedByNLRI() bool {
	return k.NLRI != unique.Handle[string]{}
}

// routeName is the route a key names, for an operator: the prefix for a family
// keyed by Prefix, and the hex of the family's route identity for every other,
// whose zero Prefix would print as "invalid Prefix".
func (k compactRouteKey) routeName() string {
	if k.keyedByNLRI() {
		return hex.EncodeToString([]byte(k.NLRI.Value()))
	}
	return k.Prefix.String()
}

// routeKeyFromStrings constructs a compactRouteKey from parsed string values.
// Used on cold paths (command handlers) where string arguments are available.
func routeKeyFromStrings(fam family.Family, prefix string, pathID uint32) compactRouteKey {
	pfx, _ := netip.ParsePrefix(prefix)
	return compactRouteKey{Fam: fam, Prefix: pfx, PathID: pathID}
}

// routeKeyFromWire constructs a compactRouteKey from wire-parsed prefix.
func routeKeyFromWire(fam family.Family, prefix netip.Prefix, pathID uint32) compactRouteKey {
	return compactRouteKey{Fam: fam, Prefix: prefix, PathID: pathID}
}

// routeKeyFromNLRI keys one NLRI of a family whose encoding is not the plain
// [length][prefix] one, with the family's own decoder rather than by reading its
// octets as an address. payload is the NLRI without its ADD-PATH identifier, and
// withdraw says the family's withdrawal splitter framed it. Caller must hold r.mu:
// the decode writes r.keyScratch.
//
// The decoder is what keeps distinct routes apart: read as an address, the label
// octets of two labeled /24s are the same 48-bit "prefix", and so are those of two
// VPN routes, which then overwrite each other and are removed by each other's
// withdrawal.
//
// Interning an identity already held by a stored key allocates nothing, so an
// announcement that refreshes a route and a withdrawal that names one cost no
// allocation here. Only a new identity, or a withdrawal naming none, interns one.
func (r *AdjRIBInManager) routeKeyFromNLRI(fam family.Family, payload []byte, pathID uint32, withdraw bool) (compactRouteKey, error) {
	if nlrisplit.KeysByCIDR(fam) {
		// RFC 8277 Section 2.4: "Upon reception, the value of the Compatibility
		// field MUST be ignored." RouteCIDR drops it from a withdrawal, as it
		// drops the label stack from an announcement.
		cidr, err := nlrisplit.RouteCIDR(fam, payload, r.keyScratch[:], withdraw)
		if err != nil {
			return compactRouteKey{}, err
		}
		// WirePrefixToKey refuses a prefix longer than its octets, and PrefixFrom
		// leaves a prefix length beyond the address invalid: both name no route.
		prefix, ok := nlri.WirePrefixToKey(cidr, fam)
		if ok && prefix.IsValid() {
			return compactRouteKey{Fam: fam, PathID: pathID, Prefix: prefix}, nil
		}
		return compactRouteKey{}, errRouteKey
	}
	// RFC 4364 Section 4.1: "The purpose of the RD is solely to allow one to
	// create distinct routes to a common IPv4 address prefix." The identity the
	// family's key function returns keeps the RD, so it stays part of the key.
	identity, err := nlrisplit.GetPrefixKey(fam)(payload, r.keyScratch[:], withdraw)
	if err != nil {
		return compactRouteKey{}, err
	}
	return compactRouteKey{Fam: fam, PathID: pathID, NLRI: unique.Make(string(identity))}, nil
}
