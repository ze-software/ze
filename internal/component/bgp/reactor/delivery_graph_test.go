package reactor

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/configop"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// setupDeliveryGraphTestServer registers the test endpoint names in the plugin
// registry (restored on cleanup) and seeds srv's BusConfig so srv.UpdateDeliveryGraph
// compiles real edges for those endpoints through the production pipeline.
func setupDeliveryGraphTestServer(t *testing.T, srv *pluginserver.Server, names ...string) {
	t.Helper()
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	busCfg := srv.BusConfig()
	if busCfg.Endpoints == nil {
		busCfg.Endpoints = make(map[string]plugin.BusBlock, len(names))
	}
	for _, name := range names {
		if registry.Lookup(name) == nil {
			require.NoError(t, registry.Register(registry.Registration{
				Name:       name,
				RunEngine:  func(net.Conn) int { return 0 },
				CLIHandler: func([]string) int { return 0 },
				Sends:      testAllBGPSendDecls(),
				Receives:   testAllBGPReceiveDecls(),
			}))
		}
		if _, exists := busCfg.Endpoints[name]; !exists {
			busCfg.Endpoints[name] = plugin.BusBlock{}
		}
	}
	srv.SetBusConfig(busCfg)
}

// deliveryTree is the resolved bgp tree both wiring tests load. Two peers, two
// programs, three relationships: 192.0.2.1 feeds the program firstBus names,
// 192.0.2.2 feeds nothing and lets route-injector announce.
func deliveryTree(firstBus map[string]any) map[string]any {
	return map[string]any{
		"router-id": "10.0.0.1",
		"session":   map[string]any{"asn": map[string]any{"local": "65000"}},
		"peer": map[string]any{
			"first": map[string]any{
				"connection": map[string]any{
					"remote": map[string]any{"ip": "192.0.2.1", "connect": "false"},
					"local":  map[string]any{"ip": "auto"},
				},
				"session": map[string]any{"asn": map[string]any{"remote": "65001"}},
				"bus":     firstBus,
			},
			"second": map[string]any{
				"connection": map[string]any{
					"remote": map[string]any{"ip": "192.0.2.2", "connect": "false"},
					"local":  map[string]any{"ip": "auto"},
				},
				"session": map[string]any{"asn": map[string]any{"remote": "65002"}},
				"bus": map[string]any{
					"send": map[string]any{"allow": "none"},
					"receive": map[string]any{
						"allow": []string{"bgp:update"},
						"from":  []string{"/bus/endpoint/route-injector"},
					},
				},
			},
		},
	}
}

// lookingGlass is the first peer's bus block before the reload: it feeds
// received UPDATEs and session state to looking-glass, and accepts nothing.
var lookingGlass = map[string]any{
	"send": map[string]any{
		"allow": []string{"bgp:update", "bgp:state"},
		"to":    []string{"/bus/endpoint/looking-glass"},
		"message": map[string]any{
			"bgp:update": map[string]any{
				"tag": map[string]any{
					"direction": map[string]any{"value": []string{"received"}},
				},
			},
		},
	},
	"receive": map[string]any{"allow": "none"},
}

// addPeersFromTree parses a resolved bgp tree and adds every peer, which is what
// the startup config load does (bgp/config/loader_create.go).
func addPeersFromTree(t *testing.T, r *Reactor, tree map[string]any) {
	t.Helper()
	peers, err := PeersFromTree(tree)
	require.NoError(t, err)
	for _, ps := range peers {
		require.NoError(t, r.AddPeer(ps))
	}
}

// peerView returns one peer's inspection entry, the operator surface's data.
func peerView(t *testing.T, g *pluginserver.DeliveryGraph, addr string) pluginserver.PeerEdges {
	t.Helper()
	for _, pe := range g.Inspect() {
		if pe.Peer == addr {
			return pe
		}
	}
	t.Fatalf("peer %s is not in the delivery graph", addr)
	return pluginserver.PeerEdges{}
}

// recv is the delivery-side question, asked exactly as the seven peer-scoped
// entry points in bgp/server/events.go ask it.
func recv(g *pluginserver.DeliveryGraph, eventType string, dir events.Direction, peerAddr string) []string {
	return g.Receivers(
		events.LookupNamespaceID(bgpevents.Namespace),
		events.LookupEventTypeID(eventType),
		dir,
		peerAddr,
	)
}

// TestDynamicPeerEntersTheIndexUnderItsOwnAddress answers AC-6 at the producer,
// and it answers R-2 in the negative: no group key is needed, because the index
// is built from RESOLVED settings and a dynamic member's settings already carry
// its group's bus block.
//
// The chain is three functions and this test drives all three. The group's
// `bus` block is read by the SAME parser a static peer's is
// (ParseDynamicGroupTemplate -> parsePeerSettings -> parsePeerBusFromTree),
// buildDynamicPeerSettings copies the template whole so Bus arrives
// by inheritance rather than by a copy list, and the index keys the member on
// the address its connection arrived from.
//
// VALIDATES: AC-6. A peer created from a dynamic group is fed by the group's
// list, and no config names its generated identity.
// PREVENTS: R-2's feared fix, a second group-keyed match in newDeliveryGraph.
// That machinery would be an alias for a key the index already has, and every
// event would pay for it.
func TestDynamicPeerEntersTheIndexUnderItsOwnAddress(t *testing.T) {
	tmpl, err := ParseDynamicGroupTemplate("ix", map[string]any{
		"connection": map[string]any{
			"remote": map[string]any{"connect": "false"},
			"local":  map[string]any{"ip": "auto"},
		},
		"session": map[string]any{"asn": map[string]any{"local": "65000"}},
		"bus": map[string]any{
			"send": map[string]any{
				"allow": []string{"bgp:state"},
				"to":    []string{"/bus/endpoint/looking-glass"},
			},
			"receive": map[string]any{"allow": "none"},
		},
	}, 65000, 0x0A000001)
	require.NoError(t, err)
	require.False(t, tmpl.Bus.IsEmpty(), "the group's bus block is parsed by the peer parser")

	dg := &DynamicGroupConfig{
		GroupName: "ix",
		Ranges:    []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")},
		Settings:  tmpl,
	}
	member, err := (&Reactor{}).buildDynamicPeerSettings(dg, netip.MustParseAddr("192.0.2.7"))
	require.NoError(t, err)

	// The member holds the group's bus block, unchanged.
	require.True(t, plugin.BusBlockEqual(tmpl.Bus, member.Bus))
	require.Equal(t, "dyn-192.0.2.7", member.Name, "no config document holds this name")

	srv := &pluginserver.Server{}
	setupDeliveryGraphTestServer(t, srv, "looking-glass")
	srv.UpdateDeliveryGraph(bgpevents.Namespace, deliveryPeersFromSettings([]*PeerSettings{member}))
	g := srv.DeliveryGraph()

	// Asked exactly as the seven peer-scoped delivery sites ask it: by address.
	assert.Equal(t, []string{"looking-glass"}, recv(g, bgpevents.EventState, events.DirUnspecified, "192.0.2.7"),
		"the group's list feeds the member the connection created")
	assert.Empty(t, recv(g, bgpevents.EventUpdate, events.DirReceived, "192.0.2.7"),
		"the group grants state alone, so the member is fed nothing else")

	// The generated identity is not a key, and does not need to be. A lookup on
	// it must find nothing rather than fall back to feeding everybody.
	assert.Empty(t, recv(g, bgpevents.EventState, events.DirUnspecified, member.Name),
		"the index is keyed on the address; dyn-<addr> names no edge")
}

// TestConfigLoadPublishesDeliveryGraph is the WIRING test: the entry point is
// the config load path (PeersFromTree then AddPeer then start), and the feature
// code is the peer-to-process index the plugin server holds.
//
// VALIDATES: the index is reachable from the config an operator writes, and it
// names exactly the processes each peer attaches.
// PREVENTS: an index nobody publishes to, which no later phase could consult.
func TestConfigLoadPublishesDeliveryGraph(t *testing.T) {
	srv := &pluginserver.Server{}
	setupDeliveryGraphTestServer(t, srv, "looking-glass", "route-injector")
	r := newTestReactor(t)
	r.api = srv

	// Before the load the index is empty, and an empty index feeds nobody.
	assert.Empty(t, recv(srv.DeliveryGraph(), bgpevents.EventState, events.DirUnspecified, "192.0.2.1"))

	addPeersFromTree(t, r, deliveryTree(lookingGlass))
	r.mu.Lock()
	r.publishDeliveryGraphLocked()
	r.mu.Unlock()

	g := srv.DeliveryGraph()
	assert.Equal(t, []string{"looking-glass"}, recv(g, bgpevents.EventState, events.DirUnspecified, "192.0.2.1"))
	assert.Equal(t, []string{"looking-glass"}, recv(g, bgpevents.EventUpdate, events.DirReceived, "192.0.2.1"))
	assert.Empty(t, recv(g, bgpevents.EventUpdate, events.DirSent, "192.0.2.1"),
		"update-received grants one direction, not both")
	assert.Empty(t, recv(g, bgpevents.EventUpdate, events.DirReceived, "192.0.2.2"),
		"a send permission feeds the process nothing")
	assert.Empty(t, recv(g, bgpevents.EventState, events.DirUnspecified, "198.51.100.9"),
		"a peer the config never named has no edges")
}

// TestReloadRepublishesDeliveryGraph is the second half of the wiring: a peer
// whose attach block changes goes through RemovePeer and AddPeer, because
// Bus with receive grants is outside hotSwappableSettings (peer_settings_apply.go), and
// both republish the index while the reactor runs.
//
// VALIDATES: AC-4's first half, that delivery edges follow the new config.
// PREVENTS: an index that answers from the config the daemon started with.
func TestReloadRepublishesDeliveryGraph(t *testing.T) {
	srv := &pluginserver.Server{}
	setupDeliveryGraphTestServer(t, srv, "looking-glass", "policy-engine", "route-injector")
	r := newTestReactor(t)
	r.api = srv
	// A LIVE index is what makes AddPeer and RemovePeer republish: before the
	// first publish they skip it, so the startup load pays one build for the
	// whole peer set. The peers are passive (connect false), so starting one
	// dials nothing.
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	r.ctx = ctx
	r.running = true

	// The startup order, exactly: every peer added, then one publish, and the
	// peers start after it (reactor.go, StartWithContext).
	addPeersFromTree(t, r, deliveryTree(lookingGlass))
	r.mu.Lock()
	r.publishDeliveryGraphLocked()
	r.mu.Unlock()
	require.Equal(t, []string{"looking-glass"},
		recv(srv.DeliveryGraph(), bgpevents.EventState, events.DirUnspecified, "192.0.2.1"))

	next := deliveryTree(map[string]any{
		"send": map[string]any{
			"allow": []string{"bgp:update"},
			"to":    []string{"/bus/endpoint/policy-engine"},
			"message": map[string]any{
				"bgp:update": map[string]any{
					"tag": map[string]any{
						"direction": map[string]any{"value": []string{"received"}},
					},
				},
			},
		},
		"receive": map[string]any{"allow": "none"},
	})
	newPeers, err := PeersFromTree(next)
	require.NoError(t, err)
	require.NoError(t, (&reactorAPIAdapter{r: r}).reconcilePeers(newPeers, Globals{}, dynamicGroupsNotRead, "test reload"))

	g := srv.DeliveryGraph()
	assert.Empty(t, recv(g, bgpevents.EventState, events.DirUnspecified, "192.0.2.1"),
		"the replaced list no longer grants state")
	assert.Equal(t, []string{"policy-engine"}, recv(g, bgpevents.EventUpdate, events.DirReceived, "192.0.2.1"))
	secondEdges := peerView(t, g, "192.0.2.2").Processes
	require.Len(t, secondEdges, 1, "the untouched peer keeps its send edge")
	assert.Equal(t, "route-injector", secondEdges[0].Process)
	assert.Equal(t, []string{"update"}, secondEdges[0].Send)
}

// TestConfigApplyDiscardsRuntimeSubscriptions is the discard half of R-10, driven
// from the apply that owns it.
//
// The apply is reconcilePeers -> reconcilePeersJournaled, and its last act is
// DiscardRuntimeSubscriptions (reactor_api.go). Driving the RELOAD rather than
// that method is what makes this evidence: the discard used to hang off
// UpdateDeliveryGraph, where every peer change canceled the subscription too,
// and a test aimed at the plugin server's method alone cannot tell the two
// wirings apart. The survival and confinement halves are
// TestRuntimeSubscribeSurvivesAPublishThatIsNotAnApply
// (plugin/server/delivery_filter_test.go).
//
// VALIDATES: AC-5 and R-10, the discard direction.
// PREVENTS: a reload leaving an override standing, which would make the config
// document a lie about what the daemon feeds.
func TestConfigApplyDiscardsRuntimeSubscriptions(t *testing.T) {
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	setupDeliveryGraphTestServer(t, srv, "looking-glass", "route-injector")

	r := newTestReactor(t)
	r.api = srv
	addPeersFromTree(t, r, deliveryTree(lookingGlass))
	r.mu.Lock()
	r.publishDeliveryGraphLocked()
	r.mu.Unlock()

	ns := events.LookupNamespaceID(bgpevents.Namespace)
	state := events.LookupEventTypeID(bgpevents.EventState)

	// 192.0.2.1 grants looking-glass state, but the process below has no startup
	// subscription. The live addition is therefore the only reason the event is
	// delivered, while the configured grant remains the authorization.
	engine := process.NewProcess(plugin.PluginConfig{Name: "looking-glass"})
	srv.Subscriptions().Add(engine, &pluginserver.Subscription{
		Namespace: ns, EventType: state, Direction: events.DirBoth,
		PeerFilter: &pluginserver.PeerFilter{Selector: "192.0.2.1"}, Runtime: true,
	})
	live := srv.PeerScopedProcs(ns, state, events.DirUnspecified, "192.0.2.1", "first")
	require.Len(t, live, 1, "the permitted live capability addition must take effect")
	require.Equal(t, "looking-glass", live[0].Name())

	// The same document, applied again: the reload's job is to make the daemon
	// match it, whether or not any peer changed.
	newPeers, err := PeersFromTree(deliveryTree(lookingGlass))
	require.NoError(t, err)
	require.NoError(t, (&reactorAPIAdapter{r: r}).reconcilePeers(newPeers, Globals{}, dynamicGroupsNotRead, "test reload"))

	assert.Empty(t, srv.PeerScopedProcs(ns, state, events.DirUnspecified, "192.0.2.1", "first"),
		"a config apply discards the live capability addition")
}

// TestRebuildOfTheRunningConfigKeepsRuntimeSubscriptions is the survival
// half of R-10 for RebuildRunningPeers. A plugin respawning with a different
// filter declaration rebuilds the peers from the running tree after startup
// (spec-filter-wire-6-stage-filters, re-insertion on a runtime declaration
// change), and that changed no config, so the operator's live `subscribe`
// stays. Method: the same reconcile as the discard test above, run as a
// rebuild.
//
// PREVENTS: a third-party plugin's restart deleting the subscription an
// operator typed to watch a live session, with no message.
func TestRebuildOfTheRunningConfigKeepsRuntimeSubscriptions(t *testing.T) {
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	setupDeliveryGraphTestServer(t, srv, "looking-glass", "route-injector")

	r := newTestReactor(t)
	r.api = srv
	addPeersFromTree(t, r, deliveryTree(lookingGlass))
	r.mu.Lock()
	r.publishDeliveryGraphLocked()
	r.mu.Unlock()

	ns := events.LookupNamespaceID(bgpevents.Namespace)
	state := events.LookupEventTypeID(bgpevents.EventState)
	engine := process.NewProcess(plugin.PluginConfig{Name: "looking-glass"})
	srv.Subscriptions().Add(engine, &pluginserver.Subscription{
		Namespace: ns, EventType: state, Direction: events.DirBoth,
		PeerFilter: &pluginserver.PeerFilter{Selector: "192.0.2.1"}, Runtime: true,
	})
	require.Len(t, srv.PeerScopedProcs(ns, state, events.DirUnspecified, "192.0.2.1", "first"), 1)

	newPeers, err := PeersFromTree(deliveryTree(lookingGlass))
	require.NoError(t, err)
	rebuild := &reactorAPIAdapter{r: r, rebuild: true}
	require.NoError(t, rebuild.reconcilePeers(newPeers, Globals{}, dynamicGroupsNotRead, "test rebuild"))

	assert.Len(t, srv.PeerScopedProcs(ns, state, events.DirUnspecified, "192.0.2.1", "first"), 1,
		"a rebuild of the running config is no apply, so the live subscription stays")
}

// overrideProbeJournal is a configJournal that reports, at the moment the apply
// reaches its first peer change, how many processes the live override still
// adds. Record is where every peer change of a config operation runs, so the
// count it takes is the state a restarted session's first events meet.
type overrideProbeJournal struct {
	testJournal
	probe       func() int
	atPeerStage []int
}

func (j *overrideProbeJournal) Record(apply, undo func() error) error {
	j.atPeerStage = append(j.atPeerStage, j.probe())
	return j.testJournal.Record(apply, undo)
}

// TestConfigOperationDiscardsRuntimeSubscriptionsBeforePeerChanges is the R-10
// discard driven from the OTHER apply: the bgp root applies through config
// operations (bgp/plugin OnConfigOperationApply -> applyConfigOperation), not
// through reconcilePeersJournaled, so the discard there proves nothing here.
//
// The order matters as much as the call. A peer the operation restarts can
// reach Established inside the apply, and its first events MUST be delivered
// under the document's grant, so the override is gone before the journal runs
// the first peer change, not after.
//
// VALIDATES: R-10 on the config-operation path, and its order.
// PREVENTS: an override outliving a config apply (plugin test
// attach-process-runtime-subscribe), or being dropped only after the
// restarted session already emitted under it.
func TestConfigOperationDiscardsRuntimeSubscriptionsBeforePeerChanges(t *testing.T) {
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	setupDeliveryGraphTestServer(t, srv, "looking-glass", "route-injector")

	r := newTestReactor(t)
	r.api = srv
	addPeersFromTree(t, r, deliveryTree(lookingGlass))
	r.mu.Lock()
	r.publishDeliveryGraphLocked()
	r.mu.Unlock()

	ns := events.LookupNamespaceID(bgpevents.Namespace)
	state := events.LookupEventTypeID(bgpevents.EventState)
	engine := process.NewProcess(plugin.PluginConfig{Name: "looking-glass"})
	srv.Subscriptions().Add(engine, &pluginserver.Subscription{
		Namespace: ns, EventType: state, Direction: events.DirBoth,
		PeerFilter: &pluginserver.PeerFilter{Selector: "192.0.2.1"}, Runtime: true,
	})
	live := func() int {
		return len(srv.PeerScopedProcs(ns, state, events.DirUnspecified, "192.0.2.1", "first"))
	}
	require.Equal(t, 1, live(), "the permitted live capability addition must take effect")

	j := &overrideProbeJournal{probe: live}
	op := rpc.ConfigOperation{
		ID:     "bgp-add-peer-edge",
		Root:   "bgp",
		Owner:  "bgp",
		Type:   configop.AddPeer,
		Target: rpc.ResourceRef{Kind: rpc.ResourcePeer, Peer: "edge"},
		Params: rpc.ConfigOperationParams{
			Peer:    "edge",
			Address: "203.0.113.1",
			Config:  json.RawMessage(`{"connection":{"remote":{"ip":"203.0.113.1"},"local":{"ip":"192.0.2.9"}},"session":{"asn":{"local":"65000","remote":"65001"}}}`),
		},
	}
	_, err = (&reactorAPIAdapter{r: r}).applyConfigOperation(&op, j)
	require.NoError(t, err)

	require.NotEmpty(t, j.atPeerStage, "the operation must reach a peer change")
	assert.Equal(t, 0, j.atPeerStage[0], "the override is discarded before the first peer change")
	assert.Equal(t, 0, live(), "a config operation discards the live capability addition")
}

// TestRemovedPeerStaysInTheIndexUntilItsDownEvent pins the order removePeer
// keeps: the down event first, the republished index after it.
//
// The down event finds its receivers through the index
// (bgpserver.onPeerStateChange asks PeerScopedProcs by the peer's address), so
// the index it reads is the one in place when doRemovePeer, the locked phase
// that precedes the event, has returned. That index must still name the peer.
// When doRemovePeer republished it, the event reached no process the peer fed,
// and the RIB and the route server kept the removed peer's routes
// (test/plugin/api-peer-create-delete-rib.ci drives that end to end).
//
// VALIDATES: the locked removal phase leaves the peer's edges in the index, and
// the completed removal drops them.
// PREVENTS: a down event for a removed peer that no process receives.
func TestRemovedPeerStaysInTheIndexUntilItsDownEvent(t *testing.T) {
	srv := &pluginserver.Server{}
	setupDeliveryGraphTestServer(t, srv, "looking-glass", "route-injector")
	r := newTestReactor(t)
	r.api = srv
	r.eventDispatcher = nil
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	r.ctx = ctx
	r.running = true

	addPeersFromTree(t, r, deliveryTree(lookingGlass))
	r.mu.Lock()
	r.publishDeliveryGraphLocked()
	r.mu.Unlock()

	first := netip.MustParseAddr("192.0.2.1")
	removal, err := r.doRemovePeer(first, message.NotifyCeasePeerDeconfigured)
	require.NoError(t, err)
	require.NotNil(t, removal.removed)
	removal.stop.run()
	assert.Equal(t, []string{"looking-glass"},
		recv(srv.DeliveryGraph(), bgpevents.EventState, events.DirUnspecified, "192.0.2.1"),
		"the down event that follows must still find the process the peer fed")

	// The whole removal of the other peer republishes, and the index it
	// publishes is built from the reactor's peers, so neither peer is in it.
	require.NoError(t, r.RemovePeer(netip.MustParseAddr("192.0.2.2")))
	for _, pe := range srv.DeliveryGraph().Inspect() {
		assert.NotContains(t, []string{"192.0.2.1", "192.0.2.2"}, pe.Peer,
			"a completed removal leaves no edge for a removed peer")
	}
}

func TestCentralBusEndpointReloadReconcilesPeers(t *testing.T) {
	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	require.NoError(t, registry.Register(registry.Registration{
		Name:       "reload-added",
		RunEngine:  func(net.Conn) int { return 0 },
		CLIHandler: func([]string) int { return 0 },
		Receives:   []rpc.MessageReceiveDecl{{Message: "bgp:state"}},
	}))
	require.NoError(t, registry.Register(registry.Registration{
		Name:       "runtime-sub",
		RunEngine:  func(net.Conn) int { return 0 },
		CLIHandler: func([]string) int { return 0 },
		Receives: []rpc.MessageReceiveDecl{
			{Message: "bgp:state"},
			{Message: "bgp:keepalive"},
			{Message: "bgp:open"},
		},
	}))

	// Initial bus config: reload-added has from none, runtime-sub allows state+keepalive.
	srv.SetBusConfig(plugin.BusConfig{
		Endpoints: map[string]plugin.BusBlock{
			"reload-added": {
				Receive: plugin.BusDirectionBlock{TargetsPresent: true, TargetsNone: true},
			},
			"runtime-sub": {
				Receive: plugin.BusDirectionBlock{AllowPresent: true, Allow: []string{"bgp:state", "bgp:keepalive"}},
			},
		},
	})

	r := newTestReactor(t)
	r.api = srv
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	r.ctx = ctx
	r.running = true

	peerTree := map[string]any{
		"router-id": "10.0.0.1",
		"session":   map[string]any{"asn": map[string]any{"local": "65000"}},
		"peer": map[string]any{
			"peer1": map[string]any{
				"connection": map[string]any{
					"remote": map[string]any{"ip": "127.0.0.1", "connect": "false"},
					"local":  map[string]any{"ip": "127.0.0.1"},
				},
				"session": map[string]any{"asn": map[string]any{"remote": "65000"}},
			},
		},
	}
	addPeersFromTree(t, r, peerTree)
	r.mu.Lock()
	r.publishDeliveryGraphLocked()
	r.mu.Unlock()

	// Case 1: reload-added gains from [/bgp/peer/peer1] -> diffEffectivePeerEdges sees gained=["reload-added"], deliverable=true (hot-swap).
	before1 := r.DeliveryGraph()
	srv.SetBusConfig(plugin.BusConfig{
		Endpoints: map[string]plugin.BusBlock{
			"reload-added": {
				Receive: plugin.BusDirectionBlock{TargetsPresent: true, Targets: []string{"/bgp/peer/peer1"}},
			},
			"runtime-sub": {
				Receive: plugin.BusDirectionBlock{AllowPresent: true, Allow: []string{"bgp:state", "bgp:keepalive"}},
			},
		},
	})
	newPeers1, err := PeersFromTree(peerTree)
	require.NoError(t, err)
	after1, err := srv.PreviewDeliveryGraph(bgpevents.Namespace, deliveryPeersFromSettings(newPeers1))
	require.NoError(t, err)
	diff1 := diffEffectivePeerEdges(before1, after1, "127.0.0.1")
	require.True(t, diff1.changed, "adding /bgp/peer/peer1 on central bus endpoint must change effective edges")
	require.Equal(t, []string{"reload-added"}, diff1.gained)
	require.Contains(t, diff1.afterRecv["reload-added"], bgpevents.EventState)
	require.True(t, effectiveBusDeliverable(&diff1, nil))

	require.NoError(t, (&reactorAPIAdapter{r: r}).reconcilePeers(newPeers1, Globals{}, dynamicGroupsNotRead, "test reload 1"))
	assert.Contains(t, recv(srv.DeliveryGraph(), bgpevents.EventState, events.DirUnspecified, "127.0.0.1"), "reload-added")

	// Case 2: runtime-sub gains bgp:open (direction=received) -> gainedOpen=true, deliverable=false (restart).
	before2 := r.DeliveryGraph()
	srv.SetBusConfig(plugin.BusConfig{
		Endpoints: map[string]plugin.BusBlock{
			"reload-added": {
				Receive: plugin.BusDirectionBlock{TargetsPresent: true, Targets: []string{"/bgp/peer/peer1"}},
			},
			"runtime-sub": {
				Receive: plugin.BusDirectionBlock{
					AllowPresent: true,
					Allow:        []string{"bgp:state", "bgp:keepalive", "bgp:open"},
					Messages: map[string]*plugin.BusMessageRule{
						"bgp:open": {Tags: map[string][]string{"direction": {"received"}}},
					},
				},
			},
		},
	})
	after2, err := srv.PreviewDeliveryGraph(bgpevents.Namespace, deliveryPeersFromSettings(newPeers1))
	require.NoError(t, err)
	diff2 := diffEffectivePeerEdges(before2, after2, "127.0.0.1")
	require.True(t, diff2.changed, "gaining bgp:open on central bus endpoint must change effective edges")
	require.True(t, diff2.gainedOpen, "gaining bgp:open must set gainedOpen")
	require.False(t, effectiveBusDeliverable(&diff2, nil), "gaining bgp:open must force peer restart")
}
