// Design: docs/architecture/api/json-format.md -- TCP connected precedes OPEN processing.
// Related: delay_open_test.go -- real Peer, TCP accept/dial and protocol timer helpers.
// Related: peer_bind_dispatch_test.go -- subscribed process delivery over the real IPC transport.
package reactor

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgpserver "github.com/ze-software/ze/internal/component/bgp/server"
	"github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
)

// TestDelayOpenConnectedEventPrecedesOpen observes the actual IPC delivery
// while TCP is held without an OPEN, then completes that same useful session.
// Both accepted and dialed sockets must report connected exactly once.
func TestDelayOpenConnectedEventPrecedesOpen(t *testing.T) {
	for _, passive := range []bool{false, true} {
		t.Run(map[bool]string{false: "dialed", true: "accepted"}[passive], func(t *testing.T) {
			delayOpenConnectedHandshake(t, passive, 240*time.Second)
		})
	}
}

// TestImmediateConnectedEventPrecedesOpenConfirm keeps the zero-delay path
// useful and checks that moving TCP notification out of the FSM duplicates
// neither the event nor its ordering before the connection's FSM transition.
func TestImmediateConnectedEventPrecedesOpenConfirm(t *testing.T) {
	delayOpenConnectedHandshake(t, false, 0)
}

func delayOpenConnectedHandshake(t *testing.T, passive bool, delay time.Duration) {
	t.Helper()
	var deliveries *stateLog
	_, session, far, _ := startDelayOpenPeer(t, passive, delay, func(peer *Peer) {
		deliveries = delayOpenConnectedSubscriber(t, peer)
	})
	const connected = `\"type\":\"connected\"`
	require.Eventually(t, func() bool {
		return strings.Contains(strings.Join(deliveries.recorded(), "\n"), connected)
	}, 5*time.Second, time.Millisecond, "TCP succeeded but connected was not delivered before peer OPEN")
	if delay != 0 {
		delayOpenNoBytes(t, far)
	}

	delayOpenWrite(t, far, delayOpenRemote(0x02020202))
	if delay != 0 {
		delayOpenReadType(t, far, 1)
	}
	delayOpenReadType(t, far, 4)
	delayOpenWrite(t, far, &message.Keepalive{})
	delayOpenAwait(t, func() bool { return session.State() == fsm.StateEstablished })
	const established = `\"to\":\"ESTABLISHED\"`
	var joined string
	require.Eventually(t, func() bool {
		joined = strings.Join(deliveries.recorded(), "\n")
		return strings.Contains(joined, established)
	}, 5*time.Second, time.Millisecond, "subscribed process never received the useful session's Established event")
	require.Equal(t, 1, strings.Count(joined, connected), "one TCP success must publish connected exactly once: %s", joined)
	transition := `\"to\":\"OPENCONFIRM\"`
	if delay == 0 {
		transition = `\"to\":\"OPENSENT\"`
	}
	require.GreaterOrEqual(t, strings.Index(joined, transition), 0, "the connection's FSM transition was not delivered: %s", joined)
	require.Less(t, strings.Index(joined, connected), strings.Index(joined, transition), "connected must precede its FSM transition: %s", joined)
}

// delayOpenConnectedSubscriber installs one long-lived IPC consumer before the
// Peer starts. Cleanup stops delivery and joins the reader; no event owns a
// goroutine and no process callback replaces the production FSM callback.
func delayOpenConnectedSubscriber(t *testing.T, peer *Peer) *stateLog {
	t.Helper()
	require.NoError(t, EnsureProcessBinding(peer.settings, "monitor", "connected fsm", ""))
	r := New(&Config{ListenAddr: "127.0.0.1:0"})
	r.peers[peer.settings.PeerKey()] = peer
	peer.reactor = r
	srv := flowForwardPluginServer(t, r)
	r.api = srv
	r.eventDispatcher = bgpserver.NewEventDispatcher(srv)

	engineSide, pluginSide := net.Pipe()
	proc := process.NewProcess(plugin.PluginConfig{Name: "monitor", Encoder: plugin.EncodingJSON})
	proc.SetConn(plugipc.NewPluginConn(engineSide, engineSide))
	proc.StartDelivery(t.Context())
	for _, kind := range []string{bgpevents.EventConnected, bgpevents.EventFSM} {
		srv.Subscriptions().Add(proc, &pluginserver.Subscription{
			Namespace: events.LookupNamespaceID(bgpevents.Namespace),
			EventType: events.LookupEventTypeID(kind),
			Direction: events.DirUnspecified,
		})
	}
	r.mu.Lock()
	r.publishDeliveryGraphLocked()
	r.mu.Unlock()
	log := &stateLog{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		answerStateEvents(t.Context(), plugipc.NewPluginConn(pluginSide, pluginSide), log, nil)
	}()
	t.Cleanup(func() {
		proc.Stop()
		if err := pluginSide.Close(); err != nil {
			t.Logf("close connected-event subscriber: %v", err)
		}
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("connected-event IPC consumer did not stop")
		}
	})
	return log
}
