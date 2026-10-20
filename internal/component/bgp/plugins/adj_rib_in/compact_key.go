// Design: docs/architecture/plugin/rib-storage-design.md — compact keys for adj-rib-in maps

package adj_rib_in

import (
	"encoding/hex"
	"errors"
	"net/netip"

	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// compactRouteKey is a value-type map key for ribIn seqmap entries.
// Replaces string RouteKey ("family:prefix" or "family:prefix:pathID")
// with a zero-allocation comparable struct.
//
// A route is keyed by Prefix when its family names routes by a CIDR: the plain
// unicast and multicast families, and labeled unicast once its label stack is
// stripped. Every other family is keyed by NLRI, the family's registered route
// identity (nlrisplit.GetPrefixKey), and leaves Prefix zero. VPN is one of those,
// because its Route Distinguisher is part of what the route is: one prefix under
// two RDs is two routes.
type compactRouteKey struct {
	Fam    family.Family
	Prefix netip.Prefix
	PathID uint32
	NLRI   string
}

// compactPendingKey is a value-type map key for pending/earlyDecisions.
// Replaces string pendingKey ("peerAddr|routeKey") with a zero-allocation
// comparable struct.
type compactPendingKey struct {
	PeerAddr netip.Addr
	Route    compactRouteKey
}

var errRouteKey = errors.New("adj-rib-in: NLRI names no route of its family")

// routeName is the route a key names, for an operator: the prefix for a family
// keyed by Prefix, and the hex of the family's route identity for every other,
// whose zero Prefix would print as "invalid Prefix".
func (k compactRouteKey) routeName() string {
	if k.NLRI != "" {
		return hex.EncodeToString([]byte(k.NLRI))
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
// withdraw says the family's withdrawal splitter framed it.
//
// The decoder is what keeps distinct routes apart: read as an address, the label
// octets of two labeled /24s are the same 48-bit "prefix", and so are those of two
// VPN routes, which then overwrite each other and are removed by each other's
// withdrawal.
func routeKeyFromNLRI(fam family.Family, payload []byte, pathID uint32, withdraw bool) (compactRouteKey, error) {
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	if nlrisplit.KeysByCIDR(fam) {
		// RFC 8277 Section 2.4: "Upon reception, the value of the Compatibility
		// field MUST be ignored." RouteCIDR drops it from a withdrawal, as it
		// drops the label stack from an announcement.
		cidr, err := nlrisplit.RouteCIDR(fam, payload, scratch[:], withdraw)
		if err != nil {
			return compactRouteKey{}, err
		}
		prefix, ok := nlri.WirePrefixToKey(cidr, fam)
		if !ok {
			return compactRouteKey{}, errRouteKey
		}
		if !prefix.IsValid() {
			return compactRouteKey{}, errRouteKey
		}
		return compactRouteKey{Fam: fam, Prefix: prefix, PathID: pathID}, nil
	}
	identity, err := nlrisplit.GetPrefixKey(fam)(payload, scratch[:], withdraw)
	if err != nil {
		return compactRouteKey{}, err
	}
	return compactRouteKey{Fam: fam, PathID: pathID, NLRI: string(identity)}, nil
}
