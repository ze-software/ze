// Design: docs/architecture/behavior/peer-lifecycle.md -- incoming ownership during startup.
// Related: peer_run.go -- selected Session publication and its initial start.
// Related: reactor_connection.go -- normal passive accept and buffered admission.
package reactor

import (
	"context"
	"net"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
)

// TestPassiveIncomingAcrossSessionStart keeps one real TCP connection through
// the interval in which runOnce announces its first Active Peer state. That
// callback currently precedes the Session start: accepting its published Idle
// Session loses TCP-confirmed but still sends OPEN, then closes on remote OPEN.
// An implementation may buffer this early socket or make the Session ready
// before publication. Both must complete this same handshake without a retry.
func TestPassiveIncomingAcrossSessionStart(t *testing.T) {
	for _, gateStart := range []bool{true, false} {
		name := "before-session-start"
		if !gateStart {
			name = "ordinary-passive-start"
		}
		t.Run(name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("127.0.0.1"), 65000, 65001, 0x01010101)
			settings.Connection = ConnectionPassive
			if err := EnsureProcessBinding(settings, "startup-monitor", "connected fsm", ""); err != nil {
				t.Fatal(err)
			}
			r := New(&Config{ListenAddr: "127.0.0.1:0"})
			if err := r.AddPeer(settings); err != nil {
				t.Fatal(err)
			}
			peer := r.Peers()[0]
			srv := newBorrowedPluginServer(t, r, "bgp-rib")
			r.SetPluginServer(srv)
			peer.setReconnectDelay(time.Hour, time.Hour)
			entered := make(chan struct{})
			proceed := make(chan struct{})
			release := sync.OnceFunc(func() { close(proceed) })
			gate := sync.OnceFunc(func() {
				close(entered)
				<-proceed
			})
			if gateStart {
				peer.SetCallback(func(_, next PeerState) {
					if next == PeerStateActive {
						gate()
					}
				})
			}
			t.Cleanup(func() {
				// The callback must be free before Stop waits for the Peer.
				release()
				r.Stop()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := r.Wait(ctx); err != nil {
					t.Errorf("passive startup reactor did not stop: %v", err)
				}
			})
			if err := r.Start(); err != nil {
				t.Fatal(err)
			}
			deliveries := passiveStartEventSubscriber(t, r)
			t.Cleanup(func() {
				if t.Failed() {
					t.Logf("startup raw IPC deliveries: %#v", deliveries.recorded())
					for _, kind := range []string{bgpevents.EventConnected, bgpevents.EventFSM} {
						matched := r.api.PeerScopedProcs(events.LookupNamespaceID(bgpevents.Namespace), events.LookupEventTypeID(kind),
							events.DirUnspecified, settings.Address.String(), settings.Name)
						t.Logf("startup subscriber %s: matched=%d namespace=%d event=%d process-manager=%t server-context=%v",
							kind, len(matched), events.LookupNamespaceID(bgpevents.Namespace), events.LookupEventTypeID(kind), r.api.ProcessManager() != nil, r.api.Context().Err())
					}
				}
			})
			startBorrowedPeers(t, r, srv)
			if gateStart {
				incomingCorrectionAwait(t, entered, "initial Active publication")
			} else {
				delayOpenAwait(t, func() bool { return peer.SessionState() == fsm.StateActive })
			}
			incoming := incomingListenerDial(t, r)
			delayOpenWrite(t, incoming, delayOpenRemote(0x02020202))
			var selected *Session
			buffered := false
			delayOpenAwait(t, func() bool {
				peer.mu.RLock()
				selected = peer.session
				buffered = peer.inbound != nil
				peer.mu.RUnlock()
				if buffered {
					return true
				}
				return selected != nil && selected.Conn() != nil
			})
			if !buffered {
				// Conn publication alone precedes TCP-confirmed. Reading OPEN
				// fences the completed Accept, so releasing the callback cannot
				// accidentally move Start ahead of that connection event.
				delayOpenReadType(t, incoming, 1)
			}
			release()
			if buffered {
				delayOpenReadType(t, incoming, 1)
			}
			delayOpenReadType(t, incoming, 4)
			delayOpenWrite(t, incoming, &message.Keepalive{})
			delayOpenAwait(t, func() bool { return peer.SessionState() == fsm.StateEstablished })
			// enterEstablished publishes state before setEstablishedNow credits
			// this connection. Wait for that producer, then keep the exact count.
			delayOpenAwait(t, func() bool { return peer.Stats().ConnectionsEstablished != 0 })
			if selected != nil && peer.currentSession() != selected {
				t.Fatal("startup discarded the Session that owned the admitted TCP connection")
			}
			if stats := peer.Stats(); stats.OpensSent != 1 || stats.OpensReceived != 1 || stats.ConnectionsEstablished != 1 {
				t.Fatalf("one admitted handshake was replayed: OPEN sent=%d received=%d established=%d",
					stats.OpensSent, stats.OpensReceived, stats.ConnectionsEstablished)
			}
			const connected = `\"type\":\"connected\"`
			const openSent = `\"to\":\"OPENSENT\"`
			const openConfirm = `\"to\":\"OPENCONFIRM\"`
			const established = `\"to\":\"ESTABLISHED\"`
			var observed string
			delayOpenAwait(t, func() bool {
				observed = strings.Join(deliveries.recorded(), "\n")
				return strings.Contains(observed, established)
			})
			for _, event := range []string{connected, openSent, openConfirm, established} {
				if count := strings.Count(observed, event); count != 1 {
					t.Errorf("startup delivered %s %d times, want once: %s", event, count, observed)
				}
			}
			if strings.Index(observed, connected) >= strings.Index(observed, openSent) ||
				strings.Index(observed, openSent) >= strings.Index(observed, openConfirm) ||
				strings.Index(observed, openConfirm) >= strings.Index(observed, established) {
				t.Errorf("startup event order is not connected, OpenSent, OpenConfirm, Established: %s", observed)
			}
		})
	}
}

// passiveStartEventSubscriber observes the started Reactor's actual dispatcher
// through one process queue and IPC reader. It does not replace Peer callbacks.
func passiveStartEventSubscriber(t *testing.T, r *Reactor) *stateLog {
	t.Helper()
	if r.api == nil {
		t.Fatal("started reactor has no process server")
	}
	engineSide, pluginSide := net.Pipe()
	proc := process.NewProcess(plugin.PluginConfig{Name: "startup-monitor", Encoder: plugin.EncodingJSON})
	proc.SetConn(plugipc.NewPluginConn(engineSide, engineSide))
	proc.StartDelivery(t.Context())
	for _, kind := range []string{bgpevents.EventConnected, bgpevents.EventFSM} {
		r.api.Subscriptions().Add(proc, &pluginserver.Subscription{
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
			t.Logf("close startup event subscriber: %v", err)
		}
		incomingCorrectionAwait(t, done, "startup event subscriber cleanup")
	})
	return log
}
