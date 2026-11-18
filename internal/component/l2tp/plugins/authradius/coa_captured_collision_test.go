// Design: docs/guide/l2tp.md -- dynamic authorization retains the matched original.
// Related: coa.go -- signed UDP CoA and Disconnect handlers.
// Related: coa_fixture_owners_test.go -- actual registered shaping owner.
package l2tpauthradius

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/core/subscriberid"
)

// Goal: an exact accounting match on the later tunnel terminates that original,
// not an earlier equal-SID subscriber. Method: signed UDP reaches the real
// listener and its supplied service; only the captured original can terminate.
// The service models row retirement, not native cleanup or Accounting-Stop.
// MUTATION: reduce the matched original to SID and call TeardownSession again.
func TestDisconnectExactAccountingMatchKeepsEqualSIDSurvivor(t *testing.T) {
	for _, target := range []int{0, 1} {
		t.Run([]string{"earlier", "later"}[target], func(t *testing.T) {
			address, secret, service, _ := newCoACollisionFixture(t)
			response := sendAppliedCoA(t, address, radius.CodeDisconnectRequest, secret, coaCollisionAttributes(target))
			if response.Code != radius.CodeDisconnectACK {
				t.Fatalf("exact original received code %d, want Disconnect-ACK", response.Code)
			}
			want := [3]bool{true, true, false}
			want[target] = false
			service.requireRows(t, want)
			service.requireCapturedOperation(t)
		})
	}
}

// Goal: a rate change selected by a complete accounting ID keeps its original
// interface despite a SID collision. Method: signed UDP traverses the actual
// shaper command and checks both installed backend rates after the ACK.
// MUTATION: perform LookupSession(matchedSID) after the accounting match.
func TestCoAExactAccountingMatchKeepsEqualSIDSurvivor(t *testing.T) {
	for _, target := range []int{0, 1} {
		t.Run([]string{"earlier", "later"}[target], func(t *testing.T) {
			address, secret, service, backend := newCoACollisionFixture(t)
			attributes := append(coaCollisionAttributes(target), radius.Attr{Type: radius.AttrFilterID, Value: radius.AttrString("20mbit/8mbit")})
			response := sendAppliedCoA(t, address, radius.CodeCoARequest, secret, attributes)
			if response.Code != radius.CodeCoAACK {
				t.Fatalf("exact original received code %d, want CoA-ACK", response.Code)
			}
			for index := range 2 {
				download, upload := uint64(10_000_000), uint64(5_000_000)
				if index == target {
					download, upload = 20_000_000, 8_000_000
				}
				coaCollisionRequireRates(t, backend, service.originals[index].snapshot.PppInterface, download, upload)
			}
			service.requireRows(t, [3]bool{true, true, false})
			service.requireCapturedOperation(t)
		})
	}
}

// Goal: a matched original that ends before capture cannot confer authority on
// its equal-label successor or the earlier tunnel. Method: the service returns
// the original matching snapshot, then replaces that row before the handler's
// next operation. Both signed UDP request types must refuse without mutation.
// MUTATION: use a fresh SID lookup instead of the matched Birth-qualified capture.
func TestDynamicAuthorizationStaleAccountingMatchKeepsReplacement(t *testing.T) {
	for _, request := range []struct {
		name  string
		code  uint8
		nak   uint8
		cause uint32
	}{
		{name: "coa", code: radius.CodeCoARequest, nak: radius.CodeCoANAK, cause: radius.ErrorCauseSessionNotFound},
		{name: "disconnect", code: radius.CodeDisconnectRequest, nak: radius.CodeDisconnectNAK, cause: radius.ErrorCauseSessionNotRemovable},
	} {
		t.Run(request.name, func(t *testing.T) {
			address, secret, service, backend := newCoACollisionFixture(t)
			service.mu.Lock()
			service.replaceAfterSnapshot = true
			service.mu.Unlock()
			attributes := coaCollisionAttributes(1)
			if request.code == radius.CodeCoARequest {
				attributes = append(attributes, radius.Attr{Type: radius.AttrFilterID, Value: radius.AttrString("20mbit/8mbit")})
			}
			response := sendAppliedCoA(t, address, request.code, secret, attributes)
			wantNAK(t, response, request.nak, request.cause)
			service.requireRows(t, [3]bool{true, false, true})
			for index := range 2 {
				coaCollisionRequireRates(t, backend, service.originals[index].snapshot.PppInterface, 10_000_000, 5_000_000)
			}
			service.requireCapturedOperation(t)
		})
	}
}

type coaCollisionOriginal struct {
	snapshot l2tp.SessionSnapshot // immutable after the fixture publishes it
	alive    bool                 // protected by the service's mu
}

// Safe for concurrent use. This service owns three fixed rows, including the
// initially absent replacement. No interface name or numeric label is an owner.
type coaCollisionService struct {
	*fakeL2TPService
	mu                   sync.Mutex
	originals            [3]*coaCollisionOriginal
	replaceAfterSnapshot bool
	captures             int
	legacyOperations     int
}

func (service *coaCollisionService) Snapshot() l2tp.Snapshot {
	service.mu.Lock()
	defer service.mu.Unlock()
	var snapshot l2tp.Snapshot
	for _, original := range service.originals {
		if original.alive {
			snapshot.Tunnels = append(snapshot.Tunnels, l2tp.TunnelSnapshot{LocalTID: original.snapshot.TunnelLocalTID, Sessions: []l2tp.SessionSnapshot{original.snapshot}})
		}
	}
	if service.replaceAfterSnapshot {
		service.replaceAfterSnapshot = false
		service.originals[1].alive = false
		service.originals[2].alive = true
	}
	return snapshot
}

// The legacy methods deliberately retain the existing first-match semantics;
// a handler that loses its original therefore produces an observable own RED.
func (service *coaCollisionService) LookupSession(sid uint16) (l2tp.SessionSnapshot, bool) {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.legacyOperations++
	for _, original := range service.originals {
		if original.alive {
			if original.snapshot.LocalSID == sid {
				return original.snapshot, true
			}
		}
	}
	return l2tp.SessionSnapshot{}, false
}

func (service *coaCollisionService) TeardownSession(sid uint16) error {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.legacyOperations++
	for _, original := range service.originals {
		if original.alive {
			if original.snapshot.LocalSID == sid {
				original.alive = false
				return nil
			}
		}
	}
	return l2tp.ErrSessionNotFound
}

func (service *coaCollisionService) CaptureSession(selector l2tp.SessionSelector) (l2tp.CapturedSession, error) {
	service.mu.Lock()
	defer service.mu.Unlock()
	service.captures++
	var matched *coaCollisionOriginal
	for _, original := range service.originals {
		if !original.alive {
			continue
		}
		if original.snapshot.LocalSID != selector.SID {
			continue
		}
		if selector.TID != 0 {
			if original.snapshot.TunnelLocalTID != selector.TID {
				continue
			}
		}
		if selector.Birth.IsValid() {
			if original.snapshot.Birth != selector.Birth {
				continue
			}
		}
		if selector.Listener.IsValid() {
			if original.snapshot.Listener != selector.Listener {
				continue
			}
		}
		if matched != nil {
			return nil, l2tp.ErrSessionAmbiguous
		}
		matched = original
	}
	if matched == nil {
		return nil, l2tp.ErrSessionNotFound
	}
	return &coaCollisionCapture{service: service, original: matched}, nil
}

func (service *coaCollisionService) requireRows(t *testing.T, want [3]bool) {
	t.Helper()
	service.mu.Lock()
	defer service.mu.Unlock()
	for index, original := range service.originals {
		if original.alive != want[index] {
			t.Errorf("original row %d alive=%v, want %v", index, original.alive, want[index])
		}
	}
}

func (service *coaCollisionService) requireCapturedOperation(t *testing.T) {
	t.Helper()
	service.mu.Lock()
	defer service.mu.Unlock()
	if service.captures != 1 {
		t.Errorf("handler captured %d originals, want exactly one", service.captures)
	}
	if service.legacyOperations != 0 {
		t.Errorf("matched original was reduced to %d reusable-SID operations", service.legacyOperations)
	}
}

// Only CaptureSession constructs this fixture capability. Its methods recheck
// that exact row; none can follow reused labels to a different fixture row.
type coaCollisionCapture struct {
	service  *coaCollisionService
	original *coaCollisionOriginal
}

func (capture *coaCollisionCapture) Birth() subscriberid.L2TPBirth {
	return capture.original.snapshot.Birth
}
func (capture *coaCollisionCapture) Listener() netip.AddrPort {
	return capture.original.snapshot.Listener
}
func (capture *coaCollisionCapture) Snapshot() (l2tp.SessionSnapshot, error) {
	capture.service.mu.Lock()
	defer capture.service.mu.Unlock()
	if !capture.original.alive {
		return l2tp.SessionSnapshot{}, l2tp.ErrSessionNotFound
	}
	return capture.original.snapshot, nil
}
func (*coaCollisionCapture) FSMHistory() ([]l2tp.FSMTransition, error) {
	return nil, errors.New("fixture has no FSM history")
}
func (*coaCollisionCapture) SessionEvents() ([]l2tp.ObserverEvent, error) {
	return nil, errors.New("fixture has no observer history")
}
func (*coaCollisionCapture) Counters() (ppp.SessionCounters, error) {
	return ppp.SessionCounters{}, errors.New("fixture has no native counters")
}
func (*coaCollisionCapture) RecordDisconnect(string, string, uint32) error {
	return errors.New("fixture has no audit sink")
}
func (capture *coaCollisionCapture) Teardown(ctx context.Context, cause l2tpevents.TerminateCause, _ string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if cause != l2tpevents.TerminateCauseAdminReset {
		return errors.New("fixture expected the original administrative disconnect cause")
	}
	capture.service.mu.Lock()
	defer capture.service.mu.Unlock()
	if !capture.original.alive {
		return l2tp.ErrSessionNotFound
	}
	capture.original.alive = false
	return nil
}

func newCoACollisionFixture(t *testing.T) (string, []byte, *coaCollisionService, *coaApplyBackend) {
	t.Helper()
	service := &coaCollisionService{fakeL2TPService: new(fakeL2TPService)}
	for index := range service.originals {
		birth, err := subscriberid.NewL2TPBirth()
		if err != nil {
			t.Fatal(err)
		}
		tunnel := uint16(62011)
		if index > 0 {
			tunnel = 62012
		}
		service.originals[index] = &coaCollisionOriginal{alive: index < 2, snapshot: l2tp.SessionSnapshot{
			Birth: birth, SubscriberID: birth.String(), Listener: netip.MustParseAddrPort("127.0.0.1:62001"),
			LocalSID: 421, TunnelLocalTID: tunnel, Username: "ac2-collision",
			PppInterface: coaFixtureInterface(tunnel, 421),
		}}
	}
	l2tp.PublishService(service)
	t.Cleanup(func() { l2tp.PublishService(nil) })
	dispatch := coaFixtureDispatch(t)
	value, loaded := coaFixtureOwnersByTest.Load(t)
	if !loaded {
		t.Fatal("collision fixture has no registered authorization owners")
	}
	owners, ok := value.(*coaFixtureOwners)
	if !ok {
		t.Fatalf("collision fixture owners are %T", value)
	}
	for index := range 2 {
		seedCoAAccountingIdentity(t, &acctSession{birth: service.originals[index].snapshot.Birth,
			tunnelID:  service.originals[index].snapshot.TunnelLocalTID,
			sessionID: 421, username: "ac2-collision", acctSessID: coaCollisionAccountingID(index),
			pppInterface: service.originals[index].snapshot.PppInterface})
		coaCollisionRequireRates(t, owners.rateBackend, service.originals[index].snapshot.PppInterface, 10_000_000, 5_000_000)
	}
	secret := []byte("ac2-original-accounting-collision")
	listener, err := newCoAListener(coaListenerConfig{AllowedSources: coaLoopbackSources(), DefaultSecret: secret,
		Bus: &recordingBus{}, Dispatch: dispatch})
	if err != nil {
		t.Fatal(err)
	}
	// The fixture MUST close and join its listener before its original owners
	// and service are released; Cleanup's reverse order preserves this pair.
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Errorf("close original collision listener: %v", err)
		}
	})
	return listener.conn.LocalAddr().String(), secret, service, owners.rateBackend
}

func coaCollisionAccountingID(index int) string {
	return []string{"ac2-exact-earlier-account", "ac2-exact-later-account"}[index]
}

func coaCollisionAttributes(index int) []radius.Attr {
	return []radius.Attr{
		{Type: radius.AttrAcctSessionID, Value: radius.AttrString(coaCollisionAccountingID(index))},
		{Type: radius.AttrUserName, Value: radius.AttrString("ac2-collision")},
		{Type: radius.AttrNASPort, Value: radius.AttrUint32(421)},
	}
}

func coaCollisionRequireRates(t *testing.T, backend *coaApplyBackend, name string, download, upload uint64) {
	t.Helper()
	qos, err := backend.ListQdiscs(name)
	if err != nil {
		t.Fatal(err)
	}
	if qos.Qdisc.RateBps != download {
		t.Errorf("%s download=%d, want %d", name, qos.Qdisc.RateBps, download)
	}
	if qos.Ingress.RateBps != upload {
		t.Errorf("%s upload=%d, want %d", name, qos.Ingress.RateBps, upload)
	}
}
