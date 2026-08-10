// RFC: rfc/short/rfc7752.md — BGP-LS routing universes (§3.2)
//
// RFC 7752 Section 3.2 makes the 64-bit Identifier the routing-universe
// discriminator: NLRIs carrying different Identifier values describe different
// universes and must not be conflated. Ze stores BGP-LS in the non-CIDR opaque
// backend keyed by the full NLRI wire bytes (familyrib.go:278), so the
// Identifier is part of the key by construction.

package storage

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/core/family"
)

// bgpLSNodeNLRI builds a Node NLRI (type 1) for protocol IS-IS L2 with the
// given Identifier and a single AS node descriptor.
func bgpLSNodeNLRI(identifier uint64, asn uint32) []byte {
	nlri := make([]byte, 4+9+4+8)
	binary.BigEndian.PutUint16(nlri[0:], 1)  // NLRI type: Node
	binary.BigEndian.PutUint16(nlri[2:], 21) // total NLRI length
	nlri[4] = 2                              // Protocol-ID: IS-IS Level 2
	binary.BigEndian.PutUint64(nlri[5:], identifier)
	binary.BigEndian.PutUint16(nlri[13:], 256) // Local Node Descriptors
	binary.BigEndian.PutUint16(nlri[15:], 8)
	binary.BigEndian.PutUint16(nlri[17:], 512) // Autonomous System sub-TLV
	binary.BigEndian.PutUint16(nlri[19:], 4)
	binary.BigEndian.PutUint32(nlri[21:], asn)
	return nlri
}

// TestRFC7752DifferentIdentifierIsDifferentRoutingUniverse proves two BGP-LS
// NLRIs that agree on every descriptor but carry different Identifier values
// occupy separate RIB entries, which is how ze keeps routing universes apart.
//
// VALIDATES: insertOpaque keys BGP-LS routes by the full wire bytes, Identifier
// included (familyrib.go:278).
// PREVENTS: a key that skips the Identifier merging two universes into one
// route and losing one topology.
func TestRFC7752DifferentIdentifierIsDifferentRoutingUniverse(t *testing.T) {
	// RFC requirement: RFC7752-3.2-4 positive -- NLRIs whose Identifier values differ are held as separate routes, so they describe separate routing universes (§3.2)
	fam := family.Family{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkState}
	rib := newFamilyRIB(fam, false)
	defer rib.Release()

	attrs := concat(wireOriginIGP, wireASPath65001, wireNextHop)
	universe0 := bgpLSNodeNLRI(0, 65001)
	universe1 := bgpLSNodeNLRI(1, 65001)

	rib.Insert(attrs, universe0, true)
	rib.Insert(attrs, universe1, true)

	assert.Equal(t, 2, rib.Len(), "distinct Identifier values yield distinct routes")
	_, ok := rib.lookupEntry(universe0)
	assert.True(t, ok, "routing universe 0 is retrievable")
	_, ok = rib.lookupEntry(universe1)
	assert.True(t, ok, "routing universe 1 is retrievable")

	// Withdrawing one universe leaves the other intact.
	assert.True(t, rib.Remove(universe1))
	assert.Equal(t, 1, rib.Len())
	_, ok = rib.lookupEntry(universe0)
	assert.True(t, ok, "removing universe 1 does not disturb universe 0")
}

// TestRFC7752SameIdentifierIsOneRoutingUniverse is the counter-case: NLRIs that
// agree on the Identifier are the same object in the same universe, so a repeat
// advertisement is an implicit update rather than a second route.
//
// VALIDATES: the opaque key is exactly the NLRI bytes, so equal bytes collapse.
// PREVENTS: an over-eager key (peer, session, arrival order) splitting one
// universe into several.
func TestRFC7752SameIdentifierIsOneRoutingUniverse(t *testing.T) {
	// RFC requirement: RFC7752-3.2-4 negative -- NLRIs sharing an Identifier are NOT treated as separate routing universes; the second advertisement replaces the first (§3.2)
	fam := family.Family{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkState}
	rib := newFamilyRIB(fam, false)
	defer rib.Release()

	attrs := concat(wireOriginIGP, wireASPath65001, wireNextHop)
	nlri := bgpLSNodeNLRI(7, 65001)

	rib.Insert(attrs, nlri, true)
	rib.Insert(attrs, nlri, true)

	assert.Equal(t, 1, rib.Len(), "the same Identifier is one routing universe")

	// A descriptor change with the same Identifier is still a different node in
	// that same universe, so it is a different route.
	rib.Insert(attrs, bgpLSNodeNLRI(7, 65002), true)
	assert.Equal(t, 2, rib.Len(), "same universe, different node descriptor")
}
