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
	"sync"
	"time"

	"log/slog"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

func logger() *slog.Logger { return slogutil.Logger("config.transaction") }

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

// Participant describes a plugin participating in a config transaction.
type Participant struct {
	Name         string
	ConfigRoots  []string // Roots this plugin owns.
	WantsConfig  []string // Roots this plugin reads (not owner).
	VerifyBudget int      // Estimated verify seconds.
	ApplyBudget  int      // Estimated apply seconds.
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

// TxCoordinator coordinates a single config transaction across participants.
// One TxCoordinator instance per transaction. Not reusable.
type TxCoordinator struct {
	gateway      EventGateway
	participants []Participant
	restartFn    RestartFunc
	configWriter ConfigWriter
	txID         string

	// Deadline overrides for testing.
	verifyDeadlineOverride time.Duration
	applyDeadlineOverride  time.Duration

	// Computed apply deadline (from max budget across participants).
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
		txID:           fmt.Sprintf("tx-%d", time.Now().UnixNano()),
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
		o.publishAbort(err.Error())
		return &TxResult{State: StateAborted, Err: err}
	}

	// Phase 2: Apply.
	if err := o.runApply(ctx, diffs); err != nil {
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
			if !trySendVerifyAck(ch, ack) {
				logger().Warn("ack channel full, dropping verify ack",
					"tx", o.txID, "plugin", ack.Plugin, "event-type", eventType)
			}
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
			if !trySendApplyAck(ch, ack) {
				logger().Warn("ack channel full, dropping apply ack",
					"tx", o.txID, "plugin", ack.Plugin, "event-type", eventType)
			}
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
		if !trySendRollbackAck(o.rollbackOKCh, ack) {
			logger().Warn("ack channel full, dropping rollback ack",
				"tx", o.txID, "plugin", ack.Plugin, "event-type", EventRollbackOK)
		}
	}))
}

// trySendVerifyAck attempts a non-blocking send onto a VerifyAck channel.
// Returns false if the channel is full.
func trySendVerifyAck(ch chan<- VerifyAck, ack VerifyAck) bool {
	select {
	case ch <- ack:
		return true
	default:
		return false
	}
}

// trySendApplyAck attempts a non-blocking send onto an ApplyAck channel.
// Returns false if the channel is full.
func trySendApplyAck(ch chan<- ApplyAck, ack ApplyAck) bool {
	select {
	case ch <- ack:
		return true
	default:
		return false
	}
}

// trySendRollbackAck attempts a non-blocking send onto a RollbackAck channel.
// Returns false if the channel is full.
func trySendRollbackAck(ch chan<- RollbackAck, ack RollbackAck) bool {
	select {
	case ch <- ack:
		return true
	default:
		return false
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
		ev := ApplyEvent{
			TransactionID: o.txID,
			Diffs:         pluginDiffs,
			DeadlineMS:    deadlineMS,
		}
		payload, err := json.Marshal(ev)
		if err != nil {
			return fmt.Errorf("marshal apply event for %s: %w", p.Name, err)
		}
		if _, err := o.gateway.EmitConfigEvent(EventApplyFor(p.Name), payload); err != nil {
			return fmt.Errorf("emit apply event for %s: %w", p.Name, err)
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

func (o *TxCoordinator) computeVerifyDeadline() time.Duration {
	if o.verifyDeadlineOverride > 0 {
		return o.verifyDeadlineOverride
	}
	maxBudget := 0
	for _, p := range o.participants {
		b := capBudget(p.VerifyBudget)
		if b > maxBudget {
			maxBudget = b
		}
	}
	if maxBudget == 0 {
		return 30 * time.Second
	}
	return time.Duration(maxBudget) * time.Second
}

func (o *TxCoordinator) computeApplyDeadline() time.Duration {
	if o.applyDeadlineOverride > 0 {
		return o.applyDeadlineOverride
	}
	maxBudget := 0
	for _, p := range o.participants {
		b := capBudget(p.ApplyBudget)
		if b > maxBudget {
			maxBudget = b
		}
	}
	if maxBudget == 0 {
		return 30 * time.Second
	}
	return time.Duration(maxBudget) * time.Second
}

// capBudget clamps a budget to MaxBudgetSeconds.
func capBudget(secs int) int {
	if secs > MaxBudgetSeconds {
		return MaxBudgetSeconds
	}
	return secs
}

// updateParticipantApplyBudget updates a participant's apply budget.
// Caller MUST hold o.mu.
func (o *TxCoordinator) updateParticipantApplyBudget(name string, secs int) {
	secs = capBudget(secs)
	for i := range o.participants {
		if o.participants[i].Name == name {
			o.participants[i].ApplyBudget = secs
			return
		}
	}
}

// updateParticipantVerifyBudget updates a participant's verify budget.
// Caller MUST hold o.mu.
func (o *TxCoordinator) updateParticipantVerifyBudget(name string, secs int) {
	secs = capBudget(secs)
	for i := range o.participants {
		if o.participants[i].Name == name {
			o.participants[i].VerifyBudget = secs
			return
		}
	}
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

func (o *TxCoordinator) collectRollbackAcks(ctx context.Context) {
	deadline := o.computeRollbackDeadline()
	timer := time.NewTimer(deadline)
	defer timer.Stop()

	remaining := o.activeCount
	for remaining > 0 {
		select {
		case ack := <-o.rollbackOKCh:
			o.mu.Lock()
			o.rollbackAcks[ack.Plugin] = ack
			o.mu.Unlock()
			if ack.Code == CodeBroken && o.restartFn != nil {
				logger().Warn("plugin broken during rollback, restarting", "plugin", ack.Plugin)
				if err := o.restartFn(ack.Plugin); err != nil {
					logger().Error("failed to restart broken plugin", "plugin", ack.Plugin, "error", err)
				}
			}
			remaining--
		case <-timer.C:
			logger().Warn("rollback ack timeout", "remaining", remaining)
			return
		case <-ctx.Done():
			return
		}
	}
}

func (o *TxCoordinator) computeRollbackDeadline() time.Duration {
	o.mu.Lock()
	applyDL := o.applyDeadline
	o.mu.Unlock()
	if applyDL == 0 {
		return 90 * time.Second
	}
	return 3 * applyDL
}

func (o *TxCoordinator) writeConfigFile() bool {
	if o.configWriter == nil {
		return true
	}
	if err := o.configWriter(); err != nil {
		logger().Warn("config file write failed (runtime is live)", "error", err)
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
