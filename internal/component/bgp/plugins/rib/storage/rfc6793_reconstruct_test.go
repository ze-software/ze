// RFC: rfc/short/rfc6793.md — the receive-side AS path reconstruction of Section 4.2.3
//
// Drives ParseAttributes (attrparse.go), the ingest path that turns a received
// UPDATE's attribute bytes into the interned RIB entry. Every case here enters
// through that entry point rather than through canonicalizeASPath, because the
// AGGREGATOR gate and the AS path merge are one procedure in the RFC and only
// the entry point runs both.

package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pool "github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
)

var (
	// AS_PATH from an OLD speaker: AS_SEQUENCE [64500, 65001, AS_TRANS] in two
	// octets. Three AS numbers, one more than the AS4_PATH beside it carries.
	wireASPathThreeHops = []byte{
		0x40, 0x02, 0x08,
		0x02, 0x03, 0xFB, 0xF4, 0xFD, 0xE9, 0x5B, 0xA0,
	}

	// AS4_PATH: AS_SEQUENCE [199524] in four octets. One AS number.
	wireAS4PathOneHop = []byte{
		0xC0, 0x11, 0x06,
		0x02, 0x01, 0x00, 0x03, 0x0B, 0x64,
	}

	// AS_PATH from an OLD speaker: AS_SEQUENCE [64500, AS_TRANS]. Two AS numbers.
	wireASPathTwoHops = []byte{
		0x40, 0x02, 0x06,
		0x02, 0x02, 0xFB, 0xF4, 0x5B, 0xA0,
	}

	// AS4_PATH: AS_SEQUENCE [65001, 199524, 199525]. Three AS numbers, one more
	// than the AS_PATH beside it carries.
	wireAS4PathThreeHops = []byte{
		0xC0, 0x11, 0x0E,
		0x02, 0x03, 0x00, 0x00, 0xFD, 0xE9, 0x00, 0x03, 0x0B, 0x64, 0x00, 0x03, 0x0B, 0x65,
	}

	// AS_PATH from an OLD speaker inside a confederation: AS_CONFED_SEQUENCE
	// [65001, 65002] leading, then AS_SEQUENCE [64500, AS_TRANS]. RFC 5065
	// leaves the confederation segment out of the AS number count, so this path
	// counts two AS numbers.
	wireASPathConfedLeading = []byte{
		0x40, 0x02, 0x0C,
		0x03, 0x02, 0xFD, 0xE9, 0xFD, 0xEA,
		0x02, 0x02, 0xFB, 0xF4, 0x5B, 0xA0,
	}

	// AS_PATH whose confederation segment TRAILS the sequence: AS_SEQUENCE
	// [64500, AS_TRANS] then AS_CONFED_SEQUENCE [65001]. Two AS numbers.
	wireASPathConfedTrailing = []byte{
		0x40, 0x02, 0x0A,
		0x02, 0x02, 0xFB, 0xF4, 0x5B, 0xA0,
		0x03, 0x01, 0xFD, 0xE9,
	}

	// AS4_PATH: AS_SEQUENCE [199524, 199525]. Two AS numbers.
	wireAS4PathTwoHops = []byte{
		0xC0, 0x11, 0x0A,
		0x02, 0x02, 0x00, 0x03, 0x0B, 0x64, 0x00, 0x03, 0x0B, 0x65,
	}

	// AGGREGATOR from an OLD speaker carrying a real two-octet AS (64500), not
	// AS_TRANS, with aggregator address 10.0.0.1.
	wireAggregatorRealAS = []byte{
		0xC0, 0x07, 0x06,
		0xFB, 0xF4, 0x0A, 0x00, 0x00, 0x01,
	}
)

// entryASPath returns the AS path bytes ParseAttributes interned for entry.
func entryASPath(t *testing.T, entry RouteEntry) []byte {
	t.Helper()
	require.True(t, entry.HasASPath(), "the route must carry an AS path")
	got, err := pool.ASPath.Get(entry.ASPath)
	require.NoError(t, err)
	return got
}

// TestRFC6793LongerASPathPrependsLeadingHops drives ParseAttributes with an
// AS_PATH of three AS numbers beside an AS4_PATH of one. The two leading AS
// numbers of the AS_PATH are prepended to the AS4_PATH, so the reconstructed
// path is three AS numbers long and the AS_TRANS placeholder is gone.
//
// RFC requirement: RFC6793-4.2.3-9 positive -- with an AS_PATH of three AS numbers and an
// AS4_PATH of one, the reconstructed AS path is built by taking the two leading AS numbers of
// the AS_PATH and prepending them to the AS4_PATH, so it carries 64500, 65001 and 199524 and
// has the same AS number count as the received AS_PATH.
//
// RFC requirement: RFC6793-4.2.3-8 negative -- the AS4_PATH is NOT ignored when the AS_PATH
// carries at least as many AS numbers: its 199524 replaces the AS_TRANS placeholder rather than
// the two-octet AS_PATH being taken whole.
func TestRFC6793LongerASPathPrependsLeadingHops(t *testing.T) {
	raw := concat(wireOriginIGP, wireASPathThreeHops, wireAS4PathOneHop)

	entry, err := ParseAttributes(raw, false)
	require.NoError(t, err)
	defer entry.Release()

	assert.Equal(t, []byte{
		0x02, 0x02, 0x00, 0x00, 0xFB, 0xF4, 0x00, 0x00, 0xFD, 0xE9,
		0x02, 0x01, 0x00, 0x03, 0x0B, 0x64,
	}, entryASPath(t, entry),
		"the two leading AS_PATH hops are prepended to the AS4_PATH")
}

// TestRFC6793LongerAS4PathIsIgnored drives ParseAttributes with an AS4_PATH
// carrying more AS numbers than the AS_PATH. The AS4_PATH is peer-supplied and
// unverifiable, so a longer one is discarded rather than trusted.
//
// RFC requirement: RFC6793-4.2.3-8 positive -- when the AS_PATH carries fewer AS numbers than
// the AS4_PATH, the AS4_PATH is ignored and the AS_PATH is taken as the AS path information, so
// the route's path is the two-octet AS_PATH widened to four octets with its AS_TRANS intact and
// none of the AS4_PATH's AS numbers.
//
// RFC requirement: RFC6793-4.2.3-9 negative -- no prepend happens in this direction: the
// reconstructed path is the AS_PATH alone rather than leading AS_PATH hops joined to the
// AS4_PATH.
func TestRFC6793LongerAS4PathIsIgnored(t *testing.T) {
	raw := concat(wireOriginIGP, wireASPathTwoHops, wireAS4PathThreeHops)

	entry, err := ParseAttributes(raw, false)
	require.NoError(t, err)
	defer entry.Release()

	assert.Equal(t, []byte{
		0x02, 0x02, 0x00, 0x00, 0xFB, 0xF4, 0x00, 0x00, 0x5B, 0xA0,
	}, entryASPath(t, entry),
		"a longer AS4_PATH is ignored and the AS_PATH is taken")
}

// TestRFC6793LeadingConfedSegmentIsPrepended drives ParseAttributes with an
// AS_PATH whose leading segment is an AS_CONFED_SEQUENCE. The confederation
// segment counts no AS numbers, so it neither consumes the prepend budget nor
// is dropped: it is prepended whole because it leads the path.
//
// RFC requirement: RFC6793-4.2.3-10 positive -- the leading AS_CONFED_SEQUENCE of the AS_PATH is
// prepended whole to the reconstructed path, and because RFC 5065 counts no AS numbers for it,
// the one AS number of prepend budget is still spent on the AS_SEQUENCE hop 64500 that follows
// it.
func TestRFC6793LeadingConfedSegmentIsPrepended(t *testing.T) {
	raw := concat(wireOriginIGP, wireASPathConfedLeading, wireAS4PathOneHop)

	entry, err := ParseAttributes(raw, false)
	require.NoError(t, err)
	defer entry.Release()

	assert.Equal(t, []byte{
		0x03, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0xFD, 0xEA,
		0x02, 0x01, 0x00, 0x00, 0xFB, 0xF4,
		0x02, 0x01, 0x00, 0x03, 0x0B, 0x64,
	}, entryASPath(t, entry),
		"the leading confederation segment is prepended and costs no budget")
}

// TestRFC6793UnadjacentConfedSegmentIsNotPrepended is the counterpart: the
// AS_PATH and the AS4_PATH count the same number of AS numbers, so nothing is
// prepended, and a confederation segment that neither leads the path nor sits
// beside a prepended segment is dropped with the rest of the AS_PATH.
//
// RFC requirement: RFC6793-4.2.3-10 negative -- the confederation-segment rule is conditional:
// an AS_CONFED_SEQUENCE that is neither the leading path segment nor adjacent to a prepended
// segment is not prepended, so the reconstructed path is the AS4_PATH alone.
func TestRFC6793UnadjacentConfedSegmentIsNotPrepended(t *testing.T) {
	raw := concat(wireOriginIGP, wireASPathConfedTrailing, wireAS4PathTwoHops)

	entry, err := ParseAttributes(raw, false)
	require.NoError(t, err)
	defer entry.Release()

	assert.Equal(t, []byte{
		0x02, 0x02, 0x00, 0x03, 0x0B, 0x64, 0x00, 0x03, 0x0B, 0x65,
	}, entryASPath(t, entry),
		"a confederation segment adjacent to nothing prepended is not prepended")
}

// TestRFC6793AggregatorWithRealASIgnoresAS4Attributes drives ParseAttributes
// with an AGGREGATOR carrying a real AS beside an AS4_AGGREGATOR and an
// AS4_PATH. The AGGREGATOR's AS is not AS_TRANS, so the OLD speaker that
// aggregated the route understood four-octet AS numbers as little as it
// understood the AS4_* pair it forwarded: both are ignored.
//
// RFC requirement: RFC6793-4.2.3-3 positive -- an AGGREGATOR whose AS is not AS_TRANS makes both
// the AS4_AGGREGATOR and the AS4_PATH ignored: the AS4_AGGREGATOR's four-octet AS is not stored
// with the route, and the AS4_PATH's AS numbers do not reach the route's AS path.
//
// RFC requirement: RFC6793-4.2.3-4 positive -- the received AGGREGATOR is taken as the
// information about the aggregating node, so the route's aggregator is the two-octet AS 64500
// with address 10.0.0.1 exactly as received.
//
// RFC requirement: RFC6793-4.2.3-5 positive -- the AS_PATH is taken as the AS path information,
// so the route's path is the received two-octet AS_PATH widened to four octets, AS_TRANS
// included.
//
// RFC requirement: RFC6793-4.2.3-6 negative -- the AGGREGATOR is ignored only when it carries
// AS_TRANS: here it carries 64500 and is kept.
//
// RFC requirement: RFC6793-4.2.3-7 negative -- the AS4_AGGREGATOR is taken as the aggregating
// node only when the AGGREGATOR carries AS_TRANS: here it is not, so 4200000001 is not the
// route's aggregating AS.
func TestRFC6793AggregatorWithRealASIgnoresAS4Attributes(t *testing.T) {
	raw := concat(wireOriginIGP,
		rfc6793WireASPathWithASTrans,
		wireAggregatorRealAS,
		rfc6793WireAS4Path,
		rfc6793WireAS4Aggregator)

	entry, err := ParseAttributes(raw, false)
	require.NoError(t, err)
	defer entry.Release()

	b := entry.GetBundle()
	require.True(t, b.HasAggregator(), "the received AGGREGATOR is the aggregating node")
	agg, err := pool.Aggregator.Get(b.Aggregator)
	require.NoError(t, err)
	assert.Equal(t, []byte{0xFB, 0xF4, 0x0A, 0x00, 0x00, 0x01}, agg,
		"the AGGREGATOR is taken as received")

	assert.Equal(t, []byte{
		0x02, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0x5B, 0xA0,
	}, entryASPath(t, entry),
		"the AS_PATH is taken as the AS path information")

	if b.HasOtherAttrs() {
		other, err := pool.OtherAttrs.Get(b.OtherAttrs)
		require.NoError(t, err)
		assert.NotContains(t, string(other), string([]byte{0xFA, 0x56, 0xEA, 0x01}),
			"the ignored AS4_AGGREGATOR is not kept with the route")
	}
}

// TestRFC6793AggregatorWithASTransPromotesAS4Aggregator is the counterpart: the
// AGGREGATOR carries AS_TRANS, which is the OLD speaker saying it could not
// encode the real aggregating AS, so the AS4_AGGREGATOR carries it instead.
//
// RFC requirement: RFC6793-4.2.3-6 positive -- an AGGREGATOR carrying AS_TRANS is ignored: the
// two-octet AS_TRANS value is not what the route records as its aggregating node.
//
// RFC requirement: RFC6793-4.2.3-7 positive -- the AS4_AGGREGATOR is taken as the information
// about the aggregating node, so the route's aggregator is the four-octet AS 4200000001 with
// address 10.0.0.1.
//
// RFC requirement: RFC6793-4.2.3-3 negative -- the AS4_AGGREGATOR and AS4_PATH are ignored only
// when the AGGREGATOR carries an AS that is not AS_TRANS: here it carries AS_TRANS, so both are
// used.
//
// RFC requirement: RFC6793-4.2.3-4 negative -- the AGGREGATOR is not taken as the aggregating
// node in this direction, so the AS_TRANS placeholder never becomes the route's aggregating AS.
//
// RFC requirement: RFC6793-4.2.3-5 negative -- the AS_PATH is not taken as the AS path
// information in this direction: the reconstruction runs and the four-octet 4200000001 replaces
// the AS_TRANS placeholder.
func TestRFC6793AggregatorWithASTransPromotesAS4Aggregator(t *testing.T) {
	raw := concat(wireOriginIGP,
		rfc6793WireASPathWithASTrans,
		rfc6793WireAggregatorASTrans,
		rfc6793WireAS4Path,
		rfc6793WireAS4Aggregator)

	entry, err := ParseAttributes(raw, false)
	require.NoError(t, err)
	defer entry.Release()

	b := entry.GetBundle()
	require.True(t, b.HasAggregator(), "the AS4_AGGREGATOR is the aggregating node")
	agg, err := pool.Aggregator.Get(b.Aggregator)
	require.NoError(t, err)
	assert.Equal(t, []byte{0xFA, 0x56, 0xEA, 0x01, 0x0A, 0x00, 0x00, 0x01}, agg,
		"the AS4_AGGREGATOR is taken as the aggregating node")

	assert.Equal(t, []byte{
		0x02, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0xFA, 0x56, 0xEA, 0x01,
	}, entryASPath(t, entry),
		"the reconstruction runs and the four-octet AS replaces AS_TRANS")
}

// TestReconstructionCostsNothingWithoutAS4Path pins the cost of the common
// case. An UPDATE from a session that negotiated four-octet AS support carries
// no AS4_PATH, so the reconstruction must not run and must not allocate. The
// two-octet case pays exactly one allocation, the widening buffer, as it did
// before the reconstruction existed.
func TestReconstructionCostsNothingWithoutAS4Path(t *testing.T) {
	asPath4Byte := []byte{0x02, 0x02, 0x00, 0x00, 0xFB, 0xF4, 0x00, 0x00, 0xFD, 0xE9}
	asPath2Byte := []byte{0x02, 0x02, 0xFB, 0xF4, 0xFD, 0xE9}

	allocs := testing.AllocsPerRun(100, func() {
		if canonicalizeASPath(asPath4Byte, nil, true) == nil {
			t.Fatal("the four-octet AS_PATH must be returned as it is")
		}
	})
	assert.Equal(t, 0.0, allocs, "a four-octet AS_PATH with no AS4_PATH allocates nothing")

	allocs = testing.AllocsPerRun(100, func() {
		if canonicalizeASPath(asPath2Byte, nil, false) == nil {
			t.Fatal("the two-octet AS_PATH must be widened")
		}
	})
	assert.Equal(t, 1.0, allocs, "widening a two-octet AS_PATH costs one buffer")
}

// TestParseAttributesCostsNothingExtraWithoutAS4Path pins the same cost at the
// ingest entry point rather than at the AS path producer alone. An UPDATE from
// a session that negotiated four-octet AS support reaches neither the AGGREGATOR
// choice's read of the AS number nor the AS path reconstruction, so the whole
// parse is what it was before RFC 6793 Section 4.2.3 ran here.
//
// The interning pools deduplicate, so the second and later parses of one
// attribute set allocate only what the parse itself needs.
func TestParseAttributesCostsNothingExtraWithoutAS4Path(t *testing.T) {
	raw := concat(wireOriginIGP,
		[]byte{0x40, 0x02, 0x0A, 0x02, 0x02, 0x00, 0x00, 0xFB, 0xF4, 0x00, 0x00, 0xFD, 0xE9},
		wireNextHop)

	// Warm the pools, so the run measures the parse rather than the first
	// intern of each value.
	warm, err := ParseAttributes(raw, true)
	require.NoError(t, err)
	warm.Release()

	allocs := testing.AllocsPerRun(100, func() {
		entry, err := ParseAttributes(raw, true)
		if err != nil {
			t.Fatal(err)
		}
		entry.Release()
	})
	t.Logf("the common ingest path costs %.0f allocations per UPDATE", allocs)
	assert.Zero(t, allocs, "an UPDATE with no AS4_PATH must allocate nothing on ingest")
}

// TestParseAttributesReconstructionCost pins what the OLD-speaker path costs,
// which is the only path that pays anything. Reading the segments of both
// attributes and writing the merged path back is what it buys.
//
// VALIDATES: the cost grows with the SEGMENT count and not with the AS number
// count. Each parsed segment costs one AS number slice however many AS numbers
// it holds, and the merged path is written into one buffer.
// PREVENTS: a reconstruction that grows with the path length, which the
// correctness cases above cannot see over their short fixtures.
func TestParseAttributesReconstructionCost(t *testing.T) {
	raw := concat(wireOriginIGP, wireASPathThreeHops, wireAS4PathOneHop)

	warm, err := ParseAttributes(raw, false)
	require.NoError(t, err)
	warm.Release()

	allocs := testing.AllocsPerRun(100, func() {
		entry, err := ParseAttributes(raw, false)
		if err != nil {
			t.Fatal(err)
		}
		entry.Release()
	})
	t.Logf("the AS4_PATH reconstruction costs %.0f allocations per UPDATE", allocs)
	assert.Equal(t, 10.0, allocs,
		"two parsed paths of one segment each, the merged path, and the buffer it is written into")
}

// AS_PATH from an OLD speaker carrying an AS_SET in its leading part:
// AS_SEQUENCE [64500], AS_SET [65001, 65002], AS_SEQUENCE [AS_TRANS].
// RFC 4271 Section 9.1.2.2 counts the set as one, so this path counts three.
var wireASPathWithLeadingSet = []byte{
	0x40, 0x02, 0x0E,
	0x02, 0x01, 0xFB, 0xF4,
	0x01, 0x02, 0xFD, 0xE9, 0xFD, 0xEA,
	0x02, 0x01, 0x5B, 0xA0,
}

// AS4_PATH whose segment claims three AS numbers and carries one. The attribute
// length is honest, so the iterator hands the value over and the AS4_PATH parse
// is what refuses it.
var wireAS4PathMalformed = []byte{
	0xC0, 0x11, 0x06,
	0x02, 0x03, 0x00, 0x03, 0x0B, 0x64,
}

// AGGREGATOR in the four-octet form, which is the wrong width for a session
// that did not negotiate the four-octet AS capability. RFC 7606 Section 7.7
// rejects every length but the negotiated one.
var wireAggregatorWrongWidth = []byte{
	0xC0, 0x07, 0x08,
	0x00, 0x00, 0xFB, 0xF4, 0x0A, 0x00, 0x00, 0x01,
}

// TestRFC6793EqualCountsPrependNothing drives an AS_PATH and an AS4_PATH that
// carry the same number of AS numbers. The leading part to take is empty, so the
// reconstructed path is the AS4_PATH alone.
//
// The boundary matters because the RFC's two branches meet here: the count
// comparison is "larger than or equal", so equality reconstructs rather than
// ignoring, and it reconstructs by prepending nothing at all.
//
// RFC requirement: RFC6793-4.2.3-9 positive -- an AS_PATH and an AS4_PATH of two AS numbers
// each reconstruct to the AS4_PATH's own two AS numbers, so the result keeps the AS_PATH's AS
// number count and no leading AS number is prepended.
func TestRFC6793EqualCountsPrependNothing(t *testing.T) {
	raw := concat(wireOriginIGP, wireASPathTwoHops, wireAS4PathTwoHops)

	entry, err := ParseAttributes(raw, false)
	require.NoError(t, err)
	defer entry.Release()

	assert.Equal(t, []byte{
		0x02, 0x02, 0x00, 0x03, 0x0B, 0x64, 0x00, 0x03, 0x0B, 0x65,
	}, entryASPath(t, entry),
		"the AS4_PATH is the whole reconstruction when the counts are equal")
}

// TestRFC6793LeadingASSetIsTakenWhole drives an AS_PATH whose leading part holds
// an AS_SET. The set is taken entire and spends one AS number of the budget,
// which is what RFC 4271 Section 9.1.2.2 counts it as.
//
// Cutting a set to fit a budget would drop the aggregated AS numbers the set
// exists to carry and would not shorten the path by the RFC's own measure, so
// the set is either taken whole or not reached.
//
// RFC requirement: RFC6793-4.2.3-9 positive -- an AS_SET in the leading part of the AS_PATH is
// prepended whole, with both of its AS numbers, and counts as one AS number toward the leading
// part the reconstruction takes.
func TestRFC6793LeadingASSetIsTakenWhole(t *testing.T) {
	raw := concat(wireOriginIGP, wireASPathWithLeadingSet, wireAS4PathOneHop)

	entry, err := ParseAttributes(raw, false)
	require.NoError(t, err)
	defer entry.Release()

	assert.Equal(t, []byte{
		0x02, 0x01, 0x00, 0x00, 0xFB, 0xF4,
		0x01, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0xFD, 0xEA,
		0x02, 0x01, 0x00, 0x03, 0x0B, 0x64,
	}, entryASPath(t, entry),
		"the set keeps both of its AS numbers and the AS4_PATH follows it")
}

// TestRFC6793MalformedAS4PathIsDiscarded drives an AS4_PATH the parse refuses.
// The UPDATE is still processed and the received AS_PATH is the AS path
// information, widened to the four-octet encoding the RIB interns.
//
// This is the branch that decides whether one malformed attribute from an OLD
// speaker costs the route or only costs the reconstruction.
//
// RFC requirement: RFC6793-6-1 positive -- a malformed AS4_PATH is discarded and the UPDATE
// continues to be processed, so the route is stored carrying the AS_PATH it arrived with rather
// than being rejected.
func TestRFC6793MalformedAS4PathIsDiscarded(t *testing.T) {
	raw := concat(wireOriginIGP, wireASPathTwoHops, wireAS4PathMalformed)

	entry, err := ParseAttributes(raw, false)
	require.NoError(t, err, "a malformed AS4_PATH must not cost the UPDATE")
	defer entry.Release()

	assert.Equal(t, []byte{
		0x02, 0x02, 0x00, 0x00, 0xFB, 0xF4, 0x00, 0x00, 0x5B, 0xA0,
	}, entryASPath(t, entry),
		"the AS_PATH is taken as received, AS_TRANS and all, with no merge")
}

// TestRFC6793AggregatorOfTheWrongWidthIsNotRead drives an AGGREGATOR whose
// length disagrees with the negotiated AS width, beside an AS4_AGGREGATOR. The
// AS number leading it cannot be read, so nothing decides the RFC's AS_TRANS
// comparison and the route records no aggregating node.
//
// Guessing the width would answer the comparison on a value nobody parsed, and
// that comparison also decides whether the AS4_PATH is used.
//
// RFC requirement: RFC6793-4.2.3-3 negative -- an AGGREGATOR whose length does not match the
// negotiated AS width does not make the AS4_PATH ignored: the AS path is still reconstructed,
// and no aggregating node is recorded from a value that could not be read.
func TestRFC6793AggregatorOfTheWrongWidthIsNotRead(t *testing.T) {
	raw := concat(wireOriginIGP, wireASPathThreeHops, wireAS4PathOneHop,
		wireAggregatorWrongWidth, rfc6793WireAS4Aggregator)

	entry, err := ParseAttributes(raw, false)
	require.NoError(t, err)
	defer entry.Release()

	assert.False(t, entry.GetBundle().HasAggregator(),
		"an AGGREGATOR that could not be read records no aggregating node")
	assert.Equal(t, []byte{
		0x02, 0x02, 0x00, 0x00, 0xFB, 0xF4, 0x00, 0x00, 0xFD, 0xE9,
		0x02, 0x01, 0x00, 0x03, 0x0B, 0x64,
	}, entryASPath(t, entry),
		"the prepended part stays its own segment, and the AS4_PATH follows it")
}

// TestRFC6793LoneAS4AggregatorIsKeptUninterpreted drives an AS4_AGGREGATOR with
// no AGGREGATOR beside it. RFC 6793 Section 4.2.3 rules on the PAIR, so a lone
// attribute leaves nothing to choose between and it stays with the route
// uninterpreted, as every attribute ze does not read does.
//
// RFC requirement: RFC6793-4.2.3-7 negative -- an AS4_AGGREGATOR arriving without an AGGREGATOR
// is not promoted to the route's aggregating node, because the rule that promotes it is written
// for the pair; it is retained with the route instead.
func TestRFC6793LoneAS4AggregatorIsKeptUninterpreted(t *testing.T) {
	raw := concat(wireOriginIGP, wireASPathTwoHops, rfc6793WireAS4Aggregator)

	entry, err := ParseAttributes(raw, false)
	require.NoError(t, err)
	defer entry.Release()

	b := entry.GetBundle()
	assert.False(t, b.HasAggregator(), "a lone AS4_AGGREGATOR is not the aggregating node")
	require.True(t, b.HasOtherAttrs(), "it is retained with the route")
	other, err := pool.OtherAttrs.Get(b.OtherAttrs)
	require.NoError(t, err)
	assert.Contains(t, string(other), string([]byte{0xFA, 0x56, 0xEA, 0x01}),
		"the four-octet aggregating AS is still there to be read later")
}
