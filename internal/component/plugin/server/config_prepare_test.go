// Design: docs/architecture/config/transaction-protocol.md -- native candidate preparation and outer acceptance.
// Related: reload_test.go -- controlled reactor and plugin response fixtures.

package server

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type recordedConfigPreparation struct {
	decisions []bool
	err       error
}

func (p *recordedConfigPreparation) Complete(_ context.Context, accepted bool) error {
	p.decisions = append(p.decisions, accepted)
	return p.err
}

func isolateConfigPreparations(t *testing.T) {
	t.Helper()
	snapshot := registry.Snapshot()
	registry.Reset()
	configPreparationMu.Lock()
	original := slices.Clone(configPreparations)
	configPreparations = nil
	configPreparationMu.Unlock()
	t.Cleanup(func() {
		configPreparationMu.Lock()
		configPreparations = original
		configPreparationMu.Unlock()
		registry.Restore(snapshot)
	})
}

// TestConfigPreparationRegistration checks invalid, duplicate and sealed
// declarations leave the original owner declaration intact.
func TestConfigPreparationRegistration(t *testing.T) {
	isolateConfigPreparations(t)
	prepare := func(*Server, context.Context, map[string]any, uint64) (ConfigPreparation, error) {
		return nil, nil //nolint:nilnil // This owner is unaffected.
	}
	for _, name := range []string{"", "1owner", "owner/bad", "owner\n"} {
		require.Error(t, RegisterConfigPreparation(name, prepare))
	}
	require.Error(t, RegisterConfigPreparation("native-owner", nil))
	require.Empty(t, configPreparations)
	require.NoError(t, RegisterConfigPreparation("native-owner", prepare))
	first := configPreparations[0]
	require.NotEmpty(t, first.owner, "registration captures the calling package")
	require.Error(t, RegisterConfigPreparation("native-owner", prepare))
	require.Len(t, configPreparations, 1)
	assert.Equal(t, first.owner, configPreparations[0].owner)
	registry.Seal()
	require.ErrorIs(t, RegisterConfigPreparation("late-owner", prepare), registry.ErrRegistrySealed)
	require.Len(t, configPreparations, 1)
}

// TestConfigPreparationNoDiffUsesActiveCandidate exercises native preparation
// through ReloadConfig even when no plugin diff exists, preserving private intent.
func TestConfigPreparationNoDiffUsesActiveCandidate(t *testing.T) {
	isolateConfigPreparations(t)
	tree := config.NewTree()
	tree.SetMemberStates("members", []config.MemberState{{Value: "active"}, {Value: "saved", Inactive: true}})
	candidate := tree.ToRunningMap()
	before := tree.ToRunningMap()
	s := newTestReloadServer(t, &mockReloadReactor{tree: candidate}, nil)
	prepared := &recordedConfigPreparation{}
	calls := 0
	require.NoError(t, RegisterConfigPreparation("native-owner", func(actual *Server, _ context.Context, active map[string]any, generation uint64) (ConfigPreparation, error) {
		calls++
		require.Same(t, s, actual)
		assert.Equal(t, config.ActiveConfigMap(before), active)
		assert.Equal(t, uint64(1), generation)
		return prepared, nil
	}))
	require.NoError(t, s.ReloadConfig(t.Context(), candidate))
	assert.Equal(t, 1, calls)
	assert.Equal(t, []bool{true}, prepared.decisions)
	assert.Equal(t, before, candidate, "preparation preserves retained running intent")
}

// TestConfigPreparationWaitsForOuterDecision proves inner success neither
// selects nor aborts a native value, and repeated final decisions finish it once.
func TestConfigPreparationWaitsForOuterDecision(t *testing.T) {
	for _, accepted := range []bool{false, true} {
		t.Run(map[bool]string{false: "abort", true: "accept"}[accepted], func(t *testing.T) {
			isolateConfigPreparations(t)
			s := newTestReloadServer(t, &mockReloadReactor{tree: map[string]any{}}, nil)
			prepared := &recordedConfigPreparation{}
			require.NoError(t, RegisterConfigPreparation("native-owner", func(*Server, context.Context, map[string]any, uint64) (ConfigPreparation, error) {
				return prepared, nil
			}))
			ctx, finish := s.DeferReloadAcceptance(t.Context())
			t.Cleanup(func() { assert.NoError(t, finish(false)) })
			require.NoError(t, s.ReloadConfig(ctx, map[string]any{"revision": "next"}))
			require.Empty(t, prepared.decisions, "inner runtime success is still tentative")
			require.NoError(t, finish(accepted))
			require.NoError(t, finish(!accepted))
			assert.Equal(t, []bool{accepted}, prepared.decisions)
		})
	}
}

// TestConfigPreparationFailureAbortsEarlierValues proves a fallible owner
// refuses before runtime writes and settles every value produced by that attempt.
func TestConfigPreparationFailureAbortsEarlierValues(t *testing.T) {
	isolateConfigPreparations(t)
	reactor := &mockReloadReactor{tree: map[string]any{}}
	s := newTestReloadServer(t, reactor, nil)
	first, failed := &recordedConfigPreparation{}, &recordedConfigPreparation{}
	failure := errors.New("native preparation refused")
	require.NoError(t, RegisterConfigPreparation("first-owner", func(*Server, context.Context, map[string]any, uint64) (ConfigPreparation, error) {
		return first, nil
	}))
	require.NoError(t, RegisterConfigPreparation("failed-owner", func(*Server, context.Context, map[string]any, uint64) (ConfigPreparation, error) {
		return failed, failure
	}))
	require.ErrorIs(t, s.ReloadConfig(t.Context(), map[string]any{"revision": "next"}), failure)
	assert.Equal(t, []bool{false}, first.decisions)
	assert.Equal(t, []bool{false}, failed.decisions)
	assert.Nil(t, reactor.applyTree)
	assert.Nil(t, reactor.setTree)
}

// TestConfigPreparationLaterRefusalKeepsPriorAttempt accepts an earlier valid
// attempt in the same scope after a later plugin verify rejects its candidate.
func TestConfigPreparationLaterRefusalKeepsPriorAttempt(t *testing.T) {
	isolateConfigPreparations(t)
	s := newTestReloadServer(t, &mockReloadReactor{tree: map[string]any{}}, []pluginDef{{
		name: "refusing-owner", roots: []string{"refused"},
		verifyResp: &rpc.ConfigVerifyOutput{Status: plugin.StatusError, Error: "test refusal"},
	}})
	var attempts []*recordedConfigPreparation
	require.NoError(t, RegisterConfigPreparation("native-owner", func(*Server, context.Context, map[string]any, uint64) (ConfigPreparation, error) {
		prepared := &recordedConfigPreparation{}
		attempts = append(attempts, prepared)
		return prepared, nil
	}))
	ctx, finish := s.DeferReloadAcceptance(t.Context())
	t.Cleanup(func() { assert.NoError(t, finish(false)) })
	require.NoError(t, s.ReloadConfig(ctx, map[string]any{}))
	require.Error(t, s.ReloadConfig(ctx, map[string]any{"refused": map[string]any{"enabled": "true"}}))
	require.Len(t, attempts, 2)
	assert.Empty(t, attempts[0].decisions)
	assert.Equal(t, []bool{false}, attempts[1].decisions)
	require.NoError(t, finish(true))
	assert.Equal(t, []bool{true}, attempts[0].decisions)
	assert.Equal(t, []bool{false}, attempts[1].decisions)
}

// TestConfigPreparationAcceptedDespiteReceiptFailure injects a recorded plugin
// receipt failure after publication; native selection still follows acceptance.
func TestConfigPreparationAcceptedDespiteReceiptFailure(t *testing.T) {
	isolateConfigPreparations(t)
	s := newTestReloadServer(t, &mockReloadReactor{tree: map[string]any{}}, nil)
	prepared := &recordedConfigPreparation{}
	require.NoError(t, RegisterConfigPreparation("native-owner", func(*Server, context.Context, map[string]any, uint64) (ConfigPreparation, error) {
		return prepared, nil
	}))
	ctx, finish := s.DeferReloadAcceptance(t.Context())
	failure := errors.New("plugin receipt failed")
	t.Cleanup(func() { assert.ErrorIs(t, finish(false), failure) })
	require.NoError(t, s.ReloadConfig(ctx, map[string]any{}))
	pending, ok := ctx.Value(reloadAcceptanceKey{}).(*reloadAcceptance)
	require.True(t, ok, "the original acceptance scope is retained in the context")
	require.NotNil(t, pending)
	pending.batches = []*reloadOperationBatch{{completed: true, err: failure}}
	require.ErrorIs(t, finish(true), failure)
	assert.Equal(t, []bool{true}, prepared.decisions)
}

// TestConfigPreparationGenerationsFollowAttempts preserves original scope
// decisions when newer and older callers finish out of admission order.
func TestConfigPreparationGenerationsFollowAttempts(t *testing.T) {
	isolateConfigPreparations(t)
	s := newTestReloadServer(t, &mockReloadReactor{tree: map[string]any{}}, nil)
	var generations []uint64
	var attempts []*recordedConfigPreparation
	require.NoError(t, RegisterConfigPreparation("native-owner", func(_ *Server, _ context.Context, _ map[string]any, generation uint64) (ConfigPreparation, error) {
		generations = append(generations, generation)
		prepared := &recordedConfigPreparation{}
		attempts = append(attempts, prepared)
		return prepared, nil
	}))
	older, finishOlder := s.DeferReloadAcceptance(t.Context())
	t.Cleanup(func() { assert.NoError(t, finishOlder(false)) })
	newer, finishNewer := s.DeferReloadAcceptance(t.Context())
	t.Cleanup(func() { assert.NoError(t, finishNewer(false)) })
	require.NoError(t, s.ReloadConfig(older, map[string]any{}))
	require.NoError(t, s.ReloadConfig(newer, map[string]any{}))
	require.Equal(t, []uint64{1, 2}, generations)
	require.NoError(t, finishNewer(true))
	assert.Empty(t, attempts[0].decisions, "newer acceptance does not decide the older native value")
	require.NoError(t, finishOlder(true))
	assert.Equal(t, []bool{true}, attempts[0].decisions)
	assert.Equal(t, []bool{true}, attempts[1].decisions)
}

// TestConfigPreparationCancellationAbortsValues cancels within the callback
// and proves native cleanup precedes a refused reload with no runtime writes.
func TestConfigPreparationCancellationAbortsValues(t *testing.T) {
	isolateConfigPreparations(t)
	s := newTestReloadServer(t, &mockReloadReactor{tree: map[string]any{}}, nil)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	prepared := &recordedConfigPreparation{}
	require.NoError(t, RegisterConfigPreparation("native-owner", func(*Server, context.Context, map[string]any, uint64) (ConfigPreparation, error) {
		cancel()
		return prepared, nil
	}))
	require.ErrorIs(t, s.ReloadConfig(ctx, map[string]any{}), context.Canceled)
	assert.Equal(t, []bool{false}, prepared.decisions)
}

// TestConfigPreparationUnaffectedOwner accepts an empty candidate when an
// owner explicitly reports no native work, without manufacturing a completion.
func TestConfigPreparationUnaffectedOwner(t *testing.T) {
	isolateConfigPreparations(t)
	s := newTestReloadServer(t, &mockReloadReactor{tree: map[string]any{}}, nil)
	calls := 0
	require.NoError(t, RegisterConfigPreparation("native-owner", func(*Server, context.Context, map[string]any, uint64) (ConfigPreparation, error) {
		calls++
		return nil, nil //nolint:nilnil // This candidate requires no native preparation.
	}))
	require.NoError(t, s.ReloadConfig(t.Context(), map[string]any{}))
	assert.Equal(t, 1, calls)
}

// TestConfigPreparationFailedAttemptKeepsEarlierGeneration makes a newer
// preparation fail within one outer scope, then accepts its earlier valid value.
func TestConfigPreparationFailedAttemptKeepsEarlierGeneration(t *testing.T) {
	isolateConfigPreparations(t)
	s := newTestReloadServer(t, &mockReloadReactor{tree: map[string]any{}}, nil)
	prepared := &recordedConfigPreparation{}
	failure := errors.New("newer preparation refused")
	var generations []uint64
	require.NoError(t, RegisterConfigPreparation("native-owner", func(_ *Server, _ context.Context, _ map[string]any, generation uint64) (ConfigPreparation, error) {
		generations = append(generations, generation)
		if generation == 2 {
			return nil, failure
		}
		return prepared, nil
	}))
	ctx, finish := s.DeferReloadAcceptance(t.Context())
	t.Cleanup(func() { assert.NoError(t, finish(false)) })
	require.NoError(t, s.ReloadConfig(ctx, map[string]any{}))
	require.ErrorIs(t, s.ReloadConfig(ctx, map[string]any{}), failure)
	assert.Empty(t, prepared.decisions)
	require.NoError(t, finish(true))
	assert.Equal(t, []uint64{1, 2}, generations)
	assert.Equal(t, []bool{true}, prepared.decisions)
}

// TestConfigPreparationCompletionFailurePreservesDecision exercises an owner's
// cleanup error at outer acceptance: every owner still finishes once with the
// original decision, and repeated completion returns the same qualified error.
func TestConfigPreparationCompletionFailurePreservesDecision(t *testing.T) {
	for _, accepted := range []bool{false, true} {
		t.Run(map[bool]string{false: "abort", true: "accept"}[accepted], func(t *testing.T) {
			isolateConfigPreparations(t)
			s := newTestReloadServer(t, &mockReloadReactor{tree: map[string]any{}}, nil)
			failure := errors.New("native cleanup failed")
			failed := &recordedConfigPreparation{err: failure}
			next := &recordedConfigPreparation{}
			require.NoError(t, RegisterConfigPreparation("failed-owner", func(*Server, context.Context, map[string]any, uint64) (ConfigPreparation, error) {
				return failed, nil
			}))
			require.NoError(t, RegisterConfigPreparation("next-owner", func(*Server, context.Context, map[string]any, uint64) (ConfigPreparation, error) {
				return next, nil
			}))
			ctx, finish := s.DeferReloadAcceptance(t.Context())
			t.Cleanup(func() { assert.ErrorIs(t, finish(false), failure) })
			require.NoError(t, s.ReloadConfig(ctx, map[string]any{}))
			err := finish(accepted)
			require.ErrorIs(t, err, failure)
			assert.Contains(t, err.Error(), "config preparation failed-owner: native cleanup failed")
			if accepted {
				assert.Contains(t, err.Error(), "candidate is active but config finalization failed")
			} else {
				assert.NotContains(t, err.Error(), "candidate is active")
			}
			repeated := finish(!accepted)
			require.ErrorIs(t, repeated, failure)
			assert.Equal(t, err.Error(), repeated.Error())
			assert.Equal(t, []bool{accepted}, failed.decisions)
			assert.Equal(t, []bool{accepted}, next.decisions, "one cleanup failure must not suppress another owner")
		})
	}
}
