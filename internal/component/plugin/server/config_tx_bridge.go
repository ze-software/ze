// Design: docs/architecture/config/transaction-protocol.md -- engine-side RPC bridge for the stream-based orchestrator
// Related: reload.go -- builds a bridge per reload and feeds the coordinator
// Related: engine_event_gateway.go -- gateway the coordinator publishes on
// Related: ../../config/transaction/orchestrator.go -- emits the per-plugin events the bridge reacts to

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/config/transaction"
	txevents "codeberg.org/thomas-mangin/ze/internal/component/config/transaction/events"
	plugin "codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/ipc"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// configTxBridge translates the transaction orchestrator's per-plugin stream
// events into plugin RPC calls (config-verify, config-apply, config-rollback)
// and feeds the resulting status back onto the stream as ack events. It is a
// thin engine-side adapter between the typed orchestrator state machine and
// the existing plugin SDK callback surface.
//
// One bridge instance is created per transaction in reload.go. Subscribe is
// called once before TxCoordinator.Execute runs so that the handlers are in
// place when the coordinator starts publishing verify-<plugin> events. Close
// unsubscribes and MUST be called when the transaction completes; callers
// typically defer it immediately after construction.
//
// Concurrency model: engine event handlers fire synchronously inside the
// orchestrator's publish goroutine (see Server.dispatchEngineEvent), so the
// bridge's verify/apply handlers block that goroutine while the plugin RPC
// is in flight. This intentionally serializes verify/apply across
// participants in registration order -- reload_tx.go sorts the participant
// list so the "bgp" subsystem applies last, matching the legacy reload.go
// semantic that BGP peer reconciliation sees every other plugin's state
// committed first. A per-RPC deadline derived from the event's DeadlineMS
// field bounds each RPC so a hung plugin cannot stall the transaction past
// the orchestrator's own deadline.
//
// The bridge holds no mutable state beyond the subscription handles and is
// safe for concurrent Close calls from Subscribe/defer paths. Every handler
// only reads immutable fields (server, gateway, verifySections,
// participantNames) and each RPC call is independent.
type configTxBridge struct {
	server  *Server
	gateway *ConfigEventGateway

	// participantNames lists the plugins this bridge handles, in the
	// order the orchestrator will emit verify/apply events. reload_tx.go
	// sorts this list so the "bgp" participant comes last; the bridge
	// treats the slice as opaque but preserves ordering when fanning out
	// rollback.
	participantNames []string

	// verifySections maps plugin name -> full post-change config sections
	// for the roots that plugin declared interest in. The bridge hands
	// these verbatim to conn.SendConfigVerify, matching the legacy
	// reload.go behavior where plugins received the candidate subtree
	// (not the diff) during verify. reload.go computed this via
	// ExtractConfigSubtree + WantsConfigRoots.
	verifySections map[string][]rpc.ConfigSection

	// mu guards unsubs so Close is idempotent even under concurrent calls.
	mu     sync.Mutex
	unsubs []func()
	closed bool
}

// newConfigTxBridge constructs an inactive bridge. Call Subscribe to wire it
// up; the bridge performs no work until Subscribe runs. Participant names MUST
// already satisfy transaction.ValidatePluginName; the orchestrator rejects
// reserved names in its own constructor, so the bridge trusts that check.
//
// verifySections MUST contain one entry per participant, with the full
// candidate subtree sections that plugin should receive during verify. Apply
// data comes from the orchestrator's ApplyEvent payload (which is already
// filtered per participant), so the bridge does not need a separate apply map.
func newConfigTxBridge(s *Server, gw *ConfigEventGateway, participantNames []string, verifySections map[string][]rpc.ConfigSection) *configTxBridge {
	return &configTxBridge{
		server:           s,
		gateway:          gw,
		participantNames: participantNames,
		verifySections:   verifySections,
	}
}

// Subscribe registers engine-side handlers for every per-plugin verify/apply
// event the orchestrator will publish, plus a single handler for the broadcast
// rollback event. Must be called exactly once before TxCoordinator.Execute.
// Returns an error if any per-plugin event type fails to register in the
// plugin event registry (which the stream system validates on emit).
func (b *configTxBridge) Subscribe(ctx context.Context) error {
	// Register per-plugin event types so the orchestrator's emits pass
	// validation. RegisterEventType is idempotent, so re-registering across
	// successive transactions is cheap. Do it outside the subscribe loop so
	// a registration failure fails fast before any handler goes live.
	for _, name := range b.participantNames {
		if err := events.RegisterEventType(txevents.Namespace, transaction.EventVerifyFor(name)); err != nil {
			return fmt.Errorf("register verify event for %s: %w", name, err)
		}
		if err := events.RegisterEventType(txevents.Namespace, transaction.EventApplyFor(name)); err != nil {
			return fmt.Errorf("register apply event for %s: %w", name, err)
		}
	}

	for _, name := range b.participantNames {
		b.subscribePhase(ctx, name, phaseVerify)
		b.subscribePhase(ctx, name, phaseApply)
	}
	b.subscribeRollback(ctx)
	return nil
}

// Close unsubscribes every handler registered by Subscribe. Idempotent:
// a second call is a no-op, which means reload.go can defer it right after
// construction regardless of whether Subscribe succeeded.
func (b *configTxBridge) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for _, unsub := range b.unsubs {
		unsub()
	}
	b.unsubs = nil
}

// addUnsub appends an unsubscribe function under the lock. Used by the
// per-event helpers so Close can reliably reach every handle Subscribe
// registered.
func (b *configTxBridge) addUnsub(unsub func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.unsubs = append(b.unsubs, unsub)
}

// phaseKind distinguishes the verify and apply phases the bridge drives.
// The two phases share almost all their logic (subscribe to a per-plugin
// event, look up the plugin process, dispatch an RPC, emit an ack) so the
// bridge encodes the differences in data (a phaseKind value) rather than
// in two near-identical handler functions.
type phaseKind int

const (
	phaseVerify phaseKind = iota
	phaseApply
)

// eventType returns the per-plugin stream event type for this phase. Used
// by the shared subscribe loop to register the handler under the correct
// "verify-<plugin>" or "apply-<plugin>" key.
func (p phaseKind) eventType(name string) string {
	if p == phaseApply {
		return transaction.EventApplyFor(name)
	}
	return transaction.EventVerifyFor(name)
}

// runRPC drives the plugin-side RPC for this phase. Verify consumes the
// per-plugin candidate sections captured at bridge construction; apply uses
// the diff payload decoded from the stream event (which the orchestrator has
// already filtered per participant). A non-empty statusErr means the plugin
// rejected the change; a non-nil err means transport or encoding failure.
func (p phaseKind) runRPC(ctx context.Context, conn *ipc.PluginConn, verifySections []rpc.ConfigSection, applyDiffs []transaction.DiffSection) (statusErr string, err error) {
	if p == phaseApply {
		out, rpcErr := conn.SendConfigApply(ctx, diffSectionsToDiffRPCSections(applyDiffs))
		if rpcErr != nil {
			return "", rpcErr
		}
		if out != nil && out.Status == plugin.StatusError {
			return out.Error, nil
		}
		return "", nil
	}
	out, rpcErr := conn.SendConfigVerify(ctx, verifySections)
	if rpcErr != nil {
		return "", rpcErr
	}
	if out != nil && out.Status == plugin.StatusError {
		return out.Error, nil
	}
	return "", nil
}

// emitFailed publishes the phase's failure ack. The orchestrator keys its
// ack channels off the event type, so verify-failed and apply-failed land
// on different channels even though the payload shapes are identical.
func (p phaseKind) emitFailed(b *configTxBridge, txID, name, errMsg string) {
	if p == phaseApply {
		b.emitApplyFailed(txID, name, errMsg)
		return
	}
	b.emitVerifyFailed(txID, name, errMsg)
}

// emitOK publishes the phase's success ack, echoing the plugin's declared
// budgets so the orchestrator picks up fresh estimates for the next tx.
func (p phaseKind) emitOK(b *configTxBridge, txID, name string, proc *process.Process) {
	if p == phaseApply {
		b.emitApplyOK(txID, name, proc)
		return
	}
	b.emitVerifyOK(txID, name, proc)
}

// phaseEnvelope is the common decoded shape for VerifyEvent and ApplyEvent.
// The two payload types from transaction/types.go share their wire format
// exactly (transaction id + diffs + deadline), so decoding once against a
// neutral local type avoids duplicating the subscribe handler for the two
// phases. Verify ignores Diffs and uses the bridge's cached sections; apply
// consumes Diffs directly.
type phaseEnvelope struct {
	TransactionID string                    `json:"transaction-id"`
	Diffs         []transaction.DiffSection `json:"diffs"`
	DeadlineMS    int64                     `json:"deadline-ms"`
}

// txIDProbe is a minimal unmarshal target used to recover the transaction
// ID from a partially-valid event payload. If the full envelope fails to
// unmarshal but the txID field is present, the bridge can still publish a
// failure ack the orchestrator will route correctly.
type txIDProbe struct {
	TransactionID string `json:"transaction-id"`
}

// subscribePhase registers a handler for the per-plugin event of the given
// phase. Each dispatch path is: extract txID (so a failure ack routes back
// even on partial parse), unmarshal the full envelope, look up the plugin
// process, run the phase RPC under a deadline derived from the event, emit
// the matching ack.
func (b *configTxBridge) subscribePhase(parentCtx context.Context, name string, ph phaseKind) {
	eventType := ph.eventType(name)
	unsub := b.server.SubscribeEngineEvent(txevents.Namespace, eventType, func(p any) {
		event, ok := p.(string)
		if !ok {
			logger().Error("config tx bridge: non-string phase payload",
				"plugin", name, "event-type", eventType, "got", reflect.TypeOf(p))
			return
		}
		raw := []byte(event)

		// Extract txID first so a failure ack from a later step has
		// the correct ack routing key even if the full envelope
		// unmarshal below fails on the diffs field.
		var probe txIDProbe
		if err := json.Unmarshal(raw, &probe); err != nil {
			logger().Error("config tx bridge: extract tx id",
				"plugin", name, "event-type", eventType, "error", err)
			return
		}

		var env phaseEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			logger().Error("config tx bridge: unmarshal phase event",
				"plugin", name, "event-type", eventType, "error", err)
			ph.emitFailed(b, probe.TransactionID, name, "unmarshal phase event: "+err.Error())
			return
		}

		proc := b.lookupProcess(name)
		if proc == nil {
			ph.emitFailed(b, env.TransactionID, name, "plugin process not found (crashed?)")
			return
		}
		conn := proc.Conn()
		if conn == nil {
			ph.emitFailed(b, env.TransactionID, name, "plugin connection closed (crashed?)")
			return
		}

		rpcCtx, cancel := deadlineCtx(parentCtx, env.DeadlineMS)
		defer cancel()

		verifySections := b.verifySections[name]
		statusErr, err := ph.runRPC(rpcCtx, conn, verifySections, env.Diffs)
		if err != nil {
			ph.emitFailed(b, env.TransactionID, name, err.Error())
			return
		}
		if statusErr != "" {
			ph.emitFailed(b, env.TransactionID, name, statusErr)
			return
		}
		ph.emitOK(b, env.TransactionID, name, proc)
	})
	b.addUnsub(unsub)
}

// deadlineCtx derives a per-RPC context. When the event carries a non-zero
// Unix-millis deadline the returned context honors it; otherwise the parent
// is returned unchanged with a no-op cancel. Derived deadlines use the
// parent so that Close or server shutdown still cancels the RPC.
func deadlineCtx(parent context.Context, deadlineMS int64) (context.Context, context.CancelFunc) {
	if deadlineMS <= 0 {
		return parent, func() {}
	}
	return context.WithDeadline(parent, time.UnixMilli(deadlineMS))
}

// subscribeRollback wires a single handler for the broadcast rollback event.
// When the orchestrator rolls back, the bridge fans out config-rollback RPCs
// to every participant and emits rollback-ok acks. RPC errors translate to a
// CodeBroken ack so the orchestrator restarts the plugin via its restartFn.
func (b *configTxBridge) subscribeRollback(parentCtx context.Context) {
	unsub := b.server.SubscribeEngineEvent(txevents.Namespace, transaction.EventRollback, func(p any) {
		event, ok := p.(string)
		if !ok {
			logger().Error("config tx bridge: non-string rollback payload",
				"got", reflect.TypeOf(p))
			return
		}
		var ev transaction.RollbackEvent
		if err := json.Unmarshal([]byte(event), &ev); err != nil {
			logger().Error("config tx bridge: unmarshal rollback event", "error", err)
			return
		}
		// Fan out to every participant. Each plugin dispatches in the
		// calling goroutine; the orchestrator waits for rollback-ok acks
		// with its own deadline so a slow plugin only delays its own tier.
		for _, name := range b.participantNames {
			b.dispatchRollback(parentCtx, ev.TransactionID, name)
		}
	})
	b.addUnsub(unsub)
}

// dispatchRollback calls config-rollback on one plugin and emits the ack.
// Missing processes or closed connections surface as CodeBroken so the
// orchestrator can restart the plugin via its restart callback.
func (b *configTxBridge) dispatchRollback(ctx context.Context, txID, name string) {
	proc := b.lookupProcess(name)
	if proc == nil {
		b.emitRollbackAck(txID, name, transaction.CodeBroken, "plugin process not found")
		return
	}
	conn := proc.Conn()
	if conn == nil {
		b.emitRollbackAck(txID, name, transaction.CodeBroken, "plugin connection closed")
		return
	}
	if err := conn.SendConfigRollback(ctx, txID); err != nil {
		b.emitRollbackAck(txID, name, transaction.CodeBroken, err.Error())
		return
	}
	b.emitRollbackAck(txID, name, transaction.CodeOK, "")
}

// lookupProcess returns the plugin process by name via the current process
// manager snapshot, or nil if the manager is not wired or the plugin has
// exited. The bridge never retains the pointer beyond the current call.
func (b *configTxBridge) lookupProcess(name string) *process.Process {
	pm := b.server.procManager.Load()
	if pm == nil {
		return nil
	}
	return pm.GetProcess(name)
}

// emitVerifyOK publishes a verify-ok ack. Budgets come straight from the
// plugin registration so the orchestrator's flight deadline math reflects
// the latest declared estimates without plumbing SDK-side updates today.
func (b *configTxBridge) emitVerifyOK(txID, name string, proc *process.Process) {
	ack := transaction.VerifyAck{
		TransactionID:    txID,
		Plugin:           name,
		Status:           transaction.CodeOK,
		VerifyBudgetSecs: registrationVerifyBudget(proc),
		ApplyBudgetSecs:  registrationApplyBudget(proc),
	}
	b.emitAck(transaction.EventVerifyOK, ack, txID, name)
}

// emitVerifyFailed publishes a verify-failed ack with the provided error
// message. The orchestrator treats any verify-failed as an abort trigger.
func (b *configTxBridge) emitVerifyFailed(txID, name, errMsg string) {
	ack := transaction.VerifyAck{
		TransactionID: txID,
		Plugin:        name,
		Status:        transaction.CodeError,
		Error:         errMsg,
	}
	b.emitAck(transaction.EventVerifyFailed, ack, txID, name)
}

// emitApplyOK publishes an apply-ok ack, echoing the plugin's registered
// budgets back to the orchestrator for the next transaction.
func (b *configTxBridge) emitApplyOK(txID, name string, proc *process.Process) {
	ack := transaction.ApplyAck{
		TransactionID:    txID,
		Plugin:           name,
		Status:           transaction.CodeOK,
		VerifyBudgetSecs: registrationVerifyBudget(proc),
		ApplyBudgetSecs:  registrationApplyBudget(proc),
	}
	b.emitAck(transaction.EventApplyOK, ack, txID, name)
}

// emitApplyFailed publishes an apply-failed ack. The orchestrator reacts by
// publishing rollback on the broadcast event type.
func (b *configTxBridge) emitApplyFailed(txID, name, errMsg string) {
	ack := transaction.ApplyAck{
		TransactionID: txID,
		Plugin:        name,
		Status:        transaction.CodeError,
		Error:         errMsg,
	}
	b.emitAck(transaction.EventApplyFailed, ack, txID, name)
}

// emitRollbackAck publishes a rollback-ok ack. A non-OK code signals the
// orchestrator to restart the plugin via restartFn before draining the
// next dependency tier.
func (b *configTxBridge) emitRollbackAck(txID, name, code, errMsg string) {
	ack := transaction.RollbackAck{
		TransactionID: txID,
		Plugin:        name,
		Code:          code,
		Error:         errMsg,
	}
	b.emitAck(transaction.EventRollbackOK, ack, txID, name)
}

// emitAck marshals an ack payload and pushes it onto the gateway. Errors
// are logged rather than propagated because ack emission happens from an
// engine event handler with no caller to receive an error -- the
// orchestrator will time out and report a coherent failure instead.
func (b *configTxBridge) emitAck(eventType string, payload any, txID, name string) {
	data, err := json.Marshal(payload)
	if err != nil {
		logger().Error("config tx bridge: marshal ack",
			"event-type", eventType, "plugin", name, "tx", txID, "error", err)
		return
	}
	if _, err := b.gateway.EmitConfigEvent(eventType, data); err != nil {
		logger().Error("config tx bridge: emit ack",
			"event-type", eventType, "plugin", name, "tx", txID, "error", err)
	}
}

// registrationVerifyBudget reads the plugin's declared verify budget from
// its registration. Returns 0 when the plugin did not declare one, matching
// the orchestrator's "trivial" default.
func registrationVerifyBudget(proc *process.Process) int {
	reg := proc.Registration()
	if reg == nil {
		return 0
	}
	return reg.VerifyBudget
}

// registrationApplyBudget mirrors registrationVerifyBudget for apply.
func registrationApplyBudget(proc *process.Process) int {
	reg := proc.Registration()
	if reg == nil {
		return 0
	}
	return reg.ApplyBudget
}

// diffSectionsToDiffRPCSections converts to rpc.ConfigDiffSection, carrying
// Added/Removed/Changed through verbatim. This is the format config-apply
// already consumes.
func diffSectionsToDiffRPCSections(diffs []transaction.DiffSection) []rpc.ConfigDiffSection {
	out := make([]rpc.ConfigDiffSection, 0, len(diffs))
	for _, d := range diffs {
		out = append(out, rpc.ConfigDiffSection{
			Root:    d.Root,
			Added:   d.Added,
			Removed: d.Removed,
			Changed: d.Changed,
		})
	}
	return out
}
