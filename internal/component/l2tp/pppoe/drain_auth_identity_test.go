// Design: docs/architecture/l2tp/subscriber-session-model.md -- PPPoE authentication identity precedes relay and local admission

package pppoe

import (
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/internal/core/callsink"
)

type identityAuthDriver struct {
	events  chan ppp.AuthEvent
	calls   int
	accept  bool
	message string
	err     error
}

func (d *identityAuthDriver) AuthEventsOut() <-chan ppp.AuthEvent { return d.events }
func (d *identityAuthDriver) AuthResponse(_, _ uint16, accept bool, message string, _ []byte) error {
	d.calls++
	d.accept, d.message = accept, message
	return d.err
}

// TestAuthDrainRequiresSubscriberIdentity drives the production loop with a
// missing identity that would otherwise enter realm selection. Neither relay,
// provider nor pending authorization sees it; valid opaque IDs remain intact.
func TestAuthDrainRequiresSubscriberIdentity(t *testing.T) {
	for _, tc := range []struct {
		name string
		id   string
		err  error
	}{
		{name: "missing"},
		{name: "missing-refusal-session-ended", err: ppp.ErrSessionNotFound},
		{name: "opaque", id: "opaque-subscriber-A"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			callsink.Unregister()
			t.Cleanup(callsink.Unregister)
			callsink.Register(&fakeRealmSink{canceled: make(chan struct{}, 1)})
			relayCalls := 0
			relay := newRealmRelay(logger, newFakeRelayDriver(),
				func(int, uint16) (realmSubscriber, bool) {
					relayCalls++
					return realmSubscriber{}, false
				}, func(int, uint16) { t.Error("unexpected relay teardown") })
			t.Cleanup(relay.stopAll)
			driver := &identityAuthDriver{events: make(chan ppp.AuthEvent, 1), err: tc.err}
			request := ppp.EventAuthRequest{SubscriberID: tc.id,
				TunnelID: 7, SessionID: 42, Method: ppp.AuthMethodNone, Username: "alice"}
			if tc.id == "" {
				request.Username = "alice@example.net"
				request.ProxyLCP = &ppp.ProxyLCPCapture{InitialReceived: []byte{1, 4, 5, 0xdc}}
			}
			driver.events <- request
			close(driver.events)
			bus := newRecordBus()
			var pending sync.Map
			var eventID string
			events := 0
			unsubscribe := subevents.SessionAuthResult.Subscribe(bus, func(result *subevents.SessionAuthResultPayload) {
				events++
				eventID = result.SessionID
			})
			defer unsubscribe()
			resolved, called := 0, 0
			var receivedID string
			done := startPPPoEAuthDrain(logger, driver, func() subscriber.AuthHandler {
				resolved++
				return func(request ppp.EventAuthRequest, _ subscriber.AuthRespondFunc) subscriber.AuthResult {
					called++
					receivedID = request.SubscriberID
					return subscriber.AuthResult{Accept: true}
				}
			}, bus, &pending, relay)
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("auth drain did not join after input closure")
			}
			if driver.calls != 1 {
				t.Fatalf("authentication reply calls=%d, want 1", driver.calls)
			}
			_, stored := pending.Load(pendingAuthKey{ifindex: 7, sessionID: 42})
			if tc.id == "" {
				if relayCalls != 0 {
					t.Fatalf("missing identity entered relay selection %d times", relayCalls)
				}
				if resolved != 0 {
					t.Fatalf("missing identity resolved provider %d times", resolved)
				}
				if called != 0 {
					t.Fatalf("missing identity called provider %d times", called)
				}
				if stored {
					t.Fatal("missing identity entered pending authorization")
				}
				if events != 0 {
					t.Fatalf("missing identity emitted %d subscriber events", events)
				}
				if driver.accept {
					t.Fatal("missing identity was authenticated")
				}
				if driver.message == "" {
					t.Fatal("missing identity refusal has no reason")
				}
				return
			}
			if !driver.accept {
				t.Fatalf("valid opaque identity refused: %s", driver.message)
			}
			if called != 1 {
				t.Fatalf("valid identity provider calls=%d, want 1", called)
			}
			if receivedID != tc.id {
				t.Fatalf("provider identity=%q, want %q", receivedID, tc.id)
			}
			if !stored {
				t.Fatal("valid identity lost pending authorization")
			}
			if events != 1 {
				t.Fatalf("valid identity events=%d, want 1", events)
			}
			if eventID != tc.id {
				t.Fatalf("event identity=%q, want %q", eventID, tc.id)
			}
		})
	}
}
