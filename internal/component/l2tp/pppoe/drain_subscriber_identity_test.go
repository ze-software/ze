// Design: docs/architecture/l2tp/subscriber-session-model.md -- refusal before unnamespaced pool admission

package pppoe

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
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
	driver.events <- ppp.EventIPRequest{TunnelID: 7, SessionID: 42, Family: ppp.AddressFamilyIPv4}
	close(driver.events)
	calls := 0
	handler := func(ppp.EventIPRequest) ppp.IPResponseArgs {
		calls++
		return ppp.IPResponseArgs{Accept: true, Family: ppp.AddressFamilyIPv4}
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	done := startPPPoEPoolDrain(logger, driver, subscriber.PoolHandler(handler), newRecordBus())
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
