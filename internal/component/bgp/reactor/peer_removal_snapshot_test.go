// Design: docs/architecture/api/architecture.md -- terminal peer delivery ownership.
package reactor

import (
	"bufio"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	bgpserver "github.com/ze-software/ze/internal/component/bgp/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRemovalDownRetainsRecipientsAcrossConcurrentPublication blocks the real
// Cease write, publishes a replacement peer's graph while it is blocked, then
// observes the removed DOWN through the real dispatcher and process transport.
// The old recipient gets exactly one terminal event; the replacement's recipient
// gets none, and publishing the replacement must not wait for the Cease.
func TestRemovalDownRetainsRecipientsAcrossConcurrentPublication(t *testing.T) {
	session, client := shutdownTestSession(t, fsm.StateEstablished)
	r := New(&Config{ListenAddr: "127.0.0.1:0"})
	srv := flowForwardPluginServer(t, r)
	r.api = srv
	r.removalEvents = bgpserver.NewEventDispatcher(srv)
	oldLog, nextLog := &stateLog{}, &stateLog{}
	go answerStateEvents(t.Context(), newStateSubscriber(t, srv, "old-owner"), oldLog, nil)
	go answerStateEvents(t.Context(), newStateSubscriber(t, srv, "next-owner"), nextLog, nil)

	settings := session.settings
	settings.Name = "removed-peer"
	settings.ProcessBindings = []ProcessBinding{{PluginName: "old-owner",
		Receive: map[string]events.Direction{bgpevents.EventState: events.DirBoth}}}
	peer := NewPeer(settings)
	peer.SetReactor(r)
	peer.session = session
	r.peers[settings.PeerKey()] = peer
	before := peer.forwardGeneration.Load()
	r.mu.Lock()
	r.publishDeliveryGraphLocked()
	r.mu.Unlock()

	entered, release := make(chan struct{}), make(chan struct{})
	barrier := &admissionNotificationConn{Conn: session.conn, entered: entered, release: release}
	session.conn = barrier
	session.bufWriter = bufio.NewWriterSize(barrier, message.MaxMsgLen)
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	done := make(chan error, 1)
	go func() { done <- r.RemovePeer(settings.Address) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("removal did not reach the Cease write")
	}

	next := NewPeerSettings(settings.Address, settings.LocalAS, 65003, settings.RouterID)
	next.Name = "replacement-peer"
	next.Connection = ConnectionPassive
	next.ProcessBindings = []ProcessBinding{{PluginName: "next-owner",
		Receive: map[string]events.Direction{bgpevents.EventState: events.DirBoth}}}
	published := make(chan error, 1)
	go func() { published <- r.AddPeer(next) }()
	select {
	case err := <-published:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("graph publication waited for the blocked Cease under the reactor lock")
	}
	require.Equal(t, []string{"next-owner"}, recv(srv.DeliveryGraph(), bgpevents.EventState,
		events.DirUnspecified, settings.Address.String()))
	close(release)
	select {
	case frame := <-readOne(client):
		require.GreaterOrEqual(t, len(frame), message.HeaderLen+2)
		require.Equal(t, byte(message.NotifyCease), frame[message.HeaderLen])
	case <-time.After(5 * time.Second):
		t.Fatal("Cease did not reach the removed peer")
	}
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("terminal delivery did not finish")
	}
	got := oldLog.recorded()
	require.Len(t, got, 1, "the old owner must receive exactly one removed DOWN")
	require.Contains(t, got[0], `\"reason\":\"`+rpc.ReasonPeerRemoved+`\"`)
	require.Contains(t, got[0], `\"state\":\"down\"`)
	require.Contains(t, got[0], "removed-peer")
	require.Empty(t, nextLog.recorded(), "a replacement's grant must not receive the removed lifetime")
	require.Greater(t, peer.forwardGeneration.Load(), before)
}
