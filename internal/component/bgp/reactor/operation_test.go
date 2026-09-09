package reactor

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/configop"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
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
		Type:  configop.AddPeer,
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

// TestPeerSettingsFromOperationConfigUsesReactorPort verifies operation reload
// peers inherit the reactor port when the peer config omits a per-peer port.
//
// VALIDATES: ADD_PEER/MODIFY_PEER operations keep test and daemon port overrides.
// PREVENTS: reload-created peers falling back to TCP/179 after the initial
// session used a custom port.
func TestPeerSettingsFromOperationConfigUsesReactorPort(t *testing.T) {
	r := New(&Config{Port: 1802})
	adapter := &reactorAPIAdapter{r: r}
	op := rpc.ConfigOperation{
		ID:    "bgp-add-peer-edge",
		Root:  "bgp",
		Owner: "bgp",
		Type:  configop.AddPeer,
		Target: rpc.ResourceRef{
			Kind: rpc.ResourcePeer,
			Peer: "edge",
		},
		Params: rpc.ConfigOperationParams{
			Peer:   "edge",
			Config: json.RawMessage(`{"connection":{"remote":{"ip":"203.0.113.1"},"local":{"ip":"192.0.2.1"}},"session":{"asn":{"local":"65000","remote":"65001"}}}`),
		},
	}

	settings, err := adapter.peerSettingsFromOperationConfig(&op)
	require.NoError(t, err)
	assert.Equal(t, uint16(1802), settings.Port)
}

// TestCandidatePeerSettingsFromOperationConfigUsesReloadFunc verifies operation
// add/modify uses the full reload parser when it is available.
//
// VALIDATES: operation-created peers keep fields populated outside parsePeerFromTree.
// PREVENTS: reload operations losing static routes and runtime port overrides.
func TestCandidatePeerSettingsFromOperationConfigUsesReloadFunc(t *testing.T) {
	r := New(&Config{ConfigPath: "ze.conf"})
	want := NewPeerSettings(mustParseAddr("198.51.100.1"), 65000, 65001, 0)
	want.Name = "edge"
	want.Port = 1802
	r.SetReloadFunc(func(path string) ([]*PeerSettings, error) {
		assert.Equal(t, "ze.conf", path)
		return []*PeerSettings{want}, nil
	})
	adapter := &reactorAPIAdapter{r: r}
	op := rpc.ConfigOperation{
		ID:    "bgp-add-peer-edge",
		Root:  "bgp",
		Owner: "bgp",
		Type:  configop.AddPeer,
		Target: rpc.ResourceRef{
			Kind: rpc.ResourcePeer,
			Peer: "edge",
		},
		Params: rpc.ConfigOperationParams{
			Peer:   "edge",
			Config: json.RawMessage(`{"connection":{"remote":{"ip":"203.0.113.1"},"local":{"ip":"192.0.2.1"}},"session":{"asn":{"local":"65000","remote":"65001"}}}`),
		},
	}

	got, err := adapter.candidatePeerSettingsFromOperationConfig(&op)
	require.NoError(t, err)
	assert.Same(t, want, got)
	assert.Equal(t, uint16(1802), got.Port)
	assert.Equal(t, mustParseAddr("198.51.100.1"), got.Address)
}

// TestCandidatePeerSettingsFallsBackToEmbeddedConfig verifies that when the
// reloadFunc is configured and loads successfully but the target peer is absent,
// candidatePeerSettingsFromOperationConfig falls through to the embedded config.
//
// VALIDATES: new peers added via config commit use the operation's embedded config.
// PREVENTS: add-peer failing because the on-disk config is stale during commit.
func TestCandidatePeerSettingsFallsBackToEmbeddedConfig(t *testing.T) {
	r := New(&Config{ConfigPath: "ze.conf"})
	other := NewPeerSettings(mustParseAddr("198.51.100.99"), 65000, 65099, 0)
	other.Name = "other-peer"
	r.SetReloadFunc(func(path string) ([]*PeerSettings, error) {
		return []*PeerSettings{other}, nil
	})
	adapter := &reactorAPIAdapter{r: r}
	op := rpc.ConfigOperation{
		ID:    "bgp-add-peer-new",
		Root:  "bgp",
		Owner: "bgp",
		Type:  configop.AddPeer,
		Target: rpc.ResourceRef{
			Kind: rpc.ResourcePeer,
			Peer: "new-peer",
		},
		Params: rpc.ConfigOperationParams{
			Peer:   "new-peer",
			Config: json.RawMessage(`{"connection":{"remote":{"ip":"203.0.113.1"},"local":{"ip":"192.0.2.1"}},"session":{"asn":{"local":"65000","remote":"65001"}}}`),
		},
	}

	got, err := adapter.candidatePeerSettingsFromOperationConfig(&op)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, mustParseAddr("203.0.113.1"), got.Address)
	assert.Equal(t, uint32(65001), got.PeerAS)
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
		Type:  configop.AddPeer,
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
		Type:  configop.RemovePeer,
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
		Type:  configop.ModifyPeer,
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

// TestApplyConfigOperationRemovePeerRollbackRestoresAnnouncedState verifies that
// the inverse of a remove-peer restores the peer the reactor was RUNNING, with
// the routes it originates and the policy it applies, and not a peer rebuilt
// from the operation's config leaves.
//
// The goal is the transaction's inverse: a rolled-back remove-peer must leave
// the daemon as it was. The operation payload below is the one the decomposer
// builds (bgpPeerOperation, ../plugin/operation.go), and it states no route,
// no filter chain and no loop-detection policy, because the full config loader
// adds all three after this package's parser has run (peersAndDynamicGroups,
// ../config/peers.go). Rebuilding the peer from that payload restored a session
// that announced nothing.
//
// VALIDATES: rollback of a remove-peer restores StaticRoutes, the filter chains
// and the loop-detection settings the peer was running with.
// PREVENTS: a rolled-back reload leaving a re-established session that
// originates no route and enforces no policy.
func TestApplyConfigOperationRemovePeerRollbackRestoresAnnouncedState(t *testing.T) {
	r := New(&Config{})
	settings := NewPeerSettings(mustParseAddr("203.0.113.1"), 65000, 65001, 0)
	settings.Name = "edge"
	settings.LocalAddress = mustParseAddr("192.0.2.1")
	settings.StaticRoutes = []StaticRoute{{
		Prefix:  netip.MustParsePrefix("192.168.1.0/24"),
		NextHop: bgptypes.NewNextHopExplicit(mustParseAddr("10.0.0.1")),
	}}
	settings.PluginRoutes = []PluginRoute{{Family: "ipv4/flow"}}
	settings.ImportFilters = []filterapi.FilterRef{{Name: "bgp-filter-prefix:CUSTOMERS"}}
	settings.ExportFilters = []filterapi.FilterRef{{Name: "bgp-filter-community:UPSTREAM"}}
	settings.LoopAllowOwnAS = 2
	require.NoError(t, r.AddPeer(settings))
	require.Len(t, r.Peers(), 1)

	j := &testJournal{}
	op := rpc.ConfigOperation{
		ID:    "bgp-remove-peer-edge",
		Root:  "bgp",
		Owner: "bgp",
		Type:  configop.RemovePeer,
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

	out, err := r.ApplyConfigOperation(&op, j)
	require.NoError(t, err)
	require.NotNil(t, out)
	assert.Equal(t, rpc.StatusOK, out.Status)
	require.Empty(t, r.Peers())

	require.Empty(t, j.Rollback())
	require.Len(t, r.Peers(), 1)
	restored := r.Peers()[0].Settings()
	assert.Equal(t, mustParseAddr("203.0.113.1"), restored.Address)
	require.Len(t, restored.StaticRoutes, 1, "the restored peer originates the route it originated before")
	assert.Equal(t, netip.MustParsePrefix("192.168.1.0/24"), restored.StaticRoutes[0].Prefix)
	require.Len(t, restored.PluginRoutes, 1)
	assert.Equal(t, "ipv4/flow", restored.PluginRoutes[0].Family)
	assert.Equal(t, settings.ImportFilters, restored.ImportFilters)
	assert.Equal(t, settings.ExportFilters, restored.ExportFilters)
	assert.Equal(t, uint8(2), restored.LoopAllowOwnAS)
}
