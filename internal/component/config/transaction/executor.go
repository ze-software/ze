// Design: docs/architecture/config/apply-ordering.md -- ordered operation execution
// Related: gateway.go -- event stream used to reach plugin callbacks

package transaction

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// OperationExecutor applies sorted operations through config operation events.
type OperationExecutor struct {
	gateway    EventGateway
	txID       string
	deadlineMS int64
}

// NewOperationExecutor creates an executor for one transaction.
func NewOperationExecutor(gateway EventGateway, txID string) *OperationExecutor {
	return &OperationExecutor{gateway: gateway, txID: txID}
}

// SetDeadlineMS sets the Unix-millis deadline propagated to per-operation events.
func (e *OperationExecutor) SetDeadlineMS(ms int64) { e.deadlineMS = ms }

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
	applyFilter := &ackFilter{}
	rollbackFilter := &ackFilter{}
	unsubs := []func(){
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

// Commit asks each operation owner to finalize its operation journal.
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

func (e *OperationExecutor) rollbackApplied(ctx context.Context, executed []ConfigOperation, okCh, failedCh <-chan ConfigOperationRollbackAck, filter *ackFilter, cause error) error {
	for i := len(executed) - 1; i >= 0; i-- {
		op := &executed[i]
		if filter != nil {
			filter.set(op.ID)
		}
		payload, err := json.Marshal(ConfigOperationRollbackEvent{TransactionID: e.txID, Operations: []ConfigOperation{*op}, DeadlineMS: e.deadlineMS})
		if err != nil {
			return fmt.Errorf("%w; marshal operation rollback %s: %w", cause, op.ID, err)
		}
		if _, err := e.gateway.EmitConfigEvent(EventOperationRollbackFor(op.Owner), payload); err != nil {
			return fmt.Errorf("%w; emit operation rollback %s: %w", cause, op.ID, err)
		}
		if err := waitRollbackAck(ctx, op.ID, okCh, failedCh); err != nil {
			return fmt.Errorf("%w; rollback %s failed: %w", cause, op.ID, err)
		}
		if filter != nil {
			filter.clear()
		}
	}
	return cause
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
	default:
		return ""
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

func owners(ops []ConfigOperation) []string {
	seen := make(map[string]struct{}, len(ops))
	owners := make([]string, 0, len(ops))
	for i := range ops {
		op := &ops[i]
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
