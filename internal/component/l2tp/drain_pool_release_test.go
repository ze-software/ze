// Design: docs/architecture/l2tp/subscriber-session-model.md -- undelivered pool answers

package l2tp

import (
	"io"
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
)

type undeliveredPoolDriver struct {
	events    chan ppp.IPEvent
	responses int
	err       error
}

func (d *undeliveredPoolDriver) IPEventsOut() <-chan ppp.IPEvent { return d.events }

func (d *undeliveredPoolDriver) IPResponse(_ ppp.EventIPRequest, _ ppp.IPResponseArgs) error {
	d.responses++
	return d.err
}

// TestPoolDrainReleasesUndeliveredAnswer proves the production drain names the
// exact allocation to release after delivery fails, while preserving delivered
// answers and refusals. A closed input and joined drain make the observations
// independent of scheduling.
func TestPoolDrainReleasesUndeliveredAnswer(t *testing.T) {
	for _, tc := range []struct {
		name    string
		accept  bool
		id      uint64
		err     error
		release bool
	}{
		{name: "session-ended", accept: true, id: 19, err: ppp.ErrSessionNotFound, release: true},
		{name: "answer-pending", accept: true, id: 20, err: ppp.ErrIPResponsePending, release: true},
		{name: "delivered", accept: true, id: 21},
		{name: "refused", id: 22, err: ppp.ErrSessionNotFound},
		{name: "no-allocation", accept: true, err: ppp.ErrSessionNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bus := newTestBus()
			releases := make(chan subevents.PoolAnswerUndeliveredPayload, 1)
			unsubscribe := subevents.PoolAnswerUndelivered.Subscribe(bus, func(payload *subevents.PoolAnswerUndeliveredPayload) {
				releases <- *payload
			})
			defer unsubscribe()
			driver := &undeliveredPoolDriver{events: make(chan ppp.IPEvent, 1), err: tc.err}
			wire := newCurrentCoreAdmission(t, nil, &currentCoreAdmissionConfig{ids: currentCoreSessionIDs{tunnelID: 7, sessionID: 11}, ipv6: false, subscriberID: "l2tp-7-11"})
			wire.acceptAuth(t, wire.authRequest(t, ""))
			driver.events <- wire.poolRequest(t)
			close(driver.events)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			done := startPoolDrain(logger, driver, func() PoolHandler {
				return func(req ppp.EventIPRequest) ppp.IPResponseArgs {
					return ppp.IPResponseArgs{Accept: tc.accept, Family: req.Family,
						Peer: netip.MustParseAddr("192.0.2.10"), AllocationID: tc.id}
				}
			}, bus)
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
				if release.TunnelID != 7 || release.SessionID != 11 || release.AllocationID != tc.id {
					t.Fatalf("release = %+v, want allocation %d for 7/11", release, tc.id)
				}
			default:
				if tc.release {
					t.Fatal("undelivered allocation was not released")
				}
			}
		})
	}
}
