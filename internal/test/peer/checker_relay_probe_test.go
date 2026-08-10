package peer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The destination peer's frames in test/plugin/role-otc-rs-withdraw-eor.ci: a
// relayed route, a relayed withdraw, and the RELAYED End-of-RIB the fixture
// declares at seq 3. That marker is byte-identical to the destination's OWN
// initial-sync marker, so the two are indistinguishable on the wire.
const (
	relayRouteHex    = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF0036020000001B4001010040020602010000FDE940030401010101C023040000FDE8180A0000"
	relayWithdrawHex = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001B020004180A00000000"
	relayEORHex      = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000"
)

// TestCheckerRelayShapeToleratesAnEarlyIdenticalMarker is the live wire of
// test/plugin/role-otc-rs-withdraw-eor.ci, and it is what makes the tolerance a
// requirement rather than a convenience.
//
// VALIDATES: a marker that matches a LATER expectation is accepted when it
// arrives early, and the expectation it matched is then satisfied by the SECOND,
// identical marker that follows. The four-frame sequence completes.
// PREVENTS: refusing that marker on arrival. The refusal red this fixture: ze
// sends the destination its own End-of-RIB at establishment (sendInitialRoutes,
// peer_initial_sync.go, and the route server's own after replay, sendEOR in
// server_handlers.go), the destination peer failed on it, the session closed,
// and the route server then had no target for the withdrawal the fixture exists
// to assert -- "forward matched no target ... 127.0.0.2=down".
//
// Declaring that first marker in the fixture is not the answer. Its position
// against the relayed route is a race with no happens-before: the route server
// learns the peer is up inside notifyPeerEstablished and sendInitialRoutes
// starts in a goroutine after that call returns, and the relayed route reaches
// the peer on the replay rail or on the live forward rail depending on which of
// the two sessions establishes first. A fixture that declared it would pin a
// coin toss.
func TestCheckerRelayShapeToleratesAnEarlyIdenticalMarker(t *testing.T) {
	c, err := newChecker([]string{
		"expect=bgp:conn=1:seq=1:hex=" + relayRouteHex,
		"expect=bgp:conn=1:seq=2:hex=" + relayWithdrawHex,
		"expect=bgp:conn=1:seq=3:hex=" + relayEORHex,
	})
	require.NoError(t, err)
	c.Init()

	matched, silent := c.ExpectedOrKeepalive(mkFrame(t, relayEORHex))
	assert.False(t, matched, "the marker does not satisfy the seq-1 route expectation")
	assert.True(t, silent, "ze's own establishment marker is not the fixture's business")
	assert.Contains(t, c.takeMisorderNote(), relayEORHex,
		"it is accepted, and recorded: had the run gone on to fail, this frame is a suspect")

	for _, frameHex := range []string{relayRouteHex, relayWithdrawHex, relayEORHex} {
		m, s := c.ExpectedOrKeepalive(mkFrame(t, frameHex))
		assert.True(t, m, "frame %.46s must satisfy its own expectation", frameHex)
		assert.False(t, s)
	}
	assert.True(t, c.Completed(),
		"the relayed marker fills seq 3, so the fixture passes on the frames it declared")
}

// TestCheckerRelayShapeInDeclaredOrder keeps the passing path honest: the same
// three frames with no early marker match one for one and record nothing.
func TestCheckerRelayShapeInDeclaredOrder(t *testing.T) {
	c, err := newChecker([]string{
		"expect=bgp:conn=1:seq=1:hex=" + relayRouteHex,
		"expect=bgp:conn=1:seq=2:hex=" + relayWithdrawHex,
		"expect=bgp:conn=1:seq=3:hex=" + relayEORHex,
	})
	require.NoError(t, err)
	c.Init()

	for _, frameHex := range []string{relayRouteHex, relayWithdrawHex, relayEORHex} {
		m, s := c.ExpectedOrKeepalive(mkFrame(t, frameHex))
		assert.True(t, m, "frame %.46s must satisfy its own expectation", frameHex)
		assert.False(t, s)
	}
	assert.True(t, c.Completed())
	assert.Empty(t, c.misorderNotes())
}
