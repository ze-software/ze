package update

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/nlri"
)

// encodeOneNLRI returns the octets WriteNLRI puts on the wire for the single
// announced NLRI of cmd, under a session whose ADD-PATH negotiation is addPath.
// It is the same pair of calls the reactor's writeBatchNLRI makes
// (internal/component/bgp/reactor/reactor_api_batch.go).
func encodeOneNLRI(t *testing.T, cmd []string, addPath bool) (nlri.NLRI, []byte) {
	t.Helper()

	result, err := ParseUpdateText(cmd)
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	require.Len(t, result.Groups[0].Announce, 1)

	n := result.Groups[0].Announce[0]
	buf := make([]byte, nlri.LenWithContext(n, addPath))
	written := nlri.WriteNLRI(n, buf, 0, addPath)
	require.Equal(t, len(buf), written, "LenWithContext disagrees with WriteNLRI")
	return n, buf
}

// labeledNoPathID is the labeled unicast section without a path identifier. Its
// encoding is the payload every case below expects to find after the four
// octets: RFC 8277 Section 2.2 <length><label><prefix>.
var labeledNoPathID = []string{
	"nhop", "10.0.0.1",
	"nlri", "ipv4/mpls-label", "label", "100", "add", "10.0.0.0/24",
}

// TestPathInformationZeroIsAnIdentifierOnTheWire pins that `path-information 0`
// names the Path Identifier zero rather than the absence of one.
//
// RFC requirement: RFC7911-3-1 positive -- an operator who writes
// `path-information 0` on a labeled unicast section gets an NLRI whose encoding
// is extended by four octets holding zero, ahead of the unchanged RFC 8277
// payload, and the NLRI itself reports that it carries an identifier.
//
// RFC 7911 Section 3: "In order to carry the Path Identifier in an UPDATE
// message, the NLRI encoding MUST be extended by prepending the Path Identifier
// field, which is of four octets."
//
// The field reserves no value, so the command's keyword is the only fact that
// says an identifier was named. Deciding it from the value instead -- which is
// what parseLabeledNLRI did with `accum.PathID != 0` -- makes identifier zero
// indistinguishable from a route that named none.
//
// VALIDATES: the presence flag reaches the NLRI, the four octets are written,
// and the RFC 8277 payload after them is byte-identical to the same section
// written without `path-information`.
// PREVENTS: `send bgp <peer> update text path-information 0 nlri
// ipv4/mpls-label label 100 add 10.0.0.0/24` reaching the peer as a route that
// carried no Path Identifier.
func TestPathInformationZeroIsAnIdentifierOnTheWire(t *testing.T) {
	_, payload := encodeOneNLRI(t, labeledNoPathID, false)

	n, wire := encodeOneNLRI(t, []string{
		"nhop", "10.0.0.1",
		"nlri", "ipv4/mpls-label", "path-information", "0", "label", "100",
		"add", "10.0.0.0/24",
	}, true)

	aware, isAware := n.(nlri.AddPathAware)
	require.True(t, isAware, "the parsed NLRI must be able to state its layout")
	assert.True(t, aware.HasAddPath(), "path-information 0 names an identifier")
	assert.Equal(t, uint32(0), n.PathID())

	require.Len(t, wire, 4+len(payload))
	assert.Equal(t, []byte{0, 0, 0, 0}, wire[:4], "four octets of Path Identifier zero")
	assert.Equal(t, payload, wire[4:], "the RFC 8277 payload is unchanged")
}

// TestPathInformationIsNotWrittenWithoutTheNegotiation holds the other polarity
// of the same requirement.
//
// RFC requirement: RFC7911-3-1 negative -- a session that did not negotiate
// ADD-PATH gets the unextended RFC 8277 encoding, and the identifier the
// operator wrote is not prepended to it.
//
// RFC 7911 Section 3: "In order to carry the Path Identifier in an UPDATE
// message, the NLRI encoding MUST be extended by prepending the Path Identifier
// field, which is of four octets."
//
// The extension belongs to the ADD-PATH encoding, so it is written when the
// session carries it and at no other time. An NLRI that prepended the four
// octets on its own would desynchronise a peer that never agreed to read them.
//
// VALIDATES: with addPath false, WriteNLRI emits the payload alone, whether or
// not the command named an identifier.
// PREVENTS: a stray four octets reaching a peer with no ADD-PATH capability.
func TestPathInformationIsNotWrittenWithoutTheNegotiation(t *testing.T) {
	_, payload := encodeOneNLRI(t, labeledNoPathID, false)

	_, wire := encodeOneNLRI(t, []string{
		"nhop", "10.0.0.1",
		"nlri", "ipv4/mpls-label", "path-information", "0", "label", "100",
		"add", "10.0.0.0/24",
	}, false)

	assert.Equal(t, payload, wire, "no ADD-PATH, no Path Identifier field")
}

// TestPathInformationKeepsTheWholeNLRI pins the other half of the same repair.
//
// A registered encoder answers with the NLRI payload alone: labeled unicast's
// EncodeNLRIHex returns LabeledUnicast.Bytes(), which excludes the Path
// Identifier by contract. encodeViaRegistry told NewWireNLRI those bytes began
// with a four-octet identifier whenever the operator wrote a non-zero one, so
// WireNLRI read the first four octets of the RFC 8277 payload as the identifier
// and WriteTo dropped them: the peer received a truncated NLRI under a path
// identifier taken from the label stack.
//
// VALIDATES: a non-zero identifier is prepended, and the payload after it still
// matches the same section written without `path-information`.
// PREVENTS: the four leading payload octets being re-read as the identifier.
func TestPathInformationKeepsTheWholeNLRI(t *testing.T) {
	_, payload := encodeOneNLRI(t, labeledNoPathID, false)

	n, wire := encodeOneNLRI(t, []string{
		"nhop", "10.0.0.1",
		"nlri", "ipv4/mpls-label", "path-information", "42", "label", "100",
		"add", "10.0.0.0/24",
	}, true)

	assert.Equal(t, uint32(42), n.PathID())
	require.Len(t, wire, 4+len(payload))
	assert.Equal(t, []byte{0, 0, 0, 42}, wire[:4])
	assert.Equal(t, payload, wire[4:], "the RFC 8277 payload is unchanged")
}

// TestPathInformationZeroReachesTheVPNWire is the same proof for the VPN
// families, whose section also encodes through the plugin registry.
//
// VALIDATES: `path-information 0` on an ipv4/mpls-vpn section prepends four
// zero octets and leaves the RFC 4364 payload unchanged.
// PREVENTS: an operator's VPN route with identifier zero reaching the peer as a
// route that carried none.
func TestPathInformationZeroReachesTheVPNWire(t *testing.T) {
	_, payload := encodeOneNLRI(t, []string{
		"nhop", "10.0.0.1",
		"nlri", "ipv4/mpls-vpn", "rd", "100:100", "label", "100",
		"add", "10.0.0.0/24",
	}, false)

	n, wire := encodeOneNLRI(t, []string{
		"nhop", "10.0.0.1",
		"nlri", "ipv4/mpls-vpn", "path-information", "0", "rd", "100:100",
		"label", "100", "add", "10.0.0.0/24",
	}, true)

	aware, isAware := n.(nlri.AddPathAware)
	require.True(t, isAware)
	assert.True(t, aware.HasAddPath())
	require.Len(t, wire, 4+len(payload))
	assert.Equal(t, []byte{0, 0, 0, 0}, wire[:4])
	assert.Equal(t, payload, wire[4:])
}
