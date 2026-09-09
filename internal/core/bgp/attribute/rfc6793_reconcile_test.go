// RFC: rfc/short/rfc6793.md -- the receive-side AS path reconciliation of Sections 4.1, 4.2.3 and 6
// RFC: rfc/short/rfc6396.md -- TABLE_DUMP_V2 RIB entries carry 4-byte AS numbers
//
// Drives ReconcileASPathFamily (as4.go), the single declaration of what a
// received UPDATE's AS-path family reduces to. Every case here enters through
// that entry point rather than through canonicalizeASPath, because the
// AGGREGATOR gate and the AS path merge are one procedure in the RFC and only
// the entry point runs both.
//
// The suite moved here from internal/component/bgp/plugins/rib/storage when the
// rule did. It used to drive ParseAttributes, which ran the reconciliation for
// the RIB alone; the ingest collapse now runs it once for the whole daemon, and
// the RIB reads what the collapse produced.

package attribute

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	// ORIGIN IGP, present in every fixture so a reported absence of another
	// attribute is a real absence rather than a section that did not parse.
	wireOriginIGP = []byte{0x40, 0x01, 0x01, 0x00}

	// NEXT_HOP 10.0.0.1.
	wireNextHop = []byte{0x40, 0x03, 0x04, 0x0A, 0x00, 0x00, 0x01}

	// AS_PATH from an OLD speaker: AS_SEQUENCE [65001, AS_TRANS] in two octets.
	// Flags 0x40, type 2, length 6.
	rfc6793WireASPathWithASTrans = []byte{
		0x40, 0x02, 0x06,
		0x02, 0x02, 0xFD, 0xE9, 0x5B, 0xA0,
	}

	// AS4_PATH alongside it: AS_SEQUENCE [65001, 4200000001] in four octets.
	// Flags 0xC0 (optional transitive), type 17, length 10.
	rfc6793WireAS4Path = []byte{
		0xC0, 0x11, 0x0A,
		0x02, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0xFA, 0x56, 0xEA, 0x01,
	}

	// AGGREGATOR from an OLD speaker: two-octet AS_TRANS + 10.0.0.1.
	rfc6793WireAggregatorASTrans = []byte{
		0xC0, 0x07, 0x06,
		0x5B, 0xA0, 0x0A, 0x00, 0x00, 0x01,
	}

	// AS4_AGGREGATOR: four-octet 4200000001 + 10.0.0.1.
	rfc6793WireAS4Aggregator = []byte{
		0xC0, 0x12, 0x08,
		0xFA, 0x56, 0xEA, 0x01, 0x0A, 0x00, 0x00, 0x01,
	}

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

	// AS_PATH from an OLD speaker carrying an AS_SET in its leading part:
	// AS_SEQUENCE [64500], AS_SET [65001, 65002], AS_SEQUENCE [AS_TRANS].
	// RFC 4271 Section 9.1.2.2 counts the set as one, so this path counts three.
	wireASPathWithLeadingSet = []byte{
		0x40, 0x02, 0x0E,
		0x02, 0x01, 0xFB, 0xF4,
		0x01, 0x02, 0xFD, 0xE9, 0xFD, 0xEA,
		0x02, 0x01, 0x5B, 0xA0,
	}

	// AS4_PATH whose segment claims three AS numbers and carries one. The
	// attribute length is honest, so the iterator hands the value over and the
	// AS4_PATH parse is what refuses it.
	wireAS4PathMalformed = []byte{
		0xC0, 0x11, 0x06,
		0x02, 0x03, 0x00, 0x03, 0x0B, 0x64,
	}

	// AGGREGATOR in the four-octet form, which is the wrong width for a session
	// that did not negotiate the four-octet AS capability. RFC 7606 Section 7.7
	// rejects every length but the negotiated one.
	wireAggregatorWrongWidth = []byte{
		0xC0, 0x07, 0x08,
		0x00, 0x00, 0xFB, 0xF4, 0x0A, 0x00, 0x00, 0x01,
	}
)

// concatAttrs concatenates path attributes into one attribute section.
func concatAttrs(slices ...[]byte) []byte {
	total := 0
	for _, s := range slices {
		total += len(s)
	}
	out := make([]byte, 0, total)
	for _, s := range slices {
		out = append(out, s...)
	}
	return out
}

// reconcileSection runs the reconciliation over a whole path-attribute section,
// which is the shape a received UPDATE presents, and answers what it produced.
//
// The split is the one the ingest collapse performs: it walks the section once
// and hands the four values over. Driving the fixtures through the same split
// keeps each case a statement about a received UPDATE rather than about four
// hand-picked byte slices.
func reconcileSection(t *testing.T, raw []byte, sourceASN4 bool) CanonicalASPathFamily {
	t.Helper()

	recv := ReceivedASPathFamily{SourceASN4: sourceASN4}
	iter := NewAttrIterator(raw)
	for code, _, value, ok := iter.Next(); ok; code, _, value, ok = iter.Next() {
		switch code { //nolint:exhaustive // only the four attributes the reconciliation reads
		case AttrASPath:
			recv.ASPath = value
		case AttrAS4Path:
			recv.AS4Path = value
		case AttrAggregator:
			recv.Aggregator = value
		case AttrAS4Aggregator:
			recv.AS4Aggregator = value
		}
	}
	require.Zero(t, iter.Remaining(), "the fixture attribute section must parse")

	out, err := ReconcileASPathFamily(recv)
	require.NoError(t, err, "the fixture AS_PATH must be readable")
	return out
}

// discardedCodes answers the attribute codes a reconciliation reported.
func discardedCodes(family CanonicalASPathFamily) []AttributeCode {
	if len(family.Discards) == 0 {
		return nil
	}
	codes := make([]AttributeCode, 0, len(family.Discards))
	for _, d := range family.Discards {
		codes = append(codes, d.Code)
	}
	return codes
}

// TestRFC6793ReceivedAS4PathAcceptedAlongsideASPath drives an UPDATE from an OLD
// speaker carrying both AS_PATH and AS4_PATH. The four-octet AS4_PATH is taken
// as the AS path information, so the AS_TRANS placeholder never reaches a
// consumer.
//
// RFC requirement: RFC6793-4.2.3-1 positive -- an UPDATE from an OLD speaker carrying an
// AS4_PATH alongside the existing AS_PATH is accepted, and the four-octet AS numbers from the
// AS4_PATH become the route's AS path instead of the AS_TRANS-bearing two-octet AS_PATH.
func TestRFC6793ReceivedAS4PathAcceptedAlongsideASPath(t *testing.T) {
	got := reconcileSection(t, concatAttrs(wireOriginIGP, rfc6793WireASPathWithASTrans, rfc6793WireAS4Path), false)

	assert.Equal(t,
		[]byte{0x02, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0xFA, 0x56, 0xEA, 0x01},
		got.ASPath,
		"the AS4_PATH four-octet AS numbers are the route's AS path")
}

// TestRFC6793ASPathAloneNotInventedIntoFourOctet is the counterpart: without an
// AS4_PATH the two-octet AS_PATH is only widened to the four-octet form, and the
// AS_TRANS placeholder is preserved rather than replaced by a guessed four-octet
// AS.
//
// RFC requirement: RFC6793-4.2.3-1 negative -- when no AS4_PATH accompanies the AS_PATH, no
// four-octet AS numbers are fabricated: the AS_TRANS placeholder is carried through as 23456,
// so the positive case's four-octet path really comes from the received AS4_PATH.
func TestRFC6793ASPathAloneNotInventedIntoFourOctet(t *testing.T) {
	got := reconcileSection(t, concatAttrs(wireOriginIGP, rfc6793WireASPathWithASTrans), false)

	assert.Equal(t,
		[]byte{0x02, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0x5B, 0xA0},
		got.ASPath,
		"AS_TRANS stays AS_TRANS when no AS4_PATH is present")
}

// TestRFC6793ReceivedAS4AggregatorAccepted drives an UPDATE from an OLD speaker
// carrying AS4_AGGREGATOR alongside AGGREGATOR.
//
// RFC requirement: RFC6793-4.2.3-2 positive -- an UPDATE from an OLD speaker carrying an
// AS4_AGGREGATOR alongside the existing AGGREGATOR is accepted rather than rejected: parsing
// succeeds and the four-octet aggregating AS the pair carries survives ingest.
//
// Which of the two is taken as the aggregating node is RFC6793-4.2.3-3 through -7, proven
// below.
func TestRFC6793ReceivedAS4AggregatorAccepted(t *testing.T) {
	got := reconcileSection(t, concatAttrs(wireOriginIGP, rfc6793WireAggregatorASTrans, rfc6793WireAS4Aggregator), false)

	assert.Equal(t, []byte{0xFA, 0x56, 0xEA, 0x01, 0x0A, 0x00, 0x00, 0x01}, got.Aggregator,
		"the four-octet aggregating AS survives ingest")
}

// TestRFC6793LongerASPathPrependsLeadingHops drives an AS_PATH of three AS
// numbers beside an AS4_PATH of one. The two leading AS numbers of the AS_PATH
// are prepended to the AS4_PATH, so the reconstructed path is three AS numbers
// long and the AS_TRANS placeholder is gone.
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
	got := reconcileSection(t, concatAttrs(wireOriginIGP, wireASPathThreeHops, wireAS4PathOneHop), false)

	assert.Equal(t, []byte{
		0x02, 0x02, 0x00, 0x00, 0xFB, 0xF4, 0x00, 0x00, 0xFD, 0xE9,
		0x02, 0x01, 0x00, 0x03, 0x0B, 0x64,
	}, got.ASPath,
		"the two leading AS_PATH hops are prepended to the AS4_PATH")
}

// TestRFC6793LongerAS4PathIsIgnored drives an AS4_PATH carrying more AS numbers
// than the AS_PATH. The AS4_PATH is peer-supplied and unverifiable, so a longer
// one is discarded rather than trusted.
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
	got := reconcileSection(t, concatAttrs(wireOriginIGP, wireASPathTwoHops, wireAS4PathThreeHops), false)

	assert.Equal(t, []byte{
		0x02, 0x02, 0x00, 0x00, 0xFB, 0xF4, 0x00, 0x00, 0x5B, 0xA0,
	}, got.ASPath,
		"a longer AS4_PATH is ignored and the AS_PATH is taken")
}

// TestRFC6793LeadingConfedSegmentIsPrepended drives an AS_PATH whose leading
// segment is an AS_CONFED_SEQUENCE. The confederation segment counts no AS
// numbers, so it neither consumes the prepend budget nor is dropped: it is
// prepended whole because it leads the path.
//
// RFC requirement: RFC6793-4.2.3-10 positive -- the leading AS_CONFED_SEQUENCE of the AS_PATH is
// prepended whole to the reconstructed path, and because RFC 5065 counts no AS numbers for it,
// the one AS number of prepend budget is still spent on the AS_SEQUENCE hop 64500 that follows
// it.
func TestRFC6793LeadingConfedSegmentIsPrepended(t *testing.T) {
	got := reconcileSection(t, concatAttrs(wireOriginIGP, wireASPathConfedLeading, wireAS4PathOneHop), false)

	assert.Equal(t, []byte{
		0x03, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0xFD, 0xEA,
		0x02, 0x01, 0x00, 0x00, 0xFB, 0xF4,
		0x02, 0x01, 0x00, 0x03, 0x0B, 0x64,
	}, got.ASPath,
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
	got := reconcileSection(t, concatAttrs(wireOriginIGP, wireASPathConfedTrailing, wireAS4PathTwoHops), false)

	assert.Equal(t, []byte{
		0x02, 0x02, 0x00, 0x03, 0x0B, 0x64, 0x00, 0x03, 0x0B, 0x65,
	}, got.ASPath,
		"a confederation segment adjacent to nothing prepended is not prepended")
}

// TestRFC6793AggregatorWithRealASIgnoresAS4Attributes drives an AGGREGATOR
// carrying a real AS beside an AS4_AGGREGATOR and an AS4_PATH. The AGGREGATOR's
// AS is not AS_TRANS, so the OLD speaker that aggregated the route understood
// four-octet AS numbers as little as it understood the AS4_* pair it forwarded:
// both are ignored.
//
// RFC requirement: RFC6793-4.2.3-3 positive -- an AGGREGATOR whose AS is not AS_TRANS makes both
// the AS4_AGGREGATOR and the AS4_PATH ignored: the AS4_AGGREGATOR's four-octet AS is not stored
// with the route, and the AS4_PATH's AS numbers do not reach the route's AS path.
//
// RFC requirement: RFC6793-4.2.3-4 positive -- the received AGGREGATOR is taken as the
// information about the aggregating node, so the aggregating AS is 64500 with address 10.0.0.1,
// written in the four-octet form every consumer downstream of the reconciliation reads.
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
	got := reconcileSection(t, concatAttrs(wireOriginIGP,
		rfc6793WireASPathWithASTrans,
		wireAggregatorRealAS,
		rfc6793WireAS4Path,
		rfc6793WireAS4Aggregator), false)

	assert.Equal(t, []byte{0x00, 0x00, 0xFB, 0xF4, 0x0A, 0x00, 0x00, 0x01}, got.Aggregator,
		"the AGGREGATOR is the aggregating node, in the four-octet form")
	assert.Equal(t, []byte{
		0x02, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0x5B, 0xA0,
	}, got.ASPath,
		"the AS_PATH is taken as the AS path information")
	assert.Equal(t, []AttributeCode{AttrAS4Aggregator}, discardedCodes(got),
		"the ignored AS4_AGGREGATOR is dropped, and the drop is reported rather than silent")
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
	got := reconcileSection(t, concatAttrs(wireOriginIGP,
		rfc6793WireASPathWithASTrans,
		rfc6793WireAggregatorASTrans,
		rfc6793WireAS4Path,
		rfc6793WireAS4Aggregator), false)

	assert.Equal(t, []byte{0xFA, 0x56, 0xEA, 0x01, 0x0A, 0x00, 0x00, 0x01}, got.Aggregator,
		"the AS4_AGGREGATOR is taken as the aggregating node")
	assert.Equal(t, []byte{
		0x02, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0xFA, 0x56, 0xEA, 0x01,
	}, got.ASPath,
		"the reconstruction runs and the four-octet AS replaces AS_TRANS")
	assert.Nil(t, discardedCodes(got),
		"the AS4_AGGREGATOR was used rather than dropped, so nothing is reported")
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
	got := reconcileSection(t, concatAttrs(wireOriginIGP, wireASPathTwoHops, wireAS4PathTwoHops), false)

	assert.Equal(t, []byte{
		0x02, 0x02, 0x00, 0x03, 0x0B, 0x64, 0x00, 0x03, 0x0B, 0x65,
	}, got.ASPath,
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
	got := reconcileSection(t, concatAttrs(wireOriginIGP, wireASPathWithLeadingSet, wireAS4PathOneHop), false)

	assert.Equal(t, []byte{
		0x02, 0x01, 0x00, 0x00, 0xFB, 0xF4,
		0x01, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0xFD, 0xEA,
		0x02, 0x01, 0x00, 0x03, 0x0B, 0x64,
	}, got.ASPath,
		"the set keeps both of its AS numbers and the AS4_PATH follows it")
}

// TestRFC6793MalformedAS4PathIsDiscarded drives an AS4_PATH the parse refuses.
// The UPDATE is still processed and the received AS_PATH is the AS path
// information, widened to four octets.
//
// This is the branch that decides whether one malformed attribute from an OLD
// speaker costs the route or only costs the reconstruction.
//
// RFC requirement: RFC6793-6-1 positive -- a malformed AS4_PATH is discarded and the UPDATE
// continues to be processed, so the route is stored carrying the AS_PATH it arrived with rather
// than being rejected.
func TestRFC6793MalformedAS4PathIsDiscarded(t *testing.T) {
	got := reconcileSection(t, concatAttrs(wireOriginIGP, wireASPathTwoHops, wireAS4PathMalformed), false)

	assert.Equal(t, []byte{
		0x02, 0x02, 0x00, 0x00, 0xFB, 0xF4, 0x00, 0x00, 0x5B, 0xA0,
	}, got.ASPath,
		"the AS_PATH is taken as received, AS_TRANS and all, with no merge")
	require.Len(t, got.Discards, 1, "the discard is reported so the caller can log it")
	assert.Equal(t, AttrAS4Path, got.Discards[0].Code)
	assert.Error(t, got.Discards[0].Reason,
		"RFC 6793 Section 6 asks for the error to be logged, so the reason travels with the report")
}

// TestRFC6793AggregatorOfTheWrongWidthIsNotRead drives an AGGREGATOR whose
// length disagrees with the negotiated AS width, beside an AS4_AGGREGATOR. The
// AS number leading it cannot be read, so nothing decides the RFC's AS_TRANS
// comparison.
//
// Guessing the width would answer the comparison on a value nobody parsed, and
// that comparison also decides whether the AS4_PATH is used.
//
// RFC requirement: RFC6793-4.2.3-3 negative -- an AGGREGATOR whose length does not match the
// negotiated AS width does not make the AS4_PATH ignored: the AS path is still reconstructed,
// and the unreadable value travels on exactly as it arrived rather than being reinterpreted at
// a width nobody negotiated.
func TestRFC6793AggregatorOfTheWrongWidthIsNotRead(t *testing.T) {
	got := reconcileSection(t, concatAttrs(wireOriginIGP, wireASPathThreeHops, wireAS4PathOneHop,
		wireAggregatorWrongWidth, rfc6793WireAS4Aggregator), false)

	assert.Equal(t, []byte{0x00, 0x00, 0xFB, 0xF4, 0x0A, 0x00, 0x00, 0x01}, got.Aggregator,
		"an AGGREGATOR that could not be read travels on unchanged")
	assert.Equal(t, []byte{
		0x02, 0x02, 0x00, 0x00, 0xFB, 0xF4, 0x00, 0x00, 0xFD, 0xE9,
		0x02, 0x01, 0x00, 0x03, 0x0B, 0x64,
	}, got.ASPath,
		"the prepended part stays its own segment, and the AS4_PATH follows it")
	assert.Equal(t, []AttributeCode{AttrAS4Aggregator}, discardedCodes(got),
		"nothing chose the AS4_AGGREGATOR, so it is dropped and the drop is reported")
}

// TestRFC6793LoneAS4AggregatorIsDropped drives an AS4_AGGREGATOR with no
// AGGREGATOR beside it. RFC 6793 Section 4.2.3 rules on the PAIR, so a lone
// attribute leaves nothing to choose between and it is not promoted.
//
// It does not travel on either. RFC 6793 Section 4.1 forbids carrying
// AS4_AGGREGATOR to a NEW BGP speaker, and everything downstream of the
// reconciliation is one, so the attribute is dropped and the drop is reported
// for the operator log rather than performed in silence.
//
// RFC requirement: RFC6793-4.2.3-7 negative -- an AS4_AGGREGATOR arriving without an AGGREGATOR
// is not promoted to the route's aggregating node, because the rule that promotes it is written
// for the pair; nothing records an aggregating node for that route.
func TestRFC6793LoneAS4AggregatorIsDropped(t *testing.T) {
	got := reconcileSection(t, concatAttrs(wireOriginIGP, wireASPathTwoHops, rfc6793WireAS4Aggregator), false)

	assert.Nil(t, got.Aggregator, "a lone AS4_AGGREGATOR is not the aggregating node")
	assert.Equal(t, []AttributeCode{AttrAS4Aggregator}, discardedCodes(got),
		"the drop is reported so an operator sees the attribute go")
}

// TestRFC6793NewSpeakerAS4AttributesAreDiscarded drives the receive obligation
// of RFC 6793 Section 4.1: a NEW speaker sending either AS4_* attribute is
// wrong, and the attribute is discarded rather than merged.
//
// The AS_PATH beside it is already four-octet, so a merge would rewrite a
// correct path from an attribute the sender was forbidden to send. That is the
// direction this test exists to hold: the discard is conditioned on the SOURCE
// being a NEW speaker, never on the destination.
//
// RFC requirement: RFC6793-4.1-7 positive -- a NEW BGP speaker receiving an AS4_PATH or an
// AS4_AGGREGATOR from another NEW BGP speaker discards the path attribute and continues
// processing the UPDATE message, so neither reaches the AS path information or the aggregating
// node.
func TestRFC6793NewSpeakerAS4AttributesAreDiscarded(t *testing.T) {
	fourOctetASPath := []byte{
		0x40, 0x02, 0x0A,
		0x02, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0xFA, 0x56, 0xEA, 0x01,
	}
	fourOctetAggregator := []byte{
		0xC0, 0x07, 0x08,
		0x00, 0x00, 0xFB, 0xF4, 0x0A, 0x00, 0x00, 0x01,
	}
	got := reconcileSection(t, concatAttrs(wireOriginIGP, fourOctetASPath, fourOctetAggregator,
		rfc6793WireAS4Path, rfc6793WireAS4Aggregator), true)

	assert.Equal(t, []byte{0x02, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0xFA, 0x56, 0xEA, 0x01}, got.ASPath,
		"the AS_PATH is already four-octet truth and nothing is merged into it")
	assert.Equal(t, []byte{0x00, 0x00, 0xFB, 0xF4, 0x0A, 0x00, 0x00, 0x01}, got.Aggregator,
		"the AGGREGATOR is already four-octet and stands")
	assert.Equal(t, []AttributeCode{AttrAS4Path, AttrAS4Aggregator}, discardedCodes(got),
		"both attributes are discarded, and both discards are reported")
}

// TestRFC6793UnreadableASPathRefusesReconciliation pins the fail-closed edge: an
// AS_PATH that does not parse at the width its sender negotiated has no
// four-octet form, so no canonical family exists.
//
// VALIDATES: the error return, which a payload rewriter MUST take as a refusal
// rather than publishing bytes whose AS path is half rewritten.
// PREVENTS: a partially rewritten UPDATE reaching a consumer, which is the one
// shape of this change that resets sessions toward every peer at once.
func TestRFC6793UnreadableASPathRefusesReconciliation(t *testing.T) {
	// A segment claiming three AS numbers and carrying one.
	_, err := ReconcileASPathFamily(ReceivedASPathFamily{
		ASPath: []byte{0x02, 0x03, 0xFB, 0xF4},
	})
	require.ErrorIs(t, err, ErrASPathUnreadable,
		"an AS_PATH with no four-octet form must be refused, not guessed at")
}

// TestRFC6396RIBEntryASPathStoredFourByte proves that an AS_PATH received in
// 2-byte encoding, from a session that did not negotiate 4-byte AS support, is
// reduced to 4-byte encoding. That value is what every consumer downstream of
// ingest reads, the RIB included, and a TABLE_DUMP_V2 RIB entry carries it.
//
// RFC 6396 Section 4.3.4: "All AS numbers in the AS_PATH attribute MUST be
// encoded as 4-byte AS numbers."
//
// METHOD: reconcile a two-octet AS_SEQUENCE [65001, 65002] from an OLD speaker,
// which is the only case where the received encoding and the required encoding
// differ. The segment header stays (type 2, count 2) and each AS widens from two
// octets to four.
//
// VALIDATES: the 2-byte to 4-byte widening that the RIB stores and the dump then
// copies.
// PREVENTS: a TABLE_DUMP_V2 file whose RIB entries carry 2-byte AS_PATHs, which
// every reader parses as 4-byte and so mis-decodes into wrong AS numbers.
//
// The MRT-side test for this clause, TestDumpV2RIBEntryASPathIs4Byte
// (internal/plugins/mrt/dump_test.go), hands the dump visitor an AS_PATH it
// built 4-byte itself and asserts it comes back 4-byte, so it measures the
// encoder's transparency rather than this widening.
//
// RFC requirement: RFC6396-4.3.4-1 positive -- every AS number reaching a
// TABLE_DUMP_V2 RIB entry's AS_PATH is 4-byte encoded, because ingest widens a
// 2-byte AS_PATH before anything stores it and the dump path copies the stored
// bytes verbatim.
func TestRFC6396RIBEntryASPathStoredFourByte(t *testing.T) {
	// AS_PATH from a session with no 4-byte AS capability: flags 0x40, type 2,
	// length 6, then AS_SEQUENCE (type 2) of 2 ASes in two octets each.
	wireTwoByteASPath := []byte{
		0x40, 0x02, 0x06,
		0x02, 0x02, 0xFD, 0xE9, 0xFD, 0xEA,
	}
	got := reconcileSection(t, concatAttrs(wireOriginIGP, wireTwoByteASPath), false)

	assert.Equal(t,
		[]byte{0x02, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0xFD, 0xEA},
		got.ASPath,
		"a 2-byte AS_PATH must be reduced to 4-byte encoding, so the MRT RIB entry that copies it is 4-byte")
}

// TestRFC6396RIBEntryASPathFourByteSessionUnchanged is the counterpart: a
// session that DID negotiate 4-byte AS support already sends the required
// encoding, so the widening must not run twice.
//
// VALIDATES: the widening is conditional on the received encoding, not applied
// unconditionally.
// PREVENTS: a doubly widened AS_PATH, which would put an 8-octet AS in a field
// every MRT reader parses as 4.
//
// RFC requirement: RFC6396-4.3.4-1 negative -- the 4-byte requirement is met by
// widening only what arrived narrow: an AS_PATH already in 4-byte encoding is
// stored unchanged rather than widened again.
func TestRFC6396RIBEntryASPathFourByteSessionUnchanged(t *testing.T) {
	// The same path in four octets per AS: flags 0x40, type 2, length 10.
	wireFourByteASPath := []byte{
		0x40, 0x02, 0x0A,
		0x02, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0xFD, 0xEA,
	}
	got := reconcileSection(t, concatAttrs(wireOriginIGP, wireFourByteASPath), true)

	assert.Equal(t,
		[]byte{0x02, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0xFD, 0xEA},
		got.ASPath,
		"an AS_PATH already 4-byte encoded is stored as received")
}

// TestExpandASPath2to4 pins the widening walk itself, including the shapes it
// refuses. A refusal is what makes the reconciliation fail closed rather than
// publish a path it could not read.
func TestExpandASPath2to4(t *testing.T) {
	tests := []struct {
		name   string
		input  []byte
		expect []byte
	}{
		{
			name:   "empty",
			input:  []byte{},
			expect: []byte{},
		},
		{
			name:   "single_sequence_one_asn",
			input:  []byte{2, 1, 0x00, 0x41}, // AS_SEQUENCE, count=1, ASN=65
			expect: []byte{2, 1, 0, 0, 0x00, 0x41},
		},
		{
			name:   "single_sequence_two_asns",
			input:  []byte{2, 2, 0x00, 0x41, 0xFF, 0xFF}, // ASN=65, ASN=65535
			expect: []byte{2, 2, 0, 0, 0x00, 0x41, 0, 0, 0xFF, 0xFF},
		},
		{
			name:   "as_set",
			input:  []byte{1, 1, 0x00, 0x01}, // AS_SET, count=1, ASN=1
			expect: []byte{1, 1, 0, 0, 0x00, 0x01},
		},
		{
			name:   "truncated_segment",
			input:  []byte{2, 3, 0x00, 0x01}, // claims 3 ASNs but only has 1
			expect: nil,
		},
		{
			name:   "trailing_byte",
			input:  []byte{2, 1, 0x00, 0x01, 0xFF}, // valid segment + trailing junk
			expect: nil,
		},
		{
			name:   "zero_count_segment",
			input:  []byte{2, 0}, // AS_SEQUENCE with 0 ASNs
			expect: []byte{2, 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, expandASPath2to4(tt.input))
		})
	}
}

// TestReconcileNoASPathAnswersNothing pins the absent-attribute case: with no
// AS_PATH there is no AS number count to compare an AS4_PATH against and no
// leading part to prepend, so the route records no AS path.
func TestReconcileNoASPathAnswersNothing(t *testing.T) {
	got, err := ReconcileASPathFamily(ReceivedASPathFamily{})
	require.NoError(t, err)
	assert.Nil(t, got.ASPath)
	assert.Nil(t, got.Aggregator)
	assert.Nil(t, got.Discards)
}

// TestReconcileCostsNothingWithoutAS4Path pins the cost of the common case. An
// UPDATE from a session that negotiated four-octet AS support carries no
// AS4_PATH, so the reconciliation must not run and must not allocate. The
// two-octet case pays exactly one allocation, the widening buffer.
func TestReconcileCostsNothingWithoutAS4Path(t *testing.T) {
	asPath4Byte := []byte{0x02, 0x02, 0x00, 0x00, 0xFB, 0xF4, 0x00, 0x00, 0xFD, 0xE9}
	asPath2Byte := []byte{0x02, 0x02, 0xFB, 0xF4, 0xFD, 0xE9}

	allocs := testing.AllocsPerRun(100, func() {
		out, err := ReconcileASPathFamily(ReceivedASPathFamily{ASPath: asPath4Byte, SourceASN4: true})
		if err != nil || out.ASPath == nil {
			t.Fatal("the four-octet AS_PATH must be returned as it is")
		}
	})
	assert.Equal(t, 0.0, allocs, "a four-octet AS_PATH with no AS4_PATH allocates nothing")

	allocs = testing.AllocsPerRun(100, func() {
		out, err := ReconcileASPathFamily(ReceivedASPathFamily{ASPath: asPath2Byte})
		if err != nil || out.ASPath == nil {
			t.Fatal("the two-octet AS_PATH must be widened")
		}
	})
	assert.Equal(t, 1.0, allocs, "widening a two-octet AS_PATH costs one buffer")
}
