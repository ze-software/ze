// Design: docs/guide/l2tp.md -- panic containment in the CoA listener's request task
// Related: panic_guard.go -- coaRequestTask, the boundary under test
// Related: rfc5176_coa_branches_test.go -- the fake service and the request helpers

package l2tpauthradius

import (
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/core/crashlog"
)

// teardownPanickingService is the one-session service whose first teardown
// panics, standing in for a Ze defect met while a Disconnect-Request is served.
type teardownPanickingService struct {
	*fakeL2TPService
	armed atomic.Bool
}

func (s *teardownPanickingService) TeardownSession(localSID uint16) error {
	if s.armed.CompareAndSwap(true, false) {
		panic("boom")
	}
	return s.fakeL2TPService.TeardownSession(localSID)
}

// TestCoAPanicContained proves the CoA listener's request task contains a Ze
// defect met while it serves one authenticated request: that request is
// answered with a Disconnect-NAK carrying Error-Cause 506 (Resources
// Unavailable), the listener answers the next request, and the panic is counted
// and crash-recorded on the real guard.
// METHOD: the published L2TP service panics on its first TeardownSession, which
// handleDisconnect calls on the listener's goroutine. Two Disconnect-Requests
// for the same session follow: the first meets the panic, the second succeeds.
// PREVENTS: a peer-reachable panic in the CoA listener killing the daemon, or
// ending the listener and with it every later CoA and Disconnect request.
func TestCoAPanicContained(t *testing.T) {
	before, declared := coaGuard.Panics(taskCoARequest)
	if !declared {
		t.Fatalf("task %q is not declared on the CoA boundary", taskCoARequest)
	}
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))

	secret := []byte("test-coa-panic-secret")
	svc := &teardownPanickingService{fakeL2TPService: oneSessionService()}
	svc.armed.Store(true)
	l2tp.PublishService(svc)
	t.Cleanup(func() { l2tp.PublishService(nil) })
	addr := coaTestListener(t, secret, nil, nil)
	session := []radius.Attr{{Type: radius.AttrAcctSessionID, Value: radius.AttrString("10-20-1")}}

	wantNAK(t, sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, session),
		radius.CodeDisconnectNAK, radius.ErrorCauseResourcesUnavailable)
	if svc.armed.Load() {
		t.Fatal("the teardown never ran: the seam was not reached")
	}
	if resp := sendCoAPacket(t, addr, radius.CodeDisconnectRequest, secret, session); resp.Code != radius.CodeDisconnectACK {
		t.Fatalf("request after the contained panic: code %d, want %d (Disconnect-ACK): the listener did not carry on",
			resp.Code, radius.CodeDisconnectACK)
	}
	if got := svc.teardowns.Load(); got != 1 {
		t.Errorf("teardowns after the panic: got %d, want 1", got)
	}

	if count, _ := coaGuard.Panics(taskCoARequest); count != before+1 {
		t.Fatalf("Panics(%q) = %d, want %d: the panic was not counted", taskCoARequest, count, before+1)
	}
	crashes := crashlog.ListCrashes()
	if len(crashes) != 1 || crashes[0].Kind != crashlog.KindRecovered {
		t.Fatalf("crash records = %+v, want one %q record", crashes, crashlog.KindRecovered)
	}
	want := "radius-coa " + taskCoARequest + ", unit request from 127.0.0.1:"
	if record := crashlog.ReadCrash(crashes[0].Name); !strings.Contains(record, want) {
		t.Fatalf("crash record lacks %q:\n%s", want, record)
	}
}
