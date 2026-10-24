// Design: docs/architecture/behavior/peer-lifecycle.md -- why a session left Established
// Overview: peer_close.go -- sessionCloseFor, the close each cause derives
// Related: reactor_api.go -- apiStateObserver.OnPeerClosed, which puts the close on the down
// Related: rfc4760_peer_down_link_test.go -- startMPLinkNeighborHolding, the running Peer

package reactor

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// closeCauseCase is one way an Established session ends, and the close the down must
// carry for it.
type closeCauseCase struct {
	name         string
	holdSeconds  uint16
	end          func(t *testing.T, link *mpLinkNeighbor)
	reason       string
	event        fsm.Event
	notification plugin.CloseNotification
}

// closeCauseCases covers the four causes an operator has to tell apart on a down:
// the neighbor's NOTIFICATION, an operator teardown, the hold timer, and the transport.
func closeCauseCases() []closeCauseCase {
	return []closeCauseCase{
		{
			name:        "the neighbor sends a Cease",
			holdSeconds: 90,
			end: func(t *testing.T, link *mpLinkNeighbor) {
				cease := &message.Notification{ErrorCode: message.NotifyCease, ErrorSubcode: message.NotifyCeaseAdminShutdown}
				_, err := link.conn.Write(message.PackTo(cease, nil))
				require.NoError(t, err)
			},
			reason: rpc.ReasonNotification,
			event:  fsm.EventNotifMsg,
			notification: plugin.CloseNotification{
				Direction: plugin.NotifReceived, Code: uint8(message.NotifyCease), Subcode: message.NotifyCeaseAdminShutdown,
			},
		},
		{
			name:        "the operator tears the session down",
			holdSeconds: 90,
			end: func(t *testing.T, link *mpLinkNeighbor) {
				require.NoError(t, link.peer.Teardown(message.NotifyCeaseAdminShutdown, ""))
			},
			reason: rpc.ReasonNotification,
			event:  fsm.EventManualStop,
			notification: plugin.CloseNotification{
				Direction: plugin.NotifSent, Code: uint8(message.NotifyCease), Subcode: message.NotifyCeaseAdminShutdown,
			},
		},
		{
			name:        "the hold timer expires",
			holdSeconds: 3,
			end:         func(*testing.T, *mpLinkNeighbor) {},
			reason:      rpc.ReasonNotification,
			event:       fsm.EventHoldTimerExpires,
			notification: plugin.CloseNotification{
				Direction: plugin.NotifSent, Code: uint8(message.NotifyHoldTimerExpired), Subcode: 0,
			},
		},
		{
			name:         "the transport fails",
			holdSeconds:  90,
			end:          func(t *testing.T, link *mpLinkNeighbor) { require.NoError(t, link.conn.Close()) },
			reason:       rpc.ReasonTCPFailure,
			event:        fsm.EventTCPConnectionFails,
			notification: plugin.CloseNotification{Direction: plugin.NotifNone},
		},
	}
}

// TestCloseCauseReachesTheStateEvent drives a running Peer to Established, ends the
// session by each cause, and delivers the close the run loop derived through the
// production state observer and EventDispatcher to a subscribed process.
//
// VALIDATES: the down a plugin receives names the cause in typed members beside the
// reason: "notification" with its direction, code and subcode for a received Cease, an
// operator teardown and a hold-timer expiry, and no "notification" member for a
// transport failure, which reads "tcp-failure".
// PREVENTS: every down reading as one reason with the cause lost, as when the reactor
// sent "connection lost" for every exit from Established
// (plan/journal/constant-reported-as-measured-state.md, 2026-10-13).
func TestCloseCauseReachesTheStateEvent(t *testing.T) {
	for _, tc := range closeCauseCases() {
		t.Run(tc.name, func(t *testing.T) {
			link := startMPLinkNeighborHolding(t, tc.holdSeconds)
			tc.end(t, link)

			closed := awaitCloseWithin(t, link, time.Duration(tc.holdSeconds)*time.Second+5*time.Second)
			require.Equal(t, tc.reason, closed.reason)
			want, ok := tc.event.Number()
			require.True(t, ok)
			require.Equal(t, want, closed.fsmEvent, "the close carries the FSM event that ended it")
			require.Equal(t, tc.notification, closed.notification)

			down := deliverDown(t, closed)
			require.Equal(t, "down", down["state"])
			require.Equal(t, tc.reason, down["reason"])
			if tc.notification.Direction == plugin.NotifNone {
				require.NotContains(t, down, "notification", "a down no NOTIFICATION closed carries none")
				return
			}
			got, isObject := down["notification"].(map[string]any)
			require.True(t, isObject, "the down carries the NOTIFICATION: %v", down)
			require.Equal(t, tc.notification.Direction.String(), got["direction"])
			require.Equal(t, float64(tc.notification.Code), got["code"])
			require.Equal(t, float64(tc.notification.Subcode), got["subcode"])
		})
	}
}

// awaitCloseWithin is awaitClose with a deadline long enough for a hold timer.
func awaitCloseWithin(t *testing.T, link *mpLinkNeighbor, wait time.Duration) sessionClose {
	t.Helper()
	select {
	case closed := <-link.events.causes:
		return closed
	case <-time.After(wait):
		t.Fatal("the run loop never reported the session closed")
	}
	return sessionClose{}
}

// deliverDown hands closed to apiStateObserver on a reactor wired to a real plugin
// server, and returns the "bgp" object of the state event the subscribed process read.
func deliverDown(t *testing.T, closed sessionClose) map[string]any {
	t.Helper()
	p, log, _ := newNegotiatedLifetimePeer(t, lowLiveSettings("192.0.2.1", 65000, 65002))
	p.announceDown(p.reactor, closed)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, payload := range log.recorded() {
			if bgp, isState := stateEventBGP(t, payload); isState {
				return bgp
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("the subscribed process never read a state event: %v", log.recorded())
	return nil
}

// stateEventBGP decodes one batch delivery's params and returns the "bgp" object of the
// state event it carries, false when the batch holds none.
func stateEventBGP(t *testing.T, payload string) (map[string]any, bool) {
	t.Helper()
	var batch struct {
		Events []string `json:"events"`
	}
	require.NoError(t, json.Unmarshal([]byte(payload), &batch), "delivery params: %s", payload)
	for _, raw := range batch.Events {
		if !strings.Contains(raw, `"type":"state"`) {
			continue
		}
		var event struct {
			BGP map[string]any `json:"bgp"`
		}
		require.NoError(t, json.Unmarshal([]byte(raw), &event), "state event: %s", raw)
		return event.BGP, true
	}
	return nil, false
}
