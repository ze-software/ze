package nlrisplit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// mvpnNLRI frames one MCAST-VPN NLRI: [route-type:1][length:1][body] (RFC 6514 Section 4).
func mvpnNLRI(routeType byte, body ...byte) []byte {
	return append([]byte{routeType, byte(len(body))}, body...)
}

// mupNLRI frames one BGP-MUP NLRI under architecture type 1 (3gpp-5g):
// [arch:1][route-type:2][length:1][body] (draft-ietf-bess-mup-safi Section 3.1).
// The splitter is architecture-agnostic; which architectures ze implements is the
// recognizer's question, asserted in the mup plugin's own tests.
func mupNLRI(routeType uint16, body ...byte) []byte {
	out := []byte{1, byte(routeType >> 8), byte(routeType), byte(len(body))}
	return append(out, body...)
}

// VALIDATES: SplitMVPN carves a section into whole NLRIs on the RFC 6514 Section 4
// length octet, in wire order, aliasing the input.
// PREVENTS: an MVPN route type being judged against the wrong bytes, which would
// discard or keep the wrong routes under RFC 7606 Section 5.4.
func TestSplitMVPNCarvesOnLength(t *testing.T) {
	a := mvpnNLRI(1, 0xaa, 0xbb)
	b := mvpnNLRI(7)
	c := mvpnNLRI(5, 0xcc)

	got, err := SplitMVPN(concat(a, b, c), false)
	require.NoError(t, err)
	assert.Equal(t, [][]byte{a, b, c}, got)
}

// VALIDATES: the 4-byte RFC 7911 path identifier is part of each returned slice and
// is counted before the header.
// PREVENTS: an ADD-PATH section being carved one NLRI short or misaligned.
func TestSplitMVPNAddPathIncludesPathID(t *testing.T) {
	pathID := []byte{0x00, 0x00, 0x00, 0x07}
	a := concat(pathID, mvpnNLRI(1, 0xaa))
	b := concat(pathID, mvpnNLRI(6, 0xbb, 0xcc))

	got, err := SplitMVPN(concat(a, b), true)
	require.NoError(t, err)
	assert.Equal(t, [][]byte{a, b}, got)
}

// VALIDATES: the two boundaries RFC 7606 Section 5.3 names -- a header that does not
// fit, and a last NLRI whose length overruns the section -- both report an error and
// return what was parsed so far.
// PREVENTS: the Section 5.4 pass inventing NLRI boundaries from a truncated section.
// A one-below and one-above pair is asserted for each so an off-by-one cannot pass.
func TestSplitMVPNBoundaries(t *testing.T) {
	good := mvpnNLRI(1, 0xaa)

	// Header boundary: exactly two octets is the shortest legal NLRI; one is not.
	_, err := SplitMVPN([]byte{1, 0x00}, false)
	assert.NoError(t, err, "a two-octet header with a zero-length body is well framed")
	_, err = SplitMVPN([]byte{1}, false)
	assert.Error(t, err, "one octet cannot hold a route type and a length")

	// Value boundary: length 1 with one octet left is legal, with none is not.
	_, err = SplitMVPN([]byte{1, 0x01, 0xaa}, false)
	assert.NoError(t, err)
	partial, err := SplitMVPN(concat(good, []byte{1, 0x01}), false)
	assert.Error(t, err, "the last NLRI overruns the section")
	assert.Equal(t, [][]byte{good}, partial, "what parsed cleanly is still returned")
}

// VALIDATES: SplitMUP reads the length octet at offset 3, after the one-octet
// architecture type and the two-octet route type.
// PREVENTS: reading the high half of the route type as a length, which would carve
// every MUP section wrongly.
func TestSplitMUPCarvesOnLengthAtOffsetThree(t *testing.T) {
	a := mupNLRI(1, 0xaa, 0xbb, 0xcc)
	b := mupNLRI(4)
	c := mupNLRI(2, 0xdd)

	got, err := SplitMUP(concat(a, b, c), false)
	require.NoError(t, err)
	assert.Equal(t, [][]byte{a, b, c}, got)
}

// VALIDATES: the MUP header boundary is four octets, not two.
// PREVENTS: a three-octet tail being accepted as a whole NLRI and read past.
func TestSplitMUPBoundaries(t *testing.T) {
	_, err := SplitMUP([]byte{1, 0x00, 0x01, 0x00}, false)
	assert.NoError(t, err, "a four-octet header with a zero-length body is well framed")
	_, err = SplitMUP([]byte{1, 0x00, 0x01}, false)
	assert.Error(t, err, "three octets cannot hold the MUP header")

	good := mupNLRI(1, 0xaa)
	partial, err := SplitMUP(concat(good, []byte{1, 0x00, 0x01, 0x02, 0xaa}), false)
	assert.Error(t, err, "the last NLRI declares two octets and carries one")
	assert.Equal(t, [][]byte{good}, partial)
}

// VALIDATES: both families are reachable through the registry, which is what
// nlritype.Register requires before a Section 5.4 recognizer may be installed.
// PREVENTS: a recognizer registration failing at startup because its family has no
// splitter, which is a fatal error in the owning plugin's init.
func TestMVPNAndMUPAreRegistered(t *testing.T) {
	for _, fam := range []family.Family{
		{AFI: family.AFIIPv4, SAFI: family.SAFIMVPN},
		{AFI: family.AFIIPv6, SAFI: family.SAFIMVPN},
		{AFI: family.AFIIPv4, SAFI: family.SAFIMUP},
		{AFI: family.AFIIPv6, SAFI: family.SAFIMUP},
	} {
		assert.True(t, Supported(fam), "%s must have a splitter", fam)
	}
}

func concat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}
