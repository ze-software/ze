package server

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/events"
)

// filterServer builds a server whose index is the two-peer graph of the spec's
// first story, with every named process running and subscribed to BOTH types in
// both directions. The plugin half is therefore never what narrows delivery:
// anything the filter drops, the CONFIG dropped.
func filterServer(t *testing.T, names ...string) (*Server, map[string]*process.Process) {
	t.Helper()
	ns, update, state := graphIDs(t)

	s := &Server{subscriptions: newSubscriptionManager()}
	s.UpdateDeliveryGraph(graphNS, twoPeerInput())

	procs := make(map[string]*process.Process, len(names))
	for _, name := range names {
		proc := process.NewProcess(plugin.PluginConfig{Name: name})
		procs[name] = proc
		for _, et := range []events.EventTypeID{update, state} {
			s.subscriptions.Add(proc, &Subscription{Namespace: ns, EventType: et, Direction: events.DirBoth})
		}
	}
	return s, procs
}

// procNames is what the delivery sites care about: which processes take the
// event, by name.
func procNames(procs []*process.Process) []string {
	out := make([]string, 0, len(procs))
	for _, p := range procs {
		out = append(out, p.Name())
	}
	return out
}

// TestPeerScopedProcsIsTheOverlapOfBothHalves verifies the delivery funnel hands
// each process only the event types the peer's own attach block grants it, even
// though every process subscribed to everything.
//
// VALIDATES: AC-1 (two processes on one peer, different lists, independent
// results), AC-3 (a type the list omits is not delivered) and AC-7 (a type the
// plugin never declared is not delivered).
// PREVENTS: the defect this spec exists to fix -- delivery decided by what each
// plugin asked for at startup, with the config ignored.
func TestPeerScopedProcsIsTheOverlapOfBothHalves(t *testing.T) {
	s, _ := filterServer(t, "looking-glass", "route-injector", "policy-engine")
	ns, update, state := graphIDs(t)

	assert.Equal(t, []string{"looking-glass"},
		procNames(s.PeerScopedProcs(ns, update, events.DirReceived, "192.0.2.1", "first")))
	assert.Equal(t, []string{"looking-glass"},
		procNames(s.PeerScopedProcs(ns, state, events.DirUnspecified, "192.0.2.1", "first")))
	assert.Empty(t, s.PeerScopedProcs(ns, update, events.DirSent, "192.0.2.1", "first"),
		"update-received grants the inbound half only")

	// route-injector is attached to the same peer with a send permission and no
	// receive line, so the same events reach it not at all.
	assert.NotContains(t, procNames(s.PeerScopedProcs(ns, update, events.DirReceived, "192.0.2.1", "first")),
		"route-injector")

	// The second peer's list is its own: policy-engine takes update in both
	// directions and no state.
	assert.Equal(t, []string{"policy-engine"},
		procNames(s.PeerScopedProcs(ns, update, events.DirSent, "192.0.2.2", "second")))
	assert.Empty(t, s.PeerScopedProcs(ns, state, events.DirUnspecified, "192.0.2.2", "second"),
		"policy-engine asked for update, not state")
}

// TestPeerScopedProcsFeedsNobodyForAnUnattachedPeer verifies a peer the config
// never named feeds nothing, while the peers that do attach a process still
// feed it.
//
// VALIDATES: AC-2, both halves, and the fail-closed guard: an index that cannot
// find a peer answers nothing, never everything (ai/rules/evidence.md).
// PREVENTS: a dynamic or unconfigured peer broadcasting to every running plugin.
func TestPeerScopedProcsFeedsNobodyForAnUnattachedPeer(t *testing.T) {
	s, _ := filterServer(t, "looking-glass")
	ns, update, _ := graphIDs(t)

	assert.Empty(t, s.PeerScopedProcs(ns, update, events.DirReceived, "198.51.100.1", "stranger"),
		"a peer the config never named feeds nobody")
	assert.NotEmpty(t, s.PeerScopedProcs(ns, update, events.DirReceived, "192.0.2.1", "first"),
		"the peer that does attach it is still served")

	// An empty index is the state before the first config apply. It feeds
	// nothing rather than everything.
	fresh := &Server{subscriptions: newSubscriptionManager()}
	proc := process.NewProcess(plugin.PluginConfig{Name: "looking-glass"})
	fresh.subscriptions.Add(proc, &Subscription{Namespace: ns, EventType: update, Direction: events.DirBoth})
	assert.Empty(t, fresh.PeerScopedProcs(ns, update, events.DirReceived, "192.0.2.1", "first"))
}

// TestPeerScopedProcsAddsNoAllocation verifies the filter costs no allocation
// beyond the one GetMatching already makes for its result.
//
// VALIDATES: AC-8 and R-8 over the WHOLE funnel rather than over the index
// alone: the graph lookup returns a stored slice and the survivors are
// compacted into the slice that already exists.
// PREVENTS: an allocation per delivered BGP message, which is what a fresh
// result slice per filtered lookup would be.
func TestPeerScopedProcsAddsNoAllocation(t *testing.T) {
	s, _ := filterServer(t, "looking-glass", "route-injector")
	ns, update, _ := graphIDs(t)

	var raw []*process.Process
	baseline := testing.AllocsPerRun(100, func() {
		raw = s.subscriptions.GetMatching(ns, update, events.DirReceived, "192.0.2.1", "first")
	})
	require.Len(t, raw, 2, "both processes subscribe; the config is what narrows them")

	var kept []*process.Process
	filtered := testing.AllocsPerRun(100, func() {
		kept = s.PeerScopedProcs(ns, update, events.DirReceived, "192.0.2.1", "first")
	})
	require.Len(t, kept, 1, "zero allocations would be vacuous if the lookup found nothing")
	t.Logf("GetMatching alone: %v allocations; filtered: %v", baseline, filtered)
	assert.Equal(t, baseline, filtered, "the filter allocates")
}

// TestRuntimeSubscribeSurvivesAPublishThatIsNotAnApply verifies the live half of
// the precedence rule: a runtime subscription is delivered where the config
// grants nothing, and a republish that is not a config apply leaves it standing.
//
// UpdateDeliveryGraph is what every peer change calls -- AddPeer, doRemovePeer,
// createDynamicPeer and removeDynamicPeer (bgp/reactor/delivery_graph.go) -- so a
// route server accepting one inbound dynamic peer republishes the index. The
// discard belongs to the apply alone (DiscardRuntimeSubscriptions), and the other
// half of R-10 is proven where that apply happens, in
// TestConfigApplyDiscardsRuntimeSubscriptions (bgp/reactor/delivery_graph_test.go).
//
// VALIDATES: AC-5 and R-10, the override direction.
// PREVENTS: an operator's live `request subscribe` silently doing nothing once
// the config is authoritative, and the discard moving back into this publish,
// where a dynamic peer connecting would cancel the subscription an operator
// typed to watch a live session, with no message.
func TestRuntimeSubscribeSurvivesAPublishThatIsNotAnApply(t *testing.T) {
	s, procs := filterServer(t, "policy-engine")
	ns, _, state := graphIDs(t)
	engine := procs["policy-engine"]

	// The config grants policy-engine update on 192.0.2.2 and no state.
	require.Empty(t, s.PeerScopedProcs(ns, state, events.DirUnspecified, "192.0.2.2", "second"))

	s.subscriptions.Add(engine, &Subscription{
		Namespace: ns, EventType: state, Direction: events.DirBoth,
		PeerFilter: &PeerFilter{Selector: "192.0.2.2"}, Runtime: true,
	})
	assert.Equal(t, []string{"policy-engine"},
		procNames(s.PeerScopedProcs(ns, state, events.DirUnspecified, "192.0.2.2", "second")),
		"a live override is delivered where the config grants nothing")
	assert.Empty(t, s.PeerScopedProcs(ns, state, events.DirUnspecified, "192.0.2.1", "first"),
		"the override names one peer and reaches no other")

	// A peer joining republishes the index. It is not an apply, so the override
	// is still delivered afterwards and is still counted as live.
	s.UpdateDeliveryGraph(graphNS, twoPeerInput())
	assert.Equal(t, []string{"policy-engine"},
		procNames(s.PeerScopedProcs(ns, state, events.DirUnspecified, "192.0.2.2", "second")),
		"a publish that is not a config apply must leave the override standing")
	assert.True(t, s.subscriptions.hasRuntimeOverride())
}

// TestEmittedPeerScopedEventIsFilteredByTheGraph drives the emit-event rail,
// which is the second producer of peer-scoped delivery beside the reactor's own
// seven sites.
//
// deliverEvent branches on the peer ADDRESS: an event that carries one is
// peer-scoped and goes through the graph, an event that carries none is not and
// keeps flowing to everything subscribed. Both in-tree emitters pass an address
// today, so the second branch is not reachable from a plugin, but the first one
// is the fix for a round-1 BLOCKER: this path used to call GetMatching directly,
// and no attach block decided anything for the two bgp events that travel it.
//
// VALIDATES: an emitted peer-scoped event reaches only the processes that peer
// attaches, and the no-peer branch is not narrowed by the graph.
// PREVENTS: the filter being reinstated on the reactor's sites alone, leaving
// bgp/update-rpki and bgp/rpki delivered to every subscriber.
func TestEmittedPeerScopedEventIsFilteredByTheGraph(t *testing.T) {
	s, procs := filterServer(t, "looking-glass", "route-injector")
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	for _, proc := range procs {
		proc.StartDelivery(ctx)
	}
	// A third process, so neither subscriber is skipped as the emitter.
	emitter := process.NewProcess(plugin.PluginConfig{Name: "emitter"})

	// 192.0.2.1 grants looking-glass update-received and grants route-injector
	// nothing, though both subscribed to everything.
	delivered, err := s.deliverEvent(emitter, graphNS, graphUpdate, events.DirectionReceived, "192.0.2.1", "{}")
	require.NoError(t, err)
	assert.Equal(t, 1, delivered, "only the process the peer attaches takes delivery")

	// The same event with no peer: no attach block describes it, so it is not
	// narrowed. This also proves the assertion above is not measuring a
	// subscription both processes lack.
	delivered, err = s.deliverEvent(emitter, graphNS, graphUpdate, events.DirectionReceived, "", "{}")
	require.NoError(t, err)
	assert.Equal(t, 2, delivered, "an event with no peer keeps flowing to every subscriber")
}

// TestReconcileNamesEachDisagreementAndIsSilentOnAgreement verifies the report
// that gives R-9 a voice: what the config grants and what the plugin declared
// are compared once both are known, and every triple they disagree about is
// named.
//
// VALIDATES: AC-7 (granted, never declared), AC-7b (declared, never granted),
// a direction pair that never meets, and R-12 (silence when they agree).
// PREVENTS: a filter an operator cannot debug -- today neither half can tell
// them the other one disagrees, so the event simply never arrives.
func TestReconcileNamesEachDisagreementAndIsSilentOnAgreement(t *testing.T) {
	ns, update, state := graphIDs(t)

	s := &Server{subscriptions: newSubscriptionManager()}
	s.UpdateDeliveryGraph(graphNS, []DeliveryPeer{{
		Addr: "192.0.2.1", Name: "first",
		Bindings: []plugin.PeerProcessBinding{{
			PluginName: "policy-engine",
			Receive:    map[string]events.Direction{graphUpdate: events.DirReceived, graphState: events.DirBoth},
		}},
	}})

	// Nothing has declared yet: the plugin half is unknown, so there is nothing
	// to disagree with and nothing to say.
	assert.Empty(t, s.deliveryDisagreements(nil), "a process that has not declared is not a disagreement")

	engine := process.NewProcess(plugin.PluginConfig{Name: "policy-engine"})
	s.subscriptions.Add(engine, &Subscription{Namespace: ns, EventType: update, Direction: events.DirSent})

	// One line per disagreeing triple, in the order the operator surface prints
	// the granted tokens, which is sorted.
	found := s.deliveryDisagreements(nil)
	require.Len(t, found, 2)
	assert.Equal(t, deliveryDisagreement{
		peer: "192.0.2.1", process: "policy-engine",
		reason: reasonUndeclared, granted: graphState,
	}, found[0], "the config grants state and the plugin never asked for it")
	assert.Equal(t, deliveryDisagreement{
		peer: "192.0.2.1", process: "policy-engine",
		reason: reasonDirection, granted: "update-received", declared: "update-sent",
	}, found[1], "the type agrees and the directions never meet, so nothing is delivered")

	// The plugin declares a type for this peer that the peer does not grant.
	s.subscriptions.Add(engine, &Subscription{
		Namespace: ns, EventType: state, Direction: events.DirBoth,
		PeerFilter: &PeerFilter{Selector: "192.0.2.1"},
	})
	s.subscriptions.Add(engine, &Subscription{
		Namespace: ns, EventType: update, Direction: events.DirReceived,
	})
	assert.Empty(t, s.deliveryDisagreements(nil), "both halves now agree, so the report says nothing")

	// A peer the config does not attach the process to is not reported: the
	// operator asked for nothing there, so there is nothing to reconcile.
	s.UpdateDeliveryGraph(graphNS, []DeliveryPeer{
		{Addr: "192.0.2.1", Name: "first", Bindings: []plugin.PeerProcessBinding{{
			PluginName: "policy-engine",
			Receive:    map[string]events.Direction{graphUpdate: events.DirBoth, graphState: events.DirBoth},
		}}},
		{Addr: "192.0.2.9", Name: "ninth"},
	})
	assert.Empty(t, s.deliveryDisagreements(nil))
}

// TestReconcileNamesAProcessNoPeerAttaches verifies the one disagreement that
// has no peer to name: a running program that declared events and appears in no
// attach block anywhere in the config.
//
// VALIDATES: AC-7b at whole-process scope. The per-edge report walks the index,
// and a process nobody attaches HAS no edge, so it was the one case the report
// could not reach -- while being the case with the largest consequence, since no
// peer-scoped event of any type reaches the program rather than it being fed
// less than it asked for.
// PREVENTS: the silence measured on 2026-08-15, where two route-server fixtures
// ran bgp-rs with no `attach process rs` on any peer. The plugin took delivery
// of no peer-up event, so it never made a peer a live forward target, its
// FastPathSkipped fallback and its peer-up replay could not run, and nothing in
// the daemon said so (test/plugin/bgp-rs-reactor-fastpath-fallback.ci).
func TestReconcileNamesAProcessNoPeerAttaches(t *testing.T) {
	ns, update, state := graphIDs(t)

	s := &Server{subscriptions: newSubscriptionManager()}
	s.UpdateDeliveryGraph(graphNS, twoPeerInput())

	// Attached and declaring exactly what its peer grants, so the two halves
	// agree and nothing is said about it.
	glass := process.NewProcess(plugin.PluginConfig{Name: "looking-glass"})
	s.subscriptions.Add(glass, &Subscription{Namespace: ns, EventType: update, Direction: events.DirReceived})
	s.subscriptions.Add(glass, &Subscription{Namespace: ns, EventType: state, Direction: events.DirBoth})

	// Running and declaring, attached by no peer in the index.
	stranger := process.NewProcess(plugin.PluginConfig{Name: "route-server"})
	s.subscriptions.Add(stranger, &Subscription{Namespace: ns, EventType: update, Direction: events.DirBoth})

	assert.Equal(t, []deliveryDisagreement{{process: "route-server", reason: reasonUnattached}},
		s.deliveryDisagreements(nil),
		"the process no peer attaches is named, and the attached one is not")

	// Scoped to one process, the finding still holds for that process alone.
	assert.Empty(t, s.deliveryDisagreements(glass), "the attached process has nothing to report")
	assert.Len(t, s.deliveryDisagreements(stranger), 1)

	// Attaching it anywhere ends the finding: one peer serving it is enough for
	// the process to be reachable, and the per-edge rows own everything after.
	s.UpdateDeliveryGraph(graphNS, append(twoPeerInput(), DeliveryPeer{
		Addr: "192.0.2.3", Name: "third", Bindings: []plugin.PeerProcessBinding{{
			PluginName: "route-server",
			Receive:    map[string]events.Direction{graphUpdate: events.DirBoth},
		}},
	}))
	assert.Empty(t, s.deliveryDisagreements(stranger))

	// A process that declared NOTHING is not reported: its half is not known
	// yet, and the existing report already refuses to guess about it.
	silent := process.NewProcess(plugin.PluginConfig{Name: "not-yet-declared"})
	s.subscriptions.subscriptions[silent] = nil
	assert.Empty(t, s.deliveryDisagreements(silent))
}
