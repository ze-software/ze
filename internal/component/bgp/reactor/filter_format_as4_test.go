// Design: docs/architecture/api/commands.md -- show policy test, its text-before and text-after
// RFC: rfc/short/rfc6793.md -- AS path reconstruction on receive (Section 4.2.3)
// Related: filter_format.go -- asPathForFilter, the producer under test
// Related: policy_dryrun_display.go -- displayUpdate, the dry-run display that shows it

package reactor

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// as4FilterNonMappable does not fit in two octets, so a two-octet AS_PATH
// carries AS_TRANS in its place and the real number travels in AS4_PATH
// (RFC 6793 Section 4.2.2).
const as4FilterNonMappable uint32 = 199524

// as4FilterAttr wraps one attribute value in its wire header.
func as4FilterAttr(flags attribute.AttributeFlags, code attribute.AttributeCode, value []byte) []byte {
	buf := make([]byte, 3+len(value))
	attribute.WriteHeaderTo(buf, 0, flags, code, uint16(len(value))) //nolint:gosec // test data, bounded
	copy(buf[3:], value)
	return buf
}

// as4FilterPath2 encodes one AS_SEQUENCE with two-octet AS numbers, the
// encoding an OLD speaker sends.
func as4FilterPath2(asns ...uint32) []byte {
	value := make([]byte, 2+len(asns)*2)
	value[0] = byte(attribute.ASSequence)
	value[1] = byte(len(asns))
	for i, asn := range asns {
		binary.BigEndian.PutUint16(value[2+i*2:], uint16(asn)) //nolint:gosec // test data, mappable by construction
	}
	return value
}

// as4FilterPath4 encodes one AS_SEQUENCE with four-octet AS numbers, the
// encoding AS4_PATH always uses and a NEW session's AS_PATH uses.
func as4FilterPath4(asns ...uint32) []byte {
	value := make([]byte, 2+len(asns)*4)
	value[0] = byte(attribute.ASSequence)
	value[1] = byte(len(asns))
	for i, asn := range asns {
		binary.BigEndian.PutUint32(value[2+i*4:], asn)
	}
	return value
}

// as4FilterConfedThenSeq2 encodes an AS_CONFED_SEQUENCE followed by an
// AS_SEQUENCE, both with two-octet AS numbers. This is the AS_PATH shape a
// speaker inside a confederation sends to an OLD peer: RFC 5065 puts the
// confederation segment at the head, and RFC 6793 Section 3 forbids it in the
// AS4_PATH beside it.
func as4FilterConfedThenSeq2(confed, seq []uint32) []byte {
	value := make([]byte, 0, 4+len(confed)*2+len(seq)*2)
	value = append(value, byte(attribute.ASConfedSequence), byte(len(confed)))
	for _, asn := range confed {
		value = binary.BigEndian.AppendUint16(value, uint16(asn)) //nolint:gosec // test data, mappable by construction
	}
	value = append(value, byte(attribute.ASSequence), byte(len(seq)))
	for _, asn := range seq {
		value = binary.BigEndian.AppendUint16(value, uint16(asn)) //nolint:gosec // test data, mappable by construction
	}
	return value
}

// as4FilterUpdate builds the UPDATE a session of the given ASN4 width would
// hand the dry run: the attributes, no NLRI, on an encoding context of that
// width.
func as4FilterUpdate(t *testing.T, asn4 bool, attrs ...[]byte) *wireu.WireUpdate {
	t.Helper()
	var packed []byte
	for _, a := range attrs {
		packed = append(packed, a...)
	}
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(asn4))
	require.NoError(t, err)
	return wireu.NewWireUpdate(buildUpdatePayload(packed, nil), ctxID)
}

// as4FilterDisplay answers the line show policy test prints for update, as
// displayUpdate renders it.
func as4FilterDisplay(t *testing.T, update *wireu.WireUpdate) string {
	t.Helper()
	display, err := displayUpdate(update)
	require.NoError(t, err)
	return display.text()
}

// as4FilterNextHop is the companion attribute every fixture below carries, so
// the display holds more than the AS path and the separator is exercised.
//
// NEXT_HOP rather than ORIGIN, and the reason is now historical. The ORIGIN,
// MULTI_EXIT_DISC, LOCAL_PREF, ATOMIC_AGGREGATE and CLUSTER_LIST arms of
// appendSingleAttr each named a pointer type no parser builds, so those five
// attributes were never shown and could not serve as a companion here
// (plan/journal/silent-fall-through.md, 2026-08-15 and 2026-09-03). The arms
// name the value form since 2026-09-04, so any of the five would serve now.
// NEXT_HOP stays because every assertion below is written against it.
func as4FilterNextHop() []byte {
	return as4FilterAttr(attribute.FlagTransitive, attribute.AttrNextHop, []byte{10, 0, 0, 1})
}

// TestFilterSubjectShowsSemanticASPath drives the dry-run display with the
// UPDATE an OLD speaker sends: a two-octet AS_PATH carrying AS_TRANS where a
// four-octet AS belongs, and an AS4_PATH carrying the real numbers.
//
// VALIDATES: show policy test shows the ASNs the route really traversed,
// whatever ASN width the session negotiated. A display showing 23456 tells
// the operator a reject-asn, as-path-list or as-path-length rule naming a
// four-octet ASN should accept the route it rejects.
// PREVENTS: the display rendering AS_PATH alone, which is the encoding and
// not the AS path information (RFC 6793 Section 4.2.3).
func TestFilterSubjectShowsSemanticASPath(t *testing.T) {
	subject := as4FilterDisplay(t, as4FilterUpdate(t, false,
		as4FilterNextHop(),
		as4FilterAttr(attribute.FlagTransitive, attribute.AttrASPath,
			as4FilterPath2(65001, attribute.ASTrans, 65002)),
		as4FilterAttr(attribute.FlagOptional|attribute.FlagTransitive, attribute.AttrAS4Path,
			as4FilterPath4(65001, as4FilterNonMappable, 65002)),
	))

	assert.Equal(t, "as-path [65001 199524 65002] next-hop 10.0.0.1", subject,
		"the display must carry the ASNs the route traversed, not the AS_TRANS placeholder")
}

// TestFilterSubjectFourOctetSessionUnchanged is the paired positive: a session
// that negotiated four-octet AS carries the real numbers in AS_PATH and sends
// no AS4_PATH, so the display is what it always was.
//
// VALIDATES: the merge is conditional on AS4_PATH being present.
// PREVENTS: a fix that rewrites the AS path on the common path too.
func TestFilterSubjectFourOctetSessionUnchanged(t *testing.T) {
	subject := as4FilterDisplay(t, as4FilterUpdate(t, true,
		as4FilterNextHop(),
		as4FilterAttr(attribute.FlagTransitive, attribute.AttrASPath,
			as4FilterPath4(65001, as4FilterNonMappable, 65002)),
	))

	assert.Equal(t, "as-path [65001 199524 65002] next-hop 10.0.0.1", subject)
}

// TestFilterSubjectShorterASPathIgnoresAS4Path drives the branch RFC 6793
// Section 4.2.3 opens with: an AS4_PATH longer than the AS_PATH cannot be a
// reconstruction of it, so the AS_PATH stands alone.
//
// VALIDATES: the display takes attribute.MergeAS4Path's verdict, which is the
// AS-number-count comparison the section requires.
// PREVENTS: a peer lengthening the path the operator is shown by sending an
// oversized AS4_PATH.
func TestFilterSubjectShorterASPathIgnoresAS4Path(t *testing.T) {
	subject := as4FilterDisplay(t, as4FilterUpdate(t, false,
		as4FilterAttr(attribute.FlagTransitive, attribute.AttrASPath,
			as4FilterPath2(65001, attribute.ASTrans)),
		as4FilterAttr(attribute.FlagOptional|attribute.FlagTransitive, attribute.AttrAS4Path,
			as4FilterPath4(65001, as4FilterNonMappable, 65002)),
	))

	assert.Equal(t, "as-path [65001 23456]", subject,
		"an AS4_PATH holding more AS numbers than the AS_PATH is ignored")
}

// TestFilterSubjectKeepsConfederationHopsOnReconstruction drives the merge with
// an AS_PATH whose leading segment is an AS_CONFED_SEQUENCE, which RFC 6793
// Section 3 forbids the AS4_PATH from carrying, so the AS_PATH is the only
// place those AS numbers exist.
//
// RFC 6793 Section 4.2.3: "Note that a valid AS_CONFED_SEQUENCE or AS_CONFED_SET
// path segment SHALL be prepended if it is either the leading path segment or
// is adjacent to a path segment that is prepended."
//
// VALIDATES: the confederation segment reaches the display whole, and spends
// none of the prepend budget, so 65001 is still taken from the AS_SEQUENCE
// beside it. RFC 5065 counts no AS number for the segment, and the merge
// spends the budget on the same basis it was computed.
// PREVENTS: a merge that charges the budget for confederation AS numbers, which
// cuts the segment at the first hop and drops the AS_SEQUENCE hop that should
// have been taken, so the operator is shown a path missing the member AS
// numbers a confederation-aware rule exists to match.
func TestFilterSubjectKeepsConfederationHopsOnReconstruction(t *testing.T) {
	subject := as4FilterDisplay(t, as4FilterUpdate(t, false,
		as4FilterAttr(attribute.FlagTransitive, attribute.AttrASPath,
			as4FilterConfedThenSeq2([]uint32{64512, 64513}, []uint32{65001, attribute.ASTrans})),
		as4FilterAttr(attribute.FlagOptional|attribute.FlagTransitive, attribute.AttrAS4Path,
			as4FilterPath4(as4FilterNonMappable)),
	))

	assert.Equal(t, "as-path [64512 64513 65001 199524]", subject,
		"the leading confederation segment is prepended whole and costs no budget")
}

// TestFilterSubjectMalformedAS4PathIsDiscarded drives the one branch a peer can
// reach: an AS4_PATH whose value length is odd, which ParseAS4Path refuses.
//
// RFC 6793 Section 6: "A NEW BGP speaker that receives a malformed AS4_PATH
// attribute in an UPDATE message from an OLD BGP speaker MUST discard the
// attribute and continue processing the UPDATE message."
//
// VALIDATES: the malformed attribute is discarded and the UPDATE is still
// displayed, rendered from AS_PATH alone.
// PREVENTS: a parse error emptying the display, or failing the dry run, which
// would hide every other attribute of the route from the operator.
func TestFilterSubjectMalformedAS4PathIsDiscarded(t *testing.T) {
	subject := as4FilterDisplay(t, as4FilterUpdate(t, false,
		as4FilterNextHop(),
		as4FilterAttr(attribute.FlagTransitive, attribute.AttrASPath,
			as4FilterPath2(65001, attribute.ASTrans)),
		// An odd value length can hold no whole four-octet AS number.
		as4FilterAttr(attribute.FlagOptional|attribute.FlagTransitive, attribute.AttrAS4Path,
			[]byte{byte(attribute.ASSequence), 1, 0}),
	))

	assert.Equal(t, "as-path [65001 23456] next-hop 10.0.0.1", subject,
		"the malformed AS4_PATH is discarded and the UPDATE is still rendered")
}
