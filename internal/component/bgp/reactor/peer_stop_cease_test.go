package reactor

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/configop"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// RFC 4486 Section 4: "If a BGP speaker decides to de-configure a peer, then
// the speaker SHOULD send a NOTIFICATION message with the Error Code Cease and
// the Error Subcode "Peer De-configured"." The same section gives "Other
// Configuration Change" to a reset caused by any other configuration change.
//
// TestStopWithCeaseTellsThePeerWhy drives the stop a config reload uses for a
// removed peer and for a restarted one. The peer MUST read a Cease with the
// subcode the caller chose before the socket closes.
//
// PREVENTS: a reload that closes TCP with no NOTIFICATION. Peer.Stop only
// cancels a context, and the session's cancel goroutine then closes the
// connection with nothing on the wire.
func TestStopWithCeaseTellsThePeerWhy(t *testing.T) {
	for _, subcode := range []uint8{message.NotifyCeasePeerDeconfigured, message.NotifyCeaseOtherConfigChange} {
		t.Run(message.CeaseSubcodeString(subcode), func(t *testing.T) {
			session, client := shutdownTestSession(t, fsm.StateEstablished)
			peer := NewPeer(session.settings)
			peer.mu.Lock()
			peer.session = session
			peer.mu.Unlock()

			got := readOne(client)
			peer.stopWithCease(subcode)

			// Marker, length 21 (header plus code and subcode, no data:
			// RFC 8203 data is only for subcodes 2 and 4), type NOTIFICATION.
			want := append(bytes.Repeat([]byte{0xFF}, 16), 0x00, 0x15, 0x03, byte(message.NotifyCease), subcode)
			select {
			case msg, ok := <-got:
				require.True(t, ok, "socket closed with no NOTIFICATION on it")
				require.Equal(t, want, msg)
			case <-time.After(5 * time.Second):
				t.Fatal("the peer was stopped without being told why")
			}
		})
	}
}

// TestRemovePeerSendsCeaseWithoutHoldingTheReactorLock proves the Cease is
// written after r.mu is released.
//
// VALIDATES: while the NOTIFICATION write of a removed peer is blocked, because
// nothing reads the other end of the pipe, the reactor lock is free; once the
// peer reads, it gets the Cease / Peer De-configured and RemovePeer returns.
// PREVENTS: one peer with a full send buffer holding r.mu up to the write
// deadline, which stalls the reload and every other path that takes the lock.
func TestRemovePeerSendsCeaseWithoutHoldingTheReactorLock(t *testing.T) {
	session, client := shutdownTestSession(t, fsm.StateEstablished)
	r := newRemovalTestReactor()
	peer := NewPeer(session.settings)
	peer.SetReactor(r)
	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()
	key := peerKeyFromAddrPort(session.settings.Address, DefaultBGPPort)
	r.peers[key] = peer

	done := make(chan error, 1)
	go func() { done <- r.RemovePeer(session.settings.Address) }()

	// net.Pipe has no buffer, so the NOTIFICATION write cannot complete before
	// the client reads. The lock must come free while it waits.
	require.Eventually(t, func() bool {
		if !r.mu.TryLock() {
			return false
		}
		_, present := r.peers[key]
		r.mu.Unlock()
		return !present
	}, 3*time.Second, 5*time.Millisecond, "the reactor lock was held while the Cease write was blocked")
	select {
	case err := <-done:
		t.Fatalf("RemovePeer returned (%v) before the peer read its NOTIFICATION", err)
	default:
	}

	got := readOne(client)
	want := append(bytes.Repeat([]byte{0xFF}, 16), 0x00, 0x15, 0x03, byte(message.NotifyCease), message.NotifyCeasePeerDeconfigured)
	select {
	case msg, ok := <-got:
		require.True(t, ok, "socket closed with no NOTIFICATION on it")
		require.Equal(t, want, msg)
	case <-time.After(5 * time.Second):
		t.Fatal("the removed peer was never told why")
	}
	require.NoError(t, <-done)
}

// TestConfigOperationCeaseNamesWhyTheSessionEnds proves the config-apply path
// tells the peer why its session ends. A remove-peer the configuration no
// longer holds sends RFC 4486 Section 4 subcode 3 "Peer De-configured". The
// remove half of a remove and add pair carries the peer's new config
// (bgpPeerOperation), and sends subcode 6 "Other Configuration Change", which
// the modify-peer restart sends too (restartPeerForOperation).
//
// VALIDATES: applyConfigOperation picks the subcode from the operation.
// PREVENTS: a reload that restarts a peer telling it it was de-configured.
func TestConfigOperationCeaseNamesWhyTheSessionEnds(t *testing.T) {
	cases := []struct {
		name    string
		config  json.RawMessage
		subcode uint8
	}{
		{"peer removed", nil, message.NotifyCeasePeerDeconfigured},
		{"peer restarted under a changed config", json.RawMessage(`{"session":{}}`), message.NotifyCeaseOtherConfigChange},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := New(&Config{})
			settings := NewPeerSettings(mustParseAddr("203.0.113.1"), 65000, 65001, 0)
			settings.Name = "edge"
			require.NoError(t, r.AddPeer(settings))
			session, client := shutdownTestSession(t, fsm.StateEstablished)
			peer := r.Peers()[0]
			peer.mu.Lock()
			peer.session = session
			peer.mu.Unlock()

			op := rpc.ConfigOperation{
				ID: "bgp-remove-peer-edge", Root: "bgp", Owner: "bgp", Type: configop.RemovePeer,
				Target: rpc.ResourceRef{Kind: rpc.ResourcePeer, Peer: "edge"},
				Params: rpc.ConfigOperationParams{Peer: "edge", Config: tc.config},
			}
			done := make(chan error, 1)
			go func() {
				_, err := (&reactorAPIAdapter{r: r}).applyConfigOperation(&op, &testJournal{})
				done <- err
			}()

			want := append(bytes.Repeat([]byte{0xFF}, 16), 0x00, 0x15, 0x03, byte(message.NotifyCease), tc.subcode)
			select {
			case msg, ok := <-readOne(client):
				require.True(t, ok, "socket closed with no NOTIFICATION on it")
				require.Equal(t, want, msg)
			case <-time.After(5 * time.Second):
				t.Fatal("the peer was never told why")
			}
			require.NoError(t, <-done)
		})
	}
}

// TestStopWithCeaseMarksThePeerBeforeTheTeardown proves a removed peer cannot
// redial between its Cease and the cancel that follows.
//
// Method: the Cease write blocks, because nothing reads the far end of the
// pipe. While it is blocked, the peer MUST already read as stopping. The
// teardown ends the session with ErrTeardown, and run() answers that error by
// dialing again with no wait (peer_run.go). p.stopping is the only thing that
// ends the loop before Stop cancels p.ctx.
//
// PREVENTS: a config reload restart where the OLD peer redials, reaches
// Established, and is then closed by the cancel with an Administrative
// Shutdown, so the remote reads a session end the re-added peer never sent
// (test/plugin/attach-process-runtime-subscribe.ci).
func TestStopWithCeaseMarksThePeerBeforeTheTeardown(t *testing.T) {
	session, client := shutdownTestSession(t, fsm.StateEstablished)
	peer := NewPeer(session.settings)
	peer.mu.Lock()
	peer.session = session
	peer.mu.Unlock()

	done := make(chan struct{})
	go func() {
		peer.stopWithCease(message.NotifyCeaseOtherConfigChange)
		close(done)
	}()

	// net.Pipe has no buffer, so the NOTIFICATION write is still blocked here.
	require.Eventually(t, peer.stopping.Load, 3*time.Second, time.Millisecond,
		"the peer was not marked stopping while its Cease was on the wire, so "+
			"run() redials once the teardown returns ErrTeardown")
	select {
	case <-done:
		t.Fatal("stopWithCease returned before the peer read its NOTIFICATION, so this test proves nothing")
	default:
	}

	select {
	case _, ok := <-readOne(client):
		require.True(t, ok, "socket closed with no NOTIFICATION on it")
	case <-time.After(5 * time.Second):
		t.Fatal("the peer was never told why")
	}
	<-done
}
