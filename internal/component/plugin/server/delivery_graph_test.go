package server

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/events"
)

// graphNS is this file's own event namespace. The graph is namespace-agnostic,
// so the tests neither depend on the bgp registry nor disturb it.
const graphNS = "delivery-graph-test"

const (
	graphUpdate = "update"
	graphState  = "state"
)

func init() {
	_ = events.RegisterNamespace(graphNS, graphUpdate, graphState)
}

func graphIDs(t *testing.T) (events.NamespaceID, events.EventTypeID, events.EventTypeID) {
	t.Helper()
	ns := events.LookupNamespaceID(graphNS)
	require.NotEqual(t, events.NamespaceUnknown, ns)
	return ns, events.LookupEventTypeID(graphUpdate), events.LookupEventTypeID(graphState)
}

// twoPeerGraph is the shape story 1 in the spec describes: one peer feeds a
// program received UPDATEs and state and lets a second program announce, the
// other feeds a third program both directions of UPDATE.
func twoPeerGraph() *DeliveryGraph {
	return newDeliveryGraph(graphNS, twoPeerInput())
}

// twoPeerInput is the same shape as the reactor hands it over, for a test that
// publishes it into a server rather than building the index directly.
func twoPeerInput() []DeliveryPeer {
	return []DeliveryPeer{
		{Addr: "192.0.2.1", Name: "first", Bindings: []plugin.PeerProcessBinding{
			{
				PluginName: "looking-glass",
				Receive:    map[string]events.Direction{graphUpdate: events.DirReceived, graphState: events.DirBoth},
			},
			{PluginName: "route-injector", Send: map[string]bool{graphUpdate: true}},
		}},
		{Addr: "192.0.2.2", Name: "second", Bindings: []plugin.PeerProcessBinding{
			{PluginName: "policy-engine", Receive: map[string]events.Direction{graphUpdate: events.DirBoth}},
		}},
	}
}

// TestGraphFeedsOnlyAttachedProcesses verifies a peer's edges name exactly the
// processes its config attaches, in the direction each one was granted.
//
// VALIDATES: AC-2 both ways (a process with no block on a peer is fed nothing
// by it, and is still fed by the peer that does attach it) and AC-3 (a
// direction the list omits is not delivered).
// PREVENTS: the defect this spec exists to fix -- every process fed every
// peer's events whatever the config said.
func TestGraphFeedsOnlyAttachedProcesses(t *testing.T) {
	g := twoPeerGraph()
	ns, update, state := graphIDs(t)

	assert.Equal(t, []string{"looking-glass"}, g.Receivers(ns, update, events.DirReceived, "192.0.2.1"))
	assert.Equal(t, []string{"looking-glass"}, g.Receivers(ns, state, events.DirUnspecified, "192.0.2.1"))
	assert.Empty(t, g.Receivers(ns, update, events.DirSent, "192.0.2.1"),
		"the received direction alone was granted")
	assert.Empty(t, g.Receivers(ns, state, events.DirUnspecified, "192.0.2.2"),
		"policy-engine asked for update, not state")

	// policy-engine runs and is attached to 192.0.2.2, so it must not appear on
	// 192.0.2.1, and looking-glass must not appear on 192.0.2.2.
	assert.Equal(t, []string{"policy-engine"}, g.Receivers(ns, update, events.DirSent, "192.0.2.2"))
	assert.NotContains(t, g.Receivers(ns, update, events.DirReceived, "192.0.2.1"), "policy-engine")

	// A peer no config named has no edges. The guard fails closed: nothing, not
	// everything.
	assert.Empty(t, g.Receivers(ns, update, events.DirReceived, "198.51.100.1"))
	assert.Empty(t, g.Receivers(ns, update, events.DirUnspecified, ""))

	// The send permission is carried, and it feeds route-injector nothing.
	edges := g.Inspect()
	require.Len(t, edges, 2)
	assert.Equal(t, []string{"192.0.2.1", "192.0.2.2"}, []string{edges[0].Peer, edges[1].Peer})
	injector := edges[0].Processes[1]
	assert.Equal(t, "route-injector", injector.Process)
	assert.Equal(t, []string{graphUpdate}, injector.Send)
	assert.Empty(t, injector.Receive)
}

// TestGraphWildcardGrantsEveryRegisteredType verifies "*" names every type the
// registry holds, in both directions, and that a named type beside it takes
// nothing away.
//
// VALIDATES: the wildcard adopted in phase 2, expanded the same way the ready
// RPC expands a plugin's own "*" (dispatch.go).
// PREVENTS: "*" silently meaning "no type", which is what a literal reading of
// an empty Receive map would give.
func TestGraphWildcardGrantsEveryRegisteredType(t *testing.T) {
	g := newDeliveryGraph(graphNS, []DeliveryPeer{
		{Addr: "192.0.2.1", Bindings: []plugin.PeerProcessBinding{
			{
				PluginName: "exabgp",
				ReceiveAll: true,
				Receive:    map[string]events.Direction{graphUpdate: events.DirReceived},
			},
		}},
	})
	ns, update, state := graphIDs(t)

	assert.Equal(t, []string{"exabgp"}, g.Receivers(ns, state, events.DirSent, "192.0.2.1"))
	assert.Equal(t, []string{"exabgp"}, g.Receivers(ns, update, events.DirSent, "192.0.2.1"),
		"a named type beside the wildcard adds, it never narrows")
	assert.Equal(t, []string{"exabgp"}, g.Receivers(ns, update, events.DirReceived, "192.0.2.1"))
}

// TestGraphReportsUnresolvedTokens verifies a granted token the event registry
// does not know carries no edge, and is named where an operator can see it.
//
// VALIDATES: ai/rules/evidence.md -- a guard that drops an edge says so.
// PREVENTS: a custom event type whose plugin never loaded looking identical to
// a working edge.
func TestGraphReportsUnresolvedTokens(t *testing.T) {
	g := newDeliveryGraph(graphNS, []DeliveryPeer{
		{Addr: "192.0.2.1", Bindings: []plugin.PeerProcessBinding{
			{PluginName: "decorator", Receive: map[string]events.Direction{"update-rpki": events.DirReceived}},
		}},
	})
	ns, _, _ := graphIDs(t)

	assert.Empty(t, g.Receivers(ns, events.LookupEventTypeID("update-rpki"), events.DirReceived, "192.0.2.1"))
	edges := g.Inspect()
	require.Len(t, edges, 1)
	require.Len(t, edges[0].Processes, 1)
	assert.Equal(t, []string{"update-rpki-received"}, edges[0].Processes[0].Unresolved)
	assert.Empty(t, edges[0].Processes[0].Receive)
}

// TestGraphLookupAllocatesNothing verifies the per-event lookup returns a
// stored slice and allocates nothing.
//
// VALIDATES: AC-8 and R-8, proven by measurement rather than by inspection.
// The lookup runs once per peer-scoped event, on the UPDATE path, so an
// allocation here is an allocation per BGP message.
// PREVENTS: the cost SubscriptionManager.GetMatching has today -- a scan of
// every process and a fresh result slice on every event.
func TestGraphLookupAllocatesNothing(t *testing.T) {
	g := twoPeerGraph()
	ns, update, _ := graphIDs(t)

	var hit []string
	hits := testing.AllocsPerRun(100, func() {
		hit = g.Receivers(ns, update, events.DirReceived, "192.0.2.1")
	})
	require.NotEmpty(t, hit, "a lookup that finds nothing would make zero allocations vacuous")
	t.Logf("hit path: %v allocations per lookup", hits)
	assert.Zero(t, hits, "the hit path allocates")

	var miss []string
	misses := testing.AllocsPerRun(100, func() {
		miss = g.Receivers(ns, update, events.DirReceived, "198.51.100.1")
	})
	assert.Empty(t, miss)
	assert.Zero(t, misses, "the miss path allocates")
}

// TestGraphSwapIsAtomicAcrossReload verifies a reader takes the whole index in
// one load, so a rebuild shows it the old graph or the new one and never a
// half-built one.
//
// VALIDATES: R-7 and AC-4's second half -- an edge that survives the reload
// misses no event during it.
// PREVENTS: a reload window in which a peer that feeds a process before and
// after the reload feeds it nothing during.
// Run with -race.
func TestGraphSwapIsAtomicAcrossReload(t *testing.T) {
	s := &Server{}
	ns, update, _ := graphIDs(t)

	graphFor := func(process string) []DeliveryPeer {
		return []DeliveryPeer{{Addr: "192.0.2.1", Bindings: []plugin.PeerProcessBinding{
			{PluginName: process, Receive: map[string]events.Direction{graphUpdate: events.DirReceived}},
		}}}
	}
	s.UpdateDeliveryGraph(graphNS, graphFor("alpha"))

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
			s.UpdateDeliveryGraph(graphNS, graphFor("beta"))
			continue
		}
		s.UpdateDeliveryGraph(graphNS, graphFor("alpha"))
	}
	close(stop)
	wg.Wait()

	assert.Equal(t, []string{"alpha"}, s.DeliveryGraph().Receivers(ns, update, events.DirReceived, "192.0.2.1"))
}

// TestEmptyGraphFeedsNobody verifies a server that has applied no config feeds
// nothing, rather than answering nil and being read as "no filter".
//
// VALIDATES: ai/rules/evidence.md -- the guard fails closed.
// PREVENTS: a daemon whose index is not published yet delivering every event to
// every process.
func TestEmptyGraphFeedsNobody(t *testing.T) {
	s := &Server{}
	ns, update, _ := graphIDs(t)

	g := s.DeliveryGraph()
	require.NotNil(t, g, "the accessor must never hand back nil")
	assert.Empty(t, g.Receivers(ns, update, events.DirReceived, "192.0.2.1"))
	assert.Empty(t, g.Inspect())
	assert.Zero(t, g.Len())
}
