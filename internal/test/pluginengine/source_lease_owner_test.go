// Design: docs/contributing/testing.md -- source identity in transport fixtures.
package pluginengine

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSourceLeaseFixtureBindsExplicitPeerOwner issues a real fixture receipt;
// wrong owners cannot mutate it, and terminal zero-owner disposal stays useful.
func TestSourceLeaseFixtureBindsExplicitPeerOwner(t *testing.T) {
	engine := &Engine{}
	receipt := engine.IssueSource("192.0.2.1", 701)
	invoke := func(action rpc.SourceLeaseAction, owner uint64) error {
		input := rpc.SourceLeaseInput{Action: action, SourceReceipt: receipt, PeerOwner: owner,
			DeadlineMS: time.Now().Add(time.Second).UnixMilli()}
		raw, err := json.Marshal(input)
		require.NoError(t, err)
		output, err := engine.sourceLeaseAnswer(raw)
		if err == nil {
			require.NoError(t, output.Validate(input))
		}
		return err
	}
	require.Error(t, invoke(rpc.SourceLeaseAcquire, 702))
	require.False(t, engine.sources[receipt].acquired)
	require.NoError(t, invoke(rpc.SourceLeaseAcquire, 701))
	for _, action := range []rpc.SourceLeaseAction{rpc.SourceLeaseRetain, rpc.SourceLeaseRelease} {
		require.Error(t, invoke(action, 702))
		require.True(t, engine.sources[receipt].acquired)
	}
	require.NoError(t, invoke(rpc.SourceLeaseRetain, 701))
	require.True(t, engine.sources[receipt].retained)
	require.NoError(t, invoke(rpc.SourceLeaseRelease, 0))
	require.False(t, engine.sources[receipt].acquired)
	require.False(t, engine.sources[receipt].retained)
}
