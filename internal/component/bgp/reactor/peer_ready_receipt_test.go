// Design: docs/architecture/api/architecture.md — captured peer-UP readiness
// Related: peer.go — creditAPIReady; forward_replay_fence_test.go — live queue
package reactor

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/peer"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestPeerReadyReceiptReleasesOnlyCapturedSession sends the real registered
// command into a real Peer after reconnect. Missing/old receipts cannot release
// the replacement's live work, even when the completed replay had no routes.
// The matched current receipt releases actual BGP output, not a mock callback.
// EOR is already complete throughout: readiness owns the replay fence, not EOR.
func TestPeerReadyReceiptReleasesOnlyCapturedSession(t *testing.T) {
	for _, populated := range []bool{false, true} {
		t.Run(fmt.Sprintf("populated=%t", populated), func(t *testing.T) {
			r, source, destination, _, ctxID := newSyncOrderRail(t)
			destination.resetAPISync([]string{replayFenceOwner})
			raiseTestReplayFence(destination, replayFenceOwner)
			oldToken := destination.initialReplayToken()
			require.NotZero(t, oldToken)

			// The same Peer now owns a new Session. An old zero-group replay
			// has no UPDATE writer through which a stale token could be caught.
			replacement, conn := newSyncOrderDest(t, destination.sendCtx.Load(), ctxID)
			destination.mu.Lock()
			destination.session = replacement.session
			destination.mu.Unlock()
			destination.sendingInitialRoutes.Store(0)
			destination.initialSyncEOROwed.Store(false)
			destination.resetAPISync([]string{replayFenceOwner})
			raiseTestReplayFence(destination, replayFenceOwner)
			captured := destination.initialReplayToken()
			require.NotZero(t, captured, "a replay fence survives the independently completed EOR")
			require.NotEqual(t, oldToken, captured)

			api := &reactorAPIAdapter{r: r}
			liveBody := syncOrderAnnounceBody
			if populated {
				liveBody = syncOrderWithdrawBody
			}
			live := syncOrderPublish(t, r, ctxID, 8700, liveBody)
			require.NoError(t, api.forwardUpdateCore(live, 8700, []*Peer{destination}, replayFenceSource(source, false)))
			if populated {
				replay := syncOrderPublish(t, r, ctxID, 8701, syncOrderAnnounceBody)
				require.NoError(t, api.forwardUpdateCore(replay, 8701, []*Peer{destination}, replayFenceSource(source, true)))
				require.Eventually(t, func() bool {
					return len(parseWireUpdates(t, conn.written())) == 1
				}, 5*time.Second, time.Millisecond, "replay output passes the live-forward fence")
			}

			srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, api)
			require.NoError(t, err)
			ctx := &pluginserver.CommandContext{Server: srv, Sender: plugin.ProcessSender(replayFenceOwner)}
			command := "request peer " + destination.Settings().Address.String() + " plugin session ready"
			for _, suffix := range []string{"", " session 0", fmt.Sprintf(" session %d", oldToken), " session invalid", " session 18446744073709551616"} {
				_, err := srv.Dispatcher().Dispatch(ctx, command+suffix)
				require.Error(t, err, "invalid receipt %q must not be acknowledged", suffix)
				require.False(t, apiSyncReleased(destination), "a refused command cannot credit the replacement")
				require.True(t, destination.forwardOrderHold(false), "replacement live work remains fenced")
			}

			// Operator readiness retains its explicit no-op semantics; it does
			// not acquire a named process's authority by omitting a token.
			ctx.Sender = plugin.OperatorSender()
			_, err = srv.Dispatcher().Dispatch(ctx, command)
			require.NoError(t, err)
			require.False(t, apiSyncReleased(destination))
			require.True(t, destination.forwardOrderHold(false))

			ctx.Sender = plugin.ProcessSender(replayFenceOwner)
			_, err = srv.Dispatcher().Dispatch(ctx, fmt.Sprintf("%s session %d", command, captured))
			require.NoError(t, err)
			require.True(t, apiSyncReleased(destination))
			require.False(t, destination.forwardOrderHold(false))
			want := []string{"announce"}
			if populated {
				want = []string{"announce", "withdraw"}
			}
			var seen []wireUpdate
			require.Eventually(t, func() bool {
				seen = parseWireUpdates(t, conn.written())
				return len(seen) >= len(want)
			}, 5*time.Second, time.Millisecond, "only the captured current receipt releases the queued live change")
			require.Equal(t, want, wireKinds(seen))
		})
	}
}

// TestPeerReadyReceiptRefusesRetiredSession covers a matching token whose writer
// was retired before replay completion. It cannot reopen live forwarding even
// though this Session is still installed while teardown finishes.
func TestPeerReadyReceiptRefusesRetiredSession(t *testing.T) {
	r, source, destination, conn, ctxID := newSyncOrderRail(t)
	destination.sendingInitialRoutes.Store(0)
	destination.resetAPISync([]string{replayFenceOwner})
	raiseTestReplayFence(destination, replayFenceOwner)
	captured := destination.initialReplayToken()
	destination.session.tearingDown.Store(true)
	api := &reactorAPIAdapter{r: r}
	live := syncOrderPublish(t, r, ctxID, 8800, syncOrderAnnounceBody)
	require.NoError(t, api.forwardUpdateCore(live, 8800, []*Peer{destination}, replayFenceSource(source, false)))
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, api)
	require.NoError(t, err)
	ctx := &pluginserver.CommandContext{Server: srv, Sender: plugin.ProcessSender(replayFenceOwner)}
	_, err = srv.Dispatcher().Dispatch(ctx, fmt.Sprintf("request peer %s plugin session ready session %d", destination.Settings().Address, captured))
	require.Error(t, err)
	require.False(t, apiSyncReleased(destination))
	require.True(t, destination.forwardOrderHold(false))
	require.Empty(t, conn.written(), "retired-session readiness must not authorize wire output")
}
