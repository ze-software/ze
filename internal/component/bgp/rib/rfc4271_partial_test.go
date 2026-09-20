// Related: commit.go — packAttributesWithASPath, the rail that re-encodes a stored
// route's attributes for a peer
// Related: internal/core/bgp/attribute/wire.go — AttributesWire.All, the parse the
// receive path performs before the RIB stores a route
//
// VALIDATES: an ORIGIN, a LOCAL_PREF and a MULTI_EXIT_DISC that arrive carrying the
// Partial bit leave the RIB commit rail with that bit clear.
// PREVENTS: ze re-advertising "partial" on an attribute class RFC 4271 Section 4.3
// forbids it on, because a peer sent it that way.
package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

// flagsOnTheWire walks a packed attribute block and returns each attribute's code
// mapped to the flags octet the block carries for it.
func flagsOnTheWire(t *testing.T, block []byte) map[byte]byte {
	t.Helper()

	out := map[byte]byte{}
	for pos := 0; pos < len(block); {
		require.LessOrEqual(t, pos+3, len(block), "truncated attribute header")
		flags := block[pos]
		code := block[pos+1]

		hdr, valLen := 3, int(block[pos+2])
		if flags&0x10 != 0 {
			require.LessOrEqual(t, pos+4, len(block))
			hdr, valLen = 4, int(block[pos+2])<<8|int(block[pos+3])
		}
		require.LessOrEqual(t, pos+hdr+valLen, len(block), "attribute runs past the block")

		out[code] = flags
		pos += hdr + valLen
	}
	return out
}

// receivedAttributes parses a peer's path-attribute block through the same
// AttributesWire the receive path builds, so the values the commit rail is handed
// are the ones a peer really sent rather than values a test constructed.
func receivedAttributes(t *testing.T, raw []byte, ctx *bgpctx.EncodingContext) []attribute.Attribute {
	t.Helper()

	id, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)

	attrs, err := attribute.NewAttributesWire(raw, id).All()
	require.NoError(t, err)
	return attrs
}

// TestRFC4271PartialClearedWhenTheRailReadvertises drives the readvertise rail with a
// block a non-conformant peer sent.
//
// RFC 7606 Section 4(c) is why such a block reaches the rail at all: it narrows the
// flags check to "the value of either the Optional or Transitive bits in the Attribute
// Flags", so a well-known attribute carrying a Partial bit is not malformed and is not
// withdrawn. Ze accepts it, and Section 4.3 then binds ze as the sender.
//
// VALIDATES: ORIGIN 0x60, LOCAL_PREF 0x60 and MULTI_EXIT_DISC 0xA0 are packed for a
// peer as 0x40, 0x40 and 0x80.
//
// PREVENTS: a rail that copies the received flags octet forward, which would make ze
// the AS that advertised complete information as partial.
//
// RFC requirement: RFC4271-4.3-2 negative -- the clear is not an accident of an
// encoder that only ever sees a clean input: each attribute here ARRIVES with 0x20
// set and is parsed by the live AttributesWire, and packAttributesWithASPath still
// writes the type's own flags octet, on both of its write paths (ORIGIN and
// LOCAL_PREF through WriteAttrTo, MED through WriteAttrToWithContext).
func TestRFC4271PartialClearedWhenTheRailReadvertises(t *testing.T) {
	// One AS on both ends, so the rail takes its iBGP branch and emits LOCAL_PREF.
	ctx := testContext(65000, 65000, true)

	raw := []byte{
		0x60, 0x01, 0x01, 0x00, // ORIGIN, well-known, Partial set (0x40|0x20)
		0xA0, 0x04, 0x04, 0x00, 0x00, 0x00, 0x0a, // MULTI_EXIT_DISC, optional non-transitive, Partial set
		0x60, 0x05, 0x04, 0x00, 0x00, 0x00, 0x64, // LOCAL_PREF, well-known, Partial set
	}

	cs := NewCommitService(&mockUpdateSender{}, ctx, true)
	block, err := cs.packAttributesWithASPath(
		receivedAttributes(t, raw, ctx), nil,
		netip.MustParseAddr("10.0.0.1"),
		newIPv4NLRI("192.168.1.0/24").Family(),
		nil,
	)
	require.NoError(t, err)

	wire := flagsOnTheWire(t, block)

	require.Contains(t, wire, byte(attribute.AttrOrigin), "ORIGIN is on the wire")
	require.Contains(t, wire, byte(attribute.AttrLocalPref), "LOCAL_PREF is on the wire")
	require.Contains(t, wire, byte(attribute.AttrMED), "MULTI_EXIT_DISC is on the wire")

	assert.Equal(t, byte(0x40), wire[byte(attribute.AttrOrigin)],
		"RFC 4271 Section 4.3: the Partial bit must be 0 for a well-known attribute")
	assert.Equal(t, byte(0x40), wire[byte(attribute.AttrLocalPref)],
		"RFC 4271 Section 4.3: the Partial bit must be 0 for a well-known attribute")
	assert.Equal(t, byte(0x80), wire[byte(attribute.AttrMED)],
		"RFC 4271 Section 4.3: the Partial bit must be 0 for an optional non-transitive attribute")
}
