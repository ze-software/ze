// Design: docs/architecture/bgp/structural-forwarding.md -- mixed-refusal native handback
// Related: ls71_pair_recovery_test.go -- explicit seed, refusal, and recovery phases
// Related: relay_proof_helpers_test.go -- strict native dispatch and cached forwarding
package reactor

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ls71PairRelay requires the phase's exact rail outcome before its caller checks
// the complete wire result. Only a known policy-produced mixed refusal expects
// handback; positive phases and received refusals keep the strict shared driver.
func ls71PairRelay(t *testing.T, f *aigpReplayFixture, body []byte, native, expectHandback bool) {
	t.Helper()
	original := bytes.Clone(body)
	defer func() {
		require.Equal(t, original, body, "received bytes stay borrowed and unchanged")
	}()
	if !expectHandback {
		rfc4271ProofRelay(t, f, body, native)
		return
	}

	require.True(t, native, "only the native refusal phase expects handback")
	require.Empty(t, f.conn.written(), "the caller must drain and reset the preceding phase")
	facts := f.destination.forwardFacts()
	require.NotNil(t, facts)
	require.True(t, facts.exportChainActive, "mixed policy refusal must use the active export chain")
	f.r.rsForwardingEnabled = false
	id := f.receive(t, body)
	update, present := f.r.recentUpdates.Get(id)
	require.True(t, present)
	f.r.rsForwardingEnabled = true
	skipped, dispatched := reactorForwardRS(f.r, update, id, f.source.Settings().Address, f.source)
	f.r.rsForwardingEnabled = false
	require.Equal(t, []rpc.ForwardTarget{capturedForwardTarget(f.destination, f.destination.currentSession())}, skipped,
		"the native rail must hand back exactly the intended recipient")
	require.Zero(t, dispatched, "the native rail must dispatch nothing before handback")
	f.source.session.flushFwdDirty()
	require.Empty(t, f.source.session.fwdDirty, "the native read boundary flushes every dirty destination")
	f.drain(t)
	require.Empty(t, f.conn.written(), "the native rail must emit no bytes before cached continuation")

	// The plugin rail forwards this same cache ID to the handed-back recipient.
	// The existing fixture selects only its destination; ForwardUpdate resolves
	// the original source from this entry before it runs the real egress chain.
	current, present := f.r.recentUpdates.Get(id)
	require.True(t, present)
	require.Same(t, update, current)
	require.Equal(t, f.source.Settings().Address, current.SourcePeerIP)
	require.Same(t, f.source, current.receivedPeer)
	require.Equal(t, id, current.WireUpdate.MessageID())
	require.Equal(t, original, current.WireUpdate.Payload(), "handback must retain the received UPDATE")
	f.forward(t, id)
	f.drain(t)
}
