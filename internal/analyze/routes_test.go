package analyze

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/mrt"
)

// ribMPReach builds the abbreviated MP_REACH_NLRI that a TABLE_DUMP_V2 RIB
// entry carries: Next Hop Length + Next Hop only (RFC 6396 Section 4.3.4).
func ribMPReach(nextHop []byte) mrt.PathAttribute {
	v := make([]byte, 1+len(nextHop))
	v[0] = byte(len(nextHop))
	copy(v[1:], nextHop)
	return mrt.PathAttribute{Flags: 0x80, Code: mrt.AttrMPReachNLRI, Value: v}
}

func asPathAttr(fourByte bool, asns ...uint32) mrt.PathAttribute {
	width := 2
	if fourByte {
		width = 4
	}
	v := make([]byte, 2+len(asns)*width)
	v[0] = 2 // AS_SEQUENCE
	v[1] = byte(len(asns))
	for i, asn := range asns {
		off := 2 + i*width
		if fourByte {
			binary.BigEndian.PutUint32(v[off:], asn)
			continue
		}
		binary.BigEndian.PutUint16(v[off:], uint16(asn))
	}
	return mrt.PathAttribute{Flags: 0x40, Code: mrt.AttrASPath, Value: v}
}

func TestBuildRouteRecord_IPv6NextHopFromTruncatedMPReach(t *testing.T) {
	// VALIDATES: `ze-analyze routes` emits the correct next-hop for an IPv6
	// TABLE_DUMP_V2 RIB entry, whose MP_REACH_NLRI is abbreviated to
	// Next Hop Length + Next Hop (RFC 6396 Section 4.3.4).
	// PREVENTS: the full-form decoder reading the length from offset 3 and
	// emitting an empty or garbage next-hop for every IPv6 route in a RIB dump.
	nh := netip.MustParseAddr("2001:db8::1").As16()
	attrs := []mrt.PathAttribute{ribMPReach(nh[:])}

	rec := buildRouteRecord("2001:db8::/32", attrs, nil, 0, true)
	assert.Equal(t, "2001:db8::1", rec.NextHop)
}

func TestBuildRouteRecord_IPv6NextHopGlobalPlusLinkLocal(t *testing.T) {
	// VALIDATES: the 32-byte next-hop form (global + link-local, RFC 2545
	// Section 3) that route collectors commonly emit resolves to the global
	// address rather than being dropped.
	// PREVENTS: an empty next-hop column for peers announcing both addresses.
	global := netip.MustParseAddr("2001:db8::1").As16()
	linkLocal := netip.MustParseAddr("fe80::1").As16()
	attrs := []mrt.PathAttribute{ribMPReach(append(append([]byte{}, global[:]...), linkLocal[:]...))}

	rec := buildRouteRecord("2001:db8::/32", attrs, nil, 0, true)
	assert.Equal(t, "2001:db8::1", rec.NextHop)
}

func TestBuildRouteRecord_ASPathWidthComesFromRecordType(t *testing.T) {
	// VALIDATES: the AS_PATH width reaches buildRouteRecord from the MRT record
	// type. TABLE_DUMP_V2 is 4-byte (Section 4.3.4); BGP4MP_MESSAGE is 2-byte
	// (Section 4.4.2).
	// PREVENTS: a hardcoded 4-byte assumption silently corrupting the AS path of
	// every 2-byte record, the RFC 6396 "AS width mismatch" pitfall.
	fourByteWire := asPathAttr(true, 65000, 65001)
	rec := buildRouteRecord("10.0.0.0/8", []mrt.PathAttribute{fourByteWire}, nil, 0, true)
	assert.Equal(t, []uint32{65000, 65001}, rec.ASPath)

	twoByteWire := asPathAttr(false, 65000, 65001)
	rec = buildRouteRecord("10.0.0.0/8", []mrt.PathAttribute{twoByteWire}, nil, 0, false)
	assert.Equal(t, []uint32{65000, 65001}, rec.ASPath)

	// Reading the 2-byte wire at 4-byte width must not yield the true path.
	rec = buildRouteRecord("10.0.0.0/8", []mrt.PathAttribute{twoByteWire}, nil, 0, true)
	assert.NotEqual(t, []uint32{65000, 65001}, rec.ASPath)
}

func TestBuildRouteRecord_IPv4NextHopAttribute(t *testing.T) {
	// VALIDATES: an IPv4 RIB entry using the plain NEXT_HOP attribute (type 3),
	// which is how ze's own TABLE_DUMP_V2 writer encodes it, still works.
	// PREVENTS: the RIB-specific path regressing the common IPv4 case.
	attrs := []mrt.PathAttribute{{Flags: 0x40, Code: mrt.AttrNextHop, Value: []byte{192, 0, 2, 1}}}
	rec := buildRouteRecord("10.0.0.0/8", attrs, nil, 0, true)
	assert.Equal(t, "192.0.2.1", rec.NextHop)
}

func TestBuildRouteRecord_PeerFromIndexTable(t *testing.T) {
	// VALIDATES: peer IP and ASN are resolved through the PEER_INDEX_TABLE.
	// PREVENTS: mismatched peer attribution when a RIB record carries entries
	// from several peers.
	pit := &mrt.PeerIndexTable{Peers: []mrt.PeerEntry{
		{IP: []byte{10, 0, 0, 1}, ASN: 64500},
		{IP: []byte{10, 0, 0, 2}, ASN: 64501},
	}}
	rec := buildRouteRecord("10.0.0.0/8", nil, pit, 1, true)
	assert.Equal(t, "10.0.0.2", rec.PeerIP)
	assert.Equal(t, uint32(64501), rec.PeerASN)
}

func TestBuildRouteRecord_LargeCommunities(t *testing.T) {
	// VALIDATES: RFC 8092 large communities reach the routes output as
	// "global:local1:local2".
	// PREVENTS: silently dropping large communities from the prefix table --
	// they are widely deployed and invisible in the standard community list.
	v := make([]byte, 24)
	binary.BigEndian.PutUint32(v[0:], 65000)
	binary.BigEndian.PutUint32(v[4:], 1)
	binary.BigEndian.PutUint32(v[8:], 2)
	binary.BigEndian.PutUint32(v[12:], 65001)
	binary.BigEndian.PutUint32(v[16:], 3)
	binary.BigEndian.PutUint32(v[20:], 4)

	attrs := []mrt.PathAttribute{{Code: mrt.AttrLargeCommunity, Value: v}}
	rec := buildRouteRecord("10.0.0.0/8", attrs, nil, 0, true)
	assert.Equal(t, []string{"65000:1:2", "65001:3:4"}, rec.LargeCommunities)
}

func TestBuildRouteRecord_ExtendedCommunities(t *testing.T) {
	// VALIDATES: RFC 4360 extended communities reach the output with type,
	// subtype and value.
	// PREVENTS: dropping route targets and other extended communities.
	v := []byte{0x00, 0x02, 0xFD, 0xE8, 0x00, 0x00, 0x00, 0x64}
	attrs := []mrt.PathAttribute{{Code: mrt.AttrExtCommunity, Value: v}}
	rec := buildRouteRecord("10.0.0.0/8", attrs, nil, 0, true)
	require.Len(t, rec.ExtendedCommunities, 1)
	assert.Equal(t, "0:2:fde800000064", rec.ExtendedCommunities[0])
}

func TestBuildRouteRecord_AggregatorAndAtomicAggregate(t *testing.T) {
	// VALIDATES: AGGREGATOR (RFC 4271 Section 5.1.7) and ATOMIC_AGGREGATE
	// (Section 5.1.6) reach the output.
	// PREVENTS: losing the aggregation provenance of a route, which is what
	// explains an unexpectedly short AS path.
	agg := []byte{0, 1, 0, 0, 192, 0, 2, 1} // AS 65536, 192.0.2.1
	attrs := []mrt.PathAttribute{
		{Code: mrt.AttrAggregator, Value: agg},
		{Code: mrt.AttrAtomicAggregate},
	}
	rec := buildRouteRecord("10.0.0.0/8", attrs, nil, 0, true)
	assert.Equal(t, "AS65536:192.0.2.1", rec.Aggregator)
	assert.True(t, rec.AtomicAggregate)
}

func TestBuildRouteRecord_OnlyToCustomer(t *testing.T) {
	// VALIDATES: the RFC 9234 OTC attribute reaches the output as the AS that
	// set it, so a route leak is visible in the prefix table.
	// PREVENTS: dropping the one attribute that marks a route as
	// customer-only, which is the signal a leak analysis looks for.
	v := make([]byte, 4)
	binary.BigEndian.PutUint32(v, 64500)
	attrs := []mrt.PathAttribute{{Code: mrt.AttrOTC, Value: v}}
	rec := buildRouteRecord("10.0.0.0/8", attrs, nil, 0, true)
	assert.Equal(t, uint32(64500), rec.OnlyToCustomer)

	// Absent OTC must stay zero so `omitempty` drops the field entirely.
	bare := buildRouteRecord("10.0.0.0/8", nil, nil, 0, true)
	assert.Zero(t, bare.OnlyToCustomer)
}

func TestBuildRouteRecord_OutOfRangePeerIndex(t *testing.T) {
	// VALIDATES: a peer index past the end of the table leaves the peer fields
	// empty instead of panicking.
	// PREVENTS: an index-out-of-range crash on a dump whose RIB records
	// reference a peer the PEER_INDEX_TABLE does not contain.
	pit := &mrt.PeerIndexTable{Peers: []mrt.PeerEntry{{IP: []byte{10, 0, 0, 1}, ASN: 64500}}}
	require.NotPanics(t, func() {
		rec := buildRouteRecord("10.0.0.0/8", nil, pit, 99, true)
		assert.Empty(t, rec.PeerIP)
	})
}
