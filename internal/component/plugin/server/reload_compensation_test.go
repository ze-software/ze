// Design: docs/architecture/config/transaction-protocol.md -- outer reload ownership
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
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
			reactor := &removalRecoveryReactor{mockReloadReactor: mockReloadReactor{tree: map[string]any{"revision": 1}}}
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
			reactor := &removalRecoveryReactor{mockReloadReactor: mockReloadReactor{tree: initial}}
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
				removalRecoveryReactor: removalRecoveryReactor{mockReloadReactor: mockReloadReactor{tree: committed}},
				refuse: func(tree map[string]any) bool {
					return reactorFailure && refuse.Load() && tree[root].(map[string]any)["value"] == int64(42)
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
