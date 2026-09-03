// Design: docs/architecture/api/process-protocol.md -- the filter text protocol
// RFC: rfc/short/rfc6793.md -- AS path reconstruction on receive (Section 4.2.3)
// Related: filter_format.go -- asPathForFilter, the producer under test

package reactor

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// as4FilterWire builds the AttributesWire a session of the given ASN4 width
// would hand the filter chain.
func as4FilterWire(t *testing.T, asn4 bool, attrs ...[]byte) *attribute.AttributesWire {
	t.Helper()
	var packed []byte
	for _, a := range attrs {
		packed = append(packed, a...)
	}
	ctxID, err := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(asn4))
	require.NoError(t, err)
	return attribute.NewAttributesWire(packed, ctxID)
}

// as4FilterNextHop is the companion attribute every fixture below carries, so
// the subject holds more than the AS path and the separator is exercised.
//
// NEXT_HOP rather than ORIGIN: the ORIGIN, MULTI_EXIT_DISC, LOCAL_PREF and
// CLUSTER_LIST arms of appendSingleAttr match a pointer type the parsers never
// produce, so those four attributes reach no filter at all
// (plan/journal/silent-fall-through.md, 2026-08-15 and 2026-09-03).
func as4FilterNextHop() []byte {
	return as4FilterAttr(attribute.FlagTransitive, attribute.AttrNextHop, []byte{10, 0, 0, 1})
}

// TestFilterSubjectShowsSemanticASPath drives the filter text builder with the
// UPDATE an OLD speaker sends: a two-octet AS_PATH carrying AS_TRANS where a
// four-octet AS belongs, and an AS4_PATH carrying the real numbers.
//
// VALIDATES: every text-mode filter judges the ASNs the route really
// traversed, whatever ASN width the session negotiated. A subject showing
// 23456 makes a reject-asn, as-path-list or as-path-length rule naming a
// four-octet ASN accept the leak it exists to reject.
// PREVENTS: the builder rendering AS_PATH alone, which is the encoding and
// not the AS path information (RFC 6793 Section 4.2.3).
func TestFilterSubjectShowsSemanticASPath(t *testing.T) {
	attrs := as4FilterWire(t, false,
		as4FilterNextHop(),
		as4FilterAttr(attribute.FlagTransitive, attribute.AttrASPath,
			as4FilterPath2(65001, attribute.ASTrans, 65002)),
		as4FilterAttr(attribute.FlagOptional|attribute.FlagTransitive, attribute.AttrAS4Path,
			as4FilterPath4(65001, as4FilterNonMappable, 65002)),
	)

	subject := string(AppendAttrsForFilter(nil, attrs, nil))
	assert.Equal(t, "as-path [65001 199524 65002] next-hop 10.0.0.1", subject,
		"the subject must carry the ASNs the route traversed, not the AS_TRANS placeholder")
}

// TestFilterSubjectDeclaredASPathShowsSemanticPath drives the same UPDATE
// through the declared-attribute arm, which a filter registering
// `attributes: ["as-path"]` reaches instead of the render-everything arm.
//
// VALIDATES: both arms of AppendAttrsForFilter share one AS path producer.
// PREVENTS: a fix applied to appendAllAttrs alone, leaving every filter that
// declares its attributes reading the placeholder.
func TestFilterSubjectDeclaredASPathShowsSemanticPath(t *testing.T) {
	attrs := as4FilterWire(t, false,
		as4FilterNextHop(),
		as4FilterAttr(attribute.FlagTransitive, attribute.AttrASPath,
			as4FilterPath2(65001, attribute.ASTrans, 65002)),
		as4FilterAttr(attribute.FlagOptional|attribute.FlagTransitive, attribute.AttrAS4Path,
			as4FilterPath4(65001, as4FilterNonMappable, 65002)),
	)

	subject := string(AppendAttrsForFilter(nil, attrs, []string{policyAttrASPath}))
	assert.Equal(t, "as-path [65001 199524 65002]", subject)
}

// TestFilterSubjectFourOctetSessionUnchanged is the paired positive: a session
// that negotiated four-octet AS carries the real numbers in AS_PATH and sends
// no AS4_PATH, so the subject is what it always was.
//
// VALIDATES: the merge is conditional on AS4_PATH being present.
// PREVENTS: a fix that rewrites the AS path on the common path too.
func TestFilterSubjectFourOctetSessionUnchanged(t *testing.T) {
	attrs := as4FilterWire(t, true,
		as4FilterNextHop(),
		as4FilterAttr(attribute.FlagTransitive, attribute.AttrASPath,
			as4FilterPath4(65001, as4FilterNonMappable, 65002)),
	)

	subject := string(AppendAttrsForFilter(nil, attrs, nil))
	assert.Equal(t, "as-path [65001 199524 65002] next-hop 10.0.0.1", subject)
}

// TestFilterSubjectShorterASPathIgnoresAS4Path drives the branch RFC 6793
// Section 4.2.3 opens with: an AS4_PATH longer than the AS_PATH cannot be a
// reconstruction of it, so the AS_PATH stands alone.
//
// VALIDATES: the builder takes attribute.MergeAS4Path's verdict, which is the
// AS-number-count comparison the section requires.
// PREVENTS: a peer lengthening the path every filter sees by sending an
// oversized AS4_PATH.
func TestFilterSubjectShorterASPathIgnoresAS4Path(t *testing.T) {
	attrs := as4FilterWire(t, false,
		as4FilterAttr(attribute.FlagTransitive, attribute.AttrASPath,
			as4FilterPath2(65001, attribute.ASTrans)),
		as4FilterAttr(attribute.FlagOptional|attribute.FlagTransitive, attribute.AttrAS4Path,
			as4FilterPath4(65001, as4FilterNonMappable, 65002)),
	)

	subject := string(AppendAttrsForFilter(nil, attrs, nil))
	assert.Equal(t, "as-path [65001 23456]", subject,
		"an AS4_PATH holding more AS numbers than the AS_PATH is ignored")
}

// TestFilterSubjectMalformedAS4PathIsDiscarded drives the one branch a peer can
// reach: an AS4_PATH whose value length is odd, which ParseAS4Path refuses.
//
// RFC 6793 Section 6: "A NEW BGP speaker that receives a malformed AS4_PATH
// attribute in an UPDATE message from an OLD BGP speaker MUST discard the
// attribute and continue processing the UPDATE message."
//
// VALIDATES: the malformed attribute is discarded and the UPDATE still reaches
// the filter chain, rendered from AS_PATH alone.
// PREVENTS: a parse error emptying the subject, or dropping the route, which
// would let one bad attribute suppress every filter decision on the session.
func TestFilterSubjectMalformedAS4PathIsDiscarded(t *testing.T) {
	attrs := as4FilterWire(t, false,
		as4FilterNextHop(),
		as4FilterAttr(attribute.FlagTransitive, attribute.AttrASPath,
			as4FilterPath2(65001, attribute.ASTrans)),
		// An odd value length can hold no whole four-octet AS number.
		as4FilterAttr(attribute.FlagOptional|attribute.FlagTransitive, attribute.AttrAS4Path,
			[]byte{byte(attribute.ASSequence), 1, 0}),
	)

	subject := string(AppendAttrsForFilter(nil, attrs, nil))
	assert.Equal(t, "as-path [65001 23456] next-hop 10.0.0.1", subject,
		"the malformed AS4_PATH is discarded and the UPDATE is still rendered")
}

// TestFilterSubjectAllocatesNothingOnTheCommonPath pins the cost of the
// change on the path nearly every UPDATE takes: a session that negotiated
// four-octet AS, carrying no AS4_PATH. The AS4_PATH lookup is answered from
// the span presence bitset, which takes no lock, no scan and no parse.
//
// VALIDATES: the AS path merge costs the common path nothing.
// PREVENTS: a lookup that parses AS4_PATH, or builds a merged path, for every
// UPDATE on every session -- invisible in every correctness test above and
// paid millions of times.
func TestFilterSubjectAllocatesNothingOnTheCommonPath(t *testing.T) {
	attrs := as4FilterWire(t, true,
		as4FilterNextHop(),
		as4FilterAttr(attribute.FlagTransitive, attribute.AttrASPath,
			as4FilterPath4(65001, 65002, 65003)),
	)
	scratch := make([]byte, 0, 4096)

	// Warm the parsed side table and the scratch, so the run measures the
	// builder rather than the one-time lazy parse behind it.
	scratch = AppendUpdateForFilter(scratch[:0], attrs, nil, nil)
	require.Equal(t, "as-path [65001 65002 65003] next-hop 10.0.0.1", string(scratch))

	hits := testing.AllocsPerRun(100, func() {
		scratch = AppendUpdateForFilter(scratch[:0], attrs, nil, nil)
	})
	assert.Zero(t, hits, "the common path must allocate nothing")
}

// TestFilterSubjectAS4PathReconstructionCost pins what the OLD-speaker path
// costs, which is the only path that pays anything. attribute.MergeAS4Path
// builds one ASPath and one segment slice; the parses behind it are cached in
// the AttributesWire side table, so they are not repaid per call.
//
// VALIDATES: the reconstruction is two allocations, not one per AS number.
// PREVENTS: a merge that grows with the path length, which a correctness test
// over a three-hop fixture cannot see.
func TestFilterSubjectAS4PathReconstructionCost(t *testing.T) {
	attrs := as4FilterWire(t, false,
		as4FilterNextHop(),
		as4FilterAttr(attribute.FlagTransitive, attribute.AttrASPath,
			as4FilterPath2(65001, attribute.ASTrans, 65002)),
		as4FilterAttr(attribute.FlagOptional|attribute.FlagTransitive, attribute.AttrAS4Path,
			as4FilterPath4(65001, as4FilterNonMappable, 65002)),
	)
	scratch := make([]byte, 0, 4096)

	scratch = AppendUpdateForFilter(scratch[:0], attrs, nil, nil)
	require.Equal(t, "as-path [65001 199524 65002] next-hop 10.0.0.1", string(scratch))

	hits := testing.AllocsPerRun(100, func() {
		scratch = AppendUpdateForFilter(scratch[:0], attrs, nil, nil)
	})
	t.Logf("AS4_PATH reconstruction costs %.0f allocations per UPDATE", hits)
	assert.Equal(t, 2.0, hits, "the reconstruction is one ASPath and one segment slice")
}
