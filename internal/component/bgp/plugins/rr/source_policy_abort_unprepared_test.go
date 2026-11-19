// Design: docs/architecture/plugin/rib-storage-design.md -- unprepared source-policy compensation.
package rr

import (
	"context"
	"maps"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/core/bgp/sourcepolicy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// A zero-target abort of a transaction that never prepared has nothing to
// release. Exercise the real owner with no ledger and another prepared ledger;
// neither may change pins, epochs or output. A targeted or ended-actor request
// retains its refusal, so this no-op cannot authorize a different operation.
func TestRRUnpreparedAbortPreservesLiveSource(t *testing.T) {
	for _, held := range []bool{false, true} {
		name := "no-ledger"
		if held {
			name = "another-prepared-ledger"
		}
		t.Run(name, func(t *testing.T) {
			rr, engine := newSourcePolicyReflector(t)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			if held {
				require.NoError(t, rr.handleSourcePolicy(ctx, spInput("held", rpc.SourcePolicyPrepare, 0)))
			}
			original := rr.sourcePolicy[1]
			var pins map[withdrawalKey]withdrawalInfo
			if held {
				require.NotNil(t, original)
				pins = maps.Clone(original.pins)
			}
			epochs := spEpochs(rr, spSource)
			require.Empty(t, engine.take(), "preparing the control must emit no output")
			unchanged := func() {
				t.Helper()
				rr.withdrawalMu.Lock()
				defer rr.withdrawalMu.Unlock()
				require.True(t, rr.sourcePolicy[1] == original, "unprepared compensation replaced the current ledger")
				if held {
					require.Equal(t, sourcepolicy.StagePrepared, original.ledger.Stage())
					require.Equal(t, pins, original.pins, "another transaction's pins changed")
				}
				spSameEpochs(t, "unprepared compensation", spEpochs(rr, spSource), epochs)
			}
			for _, target := range []uint64{0, 7} {
				err := rr.handleSourcePolicy(ctx, spInput("never-prepared", rpc.SourcePolicyAbort, target))
				if target == 0 {
					require.NoError(t, err, "there are no pins for this transaction to release")
				} else {
					require.Error(t, err, "an unprepared request cannot restamp a source")
				}
				unchanged()
				require.Empty(t, engine.take(), "unprepared compensation must emit no output")
			}
			actor := rrOriginalFixture(t, spSource, 1, 41)
			rr.mu.Lock()
			rr.peers[actor].Up = false
			rr.mu.Unlock()
			require.ErrorContains(t, rr.handleSourcePolicy(ctx, spInput("never-prepared", rpc.SourcePolicyAbort, 0)),
				"physical actor is no longer live")
			unchanged()
			require.Empty(t, engine.take(), "an ended original actor must emit no output")
		})
	}
}
