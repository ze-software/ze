// Design: docs/architecture/api/process-protocol.md — direct transport bridge
// Related: conn.go — socket-based RPC transport (replaced by bridge for internal plugins)

package rpc

import (
	"encoding/json"
	"errors"
	"net"
	"sync/atomic"
)

// DirectBridge mediates direct function calls between engine and plugin sides
// for internal plugins, bypassing JSON serialization and socket I/O.
//
// After the 5-stage startup completes over sockets, both sides register their
// handlers and signal ready. Once ready, the engine calls DeliverEvents directly
// (instead of connB.SendDeliverBatch) and the plugin calls DispatchRPC directly
// (instead of engineMux.CallRPC).
type DirectBridge struct {
	deliverEvents func(events []string) error
	dispatchRPC   func(method string, params json.RawMessage) (json.RawMessage, error)
	ready         atomic.Bool
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
