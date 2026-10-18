package reactor

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgpserver "github.com/ze-software/ze/internal/component/bgp/server"
	"github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// stateLog records every state event one test process was delivered, in
// order, as the JSON the plugin server sent it.
type stateLog struct {
	mu       sync.Mutex
	payloads []string
}

func (l *stateLog) add(payload string) {
	l.mu.Lock()
	l.payloads = append(l.payloads, payload)
	l.mu.Unlock()
}

func (l *stateLog) recorded() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.payloads...)
}

// answerStateEvents is the plugin side of a test process: it records each
// delivery, runs onFirst once before acknowledging the first one, and
// acknowledges every delivery. It returns when the connection closes.
func answerStateEvents(ctx context.Context, conn *plugipc.PluginConn, log *stateLog, onFirst func()) {
	for {
		req, err := conn.ReadRequest(ctx)
		if err != nil {
			return
		}
		log.add(string(req.Params))
		if onFirst != nil {
			onFirst()
			onFirst = nil
		}
		if err := conn.SendResult(ctx, req.ID, nil); err != nil {
			return
		}
	}
}

// newStateSubscriber connects a process named name to srv over a pipe,
// subscribed to peer state events, the subscription the dispatcher's up and
// down select on.
func newStateSubscriber(t *testing.T, srv *pluginserver.Server, name string) *plugipc.PluginConn {
	t.Helper()
	engineSide, pluginSide := net.Pipe()
	proc := process.NewProcess(plugin.PluginConfig{Name: name})
	proc.SetConn(plugipc.NewPluginConn(engineSide, engineSide))
	proc.StartDelivery(t.Context())
	t.Cleanup(func() {
		proc.Stop()
		if err := pluginSide.Close(); err != nil {
			t.Logf("close plugin side: %v", err)
		}
	})
	srv.Subscriptions().Add(proc, &pluginserver.Subscription{
		Namespace: events.LookupNamespaceID(bgpevents.Namespace),
		EventType: events.LookupEventTypeID(bgpevents.EventState),
		Direction: events.DirUnspecified,
	})
	return plugipc.NewPluginConn(pluginSide, pluginSide)
}

// TestAttachWhileSessionClosingDispatcherDeliversDown proves review finding I5
// end to end, through the production EventDispatcher and plugin server rather
// than a recorder: a process a reload attaches while the session closes is
// told down for that session after its up.
//
// Method: a real plugin server with a real process manager, the reactor wired
// to it as Reactor.StartWithContext wires it (the dispatcher as the bind sink
// and behind apiStateObserver), and a process subscribed to state events. The
// process runs the FSM's close announcement when it receives its targeted up,
// before acknowledging it, so the dispatcher's down reads the graph before the
// reload publishes the process's edge. No sleep orders the two.
//
// VALIDATES: applyHotSwappableSettings step 4 tells an attached process down
// when announceClosed marked the session closing before the publish.
// PREVENTS: a monitor holding an `attached` up for a session that is gone.
func TestAttachWhileSessionClosingDispatcherDeliversDown(t *testing.T) {
	peer, _ := newInitialSyncPeer(t, true, family.IPv4Unicast)
	setSessionFacts(peer, true, true)
	r := New(&Config{ListenAddr: "127.0.0.1:0"})
	srv := flowForwardPluginServer(t, r)
	r.api = srv
	dispatcher := bgpserver.NewEventDispatcher(srv)
	r.eventDispatcher = dispatcher
	r.bindEvents = dispatcher
	r.addPeerObserver(&apiStateObserver{dispatcher: dispatcher, reactor: r})
	r.peers[peer.settings.PeerKey()] = peer
	peer.mu.Lock()
	peer.reactor = r
	session := peer.session
	peer.mu.Unlock()
	r.mu.Lock()
	r.publishDeliveryGraphLocked()
	r.mu.Unlock()

	conn := newStateSubscriber(t, srv, "monitor")
	log := &stateLog{}
	go answerStateEvents(t.Context(), conn, log, func() {
		peer.announceClosed(r, session, closeWithReason(rpc.ReasonTCPFailure))
	})

	next := withBindings(peer.settings, operatorBinding(t, "monitor", "update-received state", ""))
	copier, reason := peerSettingsSwapPlan(peer.settings, next, peer.session)
	require.Empty(t, reason)
	peer.applyHotSwappableSettings(next, copier)

	got := log.recorded()
	require.Len(t, got, 2, "the attached process is told up, then down: %v", got)
	assert.Contains(t, got[0], `\"state\":\"up\"`, "the first event is an up")
	assert.Contains(t, got[0], `\"reason\":\"`+rpc.ReasonAttached+`\"`, "the reload's targeted up")
	assert.Contains(t, got[1], `\"state\":\"down\"`, "the second event takes the session back")
}
