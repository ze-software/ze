// RFC: rfc/short/rfc6793.md — receiving AS4_PATH / AS4_AGGREGATOR from an OLD speaker
//
// Drives ParseAttributes (attrparse.go), the ingest path that turns a received
// UPDATE's attribute bytes into the interned RIB entry.

package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pool "github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
)

var (
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
)

// TestRFC6793ReceivedAS4PathAcceptedAlongsideASPath drives ParseAttributes with
// an UPDATE from an OLD speaker carrying both AS_PATH and AS4_PATH.
// canonicalizeASPath (attrparse.go) takes the four-octet AS4_PATH as the route's
// AS path so the AS_TRANS placeholder never reaches the RIB.
//
// RFC requirement: RFC6793-4.2.3-1 positive -- an UPDATE from an OLD speaker carrying an
// AS4_PATH alongside the existing AS_PATH is accepted, and the four-octet AS numbers from the
// AS4_PATH become the route's AS path instead of the AS_TRANS-bearing two-octet AS_PATH.
func TestRFC6793ReceivedAS4PathAcceptedAlongsideASPath(t *testing.T) {
	raw := concat(wireOriginIGP, rfc6793WireASPathWithASTrans, rfc6793WireAS4Path)

	// asn4=false: the source session did not negotiate four-octet AS support.
	entry, err := ParseAttributes(raw, false)
	require.NoError(t, err, "an AS4_PATH alongside AS_PATH must be accepted")
	defer entry.Release()

	require.True(t, entry.HasASPath())
	got, err := pool.ASPath.Get(entry.ASPath)
	require.NoError(t, err)
	assert.Equal(t,
		[]byte{0x02, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0xFA, 0x56, 0xEA, 0x01},
		got,
		"the AS4_PATH four-octet AS numbers are the route's AS path")
}

// TestRFC6793ASPathAloneNotInventedIntoFourOctet is the counterpart: without an
// AS4_PATH the two-octet AS_PATH is only widened to the four-octet in-memory
// form, and the AS_TRANS placeholder is preserved rather than replaced by a
// guessed four-octet AS.
//
// RFC requirement: RFC6793-4.2.3-1 negative -- when no AS4_PATH accompanies the AS_PATH, no
// four-octet AS numbers are fabricated: the AS_TRANS placeholder is carried through as 23456,
// so the positive case's four-octet path really comes from the received AS4_PATH.
func TestRFC6793ASPathAloneNotInventedIntoFourOctet(t *testing.T) {
	raw := concat(wireOriginIGP, rfc6793WireASPathWithASTrans)

	entry, err := ParseAttributes(raw, false)
	require.NoError(t, err)
	defer entry.Release()

	require.True(t, entry.HasASPath())
	got, err := pool.ASPath.Get(entry.ASPath)
	require.NoError(t, err)
	assert.Equal(t,
		[]byte{0x02, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0x5B, 0xA0},
		got,
		"AS_TRANS stays AS_TRANS when no AS4_PATH is present")
}

// TestRFC6793ReceivedAS4AggregatorAccepted drives ParseAttributes with an UPDATE
// from an OLD speaker carrying AS4_AGGREGATOR alongside AGGREGATOR.
//
// RFC requirement: RFC6793-4.2.3-2 positive -- an UPDATE from an OLD speaker carrying an
// AS4_AGGREGATOR alongside the existing AGGREGATOR is accepted rather than rejected: parsing
// succeeds, the AGGREGATOR is interned and the AS4_AGGREGATOR is retained with the route.
func TestRFC6793ReceivedAS4AggregatorAccepted(t *testing.T) {
	raw := concat(wireOriginIGP, rfc6793WireAggregatorASTrans, rfc6793WireAS4Aggregator)

	entry, err := ParseAttributes(raw, false)
	require.NoError(t, err, "AS4_AGGREGATOR alongside AGGREGATOR must be accepted")
	defer entry.Release()

	b := entry.GetBundle()
	require.True(t, b.HasAggregator(), "AGGREGATOR is retained")
	aggValue, err := pool.Aggregator.Get(b.Aggregator)
	require.NoError(t, err)
	assert.Equal(t, []byte{0x5B, 0xA0, 0x0A, 0x00, 0x00, 0x01}, aggValue)

	require.True(t, b.HasOtherAttrs(), "AS4_AGGREGATOR is retained with the route")
	other, err := pool.OtherAttrs.Get(b.OtherAttrs)
	require.NoError(t, err)
	assert.Contains(t, string(other), string([]byte{0xFA, 0x56, 0xEA, 0x01}),
		"the four-octet aggregating AS survives ingest")
}
