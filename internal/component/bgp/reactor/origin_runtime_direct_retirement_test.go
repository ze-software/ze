// Design: docs/architecture/update-building.md -- direct registry retirement versus selected ownership.
// Related: origin_runtime_api_test.go -- actual SDK provider and final recording Session.
package reactor

import (
	"bytes"
	"context"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgpannounce "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/announce"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// directRegistryExpiry holds the actual Registry timer callback before its API
// withdrawal. The entry and descriptor were accepted while the live SDK provider
// explicitly lacked intake. No ordinary admission is synthesized for that entry.
type directRegistryExpiry struct {
	resume    chan struct{}
	completed chan error
	release   sync.Once
}

func beginDirectRegistryExpiry(t *testing.T, f *originRuntimeSDKFixture, batch bgptypes.NLRIBatch) *directRegistryExpiry {
	t.Helper()
	return beginDirectRegistryExpiryWithWithdrawal(t, f, batch, f.writer.api.WithdrawNLRIBatch)
}

func beginDirectRegistryExpiryWithWithdrawal(t *testing.T, f *originRuntimeSDKFixture, batch bgptypes.NLRIBatch, withdraw func(context.Context, *selector.Selector, bgptypes.NLRIBatch, plugin.Sender) error) *directRegistryExpiry {
	t.Helper()
	result, err := f.writer.api.AnnounceNLRIBatchResult(f.ctx, selector.ParseDefault(f.writer.peer.addrString), batch, f.sender)
	require.NoError(t, err)
	require.Equal(t, uint32(1), result.Announced)
	require.Empty(t, result.Admissions, "a live explicitly legacy provider must use the actual direct rail")
	retirement, err := plugin.CaptureOriginRetirement(f.sender, plugin.SelectedOwner{}, result)
	require.NoError(t, err)
	batch.OriginRetirement = retirement
	expiry := &directRegistryExpiry{resume: make(chan struct{}), completed: make(chan error, 1)}
	entered, joined := make(chan struct{}), make(chan struct{})
	registry := bgpannounce.NewRegistry(func(sel *selector.Selector, original bgptypes.NLRIBatch, sender plugin.Sender) error {
		defer close(joined)
		close(entered)
		select {
		case <-expiry.resume:
		case <-f.ctx.Done():
		}
		err := withdraw(plugin.WithOriginRetirement(f.ctx, original.OriginRetirement), sel, original, sender)
		expiry.completed <- err
		return err
	})
	t.Cleanup(func() {
		expiry.release.Do(func() { close(expiry.resume) })
		select {
		case <-joined:
		case <-time.After(5 * time.Second):
			t.Error("actual registry timer callback did not join")
		}
	})
	_, err = registry.Announce("direct", "accepted", selector.ParseDefault(f.writer.peer.addrString), batch, "registry-test", f.sender, time.Nanosecond)
	require.NoError(t, err)
	select {
	case <-entered:
	case <-f.ctx.Done():
		t.Fatal("actual registry timer did not reach its withdrawal")
	}
	require.Zero(t, registry.Len(), "the actual expiry removed its accepted registry entry")
	return expiry
}

func (expiry *directRegistryExpiry) finish(t *testing.T, ctx context.Context) {
	t.Helper()
	expiry.release.Do(func() { close(expiry.resume) })
	select {
	case err := <-expiry.completed:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("registry withdrawal did not settle")
	}
}

// holdDirectRegistryQueue uses a real counted channel item to hold the forward
// worker before its socket lock. A's real registry withdrawal is queued behind
// it while A still owns the wire. B then reaches the selected final writer, so
// a dispatch-time check alone cannot protect B when that old queued item drains.
func holdDirectRegistryQueue(t *testing.T, f *originRuntimeSDKFixture) func() {
	t.Helper()
	entered, resume := make(chan struct{}), make(chan struct{})
	var first, release sync.Once
	pool := newFwdPool(func(key fwdKey, items []fwdItem) {
		first.Do(func() {
			close(entered)
			select {
			case <-resume:
			case <-f.ctx.Done():
			}
		})
		fwdBatchHandler(key, items)
	}, fwdPoolConfig{chanSize: 8, idleTimeout: time.Second})
	t.Cleanup(pool.Stop)
	t.Cleanup(func() { release.Do(func() { close(resume) }) })
	f.writer.api.r.fwdPool = pool
	key := fwdKey{peerAddr: f.writer.peer.Settings().PeerKey()}
	pool.registerOutgoingPool(key, 4096)
	require.True(t, pool.TryDispatch(key, &fwdItem{peer: f.writer.peer, session: f.writer.peer.currentSession()}))
	select {
	case <-entered:
	case <-f.ctx.Done():
		t.Fatal("real forwarding worker did not reach its test barrier")
	}
	require.True(t, f.writer.peer.withdrawBehindForwards(), "the real counted item must require ordered withdrawal")
	return func() {
		release.Do(func() { close(resume) })
		require.NoError(t, pool.Barrier(f.ctx))
	}
}

// MUTATION: removing the final selected-owner guard lets stale direct A withdraw selected B.
func TestOriginRuntimeDirectRegistryCannotWithdrawSelectedLocal(t *testing.T) {
	for _, typed := range []bool{false, true} {
		t.Run(map[bool]string{false: "framed", true: "typed"}[typed], func(t *testing.T) {
			for _, rail := range []string{"synchronous", "queued-before-selection", "own-route", "prefiltered", "policy-child"} {
				t.Run(rail, func(t *testing.T) {
					queued := rail == "queued-before-selection"
					var intake atomic.Bool
					f := newOriginRuntimeSDKFixtureWithIntake(t, typed, intake.Load)
					oldProvider := f.host.ProcessManager().GetProcess(f.name)
					require.NotNil(t, oldProvider)
					oldOwner, supported, err := f.host.OriginIntakeOwner(f.name)
					require.NoError(t, err)
					require.False(t, supported)
					require.NoError(t, oldOwner.Current(), "the original provider must be live, not an unavailable fallback")
					withdraw := f.writer.api.WithdrawNLRIBatch
					if rail == "own-route" || rail == "prefiltered" || rail == "policy-child" {
						// These are the real direct maintenance senders after a
						// policy decision. Their restrictions must also survive
						// prefiltered output and its reconstructed removal child.
						withdraw = func(ctx context.Context, _ *selector.Selector, batch bgptypes.NLRIBatch, _ plugin.Sender) error {
							target := announceTarget{peer: f.writer.peer, session: f.writer.peer.currentSession()}
							update := &message.Update{WithdrawnRoutes: batch.NLRIs[0].Bytes()}
							if rail == "own-route" {
								_, err := target.sendOwnRouteWithdrawal(ctx, update, 4096, false, &batch)
								return err
							}
							var removals [][]byte
							if rail == "policy-child" {
								removals = [][]byte{makeUpdateBody(batch.NLRIs[0].Bytes(), nil, nil)}
							}
							return target.sendBatchUpdatePreFiltered(ctx, update, nil, 4096, false, &batch, removals)
						}
					}
					stale := beginDirectRegistryExpiryWithWithdrawal(t, f, f.batch, withdraw)
					sibling := f.batch
					sibling.NLRIs = []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("198.18.9.0/24"), 0, false)}
					useful := beginDirectRegistryExpiry(t, f, sibling)
					initial := updateBodies(t, f.writer.conn.written())
					require.Len(t, initial, 2)
					require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, initial[0], false))
					require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.9.0/24")}, updatePrefixes(t, initial[1], false))
					originRuntimeExpectCounts(t, f.writer.api.r.originReceipts, 0, 0, 0, 0)
					intake.Store(true)
					response, err := f.host.Dispatcher().Dispatch(&pluginserver.CommandContext{Server: f.host}, "request plugin restart "+f.name)
					require.NoError(t, err)
					require.Equal(t, plugin.StatusDone, response.Status, response)
					require.NotSame(t, oldProvider, f.host.ProcessManager().GetProcess(f.name))
					require.Error(t, oldOwner.Current())
					provider, supported, err := f.host.OriginIntakeOwner(f.name)
					require.NoError(t, err)
					require.True(t, supported, "the replacement must complete the real optional SDK handshake")
					require.NoError(t, provider.Current())
					require.Len(t, updateBodies(t, f.writer.conn.written()), 2, "restart must leave direct output useful")
					drain := func() {}
					if queued {
						drain = holdDirectRegistryQueue(t, f)
						stale.finish(t, f.ctx)
						require.Len(t, updateBodies(t, f.writer.conn.written()), 2, "retirement must still be waiting behind the counted item")
					}
					selected := f.batch
					selected.NextHop = bgptypes.NewNextHopExplicit(netip.MustParseAddr("192.0.2.253"))
					result, _ := originRegistryAnnounce(t, f, selected, plugin.SelectedOwner{})
					require.Equal(t, rpc.RouteAdmissionSelected, result.Admissions[0].Disposition)
					store := f.writer.api.r.originReceipts
					store.mu.Lock()
					original := store.receipts[result.Admissions[0].Generation]
					store.mu.Unlock()
					require.NotNil(t, original)
					require.Equal(t, []byte{24, 198, 18, 7}, original.build.native)
					before := bytes.Clone(f.writer.conn.written())
					bodies := updateBodies(t, before)
					require.Len(t, bodies, 3)
					hop, present := bodyPathAttr(t, bodies[2], 3)
					require.True(t, present)
					require.Equal(t, []byte{192, 0, 2, 253}, hop, "B must actually replace A with its own original build")
					table := &f.writer.peer.adjOut
					table.mu.Lock()
					selectedCount := len(table.selectedPaths)
					table.mu.Unlock()
					require.Equal(t, 1, selectedCount)
					if !queued {
						stale.finish(t, f.ctx)
					}
					drain()
					require.Equal(t, before, f.writer.conn.written(), "old A must never withdraw B, even transiently before replay")
					require.Same(t, original, originRuntimeCurrent(t, store, original.key))
					require.NoError(t, original.ctx.Err())
					require.NoError(t, provider.Current())
					originRuntimeExpectCounts(t, store, 1, 1, 0, 0)
					table.mu.Lock()
					selectedCount, routeCount := len(table.selectedPaths), len(table.routes)
					table.mu.Unlock()
					require.Equal(t, 1, selectedCount)
					require.Equal(t, 2, routeCount)
					useful.finish(t, f.ctx)
					require.NoError(t, f.writer.api.r.fwdPool.Barrier(f.ctx))
					bodies = updateBodies(t, f.writer.conn.written())
					require.Len(t, bodies, 4, "genuine still-current direct C must really withdraw")
					require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.9.0/24")}, updatePrefixes(t, bodies[3], true))
					require.Empty(t, updatePrefixes(t, bodies[3], false))
					require.Same(t, original, originRuntimeCurrent(t, store, original.key))
					require.NoError(t, f.writer.api.WithdrawNLRIBatch(f.ctx, selector.ParseDefault(f.writer.peer.addrString), selected, f.sender), "explicit current manual withdrawal retains its ordinary semantics")
					bodies = updateBodies(t, f.writer.conn.written())
					require.Len(t, bodies, 5)
					require.Equal(t, []netip.Prefix{netip.MustParsePrefix("198.18.7.0/24")}, updatePrefixes(t, bodies[4], true))
					originRuntimeExpectCounts(t, store, 0, 0, 0, 0)
					table.mu.Lock()
					selectedCount, routeCount = len(table.selectedPaths), len(table.routes)
					table.mu.Unlock()
					require.Zero(t, selectedCount)
					require.Zero(t, routeCount)
				})
			}
		})
	}
}
