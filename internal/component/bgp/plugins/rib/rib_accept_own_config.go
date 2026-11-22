// Design: docs/architecture/route-selection.md -- RFC 7611 ACCEPT_OWN configuration and VPN-IP decision step
// RFC: rfc/short/rfc7611.md -- Section 2.3 (default disabled) and Section 3 (VPN-IP decision process)
// Related: bestpath.go -- compareBeforeMED, comparePairWithReason, multipathEqual
// Related: config_tx.go -- bgp/accept-own config transaction installation

package rib

import (
	"encoding/binary"
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
)

// extractAcceptOwnConfig reads the bgp/accept-own boolean leaf from a parsed
// bgp config subtree.
//
// RFC 7611 Section 2.3: "ACCEPT_OWN handling SHOULD be controlled by
// configuration, and if controlled by configuration, it MUST default to being
// disabled."
func extractAcceptOwnConfig(bgp map[string]any) (bool, error) {
	if bgp == nil {
		return false, nil
	}
	value, present := bgp["accept-own"]
	if !present {
		return false, nil
	}
	switch v := value.(type) {
	case nil:
		return false, nil
	case bool:
		return v, nil
	case string:
		if v == "true" {
			return true, nil
		}
		if v == "false" {
			return false, nil
		}
		return false, fmt.Errorf("bgp accept-own %q is not a boolean: "+
			"RFC 7611 Section 2.3 requires a boolean toggle defaulting to false; "+
			"fix: set bgp accept-own true (or false)", v)
	default:
		return false, fmt.Errorf("bgp accept-own must be a boolean, got %T: "+
			"RFC 7611 Section 2.3 requires a boolean toggle defaulting to false; "+
			"fix: set bgp accept-own true (or false)", value)
	}
}

// entryHasAcceptOwn reports whether b carries the RFC 7611 ACCEPT_OWN
// well-known community (0xFFFF0001).
func entryHasAcceptOwn(b storage.Bundle) bool {
	if !b.HasCommunities() {
		return false
	}
	data, err := pool.Communities.Get(b.Communities)
	if err != nil {
		panic("BUG: rib: valid communities handle missing from pool: " + err.Error())
	}
	for off := 0; off+4 <= len(data); off += 4 {
		if binary.BigEndian.Uint32(data[off:off+4]) == uint32(attribute.CommunityAcceptOwn) {
			return true
		}
	}
	return false
}

// isVPNIPFamily reports whether fam is an RFC 4364 VPN-IP address family
// (IPv4/VPN or IPv6/VPN, SAFI 128).
func isVPNIPFamily(fam family.Family) bool {
	if fam.SAFI != family.SAFIVPN {
		return false
	}
	if fam.AFI == family.AFIIPv4 {
		return true
	}
	return fam.AFI == family.AFIIPv6
}

// compareAcceptOwn enforces RFC 7611 Section 3: "If a BGP speaker supports
// ACCEPT_OWN and is configured for the extensions defined in this document,
// the following step is inserted after the LOCAL_PREF comparison step in the
// BGP decision process: When comparing a pair of routes for a BGP destination,
// the route with the ACCEPT_OWN community attached is preferred over the route
// that does not have the community. In all other respects, the decision process
// remains unchanged. This extra step MUST only be invoked during the best path
// selection process of VPN-IP routes [RFC4364] (i.e., it MUST NOT be invoked
// for the best path selection of imported IP routes in a VRF)."
func compareAcceptOwn(a, b *Candidate) int {
	if !a.AcceptOwnEnabled {
		return 0
	}
	if !b.AcceptOwnEnabled {
		return 0
	}
	if !isVPNIPFamily(a.Family) {
		return 0
	}
	if !isVPNIPFamily(b.Family) {
		return 0
	}
	if a.HasAcceptOwn == b.HasAcceptOwn {
		return 0
	}
	if a.HasAcceptOwn {
		return -1
	}
	return 1
}
