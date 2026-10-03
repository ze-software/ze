// Design: docs/architecture/core-design.md -- egress route decisions on the forward rails
// Related: reactor_api_forward.go -- forwardUpdateSection, the split-horizon refusal under test
// Related: rfc8950_reactor_a2_forward_test.go -- a2ForwardWith and a2Parts, the harness

package reactor

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRFC4271ForwardRailNeverRedistributesInternalRoutesToInternalPeers drives
// RFC 4271 Section 9.2 on the general forward rail (forwardUpdateCore, which
// runs forwardUpdateSection): "When a BGP speaker receives an UPDATE message
// from an internal peer, the receiving BGP speaker SHALL NOT re-distribute the
// routing information contained in that UPDATE message to other internal peers
// (unless the speaker acts as a BGP Route Reflector [RFC2796])."
//
// Method: one route from an internal source that is not a route-reflector
// client fans out to three destinations at once: an internal non-client, an
// external peer, and an internal peer configured as this speaker's
// route-reflector client. The external and client destinations are the
// controls that the fan-out ran; the non-client is waited for over the same
// window as the two that arrive.
//
// VALIDATES: the internal non-client destination is sent nothing; the external
// destination is sent the route; the route-reflector client destination is sent
// it too, which is the Route Reflector exception with a client destination.
// PREVENTS: the general rail re-distributing an IBGP-learned route to another
// internal peer, which only the route-server rail was proven to refuse.
//
// RFC requirement: RFC4271-9.2-6 negative -- on the general forward rail, a route from an internal non-client source is not sent to an internal non-client destination, while the external destination and a route-reflector client destination in the same fan-out are sent it.
func TestRFC4271ForwardRailNeverRedistributesInternalRoutesToInternalPeers(t *testing.T) {
	nonClient := a2Dest(t, "192.0.2.71", 65000, netip.Addr{}, false)
	external := a2Dest(t, "192.0.2.72", 65072, netip.Addr{}, false)
	client := a2Dest(t, "192.0.2.73", 65000, netip.Addr{}, false)
	client.settings.RouteReflectorClient = true
	client.refreshForwardFacts()

	internalSource := func(_ *Reactor, source *forwardSourceInfo) {
		source.isIBGP = true
		source.isRRClient = false
		source.remoteRouterID = 0x0A000009
	}
	got := a2ForwardWith(t, false, internalSource, a2InlinePayload(), nonClient, external, client)

	_, sent := got[netip.MustParseAddr("192.0.2.71")]
	assert.False(t, sent, "an internal route SHALL NOT be re-distributed to another internal peer")

	for _, addr := range []string{"192.0.2.72", "192.0.2.73"} {
		parts, ok := got[netip.MustParseAddr(addr)]
		require.True(t, ok, "%s is owed the route", addr)
		assert.Equal(t, a2InlinePrefix, parts.nlri, "%s is sent the route", addr)
	}
}
