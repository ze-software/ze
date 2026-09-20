package rib

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// TestFormatNLRIAsPrefixKeepsAPathIdentifierOfZero pins the operator-visible RIB
// text for an ADD-PATH route whose Path Identifier is zero.
//
// RFC 7911 Section 3 gives the Path Identifier four octets and reserves no
// value, so zero is an identifier a peer can legitimately send. The function
// wrote `ap && pathID != 0`, and ap is the negotiated layout these octets were
// read under: the fact it needed was already in the function, and the value
// test threw it away. A peer sending identifier zero was shown as a route that
// carried none, beside a peer's route that really carried none.
//
// VALIDATES: the negotiated flag alone decides the [pathID=N] clause, so zero
// is rendered and a non-ADD-PATH route stays a bare prefix.
// PREVENTS: `show ... rib` collapsing an ADD-PATH path of identifier zero onto
// a plain route.
func TestFormatNLRIAsPrefixKeepsAPathIdentifierOfZero(t *testing.T) {
	fam, ok := parseFamily("ipv4/unicast")
	require.True(t, ok)

	// [00 00 00 00][18 0a 00 00] = Path Identifier 0, then 10.0.0.0/24.
	addPathZero := []byte{0, 0, 0, 0, 24, 10, 0, 0}
	assert.Equal(t, "10.0.0.0/24 [pathID=0]", formatNLRIAsPrefix(fam, addPathZero, true))

	// [00 00 00 07][18 0a 00 00] = Path Identifier 7, then 10.0.0.0/24.
	addPathSeven := []byte{0, 0, 0, 7, 24, 10, 0, 0}
	assert.Equal(t, "10.0.0.0/24 [pathID=7]", formatNLRIAsPrefix(fam, addPathSeven, true))

	// The same prefix with no ADD-PATH carries no identifier and gets no clause.
	plain := []byte{24, 10, 0, 0}
	assert.Equal(t, "10.0.0.0/24", formatNLRIAsPrefix(fam, plain, false))
	assert.Equal(t, "10.0.0.0/24", formatNLRIAsPrefix(fam, plain))
}

// TestFormatNLRIAsPrefixIPv6AddPathZero holds the same fact for IPv6, whose
// prefix bytes are read by the other arm of wireToPrefix.
//
// VALIDATES: an IPv6 ADD-PATH route with identifier zero keeps its clause.
// PREVENTS: the repair landing on the IPv4 arm alone.
func TestFormatNLRIAsPrefixIPv6AddPathZero(t *testing.T) {
	fam := family.IPv6Unicast

	// [00 00 00 00][20 20 01 0d b8] = Path Identifier 0, then 2001:db8::/32.
	wire := []byte{0, 0, 0, 0, 32, 0x20, 0x01, 0x0d, 0xb8}
	assert.Equal(t, "2001:db8::/32 [pathID=0]", formatNLRIAsPrefix(fam, wire, true))
}
