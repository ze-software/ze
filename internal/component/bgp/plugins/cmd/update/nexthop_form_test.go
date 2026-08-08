package update

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestParseUpdateTextRejectsLinkLocalNextHop drives the RFC 2545 Section 3
// next-hop form guard from the command grammar that reaches it, not from the
// helper alone.
//
// RFC 2545 Section 3: "A BGP speaker shall advertise to its peer in the Network
// Address of Next Hop field the global IPv6 address of the next hop, potentially
// followed by the link-local IPv6 address of the next hop." The link-local half
// is appended by the encoder from the session's link-local leaf when the
// section's condition holds, so one offered as THE next hop is refused here.
//
// VALIDATES: `update text ... next-hop fe80::cafe ...` fails to parse.
// PREVENTS: a 16-octet Next Hop field whose only address is link-local.
func TestParseUpdateTextRejectsLinkLocalNextHop(t *testing.T) {
	_, err := ParseUpdateText(strings.Fields(
		"origin igp next-hop fe80::cafe nlri ipv6/unicast add 2001:db8:1::1/128"))

	require.Error(t, err)
	assert.ErrorIs(t, err, attribute.ErrLinkLocalNextHop)
	assert.Contains(t, err.Error(), "fe80::cafe")
}

// TestParseUpdateTextAcceptsGlobalNextHop is the other side of the guard.
//
// VALIDATES: a global IPv6 next hop, the loopback, an IPv4 next hop and `self`
// all parse through the same grammar.
func TestParseUpdateTextAcceptsGlobalNextHop(t *testing.T) {
	for _, nextHop := range []string{"2001:db8::ffff", "::1", "192.0.2.1", "self"} {
		t.Run(nextHop, func(t *testing.T) {
			_, err := ParseUpdateText(strings.Fields(
				"origin igp next-hop " + nextHop +
					" nlri ipv6/unicast add 2001:db8:1::1/128"))
			assert.NoError(t, err)
		})
	}
}
