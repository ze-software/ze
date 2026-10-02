// Design: docs/architecture/edge-cases/as4.md -- the RFC 6793 receive procedure, run once
// RFC: rfc/short/rfc6793.md -- RFC6793-6-3, confederation segments received in an AS4_PATH
// Related: aspath_collapse.go -- CollapseAS4Family, the ingest rewrite under test
//
// These units judge the RECEIVE side of RFC 6793 Section 6: what the ingest
// collapse keeps of an AS4_PATH an OLD speaker sent with confederation
// segments in it. They read the collapsed UPDATE, which is what every
// consumer downstream of the session sees, and never an egress AS4_PATH: the
// AS4_PATH encoder drops confederation segments on its own (RFC6793-3-1), so
// an egress reading cannot tell a receive-side discard from that filter.

package wireu

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rfc6793ReceivedOrigin and rfc6793ReceivedNextHop are the two attributes the
// collapse does not read, written out so each expected payload below is one
// literal a reader can check octet by octet.
var (
	rfc6793ReceivedOrigin  = []byte{0x40, 0x01, 0x01, 0x00}
	rfc6793ReceivedNextHop = []byte{0x40, 0x03, 0x04, 0xC0, 0x00, 0x02, 0xFE}
	rfc6793ReceivedNLRI    = []byte{0x18, 0x0A, 0x00, 0x00}
)

// TestRFC6793ReceivedAS4PathConfedSegmentsDiscarded sends the collapse an
// UPDATE from an OLD speaker whose AS4_PATH carries an AS_CONFED_SEQUENCE and
// an AS_CONFED_SET ahead of its AS_SEQUENCE, and compares the whole collapsed
// payload with the RFC's answer.
//
// RFC 6793 Section 6: "A NEW BGP speaker that receives these path segment
// types in the AS4_PATH attribute of an UPDATE message from an OLD BGP speaker
// MUST discard these path segments, adjust the relevant attribute fields
// accordingly, and continue processing the UPDATE message."
//
// The AS_PATH holds three AS numbers and the AS4_PATH two (confederation
// segments count none, RFC 5065), so Section 4.2.3 takes one AS number from
// the AS_PATH and appends the AS4_PATH segments that survive the discard.
//
// RFC requirement: RFC6793-6-3 positive -- an AS4_PATH from an OLD speaker carrying an
// AS_CONFED_SEQUENCE and an AS_CONFED_SET is collapsed with both segments discarded: the
// reconstructed AS_PATH is exactly AS_SEQUENCE [65001] then AS_SEQUENCE [4200000001,
// 4200000002], its attribute length is 16, the attribute section length is 30, and the
// UPDATE goes on with its NLRI 10.0.0.0/24 and no error.
func TestRFC6793ReceivedAS4PathConfedSegmentsDiscarded(t *testing.T) {
	attrs := concatAttrs(
		rfc6793ReceivedOrigin,
		// AS_PATH from an OLD speaker, two-octet: AS_SEQUENCE [65001, AS_TRANS, AS_TRANS].
		[]byte{0x40, 0x02, 0x08, 0x02, 0x03, 0xFD, 0xE9, 0x5B, 0xA0, 0x5B, 0xA0},
		rfc6793ReceivedNextHop,
		// AS4_PATH, 26 octets: AS_CONFED_SEQUENCE [64512], AS_CONFED_SET {64514, 64515},
		// AS_SEQUENCE [4200000001, 4200000002].
		[]byte{
			0xC0, 0x11, 0x1A,
			0x03, 0x01, 0x00, 0x00, 0xFC, 0x00,
			0x04, 0x02, 0x00, 0x00, 0xFC, 0x02, 0x00, 0x00, 0xFC, 0x03,
			0x02, 0x02, 0xFA, 0x56, 0xEA, 0x01, 0xFA, 0x56, 0xEA, 0x02,
		},
	)
	payload := buildPayload(nil, attrs, rfc6793ReceivedNLRI)

	got, _ := collapseRun(t, payload, false)
	require.NotNil(t, got, "an UPDATE carrying an AS4_PATH from an OLD speaker must be collapsed")

	want := concatAttrs(
		[]byte{0x00, 0x00, 0x00, 0x1E},
		rfc6793ReceivedOrigin,
		[]byte{
			0x40, 0x02, 0x10,
			0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9,
			0x02, 0x02, 0xFA, 0x56, 0xEA, 0x01, 0xFA, 0x56, 0xEA, 0x02,
		},
		rfc6793ReceivedNextHop,
		rfc6793ReceivedNLRI,
	)
	assert.Equal(t, want, got,
		"both confederation segments are discarded, the AS_PATH length and the attribute section length follow, and the NLRI is kept")
}

// TestRFC6793ReceivedAS4PathDiscardTakesOnlyConfedSegments sends the collapse
// the two segments the Section 6 discard must leave alone: an ordinary AS_SET
// inside the AS4_PATH, and an AS_CONFED_SEQUENCE leading the AS_PATH, where a
// confederation member legitimately writes it (RFC 5065) and Section 4.2.3
// prepends it.
//
// RFC requirement: RFC6793-6-3 negative -- the discard takes only confederation segments
// received in the AS4_PATH: an AS_SET {4200000003, 4200000004} in the AS4_PATH and an
// AS_CONFED_SEQUENCE [64512] leading the AS_PATH both survive the collapse, giving exactly
// AS_CONFED_SEQUENCE [64512], AS_SEQUENCE [65001], AS_SET {4200000003, 4200000004},
// AS_SEQUENCE [4200000001].
func TestRFC6793ReceivedAS4PathDiscardTakesOnlyConfedSegments(t *testing.T) {
	attrs := concatAttrs(
		rfc6793ReceivedOrigin,
		// AS_PATH, two-octet: AS_CONFED_SEQUENCE [64512], AS_SEQUENCE [65001, AS_TRANS, AS_TRANS].
		[]byte{
			0x40, 0x02, 0x0C,
			0x03, 0x01, 0xFC, 0x00,
			0x02, 0x03, 0xFD, 0xE9, 0x5B, 0xA0, 0x5B, 0xA0,
		},
		rfc6793ReceivedNextHop,
		// AS4_PATH, 16 octets: AS_SET {4200000003, 4200000004}, AS_SEQUENCE [4200000001].
		[]byte{
			0xC0, 0x11, 0x10,
			0x01, 0x02, 0xFA, 0x56, 0xEA, 0x03, 0xFA, 0x56, 0xEA, 0x04,
			0x02, 0x01, 0xFA, 0x56, 0xEA, 0x01,
		},
	)
	payload := buildPayload(nil, attrs, rfc6793ReceivedNLRI)

	got, _ := collapseRun(t, payload, false)
	require.NotNil(t, got, "an UPDATE carrying an AS4_PATH from an OLD speaker must be collapsed")

	want := concatAttrs(
		[]byte{0x00, 0x00, 0x00, 0x2A},
		rfc6793ReceivedOrigin,
		[]byte{
			0x40, 0x02, 0x1C,
			0x03, 0x01, 0x00, 0x00, 0xFC, 0x00,
			0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9,
			0x01, 0x02, 0xFA, 0x56, 0xEA, 0x03, 0xFA, 0x56, 0xEA, 0x04,
			0x02, 0x01, 0xFA, 0x56, 0xEA, 0x01,
		},
		rfc6793ReceivedNextHop,
		rfc6793ReceivedNLRI,
	)
	assert.Equal(t, want, got,
		"an ordinary AS_SET in the AS4_PATH and a confederation segment in the AS_PATH are not discarded")
}
