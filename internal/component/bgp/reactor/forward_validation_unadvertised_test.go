// Design: docs/architecture/bgp/structural-forwarding.md -- receive validation and cache changes
// Related: session_ownership.go -- final writer admission of synthesized withdrawals
package reactor

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/bgp/wire"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// advertiseFirst writes body to the destination through its real Session writer
// as the source's forwarded announcement, then forgets that output. A withheld
// route is withdrawn only from a destination that was advertised it (RFC 4271
// Section 3.1), so a fixture that proves a gate's withdrawal first establishes
// the recipient owner this way (structural-forwarding.md, "Withdrawal and
// recovery fixtures").
func (f *aigpReplayFixture) advertiseFirst(t *testing.T, body []byte) {
	t.Helper()
	require.NoError(t, ownershipWriterForward(t, f.destination, f.source, body, false))
	f.conn.reset()
}

// validationSocketCounts drains the destination worker and counts, over the
// UPDATEs written to the socket since the last call, those announcing and
// those withdrawing prefix.
func validationSocketCounts(t *testing.T, f *aigpReplayFixture, prefix []byte, seen *int) (announced, withdrawn int) {
	t.Helper()
	forwardSocketBarrier(t, f.r)
	bodies := aigpSocketBodies(t, f.conn)
	for _, body := range bodies[*seen:] {
		sections, err := wire.ParseUpdateSections(body)
		require.NoError(t, err)
		if bytes.Equal(sections.NLRI(body), prefix) {
			announced++
		}
		if bytes.Equal(sections.Withdrawn(body), prefix) {
			withdrawn++
		}
	}
	*seen = len(bodies)
	return announced, withdrawn
}

// TestValidationHeldPathWithdrawsOnlyAdvertised drives the two rails a receive
// validation verdict reaches a destination on, the live forward of each received
// UPDATE and the stored-route relay a ValidationChange triggers, through the real
// Session writer. A path held ineligible from its first receipt was never
// advertised, so no gate event may send the destination its withdrawal. Once the
// path was advertised, turning ineligible withdraws it exactly once however many
// gate events follow, and recovery announces it again.
func TestValidationHeldPathWithdrawsOnlyAdvertised(t *testing.T) {
	eligible := false
	ribevents.RegisterValidationLookup(func(ribevents.ValidationRoute, uint64) bool { return eligible }, nil)
	t.Cleanup(func() { ribevents.RegisterValidationLookup(nil, nil) })
	f := newAIGPReplayFixture(t, nil)
	f.metric(7)
	prefix := []byte{24, 10, 20, 0}
	route := storedIPv4Route(f.source.Settings().Address.String())
	route.SourcePolicyEpoch = f.source.sourcePolicyEpoch.Load()
	route.NLRIHex = "180A1400"
	api := &reactorAPIAdapter{r: f.r}
	relay := func() {
		t.Helper()
		require.NoError(t, api.RelayStoredRoute(f.destination.Settings().Address, []rpc.StoredRoute{route}, plugin.OperatorSender()))
	}
	receive := func() {
		t.Helper()
		f.forward(t, f.receive(t, f.body(t, 100)))
	}
	seen := 0

	// Pending, then Invalid: two receipts and two ValidationChange relays.
	receive()
	relay()
	receive()
	relay()
	announced, withdrawn := validationSocketCounts(t, f, prefix, &seen)
	require.Zero(t, announced, "an ineligible path must not be advertised")
	require.Zero(t, withdrawn, "a path never advertised to the destination must not be withdrawn from it")

	eligible = true
	relay()
	announced, withdrawn = validationSocketCounts(t, f, prefix, &seen)
	require.Equal(t, 1, announced, "the revalidated path must be announced")
	require.Zero(t, withdrawn)

	eligible = false
	relay()
	receive()
	relay()
	announced, withdrawn = validationSocketCounts(t, f, prefix, &seen)
	require.Zero(t, announced)
	require.Equal(t, 1, withdrawn, "an advertised path turning ineligible must be withdrawn exactly once")

	eligible = true
	relay()
	announced, withdrawn = validationSocketCounts(t, f, prefix, &seen)
	require.Equal(t, 1, announced, "recovery must announce the path again")
	require.Zero(t, withdrawn)
}
