// Design: docs/architecture/l2tp/subscriber-session-model.md -- undelivered pool answers

package pppoe

import (
	"io"
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
)

type undeliveredPoolDriver struct {
	events    chan ppp.IPEvent
	responses int
	err       error
}

func (d *undeliveredPoolDriver) IPEventsOut() <-chan ppp.IPEvent { return d.events }

func (d *undeliveredPoolDriver) IPResponse(_, _ uint16, _ ppp.IPResponseArgs) error {
	d.responses++
	return d.err
}

// TestPoolDrainReleasesUndeliveredAnswer proves PPPoE publishes the exact
// allocation identity only when a successful pool answer cannot be delivered.
// Closing the input and joining the production drain bounds the test lifetime.
func TestPoolDrainReleasesUndeliveredAnswer(t *testing.T) {
	for _, tc := range []struct {
		name    string
		accept  bool
		id      uint64
		err     error
		release bool
	}{
		{name: "session-ended", accept: true, id: 29, err: ppp.ErrSessionNotFound, release: true},
		{name: "answer-pending", accept: true, id: 30, err: ppp.ErrIPResponsePending, release: true},
		{name: "delivered", accept: true, id: 31},
		{name: "refused", id: 32, err: ppp.ErrSessionNotFound},
		{name: "no-allocation", accept: true, err: ppp.ErrSessionNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bus := newRecordBus()
			releases := make(chan subevents.PoolAnswerUndeliveredPayload, 1)
			unsubscribe := subevents.PoolAnswerUndelivered.Subscribe(bus, func(payload *subevents.PoolAnswerUndeliveredPayload) {
				releases <- *payload
			})
			defer unsubscribe()
			driver := &undeliveredPoolDriver{events: make(chan ppp.IPEvent, 1), err: tc.err}
			driver.events <- ppp.EventIPRequest{SubscriberID: "pppoe-9-13", TunnelID: 9, SessionID: 13, Family: ppp.AddressFamilyIPv4}
			close(driver.events)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			handler := func(req ppp.EventIPRequest) ppp.IPResponseArgs {
				return ppp.IPResponseArgs{Accept: tc.accept, Family: req.Family,
					Peer: netip.MustParseAddr("192.0.2.10"), AllocationID: tc.id}
			}
			done := startPPPoEPoolDrain(logger, driver, subscriber.PoolHandler(handler), bus)
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("pool drain did not finish after its input closed")
			}
			if driver.responses != 1 {
				t.Fatalf("response count = %d, want 1", driver.responses)
			}
			select {
			case release := <-releases:
				if !tc.release {
					t.Fatalf("unexpected allocation release: %+v", release)
				}
				if release.TunnelID != 9 || release.SessionID != 13 || release.AllocationID != tc.id {
					t.Fatalf("release = %+v, want allocation %d for 9/13", release, tc.id)
				}
			default:
				if tc.release {
					t.Fatal("undelivered allocation was not released")
				}
			}
		})
	}
}
