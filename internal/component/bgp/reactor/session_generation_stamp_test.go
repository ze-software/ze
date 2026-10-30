// Design: docs/architecture/api/json-format.md -- State Events, session-generation.
// Related: peer_session_end.go -- leaveSession, endNegotiatedSession
// Related: peer_derived_bindings.go -- unbindProcesses
// Related: reactor_api.go -- apiStateObserver.OnPeerClosed

package reactor

import (
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// generationObserver records the generation each session close carries.
type generationObserver struct {
	mu     sync.Mutex
	closed []uint64
}

func (o *generationObserver) OnPeerEstablished(*Peer) {}

func (o *generationObserver) OnPeerClosed(_ *Peer, closed sessionClose) {
	o.mu.Lock()
	o.closed = append(o.closed, closed.generation)
	o.mu.Unlock()
}

func (o *generationObserver) generations() []uint64 {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]uint64(nil), o.closed...)
}

// unbindGenerations records the generation each targeted down carries.
type unbindGenerations struct {
	downs []uint64
}

func (u *unbindGenerations) OnPeerBound(*plugin.PeerInfo, any, []string, []string) {}

func (u *unbindGenerations) OnPeerUnbound(peer *plugin.PeerInfo, _ rpc.SessionState, _ string, _ []string) {
	u.downs = append(u.downs, peer.SessionGeneration)
}

// PREVENTS: the down of an Established session that ends naming no session,
// or naming the next one, so a forwarding plugin's departure owes the routes
// of a session that is still running or refuses to name any. Method: a live
// session ends with its UPDATEs held, so its down waits behind them; while it
// waits the peer's generation advances, as the next session reaching
// Established advances it. The down must name the ended session: leaveSession
// reads the generation in the leave arm, before the next session can start.
func TestSessionLeaveStampsEndedGeneration(t *testing.T) {
	c := newSessionEndCase(t, acceptingLiveSettings(t, "10.0.0.1", 65000, 65002))
	observer := &generationObserver{}
	c.r.addPeerObserver(observer)
	c.install(t, sessionEndHooks{})
	ended := c.p.forwardGeneration.Load()
	require.NotZero(t, ended, "an Established session has a generation")

	c.sendHeld(t, 1, false)
	_ = c.v.remote.Close() // The remote ends the session with no NOTIFICATION.
	c.leftWithoutDown(t)
	c.p.advanceForwardGeneration() // The next session reaches Established.
	c.finish(t, 1)

	require.Equal(t, []uint64{ended}, observer.generations(),
		"the down names the session that ended, read before the next one began")
}

// PREVENTS: the down of a session that negotiated but never reached
// Established reaching plugins without the generation it ended on. Method: the
// peer already ran a session, then a new connection negotiates and its remote
// closes before Established; the process's state down event must carry
// session-generation with the peer's generation, which endNegotiatedSession
// stamps and apiStateObserver.OnPeerClosed copies into the event.
func TestNegotiatedSessionEndStampsGeneration(t *testing.T) {
	p, log, _ := newNegotiatedLifetimePeer(t, lowLiveSettings("192.0.2.1", 65000, 65002))
	p.advanceForwardGeneration() // An earlier session reached Established.
	ended := p.forwardGeneration.Load()

	done := make(chan error, 1)
	go func() { done <- p.runOnce() }()
	lowEventually(t, func() bool { return p.SessionState() == fsm.StateActive }, "passive session")
	server, remote := net.Pipe()
	v := &lowLivePeer{peer: p, remote: remote, stopped: make(chan struct{})}
	v.startCapture(t)
	t.Cleanup(func() {
		_ = remote.Close()
		_ = server.Close()
		<-v.stopped
	})
	require.NoError(t, p.acceptConnection(server))
	open := &message.Open{Version: 4, MyAS: 65002, HoldTime: 90, BGPIdentifier: 0x0a000001,
		OptionalParams: []byte{2, 6, 65, 4, 0, 0, 0xfd, 0xea, 2, 6, 1, 4, 0, 1, 0, 1}}
	v.send(t, message.PackTo(open, nil))
	lowEventually(t, func() bool {
		v.mu.Lock()
		defer v.mu.Unlock()
		for _, frame := range v.frames {
			if frame[18] == byte(msgtype.TypeKEEPALIVE) {
				return true
			}
		}
		return false
	}, "OpenConfirm KEEPALIVE")
	require.NoError(t, remote.Close())
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("session teardown did not return")
	}

	want := `"session-generation":"` + strconv.FormatUint(ended, 10) + `"`
	lowEventually(t, func() bool { return negotiatedDownCarries(t, log, want) },
		"the state down event names the generation the session ended on")
}

// negotiatedDownCarries reports whether a recorded state down event holds want.
func negotiatedDownCarries(t *testing.T, log *stateLog, want string) bool {
	t.Helper()
	for _, payload := range log.recorded() {
		var batch struct {
			Events []string `json:"events"`
		}
		require.NoError(t, json.Unmarshal([]byte(payload), &batch))
		for _, line := range batch.Events {
			if !strings.Contains(line, `"state":"down"`) {
				continue
			}
			if strings.Contains(line, want) {
				return true
			}
		}
	}
	return false
}

// PREVENTS: a targeted down (a reload unbinding processes from a session that
// stays up) naming no session, which the engine refuses as a departure, so the
// unbound plugin's routes are never owed. Method: the peer runs a session; the
// unbind's down must carry that session's generation.
func TestUnbindStampsRunningGeneration(t *testing.T) {
	peer, _ := newInitialSyncPeer(t, true, family.IPv4Unicast)
	r := newTestReactor(t)
	recorder := &unbindGenerations{}
	r.bindEvents = recorder
	peer.advanceForwardGeneration() // The running session reached Established.
	running := peer.forwardGeneration.Load()

	peer.unbindProcesses(r, []string{"bgp-rs"})

	require.Equal(t, []uint64{running}, recorder.downs,
		"the targeted down names the session that stays up")
}
