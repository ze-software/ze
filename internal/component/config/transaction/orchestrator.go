// Design: docs/architecture/config/transaction-protocol.md -- transaction orchestrator
// Related: topics.go -- bus topic constants
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
	"codeberg.org/thomas-mangin/ze/pkg/ze"
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
	bus          ze.Bus
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

	// Stored subscription handles for cleanup.
	subs []ze.Subscription

	// Number of participants that received verify/apply events (have diffs).
	activeCount int
}

// NewTxCoordinator creates a transaction coordinator.
// restartFn may be nil if broken-plugin recovery is not needed.
func NewTxCoordinator(bus ze.Bus, participants []Participant, restartFn RestartFunc) *TxCoordinator {
	return &TxCoordinator{
		bus:            bus,
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
	}
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

// subscribeAcks registers bus consumers for all ack topics.
func (o *TxCoordinator) subscribeAcks() {
	subscribe := func(topic string, handler func([]ze.Event) error) {
		sub, err := o.bus.Subscribe(topic, nil, consumerFunc(handler))
		if err != nil {
			logger().Error("subscribe failed", "topic", topic, "error", err)
			return
		}
		o.subs = append(o.subs, sub)
	}

	subscribe(TopicAckVerifyOK, func(events []ze.Event) error {
		for _, ev := range events {
			var ack VerifyAck
			if err := json.Unmarshal(ev.Payload, &ack); err != nil {
				continue
			}
			if ack.TransactionID != o.txID {
				continue
			}
			o.verifyOKCh <- ack
		}
		return nil
	})

	subscribe(TopicAckVerifyFailed, func(events []ze.Event) error {
		for _, ev := range events {
			var ack VerifyAck
			if err := json.Unmarshal(ev.Payload, &ack); err != nil {
				continue
			}
			if ack.TransactionID != o.txID {
				continue
			}
			o.verifyFailedCh <- ack
		}
		return nil
	})

	subscribe(TopicAckApplyOK, func(events []ze.Event) error {
		for _, ev := range events {
			var ack ApplyAck
			if err := json.Unmarshal(ev.Payload, &ack); err != nil {
				continue
			}
			if ack.TransactionID != o.txID {
				continue
			}
			o.applyOKCh <- ack
		}
		return nil
	})

	subscribe(TopicAckApplyFailed, func(events []ze.Event) error {
		for _, ev := range events {
			var ack ApplyAck
			if err := json.Unmarshal(ev.Payload, &ack); err != nil {
				continue
			}
			if ack.TransactionID != o.txID {
				continue
			}
			o.applyFailedCh <- ack
		}
		return nil
	})

	subscribe(TopicAckRollbackOK, func(events []ze.Event) error {
		for _, ev := range events {
			var ack RollbackAck
			if err := json.Unmarshal(ev.Payload, &ack); err != nil {
				continue
			}
			if ack.TransactionID != o.txID {
				continue
			}
			o.rollbackOKCh <- ack
		}
		return nil
	})
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
		o.bus.Publish(TopicVerifyFor(p.Name), payload, map[string]string{"plugin": p.Name})
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
		o.bus.Publish(TopicApplyFor(p.Name), payload, map[string]string{"plugin": p.Name})
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
	payload, _ := json.Marshal(ev)
	o.bus.Publish(TopicVerifyAbort, payload, nil)
}

func (o *TxCoordinator) publishRollback(reason string) {
	ev := RollbackEvent{TransactionID: o.txID, Reason: reason}
	payload, _ := json.Marshal(ev)
	o.bus.Publish(TopicRollback, payload, nil)
}

func (o *TxCoordinator) publishCommitted() {
	ev := CommittedEvent{TransactionID: o.txID}
	payload, _ := json.Marshal(ev)
	o.bus.Publish(TopicCommitted, payload, nil)
}

func (o *TxCoordinator) publishApplied(saved bool) {
	ev := AppliedEvent{TransactionID: o.txID, Saved: saved}
	payload, _ := json.Marshal(ev)
	o.bus.Publish(TopicApplied, payload, nil)
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

// unsubscribeAcks removes all bus subscriptions created by subscribeAcks.
func (o *TxCoordinator) unsubscribeAcks() {
	for _, sub := range o.subs {
		o.bus.Unsubscribe(sub)
	}
	o.subs = nil
}

// consumerFunc adapts a function to the ze.Consumer interface.
type consumerFunc func([]ze.Event) error

func (f consumerFunc) Deliver(events []ze.Event) error { return f(events) }
