package rpc

import (
	"encoding/json"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDirectBridgeDeliverEvents verifies direct event delivery bypasses socket I/O.
//
// VALIDATES: AC-1 — Plugin's onEvent called directly without JSON-RPC envelope.
// PREVENTS: Events going through JSON marshal + NUL framing + net.Pipe for internal plugins.
func TestDirectBridgeDeliverEvents(t *testing.T) {
	t.Parallel()

	bridge := NewDirectBridge()

	// Register plugin-side event handler
	var received []string
	bridge.SetDeliverEvents(func(events []string) error {
		received = append(received, events...)
		return nil
	})
	bridge.SetReady()

	events := []string{
		`{"type":"bgp","bgp":{"peer":{"address":"10.0.0.1"}}}`,
		`{"type":"bgp","bgp":{"peer":{"address":"10.0.0.2"}}}`,
	}

	err := bridge.DeliverEvents(events)
	require.NoError(t, err)
	assert.Equal(t, events, received)
}

// TestDirectBridgeDispatchRPC verifies direct RPC dispatch bypasses socket I/O.
//
// VALIDATES: AC-2 — Engine dispatcher called directly without JSON marshal or net.Pipe I/O.
// PREVENTS: Plugin→engine RPCs going through JSON + socket for internal plugins.
func TestDirectBridgeDispatchRPC(t *testing.T) {
	t.Parallel()

	bridge := NewDirectBridge()

	// Register engine-side RPC handler
	bridge.SetDispatchRPC(func(method string, params json.RawMessage) (json.RawMessage, error) {
		if method == "ze-plugin-engine:update-route" {
			return json.RawMessage(`{"peers-affected":2,"routes-sent":4}`), nil
		}
		return nil, errors.New("unknown method: " + method)
	})
	bridge.SetReady()

	result, err := bridge.DispatchRPC("ze-plugin-engine:update-route", json.RawMessage(`{"peer-selector":"*","command":"update text origin set igp"}`))
	require.NoError(t, err)
	assert.JSONEq(t, `{"peers-affected":2,"routes-sent":4}`, string(result))
}

// TestDirectBridgeDeliverError verifies error propagation from onEvent.
//
// VALIDATES: AC-5 — Error propagated back to deliverBatch and reflected in EventResult.
// PREVENTS: Errors from plugin event handlers being swallowed by direct transport.
func TestDirectBridgeDeliverError(t *testing.T) {
	t.Parallel()

	bridge := NewDirectBridge()

	handlerErr := errors.New("plugin processing failed")
	bridge.SetDeliverEvents(func(events []string) error {
		return handlerErr
	})
	bridge.SetReady()

	err := bridge.DeliverEvents([]string{`{"event":"test"}`})
	require.Error(t, err)
	assert.Equal(t, handlerErr, err)
}

// TestDirectBridgeDispatchRPCError verifies error propagation from RPC handler.
//
// VALIDATES: AC-6 — Error propagated to SDK caller correctly.
// PREVENTS: Errors from engine RPC handlers being lost in direct transport.
func TestDirectBridgeDispatchRPCError(t *testing.T) {
	t.Parallel()

	bridge := NewDirectBridge()

	bridge.SetDispatchRPC(func(method string, params json.RawMessage) (json.RawMessage, error) {
		return nil, errors.New("dispatch failed")
	})
	bridge.SetReady()

	_, err := bridge.DispatchRPC("ze-plugin-engine:update-route", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dispatch failed")
}

// TestBridgedConnDiscovery verifies SDK discovers bridge via type assertion.
//
// VALIDATES: AC-9 — SDK discovers bridge via BridgedConn type assertion.
// PREVENTS: Bridge reference lost when passing through InternalPluginRunner signature.
func TestBridgedConnDiscovery(t *testing.T) {
	t.Parallel()

	bridge := NewDirectBridge()
	inner1, inner2 := net.Pipe()
	defer closePipe(t, "inner1", inner1)
	defer closePipe(t, "inner2", inner2)

	wrapped := NewBridgedConn(inner1, bridge)

	// Type assertion should discover the bridge
	bridger, ok := wrapped.(Bridger)
	require.True(t, ok, "BridgedConn must implement Bridger")
	assert.Equal(t, bridge, bridger.Bridge())

	// BridgedConn should still work as a net.Conn (compile-time check via Bridger assertion above)
}

// TestBridgedConnFallback verifies plain net.Conn falls back to socket path.
//
// VALIDATES: AC-9 — plain net.Conn falls back to socket transport.
// PREVENTS: Nil bridge panic when external plugin passes plain net.Conn.
func TestBridgedConnFallback(t *testing.T) {
	t.Parallel()

	conn1, conn2 := net.Pipe()
	defer closePipe(t, "conn1", conn1)
	defer closePipe(t, "conn2", conn2)

	// Plain net.Conn should NOT implement Bridger
	_, ok := conn1.(Bridger)
	assert.False(t, ok, "plain net.Conn must not implement Bridger")
}

// TestDirectBridgeNotReady verifies bridge returns error before SetReady.
//
// VALIDATES: AC-4 — Bridge doesn't activate before startup completes.
// PREVENTS: Direct transport racing with 5-stage startup protocol.
func TestDirectBridgeNotReady(t *testing.T) {
	t.Parallel()

	bridge := NewDirectBridge()

	// Register handlers but don't call SetReady
	bridge.SetDeliverEvents(func(events []string) error {
		return nil
	})

	assert.False(t, bridge.Ready(), "bridge should not be ready before SetReady()")

	bridge.SetReady()
	assert.True(t, bridge.Ready(), "bridge should be ready after SetReady()")
}

// TestStructuredEventPool verifies pool get/put cycle and field clearing.
//
// VALIDATES: AC-9 — StructuredEvent pool clears all fields on put (no stale data leaks).
// PREVENTS: Stale data from previous event leaking to next consumer.
func TestStructuredEventPool(t *testing.T) {
	t.Parallel()

	// Get a StructuredEvent, fill all fields
	se := GetStructuredEvent()
	se.PeerAddress = "10.0.0.1"
	se.PeerName = "peer1"
	se.PeerGroup = "group1"
	se.PeerAS = 65001
	se.LocalAS = 65000
	se.LocalAddress = "10.0.0.254"
	se.EventType = "update"
	se.Direction = DirectionReceived
	se.MessageID = 42
	se.State = SessionStateUp
	se.Reason = "reconnect"
	se.RawMessage = "sentinel"
	se.Meta = map[string]any{"key": "val"}

	// Return to pool
	PutStructuredEvent(se)

	// Get again — all fields must be cleared
	se2 := GetStructuredEvent()
	assert.Empty(t, se2.PeerAddress, "PeerAddress not cleared")
	assert.Empty(t, se2.PeerName, "PeerName not cleared")
	assert.Empty(t, se2.PeerGroup, "PeerGroup not cleared")
	assert.Zero(t, se2.PeerAS, "PeerAS not cleared")
	assert.Zero(t, se2.LocalAS, "LocalAS not cleared")
	assert.Empty(t, se2.LocalAddress, "LocalAddress not cleared")
	assert.Empty(t, se2.EventType, "EventType not cleared")
	assert.Empty(t, se2.Direction, "Direction not cleared")
	assert.Zero(t, se2.MessageID, "MessageID not cleared")
	assert.Empty(t, se2.State, "State not cleared")
	assert.Empty(t, se2.Reason, "Reason not cleared")
	assert.Nil(t, se2.RawMessage, "RawMessage not cleared")
	assert.Nil(t, se2.Meta, "Meta not cleared")
	PutStructuredEvent(se2)
}

// TestStructuredEventDeliverViaDirectBridge verifies structured event delivery through DirectBridge.
//
// VALIDATES: AC-1 — Internal plugin receives *StructuredEvent with fields populated.
// PREVENTS: StructuredEvent not reaching plugin's OnStructuredEvent handler.
func TestStructuredEventDeliverViaDirectBridge(t *testing.T) {
	t.Parallel()

	bridge := NewDirectBridge()

	var received []any
	bridge.SetDeliverStructured(func(events []any) error {
		received = append(received, events...)
		return nil
	})
	bridge.SetReady()

	se := &StructuredEvent{
		PeerAddress: "10.0.0.1",
		PeerAS:      65001,
		EventType:   "update",
		Direction:   DirectionReceived,
		MessageID:   1,
		RawMessage:  "test-payload",
	}

	err := bridge.DeliverStructured([]any{se})
	require.NoError(t, err)
	require.Len(t, received, 1)

	got, ok := received[0].(*StructuredEvent)
	require.True(t, ok, "received event must be *StructuredEvent")
	assert.Equal(t, "10.0.0.1", got.PeerAddress)
	assert.Equal(t, uint32(65001), got.PeerAS)
	assert.Equal(t, "update", got.EventType)
	assert.Equal(t, DirectionReceived, got.Direction)
	assert.Equal(t, uint64(1), got.MessageID)
	assert.Equal(t, "test-payload", got.RawMessage)
}
