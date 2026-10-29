// Design: docs/architecture/config/transaction-protocol.md -- rejected reload compensation
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/transaction"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// scheduleOwner is a retained journal owner whose undo can be made to fail.
// undos counts every inverse the owner received, successful or not.
type scheduleOwner struct {
	value    atomic.Int64
	undos    atomic.Int64
	failUndo atomic.Bool
}

// registerScheduleOwner registers, without starting, an owner of root name
// that applies one declared operation with a runtime journal. startAfter is
// its declared ordering metadata.
func registerScheduleOwner(t *testing.T, s *Server, name string, wire bool, startAfter []string) *scheduleOwner {
	t.Helper()
	owner := &scheduleOwner{}
	owner.value.Store(73)
	require.NoError(t, registry.Register(registry.Registration{
		Name: name, Description: "retained journal owner for the recovery schedule",
		ConfigRoots: []string{name}, StartAfter: startAfter,
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
				return errors.New("schedule owner requires declared operations")
			})
			p.OnConfigOperationDecompose(func(sdk.ConfigOperationDecomposeInput) (*sdk.ConfigOperationDecomposeOutput, error) {
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
				return nil, nil //nolint:nilnil // The SDK reads a nil output as an apply that reports nothing, as the other journal fixtures do.
			})
			p.OnConfigOperationRollback(func(input sdk.ConfigOperationRollbackInput) error {
				owner.undos.Add(1)
				j := journals[input.TransactionID]
				if j == nil {
					return errors.New("original journal is missing")
				}
				delete(journals, input.TransactionID)
				if owner.failUndo.Load() {
					return errors.New("injected undo failure")
				}
				return errors.Join(j.Rollback()...)
			})
			p.OnConfigOperationCommit(func(sdk.ConfigOperationCommitInput) error { return nil })
			p.OnConfigComplete(func(_ context.Context, input sdk.ConfigCompleteInput) error {
				if j := journals[input.TransactionID]; j != nil {
					if !input.Accepted {
						return errors.New("abort completion preceded original undo")
					}
					j.Discard()
					delete(journals, input.TransactionID)
				}
				return nil
			})
			defer p.Close() //nolint:errcheck // Teardown of a test plugin: Run's result is what the fixture reports.
			if err := p.Run(s.Context(), &sdk.Registration{
				WantsConfig:      []string{name},
				ConfigOperations: []sdk.ConfigOperationDecl{{Root: name, Decompose: true, Operations: []sdk.ConfigOperationType{testOpSetProperty}}},
			}); err != nil {
				return 1
			}
			return 0
		},
	}))
	return owner
}

// scheduleService is a coarse service whose root a reload removes.
type scheduleService struct {
	starts  atomic.Int64
	serving atomic.Bool
}

func registerScheduleService(t *testing.T, s *Server, name string, wire bool, startAfter []string) *scheduleService {
	t.Helper()
	service := &scheduleService{}
	require.NoError(t, registry.Register(registry.Registration{
		Name: name, Description: "removed coarse service for the recovery schedule",
		ConfigRoots: []string{name}, StartAfter: startAfter,
		CLIHandler: func([]string) int { return 0 },
		RunEngine: func(conn net.Conn) int {
			service.starts.Add(1)
			if wire {
				conn = struct{ net.Conn }{conn}
			}
			p := sdk.NewWithConn(name, conn)
			p.OnConfigure(func(sections []sdk.ConfigSection) error {
				for _, section := range sections {
					var decoded map[string]struct{ Enabled bool }
					if err := json.Unmarshal([]byte(section.Data), &decoded); err != nil {
						return err
					}
					service.serving.Store(decoded[name].Enabled)
				}
				return nil
			})
			p.OnConfigVerify(func([]sdk.ConfigSection) error { return nil })
			p.OnConfigApply(func([]sdk.ConfigDiffSection) error { return nil })
			p.OnBye(func(string) error { service.serving.Store(false); return nil })
			defer p.Close() //nolint:errcheck // Teardown of a test plugin: Run's result is what the fixture reports.
			if err := p.Run(s.Context(), &sdk.Registration{WantsConfig: []string{name}}); err != nil {
				return 1
			}
			return 0
		},
	}))
	return service
}

// A removed service and a retained owner that each declare they start after
// the other cannot be ordered. Goal: the compensation refuses the cycle before
// it executes anything. Method: start the two in separate phases, as when one
// was added later, so startup never sees the cycle; reject a reload that
// removes the service and changes the owner; then check that no inverse was
// sent and no recovery startup ran, over both transports.
func TestReloadCompensationRefusesRecoveryCycleBeforeExecution(t *testing.T) {
	for _, wire := range []bool{false, true} {
		t.Run(fmt.Sprintf("wire=%t", wire), func(t *testing.T) {
			snapshot := registry.Snapshot()
			registry.Reset()
			t.Cleanup(func() { registry.Restore(snapshot) })
			const owner, service = "cycle-retained-owner", "cycle-removed-service"
			oldTree := map[string]any{
				owner:   map[string]any{"value": "old"},
				service: map[string]any{"enabled": true},
			}
			candidate := map[string]any{owner: map[string]any{"value": "new"}}
			s, spawner := newLifecycleStartupServer(t)
			s.reactor = &removalRecoveryReactor{tree: oldTree}
			removed := registerScheduleService(t, s, service, wire, []string{owner})
			retained := registerScheduleOwner(t, s, owner, wire, []string{service})
			require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{{Name: service, Internal: true, Encoder: plugin.EncodingJSON}}))
			require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{{Name: owner, Internal: true, Encoder: plugin.EncodingJSON}}))
			original := spawner.pm.GetProcess(owner)
			require.True(t, removed.serving.Load())

			ctx, finish := s.DeferReloadAcceptance(t.Context())
			require.NoError(t, s.ReloadConfig(ctx, candidate))
			require.Equal(t, int64(99), retained.value.Load())
			require.Nil(t, spawner.pm.GetProcess(service))

			require.ErrorContains(t, finish(false), "cannot order the removed and retained plugins")
			require.Zero(t, retained.undos.Load(), "a refused schedule must send no inverse")
			require.Equal(t, int64(99), retained.value.Load())
			require.Equal(t, int64(1), removed.starts.Load(), "a refused schedule must start no removed plugin")
			require.Nil(t, spawner.pm.GetProcess(service))
			require.Same(t, original, spawner.pm.GetProcess(owner))
		})
	}
}

// A failed original inverse consumes its journal. Goal: the schedule records
// the failure, still undoes the other retained owner, and a retry reports the
// recorded failure without sending that inverse again. Method: two retained
// owners change in one reload, one refuses its undo; reject the reload, then
// start another reload, which retries the pending compensation first, over
// both transports.
func TestReloadCompensationConsumesFailedOriginalInverse(t *testing.T) {
	for _, wire := range []bool{false, true} {
		t.Run(fmt.Sprintf("wire=%t", wire), func(t *testing.T) {
			snapshot := registry.Snapshot()
			registry.Reset()
			t.Cleanup(func() { registry.Restore(snapshot) })
			const failing, healthy = "undo-failing-owner", "undo-healthy-owner"
			oldTree := map[string]any{
				failing: map[string]any{"value": "old"},
				healthy: map[string]any{"value": "old"},
			}
			candidate := map[string]any{
				failing: map[string]any{"value": "new"},
				healthy: map[string]any{"value": "new"},
			}
			s, _ := newLifecycleStartupServer(t)
			s.reactor = &removalRecoveryReactor{tree: oldTree}
			refusing := registerScheduleOwner(t, s, failing, wire, nil)
			accepting := registerScheduleOwner(t, s, healthy, wire, nil)
			refusing.failUndo.Store(true)
			require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{
				{Name: failing, Internal: true, Encoder: plugin.EncodingJSON},
				{Name: healthy, Internal: true, Encoder: plugin.EncodingJSON},
			}))

			ctx, finish := s.DeferReloadAcceptance(t.Context())
			require.NoError(t, s.ReloadConfig(ctx, candidate))
			require.Equal(t, int64(99), refusing.value.Load())
			require.Equal(t, int64(99), accepting.value.Load())

			require.ErrorContains(t, finish(false), "injected undo failure")
			require.Equal(t, int64(1), refusing.undos.Load())
			require.Equal(t, int64(1), accepting.undos.Load(), "a failed inverse must not stop the other owner's undo")
			require.Equal(t, int64(73), accepting.value.Load())

			retryCtx, retryFinish := s.DeferReloadAcceptance(t.Context())
			require.ErrorContains(t, s.ReloadConfig(retryCtx, candidate), "injected undo failure",
				"the retry must report the consumed inverse's recorded failure")
			require.NoError(t, retryFinish(false))
			require.Equal(t, int64(1), refusing.undos.Load(), "a consumed inverse must never be sent again")
			require.Equal(t, int64(1), accepting.undos.Load())
		})
	}
}

// plannerStep is one retained inverse of a planner fixture: the owner that
// undoes it and the resource kind its original operation targeted.
type plannerStep struct {
	owner string
	kind  transaction.ResourceKind
}

// registerPlannerPlugin registers, without starting, a plugin row whose only
// purpose is its StartAfter edges: the planner reads the startup graph from
// the registry.
func registerPlannerPlugin(t *testing.T, name string, startAfter ...string) {
	t.Helper()
	require.NoError(t, registry.Register(registry.Registration{
		Name: name, Description: "recovery planner fixture", StartAfter: startAfter,
		CLIHandler: func([]string) int { return 0 },
		RunEngine:  func(net.Conn) int { return 0 },
	}))
}

// plannerChain builds a retained inverse chain in dispatch order, one
// declared modify per step, each owner one captured process.
func plannerChain(steps ...plannerStep) []inverseAction {
	batch := &reloadOperationBatch{txID: "planner-tx", participants: make(map[string]reloadParticipant)}
	for _, step := range steps {
		if _, seen := batch.participants[step.owner]; !seen {
			batch.participants[step.owner] = reloadParticipant{proc: process.NewProcess(plugin.PluginConfig{Name: step.owner, Internal: true})}
		}
		batch.operations = append(batch.operations, transaction.ConfigOperation{
			ID: step.owner + "-change", Root: step.owner, Owner: step.owner, Type: testOpSetProperty,
			Verb: transaction.VerbModify, Target: transaction.ResourceRef{Kind: step.kind, Name: step.owner},
		})
	}
	chain := make([]inverseAction, len(steps))
	for position, step := range steps {
		chain[position] = inverseAction{batch: batch, position: position, participant: batch.participants[step.owner]}
	}
	return chain
}

// plannerRecovery names the removed owners to restart, as internal plugins.
func plannerRecovery(names ...string) []plugin.PluginConfig {
	recovery := make([]plugin.PluginConfig, len(names))
	for i, name := range names {
		recovery[i] = plugin.PluginConfig{Name: name, Internal: true}
	}
	return recovery
}

// isolatePlannerRegistry gives a planner test an empty registry it restores.
func isolatePlannerRegistry(t *testing.T) {
	t.Helper()
	snapshot := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snapshot) })
}

// slotOf answers the slot a removed owner starts in, or -1 when it starts in
// none.
func slotOf(schedule *recoverySchedule, name string) int {
	for slot, starts := range schedule.starts {
		for _, cfg := range starts {
			if cfg.Name == name {
				return slot
			}
		}
	}
	return -1
}

// A removed plugin P that must start after retained R1 is undone and before
// retained R2 is undone, while the fixed reverse apply order undoes R2 first,
// has no slot. No cycle exists. Goal: the planner refuses, and the refusal
// names P, R1 and R2, the undo order that crosses, and the restart that
// recovers. Method: call the planner directly with chain [R2, R1].
func TestPlanRecoveryScheduleRefusesCrossedBounds(t *testing.T) {
	isolatePlannerRegistry(t)
	const removed, first, second = "crossed-p", "crossed-r1", "crossed-r2"
	registerPlannerPlugin(t, removed, first)
	registerPlannerPlugin(t, first)
	registerPlannerPlugin(t, second, removed)

	_, err := planRecoverySchedule(plannerChain(plannerStep{second, testResourceSysctl}, plannerStep{first, testResourceSysctl}), plannerRecovery(removed))
	require.Error(t, err)
	message := err.Error()
	for _, name := range []string{removed, first, second} {
		require.Contains(t, message, name)
	}
	require.Contains(t, message, "reverse apply order")
	require.Contains(t, message, "systemctl restart ze")
	require.NotContains(t, message, "one direction only", "no cycle exists, so the refusal must not ask for one to be removed")
	require.NotContains(t, message, "cycle")
}

// When the fixed undo order leaves room, the removed plugin takes the one slot
// its declared edges allow. Goal: a valid chain places P after R1's undo and
// before R2's. Method: chain [R1, R2], P after R1, R2 after P.
func TestPlanRecoverySchedulePlacesBetweenDeclaredBounds(t *testing.T) {
	isolatePlannerRegistry(t)
	const removed, first, second = "valid-p", "valid-r1", "valid-r2"
	registerPlannerPlugin(t, removed, first)
	registerPlannerPlugin(t, first)
	registerPlannerPlugin(t, second, removed)

	schedule, err := planRecoverySchedule(plannerChain(plannerStep{first, testResourceSysctl}, plannerStep{second, testResourceSysctl}), plannerRecovery(removed))
	require.NoError(t, err)
	require.Len(t, schedule.starts, 3)
	require.Equal(t, 1, slotOf(schedule, removed))
}

// Reach runs through removed plugins. Goal: a removed plugin that depends on
// a retained owner only through another removed plugin still starts after
// that owner's undo, and the other removed plugin inherits the upper bound of
// the retained owner that depends on its dependent. Method: P2 after P1, P1
// after R1, R2 after P2, chain [R0, R1, R2] of binders, so the undeclared
// placement alone would put both at slot 0.
func TestPlanRecoveryScheduleReachesThroughRemovedPlugins(t *testing.T) {
	isolatePlannerRegistry(t)
	const p1, p2, r0, r1, r2 = "reach-p1", "reach-p2", "reach-r0", "reach-r1", "reach-r2"
	registerPlannerPlugin(t, p1, r1)
	registerPlannerPlugin(t, p2, p1)
	registerPlannerPlugin(t, r0)
	registerPlannerPlugin(t, r1)
	registerPlannerPlugin(t, r2, p2)

	chain := plannerChain(plannerStep{r0, testResourceSysctl}, plannerStep{r1, testResourceSysctl}, plannerStep{r2, testResourceSysctl})
	schedule, err := planRecoverySchedule(chain, plannerRecovery(p1, p2))
	require.NoError(t, err)
	require.Equal(t, 2, slotOf(schedule, p1))
	require.Equal(t, 2, slotOf(schedule, p2), "P2 reaches R1 through removed P1, so it starts after R1's undo")

	// R2 after P2 with chain [R2, R1]: P1's upper bound comes from R2 through
	// removed P2, which crosses R1's lower bound.
	isolatePlannerRegistry(t)
	registerPlannerPlugin(t, p1, r1)
	registerPlannerPlugin(t, p2, p1)
	registerPlannerPlugin(t, r1)
	registerPlannerPlugin(t, r2, p2)
	_, err = planRecoverySchedule(plannerChain(plannerStep{r2, testResourceSysctl}, plannerStep{r1, testResourceSysctl}), plannerRecovery(p1, p2))
	require.ErrorContains(t, err, p1)
}

// A removed plugin with no declared edge takes the coarse placement. Goal:
// pin the fallback slot: after the last inverse that can restore addressing,
// before the first binder an inverse brings back. Method: chain [interface
// modify, sysctl modify, sysctl modify] places P at slot 1, not 0 or 3.
func TestPlanRecoverySchedulePlacesUndeclaredByAddressing(t *testing.T) {
	isolatePlannerRegistry(t)
	const removed, addressing, binder, later = "fallback-p", "fallback-iface", "fallback-binder", "fallback-later"
	for _, name := range []string{removed, addressing, binder, later} {
		registerPlannerPlugin(t, name)
	}
	chain := plannerChain(
		plannerStep{addressing, transaction.ResourceInterface},
		plannerStep{binder, testResourceSysctl},
		plannerStep{later, testResourceSysctl},
	)
	schedule, err := planRecoverySchedule(chain, plannerRecovery(removed))
	require.NoError(t, err)
	require.Equal(t, 1, slotOf(schedule, removed))
}

// A cycle made only of retained owners never blocked compensation before the
// recovery schedule, because no removed plugin's order depends on it. Goal:
// the planner refuses only a cycle that passes through a removed plugin.
// Method: R1 and R2 start after each other, P is unrelated; then P joins the
// cycle and the planner refuses it.
func TestPlanRecoveryScheduleIgnoresRetainedOnlyCycle(t *testing.T) {
	isolatePlannerRegistry(t)
	const removed, first, second = "loop-p", "loop-r1", "loop-r2"
	registerPlannerPlugin(t, removed)
	registerPlannerPlugin(t, first, second)
	registerPlannerPlugin(t, second, first)
	chain := plannerChain(plannerStep{first, testResourceSysctl}, plannerStep{second, testResourceSysctl})
	schedule, err := planRecoverySchedule(chain, plannerRecovery(removed))
	require.NoError(t, err)
	require.NotEqual(t, -1, slotOf(schedule, removed))

	isolatePlannerRegistry(t)
	registerPlannerPlugin(t, removed, first)
	registerPlannerPlugin(t, first, second)
	registerPlannerPlugin(t, second, removed)
	_, err = planRecoverySchedule(chain, plannerRecovery(removed))
	require.ErrorContains(t, err, "cannot order the removed and retained plugins")
	require.ErrorIs(t, err, registry.ErrCircularDependency)
}

// Goal: a cycle of retained owners beside an unrelated removed service does
// not stop the compensation: both owners are undone and the service is
// started again. Method: start the two owners, which start after each other,
// in separate phases and the service in a third; reject a reload that removes
// the service and changes both owners, over both transports.
func TestReloadCompensationRetainedOnlyCycleStillCompensates(t *testing.T) {
	for _, wire := range []bool{false, true} {
		t.Run(fmt.Sprintf("wire=%t", wire), func(t *testing.T) {
			snapshot := registry.Snapshot()
			registry.Reset()
			t.Cleanup(func() { registry.Restore(snapshot) })
			const first, second, service = "loop-owner-one", "loop-owner-two", "loop-removed-service"
			oldTree := map[string]any{
				first:   map[string]any{"value": "old"},
				second:  map[string]any{"value": "old"},
				service: map[string]any{"enabled": true},
			}
			candidate := map[string]any{
				first:  map[string]any{"value": "new"},
				second: map[string]any{"value": "new"},
			}
			s, spawner := newLifecycleStartupServer(t)
			s.reactor = &removalRecoveryReactor{tree: oldTree}
			one := registerScheduleOwner(t, s, first, wire, []string{second})
			two := registerScheduleOwner(t, s, second, wire, []string{first})
			removed := registerScheduleService(t, s, service, wire, nil)
			require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{{Name: first, Internal: true, Encoder: plugin.EncodingJSON}}))
			require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{{Name: second, Internal: true, Encoder: plugin.EncodingJSON}}))
			require.NoError(t, s.runPluginPhase([]plugin.PluginConfig{{Name: service, Internal: true, Encoder: plugin.EncodingJSON}}))

			ctx, finish := s.DeferReloadAcceptance(t.Context())
			require.NoError(t, s.ReloadConfig(ctx, candidate))
			require.Nil(t, spawner.pm.GetProcess(service))

			require.NoError(t, finish(false))
			require.Equal(t, int64(73), one.value.Load())
			require.Equal(t, int64(73), two.value.Load())
			require.Equal(t, int64(2), removed.starts.Load(), "the removed service must start again")
			require.True(t, removed.serving.Load())
		})
	}
}

// An inverse that never reached its owner is still owed. Goal: a send
// failure (here a payload that cannot be marshaled) is not recorded as
// consumed, the batch is not settled, and the schedule stops before the next
// inverse. Method: run a hand-built schedule whose first operation carries an
// invalid raw JSON parameter, followed by a valid one.
func TestReloadCompensationUnsentInverseStaysOwed(t *testing.T) {
	isolatePlannerRegistry(t)
	s, _ := newLifecycleStartupServer(t)
	chain := plannerChain(plannerStep{"unsent-owner", testResourceSysctl}, plannerStep{"later-owner", testResourceSysctl})
	chain[0].batch.operations[0].Params.Spec = json.RawMessage("{not json")
	undo := &reloadCompensation{inverses: make(map[inverseKey]inverseOutcome)}
	schedule, err := planRecoverySchedule(chain, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	err = s.runRecoverySchedule(ctx, &reloadAcceptance{}, undo, schedule, nil)
	require.ErrorContains(t, err, "unsent-owner")
	_, recorded := undo.inverses[chain[0].key()]
	require.False(t, recorded, "an inverse that was never sent must stay owed")
	_, recorded = undo.inverses[chain[1].key()]
	require.False(t, recorded, "the schedule must stop before the next inverse")
	require.False(t, chain[0].batch.undoDone)
}
