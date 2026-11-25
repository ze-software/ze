// Design: docs/architecture/plugin/rib-storage-design.md -- original source catch-up before replay.
// Related: replay_sources.go -- read-only progress of an exact physical ingress.
package adj_rib_in

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
)

// VALIDATES: the actual replay-progress command waits for the original
// physical source to apply its UPDATE on both store input transports.
// PREVENTS: a higher message from an alias or replacement covering an
// unrelated physical connection.
func TestReplaySourceProgressRequiresOriginalPhysicalIngest(t *testing.T) {
	for _, rail := range []struct {
		name string
		json bool
	}{
		{name: "typed"},
		{name: "full-json", json: true},
	} {
		t.Run(rail.name, func(t *testing.T) {
			producer := newAdjOwnerProducer(t, rail.json)
			source := ribevents.ReplaySource{Address: producer.address, PeerOwner: 1701, ConnectionID: 41, MessageID: 80}
			payload, err := json.Marshal([]ribevents.ReplaySource{source})
			require.NoError(t, err)
			covered := func() bool {
				status, raw, err := producer.r.replayRoutesCommand([]string{"192.0.2.2", "0", ribevents.ReplaySourceProgressArgument, string(payload)})
				require.NoError(t, err)
				require.Equal(t, statusDone, status)
				answer, ok := raw.(ribevents.ReplaySourceProgress)
				require.True(t, ok)
				require.NotNil(t, answer.Covered)
				return *answer.Covered
			}
			producer.state(1701, 41, true)
			producer.receive(1701, 41, 79, 1)
			producer.state(1702, 42, true)
			producer.receive(1702, 42, 100, 2)
			require.False(t, covered(), "a higher message from an alias must not cover the original")
			producer.receive(1701, 41, 80, 3)
			require.True(t, covered(), "the original UPDATE advances the original connection")
			producer.state(1701, 43, true)
			producer.receive(1701, 43, 120, 4)
			require.False(t, covered(), "a replacement connection must not cover its predecessor")
		})
	}
}
