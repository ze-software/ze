// Design: docs/architecture/edge-cases/as4.md -- the RFC 6793 receive procedure, run once
// RFC: rfc/short/rfc6793.md -- AS4_PATH reconstruction and the NEW-speaker discard
// Related: aspath_collapse.go -- CollapseAS4Family, the payload rewrite under test
//
// Every test here drives CollapseAS4Family over a whole UPDATE payload, so an
// arm of the reconciliation is judged by the bytes a peer would receive rather
// than by the value the rule returned. The entry-point tests that prove a
// received UPDATE reaches this code at all live in
// internal/component/bgp/reactor/rfc6793_ingest_collapse_test.go.

package wireu

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// The AS numbers one mixed-width UPDATE is built from.
//
// collapseMappableAS fits two octets, so an OLD speaker writes it into AS_PATH
// as itself. collapseRealAS does not, so the same speaker writes AS_TRANS in
// its place and carries the real value in AS4_PATH (RFC 6793 Section 4.2.2).
const (
	collapseMappableAS uint32 = 65100
	collapseRealAS     uint32 = 4200000123
	collapseOtherAS    uint32 = 4200000456
	collapseASTrans    uint32 = 23456
)

// collapseSeg is one AS path segment a fixture builds, at whichever AS number
// width the attribute it lands in uses.
type collapseSeg struct {
	kind attribute.ASPathSegmentType
	asns []uint32
}

// collapseSeq is the ordinary segment: an AS_SEQUENCE holding asns.
func collapseSeq(asns ...uint32) collapseSeg {
	return collapseSeg{kind: attribute.ASSequence, asns: asns}
}

// collapsePathValue builds an AS_PATH or AS4_PATH attribute value at the given
// AS number width, which is 2 octets for an AS_PATH an OLD speaker sent and 4
// for every AS4_PATH (RFC 6793 Section 3).
func collapsePathValue(octets int, segs ...collapseSeg) []byte {
	var out []byte
	for _, seg := range segs {
		out = append(out, byte(seg.kind), byte(len(seg.asns))) //nolint:gosec // fixture segments hold far fewer than 255 AS numbers
		for _, asn := range seg.asns {
			if octets == 2 {
				out = binary.BigEndian.AppendUint16(out, uint16(asn)) //nolint:gosec // a 2-octet fixture ASN is always mappable
				continue
			}
			out = binary.BigEndian.AppendUint32(out, asn)
		}
	}
	return out
}

// collapseAggValue builds an AGGREGATOR or AS4_AGGREGATOR value: the
// aggregating AS at the given width, followed by its IPv4 address.
func collapseAggValue(octets int, asn uint32) []byte {
	var out []byte
	if octets == 2 {
		out = binary.BigEndian.AppendUint16(out, uint16(asn)) //nolint:gosec // callers pass a mappable ASN or AS_TRANS
	} else {
		out = binary.BigEndian.AppendUint32(out, asn)
	}
	return append(out, 192, 0, 2, 1)
}

// collapseAttrWire builds one path attribute in wire form. WriteHeaderTo picks
// the extended-length header for a value past 255 octets, so a fixture never
// has to choose one.
func collapseAttrWire(flags attribute.AttributeFlags, code attribute.AttributeCode, value []byte) []byte {
	out := make([]byte, 4+len(value))
	n := attribute.WriteHeaderTo(out, 0, flags, code, uint16(len(value))) //nolint:gosec // fixture values are well inside the wire ceiling
	n += copy(out[n:], value)
	return out[:n]
}

// collapseWellKnown is the ORIGIN and NEXT_HOP pair every fixture carries, so
// each test can assert that the attributes the collapse does not read travel
// through it untouched.
func collapseWellKnown() []byte {
	attrs := collapseAttrWire(attribute.FlagTransitive, attribute.AttrOrigin, []byte{0x00})
	return append(attrs, collapseAttrWire(attribute.FlagTransitive, attribute.AttrNextHop, []byte{192, 0, 2, 254})...)
}

// collapseNLRI is one announced prefix, 10.0.0.0/24.
var collapseNLRI = []byte{24, 10, 0, 0}

// collapseRun drives one payload through CollapseAS4Family with a buffer sized
// the way the ingest caller sizes it, and answers the collapsed payload.
//
// A nil payload answer means the collapse reported that nothing was owed, which
// is the fast path: the caller then keeps the payload it already has.
func collapseRun(t *testing.T, payload []byte, srcASN4 bool) ([]byte, []attribute.ASPathDiscard) {
	t.Helper()

	dst := make([]byte, CollapseAS4FamilySize(payload))
	n, discards, err := CollapseAS4Family(dst, payload, srcASN4)
	require.NoError(t, err, "a well-formed payload must collapse")
	if n == 0 {
		return nil, discards
	}
	require.LessOrEqual(t, n, len(dst),
		"CollapseAS4FamilySize must bound what CollapseAS4Family writes")
	return dst[:n], discards
}

// collapseAttrValue answers one attribute value of a payload, and whether the
// attribute is present at all.
func collapseAttrValue(t *testing.T, payload []byte, code attribute.AttributeCode) ([]byte, bool) {
	t.Helper()
	require.GreaterOrEqual(t, len(payload), 4, "an UPDATE payload carries two length fields")

	wdLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrLenOff := 2 + wdLen
	require.GreaterOrEqual(t, len(payload), attrLenOff+2, "the attribute length field must be present")
	attrLen := int(binary.BigEndian.Uint16(payload[attrLenOff : attrLenOff+2]))
	start := attrLenOff + 2
	require.GreaterOrEqual(t, len(payload), start+attrLen, "the attribute section must be present")

	iter := attribute.NewAttrIterator(payload[start : start+attrLen])
	for typeCode, _, value, ok := iter.Next(); ok; typeCode, _, value, ok = iter.Next() {
		if typeCode == code {
			return value, true
		}
	}
	require.Zero(t, iter.Remaining(), "the collapsed attribute section must parse to its end")
	return nil, false
}

// collapseTrailer answers the NLRI bytes of a payload, which the collapse must
// carry through unchanged whatever it did to the attributes.
func collapseTrailer(t *testing.T, payload []byte) []byte {
	t.Helper()
	wdLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrLenOff := 2 + wdLen
	attrLen := int(binary.BigEndian.Uint16(payload[attrLenOff : attrLenOff+2]))
	return payload[attrLenOff+2+attrLen:]
}

// TestCollapseAS4KeepsPayloadWhenNoWorkIsOwed pins the fast path: an UPDATE
// from a NEW speaker carrying neither AS4 attribute is already four-octet
// truth, so the collapse answers 0 and reads no AS_PATH at all.
//
// VALIDATES: AC-9. The answer is 0, so the caller keeps its own slice, and the
// fixture's AS_PATH is deliberately unparseable: a collapse that parsed it
// would have to fail, so a clean 0 is proof that it did not look.
// PREVENTS: the transition machinery charging a four-octet fleet for a case
// only a mixed-width peering reaches.
func TestCollapseAS4KeepsPayloadWhenNoWorkIsOwed(t *testing.T) {
	// One AS_SEQUENCE claiming three AS numbers and carrying one. ParseASPath
	// and expandASPath2to4 both refuse it; the fast path never asks either.
	unreadable := []byte{byte(attribute.ASSequence), 3, 0, 0, 0, 1}
	attrs := collapseWellKnown()
	attrs = append(attrs, collapseAttrWire(attribute.FlagTransitive, attribute.AttrASPath, unreadable)...)
	payload := baseTestBody(attrs, collapseNLRI)

	dst := make([]byte, CollapseAS4FamilySize(payload))
	n, discards, err := CollapseAS4Family(dst, payload, true)

	require.NoError(t, err, "the fast path parses no AS_PATH, so an unreadable one cannot fail it")
	assert.Zero(t, n, "the caller keeps its own payload: a copy here is an allocation on every UPDATE")
	assert.Nil(t, discards, "nothing was dropped, so nothing is owed to the log")
}

// TestCollapseAS4FastPathAllocatesNothing is AC-9's other half: the fast path
// costs no allocation, which is what makes the ceiling in
// internal/perf/allocgate.go enforceable rather than aspirational.
//
// VALIDATES: AC-9 and R-1.
// PREVENTS: a span walk or a parse creeping into the four-octet path, where it
// would be paid once per received UPDATE per peer.
func TestCollapseAS4FastPathAllocatesNothing(t *testing.T) {
	attrs := collapseWellKnown()
	attrs = append(attrs, collapseAttrWire(attribute.FlagTransitive, attribute.AttrASPath,
		collapsePathValue(4, collapseSeq(collapseMappableAS, collapseRealAS)))...)
	payload := baseTestBody(attrs, collapseNLRI)
	dst := make([]byte, CollapseAS4FamilySize(payload))

	allocs := testing.AllocsPerRun(100, func() {
		if n, _, err := CollapseAS4Family(dst, payload, true); n != 0 || err != nil {
			t.Fatalf("fast path answered n=%d err=%v", n, err)
		}
	})

	assert.Zero(t, allocs, "the four-octet fast path must allocate nothing")
}

// TestCollapseAS4MergesAS4PathIntoASPath is the reconstruction itself: a route
// an OLD speaker relayed carries AS_TRANS where a four-octet AS number was, and
// the real value comes back out of the AS4_PATH beside it.
//
// VALIDATES: AC-1 at the payload level, and RFC 6793 Section 4.2.3's second
// arm. The attributes the collapse does not read, and the NLRI behind them,
// travel through unchanged.
// PREVENTS: AS 23456 being relayed onward as if it were a real AS number,
// which is D-1 and is permanent once a downstream speaker stores it.
func TestCollapseAS4MergesAS4PathIntoASPath(t *testing.T) {
	attrs := collapseWellKnown()
	attrs = append(attrs, collapseAttrWire(attribute.FlagTransitive, attribute.AttrASPath,
		collapsePathValue(2, collapseSeq(collapseMappableAS, collapseASTrans)))...)
	attrs = append(attrs, collapseAttrWire(attribute.FlagOptional|attribute.FlagTransitive,
		attribute.AttrAS4Path, collapsePathValue(4, collapseSeq(collapseMappableAS, collapseRealAS)))...)
	payload := baseTestBody(attrs, collapseNLRI)

	out, discards := collapseRun(t, payload, false)
	require.NotNil(t, out, "a two-octet source always owes the widening")
	assert.Empty(t, discards, "a well-formed pair drops nothing")

	asPath, ok := collapseAttrValue(t, out, attribute.AttrASPath)
	require.True(t, ok, "the collapsed payload must still carry an AS_PATH")
	assert.Equal(t, collapsePathValue(4, collapseSeq(collapseMappableAS, collapseRealAS)), asPath,
		"RFC 6793 Section 4.2.3: the AS path information is the AS4_PATH with the leading part of AS_PATH prepended")

	_, hasAS4Path := collapseAttrValue(t, out, attribute.AttrAS4Path)
	assert.False(t, hasAS4Path,
		"RFC 6793 Section 4.1: AS4_PATH MUST NOT be carried in an UPDATE between NEW BGP speakers")

	nextHop, ok := collapseAttrValue(t, out, attribute.AttrNextHop)
	require.True(t, ok, "an attribute the collapse does not read must survive it")
	assert.Equal(t, []byte{192, 0, 2, 254}, nextHop, "NEXT_HOP is copied through untouched")
	assert.Equal(t, collapseNLRI, collapseTrailer(t, out), "the announced prefix is untouched")
}

// TestCollapseAS4WidensASPathWithoutInventingAS4Path covers the OLD speaker
// that sent no AS4_PATH: its AS_PATH is the AS path information and it is
// widened, with AS_TRANS left exactly where it stood.
//
// VALIDATES: AC-2.
// PREVENTS: an invented AS4_PATH, and an AS_TRANS "recovered" into a value no
// peer ever sent. With no AS4_PATH there is nothing to recover it from.
func TestCollapseAS4WidensASPathWithoutInventingAS4Path(t *testing.T) {
	attrs := collapseWellKnown()
	attrs = append(attrs, collapseAttrWire(attribute.FlagTransitive, attribute.AttrASPath,
		collapsePathValue(2, collapseSeq(collapseMappableAS, collapseASTrans)))...)
	payload := baseTestBody(attrs, collapseNLRI)

	out, discards := collapseRun(t, payload, false)
	require.NotNil(t, out)
	assert.Empty(t, discards)

	asPath, ok := collapseAttrValue(t, out, attribute.AttrASPath)
	require.True(t, ok)
	assert.Equal(t, collapsePathValue(4, collapseSeq(collapseMappableAS, collapseASTrans)), asPath,
		"AS_TRANS stays AS_TRANS when no AS4_PATH carries the value it stands for")
	_, hasAS4Path := collapseAttrValue(t, out, attribute.AttrAS4Path)
	assert.False(t, hasAS4Path, "no AS4_PATH is invented")
}

// TestCollapseAS4IgnoresOversizedAS4Path is RFC 6793 Section 4.2.3's first arm,
// and it is a bound as much as a rule: without it a peer sending a long
// AS4_PATH lengthens every path ze stores and relays.
//
// VALIDATES: AC-6 and R-4.
// PREVENTS: a path-length inflation a peer chooses, which changes route
// selection (RFC 4271 Section 9.1.2.2) on every speaker downstream of ze.
func TestCollapseAS4IgnoresOversizedAS4Path(t *testing.T) {
	attrs := collapseWellKnown()
	attrs = append(attrs, collapseAttrWire(attribute.FlagTransitive, attribute.AttrASPath,
		collapsePathValue(2, collapseSeq(collapseMappableAS, collapseASTrans)))...)
	attrs = append(attrs, collapseAttrWire(attribute.FlagOptional|attribute.FlagTransitive,
		attribute.AttrAS4Path, collapsePathValue(4,
			collapseSeq(collapseMappableAS, collapseRealAS, collapseOtherAS)))...)
	payload := baseTestBody(attrs, collapseNLRI)

	out, _ := collapseRun(t, payload, false)
	require.NotNil(t, out)

	asPath, ok := collapseAttrValue(t, out, attribute.AttrASPath)
	require.True(t, ok)
	assert.Equal(t, collapsePathValue(4, collapseSeq(collapseMappableAS, collapseASTrans)), asPath,
		"RFC 6793 Section 4.2.3: an AS4_PATH holding more AS numbers than the AS_PATH SHALL be ignored")
}

// TestCollapseAS4AppliesConfedAdjacencyRule covers the confederation sentence of
// RFC 6793 Section 4.2.3, whose two halves need one fixture each: a leading
// confederation segment is prepended, and one further along the path is not.
//
// VALIDATES: AC-7.
// PREVENTS: a confederation segment being dropped from, or invented into, a
// reconstructed path, which either leaks a member AS beyond the confederation
// or breaks loop detection inside it.
func TestCollapseAS4AppliesConfedAdjacencyRule(t *testing.T) {
	confed := collapseSeg{kind: attribute.ASConfedSequence, asns: []uint32{65001, 65002}}

	tests := []struct {
		name    string
		asPath  []collapseSeg
		as4Path []collapseSeg
		want    []collapseSeg
	}{
		{
			name:    "leading confederation segment is prepended",
			asPath:  []collapseSeg{confed, collapseSeq(collapseMappableAS, collapseASTrans)},
			as4Path: []collapseSeg{collapseSeq(collapseRealAS)},
			// The AS4_PATH holds one AS number and the AS_PATH two, so one
			// leading AS number is taken. The confederation segment is the
			// leading segment, so it is prepended and spends none of the budget
			// (RFC 5065 counts no AS number for it).
			want: []collapseSeg{confed, collapseSeq(collapseMappableAS), collapseSeq(collapseRealAS)},
		},
		{
			name:    "a confederation segment adjacent to nothing prepended is dropped",
			asPath:  []collapseSeg{collapseSeq(collapseMappableAS, collapseASTrans), confed},
			as4Path: []collapseSeg{collapseSeq(collapseRealAS, collapseOtherAS)},
			// Both counts are two, so no leading AS number is taken and the walk
			// stops before the confederation segment: it is adjacent to nothing
			// prepended.
			want: []collapseSeg{collapseSeq(collapseRealAS, collapseOtherAS)},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			attrs := collapseWellKnown()
			attrs = append(attrs, collapseAttrWire(attribute.FlagTransitive, attribute.AttrASPath,
				collapsePathValue(2, test.asPath...))...)
			attrs = append(attrs, collapseAttrWire(attribute.FlagOptional|attribute.FlagTransitive,
				attribute.AttrAS4Path, collapsePathValue(4, test.as4Path...))...)

			out, _ := collapseRun(t, baseTestBody(attrs, collapseNLRI), false)
			require.NotNil(t, out)

			asPath, ok := collapseAttrValue(t, out, attribute.AttrASPath)
			require.True(t, ok)
			assert.Equal(t, collapsePathValue(4, test.want...), asPath,
				"RFC 6793 Section 4.2.3: a valid AS_CONFED_SEQUENCE or AS_CONFED_SET path segment SHALL be prepended if it is either the leading path segment or is adjacent to a path segment that is prepended")
		})
	}
}

// TestCollapseAS4SelectsAggregatorPerSection423 covers both arms of the
// AGGREGATOR gate, which decides the aggregating node AND whether the AS4_PATH
// is used at all.
//
// VALIDATES: AC-4 and AC-5.
// PREVENTS: the two arms being read as one. An AGGREGATOR that is not AS_TRANS
// silences the AS4_PATH as well as the AS4_AGGREGATOR, and a fix that only
// chose the aggregating node would still merge a path the RFC says to ignore.
func TestCollapseAS4SelectsAggregatorPerSection423(t *testing.T) {
	as4Path := collapsePathValue(4, collapseSeq(collapseMappableAS, collapseRealAS))
	asPath2 := collapsePathValue(2, collapseSeq(collapseMappableAS, collapseASTrans))

	tests := []struct {
		name          string
		aggregator    []byte
		wantAggregate []byte
		wantPath      []byte
	}{
		{
			name:       "an AGGREGATOR that is not AS_TRANS ignores both AS4 attributes",
			aggregator: collapseAggValue(2, collapseMappableAS),
			// RFC 6793 Section 4.2.3: the AGGREGATOR is taken as the information
			// about the aggregating node, widened to four octets because nothing
			// downstream is an OLD speaker.
			wantAggregate: collapseAggValue(4, collapseMappableAS),
			// AC-4: the AS4_PATH is ignored, so the AS_PATH is widened rather
			// than merged and AS_TRANS stays where it stood.
			wantPath: collapsePathValue(4, collapseSeq(collapseMappableAS, collapseASTrans)),
		},
		{
			name:          "an AGGREGATOR carrying AS_TRANS promotes the AS4_AGGREGATOR",
			aggregator:    collapseAggValue(2, collapseASTrans),
			wantAggregate: collapseAggValue(4, collapseRealAS),
			wantPath:      collapsePathValue(4, collapseSeq(collapseMappableAS, collapseRealAS)),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			attrs := collapseWellKnown()
			attrs = append(attrs, collapseAttrWire(attribute.FlagTransitive, attribute.AttrASPath, asPath2)...)
			attrs = append(attrs, collapseAttrWire(attribute.FlagOptional|attribute.FlagTransitive,
				attribute.AttrAggregator, test.aggregator)...)
			attrs = append(attrs, collapseAttrWire(attribute.FlagOptional|attribute.FlagTransitive,
				attribute.AttrAS4Path, as4Path)...)
			attrs = append(attrs, collapseAttrWire(attribute.FlagOptional|attribute.FlagTransitive,
				attribute.AttrAS4Aggregator, collapseAggValue(4, collapseRealAS))...)

			out, _ := collapseRun(t, baseTestBody(attrs, collapseNLRI), false)
			require.NotNil(t, out)

			aggregator, ok := collapseAttrValue(t, out, attribute.AttrAggregator)
			require.True(t, ok, "the collapsed payload must carry an aggregating node")
			assert.Equal(t, test.wantAggregate, aggregator)

			asPath, ok := collapseAttrValue(t, out, attribute.AttrASPath)
			require.True(t, ok)
			assert.Equal(t, test.wantPath, asPath)

			_, hasAS4Agg := collapseAttrValue(t, out, attribute.AttrAS4Aggregator)
			assert.False(t, hasAS4Agg, "RFC 6793 Section 4.1: no AS4_AGGREGATOR survives ingest")
			_, hasAS4Path := collapseAttrValue(t, out, attribute.AttrAS4Path)
			assert.False(t, hasAS4Path, "RFC 6793 Section 4.1: no AS4_PATH survives ingest")
		})
	}
}

// TestCollapseAS4RewritesAggregatorToFourOctets covers the AGGREGATOR an OLD
// speaker sends alone, with no AS4_AGGREGATOR to choose against. This is where
// the 2 to 4 widening cases of aspath_transcode_test.go land (R-6).
//
// VALIDATES: AC-5's AGGREGATOR half for the unpaired case.
// PREVENTS: a two-octet AGGREGATOR reaching a consumer that reads it at four
// octets, which turns 65100 into a value in the 4.2 billion range.
func TestCollapseAS4RewritesAggregatorToFourOctets(t *testing.T) {
	attrs := collapseWellKnown()
	attrs = append(attrs, collapseAttrWire(attribute.FlagTransitive, attribute.AttrASPath,
		collapsePathValue(2, collapseSeq(collapseMappableAS)))...)
	attrs = append(attrs, collapseAttrWire(attribute.FlagOptional|attribute.FlagTransitive,
		attribute.AttrAggregator, collapseAggValue(2, collapseMappableAS))...)

	out, discards := collapseRun(t, baseTestBody(attrs, collapseNLRI), false)
	require.NotNil(t, out)
	assert.Empty(t, discards, "an unpaired AGGREGATOR chooses against nothing, so nothing is dropped")

	aggregator, ok := collapseAttrValue(t, out, attribute.AttrAggregator)
	require.True(t, ok)
	assert.Equal(t, collapseAggValue(4, collapseMappableAS), aggregator,
		"RFC 6793 Section 3: the AGGREGATOR carries a four-octet AS number between NEW BGP speakers")
}

// TestCollapseAS4DropsALoneAS4Aggregator covers the AS4_AGGREGATOR arriving
// with no AGGREGATOR beside it. RFC 6793 Section 4.2.3 writes the promotion
// rule for the PAIR, so an unpaired AS4_AGGREGATOR is not the aggregating node
// and Section 4.1 leaves it nowhere to go: it is dropped and the drop is
// reported (attribute.TestRFC6793LoneAS4AggregatorIsDropped is the same rule at
// the value level).
//
// VALIDATES: the payload consequence of that rule, which is that the collapsed
// UPDATE records no aggregating node at all.
// PREVENTS: an aggregating node ze invented from one half of a pair, published
// to every destination as if the peer had claimed it.
func TestCollapseAS4DropsALoneAS4Aggregator(t *testing.T) {
	attrs := collapseWellKnown()
	attrs = append(attrs, collapseAttrWire(attribute.FlagTransitive, attribute.AttrASPath,
		collapsePathValue(2, collapseSeq(collapseMappableAS)))...)
	attrs = append(attrs, collapseAttrWire(attribute.FlagOptional|attribute.FlagTransitive,
		attribute.AttrAS4Aggregator, collapseAggValue(4, collapseRealAS))...)

	out, discards := collapseRun(t, baseTestBody(attrs, collapseNLRI), false)
	require.NotNil(t, out)

	_, hasAggregator := collapseAttrValue(t, out, attribute.AttrAggregator)
	assert.False(t, hasAggregator, "the rule that promotes an AS4_AGGREGATOR is written for the pair")
	_, hasAS4Agg := collapseAttrValue(t, out, attribute.AttrAS4Aggregator)
	assert.False(t, hasAS4Agg, "RFC 6793 Section 4.1: no AS4_AGGREGATOR survives ingest")

	require.Len(t, discards, 1, "an attribute went, so a line is owed to the operator log")
	assert.Equal(t, attribute.AttrAS4Aggregator, discards[0].Code)
	assert.Equal(t, collapseNLRI, collapseTrailer(t, out),
		"an attribute section that shrank must still leave the NLRI where the length field says it is")
}

// TestCollapseAS4DiscardsMalformedAS4Path is RFC 6793 Section 6: the attribute
// is dropped, the UPDATE continues, and the reason comes back for the log.
//
// VALIDATES: AC-8's collapse half. The discard is REPORTED rather than
// returned as a failure, which is what lets the caller log it and keep going.
// PREVENTS: a peer stopping the processing of an UPDATE, or ze silently
// dropping an attribute the operator has no way to see was dropped.
func TestCollapseAS4DiscardsMalformedAS4Path(t *testing.T) {
	// One segment claiming two AS numbers and carrying one.
	malformed := []byte{byte(attribute.ASSequence), 2, 0, 0, 0, 1}
	attrs := collapseWellKnown()
	attrs = append(attrs, collapseAttrWire(attribute.FlagTransitive, attribute.AttrASPath,
		collapsePathValue(2, collapseSeq(collapseMappableAS, collapseASTrans)))...)
	attrs = append(attrs, collapseAttrWire(attribute.FlagOptional|attribute.FlagTransitive,
		attribute.AttrAS4Path, malformed)...)

	out, discards := collapseRun(t, baseTestBody(attrs, collapseNLRI), false)
	require.NotNil(t, out, "RFC 6793 Section 6: the UPDATE continues to be processed")

	require.Len(t, discards, 1, "one attribute was dropped, so one line is owed to the log")
	assert.Equal(t, attribute.AttrAS4Path, discards[0].Code)
	require.Error(t, discards[0].Reason, "the log line names the parse error, so an operator can analyze it")

	asPath, ok := collapseAttrValue(t, out, attribute.AttrASPath)
	require.True(t, ok)
	assert.Equal(t, collapsePathValue(4, collapseSeq(collapseMappableAS, collapseASTrans)), asPath,
		"the received AS_PATH is taken as the AS path information when the AS4_PATH cannot be read")
}

// TestCollapseAS4DiscardsAS4AttributesFromNewSpeaker is the receive obligation
// of RFC 6793 Section 4.1, and it is the row rfc/short/rfc6793.md records as
// RFC6793-4.1-7.
//
// VALIDATES: AC-3 at the payload level. Both attributes go, the AS_PATH is
// untouched, and the two drops are reported separately.
// PREVENTS: a peer rewriting every path ze holds by sending an attribute the
// RFC forbids it to send. Today's ingest merges it because the merge consults
// no width.
func TestCollapseAS4DiscardsAS4AttributesFromNewSpeaker(t *testing.T) {
	asPath4 := collapsePathValue(4, collapseSeq(collapseMappableAS, collapseRealAS))
	aggregator4 := collapseAggValue(4, collapseMappableAS)

	attrs := collapseWellKnown()
	attrs = append(attrs, collapseAttrWire(attribute.FlagTransitive, attribute.AttrASPath, asPath4)...)
	attrs = append(attrs, collapseAttrWire(attribute.FlagOptional|attribute.FlagTransitive,
		attribute.AttrAggregator, aggregator4)...)
	attrs = append(attrs, collapseAttrWire(attribute.FlagOptional|attribute.FlagTransitive,
		attribute.AttrAS4Path, collapsePathValue(4, collapseSeq(65200, 65201, 65202)))...)
	attrs = append(attrs, collapseAttrWire(attribute.FlagOptional|attribute.FlagTransitive,
		attribute.AttrAS4Aggregator, collapseAggValue(4, collapseRealAS))...)

	out, discards := collapseRun(t, baseTestBody(attrs, collapseNLRI), true)
	require.NotNil(t, out, "the two attributes have to be removed, so the payload is rewritten")

	asPath, ok := collapseAttrValue(t, out, attribute.AttrASPath)
	require.True(t, ok)
	assert.Equal(t, asPath4, asPath,
		"RFC 6793 Section 4.1: the attribute is DISCARDED, so nothing is merged into the AS_PATH")
	aggregator, ok := collapseAttrValue(t, out, attribute.AttrAggregator)
	require.True(t, ok)
	assert.Equal(t, aggregator4, aggregator,
		"the AS4_AGGREGATOR is discarded rather than chosen, so the received AGGREGATOR stands")

	_, hasAS4Path := collapseAttrValue(t, out, attribute.AttrAS4Path)
	assert.False(t, hasAS4Path)
	_, hasAS4Agg := collapseAttrValue(t, out, attribute.AttrAS4Aggregator)
	assert.False(t, hasAS4Agg)

	codes := make([]attribute.AttributeCode, 0, len(discards))
	for _, discard := range discards {
		codes = append(codes, discard.Code)
	}
	assert.Equal(t, []attribute.AttributeCode{attribute.AttrAS4Path, attribute.AttrAS4Aggregator}, codes,
		"each discarded attribute is reported, so each earns its own log line")
}

// TestCollapseAS4RefusesAnUnreadableASPath is the fail-closed case: an AS_PATH
// that does not parse at the width its sender negotiated has no four-octet
// form, so the collapse errors rather than publish a half-rewritten payload.
//
// VALIDATES: the Security Review "Fail closed" row.
// PREVENTS: a payload whose AS_PATH is the received two-octet bytes while its
// context claims four, which every consumer downstream would misread.
func TestCollapseAS4RefusesAnUnreadableASPath(t *testing.T) {
	unreadable := []byte{byte(attribute.ASSequence), 3, 0, 1, 0, 2}
	attrs := collapseWellKnown()
	attrs = append(attrs, collapseAttrWire(attribute.FlagTransitive, attribute.AttrASPath, unreadable)...)
	payload := baseTestBody(attrs, collapseNLRI)

	dst := make([]byte, CollapseAS4FamilySize(payload))
	n, _, err := CollapseAS4Family(dst, payload, false)

	require.Error(t, err, "no four-octet form of this AS_PATH exists, so there is nothing to publish")
	assert.ErrorIs(t, err, attribute.ErrASPathUnreadable)
	assert.Zero(t, n, "a refused collapse writes no payload the caller could dispatch by mistake")
}

// TestCollapseAS4RefusesATruncatedPayload covers the three length fields a peer
// controls, each cut one octet short of what it declares.
//
// VALIDATES: the Security Review "Input validation" row.
// PREVENTS: an index past the end of a peer-supplied buffer, which is a panic
// and therefore a way for one peer to stop the daemon for every peer.
func TestCollapseAS4RefusesATruncatedPayload(t *testing.T) {
	attrs := collapseWellKnown()
	attrs = append(attrs, collapseAttrWire(attribute.FlagTransitive, attribute.AttrASPath,
		collapsePathValue(2, collapseSeq(collapseMappableAS)))...)
	full := baseTestBody(attrs, collapseNLRI)

	// An AS_PATH declaring one octet more of value than the attribute section
	// holds. Those octets exist in the payload, because the NLRI follows the
	// section, so only a bound taken at the SECTION end refuses this one.
	overlong := collapseWellKnown()
	asPathValue := collapsePathValue(2, collapseSeq(collapseMappableAS))
	overlong = append(overlong, byte(attribute.FlagTransitive), byte(attribute.AttrASPath), byte(len(asPathValue)+1))
	overlong = append(overlong, asPathValue...)

	tests := []struct {
		name    string
		payload []byte
	}{
		{name: "shorter than the two length fields", payload: full[:3]},
		{name: "attribute section cut short", payload: full[:len(full)-len(collapseNLRI)-1]},
		{name: "attribute value overflows the section", payload: baseTestBody(overlong, collapseNLRI)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dst := make([]byte, CollapseAS4FamilySize(test.payload))
			n, _, err := CollapseAS4Family(dst, test.payload, false)
			require.Error(t, err, "a payload that does not describe itself is refused, never guessed at")
			assert.Zero(t, n)
		})
	}
}

// TestCollapseAS4RefusesABufferItCannotFillSafely proves the collapse fails
// closed on a caller that sized its buffer by other arithmetic than
// CollapseAS4FamilySize.
//
// VALIDATES: the Security Review "Buffer bounds" row.
// PREVENTS: a write past the end of dst. That is an index panic on the session
// read goroutine, which ends the daemon for every peer, and it would be reached
// through a payload a peer chose the length of.
func TestCollapseAS4RefusesABufferItCannotFillSafely(t *testing.T) {
	attrs := collapseWellKnown()
	attrs = append(attrs, collapseAttrWire(attribute.FlagTransitive, attribute.AttrASPath,
		collapsePathValue(2, collapseSeq(collapseMappableAS, collapseASTrans)))...)
	payload := baseTestBody(attrs, collapseNLRI)

	n, _, err := CollapseAS4Family(make([]byte, len(payload)), payload, false)

	require.Error(t, err, "a buffer sized from the RECEIVED payload cannot hold the widened one")
	assert.Zero(t, n, "a refused collapse writes no payload the caller could dispatch by mistake")
}

// TestCollapseAS4WidensTheLongestASPathAPeerCanSend is the boundary case R-3
// names: an AS_PATH filling a standard UPDATE doubles in size when it is
// widened, and the collapse must size its output from the widened value rather
// than from the received one.
//
// VALIDATES: the Boundary Tests row "Collapsed UPDATE body size".
// PREVENTS: a truncated AS_PATH, which is a malformed attribute value ze would
// then relay to every peer at once.
func TestCollapseAS4WidensTheLongestASPathAPeerCanSend(t *testing.T) {
	// One AS_SEQUENCE holds at most 255 AS numbers, so the longest path a
	// standard 4096-octet UPDATE can carry is a run of full segments.
	const segments = 7
	const perSegment = 255

	var asPathSegs []collapseSeg
	want := make([]collapseSeg, 0, segments)
	for segment := range segments {
		asns := make([]uint32, perSegment)
		for i := range asns {
			asns[i] = uint32(1000 + segment*perSegment + i) //nolint:gosec // every value is far inside two octets
		}
		asPathSegs = append(asPathSegs, collapseSeq(asns...))
		want = append(want, collapseSeq(asns...))
	}

	attrs := collapseWellKnown()
	attrs = append(attrs, collapseAttrWire(attribute.FlagTransitive, attribute.AttrASPath,
		collapsePathValue(2, asPathSegs...))...)
	payload := baseTestBody(attrs, collapseNLRI)
	require.Less(t, len(payload), 4077, "the fixture must fit a standard UPDATE a peer can send")

	out, _ := collapseRun(t, payload, false)
	require.NotNil(t, out)

	asPath, ok := collapseAttrValue(t, out, attribute.AttrASPath)
	require.True(t, ok)
	assert.Equal(t, collapsePathValue(4, want...), asPath,
		"every AS number widens, and the extended-length header carries the result")
	assert.Equal(t, collapseNLRI, collapseTrailer(t, out), "the NLRI still follows the widened section")
}

// BenchmarkCollapseAS4FastPath measures what a four-octet fleet pays for the
// RFC 6793 transition machinery. Its ceiling is registered at 0 in
// internal/perf/allocgate.go, so a regression is a red gate rather than a
// review question (AC-9, R-1).
func BenchmarkCollapseAS4FastPath(b *testing.B) {
	attrs := collapseWellKnown()
	attrs = append(attrs, collapseAttrWire(attribute.FlagTransitive, attribute.AttrASPath,
		collapsePathValue(4, collapseSeq(collapseMappableAS, collapseRealAS)))...)
	attrs = append(attrs, collapseAttrWire(attribute.FlagOptional|attribute.FlagTransitive,
		attribute.AttrCommunity, []byte{0xfd, 0xe9, 0x00, 0x07})...)
	payload := baseTestBody(attrs, collapseNLRI)
	dst := make([]byte, CollapseAS4FamilySize(payload))

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		n, _, err := CollapseAS4Family(dst, payload, true)
		if n != 0 || err != nil {
			b.Fatalf("fast path answered n=%d err=%v", n, err)
		}
	}
}

// FuzzCollapseAS4 drives arbitrary peer-supplied bytes at the collapse.
//
// It runs on every received UPDATE from every peer, so the properties asserted
// are the two a hostile peer could break: it never panics, and it never writes
// past the bound CollapseAS4FamilySize promised the caller. A fuzz target
// proves defects are present rather than absent, so the arms above still carry
// the correctness assertions.
func FuzzCollapseAS4(f *testing.F) {
	attrs := collapseWellKnown()
	attrs = append(attrs, collapseAttrWire(attribute.FlagTransitive, attribute.AttrASPath,
		collapsePathValue(2, collapseSeq(collapseMappableAS, collapseASTrans)))...)
	f.Add(baseTestBody(attrs, collapseNLRI))

	attrs = append(attrs, collapseAttrWire(attribute.FlagOptional|attribute.FlagTransitive,
		attribute.AttrAS4Path, collapsePathValue(4, collapseSeq(collapseMappableAS, collapseRealAS)))...)
	f.Add(baseTestBody(attrs, collapseNLRI))

	attrs = append(attrs, collapseAttrWire(attribute.FlagOptional|attribute.FlagTransitive,
		attribute.AttrAS4Aggregator, collapseAggValue(4, collapseRealAS))...)
	f.Add(baseTestBody(attrs, collapseNLRI))
	f.Add([]byte{})
	f.Add([]byte{0x00, 0x00, 0x00, 0x00})

	f.Fuzz(func(t *testing.T, payload []byte) {
		for _, srcASN4 := range []bool{false, true} {
			size := CollapseAS4FamilySize(payload)
			dst := make([]byte, size)
			n, _, err := CollapseAS4Family(dst, payload, srcASN4)
			if err != nil {
				if n != 0 {
					t.Fatalf("a refused collapse wrote %d octets", n)
				}
				continue
			}
			if n > size {
				t.Fatalf("collapse wrote %d octets into a buffer sized %d", n, size)
			}
		}
	})
}
