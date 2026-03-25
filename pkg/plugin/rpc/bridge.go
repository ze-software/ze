// Design: docs/architecture/api/process-protocol.md — direct transport bridge
// Related: conn.go — socket-based RPC transport (replaced by bridge for internal plugins)

package rpc

import (
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
type DirectBridge struct {
	deliverEvents     func(events []string) error
	deliverStructured func(events []any) error
	dispatchRPC       func(method string, params json.RawMessage) (json.RawMessage, error)
	ready             atomic.Bool
}

// NewDirectBridge creates a bridge. Both sides must register handlers and call
// SetReady before the bridge activates.
func NewDirectBridge() *DirectBridge {
	return &DirectBridge{}
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
func (b *DirectBridge) SetDeliverStructured(fn func(events []any) error) {
	b.deliverStructured = fn
}

// HasStructuredHandler reports whether a structured delivery handler is registered.
func (b *DirectBridge) HasStructuredHandler() bool {
	return b.deliverStructured != nil
}

// DeliverStructured calls the plugin's structured event handler directly.
// Returns error if the bridge is not ready or the handler is not set.
func (b *DirectBridge) DeliverStructured(events []any) error {
	if !b.ready.Load() {
		return errors.New("bridge not ready")
	}
	if b.deliverStructured == nil {
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
