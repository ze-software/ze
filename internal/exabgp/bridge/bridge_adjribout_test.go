// Design: docs/architecture/exabgp-bridge.md — clear and flush of the adj-rib-out
// Related: bridge_batch.go — Net, the fourth rule

package bridge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBridgeAdjRIBOutClearCancelsEarlierRoutes holds ExaBGP's `clear adj-rib out`
// against the routes written before it in the same read.
//
// OutgoingRIB.withdraw at the pin pops every route queued for the peer, so an
// announce or a withdrawal written earlier in the batch never reaches the wire.
//
// VALIDATES: Net dispatches the clear alone when every earlier route was aimed at
// the cleared peers.
// PREVENTS: ze announcing a route and withdrawing it in the next UPDATE, a pair
// ExaBGP never sends (api-rib, api-rr-rib).
func TestBridgeAdjRIBOutClearCancelsEarlierRoutes(t *testing.T) {
	netted := Net(batchOf(t,
		"announce route 1.1.0.0/24 next-hop 101.1.101.1",
		"withdraw route 1.2.0.0/24 next-hop 101.1.101.1",
		"clear adj-rib out",
	))

	assert.Equal(t, []string{"clear bgp rib advertised *"}, dispatchedText(netted))
}

// TestBridgeAdjRIBOutClearKeepsLaterRoutesAfterIt proves what the script writes
// after the clear is queued after it, so the clear does not withdraw it.
//
// VALIDATES: an announce written after the clear dispatches after the clear,
// although Net otherwise sends every route before every command that carries none.
// PREVENTS: the engine withdrawing the route the script announced last.
func TestBridgeAdjRIBOutClearKeepsLaterRoutesAfterIt(t *testing.T) {
	netted := Net(batchOf(t,
		"announce route 1.1.0.0/24 next-hop 101.1.101.1",
		"clear adj-rib out",
		"announce route 1.3.0.0/24 next-hop 101.1.101.1",
	))

	texts := dispatchedText(netted)
	require.Len(t, texts, 2, "the earlier announce is canceled: %q", texts)
	assert.Equal(t, "clear bgp rib advertised *", texts[0])
	assert.Contains(t, texts[1], "add 1.3.0.0/24")
}

// TestBridgeAdjRIBOutClearLeavesOtherPeersRoutes proves the cancellation follows
// the clear's selector: a route aimed at a peer the clear leaves alone stands, and
// so does one aimed wider than the clear, which the clear then withdraws from the
// cleared peer alone.
func TestBridgeAdjRIBOutClearLeavesOtherPeersRoutes(t *testing.T) {
	netted := Net(batchOf(t,
		"neighbor 10.0.0.1 announce route 1.1.0.0/24 next-hop 101.1.101.1",
		"neighbor 10.0.0.2 announce route 1.2.0.0/24 next-hop 101.1.101.1",
		"announce route 1.3.0.0/24 next-hop 101.1.101.1",
		"neighbor 10.0.0.1 clear adj-rib out",
	))

	texts := dispatchedText(netted)
	require.Len(t, texts, 3, "only the 10.0.0.1 announce is canceled: %q", texts)
	assert.Contains(t, texts[0], "add 1.2.0.0/24")
	assert.Contains(t, texts[1], "add 1.3.0.0/24")
	assert.Equal(t, "clear bgp rib advertised 10.0.0.1", texts[2])
}

// TestBridgeAdjRIBOutFlushCancelsNothing proves `flush adj-rib out` is a resend,
// not a withdrawal: the routes before it stand and it dispatches after them.
func TestBridgeAdjRIBOutFlushCancelsNothing(t *testing.T) {
	netted := Net(batchOf(t,
		"announce route 1.1.0.0/24 next-hop 101.1.101.1",
		"flush adj-rib out",
	))

	texts := dispatchedText(netted)
	require.Len(t, texts, 2, "%q", texts)
	assert.Contains(t, texts[0], "add 1.1.0.0/24")
	assert.Equal(t, "clear bgp rib out *", texts[1])
}
