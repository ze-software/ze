// Design: docs/architecture/config/transaction-protocol.md -- outer reload ownership
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net"
	"net/netip"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// A later accepted reload owns the running tree even when it was a no-op.
// Rejecting an older outer scope must not replace that accepted configuration.
func TestRejectedReloadScopePreservesNewerAcceptedTree(t *testing.T) {
	for _, next := range []int{2, 3} {
		t.Run(fmt.Sprintf("accepted=%d", next), func(t *testing.T) {
			s, _ := newLifecycleStartupServer(t)
			reactor := &removalRecoveryReactor{tree: map[string]any{"revision": 1}}
			s.reactor = reactor
			ctx, finish := s.DeferReloadAcceptance(t.Context())
			defer finish(false)
			require.NoError(t, s.ReloadConfig(ctx, map[string]any{"revision": 2}))
			accepted := map[string]any{"revision": next}
			require.NoError(t, s.ReloadConfig(t.Context(), accepted))
			finish(false)
			require.Equal(t, accepted, reactor.GetConfigTree())
			require.Equal(t, accepted, reactor.applyTree)
		})
	}
}

func TestRejectedReloadScopesRestorePredecessorOwnership(t *testing.T) {
	for _, parentFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("parent-first=%t", parentFirst), func(t *testing.T) {
			s, _ := newLifecycleStartupServer(t)
			initial := map[string]any{"revision": 1}
			parentTree := map[string]any{"revision": 2}
			childTree := map[string]any{"revision": 3}
			reactor := &removalRecoveryReactor{tree: initial}
			s.reactor = reactor
			parent, rejectParent := s.DeferReloadAcceptance(t.Context())
			defer rejectParent(false)
			require.NoError(t, s.ReloadConfig(parent, parentTree))
			child, rejectChild := s.DeferReloadAcceptance(t.Context())
			defer rejectChild(false)
			require.NoError(t, s.ReloadConfig(child, childTree))
			if parentFirst {
				rejectParent(false)
				require.Equal(t, childTree, reactor.GetConfigTree())
				rejectChild(false)
			} else {
				rejectChild(false)
				require.Equal(t, parentTree, reactor.GetConfigTree())
				rejectParent(false)
			}
			require.Equal(t, initial, reactor.GetConfigTree())
			require.Equal(t, initial, reactor.applyTree)
		})
	}
}

type refusingCompensationReactor struct {
	removalRecoveryReactor
	refuse func(map[string]any) bool
}

func (r *refusingCompensationReactor) ApplyConfigDiff(tree map[string]any) error {
	if r.refuse(tree) {
		return errors.New("reactor refused compensation")
	}
	return r.removalRecoveryReactor.ApplyConfigDiff(tree)
}

// A failed inverse must remain actionable even when the next request matches
// the still-published tree. Completed participant work must not be replayed
// when only the reactor's restoration failed.
func TestFailedReloadCompensationRetainsRetry(t *testing.T) {
	for _, reactorFailure := range []bool{false, true} {
		t.Run(fmt.Sprintf("reactor-failure=%t", reactorFailure), func(t *testing.T) {
			snapshot := registry.Snapshot()
			registry.Reset()
			t.Cleanup(func() { registry.Restore(snapshot) })
			const name, root = "compensation-counter", "compensation-counter"
			var refuse atomic.Bool
			var active atomic.Int64
			runDone := make(chan error, 1)
			require.NoError(t, registry.Register(registry.Registration{
				Name: name, Description: "fallible compensation participant",
				CLIHandler: func([]string) int { return 0 },
				RunEngine: func(conn net.Conn) int {
					p := sdk.NewWithConn(name, conn)
					var staged int64
					stage := func(sections []sdk.ConfigSection) error {
						for _, section := range sections {
							var decoded map[string]struct{ Value int64 }
							if err := json.Unmarshal([]byte(section.Data), &decoded); err != nil {
								return err
							}
							staged = decoded[root].Value
						}
						return nil
					}
					p.OnConfigure(func(sections []sdk.ConfigSection) error {
						if err := stage(sections); err != nil {
							return err
						}
						active.Store(staged)
						return nil
					})
					p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
						if err := stage(sections); err != nil {
							return err
						}
						if !reactorFailure && refuse.Load() && staged == 42 {
							return errors.New("participant refused compensation")
						}
						return nil
					})
					p.OnConfigApply(func([]sdk.ConfigDiffSection) error {
						if active.Load() == staged {
							return errors.New("resource revision already installed")
						}
						active.Store(staged)
						return nil
					})
					err := p.Run(context.Background(), sdk.Registration{WantsConfig: []string{root}})
					if closeErr := p.Close(); err == nil {
						err = closeErr
					}
					runDone <- err
					return 0
				},
			}))
			s, spawner := newLifecycleStartupServer(t)
			committed := map[string]any{root: map[string]any{"value": int64(42)}}
			candidate := map[string]any{root: map[string]any{"value": int64(99)}}
			reactor := &refusingCompensationReactor{
				tree: committed,
				refuse: func(tree map[string]any) bool {
					if !reactorFailure || !refuse.Load() {
						return false
					}
					container, ok := tree[root].(map[string]any)
					if !ok {
						t.Errorf("candidate %s is %T, want a map", root, tree[root])
						return false
					}
					return container["value"] == int64(42)
				},
			}
			s.reactor = reactor
			require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: plugin.EncodingJSON}}))
			ctx, finish := s.DeferReloadAcceptance(t.Context())
			defer finish(false)
			require.NoError(t, s.ReloadConfig(ctx, candidate))
			require.Equal(t, int64(99), active.Load())
			refuse.Store(true)
			finish(false)
			require.Equal(t, candidate, reactor.GetConfigTree(), "failed compensation must not claim to publish the prior tree")
			wantActive := int64(99)
			if reactorFailure {
				wantActive = 42
			}
			require.Equal(t, wantActive, active.Load())
			require.Error(t, s.ReloadConfig(t.Context(), candidate), "a no-diff request must not discard failed compensation")
			require.Equal(t, wantActive, active.Load())
			refuse.Store(false)
			require.NoError(t, s.ReloadConfig(t.Context(), committed))
			require.Equal(t, int64(42), active.Load())
			require.Equal(t, committed, reactor.GetConfigTree())
			require.NoError(t, s.rollbackStartupProcess(spawner.pm.GetProcess(name)))
			require.NoError(t, <-runDone)
		})
	}
}

// createdPeersReactor is a reactor holding one peer created at runtime, which
// every applied tree takes over, the way a candidate declaring it does.
type createdPeersReactor struct {
	removalRecoveryReactor
	created map[netip.Addr]string
}

func (r *createdPeersReactor) CreatedPeers() map[netip.Addr]string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return maps.Clone(r.created)
}

func (r *createdPeersReactor) SetConfigTree(tree map[string]any) {
	r.removalRecoveryReactor.SetConfigTree(tree)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.created = nil
}

func (r *createdPeersReactor) RestoreCreatedPeers(created map[netip.Addr]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.created = maps.Clone(created)
}

// TestRejectedReloadMarksTheCreatedPeersAgain holds AC-9 of
// spec-yang-rpc-declarations-with-no-handler across a rejected reload.
//
// GOAL: a reload the hub rejects after the reactor applied it gives back the
// created peers it took over, so the next reload whose candidate does not
// declare them keeps them.
// METHOD: a reactor with one created peer that the applied tree takes over;
// reload under a deferred acceptance, then reject it.
// VALIDATES: the reload took the peer over, and the compensation handed the
// marks the reactor held before the reload back to RestoreCreatedPeers.
// PREVENTS: a created peer left configured by a commit that never took effect.
func TestRejectedReloadMarksTheCreatedPeersAgain(t *testing.T) {
	s, _ := newLifecycleStartupServer(t)
	created := map[netip.Addr]string{netip.MustParseAddr("192.0.2.7"): "peer-192.0.2.7"}
	reactor := &createdPeersReactor{created: maps.Clone(created)}
	reactor.tree = map[string]any{"revision": 1}
	s.reactor = reactor

	ctx, finish := s.DeferReloadAcceptance(t.Context())
	defer finish(false)
	require.NoError(t, s.ReloadConfig(ctx, map[string]any{"revision": 2}))
	require.Empty(t, reactor.CreatedPeers(), "the applied tree took the created peer over")

	finish(false)
	require.Equal(t, map[string]any{"revision": 1}, reactor.GetConfigTree())
	require.Equal(t, created, reactor.CreatedPeers(), "the rejected reload marks the created peer again")
}

// lockOrderReactor reports a CreatedPeers call made while the reload scope's
// own lock is held.
type lockOrderReactor struct {
	createdPeersReactor
	t       *testing.T
	pending *reloadAcceptance
	calls   atomic.Int64
}

func (r *lockOrderReactor) CreatedPeers() map[netip.Addr]string {
	r.calls.Add(1)
	if r.pending != nil {
		if r.pending.mu.TryLock() {
			r.pending.mu.Unlock()
		} else {
			r.t.Error("CreatedPeers called with the reload scope's lock held")
		}
	}
	return r.createdPeersReactor.CreatedPeers()
}

// TestReloadCompensationReadsCreatedPeersOutsideTheScopeLock holds the lock
// order of recordReloadCompensation.
//
// GOAL: the reload scope's lock is never held while the reactor's lock is
// taken, so no path can order the two the other way and deadlock.
// METHOD: a reactor whose CreatedPeers tries the reload scope's lock; reload
// under a deferred acceptance, which records the compensation.
// VALIDATES: the compensation read the created peers, and the scope's lock was
// free when it did.
// PREVENTS: a pending.mu -> reactor lock order that a later reactor path
// taking the scope lock under its own would turn into a deadlock.
func TestReloadCompensationReadsCreatedPeersOutsideTheScopeLock(t *testing.T) {
	s, _ := newLifecycleStartupServer(t)
	reactor := &lockOrderReactor{t: t}
	reactor.created = map[netip.Addr]string{netip.MustParseAddr("192.0.2.7"): "peer-192.0.2.7"}
	reactor.tree = map[string]any{"revision": 1}
	s.reactor = reactor

	ctx, finish := s.DeferReloadAcceptance(t.Context())
	defer finish(false)
	pending, ok := ctx.Value(reloadAcceptanceKey{}).(*reloadAcceptance)
	require.True(t, ok, "the deferred acceptance carries its reload scope")
	reactor.pending = pending
	require.NoError(t, s.ReloadConfig(ctx, map[string]any{"revision": 2}))
	require.NotZero(t, reactor.calls.Load(), "the compensation read the created peers")
}
