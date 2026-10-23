package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
)

// stuckResponder reads each delivery RPC and never answers it, the shape of
// a wedged subscriber: the delivery loop then waits its full timeout.
func stuckResponder(ctx context.Context, conn *plugipc.PluginConn) {
	for {
		if _, err := conn.ReadRequest(ctx); err != nil {
			return
		}
	}
}

// subscribePeerLines registers proc for the three per-peer line events the
// FSM and the signal publish emit.
func subscribePeerLines(srv *pluginserver.Server, proc *process.Process) {
	for _, eventType := range []string{bgpevents.EventConnected, bgpevents.EventFSM, bgpevents.EventSignal} {
		srv.Subscriptions().Add(proc, &pluginserver.Subscription{
			Namespace: events.LookupNamespaceID("bgp"),
			EventType: events.LookupEventTypeID(eventType),
			Direction: events.DirUnspecified,
		})
	}
}

// TestPeerLineEventsDoNotWaitForSubscribers proves that connected, fsm and
// signal delivery returns without waiting for any subscriber's answer. The
// FSM emits connected and fsm before it sends the OPEN, and the hub publishes
// signal before it starts the stop, so a wait there let one wedged `*`
// subscriber hold the OPEN or SIGTERM for the delivery timeout (5 s) per
// event. Method: one subscriber never answers; the three emits together must
// return well inside one delivery timeout. A second subscriber that answers
// must still receive the three events in the order they were emitted.
func TestPeerLineEventsDoNotWaitForSubscribers(t *testing.T) {
	srv, rec := newBarrierTestServer(t)

	stuck, stuckConn := newTestProcWithConn(t, "stuck")
	subscribePeerLines(srv, stuck)
	go stuckResponder(t.Context(), stuckConn)

	live, liveConn := newTestProcWithConn(t, "live")
	subscribePeerLines(srv, live)
	go payloadResponder(t.Context(), liveConn, rec, "live")

	peer := testPeerInfo()
	attachForDelivery(t, srv, peer, "stuck", "live")

	start := time.Now()
	onPeerConnected(srv, peer)
	onPeerFSMChange(srv, peer, fsm.StateConnect, fsm.StateOpenSent)
	onPeerSignal(srv, peer, 15, "SIGTERM")
	elapsed := time.Since(start)

	assert.Less(t, elapsed, time.Second,
		"a subscriber that never answers must not delay the emitter (took %v)", elapsed)

	var joined string
	require.Eventually(t, func() bool {
		joined = strings.Join(rec.recorded(), "\n")
		return strings.Contains(joined, "SIGTERM")
	}, 5*time.Second, 10*time.Millisecond, "the answering subscriber receives every event")

	connectedAt := strings.Index(joined, "connected")
	fsmAt := strings.Index(joined, "fsm")
	signalAt := strings.Index(joined, "SIGTERM")
	require.GreaterOrEqual(t, connectedAt, 0, "connected delivered: %s", joined)
	require.GreaterOrEqual(t, fsmAt, 0, "fsm delivered: %s", joined)
	assert.Less(t, connectedAt, fsmAt, "connected before fsm: %s", joined)
	assert.Less(t, fsmAt, signalAt, "fsm before signal: %s", joined)
	t.Log(joined)
}
