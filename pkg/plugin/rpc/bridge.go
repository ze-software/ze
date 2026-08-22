// Design: docs/architecture/api/process-protocol.md — direct transport bridge
// Related: conn.go — socket-based RPC transport (replaced by bridge for internal plugins)

package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"iter"
	"net"
	"sync"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/selector"
)

// structuredEventPool eliminates per-event heap allocation of StructuredEvent
// on the DirectBridge hot path. Get, fill fields, deliver, then put back.
var structuredEventPool = sync.Pool{
	New: func() any { return new(StructuredEvent) },
}

// DirectBridge mediates direct function calls between engine and plugin sides
// for internal plugins, bypassing JSON serialization and socket I/O.
//
// After the 5-stage startup completes, both sides register their handlers and
// signal ready. Once ready, the engine calls DeliverEvents directly (bypassing
// SendDeliverBatch) and the plugin calls DispatchRPC directly (bypassing
// engineMux.CallRPC).
// ErrBridgeClosed is returned by callback senders when callback channels are closed.
var ErrBridgeClosed = errors.New("bridge closed")

// ErrBridgeFailed is returned by callback senders after the plugin callback loop failed.
var ErrBridgeFailed = errors.New("bridge failed")

// BridgeCallback is an engine->plugin callback delivered through the bridge channel.
// The engine pushes these; the plugin's bridge event loop drains them serially.
type BridgeCallback struct {
	Method string                      // Callback method name (e.g., "ze-plugin-callback:execute-command")
	Params json.RawMessage             // Callback params (JSON -- reuses existing handler signatures)
	Result chan<- BridgeCallbackResult // Engine blocks on this until plugin responds
}

// BridgeCallbackResult is the plugin's response to a bridge callback.
type BridgeCallbackResult struct {
	Data json.RawMessage
	Err  error
}

type DirectBridge struct {
	deliverEvents         func(events []string) error
	deliverStructured     func(events []any) error
	hasStructured         atomic.Bool // set atomically when deliverStructured is written
	dispatchRPC           func(method string, params json.RawMessage) (json.RawMessage, error)
	dispatchCommand       DispatchCommandHandler       // Typed fast path (no JSON)
	hasDispatchCmd        atomic.Bool                  // set atomically when dispatchCommand is written
	dispatchCommandArgs   DispatchCommandArgsHandler   // Typed fast path with pre-tokenized args (no JSON)
	hasDispatchCmdArgs    atomic.Bool                  // set atomically when dispatchCommandArgs is written
	dispatchCommandAnswer DispatchCommandAnswerHandler // Typed answer path: a head, its records, a terminator
	hasDispatchCmdAnswer  atomic.Bool                  // set atomically when dispatchCommandAnswer is written
	executeCommand        ExecuteCommandHandler        // Typed engine->plugin command handler (no JSON input)
	hasExecuteCommand     atomic.Bool                  // set atomically when executeCommand is written
	executeCommandCh      chan ExecuteCommandRequest   // Engine->plugin typed execute-command callbacks
	emitEvent             EmitEventHandler             // Typed fast path (no JSON)
	hasEmitEvent          atomic.Bool                  // set atomically when emitEvent is written
	forwardCached         ForwardCachedHandler         // Typed fast path (no JSON) -- rs-fastpath-3
	hasForwardCached      atomic.Bool                  // set atomically when forwardCached is written
	releaseCached         ReleaseCachedHandler         // Typed fast path (no JSON) -- rs-fastpath-3
	hasReleaseCached      atomic.Bool                  // set atomically when releaseCached is written
	relayStoredRoute      RelayStoredRouteHandler      // Typed fast path (no JSON) -- egress-rail
	hasRelayStoredRoute   atomic.Bool                  // set atomically when relayStoredRoute is written
	injectWireRoute       InjectWireRouteHandler       // Typed fast path (no JSON) -- bmp-6
	hasInjectWireRoute    atomic.Bool                  // set atomically when injectWireRoute is written
	updateRouteSel        UpdateRouteSelHandler        // Typed fast path with *selector.Selector
	hasUpdateRouteSel     atomic.Bool                  // set atomically when updateRouteSel is written
	batchValidate         BatchValidateHandler         // Typed fast path (no string serialization) -- rpki batching
	hasBatchValidate      atomic.Bool                  // set atomically when batchValidate is written
	callbackCh            chan BridgeCallback          // Engine->plugin callbacks (replaces pipe after startup)
	closeOnce             sync.Once                    // Guards callbackCh close (Stop may be called multiple times)
	sendMu                sync.RWMutex                 // Held for reading by senders, for writing by CloseCallbacks
	sendClosed            bool                         // Guarded by sendMu: the callback channels are closed
	dispatchMu            sync.Mutex                   // Serializes dispatch admission with StopDispatch.
	dispatchClosed        bool
	dispatchWG            sync.WaitGroup
	failed                atomic.Bool  // Set after callback loop failure; callers fail fast.
	failureMu             sync.RWMutex // Guards failureErr, read only after failed is set.
	failureErr            error        // First callback loop failure reported to later callers.
	ready                 atomic.Bool
}

// NewDirectBridge creates a bridge. Both sides must register handlers and call
// SetReady before the bridge activates.
func NewDirectBridge() *DirectBridge {
	return &DirectBridge{
		callbackCh:       make(chan BridgeCallback, 16),
		executeCommandCh: make(chan ExecuteCommandRequest, 16),
	}
}

// CallbackCh returns the channel for engine->plugin callbacks.
// The plugin's bridge event loop reads from this after pipe shutdown.
func (b *DirectBridge) CallbackCh() <-chan BridgeCallback {
	return b.callbackCh
}

// SendCallback sends an engine->plugin callback through the bridge channel.
// Blocks until the plugin processes it and returns a result, or ctx expires.
// Used by PluginConn methods that do not have a typed bridge callback.
// Returns ErrBridgeClosed if the callback channel was closed during shutdown.
func (b *DirectBridge) SendCallback(ctx context.Context, method string, params json.RawMessage) (result json.RawMessage, err error) {
	if failErr := b.callbackFailure(); failErr != nil {
		return nil, failErr
	}
	// Sending on a closed channel panics. CloseCallbacks may race with this
	// send during shutdown (context canceled but select picks the send arm).
	defer func() {
		if r := recover(); r != nil {
			if failErr := b.callbackFailure(); failErr != nil {
				err = failErr
				return
			}
			err = ErrBridgeClosed
		}
	}()
	resultCh := make(chan BridgeCallbackResult, 1)
	if !b.beginSend() {
		return nil, ErrBridgeClosed
	}
	select {
	case b.callbackCh <- BridgeCallback{
		Method: method,
		Params: params,
		Result: resultCh,
	}:
		b.endSend()
	case <-ctx.Done():
		b.endSend()
		return nil, ctx.Err()
	}
	select {
	case r := <-resultCh:
		return r.Data, r.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// FailCallbacks marks the callback bridge failed and closes callback channels.
// Safe to call multiple times; the first error is retained for future callers.
func (b *DirectBridge) FailCallbacks(err error) {
	if err == nil {
		err = ErrBridgeFailed
	}
	b.failureMu.Lock()
	if b.failureErr == nil {
		b.failureErr = err
	}
	b.failureMu.Unlock()
	if b.failed.CompareAndSwap(false, true) {
		b.ready.Store(false)
	}
	b.CloseCallbacks()
}

func (b *DirectBridge) callbackFailure() error {
	if !b.failed.Load() {
		return nil
	}
	b.failureMu.RLock()
	err := b.failureErr
	b.failureMu.RUnlock()
	if err == nil {
		return ErrBridgeFailed
	}
	return err
}

// CloseCallbacks closes the callback channels, signaling the plugin's bridge
// event loop to exit. Called during shutdown. Safe to call multiple times.
//
// It takes sendMu for writing, so it cannot overlap a send. The recover in the
// senders catches the panic from a send on a closed channel. A panic is not the
// only cost. A send concurrent with a close is a data race whatever the outcome,
// and -race fails the test that provoked it.
func (b *DirectBridge) CloseCallbacks() {
	b.StopDispatch()
	b.closeOnce.Do(func() {
		b.sendMu.Lock()
		b.sendClosed = true
		close(b.callbackCh)
		close(b.executeCommandCh)
		b.sendMu.Unlock()
	})
}

// beginSend takes the send side of sendMu and reports whether the caller CAN
// send. It returns false when the channels are already closed, and the caller
// must not call endSend in that case.
//
// A sender that blocks on a full channel holds the lock and delays a concurrent
// close. The reader is still draining at that point, because the close it is
// waiting for has not happened, so the send completes. A caller whose ctx ends
// first releases the lock and gives up.
func (b *DirectBridge) beginSend() bool {
	b.sendMu.RLock()
	if b.sendClosed {
		b.sendMu.RUnlock()
		return false
	}
	return true
}

func (b *DirectBridge) endSend() { b.sendMu.RUnlock() }

// beginDispatch admits one plugin-to-engine call before shutdown. The caller
// MUST call endDispatch once for each admission it takes, on every path out,
// or WaitDispatch never returns.
func (b *DirectBridge) beginDispatch() bool {
	b.dispatchMu.Lock()
	defer b.dispatchMu.Unlock()
	if b.dispatchClosed {
		return false
	}
	b.dispatchWG.Add(1)
	return true
}

// endDispatch releases one admission. MUST be called exactly once for each
// beginDispatch that returned true, and MUST NOT be called for one that
// returned false.
func (b *DirectBridge) endDispatch() {
	b.dispatchWG.Done()
}

// StopDispatch rejects new plugin-to-engine calls without waiting for active calls.
// Safe to call multiple times.
func (b *DirectBridge) StopDispatch() {
	b.dispatchMu.Lock()
	b.dispatchClosed = true
	b.ready.Store(false)
	b.dispatchMu.Unlock()
}

// WaitDispatch waits for admitted plugin-to-engine calls to finish.
// Caller MUST call StopDispatch before WaitDispatch.
func (b *DirectBridge) WaitDispatch() {
	b.dispatchWG.Wait()
}

// SetDeliverEvents registers the plugin-side event handler (engine→plugin direction).
// Called by the SDK after startup to register the onEvent dispatcher.
func (b *DirectBridge) SetDeliverEvents(fn func(events []string) error) {
	b.deliverEvents = fn
}

// SetDispatchRPC registers the engine-side RPC handler (plugin→engine direction).
// Called by the engine after startup to register the dispatch function.
func (b *DirectBridge) SetDispatchRPC(fn func(method string, params json.RawMessage) (json.RawMessage, error)) {
	b.dispatchRPC = fn
}

// SetReady signals that both sides have registered their handlers and the bridge
// can be used for direct transport. Must be called after both SetDeliverEvents
// and SetDispatchRPC.
func (b *DirectBridge) SetReady() {
	b.dispatchMu.Lock()
	if !b.dispatchClosed {
		b.ready.Store(true)
	}
	b.dispatchMu.Unlock()
}

// Ready reports whether the bridge is ready for direct transport.
func (b *DirectBridge) Ready() bool {
	return b.ready.Load()
}

// SetDeliverStructured registers the plugin-side structured event handler.
// Called by the SDK after startup to enable structured delivery (engine→plugin).
// When set, the engine delivers structured events directly instead of formatting text.
// The hasStructured atomic bool creates a happens-before edge so that readers
// calling HasStructuredHandler or DeliverStructured see the function pointer.
func (b *DirectBridge) SetDeliverStructured(fn func(events []any) error) {
	b.deliverStructured = fn
	b.hasStructured.Store(fn != nil)
}

// HasStructuredHandler reports whether a structured delivery handler is registered.
// Uses atomic hasStructured flag — no direct read of the function pointer.
func (b *DirectBridge) HasStructuredHandler() bool {
	return b.ready.Load() && b.hasStructured.Load()
}

// DeliverStructured calls the plugin's structured event handler directly.
// Returns error if the handler is not set. The hasStructured atomic load
// creates a happens-before from SetDeliverStructured's write.
func (b *DirectBridge) DeliverStructured(events []any) error {
	if !b.hasStructured.Load() {
		return errors.New("structured handler not set")
	}
	return b.deliverStructured(events)
}

// DeliverEvents calls the plugin's event handler directly. Returns error if
// the bridge is not ready or the handler is not set.
func (b *DirectBridge) DeliverEvents(events []string) error {
	if !b.ready.Load() {
		return errors.New("bridge not ready")
	}
	if b.deliverEvents == nil {
		return errors.New("deliver handler not set")
	}
	return b.deliverEvents(events)
}

// DispatchCommandHandler is the typed handler for dispatch-command via DirectBridge.
// Returns the full DispatchCommandOutput struct to preserve raw JSON data and
// the separate error field without re-encoding.
type DispatchCommandHandler func(command string) (*DispatchCommandOutput, error)

// SetDispatchCommand registers the engine-side typed dispatch-command handler.
// Called by the engine after startup alongside SetDispatchRPC.
// The hasDispatchCmd atomic creates a happens-before edge so that readers
// calling HasDispatchCommand or DispatchCommand see the function pointer.
func (b *DirectBridge) SetDispatchCommand(fn DispatchCommandHandler) {
	b.dispatchCommand = fn
	b.hasDispatchCmd.Store(fn != nil)
}

// DispatchCommand calls the engine's typed dispatch-command handler directly.
// Returns error if the handler is not set. The caller owns any transport
// completion action attached to the returned output.
func (b *DirectBridge) DispatchCommand(command string) (*DispatchCommandOutput, error) {
	if !b.beginDispatch() {
		return nil, ErrBridgeClosed
	}
	defer b.endDispatch()
	if !b.hasDispatchCmd.Load() {
		return nil, errors.New("dispatch-command handler not set")
	}
	return b.dispatchCommand(command)
}

// HasDispatchCommand reports whether the typed dispatch-command handler is set.
func (b *DirectBridge) HasDispatchCommand() bool {
	return b.ready.Load() && b.hasDispatchCmd.Load()
}

// DispatchCommandArgsHandler is the typed handler for dispatch-command-args via DirectBridge.
// It carries the exact registered command name, pre-tokenized args, and peer selector.
type DispatchCommandArgsHandler func(command string, args []string, peer string) (*DispatchCommandOutput, error)

// SetDispatchCommandArgs registers the engine-side typed dispatch-command-args handler.
// Called by the engine after startup alongside SetDispatchRPC.
// The hasDispatchCmdArgs atomic creates a happens-before edge so that readers
// calling HasDispatchCommandArgs or DispatchCommandArgs see the function pointer.
func (b *DirectBridge) SetDispatchCommandArgs(fn DispatchCommandArgsHandler) {
	b.dispatchCommandArgs = fn
	b.hasDispatchCmdArgs.Store(fn != nil)
}

// DispatchCommandArgs calls the engine's typed dispatch-command-args handler
// directly. Returns error if the handler is not set. The hasDispatchCmdArgs
// atomic load creates a happens-before from SetDispatchCommandArgs' write. The
// caller owns any transport completion action attached to the returned output.
func (b *DirectBridge) DispatchCommandArgs(command string, args []string, peer string) (*DispatchCommandOutput, error) {
	if !b.beginDispatch() {
		return nil, ErrBridgeClosed
	}
	defer b.endDispatch()
	if !b.hasDispatchCmdArgs.Load() {
		return nil, errors.New("dispatch-command-args handler not set")
	}
	return b.dispatchCommandArgs(command, args, peer)
}

// HasDispatchCommandArgs reports whether the typed dispatch-command-args handler is set.
func (b *DirectBridge) HasDispatchCommandArgs() bool {
	return b.ready.Load() && b.hasDispatchCmdArgs.Load()
}

// DispatchCommandAnswerHandler is the typed handler for an answer-returning
// dispatch-command via DirectBridge. It hands back the answer itself -- the
// head, the records the walk produced, and the terminator -- rather than the
// {status, data, error} projection DispatchCommandHandler returns, so an
// in-process plugin reads the rows one at a time exactly as a plugin on the
// socket does (AC-7 of spec-record-answers-1-sdk-path).
//
// The handler MUST NOT start a goroutine for the answer or for a row: the
// records are pulled by the caller's own range (ai/rules/goroutine-lifecycle.md).
type DispatchCommandAnswerHandler func(command string) (*Answer, error)

// SetDispatchCommandAnswer registers the engine-side answer-returning
// dispatch-command handler. The hasDispatchCmdAnswer atomic creates a
// happens-before edge so that a caller of DispatchCommandAnswer sees the
// function pointer.
func (b *DirectBridge) SetDispatchCommandAnswer(fn DispatchCommandAnswerHandler) {
	b.dispatchCommandAnswer = fn
	b.hasDispatchCmdAnswer.Store(fn != nil)
}

// DispatchCommandAnswer calls the engine's answer-returning dispatch-command
// handler directly. Returns an error when the handler is not set.
//
// The dispatch admission spans the WALK, not just the call: the rows of an
// in-process answer are pulled from engine state after this returns, and
// StopDispatch plus WaitDispatch is the rollback barrier that must cover them.
// Releasing at the call would let a rollback tear that state down under a live
// walk.
//
// The caller therefore MUST range over Answer.Records, to its end or to a stop
// of its own, and MUST read Answer.Verdict, Answer.Err and Answer.Message after
// that range, exactly as it must for an answer that arrived over the socket
// (Answer, types.go). An answer whose records are never ranged holds the
// admission, and WaitDispatch waits for it.
func (b *DirectBridge) DispatchCommandAnswer(command string) (*Answer, error) {
	if !b.beginDispatch() {
		return nil, ErrBridgeClosed
	}
	if !b.hasDispatchCmdAnswer.Load() {
		b.endDispatch()
		return nil, errors.New("dispatch-command-answer handler not set")
	}
	answer, err := b.dispatchCommandAnswer(command)
	if err != nil {
		b.endDispatch()
		return nil, err
	}
	if answer == nil {
		b.endDispatch()
		return nil, errors.New("dispatch-command-answer handler returned no answer")
	}
	answer.Records = b.releaseOnWalkEnd(answer.Records)
	return answer, nil
}

// releaseOnWalkEnd wraps the records of an in-process answer so the dispatch
// admission DispatchCommandAnswer took is released when the range over them
// ends, by exhaustion or by the consumer stopping, and released exactly once.
//
// The once is what makes a second range harmless. Whether a second range yields
// anything is the producer's business -- an answer read off the socket is spent
// and one built over a slice is not -- and either way there is exactly ONE
// admission to release. Without the once a second range would release an
// admission this call no longer holds and drive the wait group negative.
//
// MUST be paired with beginDispatch: this is the endDispatch for the call that
// produced the answer, and it is the only one on this path.
func (b *DirectBridge) releaseOnWalkEnd(records iter.Seq[Record]) iter.Seq[Record] {
	var released sync.Once
	return func(yield func(Record) bool) {
		defer released.Do(b.endDispatch)
		if records == nil {
			return
		}
		for record := range records {
			if !yield(record) {
				return
			}
		}
	}
}

// UpdateRouteSelHandler is the typed handler for update-route that carries
// a *selector.Selector instead of a peer selector string.
type UpdateRouteSelHandler func(sel *selector.Selector, command string, meta map[string]any) (announced, withdrawn uint32, err error)

// SetUpdateRouteSel registers the engine-side typed selector update-route handler.
func (b *DirectBridge) SetUpdateRouteSel(fn UpdateRouteSelHandler) {
	b.updateRouteSel = fn
	b.hasUpdateRouteSel.Store(fn != nil)
}

// UpdateRouteSel calls the typed selector update-route handler directly.
func (b *DirectBridge) UpdateRouteSel(sel *selector.Selector, command string, meta map[string]any) (announced, withdrawn uint32, err error) {
	if !b.beginDispatch() {
		return 0, 0, ErrBridgeClosed
	}
	defer b.endDispatch()
	if !b.hasUpdateRouteSel.Load() {
		return 0, 0, errors.New("update-route-sel handler not set")
	}
	return b.updateRouteSel(sel, command, meta)
}

// HasUpdateRouteSel reports whether the typed selector update-route handler is set.
func (b *DirectBridge) HasUpdateRouteSel() bool {
	return b.ready.Load() && b.hasUpdateRouteSel.Load()
}

// ExecuteCommandHandler is the plugin-side typed handler for execute-command.
// It preserves the existing command callback API while allowing DirectBridge to
// carry typed command data through the plugin callback loop.
type ExecuteCommandHandler func(serial, command string, args []string, peer string) (*ExecuteCommandOutput, error)

// ExecuteCommandRequest is an engine->plugin command callback carried through
// DirectBridge without JSON marshaling.
type ExecuteCommandRequest struct {
	Serial  string
	Command string
	Args    []string
	Peer    string
	Result  chan<- ExecuteCommandResult
}

// ExecuteCommandResult is the plugin response for ExecuteCommandRequest.
type ExecuteCommandResult struct {
	Output *ExecuteCommandOutput
	Err    error
}

// SetExecuteCommand registers the plugin-side typed execute-command handler.
// Called by the SDK after startup alongside SetDeliverEvents.
func (b *DirectBridge) SetExecuteCommand(fn ExecuteCommandHandler) {
	b.executeCommand = fn
	b.hasExecuteCommand.Store(fn != nil)
}

// ExecuteCommand sends a typed execute-command callback to the plugin event
// loop and waits for the result. It preserves callback-loop serialization and
// caller cancellation while avoiding JSON marshaling for the request.
func (b *DirectBridge) ExecuteCommand(ctx context.Context, serial, command string, args []string, peer string) (out *ExecuteCommandOutput, err error) {
	if failErr := b.callbackFailure(); failErr != nil {
		return nil, failErr
	}
	defer func() {
		if r := recover(); r != nil {
			if failErr := b.callbackFailure(); failErr != nil {
				out = nil
				err = failErr
				return
			}
			out = nil
			err = ErrBridgeClosed
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !b.beginSend() {
		return nil, ErrBridgeClosed
	}
	if !b.HasExecuteCommand() {
		b.endSend()
		return nil, errors.New("execute-command handler not set")
	}
	resultCh := make(chan ExecuteCommandResult, 1)
	select {
	case b.executeCommandCh <- ExecuteCommandRequest{
		Serial:  serial,
		Command: command,
		Args:    args,
		Peer:    peer,
		Result:  resultCh,
	}:
		b.endSend()
	case <-ctx.Done():
		b.endSend()
		return nil, ctx.Err()
	}
	select {
	case r := <-resultCh:
		return r.Output, r.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// ExecuteCommandRequests returns the typed execute-command callback channel.
// The SDK bridge event loop drains it serially with the JSON callback channel.
func (b *DirectBridge) ExecuteCommandRequests() <-chan ExecuteCommandRequest {
	return b.executeCommandCh
}

// RunExecuteCommand runs the registered typed execute-command handler for req.
func (b *DirectBridge) RunExecuteCommand(req ExecuteCommandRequest) (*ExecuteCommandOutput, error) {
	if !b.hasExecuteCommand.Load() {
		return nil, errors.New("execute-command handler not set")
	}
	return b.executeCommand(req.Serial, req.Command, req.Args, req.Peer)
}

// HasExecuteCommand reports whether the typed execute-command handler is set.
func (b *DirectBridge) HasExecuteCommand() bool {
	return b.ready.Load() && b.hasExecuteCommand.Load()
}

// EmitEventHandler is the typed handler for emit-event via DirectBridge.
// Skips all JSON serialization -- takes Go strings, returns delivered count.
type EmitEventHandler func(namespace, eventType, direction, peerAddress, event string) (int, error)

// SetEmitEvent registers the engine-side typed emit-event handler.
// Called by the engine after startup alongside SetDispatchRPC.
// The hasEmitEvent atomic creates a happens-before edge so that readers
// calling HasEmitEvent or EmitEvent see the function pointer.
func (b *DirectBridge) SetEmitEvent(fn EmitEventHandler) {
	b.emitEvent = fn
	b.hasEmitEvent.Store(fn != nil)
}

// EmitEvent calls the engine's typed emit-event handler directly.
// Returns error if the handler is not set. The hasEmitEvent atomic load
// creates a happens-before from SetEmitEvent's write.
func (b *DirectBridge) EmitEvent(namespace, eventType, direction, peerAddress, event string) (int, error) {
	if !b.beginDispatch() {
		return 0, ErrBridgeClosed
	}
	defer b.endDispatch()
	if !b.hasEmitEvent.Load() {
		return 0, errors.New("emit-event handler not set")
	}
	return b.emitEvent(namespace, eventType, direction, peerAddress, event)
}

// HasEmitEvent reports whether the typed emit-event handler is set.
func (b *DirectBridge) HasEmitEvent() bool {
	return b.ready.Load() && b.hasEmitEvent.Load()
}

// ForwardCachedHandler is the typed handler for forward-cached via DirectBridge.
// Skips JSON entirely -- takes native Go slices. rs-fastpath-3.
//
// ctx is checked before dispatch; a canceled context short-circuits the call.
// Engine-side handlers may also honor ctx for long-running dispatch paths,
// though the current reactor path completes promptly.
type ForwardCachedHandler func(ctx context.Context, ids []uint64, destinations []string) error

// SetForwardCached registers the engine-side typed forward-cached handler.
// Called by the engine after startup alongside SetDispatchRPC.
func (b *DirectBridge) SetForwardCached(fn ForwardCachedHandler) {
	b.forwardCached = fn
	b.hasForwardCached.Store(fn != nil)
}

// ForwardCached calls the engine's typed forward-cached handler directly.
// Returns ctx.Err() when ctx is already canceled before dispatch, or the
// handler's error otherwise. Not-set returns an error without dispatching.
func (b *DirectBridge) ForwardCached(ctx context.Context, ids []uint64, destinations []string) error {
	if !b.beginDispatch() {
		return ErrBridgeClosed
	}
	defer b.endDispatch()
	if err := ctx.Err(); err != nil {
		return err
	}
	if !b.hasForwardCached.Load() {
		return errors.New("forward-cached handler not set")
	}
	return b.forwardCached(ctx, ids, destinations)
}

// HasForwardCached reports whether the typed forward-cached handler is set.
func (b *DirectBridge) HasForwardCached() bool {
	return b.ready.Load() && b.hasForwardCached.Load()
}

// ReleaseCachedHandler is the typed handler for release-cached via DirectBridge.
// Skips JSON entirely -- takes native Go slices. rs-fastpath-3.
//
// ctx is checked before dispatch; a canceled context short-circuits the call.
type ReleaseCachedHandler func(ctx context.Context, ids []uint64) error

// SetReleaseCached registers the engine-side typed release-cached handler.
// Called by the engine after startup alongside SetDispatchRPC.
func (b *DirectBridge) SetReleaseCached(fn ReleaseCachedHandler) {
	b.releaseCached = fn
	b.hasReleaseCached.Store(fn != nil)
}

// ReleaseCached calls the engine's typed release-cached handler directly.
// Returns ctx.Err() when ctx is already canceled before dispatch, or the
// handler's error otherwise. Not-set returns an error without dispatching.
func (b *DirectBridge) ReleaseCached(ctx context.Context, ids []uint64) error {
	if !b.beginDispatch() {
		return ErrBridgeClosed
	}
	defer b.endDispatch()
	if err := ctx.Err(); err != nil {
		return err
	}
	if !b.hasReleaseCached.Load() {
		return errors.New("release-cached handler not set")
	}
	return b.releaseCached(ctx, ids)
}

// HasReleaseCached reports whether the typed release-cached handler is set.
func (b *DirectBridge) HasReleaseCached() bool {
	return b.ready.Load() && b.hasReleaseCached.Load()
}

// RelayStoredRouteHandler is the typed handler for relay-stored-route via
// DirectBridge. Skips JSON entirely -- takes native Go values.
// spec-fixit-bgp-egress-rail-divergence.
//
// ctx is checked before dispatch; a canceled context short-circuits the call.
type RelayStoredRouteHandler func(ctx context.Context, destination string, routes []StoredRoute) error

// SetRelayStoredRoute registers the engine-side typed relay-stored-route handler.
// Called by the engine after startup alongside SetDispatchRPC.
func (b *DirectBridge) SetRelayStoredRoute(fn RelayStoredRouteHandler) {
	b.relayStoredRoute = fn
	b.hasRelayStoredRoute.Store(fn != nil)
}

// RelayStoredRoute calls the engine's typed relay-stored-route handler directly.
// Returns ctx.Err() when ctx is already canceled before dispatch, or the
// handler's error otherwise. Not-set returns an error without dispatching.
func (b *DirectBridge) RelayStoredRoute(ctx context.Context, destination string, routes []StoredRoute) error {
	if !b.beginDispatch() {
		return ErrBridgeClosed
	}
	defer b.endDispatch()
	if err := ctx.Err(); err != nil {
		return err
	}
	if !b.hasRelayStoredRoute.Load() {
		return errors.New("relay-stored-route handler not set")
	}
	return b.relayStoredRoute(ctx, destination, routes)
}

// HasRelayStoredRoute reports whether the typed relay-stored-route handler is set.
func (b *DirectBridge) HasRelayStoredRoute() bool {
	return b.ready.Load() && b.hasRelayStoredRoute.Load()
}

// InjectWireRouteHandler is the typed handler for inject-wire-route via DirectBridge.
// Skips JSON entirely -- takes raw BGP UPDATE body bytes. bmp-6.
// protocol identifies the source (e.g. "bmp"); peerKey identifies the peer
// within that protocol's namespace; updateBody is the BGP UPDATE payload
// (RFC 4271 Section 4.3, without the 19-byte BGP header).
type InjectWireRouteHandler func(protocol, peerKey string, updateBody []byte) error

// globalRouteInjector holds the process-wide InjectWireRouteHandler registered
// by the RIB plugin. The engine-side bridge handler reads this to dispatch
// inject-wire-route calls from any plugin to the RIB.
var globalRouteInjector atomic.Value

// RegisterRouteInjector stores the route injector handler (called by the RIB
// plugin at startup). Thread-safe via atomic.Value.
func RegisterRouteInjector(fn InjectWireRouteHandler) {
	globalRouteInjector.Store(fn)
}

// GetRouteInjector returns the registered route injector, or nil.
func GetRouteInjector() InjectWireRouteHandler {
	v := globalRouteInjector.Load()
	if v == nil {
		return nil
	}
	fn, _ := v.(InjectWireRouteHandler)
	return fn
}

// SetInjectWireRoute registers the engine-side typed inject-wire-route handler.
// Called by the engine after startup alongside SetDispatchRPC.
func (b *DirectBridge) SetInjectWireRoute(fn InjectWireRouteHandler) {
	b.injectWireRoute = fn
	b.hasInjectWireRoute.Store(fn != nil)
}

// InjectWireRoute calls the engine's typed inject-wire-route handler directly.
// Returns error if the handler is not set.
func (b *DirectBridge) InjectWireRoute(protocol, peerKey string, updateBody []byte) error {
	if !b.beginDispatch() {
		return ErrBridgeClosed
	}
	defer b.endDispatch()
	if !b.hasInjectWireRoute.Load() {
		return errors.New("inject-wire-route handler not set")
	}
	return b.injectWireRoute(protocol, peerKey, updateBody)
}

// HasInjectWireRoute reports whether the typed inject-wire-route handler is set.
func (b *DirectBridge) HasInjectWireRoute() bool {
	return b.ready.Load() && b.hasInjectWireRoute.Load()
}

// ValidationDecision carries a single RPKI validation accept/reject decision
// without string serialization. Used by the typed BatchValidate bridge path.
type ValidationDecision struct {
	Accept   bool // true=accept, false=reject
	PeerAddr string
	Family   string
	Prefix   string
	PathID   uint32
	ValState uint8 // validation state for accepts (1=Valid, 2=NotFound); ignored for rejects
}

// BatchValidateResult carries the outcome counters from a batch validation.
type BatchValidateResult struct {
	Accepted int `json:"accepted"`
	Rejected int `json:"rejected"`
	Early    int `json:"early"`
}

// BatchValidateHandler is the typed handler for batch-validate via DirectBridge.
// Skips string serialization for internal plugins.
type BatchValidateHandler func(decisions []ValidationDecision) (*BatchValidateResult, error)

// globalBatchValidator holds the process-wide BatchValidateHandler registered
// by the bgp adj-rib-in plugin. The engine-side bridge handler reads this to dispatch
// batch-validate calls from any plugin to bgp adj-rib-in.
var globalBatchValidator atomic.Value

// RegisterBatchValidator stores the batch validate handler (called by
// bgp adj-rib-in at startup). Thread-safe via atomic.Value.
func RegisterBatchValidator(fn BatchValidateHandler) {
	globalBatchValidator.Store(fn)
}

// GetBatchValidator returns the registered batch validate handler, or nil.
func GetBatchValidator() BatchValidateHandler {
	v := globalBatchValidator.Load()
	if v == nil {
		return nil
	}
	fn, _ := v.(BatchValidateHandler)
	return fn
}

// SetBatchValidate registers the engine-side typed batch-validate handler.
func (b *DirectBridge) SetBatchValidate(fn BatchValidateHandler) {
	b.batchValidate = fn
	b.hasBatchValidate.Store(fn != nil)
}

// BatchValidate calls the engine's typed batch-validate handler directly.
// Returns error if the handler is not set.
func (b *DirectBridge) BatchValidate(decisions []ValidationDecision) (*BatchValidateResult, error) {
	if !b.beginDispatch() {
		return nil, ErrBridgeClosed
	}
	defer b.endDispatch()
	if !b.hasBatchValidate.Load() {
		return nil, errors.New("batch-validate handler not set")
	}
	return b.batchValidate(decisions)
}

// HasBatchValidate reports whether the typed batch-validate handler is set.
func (b *DirectBridge) HasBatchValidate() bool {
	return b.ready.Load() && b.hasBatchValidate.Load()
}

// DispatchRPC calls the engine's RPC handler directly. Returns error if
// the bridge is not ready or the handler is not set.
func (b *DirectBridge) DispatchRPC(method string, params json.RawMessage) (json.RawMessage, error) {
	if !b.beginDispatch() {
		return nil, ErrBridgeClosed
	}
	defer b.endDispatch()
	if !b.ready.Load() {
		return nil, errors.New("bridge not ready")
	}
	if b.dispatchRPC == nil {
		return nil, errors.New("dispatch handler not set")
	}
	return b.dispatchRPC(method, params)
}

// StructuredEvent carries peer context and event data through DirectBridge.
// Used by events.go to deliver BGP events to in-process plugins without text formatting.
//
// For UPDATE events, RawMessage is set to *types.RawMessage (carries AttrsWire
// for lazy per-attribute parsing and WireUpdate for zero-copy section access).
// For state events, State and Reason carry the event data; RawMessage is nil.
// For other wire messages (OPEN, NOTIFICATION, etc.), RawMessage is set.
//
// Async safety: RawMessage may reference zero-copy wire buffers that are reused
// after the callback returns. Plugins MUST copy any data they need to retain
// beyond the handler invocation. See types.RawMessage.IsAsyncSafe().
//
// Pooled via GetStructuredEvent/PutStructuredEvent — callers MUST return via
// PutStructuredEvent after all consumers have processed the event.
type StructuredEvent struct {
	PeerAddress   string           // Source peer address string
	PeerName      string           // Peer name from config
	PeerGroup     string           // Peer group name from config
	PeerAS        uint32           // Remote peer AS number
	LocalAS       uint32           // Local AS number
	RouterID      uint32           // Remote peer's BGP Identifier (from OPEN)
	LocalAddress  string           // Local address string
	EventType     EventKind        // EventKindUpdate, EventKindOpen, etc.
	Direction     MessageDirection // DirectionSent / DirectionReceived
	MessageID     uint64           // Unique message ID (0 for non-message events)
	State         SessionState     // For state events: SessionStateUp, SessionStateDown
	Reason        string           // For state events: close reason
	RawMessage    any              // *types.RawMessage for wire messages, nil for synthetic events
	Meta          map[string]any   // Route metadata (sent events only)
	SourcePeerStr string           // Source peer address for ribOut stale-scoping (sent events only)
}

// GetStructuredEvent returns a StructuredEvent from the pool.
// All fields are zeroed. Caller MUST call PutStructuredEvent after use.
func GetStructuredEvent() *StructuredEvent {
	se, ok := structuredEventPool.Get().(*StructuredEvent)
	if !ok {
		se = new(StructuredEvent)
	}
	return se
}

// PutStructuredEvent returns a StructuredEvent to the pool after clearing all fields.
// MUST be called after all consumers have processed the event.
func PutStructuredEvent(se *StructuredEvent) {
	se.PeerAddress = ""
	se.PeerName = ""
	se.PeerGroup = ""
	se.PeerAS = 0
	se.LocalAS = 0
	se.RouterID = 0
	se.LocalAddress = ""
	se.EventType = EventKindUnspecified
	se.Direction = DirectionUnspecified
	se.MessageID = 0
	se.State = SessionStateUnspecified
	se.Reason = ""
	se.RawMessage = nil
	se.Meta = nil
	se.SourcePeerStr = ""
	structuredEventPool.Put(se)
}

// Bridger is implemented by connections that carry a DirectBridge reference.
// The SDK discovers the bridge via type assertion on net.Conn in NewWithConn.
type Bridger interface {
	Bridge() *DirectBridge
}

// BridgedConn wraps a net.Conn and carries a DirectBridge reference.
// It implements net.Conn by delegating all methods to the inner connection,
// and implements Bridger for bridge discovery via type assertion.
type BridgedConn struct {
	net.Conn
	bridge *DirectBridge
}

// NewBridgedConn wraps conn with a bridge reference. The returned connection
// is a drop-in replacement for net.Conn that also implements Bridger.
func NewBridgedConn(conn net.Conn, bridge *DirectBridge) net.Conn {
	return &BridgedConn{Conn: conn, bridge: bridge}
}

// Bridge returns the DirectBridge reference carried by this connection.
func (bc *BridgedConn) Bridge() *DirectBridge {
	return bc.bridge
}
