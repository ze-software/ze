// Design: docs/architecture/l2tp/subscriber-session-model.md -- the order of one session's notifications
// Related: reactor.go -- handle and handleTick deliver the owed session downs before the tunnel events

package l2tp

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// TestTunnelCallbackRunsAfterOwedSessionDowns proves that a tunnel-down reader
// runs only after the session downs its tunnel's teardown owes were delivered,
// and that it can end another session through the real teardown path and
// return, with every owed down delivered once and in ticket order.
//
// Method: tunnel 11 carries sessions 7 and 8, tunnel 12 carries session 9.
// Tunnel 11 is closed through each real path that emits tunnel-down: a peer
// StopCCN through handle, and a keepalive timeout through handleTick. The
// tunnel-down reader runs on the reactor's goroutine, as an in-process reader
// does; it records the session downs already published, then ends session 9
// through the captured-session teardown, the path the operator clear and the
// CoA Disconnect-Request take.
// MUTATION: emitting tunnel-down before deliverSessionDowns (the code before
// this test) makes the reader see no session down of tunnel 11.
func TestTunnelCallbackRunsAfterOwedSessionDowns(t *testing.T) {
	peer := netip.MustParseAddrPort("10.0.0.9:1701")
	cases := []struct {
		name string
		end  func(t *testing.T, r *l2tpReactor)
	}{
		{
			name: "peer StopCCN",
			end: func(t *testing.T, r *l2tpReactor) {
				t.Helper()
				r.handle(rxPacket{from: peer, bytes: buildStopCCN(t, 11, 0, 0, 22, 1), release: func() {}})
			},
		},
		{
			name: "keepalive timeout",
			end: func(t *testing.T, r *l2tpReactor) {
				t.Helper()
				r.tunnelsMu.Lock()
				tun := r.tunnelsByLocalID[11]
				tun.lastLiveness = r.params.Clock().Add(-time.Minute)
				tun.lastActivity = r.params.Clock().Add(-time.Minute)
				r.tunnelsMu.Unlock()
				r.handleTick(tickReq{tunnelID: 11})
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bus := newCauseBus()
			r := newReactorForPeerTeardown(t, bus)
			r.params.HelloInterval = time.Second
			r.params.HelloRetries = 3
			insertEstablishedTunnel(t, r, 11, 22, peer,
				&L2TPSession{localSID: 7, remoteSID: 70, state: L2TPSessionEstablished, username: "grace"},
				&L2TPSession{localSID: 8, remoteSID: 80, state: L2TPSessionEstablished, username: "ada"})
			insertEstablishedTunnel(t, r, 12, 23, netip.MustParseAddrPort("10.0.0.10:1701"),
				&L2TPSession{localSID: 9, remoteSID: 90, state: L2TPSessionEstablished, username: "lin"})

			var (
				calls        int
				seenAtTunnel []uint16
				teardownErr  error
			)
			bus.Subscribe(l2tpevents.Namespace, l2tpevents.TunnelDownEvent, func(payload any) {
				down, ok := payload.(*l2tpevents.TunnelDownPayload)
				if !ok {
					return
				}
				if down.TunnelID != 11 {
					return
				}
				calls++
				seenAtTunnel = sessionDownIDs(bus.downEvents())
				subsystem := NewSubsystem(&Parameters{})
				subsystem.reactors = []*l2tpReactor{r}
				captured, err := subsystem.CaptureSession(SessionSelector{SID: 9, TID: 12})
				if err != nil {
					teardownErr = err
					return
				}
				teardownErr = captured.Teardown(t.Context(), l2tpevents.TerminateCauseAdminReset, "")
			})

			tc.end(t, r)

			require.Equal(t, 1, calls, "tunnel 11 closes once, so its reader runs once")
			require.ElementsMatch(t, []uint16{7, 8}, seenAtTunnel,
				"the tunnel-down reader must run after both owed session downs were delivered")
			require.NoError(t, teardownErr, "the reader's teardown must reach session 9 and return")
			delivered := sessionDownIDs(bus.downEvents())
			require.Len(t, delivered, 3, "each owed down is delivered once: %v", delivered)
			require.ElementsMatch(t, []uint16{7, 8}, delivered[:2])
			require.Equal(t, uint16(9), delivered[2], "the reader's own teardown is published after it")
		})
	}
}

// sessionDownIDs answers the local session ID of each session down, in the
// order it was published.
func sessionDownIDs(downs []*l2tpevents.SessionDownPayload) []uint16 {
	ids := make([]uint16, 0, len(downs))
	for _, d := range downs {
		ids = append(ids, d.SessionID)
	}
	return ids
}
