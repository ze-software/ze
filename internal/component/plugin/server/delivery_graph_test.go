// Design: docs/architecture/api/architecture.md -- compiled bus DeliveryGraph tests and benchmarks
// Related: delivery_graph.go -- BuildDeliveryGraph, DeliveryGraph, Server.SetDeliveryGraph
// Related: bus_filter.go -- CompiledBus

package server

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// graphNS is this file's own event namespace. The graph is namespace-agnostic,
// so the tests neither depend on the bgp registry nor disturb it.
const graphNS = "delivery-graph-test"

const (
	graphUpdate = "update"
	graphState  = "state"
	graphRaw    = "raw"
)

func init() {
	_ = events.RegisterNamespace(graphNS, graphUpdate, graphState, graphRaw)
}

func graphIDs(t *testing.T) (events.NamespaceID, events.EventTypeID, events.EventTypeID) {
	t.Helper()
	ns := events.LookupNamespaceID(graphNS)
	require.NotEqual(t, events.NamespaceUnknown, ns)
	return ns, events.LookupEventTypeID(graphUpdate), events.LookupEventTypeID(graphState)
}

func graphMsg(name string) string {
	return graphNS + ":" + name
}

func graphDirTag(dirs ...string) []rpc.MessageTagDecl {
	return []rpc.MessageTagDecl{{
		Name:   events.TagDirection,
		Values: dirs,
	}}
}

// twoPeerInput returns two address participants used across delivery_graph_test
// and delivery_filter_test.
func twoPeerInput() []DeliveryPeer {
	return []DeliveryPeer{
		{
			Addr:           "192.0.2.1",
			Name:           "first",
			Path:           "/" + graphNS + "/entry/first",
			SecondaryPaths: []string{"/" + graphNS + "/ip/192.0.2.1"},
		},
		{
			Addr:           "192.0.2.2",
			Name:           "second",
			Path:           "/" + graphNS + "/entry/second",
			SecondaryPaths: []string{"/" + graphNS + "/ip/192.0.2.2"},
		},
	}
}

// twoPeerEndpoints returns the three endpoints matching the two-participant
// fixture: looking-glass receives update(received) and state from first;
// route-injector sends update to first; policy-engine receives update(both)
// from second.
func twoPeerEndpoints() []BusParticipantInput {
	return []BusParticipantInput{
		{
			Path:         plugin.EndpointPath("looking-glass"),
			EndpointName: "looking-glass",
			Receives: []rpc.MessageReceiveDecl{
				{Message: graphMsg(graphUpdate), Tags: graphDirTag(events.DirectionReceived)},
				{Message: graphMsg(graphState)},
			},
			OwnBlock: plugin.BusBlock{
				Receive: plugin.BusDirectionBlock{
					TargetsPresent: true,
					Targets:        []string{"/" + graphNS + "/entry/first"},
				},
			},
		},
		{
			Path:         plugin.EndpointPath("route-injector"),
			EndpointName: "route-injector",
			Sends: []rpc.MessageSendDecl{
				{Message: graphMsg(graphUpdate)},
			},
			OwnBlock: plugin.BusBlock{
				Send: plugin.BusDirectionBlock{
					TargetsPresent: true,
					Targets:        []string{"/" + graphNS + "/entry/first"},
				},
			},
		},
		{
			Path:         plugin.EndpointPath("policy-engine"),
			EndpointName: "policy-engine",
			Receives: []rpc.MessageReceiveDecl{
				{Message: graphMsg(graphUpdate), Tags: graphDirTag(events.DirectionReceived, events.DirectionSent)},
			},
			OwnBlock: plugin.BusBlock{
				Receive: plugin.BusDirectionBlock{
					TargetsPresent: true,
					Targets:        []string{"/" + graphNS + "/entry/second"},
				},
			},
		},
	}
}

func twoPeerGraph(t testing.TB) *DeliveryGraph {
	t.Helper()
	g, err := BuildDeliveryGraph(graphNS, twoPeerInput(), plugin.BusConfig{}, twoPeerEndpoints())
	require.NoError(t, err)
	return g
}

// TestGraphFeedsOnlyAdmittedEndpoints verifies that each participant feeds
// only endpoints admitted by the compiled bus filter in the declared direction.
//
// VALIDATES: Receivers, MaySend, SendersForPeer, ProcessReceivesState,
// ParticipantPathForAddr, ProcessListForPeer, and Inspect over CompiledBus.
// PREVENTS: delivering events across narrowed to/from boundaries or wrong
// directions, or failing open on unknown addresses.
func TestGraphFeedsOnlyAdmittedEndpoints(t *testing.T) {
	g := twoPeerGraph(t)
	ns, update, state := graphIDs(t)

	assert.Equal(t, []string{"looking-glass"}, g.Receivers(ns, update, events.DirReceived, "192.0.2.1"))
	assert.Equal(t, []string{"looking-glass"}, g.ReceiversForPeer("192.0.2.1", ns, state, events.DirUnspecified))
	assert.Empty(t, g.Receivers(ns, update, events.DirSent, "192.0.2.1"),
		"looking-glass only receives direction received")
	assert.Empty(t, g.Receivers(ns, state, events.DirUnspecified, "192.0.2.2"),
		"policy-engine declared update, not state")

	assert.Equal(t, []string{"policy-engine"}, g.Receivers(ns, update, events.DirSent, "192.0.2.2"))
	assert.NotContains(t, g.Receivers(ns, update, events.DirReceived, "192.0.2.1"), "policy-engine")

	assert.True(t, g.MaySend("192.0.2.1", "route-injector", graphUpdate))
	assert.True(t, g.MaySend("192.0.2.1", "route-injector", graphMsg(graphUpdate)))
	assert.False(t, g.MaySend("192.0.2.2", "route-injector", graphUpdate))
	assert.False(t, g.MaySend("192.0.2.1", "looking-glass", graphUpdate))
	assert.Equal(t, []string{"route-injector"}, g.SendersForPeer("192.0.2.1", graphUpdate))
	assert.Equal(t, []string{"route-injector"}, g.SendersForPeer("192.0.2.1", graphMsg(graphUpdate)))
	assert.True(t, g.ProcessReceivesState("192.0.2.1", "looking-glass"))
	assert.False(t, g.ProcessReceivesState("192.0.2.1", "route-injector"))
	assert.Equal(t, "/"+graphNS+"/entry/first", g.ParticipantPathForAddr("192.0.2.1"))
	assert.Equal(t, []string{"looking-glass", "route-injector"}, g.ProcessListForPeer("192.0.2.1"))

	// Unknown address fails closed.
	assert.Empty(t, g.Receivers(ns, update, events.DirReceived, "198.51.100.1"))
	assert.Empty(t, g.Receivers(ns, update, events.DirUnspecified, ""))
	assert.False(t, g.MaySend("198.51.100.1", "route-injector", graphUpdate))

	// Stage2FilterFor carries resolved remote addresses alongside paths.
	f := g.Stage2FilterFor("route-injector")
	require.NotNil(t, f)
	require.Len(t, f.SendRules, 1)
	require.Len(t, f.SendRules[0].Targets, 1)
	assert.Equal(t, "/"+graphNS+"/entry/first", f.SendRules[0].Targets[0].Path)
	assert.Equal(t, "192.0.2.1", f.SendRules[0].Targets[0].Address)

	// Inspect surfaces deterministic edges with compiled origins.
	edges := g.Inspect()
	require.Len(t, edges, 2)
	assert.Equal(t, []string{"192.0.2.1", "192.0.2.2"}, []string{edges[0].Peer, edges[1].Peer})
	require.Len(t, edges[0].Processes, 2)
	injector := edges[0].Processes[1]
	assert.Equal(t, "route-injector", injector.Process)
	assert.Equal(t, []string{graphUpdate}, injector.Send)
	assert.Empty(t, injector.Receive)
}

// TestDeliveryIndexFromFilters verifies that newly added and container-parented
// participants are covered by prefix paths and secondary address paths without
// modifying endpoint configuration.
//
// VALIDATES: AC-1, AC-21 (dynamic container coverage and per-address rules).
// PREVENTS: a newly added participant or parented member missing compiled
// delivery edges or leaking across sibling containers.
func TestDeliveryIndexFromFilters(t *testing.T) {
	ns, update, _ := graphIDs(t)
	endpoints := []BusParticipantInput{
		{
			Path:         plugin.EndpointPath("container-watcher"),
			EndpointName: "container-watcher",
			Receives:     []rpc.MessageReceiveDecl{{Message: graphMsg(graphUpdate)}},
			OwnBlock: plugin.BusBlock{
				Receive: plugin.BusDirectionBlock{
					TargetsPresent: true,
					Targets:        []string{"/" + graphNS + "/container/c1"},
				},
			},
		},
		{
			Path:         plugin.EndpointPath("addr-watcher"),
			EndpointName: "addr-watcher",
			Receives:     []rpc.MessageReceiveDecl{{Message: graphMsg(graphUpdate)}},
			OwnBlock: plugin.BusBlock{
				Receive: plugin.BusDirectionBlock{
					TargetsPresent: true,
					Targets:        []string{"/" + graphNS + "/ip/198.51.100.7"},
				},
			},
		},
	}

	g1, err := BuildDeliveryGraph(graphNS, []DeliveryPeer{
		{
			Addr:           "198.51.100.5",
			Name:           "m1",
			Path:           "/" + graphNS + "/container/c1/member/m1",
			ParentPath:     "/" + graphNS + "/container/c1",
			SecondaryPaths: []string{"/" + graphNS + "/ip/198.51.100.5"},
		},
		{
			Addr:           "198.51.100.9",
			Name:           "m2",
			Path:           "/" + graphNS + "/container/c2/member/m2",
			ParentPath:     "/" + graphNS + "/container/c2",
			SecondaryPaths: []string{"/" + graphNS + "/ip/198.51.100.9"},
		},
	}, plugin.BusConfig{}, endpoints)
	require.NoError(t, err)

	assert.Equal(t, []string{"container-watcher"}, g1.Receivers(ns, update, events.DirReceived, "198.51.100.5"))
	assert.Empty(t, g1.Receivers(ns, update, events.DirReceived, "198.51.100.9"))

	// Once 198.51.100.7 joins c1, both container-watcher and addr-watcher see it;
	// 198.51.100.8 in c1 is seen by container-watcher alone.
	g2, err := BuildDeliveryGraph(graphNS, []DeliveryPeer{
		{
			Addr:           "198.51.100.7",
			Name:           "m7",
			Path:           "/" + graphNS + "/container/c1/member/m7",
			ParentPath:     "/" + graphNS + "/container/c1",
			SecondaryPaths: []string{"/" + graphNS + "/ip/198.51.100.7"},
		},
		{
			Addr:           "198.51.100.8",
			Name:           "m8",
			Path:           "/" + graphNS + "/container/c1/member/m8",
			ParentPath:     "/" + graphNS + "/container/c1",
			SecondaryPaths: []string{"/" + graphNS + "/ip/198.51.100.8"},
		},
	}, plugin.BusConfig{}, endpoints)
	require.NoError(t, err)

	assert.Equal(t, []string{"addr-watcher", "container-watcher"}, g2.Receivers(ns, update, events.DirReceived, "198.51.100.7"))
	assert.Equal(t, []string{"container-watcher"}, g2.Receivers(ns, update, events.DirReceived, "198.51.100.8"))

	added, removed := g1.EdgeDiff(g2, "198.51.100.7")
	assert.Equal(t, []string{"addr-watcher", "container-watcher"}, added)
	assert.Empty(t, removed)
}

// TestDeliveryGraphAllocatesNothing verifies that hot-path Receivers and MaySend
// lookups over the compiled DeliveryGraph allocate zero heap objects.
//
// VALIDATES: AC-17 (zero allocation on broadcast lookup and send check).
// PREVENTS: per-event heap allocation or runtime path parsing on the hot path.
func TestDeliveryGraphAllocatesNothing(t *testing.T) {
	g := twoPeerGraph(t)
	ns, update, _ := graphIDs(t)

	var hit []string
	hits := testing.AllocsPerRun(100, func() {
		hit = g.Receivers(ns, update, events.DirReceived, "192.0.2.1")
	})
	require.NotEmpty(t, hit)
	assert.Zero(t, hits, "Receivers hit path must not allocate")

	var miss []string
	misses := testing.AllocsPerRun(100, func() {
		miss = g.Receivers(ns, update, events.DirReceived, "198.51.100.1")
	})
	assert.Empty(t, miss)
	assert.Zero(t, misses, "Receivers miss path must not allocate")

	var allowed bool
	sendAllocs := testing.AllocsPerRun(100, func() {
		allowed = g.MaySend("192.0.2.1", "route-injector", graphUpdate)
	})
	require.True(t, allowed)
	assert.Zero(t, sendAllocs, "MaySend hit path must not allocate")
}

// BenchmarkDeliveryGraph benchmarks hot-path Receivers and MaySend lookups over
// the compiled DeliveryGraph.
func BenchmarkDeliveryGraph(b *testing.B) {
	g := twoPeerGraph(b)
	ns := events.LookupNamespaceID(graphNS)
	update := events.LookupEventTypeID(graphUpdate)

	b.Run("Receivers", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			_ = g.Receivers(ns, update, events.DirReceived, "192.0.2.1")
		}
	})
	b.Run("MaySend", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			_ = g.MaySend("192.0.2.1", "route-injector", graphUpdate)
		}
	})
}

// TestGraphSwapIsAtomicAcrossReload verifies a reader takes the whole index in
// one load, so a rebuild shows either the old graph or the new one and never a
// half-built one.
//
// VALIDATES: R-7 atomic snapshot publication under concurrent readers.
// PREVENTS: a reload window with an empty or half-built delivery graph.
func TestGraphSwapIsAtomicAcrossReload(t *testing.T) {
	s := &Server{}
	ns, update, _ := graphIDs(t)

	peers := []DeliveryPeer{{Addr: "192.0.2.1", Path: "/" + graphNS + "/entry/first"}}
	endpointsFor := func(name string) []BusParticipantInput {
		return []BusParticipantInput{{
			Path:         plugin.EndpointPath(name),
			EndpointName: name,
			Receives:     []rpc.MessageReceiveDecl{{Message: graphMsg(graphUpdate), Tags: graphDirTag(events.DirectionReceived)}},
		}}
	}
	require.NoError(t, s.UpdateDeliveryGraphWithEndpoints(graphNS, peers, endpointsFor("alpha")))

	const reloads = 200
	var wg sync.WaitGroup
	stop := make(chan struct{})

	for range 4 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				got := s.DeliveryGraph().Receivers(ns, update, events.DirReceived, "192.0.2.1")
				if len(got) != 1 || (got[0] != "alpha" && got[0] != "beta") {
					assert.Fail(t, "reader saw a half-built index", "got %v", got)
					return
				}
			}
		})
	}

	for i := range reloads {
		if i%2 == 0 {
			require.NoError(t, s.UpdateDeliveryGraphWithEndpoints(graphNS, peers, endpointsFor("beta")))
			continue
		}
		require.NoError(t, s.UpdateDeliveryGraphWithEndpoints(graphNS, peers, endpointsFor("alpha")))
	}
	close(stop)
	wg.Wait()

	assert.Equal(t, []string{"alpha"}, s.DeliveryGraph().Receivers(ns, update, events.DirReceived, "192.0.2.1"))
}

// TestEmptyGraphFeedsNobody verifies a server that has applied no config feeds
// nothing rather than failing open.
//
// VALIDATES: empty DeliveryGraph fails closed.
// PREVENTS: an uninitialized server delivering events to every endpoint.
func TestEmptyGraphFeedsNobody(t *testing.T) {
	s := &Server{}
	ns, update, _ := graphIDs(t)

	g := s.DeliveryGraph()
	require.NotNil(t, g, "the accessor must never hand back nil")
	assert.Empty(t, g.Receivers(ns, update, events.DirReceived, "192.0.2.1"))
	assert.Empty(t, g.Inspect())
	assert.Zero(t, g.Len())
}
