package reactor

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
)

// deliveryTree is the resolved bgp tree both wiring tests load. Two peers, two
// programs, three relationships: 192.0.2.1 feeds the program firstAttach names,
// 192.0.2.2 feeds nothing and lets route-injector announce.
func deliveryTree(firstAttach map[string]any) map[string]any {
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
				"attach":  map[string]any{"process": firstAttach},
			},
			"second": map[string]any{
				"connection": map[string]any{
					"remote": map[string]any{"ip": "192.0.2.2", "connect": "false"},
					"local":  map[string]any{"ip": "auto"},
				},
				"session": map[string]any{"asn": map[string]any{"remote": "65002"}},
				"attach": map[string]any{"process": map[string]any{
					"route-injector": map[string]any{"send": "update"},
				}},
			},
		},
	}
}

// lookingGlass is the first peer's attach block before the reload: it is fed
// received UPDATEs and session state, and may announce nothing.
var lookingGlass = map[string]any{
	"looking-glass": map[string]any{"receive": "update-received state"},
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
// its group's attach blocks.
//
// The chain is three functions and this test drives all three. The group's
// `attach process` block is read by the SAME parser a static peer's is
// (ParseDynamicGroupTemplate -> parsePeerSettings -> parseProcessBindingsFromTree),
// buildDynamicPeerSettings copies the template whole so ProcessBindings arrives
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
		"attach": map[string]any{"process": map[string]any{
			"looking-glass": map[string]any{"receive": "state"},
		}},
	}, 65000, 0x0A000001)
	require.NoError(t, err)
	require.Len(t, tmpl.ProcessBindings, 1, "the group's attach block is parsed by the peer parser")

	dg := &DynamicGroupConfig{
		GroupName: "ix",
		Ranges:    []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")},
		Settings:  tmpl,
	}
	member, err := (&Reactor{}).buildDynamicPeerSettings(dg, netip.MustParseAddr("192.0.2.7"))
	require.NoError(t, err)

	// The member holds the group's bindings, unchanged. This is the field R-2
	// assumed a dynamic peer could not carry, and TestDynamicPeerInheritsEvery
	// PeerSettingsField refuses a divergence row for it.
	require.Equal(t, tmpl.ProcessBindings, member.ProcessBindings)
	require.Equal(t, "dyn-192.0.2.7", member.Name, "no config document holds this name")

	srv := &pluginserver.Server{}
	srv.UpdateDeliveryGraph(bgpevents.Namespace, DeliveryPeersFromSettings([]*PeerSettings{member}))
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
// ProcessBindings is outside hotSwappableSettings (peer_settings_apply.go), and
// both republish the index while the reactor runs.
//
// VALIDATES: AC-4's first half, that delivery edges follow the new config.
// PREVENTS: an index that answers from the config the daemon started with.
func TestReloadRepublishesDeliveryGraph(t *testing.T) {
	srv := &pluginserver.Server{}
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
		"policy-engine": map[string]any{"receive": "update-received"},
	})
	newPeers, err := PeersFromTree(next)
	require.NoError(t, err)
	require.NoError(t, (&reactorAPIAdapter{r: r}).reconcilePeers(newPeers, "test reload"))

	g := srv.DeliveryGraph()
	assert.Empty(t, recv(g, bgpevents.EventState, events.DirUnspecified, "192.0.2.1"),
		"the replaced list no longer grants state")
	assert.Equal(t, []string{"policy-engine"}, recv(g, bgpevents.EventUpdate, events.DirReceived, "192.0.2.1"))
	assert.Equal(t, []pluginserver.ProcessEdges{{Process: "route-injector", Send: []string{"update"}}},
		peerView(t, g, "192.0.2.2").Processes, "the untouched peer keeps its send edge")
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
	require.NoError(t, (&reactorAPIAdapter{r: r}).reconcilePeers(newPeers, "test reload"))

	assert.Empty(t, srv.PeerScopedProcs(ns, state, events.DirUnspecified, "192.0.2.1", "first"),
		"a config apply discards the live capability addition")
}
