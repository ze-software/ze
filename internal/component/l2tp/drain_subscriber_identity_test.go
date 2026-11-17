// Design: docs/architecture/l2tp/subscriber-session-model.md -- refusal before unnamespaced pool admission

package l2tp

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

type missingIdentityPoolDriver struct {
	events chan ppp.IPEvent
	answer ppp.IPResponseArgs
}

func (d *missingIdentityPoolDriver) IPEventsOut() <-chan ppp.IPEvent { return d.events }
func (d *missingIdentityPoolDriver) IPResponse(_ ppp.EventIPRequest, answer ppp.IPResponseArgs) error {
	d.answer = answer
	return nil
}

// TestPoolDrainRefusesMissingSubscriberIdentity proves the transport refuses
// an unnamespaced request before invoking any allocation handler.
func TestPoolDrainRefusesMissingSubscriberIdentity(t *testing.T) {
	driver := &missingIdentityPoolDriver{events: make(chan ppp.IPEvent, 1)}
	wire := newCurrentCoreAdmission(t, nil, &currentCoreAdmissionConfig{ids: currentCoreSessionIDs{tunnelID: 7, sessionID: 42}, ipv6: false, subscriberID: ""})
	wire.acceptAuth(t, wire.authRequest(t, ""))
	request := wire.poolRequest(t)
	request.SubscriberID = "" // deliberate malformed consumer input; actual private request is retained
	driver.events <- request
	close(driver.events)
	calls := 0
	handler := func(ppp.EventIPRequest) ppp.IPResponseArgs {
		calls++
		return ppp.IPResponseArgs{Accept: true, Family: ppp.AddressFamilyIPv4}
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	done := startPoolDrain(logger, driver, func() PoolHandler { return handler }, newTestBus())
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("drain did not join after input closure")
	}
	if calls != 0 {
		t.Fatalf("drain called allocator %d times for a missing identity", calls)
	}
	if driver.answer.Accept {
		t.Fatal("drain admitted a request with no subscriber identity")
	}
	if driver.answer.Reason == "" {
		t.Fatal("missing identity refusal has no reason")
	}
}

// TestPoolDrainMissingIdentityDoesNotResolveProvider drives the production
// drain with missing and opaque IDs, counting lookup separately from invocation.
// Joining the closed-input drain makes both counts and its reply final.
func TestPoolDrainMissingIdentityDoesNotResolveProvider(t *testing.T) {
	for _, tc := range []struct {
		name string
		id   string
	}{
		{name: "missing"},
		{name: "opaque", id: "opaque/pool-owner-65543"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			driver := &missingIdentityPoolDriver{events: make(chan ppp.IPEvent, 1)}
			wire := newCurrentCoreAdmission(t, nil, &currentCoreAdmissionConfig{ids: currentCoreSessionIDs{tunnelID: 7, sessionID: 42}, ipv6: false, subscriberID: tc.id})
			wire.acceptAuth(t, wire.authRequest(t, ""))
			request := wire.poolRequest(t)
			request.SubscriberID = tc.id // only missing-ID rows edit actual producer metadata
			driver.events <- request
			close(driver.events)
			resolved, called := 0, 0
			var receivedID string
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			done := startPoolDrain(logger, driver, func() PoolHandler {
				resolved++
				return func(request ppp.EventIPRequest) ppp.IPResponseArgs {
					called++
					receivedID = request.SubscriberID
					return ppp.IPResponseArgs{Accept: true, Family: request.Family}
				}
			}, newTestBus())
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("pool drain did not join after input closure")
			}
			if driver.answer.Family != ppp.AddressFamilyIPv4 {
				t.Fatalf("reply family=%v, want IPv4", driver.answer.Family)
			}
			if tc.id == "" {
				if resolved != 0 {
					t.Fatalf("missing identity resolved provider %d times", resolved)
				}
				if called != 0 {
					t.Fatalf("missing identity called allocator %d times", called)
				}
				if driver.answer.Accept {
					t.Fatal("missing identity received an accepted address answer")
				}
				if driver.answer.Reason == "" {
					t.Fatal("missing identity refusal has no reason")
				}
				return
			}
			if resolved != 1 {
				t.Fatalf("valid identity resolved provider %d times, want 1", resolved)
			}
			if called != 1 {
				t.Fatalf("valid identity allocator calls=%d, want 1", called)
			}
			if receivedID != tc.id {
				t.Fatalf("allocator identity=%q, want %q", receivedID, tc.id)
			}
			if !driver.answer.Accept {
				t.Fatalf("valid opaque identity refused: %s", driver.answer.Reason)
			}
		})
	}
}
