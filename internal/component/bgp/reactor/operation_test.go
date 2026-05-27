package reactor

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpevents "codeberg.org/thomas-mangin/ze/internal/component/bgp/events"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

type operationEventBus struct {
	events []operationEvent
}

type operationEvent struct {
	Namespace string
	EventType string
	Payload   any
}

func (b *operationEventBus) Emit(namespace, eventType string, payload any) (int, error) {
	b.events = append(b.events, operationEvent{Namespace: namespace, EventType: eventType, Payload: payload})
	return 0, nil
}

func (b *operationEventBus) Subscribe(_, _ string, _ func(any)) func() {
	return func() {}
}

// TestApplyConfigOperationAddPeerJournal verifies ADD_PEER applies one peer and
// records an inverse for executor-ordered rollback.
//
// VALIDATES: ADD_PEER operation adds the peer and rollback removes it.
// PREVENTS: BGP operation apply mutating reactor state without a journaled inverse.
func TestApplyConfigOperationAddPeerJournal(t *testing.T) {
	r := New(&Config{})
	adapter := &reactorAPIAdapter{r: r}
	j := &testJournal{}
	op := rpc.ConfigOperation{
		ID:    "bgp-add-peer-edge",
		Root:  "bgp",
		Owner: "bgp",
		Type:  rpc.OperationAddPeer,
		Target: rpc.ResourceRef{
			Kind: rpc.ResourcePeer,
			Peer: "edge",
		},
		Params: rpc.ConfigOperationParams{
			Peer:    "edge",
			Address: "192.0.2.1",
			Config:  json.RawMessage(`{"connection":{"remote":{"ip":"203.0.113.1"},"local":{"ip":"192.0.2.1"}},"session":{"asn":{"local":"65000","remote":"65001"}}}`),
		},
	}

	out, err := adapter.applyConfigOperation(&op, j)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, rpc.StatusOK, out.Status)
	require.Len(t, r.Peers(), 1)
	assert.Equal(t, "203.0.113.1", r.Peers()[0].Settings().Address.String())

	require.Empty(t, j.Rollback())
	assert.Empty(t, r.Peers())
}

// TestApplyConfigOperationAddPeerEmitsListenerReady verifies ADD_PEER makes
// listener readiness observable for the operation settlement waiter.
//
// VALIDATES: ADD_PEER emits bgp/listener-ready for its local address.
// PREVENTS: The executor waiting forever after a successful BGP operation apply.
func TestApplyConfigOperationAddPeerEmitsListenerReady(t *testing.T) {
	r := New(&Config{})
	bus := &operationEventBus{}
	r.SetEventBus(bus)
	adapter := &reactorAPIAdapter{r: r}
	j := &testJournal{}
	op := rpc.ConfigOperation{
		ID:    "bgp-add-peer-ready",
		Root:  "bgp",
		Owner: "bgp",
		Type:  rpc.OperationAddPeer,
		Target: rpc.ResourceRef{
			Kind: rpc.ResourcePeer,
			Peer: "edge",
		},
		Params: rpc.ConfigOperationParams{
			Peer:    "edge",
			Address: "192.0.2.1",
			Config:  json.RawMessage(`{"connection":{"remote":{"ip":"203.0.113.1"},"local":{"ip":"192.0.2.1"}},"session":{"asn":{"local":"65000","remote":"65001"}}}`),
		},
	}

	out, err := adapter.applyConfigOperation(&op, j)
	require.NoError(t, err)
	require.NotNil(t, out)
	require.Len(t, out.Readiness, 1)
	assert.Equal(t, bgpevents.Namespace, out.Readiness[0].Namespace)
	assert.Equal(t, bgpevents.EventListenerReady, out.Readiness[0].EventType)
	require.Len(t, bus.events, 1)
	assert.Equal(t, bgpevents.Namespace, bus.events[0].Namespace)
	assert.Equal(t, bgpevents.EventListenerReady, bus.events[0].EventType)
	payload, ok := bus.events[0].Payload.(string)
	require.True(t, ok)
	var ready bgpListenerReadyPayload
	require.NoError(t, json.Unmarshal([]byte(payload), &ready))
	assert.Equal(t, "192.0.2.1", ready.Address)
}

// TestApplyConfigOperationRemovePeerJournal verifies REMOVE_PEER uses the old
// peer config to remove the peer and journal exact restoration.
//
// VALIDATES: REMOVE_PEER removes the peer and rollback restores its old settings.
// PREVENTS: peer removal operations losing the old config needed for rollback.
func TestApplyConfigOperationRemovePeerJournal(t *testing.T) {
	r := New(&Config{})
	settings := NewPeerSettings(mustParseAddr("203.0.113.1"), 65000, 65001, 0)
	settings.Name = "edge"
	settings.LocalAddress = mustParseAddr("192.0.2.1")
	require.NoError(t, r.AddPeer(settings))
	require.Len(t, r.Peers(), 1)

	adapter := &reactorAPIAdapter{r: r}
	j := &testJournal{}
	op := rpc.ConfigOperation{
		ID:    "bgp-remove-peer-edge",
		Root:  "bgp",
		Owner: "bgp",
		Type:  rpc.OperationRemovePeer,
		Target: rpc.ResourceRef{
			Kind: rpc.ResourcePeer,
			Peer: "edge",
		},
		Params: rpc.ConfigOperationParams{
			Peer:      "edge",
			Address:   "192.0.2.1",
			OldConfig: json.RawMessage(`{"connection":{"remote":{"ip":"203.0.113.1"},"local":{"ip":"192.0.2.1"}},"session":{"asn":{"local":"65000","remote":"65001"}}}`),
		},
	}

	out, err := adapter.applyConfigOperation(&op, j)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, rpc.StatusOK, out.Status)
	assert.Empty(t, r.Peers())

	require.Empty(t, j.Rollback())
	require.Len(t, r.Peers(), 1)
	assert.Equal(t, "203.0.113.1", r.Peers()[0].Settings().Address.String())
}

// TestApplyConfigOperationModifyPeerJournal verifies MODIFY_PEER removes the
// old peer and adds the new one atomically, with rollback restoring the old.
//
// VALIDATES: MODIFY_PEER removes old config, adds new config, rollback restores old.
// PREVENTS: partial modify leaving the reactor with neither old nor new peer.
func TestApplyConfigOperationModifyPeerJournal(t *testing.T) {
	r := New(&Config{})
	settings := NewPeerSettings(mustParseAddr("203.0.113.1"), 65000, 65001, 0)
	settings.Name = "edge"
	settings.LocalAddress = mustParseAddr("192.0.2.1")
	require.NoError(t, r.AddPeer(settings))
	require.Len(t, r.Peers(), 1)

	adapter := &reactorAPIAdapter{r: r}
	j := &testJournal{}
	op := rpc.ConfigOperation{
		ID:    "bgp-modify-peer-edge",
		Root:  "bgp",
		Owner: "bgp",
		Type:  rpc.OperationModifyPeer,
		Target: rpc.ResourceRef{
			Kind: rpc.ResourcePeer,
			Peer: "edge",
		},
		Params: rpc.ConfigOperationParams{
			Peer:      "edge",
			Address:   "192.0.2.2",
			OldConfig: json.RawMessage(`{"connection":{"remote":{"ip":"203.0.113.1"},"local":{"ip":"192.0.2.1"}},"session":{"asn":{"local":"65000","remote":"65001"}}}`),
			Config:    json.RawMessage(`{"connection":{"remote":{"ip":"203.0.113.1"},"local":{"ip":"192.0.2.2"}},"session":{"asn":{"local":"65000","remote":"65001"}}}`),
		},
	}

	out, err := adapter.applyConfigOperation(&op, j)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, rpc.StatusOK, out.Status)
	require.Len(t, r.Peers(), 1)
	assert.Equal(t, mustParseAddr("192.0.2.2"), r.Peers()[0].Settings().LocalAddress)

	require.Empty(t, j.Rollback())
	require.Len(t, r.Peers(), 1)
	assert.Equal(t, mustParseAddr("192.0.2.1"), r.Peers()[0].Settings().LocalAddress)
}
