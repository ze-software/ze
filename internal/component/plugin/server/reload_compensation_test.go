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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/transaction"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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
			defer finish(false) //nolint:errcheck // Cleanup; the completion outcome is asserted below.
			require.NoError(t, s.ReloadConfig(ctx, map[string]any{"revision": 2}))
			accepted := map[string]any{"revision": next}
			require.NoError(t, s.ReloadConfig(t.Context(), accepted))
			require.NoError(t, finish(false))
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
			defer rejectParent(false) //nolint:errcheck // Cleanup; completion outcomes are asserted below.
			require.NoError(t, s.ReloadConfig(parent, parentTree))
			child, rejectChild := s.DeferReloadAcceptance(t.Context())
			defer rejectChild(false) //nolint:errcheck // Cleanup; completion outcomes are asserted below.
			require.NoError(t, s.ReloadConfig(child, childTree))
			if parentFirst {
				require.NoError(t, rejectParent(false))
				require.Equal(t, childTree, reactor.GetConfigTree())
				require.NoError(t, rejectChild(false))
			} else {
				require.NoError(t, rejectChild(false))
				require.Equal(t, parentTree, reactor.GetConfigTree())
				require.NoError(t, rejectParent(false))
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
					err := p.Run(context.Background(), &sdk.Registration{WantsConfig: []string{root}})
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
			defer finish(false) //nolint:errcheck // Replays the deliberately failed completion outcome asserted below.
			require.NoError(t, s.ReloadConfig(ctx, candidate))
			require.Equal(t, int64(99), active.Load())
			refuse.Store(true)
			require.Error(t, finish(false))
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
	created      map[netip.Addr]string
	restoredFrom map[string]any
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

func (r *createdPeersReactor) RestoreCreatedPeers(before map[string]any, created map[netip.Addr]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.created = maps.Clone(created)
	r.restoredFrom = before
	return nil
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
// marks the reactor held before the reload back to RestoreCreatedPeers, with
// the tree it restores to rebuild them from.
// PREVENTS: a created peer left configured by a commit that never took effect.
func TestRejectedReloadMarksTheCreatedPeersAgain(t *testing.T) {
	s, _ := newLifecycleStartupServer(t)
	created := map[netip.Addr]string{netip.MustParseAddr("192.0.2.7"): "peer-192.0.2.7"}
	reactor := &createdPeersReactor{created: maps.Clone(created)}
	reactor.tree = map[string]any{"revision": 1}
	s.reactor = reactor

	ctx, finish := s.DeferReloadAcceptance(t.Context())
	defer finish(false) //nolint:errcheck // Cleanup; the completion outcome is asserted below.
	require.NoError(t, s.ReloadConfig(ctx, map[string]any{"revision": 2}))
	require.Empty(t, reactor.CreatedPeers(), "the applied tree took the created peer over")

	require.NoError(t, finish(false))
	require.Equal(t, map[string]any{"revision": 1}, reactor.GetConfigTree())
	require.Equal(t, created, reactor.CreatedPeers(), "the rejected reload marks the created peer again")
	reactor.mu.Lock()
	restoredFrom := reactor.restoredFrom
	reactor.mu.Unlock()
	require.Equal(t, map[string]any{"revision": 1}, restoredFrom,
		"the created peers are restored from the tree the compensation restores")
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
	defer func() { require.NoError(t, finish(false)) }()
	pending, ok := ctx.Value(reloadAcceptanceKey{}).(*reloadAcceptance)
	require.True(t, ok, "the deferred acceptance carries its reload scope")
	reactor.pending = pending
	require.NoError(t, s.ReloadConfig(ctx, map[string]any{"revision": 2}))
	require.NotZero(t, reactor.calls.Load(), "the compensation read the created peers")
}

// completionOwner exercises real SDK callbacks over both negotiated transports.
// Its journal includes a runtime value deliberately absent from configuration:
// reconstructing the old tree cannot restore that value.
type completionOwner struct {
	value        atomic.Int64
	commits      atomic.Int64
	completions  atomic.Int64
	failCommit   atomic.Bool
	failComplete atomic.Bool
}

func startCompletionOwner(t *testing.T, s *Server, name string, wire bool) *completionOwner {
	t.Helper()
	owner := &completionOwner{}
	owner.value.Store(73)
	require.NoError(t, registry.Register(registry.Registration{
		Name: name, Description: "retained operation journal owner",
		CLIHandler: func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			if wire {
				conn = struct{ net.Conn }{conn} // Hide Bridger, exercising the external RPC path.
			}
			p := sdk.NewWithConn(name, conn)
			journals := make(map[string]*sdk.Journal)
			p.OnConfigure(func([]sdk.ConfigSection) error { return nil })
			p.OnConfigVerify(func([]sdk.ConfigSection) error { return nil })
			p.OnConfigApply(func([]sdk.ConfigDiffSection) error {
				return errors.New("completion fixture requires declared operations")
			})
			p.OnConfigOperationDecompose(func(input sdk.ConfigOperationDecomposeInput) (*sdk.ConfigOperationDecomposeOutput, error) {
				return &sdk.ConfigOperationDecomposeOutput{Operations: []sdk.ConfigOperation{{
					ID: name + "-change", Root: name, Owner: name, Type: testOpSetProperty,
					Verb: transaction.VerbModify, Target: transaction.ResourceRef{Kind: testResourceSysctl, Name: name},
				}}}, nil
			})
			p.OnConfigOperationVerify(func(sdk.ConfigOperationVerifyInput) error { return nil })
			p.OnConfigOperationApply(func(input sdk.ConfigOperationApplyInput) (*sdk.ConfigOperationApplyOutput, error) {
				before := owner.value.Load()
				j := sdk.NewJournal()
				if err := j.Record(func() error { owner.value.Store(99); return nil }, func() error {
					owner.value.Store(before)
					return nil
				}); err != nil {
					return nil, err
				}
				journals[input.TransactionID] = j
				return &sdk.ConfigOperationApplyOutput{Status: rpc.StatusOK}, nil
			})
			p.OnConfigOperationRollback(func(input sdk.ConfigOperationRollbackInput) error {
				if input.DeadlineMS <= time.Now().UnixMilli() {
					return errors.New("undo received expired apply deadline")
				}
				j := journals[input.TransactionID]
				if j == nil {
					return errors.New("original journal is missing")
				}
				delete(journals, input.TransactionID)
				return errors.Join(j.Rollback()...)
			})
			p.OnConfigOperationCommit(func(sdk.ConfigOperationCommitInput) error {
				owner.commits.Add(1)
				if owner.failCommit.Load() {
					return errors.New("injected later commit failure")
				}
				return nil
			})
			p.OnConfigComplete(func(ctx context.Context, input sdk.ConfigCompleteInput) error {
				owner.completions.Add(1)
				if !input.Accepted && journals[input.TransactionID] != nil {
					return errors.New("abort completion preceded original undo")
				}
				if owner.failComplete.Load() {
					return errors.New("injected cleanup failure")
				}
				if j := journals[input.TransactionID]; j != nil {
					j.Discard()
					delete(journals, input.TransactionID)
				}
				return ctx.Err()
			})
			defer func() { _ = p.Close() }() // Teardown: Run's result is what the fixture reports.
			if err := p.Run(s.Context(), &sdk.Registration{
				WantsConfig:      []string{name},
				ConfigOperations: []sdk.ConfigOperationDecl{{Root: name, Decompose: true, Operations: []sdk.ConfigOperationType{testOpSetProperty}}},
			}); err != nil {
				return 1
			}
			return 0
		},
	}))
	require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: plugin.EncodingJSON}}))
	return owner
}

func TestReloadOriginalJournalAcrossAcceptance(t *testing.T) {
	for _, wire := range []bool{false, true} {
		for _, fault := range []string{"commit", "publication", "finalization", "duplicate", "replacement"} {
			t.Run(fmt.Sprintf("wire=%t/%s", wire, fault), func(t *testing.T) {
				snapshot := registry.Snapshot()
				registry.Reset()
				t.Cleanup(func() { registry.Restore(snapshot) })
				const first, second = "completion-first", "completion-second"
				oldTree := map[string]any{first: map[string]any{"value": "old"}, second: map[string]any{"value": "old"}}
				candidate := map[string]any{first: map[string]any{"value": "new"}, second: map[string]any{"value": "new"}}
				s, spawner := newLifecycleStartupServer(t)
				r := &removalRecoveryReactor{tree: oldTree}
				s.reactor = r
				a := startCompletionOwner(t, s, first, wire)
				b := startCompletionOwner(t, s, second, wire)
				a.failComplete.Store(fault == "finalization")
				b.failCommit.Store(fault == "commit")
				ctx, finish := s.DeferReloadAcceptance(t.Context())
				err := s.ReloadConfig(ctx, candidate)
				if fault == "commit" {
					require.ErrorContains(t, err, "injected later commit failure")
					require.Equal(t, int64(73), a.value.Load(), "earlier committed owner lost its runtime undo")
					require.Equal(t, int64(73), b.value.Load())
					require.NoError(t, finish(false))
					return
				}
				require.NoError(t, err)
				require.Equal(t, int64(99), a.value.Load())
				require.Zero(t, a.completions.Load(), "inner commit must not release undo")
				switch fault {
				case "publication":
					require.NoError(t, s.AbortReload(ctx))
					require.NoError(t, finish(false))
					require.Equal(t, int64(73), a.value.Load())
					require.Equal(t, int64(73), b.value.Load())
					require.Equal(t, oldTree, r.GetConfigTree())
				case "finalization":
					err := finish(true)
					require.ErrorContains(t, err, "candidate is active")
					require.ErrorContains(t, err, first)
					require.ErrorContains(t, err, "injected cleanup failure")
					require.Equal(t, candidate, r.GetConfigTree())
					require.Equal(t, int64(99), a.value.Load())
					require.Equal(t, err, finish(false), "irreversible completion cannot turn into abort")
					require.ErrorContains(t, s.AbortReload(ctx), "candidate is active")
					require.Equal(t, candidate, r.GetConfigTree())
					require.Equal(t, int64(1), a.completions.Load())
				case "duplicate":
					require.NoError(t, finish(true))
					nextCtx, nextFinish := s.DeferReloadAcceptance(t.Context())
					next := map[string]any{first: map[string]any{"value": "third"}, second: map[string]any{"value": "third"}}
					require.NoError(t, s.ReloadConfig(nextCtx, next))
					require.NoError(t, finish(true))
					require.NoError(t, finish(false))
					require.Equal(t, int64(1), a.completions.Load(), "stale scope finalized newer journal")
					require.NoError(t, s.AbortReload(ctx), "stale abort must not touch the newer owner")
					require.Equal(t, next, r.GetConfigTree())
					require.NoError(t, nextFinish(false))
					require.Equal(t, candidate, r.GetConfigTree())
					require.Equal(t, int64(99), a.value.Load())
				case "replacement":
					original := spawner.pm.GetProcess(first)
					replacement := process.NewProcess(plugin.PluginConfig{Name: first})
					spawner.pm.AddProcess(first, replacement)
					err := finish(true)
					require.ErrorContains(t, err, "captured process lifetime ended")
					require.Zero(t, a.completions.Load())
					require.Equal(t, candidate, r.GetConfigTree())
					spawner.pm.AddProcess(first, original)
				}
			})
		}
	}
}

var retainedCompletionDecomposerOnce sync.Once

// A real registered decomposer must not reconstruct an owner whose original
// journal is retained, even when another participant needs coarse restoration.
func TestReloadOriginalJournalWithCoarseRestoration(t *testing.T) {
	const ownerName, serviceName = "completion-retained", "completion-dependent"
	retainedCompletionDecomposerOnce.Do(func() {
		require.NoError(t, transaction.RegisterOperationDecomposer(ownerName, func(_ context.Context, req transaction.DecomposeRequest) ([]transaction.ConfigOperation, error) {
			// The registry is process-global and the planner asks every root, so
			// answer only for a reload whose candidate holds this root.
			if req.CandidateRoot == "" || req.CandidateRoot == "{}" {
				return nil, nil
			}
			return []transaction.ConfigOperation{{
				ID: ownerName + "-change", Root: ownerName, Owner: ownerName, Type: testOpSetProperty,
				Verb: transaction.VerbModify, Target: transaction.ResourceRef{Kind: testResourceSysctl, Name: ownerName},
			}}, nil
		}))
	})
	for _, wire := range []bool{false, true} {
		t.Run(fmt.Sprintf("wire=%t", wire), func(t *testing.T) {
			snapshot := registry.Snapshot()
			registry.Reset()
			t.Cleanup(func() { registry.Restore(snapshot) })
			oldTree := map[string]any{ownerName: map[string]any{"value": "old"}, serviceName: map[string]any{"value": int64(42)}}
			candidate := map[string]any{ownerName: map[string]any{"value": "new"}, serviceName: map[string]any{"value": int64(99)}}
			s, _ := newLifecycleStartupServer(t)
			r := &removalRecoveryReactor{tree: oldTree}
			s.reactor = r
			owner := startCompletionOwner(t, s, ownerName, wire)
			var serviceValue atomic.Int64
			require.NoError(t, registry.Register(registry.Registration{
				Name: serviceName, Description: "coarse participant depending on original runtime",
				CLIHandler: func([]string) int { return 0 },
				RunEngine: func(conn net.Conn) int {
					if wire {
						conn = struct{ net.Conn }{conn}
					}
					p := sdk.NewWithConn(serviceName, conn)
					var staged int64
					stage := func(sections []sdk.ConfigSection) error {
						for _, section := range sections {
							var decoded map[string]struct{ Value int64 }
							if err := json.Unmarshal([]byte(section.Data), &decoded); err != nil {
								return err
							}
							staged = decoded[serviceName].Value
						}
						return nil
					}
					p.OnConfigure(func(sections []sdk.ConfigSection) error {
						if err := stage(sections); err != nil {
							return err
						}
						serviceValue.Store(staged)
						return nil
					})
					p.OnConfigVerify(stage)
					p.OnConfigApply(func([]sdk.ConfigDiffSection) error {
						if staged == 42 && owner.value.Load() != 73 {
							return errors.New("dependent service restored before original infrastructure")
						}
						serviceValue.Store(staged)
						return nil
					})
					defer func() { _ = p.Close() }() // Teardown: Run's result is what the fixture reports.
					if err := p.Run(s.Context(), &sdk.Registration{WantsConfig: []string{serviceName}}); err != nil {
						return 1
					}
					return 0
				},
			}))
			require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{{Name: serviceName, Internal: true, Encoder: plugin.EncodingJSON}}))
			ctx, finish := s.DeferReloadAcceptance(t.Context())
			require.NoError(t, s.ReloadConfig(ctx, candidate))
			require.Equal(t, int64(99), owner.value.Load())
			require.Equal(t, int64(99), serviceValue.Load())
			require.Zero(t, owner.completions.Load())
			require.NoError(t, finish(false))
			require.Equal(t, int64(73), owner.value.Load(), "the original journal restores runtime, not candidate-derived reconstruction")
			require.Equal(t, int64(42), serviceValue.Load(), "coarse service restoration still receives the full inverse diff")
			require.Equal(t, int64(1), owner.commits.Load(), "retained owner must not run a fresh inverse transaction")
			require.Equal(t, int64(1), owner.completions.Load())
			require.Equal(t, oldTree, r.GetConfigTree())
		})
	}
}

// Replacement happens after the callback was sent but before its successful
// receipt. The old callback must not certify the replacement's lifetime.
func TestReloadCompletionRejectsInFlightReplacement(t *testing.T) {
	for _, replaceProcess := range []bool{false, true} {
		t.Run(fmt.Sprintf("process=%t", replaceProcess), func(t *testing.T) {
			s, spawner := newLifecycleStartupServer(t)
			require.NoError(t, spawner.SpawnMore(nil))
			s.procManager.Store(spawner.pm)
			const name, txID = "inflight-completion", "captured-transaction"
			original := process.NewProcess(plugin.PluginConfig{Name: name})
			original.SetRegistration(&plugin.PluginRegistration{ConfigCompletion: true})
			bridge := rpc.NewDirectBridge()
			t.Cleanup(bridge.CloseCallbacks)
			conn := &ipc.PluginConn{}
			conn.SetBridge(bridge)
			original.SetConn(conn)
			spawner.pm.AddProcess(name, original)
			successorBridge := rpc.NewDirectBridge()
			t.Cleanup(successorBridge.CloseCallbacks)
			successorConn := &ipc.PluginConn{}
			successorConn.SetBridge(successorBridge)
			ctx, finish := s.DeferReloadAcceptance(t.Context())
			pending := ctx.Value(reloadAcceptanceKey{}).(*reloadAcceptance)
			pending.batches = []*reloadOperationBatch{{
				txID: txID, participants: map[string]reloadParticipant{name: {proc: original, conn: conn}},
			}}
			done := make(chan error, 1)
			go func() { done <- finish(true) }()
			t.Cleanup(func() {
				bridge.CloseCallbacks()
				original.SetConn(nil)
			})
			var callback rpc.BridgeCallback
			select {
			case callback = <-bridge.CallbackCh():
			case <-time.After(5 * time.Second):
				t.Fatal("completion callback did not reach captured connection")
			}
			require.Equal(t, "ze-plugin-callback:config-complete", callback.Method)
			if replaceProcess {
				successor := process.NewProcess(plugin.PluginConfig{Name: name})
				successor.SetConn(successorConn)
				spawner.pm.AddProcess(name, successor)
				t.Cleanup(func() { successor.SetConn(nil) })
			} else {
				original.SetConn(successorConn)
			}
			receipt, err := json.Marshal(rpc.ConfigCompleteOutput{TransactionID: txID, Accepted: true, Status: rpc.StatusOK})
			require.NoError(t, err)
			callback.Result <- rpc.BridgeCallbackResult{Data: receipt}
			select {
			case err := <-done:
				require.ErrorContains(t, err, "captured process lifetime ended")
				require.ErrorContains(t, err, "candidate is active")
				require.Equal(t, err, finish(false), "accepted decision is irreversible despite lifetime loss")
			case <-time.After(5 * time.Second):
				t.Fatal("completion did not return after captured receipt")
			}
			select {
			case <-successorBridge.CallbackCh():
				t.Fatal("completion was redirected to a successor")
			default:
			}
		})
	}
}

// Removed infrastructure has no live original journal. Its acknowledged
// recovery must recreate the prerequisite before a retained dependent undoes.
func TestReloadRecoversRemovedInfrastructureBeforeOriginalUndo(t *testing.T) {
	for _, wire := range []bool{false, true} {
		t.Run(fmt.Sprintf("wire=%t", wire), func(t *testing.T) {
			snapshot := registry.Snapshot()
			registry.Reset()
			t.Cleanup(func() { registry.Restore(snapshot) })
			const infrastructure, dependent = "removed-infrastructure", "retained-dependent"
			oldTree := map[string]any{
				infrastructure: map[string]any{"enabled": true},
				dependent:      map[string]any{"value": "old"},
			}
			candidate := map[string]any{dependent: map[string]any{"value": "new"}}
			s, spawner := newLifecycleStartupServer(t)
			r := &removalRecoveryReactor{tree: oldTree}
			s.reactor = r
			var present atomic.Bool
			var runtime atomic.Int64
			var infrastructureStarts, dependentDecompositions, dependentUndos atomic.Int64
			for _, name := range []string{infrastructure, dependent} {
				// The dependent's original undo needs the infrastructure running.
				var startAfter []string
				if name == dependent {
					startAfter = []string{infrastructure}
				}
				require.NoError(t, registry.Register(registry.Registration{
					Name: name, Description: "removed prerequisite and retained dependent",
					ConfigRoots: []string{name}, StartAfter: startAfter,
					CLIHandler: func([]string) int { return 0 },
					RunEngine: func(conn net.Conn) int {
						if wire {
							conn = struct{ net.Conn }{conn}
						}
						p := sdk.NewWithConn(name, conn)
						if name == infrastructure {
							infrastructureStarts.Add(1)
						}
						var enabled bool
						stage := func(sections []sdk.ConfigSection) error {
							for _, section := range sections {
								var decoded map[string]struct{ Enabled bool }
								if err := json.Unmarshal([]byte(section.Data), &decoded); err != nil {
									return err
								}
								enabled = decoded[name].Enabled
							}
							return nil
						}
						p.OnConfigure(func(sections []sdk.ConfigSection) error {
							if err := stage(sections); err != nil {
								return err
							}
							if name == infrastructure {
								present.Store(enabled)
							} else {
								runtime.Store(73)
							}
							return nil
						})
						p.OnConfigVerify(stage)
						p.OnConfigApply(func([]sdk.ConfigDiffSection) error {
							return errors.New("removed prerequisite fixture requires declared operations")
						})
						p.OnConfigOperationDecompose(func(sdk.ConfigOperationDecomposeInput) (*sdk.ConfigOperationDecomposeOutput, error) {
							if name == dependent {
								dependentDecompositions.Add(1)
							}
							return &sdk.ConfigOperationDecomposeOutput{Operations: []sdk.ConfigOperation{{
								ID: name + "-original", Root: name, Owner: name, Type: testOpSetProperty,
								Verb: transaction.VerbModify, Target: transaction.ResourceRef{Kind: testResourceSysctl, Name: name},
							}}}, nil
						})
						journals := make(map[string]*sdk.Journal)
						p.OnConfigOperationVerify(func(sdk.ConfigOperationVerifyInput) error { return nil })
						p.OnConfigOperationApply(func(input sdk.ConfigOperationApplyInput) (*sdk.ConfigOperationApplyOutput, error) {
							j := sdk.NewJournal()
							var err error
							if name == infrastructure {
								before := present.Load()
								err = j.Record(func() error { present.Store(enabled); return nil }, func() error { present.Store(before); return nil })
							} else {
								before := runtime.Load()
								err = j.Record(func() error { runtime.Store(99); return nil }, func() error { runtime.Store(before); return nil })
							}
							if err != nil {
								return nil, err
							}
							journals[input.TransactionID] = j
							return &sdk.ConfigOperationApplyOutput{Status: rpc.StatusOK}, nil
						})
						p.OnConfigOperationRollback(func(input sdk.ConfigOperationRollbackInput) error {
							j := journals[input.TransactionID]
							if j == nil || len(input.Operations) != 1 || input.Operations[0].ID != name+"-original" {
								return errors.New("undo did not address the original journal and operation token")
							}
							if name == dependent {
								if !present.Load() {
									return errors.New("removed infrastructure is absent during retained dependent undo")
								}
								dependentUndos.Add(1)
							}
							delete(journals, input.TransactionID)
							return errors.Join(j.Rollback()...)
						})
						p.OnConfigOperationCommit(func(sdk.ConfigOperationCommitInput) error { return nil })
						p.OnConfigComplete(func(_ context.Context, input sdk.ConfigCompleteInput) error {
							if j := journals[input.TransactionID]; j != nil {
								if !input.Accepted {
									return errors.New("rejected completion preceded original undo")
								}
								j.Discard()
								delete(journals, input.TransactionID)
							}
							return nil
						})
						p.OnBye(func(string) error {
							if name == infrastructure {
								present.Store(false)
							}
							return nil
						})
						defer func() { _ = p.Close() }() // Teardown: Run's result is what the fixture reports.
						if err := p.Run(s.Context(), &sdk.Registration{
							WantsConfig:      []string{name},
							ConfigOperations: []sdk.ConfigOperationDecl{{Root: name, Decompose: true, Operations: []sdk.ConfigOperationType{testOpSetProperty}}},
						}); err != nil {
							return 1
						}
						return 0
					},
				}))
			}
			require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
				{Name: infrastructure, Internal: true, Encoder: plugin.EncodingJSON},
				{Name: dependent, Internal: true, Encoder: plugin.EncodingJSON},
			}))
			oldInfrastructure := spawner.pm.GetProcess(infrastructure)
			originalDependent := spawner.pm.GetProcess(dependent)
			require.True(t, present.Load())
			ctx, finish := s.DeferReloadAcceptance(t.Context())
			require.NoError(t, s.ReloadConfig(ctx, candidate))
			require.False(t, present.Load())
			require.Nil(t, spawner.pm.GetProcess(infrastructure), "infrastructure removal must be acknowledged before rejection")
			require.Equal(t, int64(99), runtime.Load())
			require.NoError(t, finish(false))
			require.True(t, present.Load(), "removed prerequisite must be recovered from the old configuration")
			require.Equal(t, int64(73), runtime.Load(), "retained dependent must replay its original journal")
			require.Equal(t, int64(1), dependentUndos.Load())
			require.Equal(t, int64(1), dependentDecompositions.Load(), "retained owner must not reconstruct a fresh inverse transaction")
			require.Equal(t, int64(2), infrastructureStarts.Load())
			require.NotSame(t, oldInfrastructure, spawner.pm.GetProcess(infrastructure))
			require.Same(t, originalDependent, spawner.pm.GetProcess(dependent))
			require.Equal(t, oldTree, r.GetConfigTree())
		})
	}
}

// A removed service may accept boot configuration despite a missing device.
// ConfigTx then considers that configuration installed, so a later identical
// apply cannot repair premature recovery after the device journal is undone.
func TestReloadRecoversRemovedServiceAfterOriginalInfrastructureUndo(t *testing.T) {
	for _, wire := range []bool{false, true} {
		t.Run(fmt.Sprintf("wire=%t", wire), func(t *testing.T) {
			snapshot := registry.Snapshot()
			registry.Reset()
			t.Cleanup(func() { registry.Restore(snapshot) })
			const infrastructure, service = "retained-infrastructure", "removed-dependent-service"
			oldTree := map[string]any{
				infrastructure: map[string]any{"value": "old"},
				service:        map[string]any{"enabled": true},
			}
			candidate := map[string]any{infrastructure: map[string]any{"value": "new"}}
			s, spawner := newLifecycleStartupServer(t)
			r := &removalRecoveryReactor{tree: oldTree}
			s.reactor = r
			owner := startCompletionOwner(t, s, infrastructure, wire)
			var serving atomic.Bool
			var starts, prematureBoots atomic.Int64
			require.NoError(t, registry.Register(registry.Registration{
				Name: service, Description: "removed service using ConfigTx and lenient boot binding",
				ConfigRoots: []string{service}, StartAfter: []string{infrastructure},
				CLIHandler: func([]string) int { return 0 },
				RunEngine: func(conn net.Conn) int {
					starts.Add(1)
					if wire {
						conn = struct{ net.Conn }{conn}
					}
					p := sdk.NewWithConn(service, conn)
					boot := false
					tx := sdk.NewConfigTx(false, func(sections []sdk.ConfigSection) (bool, bool, error) {
						var enabled bool
						for _, section := range sections {
							var decoded map[string]struct{ Enabled bool }
							if err := json.Unmarshal([]byte(section.Data), &decoded); err != nil {
								return false, false, err
							}
							enabled = decoded[service].Enabled
						}
						return enabled, true, nil
					}, func(enabled bool) error {
						if enabled && owner.value.Load() != 73 {
							if boot {
								prematureBoots.Add(1)
								serving.Store(false)
								return nil // Matches TFTP's documented lenient boot install.
							}
							return errors.New("service device is absent during strict reload install")
						}
						serving.Store(enabled)
						return nil
					})
					tx.Register(p)
					p.OnConfigure(func(sections []sdk.ConfigSection) error {
						boot = true
						defer func() { boot = false }()
						return tx.Configure(sections)
					})
					p.OnBye(func(string) error { serving.Store(false); return nil })
					defer func() { _ = p.Close() }() // Teardown: Run's result is what the fixture reports.
					if err := p.Run(s.Context(), &sdk.Registration{WantsConfig: []string{service}}); err != nil {
						return 1
					}
					return 0
				},
			}))
			require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{{Name: service, Internal: true, Encoder: plugin.EncodingJSON}}))
			originalInfrastructure := spawner.pm.GetProcess(infrastructure)
			originalService := spawner.pm.GetProcess(service)
			require.True(t, serving.Load())
			ctx, finish := s.DeferReloadAcceptance(t.Context())
			require.NoError(t, s.ReloadConfig(ctx, candidate))
			require.Equal(t, int64(99), owner.value.Load())
			require.False(t, serving.Load())
			require.Nil(t, spawner.pm.GetProcess(service))
			require.NoError(t, finish(false))
			require.Equal(t, int64(73), owner.value.Load(), "retained infrastructure must use its original runtime journal")
			require.True(t, serving.Load(), "recovered service must actually serve, not merely record the old configuration")
			require.Zero(t, prematureBoots.Load(), "removed dependent must not boot before retained prerequisite undo")
			require.Equal(t, int64(2), starts.Load())
			require.Equal(t, int64(1), owner.commits.Load(), "retained infrastructure must not run an inverse transaction")
			require.Same(t, originalInfrastructure, spawner.pm.GetProcess(infrastructure))
			require.NotSame(t, originalService, spawner.pm.GetProcess(service))
			require.Equal(t, oldTree, r.GetConfigTree())
		})
	}
}
