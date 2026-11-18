// Design: docs/guide/l2tp.md -- original session capture in dynamic authorization
// Related: rfc5176_coa_listener_test.go -- socket fixture service

package l2tpauthradius

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// admitCoAFixtureOriginals assigns explicit fixture origins before publication.
// A supplied origin remains unchanged; no production reader mints an identity.
func admitCoAFixtureOriginals(t *testing.T, fixture *fakeL2TPService) {
	t.Helper()
	for index := range fixture.snap.Tunnels {
		for row := range fixture.snap.Tunnels[index].Sessions {
			original := &fixture.snap.Tunnels[index].Sessions[row]
			if original.Birth.IsValid() {
				continue
			}
			birth, err := subscriberid.NewL2TPBirth()
			if err != nil {
				t.Fatal(err)
			}
			original.Birth = birth
			original.SubscriberID = birth.String()
		}
	}
}

// coaFixtureCapture retains one original snapshot row. It never delegates
// an operation to LookupSession or TeardownSession with a reusable SID.
type coaFixtureCapture struct {
	service  *fakeL2TPService
	original *l2tp.SessionSnapshot
	birth    subscriberid.L2TPBirth
}

func (f *fakeL2TPService) CaptureSession(selector l2tp.SessionSelector) (l2tp.CapturedSession, error) {
	var matched *l2tp.SessionSnapshot
	for index := range f.snap.Tunnels {
		for row := range f.snap.Tunnels[index].Sessions {
			original := &f.snap.Tunnels[index].Sessions[row]
			if original.LocalSID != selector.SID {
				continue
			}
			if selector.TID != 0 {
				if original.TunnelLocalTID != selector.TID {
					continue
				}
			}
			if selector.Listener.IsValid() {
				if original.Listener != selector.Listener {
					continue
				}
			}
			if selector.Birth.IsValid() {
				if original.Birth != selector.Birth {
					continue
				}
			}
			if matched != nil {
				return nil, l2tp.ErrSessionAmbiguous
			}
			matched = original
		}
	}
	if matched == nil {
		return nil, l2tp.ErrSessionNotFound
	}
	if !matched.Birth.IsValid() {
		return nil, l2tp.ErrSessionNotFound
	}
	return &coaFixtureCapture{service: f, original: matched, birth: matched.Birth}, nil
}

func (original *coaFixtureCapture) Birth() subscriberid.L2TPBirth { return original.birth }
func (original *coaFixtureCapture) Listener() netip.AddrPort      { return original.original.Listener }
func (original *coaFixtureCapture) Snapshot() (l2tp.SessionSnapshot, error) {
	for index := range original.service.snap.Tunnels {
		for row := range original.service.snap.Tunnels[index].Sessions {
			current := &original.service.snap.Tunnels[index].Sessions[row]
			if current != original.original {
				continue
			}
			if current.Birth != original.birth {
				return l2tp.SessionSnapshot{}, l2tp.ErrSessionNotFound
			}
			return *current, nil
		}
	}
	return l2tp.SessionSnapshot{}, l2tp.ErrSessionNotFound
}
func (*coaFixtureCapture) FSMHistory() ([]l2tp.FSMTransition, error) {
	return nil, errors.New("fixture has no history owner")
}
func (*coaFixtureCapture) SessionEvents() ([]l2tp.ObserverEvent, error) {
	return nil, errors.New("fixture has no observer owner")
}
func (*coaFixtureCapture) Counters() (ppp.SessionCounters, error) {
	return ppp.SessionCounters{}, errors.New("fixture has no counter owner")
}
func (*coaFixtureCapture) RecordDisconnect(string, string, uint32) error {
	return errors.New("fixture has no audit owner")
}
func (original *coaFixtureCapture) Teardown(ctx context.Context, _ l2tpevents.TerminateCause, _ string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := original.Snapshot(); err != nil {
		return err
	}
	original.service.teardowns.Add(1)
	return nil
}

type coaRefusingCapture struct {
	l2tp.CapturedSession
	owner *teardownRefusingService
}

func (original *coaRefusingCapture) Teardown(context.Context, l2tpevents.TerminateCause, string) error {
	original.owner.attempts.Add(1)
	return errors.New("teardown refused by the test")
}

type coaPanickingCapture struct {
	l2tp.CapturedSession
	owner *teardownPanickingService
}

func (original *coaPanickingCapture) Teardown(ctx context.Context, cause l2tpevents.TerminateCause, reason string) error {
	if original.owner.armed.CompareAndSwap(true, false) {
		panic("boom")
	}
	return original.CapturedSession.Teardown(ctx, cause, reason)
}
