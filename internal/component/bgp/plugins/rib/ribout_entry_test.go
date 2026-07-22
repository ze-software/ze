// Design: docs/architecture/pool-architecture.md — Adj-RIB-Out compact storage tests

package rib

import (
	"encoding/binary"
	"net"
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/core/bgp/routeaction"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/rib/pool"
	"codeberg.org/thomas-mangin/ze/internal/core/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/core/family"
)

func TestPackEventAttrs_AllFields(t *testing.T) {
	med := uint32(100)
	lp := uint32(200)
	event := &Event{
		Origin:              "igp",
		ASPath:              []uint32{65001, 65002},
		MED:                 &med,
		LocalPreference:     &lp,
		Communities:         []string{"65000:100"},
		LargeCommunities:    []string{"65000:1:2"},
		ExtendedCommunities: []string{"0002000100000001"},
		FamilyOps: map[family.Family][]FamilyOperation{
			family.IPv4Unicast: {{
				NextHop: "10.0.0.1",
				Action:  routeaction.Add,
				NLRIs:   []any{"10.0.0.0/24"},
			}},
		},
	}

	packed := packEventAttrs(event, "10.0.0.1")
	require.NotEmpty(t, packed)

	handle, err := pool.RibOut.Intern(packed)
	require.NoError(t, err)
	defer func() { _ = pool.RibOut.Release(handle) }()

	entry := ribOutEntry{MsgID: 1, AttrHandle: handle}
	route := reconstructRoute(entry, family.IPv4Unicast, ribOutKey{Prefix: netip.MustParsePrefix("10.0.0.0/24")}, "")

	assert.Equal(t, "10.0.0.1", route.NextHop, "NextHop must survive round-trip")
	assert.NotNil(t, route.Origin, "Origin must survive round-trip")
	assert.Equal(t, attribute.Origin(0), *route.Origin)
	assert.Equal(t, []uint32{65001, 65002}, route.ASPath)
	require.NotNil(t, route.MED)
	assert.Equal(t, uint32(100), *route.MED)
	require.NotNil(t, route.LocalPreference)
	assert.Equal(t, uint32(200), *route.LocalPreference)
	assert.Len(t, route.Communities, 1)
	assert.Len(t, route.LargeCommunities, 1)
	assert.Equal(t, attribute.LargeCommunity{GlobalAdmin: 65000, LocalData1: 1, LocalData2: 2}, route.LargeCommunities[0])
	assert.Len(t, route.ExtendedCommunities, 1)
}

func TestPackEventAttrs_ASPathOver255(t *testing.T) {
	asns := make([]uint32, 300)
	for i := range asns {
		asns[i] = uint32(65000 + i)
	}

	event := &Event{
		Origin: "igp",
		ASPath: asns,
	}

	packed := packEventAttrs(event, "")
	require.NotEmpty(t, packed)

	handle, err := pool.RibOut.Intern(packed)
	require.NoError(t, err)
	defer func() { _ = pool.RibOut.Release(handle) }()

	entry := ribOutEntry{MsgID: 1, AttrHandle: handle}
	route := reconstructRoute(entry, family.IPv4Unicast, ribOutKey{Prefix: netip.MustParsePrefix("10.0.0.0/24")}, "")

	assert.Len(t, route.ASPath, 300, "all 300 ASNs must survive multi-segment encoding")
	for i, asn := range route.ASPath {
		assert.Equal(t, uint32(65000+i), asn, "ASN at index %d", i)
	}
}

func TestParseOutRouteKey(t *testing.T) {
	tests := []struct {
		key    string
		prefix string
		pathID uint32
	}{
		{"10.0.0.0/24", "10.0.0.0/24", 0},
		{"10.0.0.0/24:42", "10.0.0.0/24", 42},
		{"2001:db8::/32", "2001:db8::/32", 0},
		{"2001:db8::/32:7", "2001:db8::/32", 7},
	}
	for _, tt := range tests {
		prefix, pathID := parseOutRouteKey(tt.key)
		assert.Equal(t, tt.prefix, prefix, "key=%q", tt.key)
		assert.Equal(t, tt.pathID, pathID, "key=%q", tt.key)
	}
}

func TestRibOutSourceRefCount(t *testing.T) {
	r := newTestRIBManager(t)

	fam := family.IPv4Unicast
	key := ribOutKey{Prefix: netip.MustParsePrefix("10.0.0.0/24")}

	r.setRibOutSource(fam, key, "src-peer", true)
	assert.Equal(t, "src-peer", r.ribOutSourcePeer(fam, key))

	r.setRibOutSource(fam, key, "src-peer", true)
	r.setRibOutSource(fam, key, "src-peer", true)
	// refCount is now 3

	r.releaseRibOutSource(fam, key)
	assert.Equal(t, "src-peer", r.ribOutSourcePeer(fam, key), "source should survive 2 remaining refs")

	r.releaseRibOutSource(fam, key)
	assert.Equal(t, "src-peer", r.ribOutSourcePeer(fam, key), "source should survive 1 remaining ref")

	r.releaseRibOutSource(fam, key)
	assert.Equal(t, "", r.ribOutSourcePeer(fam, key), "source should be gone at refCount 0")
}

func TestRibOutSourceRefCount_ReannounceNoDouble(t *testing.T) {
	r := newTestRIBManager(t)

	fam := family.IPv4Unicast
	key := ribOutKey{Prefix: netip.MustParsePrefix("10.0.0.0/24")}

	// First announcement: new entry
	r.setRibOutSource(fam, key, "src-peer", true)
	// Re-announcement: existing entry, isNew=false
	r.setRibOutSource(fam, key, "src-peer", false)

	// refCount should be 1, not 2
	r.releaseRibOutSource(fam, key)
	assert.Equal(t, "", r.ribOutSourcePeer(fam, key), "re-announce must not double-count")
}

func TestReconstructRoute_InvalidHandle(t *testing.T) {
	entry := ribOutEntry{MsgID: 42}
	route := reconstructRoute(entry, family.IPv4Unicast, ribOutKey{Prefix: netip.MustParsePrefix("10.0.0.0/24")}, "src")

	assert.Equal(t, uint64(42), route.MsgID)
	assert.Equal(t, "10.0.0.0/24", route.Prefix)
	assert.Equal(t, "src", route.SourcePeer)
	assert.Empty(t, route.NextHop, "no wire bytes means no NextHop")
}

func TestParseASPathWire_MultiSegment(t *testing.T) {
	// 2 segments: AS_SEQUENCE(2 ASNs) + AS_SET(1 ASN)
	var buf []byte
	// Segment 1: AS_SEQUENCE with 2 ASNs
	buf = append(buf, 2, 2) // type=SEQUENCE, count=2
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, 65001)
	buf = append(buf, b...)
	binary.BigEndian.PutUint32(b, 65002)
	buf = append(buf, b...)
	// Segment 2: AS_SET with 1 ASN
	buf = append(buf, 1, 1) // type=SET, count=1
	binary.BigEndian.PutUint32(b, 65003)
	buf = append(buf, b...)

	result := parseASPathWire(buf)
	// All segments flattened into single slice
	assert.Equal(t, []uint32{65001, 65002, 65003}, result)
}

// packEventAttrs only encodes NEXT_HOP (type 3) for IPv4. IPv6 next-hops
// live in MP_REACH_NLRI which packEventAttrs does not build. This is
// acceptable: the JSON event path without raw-attributes is external-plugin
// only, and IPv6 families always have raw wire bytes in the structured path.
func TestPackEventAttrs_IPv6NextHopNotPacked(t *testing.T) {
	event := &Event{
		Origin: "igp",
		FamilyOps: map[family.Family][]FamilyOperation{
			family.IPv6Unicast: {{
				NextHop: "2001:db8::1",
				Action:  routeaction.Add,
				NLRIs:   []any{"2001:db8::/32"},
			}},
		},
	}

	packed := packEventAttrs(event, "2001:db8::1")
	handle, err := pool.RibOut.Intern(packed)
	require.NoError(t, err)
	defer func() { _ = pool.RibOut.Release(handle) }()

	entry := ribOutEntry{MsgID: 1, AttrHandle: handle}
	route := reconstructRoute(entry, family.IPv6Unicast, ribOutKey{Prefix: netip.MustParsePrefix("2001:db8::/32")}, "")

	assert.Empty(t, route.NextHop, "IPv6 next-hop not encoded by packEventAttrs (requires MP_REACH)")
}

func TestExtractNextHopFromMPReach_IPv6(t *testing.T) {
	// AFI(2) + SAFI(1) + NH-Len(1) + NextHop(16) + Reserved(1)
	value := make([]byte, 4+16+1)
	binary.BigEndian.PutUint16(value[0:], 2) // AFI IPv6
	value[2] = 1                             // SAFI unicast
	value[3] = 16                            // NH length
	copy(value[4:], net.ParseIP("2001:db8::1").To16())
	value[20] = 0 // reserved

	nh := extractNextHopFromMPReach(value)
	assert.Equal(t, "2001:db8::1", nh)
}
