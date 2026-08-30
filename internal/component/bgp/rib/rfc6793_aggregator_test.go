// Related: commit.go — packAttributesWithASPath, appendAS4AggregatorFor
//
// VALIDATES: a forwarded AGGREGATOR reaches an OLD speaker in the six-octet form
// RFC 4271 defines for it, carrying AS_TRANS, with the four-octet AS number
// preserved in the AS4_AGGREGATOR companion RFC 6793 Section 4.2.2 requires.
// PREVENTS: sending an OLD speaker the eight-octet attribute, which it reads as
// malformed, and losing the aggregating AS number when the downgrade happens.
package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// attrsOnTheWire walks a packed attribute block and returns each attribute's
// code mapped to its value octets.
func attrsOnTheWire(t *testing.T, block []byte) map[byte][]byte {
	t.Helper()

	out := map[byte][]byte{}
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

		out[code] = block[pos+hdr : pos+hdr+valLen]
		pos += hdr + valLen
	}
	return out
}

// packForPeer packs one route's attributes for a peer that did or did not
// negotiate four-octet AS support.
func packForPeer(t *testing.T, asn4 bool, attrs []attribute.Attribute) map[byte][]byte {
	t.Helper()

	cs := NewCommitService(&mockUpdateSender{}, testContext(65000, 65001, asn4), true)
	block, err := cs.packAttributesWithASPath(
		attrs, nil,
		netip.MustParseAddr("10.0.0.1"),
		newIPv4NLRI("192.168.1.0/24").Family(),
		nil,
	)
	require.NoError(t, err)

	return attrsOnTheWire(t, block)
}

// TestForwardedAggregatorIsDowngradedWithItsCompanion is the case RFC 6793
// states as a pair. Ze originates no AGGREGATOR of its own, so every one on this
// rail is forwarded, and forwarding it to an OLD speaker is exactly when the
// pair applies.
//
// The wireu forward rail already proves this requirement. This is the RIB COMMIT
// rail, a second producer of the same attribute, which wrote every other
// attribute context-free and so reached an OLD speaker with the eight-octet form
// and no companion.
//
// RFC requirement: RFC6793-4.2.2-5 positive -- a non-mappable four-octet
// aggregating AS is sent to an OLD speaker as AS_TRANS in a six-octet
// AGGREGATOR, with the real AS number in the AS4_AGGREGATOR companion (S4.2.2)
// RFC requirement: RFC6793-4.2.2-6 negative -- the suppression of the companion
// is conditional on the AS number being mappable, so a non-mappable one still
// sends it (S4.2.2).
func TestForwardedAggregatorIsDowngradedWithItsCompanion(t *testing.T) {
	const nonMappable = 4200000000
	router := netip.MustParseAddr("10.9.9.9")

	wire := packForPeer(t, false, []attribute.Attribute{
		attribute.Origin(0),
		&attribute.Aggregator{ASN: nonMappable, Address: router},
	})

	aggregator, ok := wire[byte(attribute.AttrAggregator)]
	require.True(t, ok, "the AGGREGATOR must still be sent")
	require.Len(t, aggregator, 6,
		"RFC 4271 defines a six-octet AGGREGATOR for a peer with no four-octet AS "+
			"support; the eight-octet form is malformed to it")
	require.Equal(t, []byte{0x5b, 0xa0}, aggregator[:2],
		"the AS number field must be AS_TRANS (23456)")

	companion, ok := wire[byte(attribute.AttrAS4Aggregator)]
	require.True(t, ok,
		"RFC 6793 Section 4.2.2: the speaker MUST use the AS4_AGGREGATOR attribute "+
			"when it sets AGGREGATOR to AS_TRANS, or the aggregating AS number is lost")
	require.Len(t, companion, 8)
	require.Equal(t, []byte{0xfa, 0x56, 0xea, 0x00}, companion[:4],
		"the companion carries the real four-octet AS number")
}

// TestAMappableAggregatorGetsNoCompanion is the discrimination case, and Section
// 4.2.2 states it as its own prohibition.
//
// RFC requirement: RFC6793-4.2.2-6 positive -- "if the AS number is mappable,
// then the AS4_AGGREGATOR attribute MUST NOT be sent" (S4.2.2)
// RFC requirement: RFC6793-4.2.2-5 negative -- the AS_TRANS substitution is
// scoped to a non-mappable AS number: a mappable one is sent as itself (S4.2.2).
func TestAMappableAggregatorGetsNoCompanion(t *testing.T) {
	wire := packForPeer(t, false, []attribute.Attribute{
		attribute.Origin(0),
		&attribute.Aggregator{ASN: 65001, Address: netip.MustParseAddr("10.9.9.9")},
	})

	aggregator, ok := wire[byte(attribute.AttrAggregator)]
	require.True(t, ok)
	require.Len(t, aggregator, 6, "an OLD speaker still gets the six-octet form")
	require.Equal(t, []byte{0xfd, 0xe9}, aggregator[:2],
		"a mappable AS number is sent as itself, not as AS_TRANS")

	_, hasCompanion := wire[byte(attribute.AttrAS4Aggregator)]
	require.False(t, hasCompanion,
		"RFC 6793 Section 4.2.2 forbids the companion when the AS number is mappable")
}

// TestANewSpeakerGetsTheEightOctetFormAndNoCompanion keeps the common path. The
// downgrade exists for OLD speakers only, and a fix that always downgrades would
// pass both tests above.
func TestANewSpeakerGetsTheEightOctetFormAndNoCompanion(t *testing.T) {
	const nonMappable = 4200000000

	wire := packForPeer(t, true, []attribute.Attribute{
		attribute.Origin(0),
		&attribute.Aggregator{ASN: nonMappable, Address: netip.MustParseAddr("10.9.9.9")},
	})

	aggregator, ok := wire[byte(attribute.AttrAggregator)]
	require.True(t, ok)
	require.Len(t, aggregator, 8,
		"a peer that negotiated four-octet AS support takes the AS number directly")
	require.Equal(t, []byte{0xfa, 0x56, 0xea, 0x00}, aggregator[:4])

	_, hasCompanion := wire[byte(attribute.AttrAS4Aggregator)]
	require.False(t, hasCompanion,
		"the companion exists to survive a downgrade, and there is no downgrade here")
}

// TestAForwardedCompanionIsNotDuplicated covers the upstream that already sent
// one. Two AS4_AGGREGATOR attributes in one UPDATE are malformed under RFC 7606
// Section 3(g).
func TestAForwardedCompanionIsNotDuplicated(t *testing.T) {
	const nonMappable = 4200000000
	router := netip.MustParseAddr("10.9.9.9")

	attrs := []attribute.Attribute{
		attribute.Origin(0),
		&attribute.Aggregator{ASN: nonMappable, Address: router},
		&attribute.AS4Aggregator{ASN: nonMappable, Address: router},
	}
	kept := appendAS4AggregatorFor(attrs, testContext(65000, 65001, false))

	require.Len(t, kept, len(attrs),
		"an AS4_AGGREGATOR the upstream supplied must not be joined by a second one")
}
