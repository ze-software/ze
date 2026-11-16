// Design: docs/architecture/l2tp/subscriber-session-model.md -- NAS authentication requires subscriber identity

package l2tp

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
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
// missing identity and a valid opaque identity. Only the valid request reaches
// the provider, and its result event retains the exact ID supplied by PPP.
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
			driver := &identityAuthDriver{events: make(chan ppp.AuthEvent, 1), err: tc.err}
			driver.events <- ppp.EventAuthRequest{SubscriberID: tc.id,
				TunnelID: 7, SessionID: 42, Method: ppp.AuthMethodNone, Username: "alice"}
			close(driver.events)
			bus := newTestBus()
			var eventID string
			events := 0
			unsubscribe := subevents.SessionAuthResult.Subscribe(bus, func(result *subevents.SessionAuthResultPayload) {
				events++
				eventID = result.SessionID
			})
			defer unsubscribe()
			resolved, called := 0, 0
			var receivedID string
			done := startAuthDrain(slog.New(slog.NewTextHandler(io.Discard, nil)), driver,
				func() AuthHandler {
					resolved++
					return func(request ppp.EventAuthRequest, _ AuthRespondFunc) AuthResult {
						called++
						receivedID = request.SubscriberID
						return AuthResult{Accept: true}
					}
				}, bus)
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("auth drain did not join after input closure")
			}
			if driver.calls != 1 {
				t.Fatalf("authentication reply calls=%d, want 1", driver.calls)
			}
			if tc.id == "" {
				if resolved != 0 {
					t.Fatalf("missing identity resolved provider %d times", resolved)
				}
				if called != 0 {
					t.Fatalf("missing identity called provider %d times", called)
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
			if events != 1 {
				t.Fatalf("valid identity events=%d, want 1", events)
			}
			if eventID != tc.id {
				t.Fatalf("event identity=%q, want %q", eventID, tc.id)
			}
		})
	}
}
