// Design: docs/architecture/api/process-protocol.md — direct transport bridge
// Related: conn.go — socket-based RPC transport (replaced by bridge for internal plugins)

package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"sync/atomic"
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
// ErrBridgeClosed is returned by SendCallback when the callback channel is closed.
var ErrBridgeClosed = errors.New("bridge closed")

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
	deliverEvents     func(events []string) error
	deliverStructured func(events []any) error
	hasStructured     atomic.Bool // set atomically when deliverStructured is written
	dispatchRPC       func(method string, params json.RawMessage) (json.RawMessage, error)
	dispatchCommand   DispatchCommandHandler // Typed fast path (no JSON)
	hasDispatchCmd    atomic.Bool            // set atomically when dispatchCommand is written
	emitEvent         EmitEventHandler       // Typed fast path (no JSON)
	hasEmitEvent      atomic.Bool            // set atomically when emitEvent is written
	callbackCh        chan BridgeCallback    // Engine->plugin callbacks (replaces pipe after startup)
	closeOnce         sync.Once              // Guards callbackCh close (Stop may be called multiple times)
	ready             atomic.Bool
}

// NewDirectBridge creates a bridge. Both sides must register handlers and call
// SetReady before the bridge activates.
func NewDirectBridge() *DirectBridge {
	return &DirectBridge{
		callbackCh: make(chan BridgeCallback, 16),
	}
}

// CallbackCh returns the channel for engine->plugin callbacks.
// The plugin's bridge event loop reads from this after pipe shutdown.
func (b *DirectBridge) CallbackCh() <-chan BridgeCallback {
	return b.callbackCh
}

// SendCallback sends an engine->plugin callback through the bridge channel.
// Blocks until the plugin processes it and returns a result, or ctx expires.
// Used by PluginConn methods (SendExecuteCommand, etc.) when bridge is active.
// Returns ErrBridgeClosed if the callback channel was closed during shutdown.
func (b *DirectBridge) SendCallback(ctx context.Context, method string, params json.RawMessage) (result json.RawMessage, err error) {
	// Sending on a closed channel panics. CloseCallbacks may race with this
	// send during shutdown (context canceled but select picks the send arm).
	defer func() {
		if r := recover(); r != nil {
			err = ErrBridgeClosed
		}
	}()
	resultCh := make(chan BridgeCallbackResult, 1)
	select {
	case b.callbackCh <- BridgeCallback{
		Method: method,
		Params: params,
		Result: resultCh,
	}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case r := <-resultCh:
		return r.Data, r.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// CloseCallbacks closes the callback channel, signaling the plugin's bridge
// event loop to exit. Called during shutdown. Safe to call multiple times.
func (b *DirectBridge) CloseCallbacks() {
	b.closeOnce.Do(func() {
		close(b.callbackCh)
	})
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
	b.ready.Store(true)
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
// Skips all JSON serialization -- takes Go strings, returns Go strings.
type DispatchCommandHandler func(command string) (status, data string, err error)

// SetDispatchCommand registers the engine-side typed dispatch-command handler.
// Called by the engine after startup alongside SetDispatchRPC.
// The hasDispatchCmd atomic creates a happens-before edge so that readers
// calling HasDispatchCommand or DispatchCommand see the function pointer.
func (b *DirectBridge) SetDispatchCommand(fn DispatchCommandHandler) {
	b.dispatchCommand = fn
	b.hasDispatchCmd.Store(fn != nil)
}

// DispatchCommand calls the engine's typed dispatch-command handler directly.
// Returns error if the handler is not set. The hasDispatchCmd atomic load
// creates a happens-before from SetDispatchCommand's write.
func (b *DirectBridge) DispatchCommand(command string) (status, data string, err error) {
	if !b.hasDispatchCmd.Load() {
		return "", "", errors.New("dispatch-command handler not set")
	}
	return b.dispatchCommand(command)
}

// HasDispatchCommand reports whether the typed dispatch-command handler is set.
func (b *DirectBridge) HasDispatchCommand() bool {
	return b.ready.Load() && b.hasDispatchCmd.Load()
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
	if !b.hasEmitEvent.Load() {
		return 0, errors.New("emit-event handler not set")
	}
	return b.emitEvent(namespace, eventType, direction, peerAddress, event)
}

// HasEmitEvent reports whether the typed emit-event handler is set.
func (b *DirectBridge) HasEmitEvent() bool {
	return b.ready.Load() && b.hasEmitEvent.Load()
}

// DispatchRPC calls the engine's RPC handler directly. Returns error if
// the bridge is not ready or the handler is not set.
func (b *DirectBridge) DispatchRPC(method string, params json.RawMessage) (json.RawMessage, error) {
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
	PeerAddress  string         // Source peer address string
	PeerName     string         // Peer name from config
	PeerGroup    string         // Peer group name from config
	PeerAS       uint32         // Remote peer AS number
	LocalAS      uint32         // Local AS number
	LocalAddress string         // Local address string
	EventType    string         // "update", "open", "notification", "keepalive", "refresh", "state", "eor", etc.
	Direction    string         // "sent" or "received"
	MessageID    uint64         // Unique message ID (0 for non-message events)
	State        string         // For state events: "up", "down"
	Reason       string         // For state events: close reason
	RawMessage   any            // *types.RawMessage for wire messages, nil for synthetic events
	Meta         map[string]any // Route metadata (sent events only)
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
	se.LocalAddress = ""
	se.EventType = ""
	se.Direction = ""
	se.MessageID = 0
	se.State = ""
	se.Reason = ""
	se.RawMessage = nil
	se.Meta = nil
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
