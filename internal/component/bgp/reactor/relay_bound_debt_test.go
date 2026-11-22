// Design: docs/architecture/api/process-protocol.md -- captured forwarding completion.
// Related: relay_bound.go -- bounded target admission and final writer joins.
package reactor

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestBoundForwardWorkerDebtSettlesAfterJoin observes actual stored/cached
// output and pending ownership. A full bounded queue refuses before admission,
// leaves its original two controls owed, then admits useful output after drain.
// MUTATION: Counting a bound carrier outside TryDispatch leaves extra debt on
// both successful channel admission and full-channel refusal.
func TestBoundForwardWorkerDebtSettlesAfterJoin(t *testing.T) {
	for _, cached := range []bool{false, true} {
		name := "stored"
		if cached {
			name = "cached"
		}
		t.Run(name, func(t *testing.T) {
			for _, full := range []bool{false, true} {
				admission := "admitted"
				if full {
					admission = "full-channel-refused"
				}
				t.Run(admission, func(t *testing.T) {
					f := newBoundRelayWireFixture(t, false)
					ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
					defer cancel()
					var send func() error
					if cached {
						source, err := f.target.api.originalRelayFacts(f.source.settings.Address, &f.route, f.sender)
						require.NoError(t, err)
						var spans []relayAttrSpan
						_, id, _, err := f.target.api.buildRelayUpdate([]rpc.StoredRoute{f.route}, &source, &spans)
						require.NoError(t, err)
						defer f.target.api.r.recentUpdates.Release(id)
						targets := []rpc.ForwardTarget{{Destination: f.target.peer.settings.PeerKey().String(),
							DestinationOwner: f.target.peer.sourceOwner, DestinationSession: f.target.peer.session.initialReplay.Load()}}
						send = func() error {
							return f.target.api.ForwardUpdatesBound(ctx, []uint64{id}, targets, "", f.target.owner.Sender(), f.target.owner)
						}
					} else {
						send = func() error {
							return f.target.api.RelayStoredRouteBound(ctx, f.target.peer.settings.Address, []rpc.StoredRoute{f.route}, f.sender, f.target.owner)
						}
					}
					require.Zero(t, f.target.peer.fwdChannelPending.Load())
					if full {
						// Replace only this fixture's idle worker, before any work is admitted.
						f.target.api.r.fwdPool.Stop()
						entered, release := make(chan struct{}), make(chan struct{})
						firstDone, secondDone := make(chan struct{}), make(chan struct{})
						var first, released sync.Once
						pool := newFwdPool(func(fwdKey, []fwdItem) {
							first.Do(func() {
								close(entered)
								<-release
							})
						}, fwdPoolConfig{chanSize: 1, batchLimit: 1, idleTimeout: time.Minute})
						t.Cleanup(pool.Stop)
						unblock := func() { released.Do(func() { close(release) }) }
						t.Cleanup(unblock)
						f.target.api.r.fwdPool = pool
						key := fwdKey{peerAddr: f.target.peer.settings.PeerKey()}
						pool.registerOutgoingPool(key, 4096)
						one := fwdItem{peer: f.target.peer, session: f.target.peer.session, done: func() { close(firstDone) }}
						require.True(t, pool.TryDispatch(key, &one))
						select {
						case <-entered:
						case <-ctx.Done():
							t.Fatal("the first real worker batch did not reach its hold")
						}
						two := fwdItem{peer: f.target.peer, session: f.target.peer.session, done: func() { close(secondDone) }}
						require.True(t, pool.TryDispatch(key, &two))
						require.Equal(t, int64(2), f.target.peer.fwdChannelPending.Load(), "one held control and one queued control")
						require.ErrorIs(t, send(), errBoundForwardUnsettled)
						require.Empty(t, f.target.conn.written(), "a refused bound carrier cannot claim useful output")
						require.Empty(t, f.target.peer.adjOut.routes)
						require.Equal(t, int64(2), f.target.peer.fwdChannelPending.Load(), "refusal cannot invent a third admitted item")
						unblock()
						for _, done := range []<-chan struct{}{firstDone, secondDone} {
							select {
							case <-done:
							case <-ctx.Done():
								t.Fatal("the admitted controls did not settle after release")
							}
						}
						require.Zero(t, f.target.peer.fwdChannelPending.Load())
					}
					require.NoError(t, send(), "the actual original source and captured destination remain useful")
					require.Len(t, updateBodies(t, f.target.conn.written()), 1, "return follows the actual socket flush")
					require.Len(t, f.target.peer.adjOut.routes, 1)
					for _, entry := range f.target.peer.adjOut.routes {
						require.Same(t, f.source, entry.owner.source)
						require.Equal(t, f.session.connectionID, entry.owner.connectionID)
					}
					require.Zero(t, f.target.peer.fwdChannelPending.Load(), "every admitted channel count was returned before join")
					require.False(t, f.target.peer.forwardChannelPending())
					require.False(t, f.target.peer.forwardOverflowPending())
				})
			}
		})
	}
}
