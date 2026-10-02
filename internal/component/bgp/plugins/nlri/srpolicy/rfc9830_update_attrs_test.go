// VALIDATES: RFC 9830 Section 2.1, the announce half: the UPDATE ze builds for an
// SR Policy (SAFI 73) route carries ORIGIN and AS_PATH beside MP_REACH_NLRI, found
// by attribute code in the path attribute section rather than by a hex substring.
// PREVENTS: an iBGP SR Policy announce leaving without AS_PATH. An iBGP path is
// empty, so its AS_PATH is the three octets 40 02 00, and a substring check for
// "4002" also matches octets elsewhere in the UPDATE.

package srpolicy

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// rfc9830PathAttributes returns the path attribute section of one UPDATE as
// EncodeRoute packs it: the 19-octet header, the Withdrawn Routes Length and its
// routes, then the Total Path Attribute Length and the attributes it spans.
func rfc9830PathAttributes(t *testing.T, update []byte) []byte {
	t.Helper()

	const headerOctets = 19
	require.GreaterOrEqual(t, len(update), headerOctets+4, "an UPDATE holds a header and two length fields")
	require.Equal(t, byte(2), update[18], "message type is UPDATE")
	withdrawnOctets := int(binary.BigEndian.Uint16(update[headerOctets:]))
	attrLengthAt := headerOctets + 2 + withdrawnOctets
	require.GreaterOrEqual(t, len(update), attrLengthAt+2)
	attrOctets := int(binary.BigEndian.Uint16(update[attrLengthAt:]))
	attrsAt := attrLengthAt + 2
	require.GreaterOrEqual(t, len(update), attrsAt+attrOctets)
	return update[attrsAt : attrsAt+attrOctets]
}

// TestRFC9830SRPolicyAnnounceCarriesMandatoryAttributes builds the SR Policy
// announce through EncodeRoute toward an eBGP and an iBGP peer and looks each
// mandatory attribute up by its code with attribute.AttrFind, checking the
// literal flags and value octets.
func TestRFC9830SRPolicyAnnounceCarriesMandatoryAttributes(t *testing.T) {
	t.Parallel()

	const cmd = "distinguisher 0 color 100 endpoint 10.0.0.1 next-hop 192.0.2.1 preference 100"

	// RFC requirement: RFC9830-2.1-3 positive -- the UPDATE carrying MP_REACH_NLRI for SAFI 73 (value starts 00 01 49) toward an eBGP peer also carries ORIGIN (flags 40, value 00) and AS_PATH (flags 40, value 02 01 0000FDE8, the local AS 65000), and toward an iBGP peer carries ORIGIN (40, 00) beside the same MP_REACH_NLRI, each found by its attribute code in the path attribute section
	ebgp, _, err := EncodeRoute(cmd, "ipv4/sr-policy", 65000, false, true, false)
	require.NoError(t, err)
	attrs := rfc9830PathAttributes(t, ebgp)

	_, flags, value, found := attribute.AttrFind(attrs, attribute.AttrMPReachNLRI)
	require.True(t, found, "eBGP: MP_REACH_NLRI")
	require.GreaterOrEqual(t, len(value), 3)
	assert.Equal(t, []byte{0x00, 0x01, 0x49}, value[:3], "eBGP: AFI 1, SAFI 73")
	assert.Equal(t, attribute.AttributeFlags(0x80), flags&0xC0, "eBGP: MP_REACH_NLRI is optional non-transitive")

	_, flags, value, found = attribute.AttrFind(attrs, attribute.AttrOrigin)
	require.True(t, found, "eBGP: ORIGIN")
	assert.Equal(t, attribute.AttributeFlags(0x40), flags, "eBGP: ORIGIN is well-known transitive")
	assert.Equal(t, []byte{0x00}, value, "eBGP: ORIGIN IGP")

	_, flags, value, found = attribute.AttrFind(attrs, attribute.AttrASPath)
	require.True(t, found, "eBGP: AS_PATH")
	assert.Equal(t, attribute.AttributeFlags(0x40), flags, "eBGP: AS_PATH is well-known transitive")
	assert.Equal(t, []byte{0x02, 0x01, 0x00, 0x00, 0xFD, 0xE8}, value, "eBGP: AS_SEQUENCE of one AS, 65000")

	ibgp, _, err := EncodeRoute(cmd, "ipv4/sr-policy", 65000, true, true, false)
	require.NoError(t, err)
	attrs = rfc9830PathAttributes(t, ibgp)

	_, _, value, found = attribute.AttrFind(attrs, attribute.AttrMPReachNLRI)
	require.True(t, found, "iBGP: MP_REACH_NLRI")
	require.GreaterOrEqual(t, len(value), 3)
	assert.Equal(t, []byte{0x00, 0x01, 0x49}, value[:3], "iBGP: AFI 1, SAFI 73")

	_, flags, value, found = attribute.AttrFind(attrs, attribute.AttrOrigin)
	require.True(t, found, "iBGP: ORIGIN")
	assert.Equal(t, attribute.AttributeFlags(0x40), flags)
	assert.Equal(t, []byte{0x00}, value, "iBGP: ORIGIN IGP")
}

// TestRFC9830SRPolicyIBGPAnnounceKeepsAnEmptyASPath forces the input that would
// let AS_PATH go missing: an iBGP session, where ze prepends nothing and the
// configured path is empty, so the attribute has no segment to carry. RFC 4271
// Section 5.1.2 still requires it, as an empty AS_PATH.
func TestRFC9830SRPolicyIBGPAnnounceKeepsAnEmptyASPath(t *testing.T) {
	t.Parallel()

	// RFC requirement: RFC9830-2.1-3 negative -- toward an iBGP peer, with no AS to put in the path, the SR Policy UPDATE still carries AS_PATH (flags 40) with a zero-length value instead of omitting the mandatory attribute
	ibgp, _, err := EncodeRoute("distinguisher 0 color 100 endpoint 10.0.0.1 next-hop 192.0.2.1",
		"ipv4/sr-policy", 65000, true, true, false)
	require.NoError(t, err)
	attrs := rfc9830PathAttributes(t, ibgp)

	_, flags, value, found := attribute.AttrFind(attrs, attribute.AttrASPath)
	require.True(t, found, "iBGP: AS_PATH is present even when empty")
	assert.Equal(t, attribute.AttributeFlags(0x40), flags)
	assert.Empty(t, value, "iBGP: the path holds no segment")
}
