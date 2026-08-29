// RFC: rfc/short/rfc9552.md -- Section 8.2.2 fault management for BGP-LS
// Overview: rfc7606_bgpls_nlri.go -- the walk these tests drive
// Related: ../reactor/rfc9552_nlri_test.go -- the same obligations proved on the receive path
//
// These are the boundary cases of the Section 8.2.2 walk. Every length in a Link-State NLRI
// is chosen by the peer, so each one is tested at the value that exactly fits and at the
// value one past it. The receive-path proof that the walk is reached at all, and that each
// class takes the action Section 8.2.2 prescribes, lives in the reactor package.

package message

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// lsNLRIOf frames one Link-State NLRI: [type:2][total length:2][body] (RFC 9552 Section 5.2).
func lsNLRIOf(nlriType uint16, body ...byte) []byte {
	out := []byte{byte(nlriType >> 8), byte(nlriType), byte(len(body) >> 8), byte(len(body))}
	return append(out, body...)
}

// lsTLVOf frames one TLV, which RFC 9552 Section 5.1 shapes identically at every depth.
func lsTLVOf(tlvType uint16, value ...byte) []byte {
	out := []byte{byte(tlvType >> 8), byte(tlvType), byte(len(value) >> 8), byte(len(value))}
	return append(out, value...)
}

// lsNodeBodyOf builds a Node, Link or Prefix NLRI body: Protocol-ID 2 (IS-IS Level 2), a
// zero Identifier, then the descriptor TLVs the caller gives.
func lsNodeBodyOf(tlvs ...byte) []byte {
	body := []byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	return append(body, tlvs...)
}

// lsAutonomousSystem is the Autonomous System sub-TLV (512), the one Table 3 sub-TLV these
// fixtures need. Its value is opaque to the walk, which reads no octet of it.
func lsAutonomousSystem() []byte {
	return lsTLVOf(512, 0x00, 0x00, 0xfd, 0xe9)
}

// lsLocalNodeDescriptors wraps sub-TLVs in the Local Node Descriptors TLV (256).
func lsLocalNodeDescriptors(subTLVs ...byte) []byte {
	return lsTLVOf(bgplsTLVLocalNodeDescriptors, subTLVs...)
}

// lsWellFormedNode is the NLRI every "one of these survives" case pairs a malformed NLRI
// with, so a test tells a discard apart from a walk that dropped everything.
func lsWellFormedNode() []byte {
	return lsNLRIOf(1, lsNodeBodyOf(lsLocalNodeDescriptors(lsAutonomousSystem()...)...)...)
}

func bytesOf(parts ...[]byte) []byte {
	var out []byte
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}

// VALIDATES: RetainWellFormedNLRI keeps exactly the NLRIs RFC 9552 Section 8.2.2 does not
// call malformed, at the boundary of every length a peer chooses.
// PREVENTS: a walk that is one octet permissive, which lets a malformed NLRI reach every
// BGP-LS Consumer behind ze, and a walk that is one octet strict, which discards conforming
// NLRIs and takes their link-state object off the topology.
func TestRetainWellFormedNLRIBoundaries(t *testing.T) {
	good := lsWellFormedNode()

	cases := []struct {
		name    string
		safi    attribute.SAFI
		section []byte
		kept    []byte
		dropped int
		framed  bool
	}{
		{
			name: "an empty section is nothing to judge",
			safi: safiBGPLS, section: nil, kept: nil, dropped: 0, framed: true,
		},
		{
			name: "a well-formed node nlri survives untouched",
			safi: safiBGPLS, section: good, kept: good, dropped: 0, framed: true,
		},
		{
			// Section 8.2.2: the TLV lengths sum to zero and the body is exactly the
			// Protocol-ID and the Identifier, so the sum corresponds.
			name: "a body of exactly the fixed fields carries no tlv and is well formed",
			safi: safiBGPLS, section: lsNLRIOf(1, lsNodeBodyOf()...),
			kept: lsNLRIOf(1, lsNodeBodyOf()...), dropped: 0, framed: true,
		},
		{
			name: "a body one octet short of the fixed fields is malformed",
			safi: safiBGPLS, section: lsNLRIOf(1, lsNodeBodyOf()[:bgplsNLRIFixedLen-1]...),
			kept: nil, dropped: 1, framed: true,
		},
		{
			name: "a tlv that fills the body exactly is well formed",
			safi: safiBGPLS, section: good, kept: good, dropped: 0, framed: true,
		},
		{
			// The TLV declares 9 octets of value where 8 follow it.
			name: "a tlv one octet past the body is malformed",
			safi: safiBGPLS,
			section: bytesOf(
				lsNLRIOf(1, lsNodeBodyOf(0x01, 0x00, 0x00, 0x09,
					0x02, 0x00, 0x00, 0x04, 0x00, 0x00, 0xfd, 0xe9)...),
				good,
			),
			kept: good, dropped: 1, framed: true,
		},
		{
			name: "a body ending inside a tlv header is malformed",
			safi: safiBGPLS,
			section: bytesOf(
				lsNLRIOf(1, lsNodeBodyOf(0x01, 0x00)...),
				good,
			),
			kept: good, dropped: 1, framed: true,
		},
		{
			// The sub-TLV declares 5 octets of value where 4 follow it, inside a Node
			// Descriptor whose own length is correct.
			name: "a sub-tlv one octet past its node descriptor is malformed",
			safi: safiBGPLS,
			section: bytesOf(
				lsNLRIOf(1, lsNodeBodyOf(lsLocalNodeDescriptors(
					0x02, 0x00, 0x00, 0x05, 0x00, 0x00, 0xfd, 0xe9)...)...),
				good,
			),
			kept: good, dropped: 1, framed: true,
		},
		{
			// Section 5.2.1.4: "The sub-TLVs within a Node Descriptor MUST be arranged in
			// ascending order by sub-TLV type."
			name: "ascending sub-tlv types are well formed",
			safi: safiBGPLS,
			section: lsNLRIOf(1, lsNodeBodyOf(lsLocalNodeDescriptors(bytesOf(
				lsAutonomousSystem(), lsTLVOf(514, 0x0a, 0x00, 0x00, 0x01))...)...)...),
			kept: lsNLRIOf(1, lsNodeBodyOf(lsLocalNodeDescriptors(bytesOf(
				lsAutonomousSystem(), lsTLVOf(514, 0x0a, 0x00, 0x00, 0x01))...)...)...),
			dropped: 0, framed: true,
		},
		{
			// Section 8.2.2 bullet 7 and Section 5.2.1.4: "At most, there MUST be one
			// instance of each sub-TLV type present in any Node Descriptor."
			name: "one sub-tlv type twice is malformed",
			safi: safiBGPLS,
			section: bytesOf(
				lsNLRIOf(1, lsNodeBodyOf(lsLocalNodeDescriptors(bytesOf(
					lsAutonomousSystem(), lsAutonomousSystem())...)...)...),
				good,
			),
			kept: good, dropped: 1, framed: true,
		},
		{
			name: "descending sub-tlv types are malformed",
			safi: safiBGPLS,
			section: bytesOf(
				lsNLRIOf(1, lsNodeBodyOf(lsLocalNodeDescriptors(bytesOf(
					lsTLVOf(514, 0x0a, 0x00, 0x00, 0x01), lsAutonomousSystem())...)...)...),
				good,
			),
			kept: good, dropped: 1, framed: true,
		},
		{
			// Section 5.1: "all TLVs within the NLRI MUST be ordered in ascending order by
			// TLV Type."
			name: "descending tlv types are malformed",
			safi: safiBGPLS,
			section: bytesOf(
				lsNLRIOf(2, lsNodeBodyOf(bytesOf(
					lsTLVOf(bgplsTLVRemoteNodeDescriptors, lsAutonomousSystem()...),
					lsTLVOf(bgplsTLVLocalNodeDescriptors, lsAutonomousSystem()...))...)...),
				good,
			),
			kept: good, dropped: 1, framed: true,
		},
		{
			// Section 5.1: "the TLVs sharing the same type MUST be first in ascending order
			// based on the Length field."
			name: "one tlv type twice with ascending lengths is well formed",
			safi: safiBGPLS,
			section: lsNLRIOf(3, lsNodeBodyOf(bytesOf(
				lsTLVOf(265, 0x18), lsTLVOf(265, 0x18, 0x0a))...)...),
			kept: lsNLRIOf(3, lsNodeBodyOf(bytesOf(
				lsTLVOf(265, 0x18), lsTLVOf(265, 0x18, 0x0a))...)...),
			dropped: 0, framed: true,
		},
		{
			name: "one tlv type twice with descending lengths is malformed",
			safi: safiBGPLS,
			section: bytesOf(
				lsNLRIOf(3, lsNodeBodyOf(bytesOf(
					lsTLVOf(265, 0x18, 0x0a), lsTLVOf(265, 0x18))...)...),
				good,
			),
			kept: good, dropped: 1, framed: true,
		},
		{
			// Section 5.1: equal lengths are then ordered "based on the Value field ...
			// treating the entire field as opaque binary data and ordered
			// lexicographically".
			name: "one tlv type twice with descending values is malformed",
			safi: safiBGPLS,
			section: bytesOf(
				lsNLRIOf(3, lsNodeBodyOf(bytesOf(
					lsTLVOf(265, 0x18, 0x0b), lsTLVOf(265, 0x18, 0x0a))...)...),
				good,
			),
			kept: good, dropped: 1, framed: true,
		},
		{
			// Section 5.1 states no uniqueness rule for a whole TLV, and Section 8.2.2
			// forbids judging an NLRI on which TLVs it includes.
			name: "two identical tlvs are ordered and well formed",
			safi: safiBGPLS,
			section: lsNLRIOf(3, lsNodeBodyOf(bytesOf(
				lsTLVOf(265, 0x18, 0x0a), lsTLVOf(265, 0x18, 0x0a))...)...),
			kept: lsNLRIOf(3, lsNodeBodyOf(bytesOf(
				lsTLVOf(265, 0x18, 0x0a), lsTLVOf(265, 0x18, 0x0a))...)...),
			dropped: 0, framed: true,
		},
		{
			// Section 5.2: "An implementation MUST handle unknown Link-State NLRI types as
			// opaque objects and MUST preserve and propagate them." Its body is not TLVs
			// and reading it as TLVs would be the defect.
			name: "an unknown nlri type is opaque and survives whatever its body holds",
			safi: safiBGPLS, section: lsNLRIOf(99, 0xde, 0xad, 0xbe),
			kept: lsNLRIOf(99, 0xde, 0xad, 0xbe), dropped: 0, framed: true,
		},
		{
			name: "an unknown nlri type with an empty body survives",
			safi: safiBGPLS, section: lsNLRIOf(99), kept: lsNLRIOf(99), dropped: 0, framed: true,
		},
		{
			// Section 5.2: for SAFI 72 the Total NLRI Length "also includes the length of
			// the Route Distinguisher".
			name: "a vpn nlri of exactly the rd and the fixed fields is well formed",
			safi: safiBGPLSVPN,
			section: lsNLRIOf(1, bytesOf(
				[]byte{0, 0, 0, 0, 0, 0, 0, 0}, lsNodeBodyOf())...),
			kept: lsNLRIOf(1, bytesOf(
				[]byte{0, 0, 0, 0, 0, 0, 0, 0}, lsNodeBodyOf())...),
			dropped: 0, framed: true,
		},
		{
			name:    "a vpn nlri one octet short of its rd is malformed",
			safi:    safiBGPLSVPN,
			section: lsNLRIOf(1, 0, 0, 0, 0, 0, 0, 0),
			kept:    nil, dropped: 1, framed: true,
		},
		{
			name: "every nlri malformed leaves an empty section",
			safi: safiBGPLS,
			section: bytesOf(
				lsNLRIOf(1, lsNodeBodyOf(0x01, 0x00)...),
				lsNLRIOf(1, lsNodeBodyOf(0x01, 0x00)...),
			),
			kept: nil, dropped: 2, framed: true,
		},
		{
			// Section 8.2.2: "The sum of all TLV lengths found in the BGP MP_REACH_NLRI
			// attribute corresponds to the BGP MP_REACH_NLRI length." One octet past means
			// no boundary after it can be located.
			name: "a total nlri length one octet past the section cannot be walked",
			safi: safiBGPLS, section: []byte{0x00, 0x01, 0x00, 0x02, 0xaa},
			kept: nil, dropped: 0, framed: false,
		},
		{
			name: "a section ending inside an nlri header cannot be walked",
			safi: safiBGPLS, section: bytesOf(good, []byte{0x00, 0x01, 0x00}),
			kept: nil, dropped: 0, framed: false,
		},
	}

	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			kept, dropped, framed := RetainWellFormedNLRI(afiBGPLS, one.safi, one.section, false)
			assert.Equal(t, one.framed, framed)
			if !framed {
				assert.Equal(t, one.section, kept, "an unwalkable section is returned unchanged")
				assert.Equal(t, 0, dropped, "no discard decision is possible")
				return
			}
			assert.Equal(t, one.dropped, dropped)
			assert.Equal(t, one.kept, kept)
		})
	}
}

// VALIDATES: nothing is dropped, and no copy is made, for a family whose specification
// prescribes no per-NLRI syntactic discard, even when its bytes would fail the BGP-LS walk.
// PREVENTS: the walk reaching a family whose NLRI is not a Link-State NLRI at all, where
// every judgment it made would be nonsense read off the wrong layout.
func TestRetainWellFormedNLRILeavesOtherFamiliesAlone(t *testing.T) {
	section := []byte{0x00, 0x01, 0x00, 0xff, 0xaa} // malformed as a Link-State NLRI

	kept, dropped, framed := RetainWellFormedNLRI(attribute.AFIIPv4, attribute.SAFIUnicast, section, false)
	require.True(t, framed)
	assert.Equal(t, 0, dropped)
	require.NotEmpty(t, kept)
	assert.Same(t, &section[0], &kept[0], "an unruled family must not be copied")
}

// VALIDATES: a conforming BGP-LS UPDATE costs no allocation in the walk, because the input
// section is returned rather than rebuilt.
// PREVENTS: an allocation for every BGP-LS UPDATE ze relays, which is the cost this walk was
// written to avoid paying on the receive path (ai/rules/performance.md).
func TestRetainWellFormedNLRIIsZeroCopyWhenNothingIsDropped(t *testing.T) {
	section := bytesOf(lsWellFormedNode(), lsWellFormedNode())

	kept, dropped, framed := RetainWellFormedNLRI(afiBGPLS, safiBGPLS, section, false)
	require.True(t, framed)
	require.Equal(t, 0, dropped)
	require.NotEmpty(t, kept)
	assert.Same(t, &section[0], &kept[0], "nothing was dropped, so the input array is the answer")
}

// VALIDATES: the RFC 7911 path identifier is stepped over, so an ADD-PATH Link-State NLRI is
// judged on its own octets and keeps its identifier when it survives.
// PREVENTS: reading the first two octets of a path identifier as an NLRI Type, which would
// condemn or excuse an NLRI on a number the peer chose for an unrelated reason.
func TestRetainWellFormedNLRISkipsThePathIdentifier(t *testing.T) {
	pathID := []byte{0x00, 0x00, 0x00, 0x07}
	keep := bytesOf(pathID, lsWellFormedNode())
	drop := bytesOf(pathID, lsNLRIOf(1, lsNodeBodyOf(0x01, 0x00)...))

	kept, dropped, framed := RetainWellFormedNLRI(afiBGPLS, safiBGPLS, bytesOf(keep, drop), true)
	require.True(t, framed)
	assert.Equal(t, 1, dropped)
	assert.Equal(t, keep, kept, "the surviving NLRI keeps the path identifier that names it")

	_, _, framed = RetainWellFormedNLRI(afiBGPLS, safiBGPLS, pathID[:3], true)
	assert.False(t, framed, "a section too short for the path identifier cannot be walked")
}

// VALIDATES: validateMPNLRISyntax routes AFI 16388 to the Link-State framing walk, answers
// session reset for the class RFC 9552 Section 8.2.2 calls non-skipable, and answers nothing
// for the class it calls skipable.
// PREVENTS: the two failures either side of the line. A framing error that returns nil
// leaves ze relaying a section it could not parse; a skipable error that returns session
// reset hands a peer a one-octet way to drop the session, which Section 8.2.2 forbids.
func TestValidateMPNLRISyntaxJudgesLinkStateFramingOnly(t *testing.T) {
	code := uint8(attribute.AttrMPReachNLRI)
	duplicateSubTLV := lsNLRIOf(1, lsNodeBodyOf(lsLocalNodeDescriptors(bytesOf(
		lsAutonomousSystem(), lsAutonomousSystem())...)...)...)

	assert.Nil(t, validateMPNLRISyntax(code, afiBGPLS, safiBGPLS, lsWellFormedNode(), false),
		"a well-formed Link-State NLRI section is not malformed")
	assert.Nil(t, validateMPNLRISyntax(code, afiBGPLS, safiBGPLS, duplicateSubTLV, false),
		"a duplicated sub-TLV is skipable, so it is the discard's business and not this walk's")

	overrun := lsWellFormedNode()
	overrun[2], overrun[3] = 0x00, 0xff
	result := validateMPNLRISyntax(code, afiBGPLS, safiBGPLS, overrun, false)
	require.NotNil(t, result, "a Total NLRI Length past the attribute is malformed")
	assert.Equal(t, RFC7606ActionSessionReset, result.Action)
	assert.Equal(t, code, result.AttrCode)

	assert.Nil(t, validateMPNLRISyntax(code, afiBGPLS, 1, overrun, false),
		"AFI 16388 with a SAFI RFC 9552 does not define is left alone")
}
