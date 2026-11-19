// Design: docs/guide/redistribution.md -- original accepted contribution removal
// Related: held.go -- the original consumer episode must survive replacement

package redistribute

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
)

// TestWithdrawalUsesOriginalAcceptingConsumer drives actual configured Add and
// Remove dispatch across registry replacement. All three contributions share a
// prefix, so source instances and the accepting episode decide what may leave.
// The unchanged-registry case proves ordinary useful dispatch still works.
// MUTATION: resolving the consumer by current registry name redirects removal.
func TestWithdrawalUsesOriginalAcceptingConsumer(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(map[bool]string{false: "same episode", true: "replacement episode"}[replace], func(t *testing.T) {
			resetState(t)
			protocol := redistevents.RegisterProtocol("withdrawal-episode-source")
			configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{{
				Source: "withdrawal-episode-source", Destination: "bgp", Final: "accept",
				Chain: familyChain{family.IPv4Unicast},
			}}))
			original := &withdrawalEpisodeConsumer{live: make(map[uint32]configredist.RouteEntry)}
			require.NoError(t, configredist.RegisterConsumer(original))
			stopWatching := watchConsumers(newTestBus(), newReplayCoordinator())
			t.Cleanup(stopWatching)
			publish := func(action redistevents.RouteAction, instance uint32) {
				batch := addBatch(protocol, afiIPv4, "192.0.2.0/24", "")
				batch.Entries[0].Action = action
				batch.Entries[0].SourceInstance = instance
				handleBatch(t.Context(), nil, batch)
			}
			publish(redistevents.ActionAdd, 11)
			publish(redistevents.ActionAdd, 22)
			require.Len(t, original.live, 2, "both original contributions must first be useful")
			current := original
			if replace {
				current = &withdrawalEpisodeConsumer{live: make(map[uint32]configredist.RouteEntry)}
				require.True(t, configredist.ReregisterConsumer(current))
			}
			publish(redistevents.ActionAdd, 33)
			require.Contains(t, current.live, uint32(33), "the current episode must also accept useful work")
			publish(redistevents.ActionRemove, 11)
			require.Len(t, original.removed, 1, "the original contribution was redirected to a replacement consumer")
			require.Equal(t, uint32(11), original.removed[0].SourceInstance)
			require.Equal(t, protocol, original.removed[0].Protocol)
			require.Equal(t, "192.0.2.0/24", original.removed[0].Prefix)
			require.NotContains(t, original.live, uint32(11))
			require.Contains(t, original.live, uint32(22), "same-prefix original neighbor must survive")
			require.Contains(t, current.live, uint32(33), "same-prefix current contribution must survive")
			if replace {
				require.Empty(t, current.removed, "old removal must not call the replacement episode")
			}
			publish(redistevents.ActionRemove, 33)
			require.NotContains(t, current.live, uint32(33), "the useful current contribution must remain removable")
			require.Contains(t, original.live, uint32(22))
		})
	}
}

// TestFailedWithdrawalRetriesTheOriginalEpisode proves a withdrawal the
// accepting consumer could not confirm keeps that consumer and the route's held
// record: the next Remove of the route withdraws it at the same original
// instance again, never at the instance that replaced it, and the record is
// released only once the original confirms.
// Method: the original answers its first withdrawal with an error, after a
// replacement registered under the same name; two Removes follow.
// MUTATION: releasing the record before the answer leaves the original one
// attempt; resolving the retry by registry name sends it to the replacement.
func TestFailedWithdrawalRetriesTheOriginalEpisode(t *testing.T) {
	resetState(t)
	protocol := redistevents.RegisterProtocol("withdrawal-retry-source")
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{{
		Source: "withdrawal-retry-source", Destination: "bgp", Final: "accept",
		Chain: familyChain{family.IPv4Unicast},
	}}))
	original := &failingWithdrawalConsumer{live: make(map[uint32]configredist.RouteEntry), failures: 1}
	require.NoError(t, configredist.RegisterConsumer(original))
	stopWatching := watchConsumers(newTestBus(), newReplayCoordinator())
	t.Cleanup(stopWatching)
	publish := func(action redistevents.RouteAction) {
		batch := addBatch(protocol, afiIPv4, "192.0.2.0/24", "")
		batch.Entries[0].Action = action
		batch.Entries[0].SourceInstance = 11
		handleBatch(t.Context(), nil, batch)
	}
	publish(redistevents.ActionAdd)
	require.Contains(t, original.live, uint32(11), "the original episode must first hold the route")
	replacement := &withdrawalEpisodeConsumer{live: make(map[uint32]configredist.RouteEntry)}
	require.True(t, configredist.ReregisterConsumer(replacement))
	key := heldKey{consumer: "bgp", source: "withdrawal-retry-source", family: family.IPv4Unicast, prefix: netip.MustParsePrefix("192.0.2.0/24"), instance: 11}

	publish(redistevents.ActionRemove)
	require.Len(t, original.attempts, 1, "the Remove must reach the episode that accepted the route")
	require.Contains(t, original.live, uint32(11), "the failed withdrawal left the original route in place")
	require.True(t, heldAnywhere(key), "an unconfirmed withdrawal must keep the held record")

	publish(redistevents.ActionRemove)
	require.Len(t, original.attempts, 2, "the next Remove must retry at the original episode")
	require.Equal(t, original.attempts[0], original.attempts[1], "the retry must name the same original contribution")
	require.NotContains(t, original.live, uint32(11), "the confirmed retry removed the original route")
	require.False(t, heldAnywhere(key), "a confirmed withdrawal must release the held record")
	require.Empty(t, replacement.removed, "no withdrawal of the original route may reach the replacement episode")
}

// TestStoreRetiresAnotherEpisodesRecord proves an Add settled at a new episode
// never overwrites the record an earlier episode holds for the same key: the
// earlier one is retired, and a Remove withdraws at both, oldest first.
// MUTATION: storing over h.set[k] loses the earlier episode's withdrawal.
func TestStoreRetiresAnotherEpisodesRecord(t *testing.T) {
	resetState(t)
	entry := redistevents.RouteChangeEntry{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix("192.0.2.0/24")}
	view := redistRoute("episode-store-source", 0, family.IPv4Unicast, &entry)
	k := heldKey{consumer: "bgp", source: "episode-store-source", family: family.IPv4Unicast, prefix: entry.Prefix}
	earlier, later := &stubConsumer{name: "bgp"}, &stubConsumer{name: "bgp"}
	_, gen := consumerHeld.evaluator()
	first, second := sentRoute{episode: earlier}, sentRoute{episode: later}
	consumerHeld.settleAdd(k, &view, &first, false, true, gen)
	consumerHeld.settleAdd(k, &view, &second, false, true, gen)
	claims, held := consumerHeld.claimRemove(k)
	require.True(t, held)
	require.Len(t, claims, 2, "both episodes hold the route")
	require.Same(t, earlier, claims[0].episode, "the earlier episode is withdrawn first")
	require.Same(t, later, claims[1].episode)
}

// TestRefusedReplacementWithdrawsAtRetiredEpisode proves a replacement the
// rules refuse also withdraws the old version an earlier episode holds, and
// sends the new episode nothing it never held.
// MUTATION: settling the refusal against h.set alone leaves the old version.
func TestRefusedReplacementWithdrawsAtRetiredEpisode(t *testing.T) {
	resetState(t)
	id := tagRules(t, 42)
	predecessor := registerBGPConsumer(t)
	bgpID, _ := redistevents.ProtocolIDOf("bgp")
	handleBatch(t.Context(), skipIDs(bgpID), taggedAddBatch(id, "10.0.0.0/8", 42))
	stopWatching := watchConsumers(newTestBus(), newReplayCoordinator())
	t.Cleanup(stopWatching)
	successor := &stubConsumer{name: "bgp"}
	require.True(t, configredist.ReregisterConsumer(successor))

	handleBatch(t.Context(), skipIDs(bgpID), taggedAddBatch(id, "10.0.0.0/8", 43))

	require.Len(t, predecessor.snapshotWithdrawn(), 1, "the refused replacement withdraws the old version where it is held")
	require.Empty(t, successor.snapshotWithdrawn(), "the successor never held the route")
}

// TestBeginRunEndsPreviousEpisodes proves a new orchestrator run holds nothing
// for the episodes the previous run dispatched to: they ended with it.
// MUTATION: keeping the held set across runs retries dead episodes forever.
func TestBeginRunEndsPreviousEpisodes(t *testing.T) {
	resetState(t)
	k := heldKey{consumer: "bgp", source: "episode-run-source", family: family.IPv4Unicast, prefix: netip.MustParsePrefix("192.0.2.0/24")}
	consumerHeld.mu.Lock()
	consumerHeld.retired[k] = []heldRoute{{state: heldUnconfirmed, sent: sentRoute{episode: &stubConsumer{name: "bgp"}}}}
	consumerHeld.set[k] = heldRoute{state: heldLive, sent: sentRoute{episode: &stubConsumer{name: "bgp"}}}
	consumerHeld.mu.Unlock()
	consumerHeld.beginRun()
	require.False(t, heldAnywhere(k), "a new run starts with no held record")
}

// heldAnywhere answers whether any episode still holds key's route.
func heldAnywhere(key heldKey) bool {
	consumerHeld.mu.Lock()
	defer consumerHeld.mu.Unlock()
	_, current := consumerHeld.set[key]
	return current || len(consumerHeld.retired[key]) > 0
}

// failingWithdrawalConsumer answers its first failures withdrawals with an
// error and leaves the route in place, then removes it as its embedded
// fixture does. attempts records every withdrawal it was asked for.
type failingWithdrawalConsumer struct {
	withdrawalEpisodeConsumer
	failures int
	attempts []configredist.RouteRemoval
}

func (consumer *failingWithdrawalConsumer) WithdrawRoute(ctx context.Context, fam family.Family, removal configredist.RouteRemoval) error {
	consumer.attempts = append(consumer.attempts, removal)
	if consumer.failures > 0 {
		consumer.failures--
		return errors.New("withdrawal not confirmed")
	}
	return consumer.withdrawalEpisodeConsumer.WithdrawRoute(ctx, fam, removal)
}

// This serialized dispatch fixture records actual consumer invocations. Its
// local bookkeeping supplies no provider settlement or native cleanup receipt.
type withdrawalEpisodeConsumer struct {
	live    map[uint32]configredist.RouteEntry
	removed []configredist.RouteRemoval
}

func (*withdrawalEpisodeConsumer) Name() string { return "bgp" }

func (consumer *withdrawalEpisodeConsumer) InjectRoute(_ context.Context, _ family.Family, entry configredist.RouteEntry) {
	consumer.live[entry.SourceInstance] = entry
}

func (consumer *withdrawalEpisodeConsumer) WithdrawRoute(_ context.Context, _ family.Family, removal configredist.RouteRemoval) error {
	consumer.removed = append(consumer.removed, removal)
	delete(consumer.live, removal.SourceInstance)
	return nil
}
