// Design: docs/architecture/update-building.md -- source withdrawals are owed to an active recovery owner.
// Related: filter_atomic_retirement_pipeline_test.go -- accepted marked subset groups.
// Related: recovery_owed_worker_test.go -- channel-gated recovery lookup and live writer.
package reactor

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestAtomicRetirementRecoveryCanFollowUnrelatedFence proves that an accepted
// marked subset owes its excluded key to an active RIB owner. The lookup channel
// holds that decision while the useful sibling and an unrelated wire fence pass.
// Releasing the owner either withdraws A or installs its alternate; a blind
// immediate withdrawal cannot satisfy the alternate control.
func TestAtomicRetirementRecoveryCanFollowUnrelatedFence(t *testing.T) {
	first, second, fence := []byte{24, 10, 46, 0}, []byte{24, 10, 47, 0}, []byte{24, 10, 48, 0}
	pair := append(bytes.Clone(first), second...)
	for _, alternateWins := range []bool{false, true} {
		name := "no-alternate"
		if alternateWins {
			name = "alternate-survives"
		}
		t.Run(name, func(t *testing.T) {
			rail := newOwedRail(t)
			// The legacy owed-writer fixture has no received context ID. The
			// policy guard needs the actual four-octet AS-path framing here.
			receiveContext := bgpctx.EncodingContextForASN4(true)
			receiveContextID, err := bgpctx.Registry.Register(receiveContext)
			require.NoError(t, err)
			rail.source.recvCtxID = receiveContextID
			alternate, _ := newAnnouncePeer(t, "10.0.0.3")
			alternate.recvCtxID = rail.source.recvCtxID
			negotiateForwardTestFamilies(alternate, family.IPv4Unicast)
			rail.r.mu.Lock()
			rail.r.peers[alternate.Settings().PeerKey()] = alternate
			rail.r.mu.Unlock()
			seed := rfc4271ProofRelayBody(pair, true, 10)
			require.NoError(t, ownershipWriterForward(t, rail.destination, rail.source, seed, false))
			seeded := recoveryWrittenUpdates(t, rail.conn.written())
			require.Len(t, seeded, 1)
			require.Equal(t, pair, seeded[0].NLRI, "the recipient must actually hold both source keys")

			input := rfc4271ProofRelayBody(pair, true, 20)
			output := rfc4271ProofRelayBody(second, true, 77)
			exportLibraryInputAnswer(t, rail.r, func(route rpc.FilterUpdateInput) rpc.FilterUpdateOutput {
				if bytes.Equal(route.Raw, input) {
					return rpc.FilterUpdateOutput{Action: rpc.FilterModify, Raw: output}
				}
				return rpc.FilterUpdateOutput{Action: rpc.FilterAccept}
			}, rail.destination)
			decision := rail.r.runEgressPolicyChain(rail.destination.settings.ExportPolicy,
				rail.destination.addrString, rail.destination.PeerAS(), rail.destination.settings.LocalAS, true,
				explicitExportTestSession(false), wireu.NewWireUpdate(input, rail.source.recvCtxID))
			require.True(t, decision.accept, "the subset must be accepted before recovery ownership decides A")
			require.False(t, decision.failed)
			require.NotNil(t, decision.wireOverride)
			require.Equal(t, output, decision.wireOverride.Payload())
			require.Len(t, decision.removals, 1)
			removal, err := message.UnpackUpdate(decision.removals[0])
			require.NoError(t, err)
			require.Equal(t, first, removal.WithdrawnRoutes, "the accepted subset owes exactly A")
			require.Empty(t, removal.NLRI)
			require.Empty(t, removal.PathAttributes)

			target, err := selector.Parse(rail.destination.Settings().Address.String())
			require.NoError(t, err)
			adapter := &reactorAPIAdapter{r: rail.r}
			syncOrderPublish(t, rail.r, rail.source.recvCtxID, 101, input)
			require.NoError(t, adapter.ForwardUpdate(target, 101, rsFixtureRelay, plugin.ProcessSender(rsFixtureRelay)))
			request := rail.request(t) // The cold owner is held until this test answers it.
			require.Len(t, request.Keys, 1)
			require.Equal(t, first, request.Keys[0].NLRI)
			require.Empty(t, request.Keys[0].Excluded)
			require.Len(t, rail.owed(), 1)

			syncOrderPublish(t, rail.r, rail.source.recvCtxID, 102, rfc4271ProofRelayBody(fence, true, 70))
			require.NoError(t, adapter.ForwardUpdate(target, 102, rsFixtureRelay, plugin.ProcessSender(rsFixtureRelay)))
			lowEventually(t, func() bool {
				return len(recoveryWrittenUpdates(t, rail.conn.written())) == 3
			}, "the useful sibling and unrelated fence pass the held recovery decision")
			beforeDecision := recoveryWrittenUpdates(t, rail.conn.written())
			require.Equal(t, second, beforeDecision[1].NLRI)
			require.Equal(t, fence, beforeDecision[2].NLRI)
			for _, update := range beforeDecision {
				require.Empty(t, update.WithdrawnRoutes, "the active owner has not decided A yet")
				require.False(t, update.IsEndOfRIBAnyFamily())
			}
			require.Len(t, rail.owed(), 1, "an unrelated wire fence does not settle recovery debt")

			var answer owedAnswer
			if alternateWins {
				survivor, err := message.UnpackUpdate(rfc4271ProofRelayBody(first, true, 88))
				require.NoError(t, err)
				answer = func(request ribevents.RecoveryRequest) []ribevents.RecoveryOutcome {
					return recoverySurvivorAnswer(request, first, ribevents.RecoveryRoute{
						Source: alternate.Settings().Address, SourcePolicyEpoch: alternate.sourcePolicyEpoch.Load(),
						NLRI: first, Attributes: survivor.PathAttributes, NextHop: []byte{198, 18, 231, 1}, MessageID: 103,
					})
				}
			}
			// A bounded set of replies also covers a retry of the same record;
			// no lookup can remain blocked during the fixture's cleanup join.
			for range 4 {
				rail.release <- answer
			}
			lowEventually(t, func() bool {
				if len(rail.owed()) != 0 {
					return false
				}
				return len(recoveryWrittenUpdates(t, rail.conn.written())) == 4
			}, "the released owner writes its final decision")
			decided := recoveryWrittenUpdates(t, rail.conn.written())
			last := decided[len(decided)-1]
			if alternateWins {
				require.Equal(t, first, last.NLRI)
				require.Empty(t, last.WithdrawnRoutes, "the eligible alternate replaces A without a blind withdrawal")
				for _, update := range decided {
					require.Empty(t, update.WithdrawnRoutes)
				}
				rail.forward(t, true, first)
				require.Len(t, recoveryWrittenUpdates(t, rail.conn.written()), 4,
					"the original source cannot withdraw the alternate owner's route")
				require.Empty(t, rail.owed())
			} else {
				require.Equal(t, first, last.WithdrawnRoutes)
				require.Empty(t, last.NLRI)
			}
			require.False(t, last.IsEndOfRIBAnyFamily(), "the decision is a real route update")
		})
	}
}
