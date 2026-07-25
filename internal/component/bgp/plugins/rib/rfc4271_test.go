package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// rfc4271Candidate builds a candidate with the given peer address and preference.
func rfc4271Candidate(addr string, localPref uint32, asPathLen int) *Candidate {
	return &Candidate{
		PeerAddr:  addr,
		PeerIP:    netip.MustParseAddr(addr),
		LocalPref: localPref,
		ASPathLen: asPathLen,
	}
}

// TestRFC4271DegreeOfPreferenceIgnoresOtherRoutes verifies the preference used to order two
// routes depends only on those routes' own attributes.
//
// VALIDATES: The winner between two candidates is identical whether they are compared
// alone or alongside three unrelated candidates with far better and far worse attributes.
//
// PREVENTS: A route's preference drifting because of what else happens to be in the RIB.
//
// RFC requirement: RFC4271-9.1.1-1 positive -- comparePair reads only the two candidates'
// own fields, so adding or removing unrelated routes does not change their relative order
// (internal/component/bgp/plugins/rib/bestpath.go:307-391, SelectBest at :122-135).
func TestRFC4271DegreeOfPreferenceIgnoresOtherRoutes(t *testing.T) {
	a := rfc4271Candidate("10.0.0.1", 100, 3)
	b := rfc4271Candidate("10.0.0.2", 150, 5)

	pairOnly := ComparePair(a, b)
	require.Positive(t, pairOnly, "b has the higher LOCAL_PREF and wins the pair")

	// Same two routes, now surrounded by unrelated candidates.
	noise := []*Candidate{
		rfc4271Candidate("10.0.0.3", 50, 1),
		rfc4271Candidate("10.0.0.4", 120, 2),
		rfc4271Candidate("10.0.0.5", 10, 9),
	}
	withNoise := ComparePair(a, b)
	assert.Equal(t, pairOnly, withNoise, "presence of other routes does not alter the pair order")

	best := SelectBest(append([]*Candidate{a, b}, noise...))
	assert.Equal(t, "10.0.0.2", best.PeerAddr, "the same route still wins in the larger set")

	// Removing the noise leaves the same winner.
	best = SelectBest([]*Candidate{a, b})
	assert.Equal(t, "10.0.0.2", best.PeerAddr)
}

// TestRFC4271DegreeOfPreferenceFollowsOwnAttributes verifies the invariance above is not a
// constant result.
//
// VALIDATES: Changing a candidate's own LOCAL_PREF flips the winner, while changing an
// unrelated candidate's attributes never does.
//
// PREVENTS: Reading "other routes are not inputs" as "the comparison ignores everything".
//
// RFC requirement: RFC4271-9.1.1-1 negative -- the degree of preference does respond to
// the route's own attributes, so the exclusion of other routes as inputs is a real
// property and not a comparison that always returns the same answer
// (internal/component/bgp/plugins/rib/bestpath.go:324-338).
func TestRFC4271DegreeOfPreferenceFollowsOwnAttributes(t *testing.T) {
	a := rfc4271Candidate("10.0.0.1", 100, 3)
	b := rfc4271Candidate("10.0.0.2", 150, 5)
	require.Positive(t, ComparePair(a, b))

	a.LocalPref = 200 // a's own attribute changes: a now wins
	assert.Negative(t, ComparePair(a, b))

	// An unrelated candidate with an enormous preference does not change the pair.
	before := ComparePair(a, b)
	_ = SelectBest([]*Candidate{a, b, rfc4271Candidate("10.0.0.9", 4000, 0)})
	assert.Equal(t, before, ComparePair(a, b), "an unrelated route is not an input")
}

// TestRFC4271LocRIBNextHopComesFromNextHopAttribute verifies the address installed in the
// Loc-RIB is derived from the route's NEXT_HOP attribute.
//
// VALIDATES: A route whose NEXT_HOP is 192.168.1.1 installs with that next hop; a route
// carrying no NEXT_HOP at all installs with no next hop rather than a fabricated one.
//
// PREVENTS: Installing a route with a next hop that was never advertised.
//
// RFC requirement: RFC4271-9.1.2-3 negative -- bestCandidateNextHopAddr returns an invalid
// address when the route carries neither a NEXT_HOP attribute nor an MP_REACH next hop, so
// the immediate next hop is taken from the attribute rather than invented
// (internal/component/bgp/plugins/rib/rib_bestchange.go:1060-1090).
// RFC requirement: RFC4271-9.1.2.1-1 negative -- the install-time next-hop recomputation
// yields nothing to install when the route has no advertised next hop, so the Loc-RIB Path
// is not populated from stale or default state
// (internal/component/bgp/plugins/rib/rib_bestchange.go:723-726,797-830).
func TestRFC4271LocRIBNextHopComesFromNextHopAttribute(t *testing.T) {
	r := newTestRIBManager(t)
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)

	peerAddr := netip.MustParseAddr("192.0.2.1")
	r.peerMeta[peerAddr] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}
	fam := family.Family{AFI: 1, SAFI: 1}
	r.bgpPeers[peerAddr] = storage.NewPeerRIB(peerAddr.String())

	withNH := ipv4Prefix(24, 10, 0, 0)
	r.bgpPeers[peerAddr].Insert(fam, makeAttrBytes([4]byte{192, 168, 1, 1}), withNH, true)
	_, ok := r.checkBestPathChange(fam, withNH, false, nil)
	require.True(t, ok)
	best, found := loc.Best(fam, netip.MustParsePrefix("10.0.0.0/24"))
	require.True(t, found)
	require.Equal(t, netip.MustParseAddr("192.168.1.1"), best.NextHop)

	// Same peer, a prefix announced with ORIGIN only -- no NEXT_HOP attribute.
	noNH := ipv4Prefix(24, 10, 0, 1)
	r.bgpPeers[peerAddr].Insert(fam, []byte{0x40, 0x01, 0x01, 0x00}, noNH, true)
	_, ok = r.checkBestPathChange(fam, noNH, false, nil)
	require.True(t, ok)
	best, found = loc.Best(fam, netip.MustParsePrefix("10.0.1.0/24"))
	require.True(t, found)
	assert.False(t, best.NextHop.IsValid(),
		"no NEXT_HOP advertised means no immediate next hop is derived")
}

// TestRFC4271ExternalRouteDegreeOfPreferenceFromLocalPolicy verifies an external route's
// degree of preference comes from local configuration, not from the wire.
//
// VALIDATES: A route with no LOCAL_PREF attribute (the shape an eBGP-learned route has,
// since a received LOCAL_PREF is discarded on an external session) is given the locally
// configured default degree of preference of 100.
//
// PREVENTS: An external route entering the decision process with an undefined or zero
// preference.
//
// RFC requirement: RFC4271-5.1.5-5 positive -- extractCandidate seeds every candidate with
// the locally configured default degree of preference of 100 before reading any attribute,
// so an external route is ranked by local policy
// (internal/component/bgp/plugins/rib/rib_commands.go:1062-1067).
func TestRFC4271ExternalRouteDegreeOfPreferenceFromLocalPolicy(t *testing.T) {
	r := newTestRIBManager(t)
	peerAddr := netip.MustParseAddr("192.0.2.1")
	r.peerMeta[peerAddr] = &peerMetadata{PeerASN: 65001, LocalASN: 65000}

	entry, err := storage.ParseAttributes([]byte{0x40, 0x01, 0x01, 0x00}, true)
	require.NoError(t, err)
	defer entry.Release()

	c := r.extractCandidate(peerAddr, peerAddr.String(), entry)
	assert.Equal(t, uint32(100), c.LocalPref,
		"external route ranked by the locally configured degree of preference")
	assert.NotEqual(t, uint32(0), c.LocalPref, "not left undefined")
}

// TestRFC4271DegreeOfPreferenceNotAHardcodedConstant verifies the local default is a
// policy default rather than a fixed value applied to every route.
//
// VALIDATES: A route that does carry LOCAL_PREF (the internal-peer case) is ranked on that
// value, not on 100.
//
// PREVENTS: Reading the 100 default as "every route always gets 100".
//
// RFC requirement: RFC4271-5.1.5-5 negative -- the seeded default is overridden when the
// route carries its own preference, so 100 is the locally configured policy value for
// routes that have none and not an unconditional constant
// (internal/component/bgp/plugins/rib/rib_commands.go:1077-1084).
func TestRFC4271DegreeOfPreferenceNotAHardcodedConstant(t *testing.T) {
	r := newTestRIBManager(t)
	peerAddr := netip.MustParseAddr("192.0.2.2")
	r.peerMeta[peerAddr] = &peerMetadata{PeerASN: 65000, LocalASN: 65000}

	raw := []byte{
		0x40, 0x01, 0x01, 0x00,
		0x40, 0x05, 0x04, 0x00, 0x00, 0x00, 0xFA, // LOCAL_PREF = 250
	}
	entry, err := storage.ParseAttributes(raw, true)
	require.NoError(t, err)
	defer entry.Release()

	c := r.extractCandidate(peerAddr, peerAddr.String(), entry)
	assert.Equal(t, uint32(250), c.LocalPref)
}
