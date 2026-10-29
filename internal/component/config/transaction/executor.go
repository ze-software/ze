// Design: docs/architecture/config/apply-ordering.md -- ordered operation execution
// Related: gateway.go -- event stream used to reach plugin callbacks

package transaction

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// SectionDiffs returns the diff sections one participant receives through the
// section apply, and reports whether that participant has any. The orchestrator
// installs it so a coarse node applies what the section apply applies.
type SectionDiffs func(name string) ([]DiffSection, bool)

// OperationExecutor applies sorted operations through config operation events.
type OperationExecutor struct {
	gateway        EventGateway
	txID           string
	deadlineMS     int64
	rollbackBudget time.Duration
	sectionDiffs   SectionDiffs
	// The coordinator restores dependent sections after infrastructure undo.
	afterRollback func(context.Context, error) error
}

// NewOperationExecutor creates an executor for one transaction.
func NewOperationExecutor(gateway EventGateway, txID string) *OperationExecutor {
	return &OperationExecutor{gateway: gateway, txID: txID}
}

// SetDeadlineMS sets the Unix-millis deadline propagated to per-operation events.
func (e *OperationExecutor) SetDeadlineMS(ms int64) { e.deadlineMS = ms }

// SetRollbackBudget reserves a fresh interval for undo, independent of apply.
func (e *OperationExecutor) SetRollbackBudget(budget time.Duration) { e.rollbackBudget = budget }

// SetSectionDiffs installs the source of a coarse node's diff sections. A
// sorted list carrying a coarse node MUST have one installed: without it the
// executor cannot say what that participant receives, and it refuses the node
// rather than applying an empty section.
func (e *OperationExecutor) SetSectionDiffs(fn SectionDiffs) { e.sectionDiffs = fn }

// Verify asks operation owners to validate every operation before mutation.
func (e *OperationExecutor) Verify(ctx context.Context, ops []ConfigOperation) error {
	if e == nil || e.gateway == nil {
		return fmt.Errorf("operation executor requires a gateway")
	}
	verifyOKCh := make(chan ConfigOperationVerifyAck, len(ops))
	verifyFailedCh := make(chan ConfigOperationVerifyAck, len(ops))
	verifyFilter := &ackFilter{}
	unsubs := []func(){
		e.gateway.SubscribeConfigEvent(EventOperationVerifyOK, func(payload []byte) {
			var ack ConfigOperationVerifyAck
			if err := json.Unmarshal(payload, &ack); err == nil && ack.TransactionID == e.txID && verifyFilter.match(ack.OperationID) {
				trySendOpVerifyAck(verifyOKCh, ack)
			}
		}),
		e.gateway.SubscribeConfigEvent(EventOperationVerifyFailed, func(payload []byte) {
			var ack ConfigOperationVerifyAck
			if err := json.Unmarshal(payload, &ack); err == nil && ack.TransactionID == e.txID && verifyFilter.match(ack.OperationID) {
				trySendOpVerifyAck(verifyFailedCh, ack)
			}
		}),
	}
	defer unsubscribeAll(unsubs)

	for i := range ops {
		op := &ops[i]
		// A coarse node owes no verify: phase 1 verified the whole candidate
		// config for its participant before the planner ran, and the plugin
		// implements no per-operation verify callback to ask twice with.
		if IsSectionApply(op) {
			continue
		}
		if op.Owner == "" {
			return fmt.Errorf("operation %q has no owner", op.ID)
		}
		verifyFilter.set(op.ID)
		payload, err := json.Marshal(ConfigOperationVerifyEvent{TransactionID: e.txID, Operation: *op, DeadlineMS: e.deadlineMS})
		if err != nil {
			return fmt.Errorf("marshal operation verify %s: %w", op.ID, err)
		}
		if _, err := e.gateway.EmitConfigEvent(EventOperationVerifyFor(op.Owner), payload); err != nil {
			return fmt.Errorf("emit operation verify %s: %w", op.ID, err)
		}
		if err := waitVerifyAck(ctx, op.ID, verifyOKCh, verifyFailedCh); err != nil {
			return err
		}
		verifyFilter.clear()
	}
	return nil
}

// Execute applies operations in the supplied order. On the first failure it
// rolls back previously applied operations in reverse order.
func (e *OperationExecutor) Execute(ctx context.Context, ops []ConfigOperation) error {
	if e == nil || e.gateway == nil {
		return fmt.Errorf("operation executor requires a gateway")
	}
	applyOKCh := make(chan ConfigOperationApplyAck, len(ops))
	applyFailedCh := make(chan ConfigOperationApplyAck, len(ops))
	rollbackOKCh := make(chan ConfigOperationRollbackAck, len(ops))
	rollbackFailedCh := make(chan ConfigOperationRollbackAck, len(ops))
	sectionOKCh := make(chan ApplyAck, len(ops))
	sectionFailedCh := make(chan ApplyAck, len(ops))
	applyFilter := &ackFilter{}
	rollbackFilter := &ackFilter{}
	// A coarse node is applied through the section apply, so its ack is the
	// section ack, which carries the plugin name and no operation id.
	sectionFilter := &ackFilter{}
	unsubs := []func(){
		e.gateway.SubscribeConfigEvent(EventApplyOK, func(payload []byte) {
			var ack ApplyAck
			if err := json.Unmarshal(payload, &ack); err == nil && ack.TransactionID == e.txID && sectionFilter.match(ack.Plugin) {
				trySendSectionApplyAck(sectionOKCh, ack)
			}
		}),
		e.gateway.SubscribeConfigEvent(EventApplyFailed, func(payload []byte) {
			var ack ApplyAck
			if err := json.Unmarshal(payload, &ack); err == nil && ack.TransactionID == e.txID && sectionFilter.match(ack.Plugin) {
				trySendSectionApplyAck(sectionFailedCh, ack)
			}
		}),
		e.gateway.SubscribeConfigEvent(EventOperationApplyOK, func(payload []byte) {
			var ack ConfigOperationApplyAck
			if err := json.Unmarshal(payload, &ack); err == nil && ack.TransactionID == e.txID && applyFilter.match(ack.OperationID) {
				trySendOpApplyAck(applyOKCh, ack)
			}
		}),
		e.gateway.SubscribeConfigEvent(EventOperationApplyFailed, func(payload []byte) {
			var ack ConfigOperationApplyAck
			if err := json.Unmarshal(payload, &ack); err == nil && ack.TransactionID == e.txID && applyFilter.match(ack.OperationID) {
				trySendOpApplyAck(applyFailedCh, ack)
			}
		}),
		e.gateway.SubscribeConfigEvent(EventOperationRollbackOK, func(payload []byte) {
			var ack ConfigOperationRollbackAck
			if err := json.Unmarshal(payload, &ack); err == nil && ack.TransactionID == e.txID && rollbackFilter.match(ack.OperationID) {
				trySendOpRollbackAck(rollbackOKCh, ack)
			}
		}),
		e.gateway.SubscribeConfigEvent(EventOperationRollbackFailed, func(payload []byte) {
			var ack ConfigOperationRollbackAck
			if err := json.Unmarshal(payload, &ack); err == nil && ack.TransactionID == e.txID && rollbackFilter.match(ack.OperationID) {
				trySendOpRollbackAck(rollbackFailedCh, ack)
			}
		}),
	}
	defer unsubscribeAll(unsubs)

	executed := make([]ConfigOperation, 0, len(ops))
	for i := range ops {
		op := &ops[i]
		if IsSectionApply(op) {
			if err := e.applySection(ctx, op, sectionOKCh, sectionFailedCh, sectionFilter); err != nil {
				return e.rollbackApplied(ctx, executed, rollbackOKCh, rollbackFailedCh, rollbackFilter, err)
			}
			executed = append(executed, *op)
			continue
		}
		waiters := e.armSettlementWaiters(op)
		if op.Owner == "" {
			unsubWaiters(waiters)
			err := fmt.Errorf("operation %q has no owner", op.ID)
			return e.rollbackApplied(ctx, executed, rollbackOKCh, rollbackFailedCh, rollbackFilter, err)
		}
		applyFilter.set(op.ID)
		payload, err := json.Marshal(ConfigOperationApplyEvent{TransactionID: e.txID, Operation: *op, DeadlineMS: e.deadlineMS})
		if err != nil {
			unsubWaiters(waiters)
			return e.rollbackApplied(ctx, executed, rollbackOKCh, rollbackFailedCh, rollbackFilter, fmt.Errorf("marshal operation apply %s: %w", op.ID, err))
		}
		if _, err := e.gateway.EmitConfigEvent(EventOperationApplyFor(op.Owner), payload); err != nil {
			unsubWaiters(waiters)
			return e.rollbackApplied(ctx, executed, rollbackOKCh, rollbackFailedCh, rollbackFilter, fmt.Errorf("emit operation apply %s: %w", op.ID, err))
		}
		if err := waitApplyAck(ctx, op.ID, applyOKCh, applyFailedCh); err != nil {
			unsubWaiters(waiters)
			return e.rollbackApplied(ctx, executed, rollbackOKCh, rollbackFailedCh, rollbackFilter, err)
		}
		applyFilter.clear()
		executed = append(executed, *op)
		if err := waitSettlement(ctx, op.ID, waiters); err != nil {
			unsubWaiters(waiters)
			return e.rollbackApplied(ctx, executed, rollbackOKCh, rollbackFailedCh, rollbackFilter, err)
		}
		unsubWaiters(waiters)
	}
	return nil
}

// Commit acknowledges each owner's reversible operation commit. Owners MUST
// retain undo until the whole-reload completion callback accepts the transaction.
func (e *OperationExecutor) Commit(ctx context.Context, ops []ConfigOperation) error {
	if e == nil || e.gateway == nil {
		return fmt.Errorf("operation executor requires a gateway")
	}
	owners := owners(ops)
	commitOKCh := make(chan ConfigOperationCommitAck, len(owners))
	commitFailedCh := make(chan ConfigOperationCommitAck, len(owners))
	commitFilter := &ackFilter{}
	unsubs := []func(){
		e.gateway.SubscribeConfigEvent(EventOperationCommitOK, func(payload []byte) {
			var ack ConfigOperationCommitAck
			if err := json.Unmarshal(payload, &ack); err == nil && ack.TransactionID == e.txID && commitFilter.match(ack.Plugin) {
				trySendOpCommitAck(commitOKCh, ack)
			}
		}),
		e.gateway.SubscribeConfigEvent(EventOperationCommitFailed, func(payload []byte) {
			var ack ConfigOperationCommitAck
			if err := json.Unmarshal(payload, &ack); err == nil && ack.TransactionID == e.txID && commitFilter.match(ack.Plugin) {
				trySendOpCommitAck(commitFailedCh, ack)
			}
		}),
	}
	defer unsubscribeAll(unsubs)

	for _, owner := range owners {
		commitFilter.set(owner)
		payload, err := json.Marshal(ConfigOperationCommitEvent{TransactionID: e.txID, DeadlineMS: e.deadlineMS})
		if err != nil {
			return fmt.Errorf("marshal operation commit %s: %w", owner, err)
		}
		if _, err := e.gateway.EmitConfigEvent(EventOperationCommitFor(owner), payload); err != nil {
			return fmt.Errorf("emit operation commit %s: %w", owner, err)
		}
		if err := waitCommitAck(ctx, owner, commitOKCh, commitFailedCh); err != nil {
			return err
		}
		commitFilter.clear()
	}
	return nil
}

// Rollback undoes the supplied applied operations in reverse order. Callers MUST
// preserve the original order, transaction identity and owner lifetimes. Each
// call reserves its own rollback budget; a schedule that interleaves other work
// between single inverses uses a RollbackDriver instead.
func (e *OperationExecutor) Rollback(ctx context.Context, ops []ConfigOperation, cause error) error {
	driver := e.NewRollbackDriver(len(ops))
	defer driver.Close()
	return e.rollbackApplied(ctx, ops, driver.okCh, driver.failedCh, driver.filter, cause)
}

// RollbackDriver dispatches original-operation inverses of one transaction one
// at a time, through the same acknowledgement path Rollback uses. Every Undo
// runs under the caller's context, so a schedule that starts plugins between
// inverses keeps one deadline instead of renewing a budget per inverse. It is
// not safe for concurrent use. NewRollbackDriver subscribes; the caller MUST
// call Close after the last Undo.
type RollbackDriver struct {
	executor *OperationExecutor
	okCh     chan ConfigOperationRollbackAck
	failedCh chan ConfigOperationRollbackAck
	filter   *ackFilter
	unsubs   []func()
}

// NewRollbackDriver subscribes to this transaction's rollback acknowledgements.
// capacity bounds the acknowledgements buffered between waits. The caller MUST
// call Close.
func (e *OperationExecutor) NewRollbackDriver(capacity int) *RollbackDriver {
	capacity = max(capacity, 1)
	driver := &RollbackDriver{
		executor: e,
		okCh:     make(chan ConfigOperationRollbackAck, capacity),
		failedCh: make(chan ConfigOperationRollbackAck, capacity),
		filter:   &ackFilter{},
	}
	driver.unsubs = []func(){
		e.gateway.SubscribeConfigEvent(EventOperationRollbackOK, func(payload []byte) {
			var ack ConfigOperationRollbackAck
			if json.Unmarshal(payload, &ack) == nil && ack.TransactionID == e.txID && driver.filter.match(ack.OperationID) {
				trySendOpRollbackAck(driver.okCh, ack)
			}
		}),
		e.gateway.SubscribeConfigEvent(EventOperationRollbackFailed, func(payload []byte) {
			var ack ConfigOperationRollbackAck
			if json.Unmarshal(payload, &ack) == nil && ack.TransactionID == e.txID && driver.filter.match(ack.OperationID) {
				trySendOpRollbackAck(driver.failedCh, ack)
			}
		}),
	}
	return driver
}

// Close ends the acknowledgement subscriptions. It MUST be called after the
// last Undo.
func (d *RollbackDriver) Close() { unsubscribeAll(d.unsubs) }

// Undo dispatches the inverse of one applied operation and waits for its
// acknowledgement. ctx MUST carry the deadline the whole schedule shares: the
// owner receives that deadline, and Undo refuses a context without one rather
// than inventing a budget here.
//
// sent reports whether the inverse reached the owner. When it did, the owner's
// journal for it is consumed whatever err says: the owner acknowledged a
// failure, or its acknowledgement did not arrive in time. When it did not
// (the operation has no inverse, the context has no deadline, or the event
// could not be built or emitted), err says why and the inverse is still owed.
func (d *RollbackDriver) Undo(ctx context.Context, op *ConfigOperation) (sent bool, err error) {
	if IsSectionApply(op) {
		return false, fmt.Errorf("operation %s is a coarse section apply and has no original inverse", op.ID)
	}
	deadline, bounded := ctx.Deadline()
	if !bounded {
		return false, fmt.Errorf("rollback of operation %s has no deadline: the caller must bound the whole schedule", op.ID)
	}
	return d.executor.rollbackOne(ctx, op, deadline, d.okCh, d.failedCh, d.filter)
}

// applySection applies one coarse node through the section apply event, the
// same event and payload the orchestrator's own apply phase publishes. The
// participant answers it with the `config-apply` callback every plugin
// implements, and its ack is the section apply ack.
func (e *OperationExecutor) applySection(ctx context.Context, op *ConfigOperation, okCh, failedCh <-chan ApplyAck, filter *ackFilter) error {
	if op.Owner == "" {
		return fmt.Errorf("section apply node %q has no owner", op.ID)
	}
	if e.sectionDiffs == nil {
		return fmt.Errorf("section apply node %q has no diff source: the caller did not call SetSectionDiffs", op.ID)
	}
	diffs, ok := e.sectionDiffs(op.Owner)
	if !ok {
		return fmt.Errorf("section apply node %q: participant %s has no diffs to apply", op.ID, op.Owner)
	}
	filter.set(op.Owner)
	if err := emitSectionApply(e.gateway, e.txID, op.Owner, diffs, e.deadlineMS); err != nil {
		return err
	}
	if err := waitSectionApplyAck(ctx, op.Owner, okCh, failedCh); err != nil {
		return err
	}
	filter.clear()
	return nil
}

func (e *OperationExecutor) rollbackApplied(ctx context.Context, executed []ConfigOperation, okCh, failedCh <-chan ConfigOperationRollbackAck, filter *ackFilter, cause error) error {
	if canceledByShutdown(ctx) {
		return cause
	}
	budget := e.rollbackBudget
	if budget <= 0 {
		budget = 90 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), budget)
	defer cancel()
	deadline, _ := ctx.Deadline()
	for i := len(executed) - 1; i >= 0; i-- {
		op := &executed[i]
		// Coarse sections have no per-operation inverse. The coordinator MUST
		// restore them after original operation undo: an installer may bind
		// to an interface that an inverse recreates.
		if IsSectionApply(op) {
			continue
		}
		// A whole rollback attempts every inverse whether or not an earlier one
		// was sent, so whether this one reached its owner changes nothing here.
		if _, err := e.rollbackOne(ctx, op, deadline, okCh, failedCh, filter); err != nil {
			cause = errors.Join(cause, err)
		}
	}
	if e.afterRollback != nil {
		return e.afterRollback(ctx, cause)
	}
	return cause
}

// rollbackOne sends the inverse of one applied operation with the shared
// deadline and waits for its acknowledgement under ctx. sent reports whether
// the event reached the owner: a marshal or emit failure leaves the inverse
// unsent, and so still owed.
func (e *OperationExecutor) rollbackOne(ctx context.Context, op *ConfigOperation, deadline time.Time, okCh, failedCh <-chan ConfigOperationRollbackAck, filter *ackFilter) (sent bool, err error) {
	if filter != nil {
		filter.set(op.ID)
		defer filter.clear()
	}
	payload, err := json.Marshal(ConfigOperationRollbackEvent{TransactionID: e.txID, Operations: []ConfigOperation{*op}, DeadlineMS: deadline.UnixMilli()})
	if err != nil {
		return false, fmt.Errorf("marshal operation rollback %s: %w", op.ID, err)
	}
	if _, err := e.gateway.EmitConfigEvent(EventOperationRollbackFor(op.Owner), payload); err != nil {
		return false, fmt.Errorf("emit operation rollback %s: %w", op.ID, err)
	}
	if err := waitRollbackAck(ctx, op.ID, okCh, failedCh); err != nil {
		return true, fmt.Errorf("rollback %s failed: %w", op.ID, err)
	}
	return true, nil
}

type ackFilter struct {
	mu    sync.RWMutex
	value string
}

func (f *ackFilter) set(value string) {
	f.mu.Lock()
	f.value = value
	f.mu.Unlock()
}

func (f *ackFilter) clear() {
	f.set("")
}

func (f *ackFilter) match(value string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return value != "" && f.value == value
}

func trySendOpVerifyAck(ch chan<- ConfigOperationVerifyAck, ack ConfigOperationVerifyAck) {
	select {
	case ch <- ack:
	default:
		logger().Warn("operation verify ack channel full, dropping ack", "tx", ack.TransactionID, "operation", ack.OperationID)
	}
}

func trySendOpApplyAck(ch chan<- ConfigOperationApplyAck, ack ConfigOperationApplyAck) {
	select {
	case ch <- ack:
	default:
		logger().Warn("operation apply ack channel full, dropping ack", "tx", ack.TransactionID, "operation", ack.OperationID)
	}
}

func trySendOpRollbackAck(ch chan<- ConfigOperationRollbackAck, ack ConfigOperationRollbackAck) {
	select {
	case ch <- ack:
	default:
		logger().Warn("operation rollback ack channel full, dropping ack", "tx", ack.TransactionID, "operation", ack.OperationID)
	}
}

func trySendSectionApplyAck(ch chan<- ApplyAck, ack ApplyAck) {
	select {
	case ch <- ack:
	default:
		logger().Warn("section apply ack channel full, dropping ack", "tx", ack.TransactionID, "plugin", ack.Plugin)
	}
}

func trySendOpCommitAck(ch chan<- ConfigOperationCommitAck, ack ConfigOperationCommitAck) {
	select {
	case ch <- ack:
	default:
		logger().Warn("operation commit ack channel full, dropping ack", "tx", ack.TransactionID, "plugin", ack.Plugin)
	}
}

func waitApplyAck(ctx context.Context, opID string, okCh, failedCh <-chan ConfigOperationApplyAck) error {
	for {
		select {
		case ack := <-okCh:
			if ack.OperationID != opID {
				continue
			}
			if ack.Status != CodeOK {
				return fmt.Errorf("operation %s apply failed: %s", opID, ack.Error)
			}
			return nil
		case ack := <-failedCh:
			if ack.OperationID != opID {
				continue
			}
			return fmt.Errorf("operation %s apply failed: %s", opID, ack.Error)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// waitSectionApplyAck waits for one participant's section apply ack. The ack
// carries the plugin name rather than an operation id, so the match is on the
// name and a stray ack from another participant is left for its own wait.
func waitSectionApplyAck(ctx context.Context, name string, okCh, failedCh <-chan ApplyAck) error {
	for {
		select {
		case ack := <-okCh:
			if ack.Plugin != name {
				continue
			}
			if ack.Status != CodeOK {
				return fmt.Errorf("section apply for %s failed: %s", name, ack.Error)
			}
			return nil
		case ack := <-failedCh:
			if ack.Plugin != name {
				continue
			}
			return fmt.Errorf("section apply for %s failed: %s", name, ack.Error)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

type settlementWaiter struct {
	readiness ConfigOperationReadiness
	timeout   time.Duration
	done      chan struct{}
	unsub     func()
}

func (e *OperationExecutor) armSettlementWaiters(op *ConfigOperation) []*settlementWaiter {
	rules := SettlementRulesFor(op)
	waiters := make([]*settlementWaiter, 0, len(rules))
	for _, rule := range rules {
		readiness := rule.Readiness
		if readiness.Resource == "" {
			readiness.Resource = settlementResource(op, rule.ResourceFrom)
		}
		if readiness.Resource == "" && rule.ResourceFrom != SettlementResourceNone {
			continue
		}
		waiter := &settlementWaiter{
			readiness: readiness,
			timeout:   rule.Timeout,
			done:      make(chan struct{}, 1),
		}
		waiter.unsub = e.gateway.SubscribeEvent(readiness.Namespace, readiness.EventType, func(payload any) {
			if !payloadMatches(readiness, payload) {
				return
			}
			select {
			case waiter.done <- struct{}{}:
			default:
			}
		})
		waiters = append(waiters, waiter)
	}
	return waiters
}

func settlementResource(op *ConfigOperation, source SettlementResourceSource) string {
	if op == nil {
		return ""
	}
	switch source {
	case SettlementResourceAddress:
		return normalizeAddress(firstNonEmpty(op.Target.Address, op.Params.CIDR, op.Params.Address))
	case SettlementResourceInterface:
		return opIfaceName(op)
	case SettlementResourcePeer:
		return firstNonEmpty(op.Target.Peer, op.Params.Peer, op.Target.Name, op.Params.Name)
	case SettlementResourceNone:
		return ""
	default:
		panic("BUG: unknown settlement resource source")
	}
}

func waitSettlement(ctx context.Context, opID string, waiters []*settlementWaiter) error {
	for _, waiter := range waiters {
		if err := waitOneSettlement(ctx, opID, waiter); err != nil {
			return err
		}
	}
	return nil
}

func waitOneSettlement(ctx context.Context, opID string, waiter *settlementWaiter) error {
	if waiter == nil {
		return nil
	}
	timer := time.NewTimer(waiter.timeout)
	defer timer.Stop()
	select {
	case <-waiter.done:
		return nil
	case <-timer.C:
		return fmt.Errorf("operation %s settlement timeout waiting for %s/%s %s after %v", opID, waiter.readiness.Namespace, waiter.readiness.EventType, waiter.readiness.Resource, waiter.timeout)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func unsubWaiters(waiters []*settlementWaiter) {
	for _, waiter := range waiters {
		if waiter != nil && waiter.unsub != nil {
			waiter.unsub()
		}
	}
}

func payloadMatches(readiness ConfigOperationReadiness, payload any) bool {
	if readiness.Resource == "" {
		return true
	}
	resource := normalizeAddress(readiness.Resource)
	for _, value := range payloadValues(payload) {
		if value == readiness.Resource || normalizeAddress(value) == resource {
			return true
		}
	}
	return false
}

func payloadValues(payload any) []string {
	switch v := payload.(type) {
	case string:
		return payloadStringValues(v)
	case []byte:
		return payloadStringValues(string(v))
	case json.RawMessage:
		return payloadStringValues(string(v))
	case map[string]any:
		return payloadMapValues(v)
	default:
		return nil
	}
}

func payloadStringValues(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed[0] != '{' {
		return []string{value}
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
		return []string{value}
	}
	nested := payloadMapValues(obj)
	values := make([]string, 0, 1+len(nested))
	values = append(values, value)
	return append(values, nested...)
}

func payloadMapValues(obj map[string]any) []string {
	keys := []string{"resource", "address", "name", "interface", "peer"}
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		if value, ok := obj[key].(string); ok && value != "" {
			values = append(values, value)
		}
	}
	return values
}

func waitVerifyAck(ctx context.Context, opID string, okCh, failedCh <-chan ConfigOperationVerifyAck) error {
	for {
		select {
		case ack := <-okCh:
			if ack.OperationID != opID {
				continue
			}
			if ack.Status != CodeOK {
				return fmt.Errorf("operation %s verify failed: %s", opID, ack.Error)
			}
			return nil
		case ack := <-failedCh:
			if ack.OperationID != opID {
				continue
			}
			return fmt.Errorf("operation %s verify failed: %s", opID, ack.Error)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func waitRollbackAck(ctx context.Context, opID string, okCh, failedCh <-chan ConfigOperationRollbackAck) error {
	for {
		select {
		case ack := <-okCh:
			if ack.OperationID != opID {
				continue
			}
			if ack.Status != CodeOK {
				return fmt.Errorf("operation %s rollback failed: %s", opID, ack.Error)
			}
			return nil
		case ack := <-failedCh:
			if ack.OperationID != opID {
				continue
			}
			return fmt.Errorf("operation %s rollback failed: %s", opID, ack.Error)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func waitCommitAck(ctx context.Context, owner string, okCh, failedCh <-chan ConfigOperationCommitAck) error {
	for {
		select {
		case ack := <-okCh:
			if ack.Plugin != owner {
				continue
			}
			if ack.Status != CodeOK {
				return fmt.Errorf("operation commit for %s failed: %s", owner, ack.Error)
			}
			return nil
		case ack := <-failedCh:
			if ack.Plugin != owner {
				continue
			}
			return fmt.Errorf("operation commit for %s failed: %s", owner, ack.Error)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// owners returns the plugins that must acknowledge operation commit, in first
// appearance order. A coarse node's owner is not one of them: it journals
// through the section apply, and the broadcast `committed` event the
// orchestrator publishes is what tells it to discard that journal.
func owners(ops []ConfigOperation) []string {
	seen := make(map[string]struct{}, len(ops))
	owners := make([]string, 0, len(ops))
	for i := range ops {
		op := &ops[i]
		if IsSectionApply(op) {
			continue
		}
		if op.Owner == "" {
			continue
		}
		if _, ok := seen[op.Owner]; ok {
			continue
		}
		seen[op.Owner] = struct{}{}
		owners = append(owners, op.Owner)
	}
	return owners
}

func unsubscribeAll(unsubs []func()) {
	for _, unsub := range unsubs {
		unsub()
	}
}
