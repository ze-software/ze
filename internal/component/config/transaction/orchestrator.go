// Design: docs/architecture/config/transaction-protocol.md -- transaction orchestrator
// Related: gateway.go -- EventGateway interface this orchestrator depends on
// Related: topics.go -- event type constants used in gateway calls
// Related: types.go -- event payload types

package transaction

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"log/slog"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func logger() *slog.Logger { return slogutil.Logger("config.transaction") }

// tierFn computes dependency tiers for a set of plugin names. Rollback ack
// collection drains them in reverse tier order, and it is the only caller: the
// deadline is a sum over participants and reads no tier
// (orchestrator_budget.go). Package-level so tests can override it without
// mutating the global plugin registry.
var tierFn = registry.TopologicalTiers

// Report bus source and codes for config transaction error events.
// The bus is the operator-visible feed behind `ze show errors`.
const (
	reportSourceConfig       = "config"
	reportCodeCommitAborted  = "commit-aborted"     // verify phase failed
	reportCodeCommitRollback = "commit-rollback"    // apply phase failed, rollback initiated
	reportCodeCommitSaveFail = "commit-save-failed" // apply succeeded but config file write failed

	// reportKeyPhase names the transaction phase in the detail map of every
	// error this file raises, so an operator can group them by phase.
	reportKeyPhase = "phase"
)

// Transaction states.
const (
	StateIdle        = "idle"
	StateVerifying   = "verifying"
	StateApplying    = "applying"
	StateCommitted   = "committed"
	StateAborted     = "aborted"
	StateRolledBack  = "rolled-back"
	StateRollingBack = "rolling-back"
)

// errConfigWriteFailed is a sentinel for config file write failure tests.
var errConfigWriteFailed = errors.New("config write failed")

// ErrShutdown is the cancellation CAUSE a caller sets, through
// context.WithCancelCause, when it stops a transaction because the process is
// going down. It is not an ordinary cancellation and it is not a plugin
// failure. Execute unwinds, and it emits no abort and no rollback:
//
//   - there is no running system left to restore, and every participant is
//     about to be killed, so the rollback fan-out and the broken-plugin
//     RESTART it can trigger are work started on processes that are exiting;
//   - the same events tell operators a plugin crashed when nothing crashed.
//
// An ordinary cancellation still rolls back. A reload whose own deadline
// expires mid-apply leaves participants half-applied inside a daemon that
// keeps running, and that daemon must be told to undo it.
var ErrShutdown = errors.New("config transaction canceled by shutdown")

// canceledByShutdown reports whether ctx carries ErrShutdown as its
// cancellation cause. context.Cause returns the ctx error itself when no
// cause was set, so a plain cancel and a deadline both answer false here.
func canceledByShutdown(ctx context.Context) bool {
	return errors.Is(context.Cause(ctx), ErrShutdown)
}

// Participant describes a plugin participating in a config transaction.
type Participant struct {
	Name             string
	ConfigRoots      []string // Roots this plugin owns.
	WantsConfig      []string // Roots this plugin reads (not owner).
	ConfigOperations []ConfigOperationDecl
	VerifyBudget     int // Estimated verify seconds.
	ApplyBudget      int // Estimated apply seconds.
}

// TxResult is the outcome of a transaction execution.
type TxResult struct {
	State string // Final state (StateCommitted, StateAborted, StateRolledBack).
	Err   error  // Non-nil only for real errors (not file write warnings).
	Saved bool   // True if config file was written successfully.
}

// ConfigWriter writes the config file after successful apply.
type ConfigWriter func() error

// RestartFunc restarts a broken plugin via the 5-stage protocol.
type RestartFunc func(pluginName string) error

// OperationPlanRequest is the input to operation planning after full verify.
type OperationPlanRequest struct {
	TransactionID string
	Diffs         map[string][]DiffSection
}

// OperationPlanner returns ordering-sensitive operations for a transaction.
type OperationPlanner func(context.Context, OperationPlanRequest) ([]ConfigOperation, error)

// TxCoordinator coordinates a single config transaction across participants.
// One TxCoordinator instance per transaction. Not reusable.
type TxCoordinator struct {
	gateway          EventGateway
	participants     []Participant
	restartFn        RestartFunc
	configWriter     ConfigWriter
	operationPlanner OperationPlanner
	txID             string

	// Deadline overrides for testing.
	verifyDeadlineOverride time.Duration
	applyDeadlineOverride  time.Duration

	// Computed apply deadline (the sum of the participants' budgets,
	// orchestrator_budget.go).
	applyDeadline time.Duration

	// Ack collection.
	mu           sync.Mutex
	verifyAcks   map[string]VerifyAck
	applyAcks    map[string]ApplyAck
	rollbackAcks map[string]RollbackAck

	// Channels for ack notification.
	verifyOKCh     chan VerifyAck
	verifyFailedCh chan VerifyAck
	applyOKCh      chan ApplyAck
	applyFailedCh  chan ApplyAck
	rollbackOKCh   chan RollbackAck

	// Stored unsubscribe functions for cleanup.
	unsubs []func()

	// Number of participants that received verify/apply events (have diffs).
	activeCount int
}

// NewTxCoordinator creates a transaction coordinator.
//
// gateway is the orchestrator's view of the stream event system; the Server
// in internal/component/plugin/server provides a ConfigEventGateway adapter
// that satisfies it. gateway MUST NOT be nil.
//
// All participant names MUST satisfy ValidatePluginName: a participant whose
// name is reserved (ok, failed, abort) would cause the per-plugin event types
// produced by EventVerifyFor/EventApplyFor to collide with broadcast or ack
// event types, silently breaking the transaction. The constructor rejects
// such participants with an error. See topics.go ReservedPluginNames for
// the reserved set.
//
// restartFn may be nil if broken-plugin recovery is not needed.
func NewTxCoordinator(gateway EventGateway, participants []Participant, restartFn RestartFunc) (*TxCoordinator, error) {
	if gateway == nil {
		return nil, errors.New("NewTxCoordinator: gateway must not be nil")
	}
	for _, p := range participants {
		if err := ValidatePluginName(p.Name); err != nil {
			return nil, fmt.Errorf("NewTxCoordinator: participant %q: %w", p.Name, err)
		}
	}
	return &TxCoordinator{
		gateway:        gateway,
		participants:   participants,
		restartFn:      restartFn,
		txID:           textbuf.StrInt("tx-", time.Now().UnixNano()),
		verifyAcks:     make(map[string]VerifyAck),
		applyAcks:      make(map[string]ApplyAck),
		rollbackAcks:   make(map[string]RollbackAck),
		verifyOKCh:     make(chan VerifyAck, len(participants)),
		verifyFailedCh: make(chan VerifyAck, len(participants)),
		applyOKCh:      make(chan ApplyAck, len(participants)),
		applyFailedCh:  make(chan ApplyAck, len(participants)),
		rollbackOKCh:   make(chan RollbackAck, len(participants)),
	}, nil
}

// TransactionID returns the unique ID for this transaction.
func (o *TxCoordinator) TransactionID() string { return o.txID }

// ApplyDeadline returns the computed apply deadline duration.
func (o *TxCoordinator) ApplyDeadline() time.Duration {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.applyDeadline
}

// SetVerifyDeadline overrides the verify deadline (for testing).
func (o *TxCoordinator) SetVerifyDeadline(d time.Duration) { o.verifyDeadlineOverride = d }

// SetApplyDeadlineOverride overrides the apply deadline (for testing).
func (o *TxCoordinator) SetApplyDeadlineOverride(d time.Duration) { o.applyDeadlineOverride = d }

// SetConfigWriter sets the function to write the config file after apply.
func (o *TxCoordinator) SetConfigWriter(fn ConfigWriter) { o.configWriter = fn }

// SetOperationPlanner sets the operation planner used after full-config verify.
func (o *TxCoordinator) SetOperationPlanner(fn OperationPlanner) { o.operationPlanner = fn }

// ParticipantBudgets returns the current budgets for a participant.
func (o *TxCoordinator) ParticipantBudgets(name string) Participant {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, p := range o.participants {
		if p.Name == name {
			return p
		}
	}
	return Participant{}
}

// Execute runs the full transaction: verify -> apply -> commit (or rollback).
// Blocks until the transaction completes or the context is canceled.
func (o *TxCoordinator) Execute(ctx context.Context, diffs map[string][]DiffSection) *TxResult {
	o.subscribeAcks()
	defer o.unsubscribeAcks()

	o.activeCount = o.activeParticipantCount(diffs)

	// Phase 1: Verify.
	if err := o.runVerify(ctx, diffs); err != nil {
		if canceledByShutdown(ctx) {
			return o.abortForShutdown("verify", err)
		}
		o.publishAbort(err.Error())
		return &TxResult{State: StateAborted, Err: err}
	}

	// Phase 2, ordered: every participant with a diff is a node in the
	// operation graph, so the ordered path runs whenever a planner is
	// installed. A participant the planner produced no operation for is
	// carried by a coarse section-apply node (operationNodes), which is what
	// makes the coverage unconditional: the path used to be abandoned for the
	// whole transaction as soon as one participant could not be decomposed,
	// and the operations that WERE ordered lost their order with it.
	if o.operationPlanner != nil {
		ops, err := o.operationPlanner(ctx, OperationPlanRequest{TransactionID: o.txID, Diffs: diffs})
		if err != nil {
			if canceledByShutdown(ctx) {
				return o.abortForShutdown("operation planning", err)
			}
			o.publishAbort(err.Error())
			return &TxResult{State: StateAborted, Err: err}
		}
		// The verb and the declared resources are what the graph orders by,
		// so an operation missing either cannot be placed. It aborts the
		// transaction here, before anything is applied, rather than being
		// ordered as if it depended on nothing.
		if err := ValidateOperations(ops); err != nil {
			o.publishAbort(err.Error())
			return &TxResult{State: StateAborted, Err: err}
		}
		// A participant that decomposed one of its roots and not another is
		// covered as far as the synthesis below can see, and the root it left
		// would reach nothing while this transaction committed.
		if err := o.checkOperationRootCoverage(ops, diffs); err != nil {
			o.publishAbort(err.Error())
			return &TxResult{State: StateAborted, Err: err}
		}
		return o.runOperationPath(ctx, o.operationNodes(ops, diffs), diffs)
	}

	// Phase 2, unordered: the section apply, which is the path when no
	// operation planner is installed.
	if err := o.runApply(ctx, diffs); err != nil {
		if canceledByShutdown(ctx) {
			return o.abortForShutdown("apply", err)
		}
		o.publishRollback(err.Error())
		o.collectRollbackAcks(ctx)
		return &TxResult{State: StateRolledBack, Err: err}
	}

	// Phase 3: Commit.
	o.publishCommitted()
	saved := o.writeConfigFile()
	o.publishApplied(saved)

	return &TxResult{State: StateCommitted, Saved: saved}
}

// subscribeAcks registers engine handlers for all config ack event types.
// Handlers parse the payload, filter by transaction ID, and push the ack
// onto the appropriate channel for the orchestrator's main loop.
//
// Channel sends are NON-BLOCKING (see trySendVerifyAck/Apply/Rollback).
// Engine handlers run synchronously inside the publisher's goroutine
// (see Server.dispatchEngineEvent), so a blocked send would block whoever
// emitted the event. Each ack channel is sized for one ack per active
// participant; if a plugin sends more acks than expected (duplicate, retry,
// malicious) the excess is dropped with a warning log instead of stalling
// the emitter.
//
// applyOKCh is read by runApply alone, so on the ordered path a coarse node's
// section ack lands in it and stays there. That is the design and not a leak.
// The executor subscribes to the same event for the node it is waiting on
// (OperationExecutor.Execute), so the ack is consumed where the wait is; and
// the buffer holds one per participant, which is the most that can arrive,
// because a participant gets at most one coarse node. What the undrained copy
// carries is a budget refresh nothing would read: the bridge fills both budget
// fields of an ack from the plugin's registration (registrationVerifyBudget,
// config_tx_bridge.go), which is where buildTxInputs read them, and this
// coordinator is discarded with its transaction. The verify ack is the one
// that has a reader, and runVerify drains it on both paths.
func (o *TxCoordinator) subscribeAcks() {
	subscribeVerifyAck := func(eventType string, ch chan<- VerifyAck) {
		unsub := o.gateway.SubscribeConfigEvent(eventType, func(payload []byte) {
			var ack VerifyAck
			if err := json.Unmarshal(payload, &ack); err != nil {
				return
			}
			if ack.TransactionID != o.txID {
				return
			}
			trySendVerifyAck(ch, ack, o.txID, eventType)
		})
		o.unsubs = append(o.unsubs, unsub)
	}

	subscribeApplyAck := func(eventType string, ch chan<- ApplyAck) {
		unsub := o.gateway.SubscribeConfigEvent(eventType, func(payload []byte) {
			var ack ApplyAck
			if err := json.Unmarshal(payload, &ack); err != nil {
				return
			}
			if ack.TransactionID != o.txID {
				return
			}
			trySendApplyAck(ch, ack, o.txID, eventType)
		})
		o.unsubs = append(o.unsubs, unsub)
	}

	subscribeVerifyAck(EventVerifyOK, o.verifyOKCh)
	subscribeVerifyAck(EventVerifyFailed, o.verifyFailedCh)
	subscribeApplyAck(EventApplyOK, o.applyOKCh)
	subscribeApplyAck(EventApplyFailed, o.applyFailedCh)

	o.unsubs = append(o.unsubs, o.gateway.SubscribeConfigEvent(EventRollbackOK, func(payload []byte) {
		var ack RollbackAck
		if err := json.Unmarshal(payload, &ack); err != nil {
			return
		}
		if ack.TransactionID != o.txID {
			return
		}
		trySendRollbackAck(o.rollbackOKCh, ack, o.txID)
	}))
}

func trySendVerifyAck(ch chan<- VerifyAck, ack VerifyAck, txID, eventType string) {
	select {
	case ch <- ack:
	default:
		logger().Warn("ack channel full, dropping verify ack",
			"tx", txID, "plugin", ack.Plugin, "event-type", eventType)
	}
}

func trySendApplyAck(ch chan<- ApplyAck, ack ApplyAck, txID, eventType string) {
	select {
	case ch <- ack:
	default:
		logger().Warn("ack channel full, dropping apply ack",
			"tx", txID, "plugin", ack.Plugin, "event-type", eventType)
	}
}

func trySendRollbackAck(ch chan<- RollbackAck, ack RollbackAck, txID string) {
	select {
	case ch <- ack:
	default:
		logger().Warn("ack channel full, dropping rollback ack",
			"tx", txID, "plugin", ack.Plugin, "event-type", EventRollbackOK)
	}
}

// runVerify publishes verify events and collects acks.
func (o *TxCoordinator) runVerify(ctx context.Context, diffs map[string][]DiffSection) error {
	deadline := o.computeVerifyDeadline()
	deadlineMS := time.Now().Add(deadline).UnixMilli()

	for _, p := range o.participants {
		pluginDiffs := o.filterDiffs(diffs, p)
		if len(pluginDiffs) == 0 {
			continue
		}
		ev := VerifyEvent{
			TransactionID: o.txID,
			Diffs:         pluginDiffs,
			DeadlineMS:    deadlineMS,
		}
		payload, err := json.Marshal(ev)
		if err != nil {
			return fmt.Errorf("marshal verify event for %s: %w", p.Name, err)
		}
		if _, err := o.gateway.EmitConfigEvent(EventVerifyFor(p.Name), payload); err != nil {
			return fmt.Errorf("emit verify event for %s: %w", p.Name, err)
		}
	}

	timer := time.NewTimer(deadline)
	defer timer.Stop()

	remaining := o.activeParticipantCount(diffs)
	for remaining > 0 {
		select {
		case ack := <-o.verifyOKCh:
			o.mu.Lock()
			o.verifyAcks[ack.Plugin] = ack
			if ack.ApplyBudgetSecs > 0 {
				o.updateParticipantApplyBudget(ack.Plugin, ack.ApplyBudgetSecs)
			}
			o.mu.Unlock()
			remaining--
		case ack := <-o.verifyFailedCh:
			o.mu.Lock()
			o.verifyAcks[ack.Plugin] = ack
			o.mu.Unlock()
			return fmt.Errorf("plugin %s verify failed: %s", ack.Plugin, ack.Error)
		case <-timer.C:
			return fmt.Errorf("verify timeout after %v", deadline)
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// runApply publishes apply events and collects acks.
func (o *TxCoordinator) runApply(ctx context.Context, diffs map[string][]DiffSection) error {
	o.mu.Lock()
	o.applyDeadline = o.computeApplyDeadline()
	deadline := o.applyDeadline
	o.mu.Unlock()

	deadlineMS := time.Now().Add(deadline).UnixMilli()

	for _, p := range o.participants {
		pluginDiffs := o.filterDiffs(diffs, p)
		if len(pluginDiffs) == 0 {
			continue
		}
		if err := emitSectionApply(o.gateway, o.txID, p.Name, pluginDiffs, deadlineMS); err != nil {
			return err
		}
	}

	timer := time.NewTimer(deadline)
	defer timer.Stop()

	remaining := o.activeParticipantCount(diffs)
	for remaining > 0 {
		select {
		case ack := <-o.applyOKCh:
			o.mu.Lock()
			o.applyAcks[ack.Plugin] = ack
			if ack.VerifyBudgetSecs > 0 {
				o.updateParticipantVerifyBudget(ack.Plugin, ack.VerifyBudgetSecs)
			}
			if ack.ApplyBudgetSecs > 0 {
				o.updateParticipantApplyBudget(ack.Plugin, ack.ApplyBudgetSecs)
			}
			o.mu.Unlock()
			remaining--
		case ack := <-o.applyFailedCh:
			o.mu.Lock()
			o.applyAcks[ack.Plugin] = ack
			o.mu.Unlock()
			return fmt.Errorf("plugin %s apply failed: %s", ack.Plugin, ack.Error)
		case <-timer.C:
			return fmt.Errorf("apply timeout after %v", deadline)
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// emitSectionApply publishes one participant's section apply event. The
// orchestrator's own apply phase and the executor's coarse node both call it,
// so a coarse node carries the payload the section apply carries: same
// participant, same diffs, same deadline.
func emitSectionApply(gateway EventGateway, txID, name string, diffs []DiffSection, deadlineMS int64) error {
	payload, err := json.Marshal(ApplyEvent{TransactionID: txID, Diffs: diffs, DeadlineMS: deadlineMS})
	if err != nil {
		return fmt.Errorf("marshal apply event for %s: %w", name, err)
	}
	if _, err := gateway.EmitConfigEvent(EventApplyFor(name), payload); err != nil {
		return fmt.Errorf("emit apply event for %s: %w", name, err)
	}
	return nil
}

// operationNodes returns every node the operation graph carries: the
// operations the planner produced, plus one coarse section-apply node for each
// participant that has diffs and owns none of them.
//
// Coverage is what makes the ordering unconditional. A participant with no
// node is reached by no phase of the operation path, so it would be verified
// and never applied, and its config change would be discarded while the
// transaction reported success.
//
// A coarse node carries no Root and no Target on purpose. It stands for one
// PARTICIPANT, which receives one section apply carrying every root it
// declared, so no single root names it; and it has no resource identity to
// order by, so no constraint rule matches it and the graph gives it no edge.
//
// Its position is therefore not decided here. The solver places it between the
// addresses this commit adds and the starts that bind them, because the core
// cannot tell whether a root with no operations binds one and the fail-safe
// default says it does (placeSectionNodes in solver.go). The order this
// function appends them in decides only the order of two coarse nodes against
// each other.
func (o *TxCoordinator) operationNodes(ops []ConfigOperation, diffs map[string][]DiffSection) []ConfigOperation {
	uncovered := o.participantsWithoutOperations(ops, diffs)
	nodes := make([]ConfigOperation, 0, len(ops)+len(uncovered))
	nodes = append(nodes, ops...)
	for _, name := range uncovered {
		var tb textbuf.Buffer
		nodes = append(nodes, ConfigOperation{
			ID:    tb.Str(string(OperationSectionApply)).Byte('-').Str(name).String(),
			Owner: name,
			Type:  OperationSectionApply,
		})
	}
	return nodes
}

// sectionDiffsFor returns the diff sections one participant receives through
// the section apply, and reports whether that participant has any. The
// executor installs it for the coarse nodes, so filterDiffs stays the single
// predicate deciding what a participant is sent, whichever path sends it.
func (o *TxCoordinator) sectionDiffsFor(diffs map[string][]DiffSection) SectionDiffs {
	return func(name string) ([]DiffSection, bool) {
		for _, p := range o.participants {
			if p.Name != name {
				continue
			}
			sections := o.filterDiffs(diffs, p)
			return sections, len(sections) > 0
		}
		return nil, false
	}
}

func (o *TxCoordinator) runOperationPath(ctx context.Context, ops []ConfigOperation, diffs map[string][]DiffSection) *TxResult {
	graph, err := BuildOperationGraph(ops, ConstraintRules())
	if err != nil {
		o.publishAbort(err.Error())
		return &TxResult{State: StateAborted, Err: err}
	}
	sorted, err := TopologicalSort(graph)
	if err != nil {
		o.publishAbort(err.Error())
		return &TxResult{State: StateAborted, Err: err}
	}
	executor := NewOperationExecutor(o.gateway, o.txID)
	executor.SetSectionDiffs(o.sectionDiffsFor(diffs))
	o.mu.Lock()
	o.applyDeadline = o.computeApplyDeadline()
	deadline := o.applyDeadline
	o.mu.Unlock()
	executor.SetDeadlineMS(time.Now().Add(deadline).UnixMilli())
	if err := executor.Verify(ctx, sorted); err != nil {
		if canceledByShutdown(ctx) {
			return o.abortForShutdown("operation verify", err)
		}
		o.publishAbort(err.Error())
		return &TxResult{State: StateAborted, Err: err}
	}
	if err := executor.Execute(ctx, sorted); err != nil {
		if canceledByShutdown(ctx) {
			return o.abortForShutdown("operation apply", err)
		}
		o.publishRollback(err.Error())
		o.collectRollbackAcks(ctx)
		return &TxResult{State: StateRolledBack, Err: err}
	}
	if err := executor.Commit(ctx, sorted); err != nil {
		if canceledByShutdown(ctx) {
			return o.abortForShutdown("operation commit", err)
		}
		o.publishRollback(err.Error())
		o.collectRollbackAcks(ctx)
		return &TxResult{State: StateRolledBack, Err: err}
	}
	o.publishCommitted()
	saved := o.writeConfigFile()
	o.publishApplied(saved)
	return &TxResult{State: StateCommitted, Saved: saved}
}

// filterDiffs returns only the diffs relevant to a participant.
// Deduplicates roots that appear in both ConfigRoots and WantsConfig.
func (o *TxCoordinator) filterDiffs(allDiffs map[string][]DiffSection, p Participant) []DiffSection {
	seen := make(map[string]bool, len(p.ConfigRoots)+len(p.WantsConfig))
	var result []DiffSection
	for _, root := range p.ConfigRoots {
		if sections, ok := allDiffs[root]; ok && !seen[root] {
			seen[root] = true
			result = append(result, sections...)
		}
	}
	for _, root := range p.WantsConfig {
		if sections, ok := allDiffs[root]; ok && !seen[root] {
			seen[root] = true
			result = append(result, sections...)
		}
	}
	return result
}

// participantsWithoutOperations returns the names of participants that have
// diffs to apply but own none of ops, sorted so the synthesized nodes are the
// same on every run.
//
// It is the input to coarse-node synthesis (operationNodes). runVerify,
// runApply and this function all decide "does this participant take part" with
// the same filterDiffs predicate, so a participant can never be verified by
// one and skipped by another.
//
// The check is per-PARTICIPANT rather than per-root because that is the
// granularity the apply events use: one participant receives one section apply
// carrying every root it declared. So a decomposer MUST be all-or-nothing for
// a root it claims. `interface` covers its whole root: an address and an
// interface become one operation each, and every key it has no primitive for
// rides the configure operation it closes with (decomposeIfaceOperations in
// internal/component/iface/operation.go). `bgp` returns none unless the diff
// touches a peer (bgpDiffTouchesPeer). A decomposer that instead emitted
// operations covering only PART of its root's diff would make its participant
// look covered, and the remainder would reach nothing: that is a defect in the
// decomposer, and this is the contract it must meet.
//
// `interface` used to answer nothing at all when any key in its diff was one
// it had no primitive for, which met this contract and lost the address
// ordering with it: a commit that edited an MTU and moved an address emitted
// no address operation, so the core read no disturbance and every binder bound
// to that address stayed up while the section apply moved it.
//
// The names come back in PARTICIPANT order, which buildTxInputs
// (internal/component/plugin/server/reload_tx.go) already makes deterministic.
// It decides only how two coarse nodes sit against each other: where they sit
// against the decomposed operations is placeSectionNodes (solver.go), which
// puts them after the addresses this commit adds and before the starts that
// bind them. Sorting the names here would be deterministic and would say
// nothing, because the order this function returns is not the applied order.
func (o *TxCoordinator) participantsWithoutOperations(ops []ConfigOperation, diffs map[string][]DiffSection) []string {
	owners := make(map[string]struct{}, len(ops))
	for i := range ops {
		if ops[i].Owner != "" {
			owners[ops[i].Owner] = struct{}{}
		}
	}
	var uncovered []string
	for _, p := range o.participants {
		if len(o.filterDiffs(diffs, p)) == 0 {
			continue
		}
		if _, ok := owners[p.Name]; !ok {
			uncovered = append(uncovered, p.Name)
		}
	}
	return uncovered
}

// ErrParticipantRootUncovered reports a participant that owns operations for
// one of the roots it has diffs on and none for another.
var ErrParticipantRootUncovered = errors.New("participant decomposes one of its roots and leaves another unapplied")

// checkOperationRootCoverage refuses a transaction in which a participant owns
// an operation for one root it has diffs on and no operation for another.
//
// It is a GUARD, and it fails closed. Coarse-node synthesis asks whether a
// participant owns ANY operation (participantsWithoutOperations), because one
// participant receives one section apply carrying every root it declared. A
// participant that decomposes root A therefore reads as covered, and its diff
// on root B reaches no phase: no operation carries it, no coarse node stands
// for it, and the transaction commits over a change nothing applied.
//
// No first-party participant can reach it today. The two that decompose
// declare one root each (internal/component/iface/register.go,
// internal/component/bgp/plugin/register.go), and neither declares the `*`
// wildcard expandWildcardRoots reads. A silently discarded root is not a thing
// to wait for a caller to reach (ai/rules/principles.md), so the second root
// aborts the transaction here, before anything is applied, and names the
// plugin and the root rather than the config values the operation carries.
//
// The remedy for a plugin that hits it is to decompose every root it declares,
// or none of them: the all-or-nothing contract on participantsWithoutOperations
// stated for one root, stated across them.
func (o *TxCoordinator) checkOperationRootCoverage(ops []ConfigOperation, diffs map[string][]DiffSection) error {
	rootsByOwner := make(map[string]map[string]struct{}, len(ops))
	for i := range ops {
		owner := ops[i].Owner
		if owner == "" {
			continue
		}
		if rootsByOwner[owner] == nil {
			rootsByOwner[owner] = make(map[string]struct{}, 1)
		}
		rootsByOwner[owner][ops[i].Root] = struct{}{}
	}

	for _, p := range o.participants {
		owned := rootsByOwner[p.Name]
		if len(owned) == 0 {
			continue
		}
		for _, section := range o.filterDiffs(diffs, p) {
			if _, ok := owned[section.Root]; ok {
				continue
			}
			return fmt.Errorf("%w: plugin %s, root %s", ErrParticipantRootUncovered, p.Name, section.Root)
		}
	}
	return nil
}

// activeParticipantCount returns how many participants have diffs to process.
func (o *TxCoordinator) activeParticipantCount(diffs map[string][]DiffSection) int {
	count := 0
	for _, p := range o.participants {
		if len(o.filterDiffs(diffs, p)) > 0 {
			count++
		}
	}
	return count
}

func (o *TxCoordinator) publishAbort(reason string) {
	ev := AbortEvent{TransactionID: o.txID, Reason: reason}
	payload, err := json.Marshal(ev)
	if err != nil {
		logger().Error("marshal abort event", "error", err)
		return
	}
	if _, err := o.gateway.EmitConfigEvent(EventVerifyAbort, payload); err != nil {
		logger().Error("emit verify-abort event", "error", err)
	}
	// Surface the abort on the operational report bus so `ze show errors`
	// reflects commit failures alongside BGP events. The bus is a separate
	// concern from the transaction event stream; this call is additive.
	report.RaiseError(
		reportSourceConfig,
		reportCodeCommitAborted,
		o.txID,
		"config commit aborted during verify: "+reason,
		map[string]any{"reason": reason, reportKeyPhase: "verify"},
	)
}

// abortForShutdown ends the transaction and emits no event. The returned
// error wraps ErrShutdown, so the reload caller can tell a canceled
// transaction from a failed one and report it as such.
func (o *TxCoordinator) abortForShutdown(phase string, err error) *TxResult {
	logger().Info("config transaction canceled by shutdown",
		"tx", o.txID, "phase", phase, "error", err)
	return &TxResult{
		State: StateAborted,
		Err:   fmt.Errorf("%w during %s: %w", ErrShutdown, phase, err),
	}
}

func (o *TxCoordinator) publishRollback(reason string) {
	ev := RollbackEvent{TransactionID: o.txID, Reason: reason}
	payload, err := json.Marshal(ev)
	if err != nil {
		logger().Error("marshal rollback event", "error", err)
		return
	}
	if _, err := o.gateway.EmitConfigEvent(EventRollback, payload); err != nil {
		logger().Error("emit rollback event", "error", err)
	}
	report.RaiseError(
		reportSourceConfig,
		reportCodeCommitRollback,
		o.txID,
		"config commit rolled back during apply: "+reason,
		map[string]any{"reason": reason, reportKeyPhase: "apply"},
	)
}

func (o *TxCoordinator) publishCommitted() {
	ev := CommittedEvent{TransactionID: o.txID}
	payload, err := json.Marshal(ev)
	if err != nil {
		logger().Error("marshal committed event", "error", err)
		return
	}
	if _, err := o.gateway.EmitConfigEvent(EventCommitted, payload); err != nil {
		logger().Error("emit committed event", "error", err)
	}
}

func (o *TxCoordinator) publishApplied(saved bool) {
	ev := AppliedEvent{TransactionID: o.txID, Saved: saved}
	payload, err := json.Marshal(ev)
	if err != nil {
		logger().Error("marshal applied event", "error", err)
		return
	}
	if _, err := o.gateway.EmitConfigEvent(EventApplied, payload); err != nil {
		logger().Error("emit applied event", "error", err)
	}
}

// collectRollbackAcks drains rollback acks in reverse dependency-tier order.
// Plugins that depend on others must complete rollback before their
// dependencies, so the orchestrator processes ack buckets from the deepest
// tier (dependents) back to the lowest tier (roots). A plugin reporting
// CodeBroken is restarted before the next tier is drained, so tier k-1
// never starts draining while tier k still has a plugin mid-restart.
//
// Acks that arrive from a tier not yet being drained are buffered in
// `pending` and consumed when that tier's turn comes.
//
// When plugins are unregistered (as in most unit tests), tierFn returns a
// single tier containing all participants; the loop degenerates to a single
// drain equivalent to the pre-tier behavior.
func (o *TxCoordinator) collectRollbackAcks(ctx context.Context) {
	deadline := o.computeRollbackDeadline()
	timer := time.NewTimer(deadline)
	defer timer.Stop()

	participantNames := make([]string, 0, len(o.participants))
	nameToIndex := make(map[string]int, len(o.participants))
	for i, p := range o.participants {
		participantNames = append(participantNames, p.Name)
		nameToIndex[p.Name] = i
	}

	tiers, err := tierFn(participantNames)
	if err != nil {
		logger().Warn("tier computation failed, draining rollback acks in single tier",
			"error", err)
		tiers = [][]string{participantNames}
	}

	pending := make(map[string]RollbackAck)

	for i, tier := range slices.Backward(tiers) {
		expected := make(map[string]struct{})
		for _, name := range tier {
			if _, ok := nameToIndex[name]; ok {
				expected[name] = struct{}{}
			}
		}
		if len(expected) == 0 {
			continue
		}

		for name, ack := range pending {
			if _, ok := expected[name]; ok {
				o.handleRollbackAck(ack)
				delete(pending, name)
				delete(expected, name)
			}
		}

		for len(expected) > 0 {
			select {
			case ack := <-o.rollbackOKCh:
				if _, ok := expected[ack.Plugin]; ok {
					o.handleRollbackAck(ack)
					delete(expected, ack.Plugin)
				} else {
					pending[ack.Plugin] = ack
				}
			case <-timer.C:
				logger().Warn("rollback ack timeout",
					"tier", i, "remaining", len(expected))
				return
			case <-ctx.Done():
				return
			}
		}
	}
}

// handleRollbackAck records a single rollback ack and restarts the plugin
// if the ack reports CodeBroken. Extracted from the tiered drain loop so
// the loop stays readable.
func (o *TxCoordinator) handleRollbackAck(ack RollbackAck) {
	o.mu.Lock()
	o.rollbackAcks[ack.Plugin] = ack
	o.mu.Unlock()
	if ack.Code == CodeBroken && o.restartFn != nil {
		logger().Warn("plugin broken during rollback, restarting", "plugin", ack.Plugin)
		if err := o.restartFn(ack.Plugin); err != nil {
			logger().Error("failed to restart broken plugin", "plugin", ack.Plugin, "error", err)
		}
	}
}

func (o *TxCoordinator) writeConfigFile() bool {
	if o.configWriter == nil {
		return true
	}
	if err := o.configWriter(); err != nil {
		logger().Warn("config file write failed (runtime is live)", "error", err)
		// Raise a commit-save-failed error on the report bus so operators
		// see it via `ze show errors`. Apply succeeded (runtime is live),
		// but the config file on disk is out of sync with the running state.
		report.RaiseError(
			reportSourceConfig,
			reportCodeCommitSaveFail,
			o.txID,
			"config commit succeeded but file write failed: "+err.Error(),
			map[string]any{"error": err.Error(), reportKeyPhase: "save"},
		)
		return false
	}
	return true
}

// unsubscribeAcks calls each unsubscribe function recorded by subscribeAcks.
func (o *TxCoordinator) unsubscribeAcks() {
	for _, unsub := range o.unsubs {
		unsub()
	}
	o.unsubs = nil
}
