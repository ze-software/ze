package reactor

import (
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// setSessionFacts gives the test session what the read path and the OPEN
// exchange would: whether the peer announced a route, and whether it
// advertised Route Refresh.
func setSessionFacts(peer *Peer, receivedRoute, routeRefresh bool) {
	peer.session.receivedRoute.Store(receivedRoute)
	peer.session.mu.Lock()
	peer.session.negotiated = &capability.Negotiated{RouteRefresh: routeRefresh}
	peer.session.mu.Unlock()
	peer.negotiated.Load().RouteRefresh = routeRefresh
}

// withDerivedRIBFeed returns a copy of settings carrying the derived bgp-rib
// feed the config builder adds for RFC 7311 AIGP selection, built by the real
// producer (EnsureProcessBinding).
func withDerivedRIBFeed(t *testing.T, settings *PeerSettings) *PeerSettings {
	t.Helper()
	next := *settings
	next.ProcessBindings = append([]ProcessBinding(nil), settings.ProcessBindings...)
	require.NoError(t, EnsureProcessBinding(&next, "bgp-rib", "update-received state", ""))
	return &next
}

// TestDerivedFeedSwapKeepsSessionAndCatchesUp proves the reload path for a
// derived feed-only binding: the session stays up, the binding reaches the
// running peer and its delivery edges, and the peer is asked to re-send what it
// already sent so the new consumer learns it.
//
// VALIDATES: for a session that announced routes, peerSettingsSwapPlan judges
// the change swappable, and
// applyHotSwappableSettings writes one ROUTE-REFRESH per negotiated family on
// the SAME session (RFC 2918 Section 3).
// PREVENTS: adding the first iBGP peer restarting every established session
// (reload-dynamic-peer-survives.ci), and a new RIB consumer that holds none of
// the routes the peer sent before the reload.
func TestDerivedFeedSwapKeepsSessionAndCatchesUp(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast, family.IPv6Unicast)
	setSessionFacts(peer, true, true)
	session := peer.session

	next := withDerivedRIBFeed(t, peer.settings)
	copier, reason := peerSettingsSwapPlan(peer.settings, next, session)
	require.Empty(t, reason, "a derived feed-only binding must not restart the session")
	require.NotNil(t, copier)

	peer.applyHotSwappableSettings(next, copier)

	assert.Same(t, session, peer.session, "the running session must survive the swap")
	assert.Equal(t, PeerStateEstablished, peer.State())
	require.Len(t, peer.settings.ProcessBindings, 1)
	assert.Equal(t, "bgp-rib", peer.settings.ProcessBindings[0].PluginName)
	delivery := deliveryPeersFromSettings([]*PeerSettings{peer.settings})
	require.Len(t, delivery[0].Bindings, 1, "the delivery graph input must carry the new feed")

	frames := bgpFrames(t, conn.written())
	require.Len(t, frames, 2, "one ROUTE-REFRESH per negotiated family")
	for i, afi := range []byte{1, 2} {
		frame := frames[i]
		require.Len(t, frame, 23, "a ROUTE-REFRESH is a header and four octets")
		assert.Equal(t, byte(5), frame[18], "message type ROUTE-REFRESH")
		assert.Equal(t, []byte{0, afi}, frame[19:21], "AFI")
		assert.Equal(t, byte(0), frame[21], "normal route refresh subtype")
		assert.Equal(t, byte(1), frame[22], "SAFI unicast")
	}
}

// TestDerivedFeedWithoutRoutesNeedsNoCatchUp proves the case
// reload-dynamic-peer-survives.ci reaches: a peer that announced no route and
// did not advertise Route Refresh owes a new consumer nothing, so the session
// stays up and the peer is asked for nothing.
//
// VALIDATES: Session.feedCatchUp answers feedCatchUpNothing before it looks
// at the capability.
// PREVENTS: a session that sent no route restarting on a derived feed.
func TestDerivedFeedWithoutRoutesNeedsNoCatchUp(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	setSessionFacts(peer, false, false)
	session := peer.session

	next := withDerivedRIBFeed(t, peer.settings)
	copier, reason := peerSettingsSwapPlan(peer.settings, next, session)
	require.Empty(t, reason, "a session with no route has nothing to catch up")
	peer.applyHotSwappableSettings(next, copier)

	assert.Same(t, session, peer.session, "the running session must survive the swap")
	require.Len(t, peer.settings.ProcessBindings, 1)
	assert.Empty(t, conn.written(), "nothing to re-send, so nothing is asked")
}

// TestDerivedFeedRouteArrivingBeforeApplyRestarts proves the apply re-decides:
// the plan saw no route, the peer announced one before the apply, and it cannot
// re-send it, so the session is ended with RFC 4486 Section 4 subcode 6 "Other
// Configuration Change" rather than leave the new consumer without that route.
func TestDerivedFeedRouteArrivingBeforeApplyRestarts(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	peer.sendingInitialRoutes.Store(0)
	setSessionFacts(peer, false, false)

	next := withDerivedRIBFeed(t, peer.settings)
	copier, reason := peerSettingsSwapPlan(peer.settings, next, peer.session)
	require.Empty(t, reason)
	peer.session.receivedRoute.Store(true)
	peer.applyHotSwappableSettings(next, copier)

	frames := bgpFrames(t, conn.written())
	require.Len(t, frames, 1, "one NOTIFICATION and no ROUTE-REFRESH")
	assert.Equal(t, byte(3), frames[0][18], "message type NOTIFICATION")
	assert.Equal(t, []byte{6, 6}, frames[0][19:21], "Cease, Other Configuration Change")
}

// TestNoteReceivedRouteCountsAnnouncementsOnly proves the flag the catch-up
// reads: an announcement sets it, and a withdrawal or an End-of-RIB, which
// leave no route for a consumer to hold, do not.
func TestNoteReceivedRouteCountsAnnouncementsOnly(t *testing.T) {
	cases := []struct {
		name    string
		payload []byte
		want    bool
	}{
		{"end-of-rib", []byte{0, 0, 0, 0}, false},
		{"withdrawal", []byte{0, 4, 24, 10, 0, 0, 0, 0}, false},
		{"announcement", []byte{0, 0, 0, 0, 24, 10, 0, 0}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			session := NewSession(&PeerSettings{})
			session.noteReceivedRoute(wireu.NewWireUpdate(tc.payload, 0))
			assert.Equal(t, tc.want, session.receivedRoute.Load())
		})
	}
}

// TestDerivedFeedWithoutRouteRefreshRestarts proves the fail-closed half: a
// peer that announced routes and did not advertise Route Refresh cannot re-send
// them to a new consumer, so the change restarts the session rather than leave
// the consumer with half the peer's routes.
func TestDerivedFeedWithoutRouteRefreshRestarts(t *testing.T) {
	peer, _ := newInitialSyncPeer(t, true, family.IPv4Unicast)
	setSessionFacts(peer, true, false)

	_, reason := peerSettingsSwapPlan(peer.settings, withDerivedRIBFeed(t, peer.settings), peer.session)
	assert.Equal(t, "ProcessBindings", reason)
}

// TestDerivedFeedRemovalSendsNoRefresh proves the removal half: the lost
// consumer is named for the targeted "down" (Peer.unbindProcesses), and the peer
// is not asked to re-send anything.
func TestDerivedFeedRemovalSendsNoRefresh(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	setSessionFacts(peer, true, true)
	peer.settings.ProcessBindings = withDerivedRIBFeed(t, peer.settings).ProcessBindings

	next := *peer.settings
	next.ProcessBindings = nil
	gained, lost := receiveBindingChange(peer.settings.ProcessBindings, next.ProcessBindings)
	assert.Empty(t, gained)
	assert.Equal(t, []string{"bgp-rib"}, lost)

	copier, reason := peerSettingsSwapPlan(peer.settings, &next, peer.session)
	require.Empty(t, reason)
	peer.applyHotSwappableSettings(&next, copier)
	assert.Empty(t, peer.settings.ProcessBindings)
	assert.Empty(t, conn.written(), "a removed feed asks the peer for nothing")
}

// operatorBinding returns the binding an operator's
// `attach process <name> { receive [ receive ]; send [ send ]; }` block parses
// to, through the same parsers parseProcessBindingsFromTree uses.
func operatorBinding(t *testing.T, name, receive, send string) ProcessBinding {
	t.Helper()
	b := ProcessBinding{PluginName: name}
	require.NoError(t, parseReceiveFlags(receive, &b))
	require.NoError(t, parseSendFlags(send, &b))
	return b
}

// withBindings returns a copy of settings holding exactly bindings.
func withBindings(settings *PeerSettings, bindings ...ProcessBinding) *PeerSettings {
	next := *settings
	next.ProcessBindings = bindings
	return &next
}

// bindStep is one event the ordered bind delivery sent, with what the plugins
// and the peer could see at that moment.
type bindStep struct {
	event      string   // "up" or "down"
	processes  []string // the processes named
	reason     string
	replay     uint64   // InitialReplay on the peer info
	graphFed   []string // processes the graph fed this peer's received UPDATEs
	wireFrames int      // BGP messages on the wire so far
}

// bindRecorder is a peerBindEvents that records each event with the graph and
// the wire as they stood, so a test can assert the ORDER of up, publish and
// ROUTE-REFRESH rather than only their presence.
type bindRecorder struct {
	t      *testing.T
	srv    *pluginserver.Server
	conn   *recordingConn
	onBind func()

	mu    sync.Mutex
	steps []bindStep
}

func (b *bindRecorder) record(event, reason string, peer *plugin.PeerInfo, processes []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.steps = append(b.steps, bindStep{
		event:      event,
		processes:  append([]string(nil), processes...),
		reason:     reason,
		replay:     peer.InitialReplay,
		graphFed:   recv(b.srv.DeliveryGraph(), bgpevents.EventUpdate, events.DirReceived, peer.AddrStr()),
		wireFrames: len(bgpFrames(b.t, b.conn.written())),
	})
}

func (b *bindRecorder) OnPeerBound(peer *plugin.PeerInfo, _ any, stateProcesses, _ []string) {
	b.record("up", rpc.ReasonAttached, peer, stateProcesses)
	if b.onBind != nil {
		b.onBind()
	}
}

func (b *bindRecorder) OnPeerUnbound(peer *plugin.PeerInfo, _ rpc.SessionState, reason string, processes []string) {
	b.record("down", reason, peer, processes)
}

func (b *bindRecorder) recorded() []bindStep {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]bindStep(nil), b.steps...)
}

// newBindingReactor puts peer in a reactor whose delivery graph is published
// into a real plugin server, with a recorder in place of the event dispatcher.
func newBindingReactor(t *testing.T, peer *Peer, conn *recordingConn) (*pluginserver.Server, *bindRecorder) {
	t.Helper()
	srv := &pluginserver.Server{}
	r := newTestReactor(t)
	r.api = srv
	rec := &bindRecorder{t: t, srv: srv, conn: conn}
	r.bindEvents = rec
	r.peers[peer.settings.PeerKey()] = peer
	peer.mu.Lock()
	peer.reactor = r
	peer.mu.Unlock()
	r.mu.Lock()
	r.publishDeliveryGraphLocked()
	r.mu.Unlock()
	return srv, rec
}

// fedUpdates returns the processes the published graph feeds peer's received
// UPDATEs to.
func fedUpdates(srv *pluginserver.Server, peer *Peer) []string {
	return recv(srv.DeliveryGraph(), bgpevents.EventUpdate, events.DirReceived, peer.settings.Address.String())
}

// TestOperatorReceiveOnlyBindingSwaps proves AC-1: an attach block the operator
// typed that grants no send type and receives no OPEN is swapped like one the
// config builder derived.
//
// VALIDATES: peerSettingsSwapPlan returns a copier and no reason.
// PREVENTS: attaching bgp-rpki or a monitor to a running peer resetting it.
func TestOperatorReceiveOnlyBindingSwaps(t *testing.T) {
	peer, _ := newInitialSyncPeer(t, true, family.IPv4Unicast)
	setSessionFacts(peer, true, true)

	next := withBindings(peer.settings, operatorBinding(t, "bgp-rpki", "update-received state", ""))
	copier, reason := peerSettingsSwapPlan(peer.settings, next, peer.session)
	assert.Empty(t, reason, "a receive-only operator binding must not restart the session")
	assert.NotNil(t, copier)
}

// TestOperatorSendOrOpenBindingStillRestarts proves AC-2: a binding that may
// put routes on the wire, or that must be shown an OPEN, restarts.
func TestOperatorSendOrOpenBindingStillRestarts(t *testing.T) {
	for _, tc := range []struct{ name, receive, send string }{
		{"send update", "update-received", "update"},
		{"receive open", "open-received state", ""},
		{"receive open both directions", "open", ""},
		{"receive wildcard", "*", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer, _ := newInitialSyncPeer(t, true, family.IPv4Unicast)
			setSessionFacts(peer, false, true)
			next := withBindings(peer.settings, operatorBinding(t, "monitor", tc.receive, tc.send))
			_, reason := peerSettingsSwapPlan(peer.settings, next, peer.session)
			assert.Equal(t, "ProcessBindings", reason)
		})
	}
}

// TestReceiveOnlyGainWithoutRouteRefreshRestarts proves AC-10: a peer that sent
// routes and cannot re-send them restarts for a route-bearing receive grant.
func TestReceiveOnlyGainWithoutRouteRefreshRestarts(t *testing.T) {
	peer, _ := newInitialSyncPeer(t, true, family.IPv4Unicast)
	setSessionFacts(peer, true, false)

	next := withBindings(peer.settings, operatorBinding(t, "monitor", "update-received", ""))
	_, reason := peerSettingsSwapPlan(peer.settings, next, peer.session)
	assert.Equal(t, "ProcessBindings", reason)
}

// TestGainedReceiveOnlyBindingGetsUpBeforeRefresh proves AC-4's order: the
// attached process is told the session is up, with reason attached and no
// initial-replay token, before the ROUTE-REFRESH that makes the peer re-send.
// No other process is named.
func TestGainedReceiveOnlyBindingGetsUpBeforeRefresh(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast, family.IPv6Unicast)
	setSessionFacts(peer, true, true)
	peer.settings.ProcessBindings = []ProcessBinding{operatorBinding(t, "bystander", "update-received state", "")}
	session := peer.session
	_, rec := newBindingReactor(t, peer, conn)

	next := withBindings(peer.settings, peer.settings.ProcessBindings[0], operatorBinding(t, "monitor", "update-received state", ""))
	copier, reason := peerSettingsSwapPlan(peer.settings, next, peer.session)
	require.Empty(t, reason)
	peer.applyHotSwappableSettings(next, copier)

	steps := rec.recorded()
	require.Len(t, steps, 1, "one targeted up and nothing else")
	assert.Equal(t, "up", steps[0].event)
	assert.Equal(t, []string{"monitor"}, steps[0].processes, "only the attached process is told")
	assert.Equal(t, rpc.ReasonAttached, steps[0].reason)
	assert.Zero(t, steps[0].replay, "an attached up carries no initial-replay token")
	assert.Zero(t, steps[0].wireFrames, "the up precedes the ROUTE-REFRESH")
	assert.Len(t, bgpFrames(t, conn.written()), 2, "then one ROUTE-REFRESH per negotiated family")
	assert.Same(t, session, peer.session)
}

// TestUpAcknowledgedBeforeGraphEdge proves AC-4/R-4: while the attached process
// is being told the session is up, the graph does not feed it, so a live UPDATE
// in that window cannot reach it; once the apply returns, it does.
func TestUpAcknowledgedBeforeGraphEdge(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	setSessionFacts(peer, false, true)
	srv, rec := newBindingReactor(t, peer, conn)

	next := withBindings(peer.settings, operatorBinding(t, "monitor", "update-received state", ""))
	copier, reason := peerSettingsSwapPlan(peer.settings, next, peer.session)
	require.Empty(t, reason)
	peer.applyHotSwappableSettings(next, copier)

	steps := rec.recorded()
	require.Len(t, steps, 1)
	assert.NotContains(t, steps[0].graphFed, "monitor", "the edge must not be published before the up returned")
	assert.Contains(t, fedUpdates(srv, peer), "monitor", "the edge is published after the up")
}

// TestChangedReceiveGrantIsDownThenUp proves AC-8 at the reactor: a kept
// receive-only binding whose grant changes is told down and then up, its edge
// absent from the graph for both, the ROUTE-REFRESH after the up, and the
// process that kept its binding named in neither.
func TestChangedReceiveGrantIsDownThenUp(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	setSessionFacts(peer, true, true)
	peer.settings.ProcessBindings = []ProcessBinding{
		operatorBinding(t, "bgp-rib", "update-received", ""),
		operatorBinding(t, "bystander", "update-received state", ""),
	}
	srv, rec := newBindingReactor(t, peer, conn)
	require.Contains(t, fedUpdates(srv, peer), "bgp-rib")

	next := withBindings(peer.settings,
		operatorBinding(t, "bgp-rib", "update-received state", ""),
		peer.settings.ProcessBindings[1])
	copier, reason := peerSettingsSwapPlan(peer.settings, next, peer.session)
	require.Empty(t, reason, "a changed receive-only grant swaps")
	peer.applyHotSwappableSettings(next, copier)

	steps := rec.recorded()
	require.Len(t, steps, 2, "a down then an up")
	assert.Equal(t, "down", steps[0].event)
	assert.Equal(t, "up", steps[1].event)
	for _, step := range steps {
		assert.Equal(t, []string{"bgp-rib"}, step.processes, "no other process is told")
		assert.NotContains(t, step.graphFed, "bgp-rib", "the edge is absent between the down and the up")
		assert.Contains(t, step.graphFed, "bystander", "the kept binding keeps its edge")
		assert.Zero(t, step.wireFrames, "the ROUTE-REFRESH follows the up")
	}
	assert.Contains(t, fedUpdates(srv, peer), "bgp-rib")
	assert.Len(t, bgpFrames(t, conn.written()), 1, "one ROUTE-REFRESH for the one family")
}

// TestSentDirectionGrantNeedsNothingAdvertised proves AC-13: a grant covering
// the routes this speaker sent restarts once the session advertised one, since
// nothing re-sends them, and swaps on a session that advertised nothing, where
// the apply keeps the one connection and writes nothing. The wildcard restarts
// either way, for AC-2's reason.
func TestSentDirectionGrantNeedsNothingAdvertised(t *testing.T) {
	for _, receive := range []string{"update-sent state", "update"} {
		t.Run(receive+" advertised", func(t *testing.T) {
			peer, _ := newInitialSyncPeer(t, true, family.IPv4Unicast)
			setSessionFacts(peer, false, true)
			peer.session.advertised.Store(true)
			require.True(t, peer.hasAdvertised())
			_, reason := peerSettingsSwapPlan(peer.settings, withBindings(peer.settings, operatorBinding(t, "monitor", receive, "")), peer.session)
			assert.Equal(t, "ProcessBindings", reason)
		})
		t.Run(receive+" nothing advertised", func(t *testing.T) {
			peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
			setSessionFacts(peer, false, false)
			session := peer.session
			next := withBindings(peer.settings, operatorBinding(t, "monitor", receive, ""))
			copier, reason := peerSettingsSwapPlan(peer.settings, next, peer.session)
			require.Empty(t, reason)
			peer.applyHotSwappableSettings(next, copier)
			assert.Same(t, session, peer.session, "one connection")
			assert.Equal(t, PeerStateEstablished, peer.State())
			assert.Empty(t, conn.written(), "no NOTIFICATION and nothing to ask for")
		})
	}
	peer, _ := newInitialSyncPeer(t, true, family.IPv4Unicast)
	setSessionFacts(peer, false, true)
	_, reason := peerSettingsSwapPlan(peer.settings, withBindings(peer.settings, operatorBinding(t, "monitor", "*", "")), peer.session)
	assert.Equal(t, "ProcessBindings", reason)
}

// TestRouteFreeGrantNeedsNoCatchUp proves AC-14: a grant carrying no route needs
// no catch-up, even on a peer that sent routes and cannot re-send them, and a
// plugin-registered type counts as route-bearing.
func TestRouteFreeGrantNeedsNoCatchUp(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	setSessionFacts(peer, true, false)
	session := peer.session
	_, rec := newBindingReactor(t, peer, conn)

	next := withBindings(peer.settings, operatorBinding(t, "watcher", "state", ""))
	copier, reason := peerSettingsSwapPlan(peer.settings, next, peer.session)
	require.Empty(t, reason, "a route-free grant does not restart")
	peer.applyHotSwappableSettings(next, copier)
	assert.Same(t, session, peer.session)
	assert.Empty(t, conn.written(), "no ROUTE-REFRESH and no NOTIFICATION")
	steps := rec.recorded()
	require.Len(t, steps, 1)
	assert.Equal(t, "up", steps[0].event)

	plugged := ProcessBinding{PluginName: "decorator", Receive: map[string]events.Direction{"update-rpki": events.DirReceived}}
	received, sent := plugged.routeDirections()
	assert.True(t, received, "a plugin-registered type is route-bearing")
	assert.False(t, sent)
}

// TestBindRacesEstablished proves the session check of AC-17: the session the
// up was delivered for is gone by the write, so the attached process is told
// down again and the session restarts, rather than leave it with an up that
// belongs to no session or a second one from the next.
func TestBindRacesEstablished(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	setSessionFacts(peer, true, true)
	_, rec := newBindingReactor(t, peer, conn)
	replacement := NewSession(peer.settings)
	rec.onBind = func() {
		peer.mu.Lock()
		peer.session = replacement
		peer.mu.Unlock()
	}

	next := withBindings(peer.settings, operatorBinding(t, "monitor", "update-received state", ""))
	copier, reason := peerSettingsSwapPlan(peer.settings, next, peer.session)
	require.Empty(t, reason)
	peer.applyHotSwappableSettings(next, copier)

	var events []string
	for _, step := range rec.recorded() {
		events = append(events, step.event+":"+strings.Join(step.processes, ","))
	}
	assert.Equal(t, []string{"up:monitor", "down:monitor"}, events, "exactly one up, matched by a down")
	assert.Empty(t, conn.written(), "no ROUTE-REFRESH on a session the up was not about")
}

// TestIdenticalOperatorBindingReplacingDerived proves AC-18: an attach block
// granting what the config builder's binding granted is the same binding, so it
// changes nothing: no restart, and no process told down or up.
func TestIdenticalOperatorBindingReplacingDerived(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	setSessionFacts(peer, true, false)
	peer.settings.ProcessBindings = withDerivedRIBFeed(t, peer.settings).ProcessBindings
	_, rec := newBindingReactor(t, peer, conn)

	next := withBindings(peer.settings, operatorBinding(t, "bgp-rib", "update-received state", ""))
	assert.True(t, peerSettingsEqual(peer.settings, next), "the two bindings are one")
	copier, reason := peerSettingsSwapPlan(peer.settings, next, peer.session)
	require.Empty(t, reason)
	peer.applyHotSwappableSettings(next, copier)
	assert.Empty(t, rec.recorded())
	assert.Empty(t, conn.written())
}

// graphUpObserver stands where the event dispatcher's observer stands, and
// records at each FSM up and each FSM down the processes the published graph
// feeds the peer's state event: the set onPeerStateChange reads at delivery
// time and sends the event to.
type graphUpObserver struct {
	srv *pluginserver.Server

	mu    sync.Mutex
	fed   [][]string
	downs [][]string
}

func (o *graphUpObserver) OnPeerEstablished(peer *Peer) {
	fed := recv(o.srv.DeliveryGraph(), bgpevents.EventState, events.DirUnspecified, peer.settings.Address.String())
	o.mu.Lock()
	o.fed = append(o.fed, fed)
	o.mu.Unlock()
}

func (o *graphUpObserver) OnPeerClosed(peer *Peer, _ string) {
	fed := recv(o.srv.DeliveryGraph(), bgpevents.EventState, events.DirUnspecified, peer.settings.Address.String())
	o.mu.Lock()
	o.downs = append(o.downs, fed)
	o.mu.Unlock()
}

// downsTo counts the downs name received: the targeted ones the recorder saw
// and the FSM ones whose graph read fed it.
func downsTo(name string, rec *bindRecorder, obs *graphUpObserver) int {
	count := 0
	for _, step := range rec.recorded() {
		if step.event == "down" && slices.Contains(step.processes, name) {
			count++
		}
	}
	obs.mu.Lock()
	defer obs.mu.Unlock()
	for _, fed := range obs.downs {
		if slices.Contains(fed, name) {
			count++
		}
	}
	return count
}

// upsTo counts the ups name received: the targeted ones the recorder saw and
// the FSM ones whose graph read fed it.
func upsTo(name string, rec *bindRecorder, obs *graphUpObserver) int {
	count := 0
	for _, step := range rec.recorded() {
		if step.event == "up" && slices.Contains(step.processes, name) {
			count++
		}
	}
	obs.mu.Lock()
	defer obs.mu.Unlock()
	for _, fed := range obs.fed {
		if slices.Contains(fed, name) {
			count++
		}
	}
	return count
}

// setUpPhase moves the peer's up phase the way announceEstablished does.
func setUpPhase(peer *Peer, phase sessionUpPhase) {
	peer.mu.RLock()
	session := peer.session
	r := peer.reactor
	peer.mu.RUnlock()
	peer.markSessionUp(r, session, phase)
}

// TestAttachBeforeEstablishedUpIsOneUp proves AC-17's Established window: the
// FSM published Established and has not yet told the plugins (setState runs
// before announceEstablished). A reload attaching a process inside that
// window must leave it exactly one up: the FSM's own, which reads the graph the
// reload published, and no targeted one.
//
// Method: an Established session whose up phase is unspecified, the apply,
// then announceEstablished with an observer reading the graph as the
// dispatcher does. Red when the apply judges the session by State() alone.
func TestAttachBeforeEstablishedUpIsOneUp(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	setSessionFacts(peer, false, true)
	srv, rec := newBindingReactor(t, peer, conn)
	obs := &graphUpObserver{srv: srv}
	peer.reactor.addPeerObserver(obs)
	session := peer.session
	setUpPhase(peer, sessionUpUnspecified)

	next := withBindings(peer.settings, operatorBinding(t, "monitor", "update-received state", ""))
	copier, reason := peerSettingsSwapPlan(peer.settings, next, peer.session)
	require.Empty(t, reason)
	peer.applyHotSwappableSettings(next, copier)
	peer.announceEstablished(peer.reactor, session, nil)

	assert.Equal(t, 1, upsTo("monitor", rec, obs), "the attached process is told up exactly once")
	assert.Same(t, session, peer.session, "the session is kept")
}

// TestAttachWhileUpDeliveringRestarts proves the other half of the window: the
// FSM up is being delivered, so nobody can say whether it read the graph
// before or after the publish. The session restarts, and the attached process
// was told nothing targeted.
func TestAttachWhileUpDeliveringRestarts(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	setSessionFacts(peer, false, true)
	_, rec := newBindingReactor(t, peer, conn)
	setUpPhase(peer, sessionUpDelivering)

	next := withBindings(peer.settings, operatorBinding(t, "monitor", "update-received state", ""))
	copier, reason := peerSettingsSwapPlan(peer.settings, next, peer.session)
	require.Empty(t, reason)
	peer.applyHotSwappableSettings(next, copier)

	assert.Empty(t, rec.recorded(), "no targeted up while the FSM up is in flight")
	assertTeardownQueued(t, peer)
}

// TestAttachWhileUpPhaseMovesRestarts proves the step-3 comparison: the FSM up
// starts between step 1, which saw no up, and the publish. Its graph read is
// then on either side of the publish, so the session restarts.
//
// Method: the copier's first call runs inside step 1, after the phase read,
// and moves the phase there.
func TestAttachWhileUpPhaseMovesRestarts(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	setSessionFacts(peer, false, true)
	_, rec := newBindingReactor(t, peer, conn)
	setUpPhase(peer, sessionUpUnspecified)

	next := withBindings(peer.settings, operatorBinding(t, "monitor", "update-received state", ""))
	copier, reason := peerSettingsSwapPlan(peer.settings, next, peer.session)
	require.Empty(t, reason)
	calls := 0
	moving := func(dst, src *PeerSettings) {
		calls++
		if calls == 1 {
			setUpPhase(peer, sessionUpDelivering)
		}
		copier(dst, src)
	}
	peer.applyHotSwappableSettings(next, moving)

	assert.Empty(t, rec.recorded(), "no targeted up: step 1 saw no up announced")
	assertTeardownQueued(t, peer)
}

// TestDetachedProcessToldDownWhenSessionClosesAfterPublish proves the step-4
// down for a detached process: the session was up at step 1 and closes after
// the publish took the process's edge away, so the FSM down will not reach it.
// The apply tells it down anyway.
//
// Method: the copier's second call runs inside step 3, after the session
// check and before the publish, and moves the peer out of Established there.
func TestDetachedProcessToldDownWhenSessionClosesAfterPublish(t *testing.T) {
	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	setSessionFacts(peer, true, true)
	peer.settings.ProcessBindings = []ProcessBinding{operatorBinding(t, "monitor", "update-received state", "")}
	_, rec := newBindingReactor(t, peer, conn)

	next := withBindings(peer.settings)
	copier, reason := peerSettingsSwapPlan(peer.settings, next, peer.session)
	require.Empty(t, reason, "removing a receive-only binding swaps")
	calls := 0
	closing := func(dst, src *PeerSettings) {
		calls++
		if calls == 2 {
			peer.state.Store(int32(PeerStateConnecting))
		}
		copier(dst, src)
	}
	peer.applyHotSwappableSettings(next, closing)

	var events []string
	for _, step := range rec.recorded() {
		events = append(events, step.event+":"+strings.Join(step.processes, ","))
	}
	assert.Equal(t, []string{"down:monitor"}, events, "the detached process is told down")
}

// TestAttachWhileSessionClosingIsToldDown proves review finding I5: the
// session closes while a reload attaches a process. The FSM down reads the
// graph before the publish carries the new edge, so it misses the process the
// reload has just told up. The apply MUST tell that process down itself, and
// MUST NOT restart: the session is going away on its own, and a teardown
// queued now is drained by the next session, which then reads a Cease.
//
// Method: the recorder's hook runs the FSM's close announcement right after
// the targeted up (step 2), so its graph read lands before the step-3 publish
// with no sleep. An observer reads the graph as the dispatcher's down does.
// The cases differ in how far the close got before the publish: only marked,
// cleared as runOnce's defer clears it, or replaced by the next session.
// Red when nothing marks the session closing before that read, or when the
// apply restarts a session that is no longer the one step 1 judged.
//
// VALIDATES: announceClosed and applyHotSwappableSettings step 4.
// PREVENTS: a process holding an up for a session no down reached it for,
// and a Cease sent to the session that follows.
func TestAttachWhileSessionClosingIsToldDown(t *testing.T) {
	for _, tc := range []struct {
		name string
		next func() *Session // the session the defer leaves, nil for none
		gone bool            // runOnce's defer ran before the publish
	}{
		{name: "marked"},
		{name: "cleared", gone: true},
		{name: "replaced", gone: true, next: func() *Session { return &Session{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
			setSessionFacts(peer, true, true)
			srv, rec := newBindingReactor(t, peer, conn)
			obs := &graphUpObserver{srv: srv}
			peer.reactor.addPeerObserver(obs)
			session := peer.session
			rec.onBind = func() {
				peer.announceClosed(peer.reactor, session, "connection lost")
				if !tc.gone {
					return
				}
				// What runOnce's defer does once the session's run returns.
				peer.mu.Lock()
				peer.session = nil
				if tc.next != nil {
					peer.session = tc.next()
				}
				peer.upSession = nil
				peer.upPhase = sessionUpUnspecified
				peer.mu.Unlock()
			}

			next := withBindings(peer.settings, operatorBinding(t, "monitor", "update-received state", ""))
			copier, reason := peerSettingsSwapPlan(peer.settings, next, peer.session)
			require.Empty(t, reason)
			peer.applyHotSwappableSettings(next, copier)

			require.Equal(t, 1, upsTo("monitor", rec, obs), "the attached process was told up once")
			assert.Equal(t, 1, downsTo("monitor", rec, obs), "and down once for the session that closed")
			peer.mu.RLock()
			defer peer.mu.RUnlock()
			for _, op := range peer.opQueue {
				assert.NotEqual(t, PeerOpTeardown, op.Type, "neither the closing session nor the next one is restarted")
			}
			assert.Empty(t, bgpFrames(t, conn.written()), "a closing session is sent no ROUTE-REFRESH")
		})
	}
}

// assertTeardownQueued checks the apply restarted the session: the peer is
// still sending its initial routes, so the teardown waits in its op queue.
func assertTeardownQueued(t *testing.T, peer *Peer) {
	t.Helper()
	peer.mu.RLock()
	defer peer.mu.RUnlock()
	for _, op := range peer.opQueue {
		if op.Type == PeerOpTeardown {
			return
		}
	}
	t.Fatal("the apply queued no session restart")
}
