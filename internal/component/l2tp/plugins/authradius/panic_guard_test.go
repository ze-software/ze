// Design: docs/guide/l2tp.md -- panic containment in the CoA listener, the
// subscriber login and the per-session accounting task
// Related: panic_guard.go -- coaRequestTask, authRequestTask, accountingTask, the boundaries under test
// Related: rfc5176_coa_branches_test.go -- the fake service and the request helpers
// Related: rfc2865_handler_test.go -- setupAuth, fakeResponder
// Related: rfc2866_accounting_test.go -- capturedAccountant

package l2tpauthradius

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
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

// wantOneRecoveredCrash fails t unless the crash directory holds exactly one
// recovered record, and that record carries want.
func wantOneRecoveredCrash(t *testing.T, want string) {
	t.Helper()
	crashes := crashlog.ListCrashes()
	if len(crashes) != 1 || crashes[0].Kind != crashlog.KindRecovered {
		t.Fatalf("crash records = %+v, want one %q record", crashes, crashlog.KindRecovered)
	}
	if record := crashlog.ReadCrash(crashes[0].Name); !strings.Contains(record, want) {
		t.Fatalf("crash record lacks %q:\n%s", want, record)
	}
}

// TestRADIUSLoginPanicContained proves the subscriber login task contains a Ze
// defect: that login is rejected as an internal error, the panic is counted and
// crash-recorded on the real guard, and the next login is served.
// METHOD: doRADIUS is called on the test goroutine with a nil RADIUS client, a
// state only a defect produces (handle rejects a nil client before it starts
// the task), so SendToServers dereferences nil. The next login goes through
// handle to a mock server that answers Access-Accept.
// PREVENTS: a login panic ending the daemon, or being logged and rejected with
// no count and no crash record, so the operator never learns of the defect.
func TestRADIUSLoginPanicContained(t *testing.T) {
	before, declared := authGuard.Panics(taskAuthRequest)
	if !declared {
		t.Fatalf("task %q is not declared on the login boundary", taskAuthRequest)
	}
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))

	a, resp, cleanup := setupAuth(t, []byte("testing123"), radius.CodeAccessAccept)
	defer cleanup()
	login := ppp.EventAuthRequest{
		TunnelID: 7, SessionID: 9, Method: ppp.AuthMethodPAP,
		Username: "alice", Response: []byte("password123"),
	}
	policy := attributePolicy{nasPortIDFormat: a.nasPortIDFormat, exclusions: a.exclusions}

	a.doRADIUS(login, nil, "test-nas", nil, policy, resp.respond)
	if call := resp.waitOne(t); call.accept || call.message != "internal error" {
		t.Fatalf("login meeting the defect: got %+v, want a reject with \"internal error\"", call)
	}
	if count, _ := authGuard.Panics(taskAuthRequest); count != before+1 {
		t.Fatalf("Panics(%q) = %d, want %d: the panic was not counted", taskAuthRequest, count, before+1)
	}
	wantOneRecoveredCrash(t, "radius-auth "+taskAuthRequest+", unit tunnel 7 session 9")

	login.SessionID = 10
	if result := a.handle(login, resp.respond); !result.Handled {
		t.Fatal("next login: handle did not start the task")
	}
	if call := resp.waitOne(t); !call.accept {
		t.Fatalf("next login after the contained panic: got %+v, want accept", call)
	}
}

// TestRADIUSAccountingPanicContained proves one session's accounting task
// contains a Ze defect: that session sends no Accounting-Start, another
// session's accounting runs, and the first session's Accounting-Stop is still
// sent when it ends (RFC 2866 Section 2 ties the Stop to the end of service).
// METHOD: the acctNow clock seam panics on its first call after the boot tag is
// minted, which is the first session's buildAcctPacket on its accounting
// goroutine. A capture server records what reaches the wire.
// PREVENTS: an accounting panic ending the daemon, ending every session's
// accounting, or losing the session's Accounting-Stop and with it its billing.
func TestRADIUSAccountingPanicContained(t *testing.T) {
	before, declared := authGuard.Panics(taskAccounting)
	if !declared {
		t.Fatalf("task %q is not declared on the accounting boundary", taskAccounting)
	}
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))

	acct, capture := capturedAccountant(t)
	acct.bootTagMint.Do(acct.mintBootTag)
	var armed atomic.Bool
	saved := acctNow
	acctNow = func() time.Time {
		if armed.CompareAndSwap(true, false) {
			panic("boom")
		}
		return time.Now()
	}
	t.Cleanup(func() { acctNow = saved })
	armed.Store(true)

	acct.onSessionIPAssigned(&events.SessionIPAssignedPayload{
		TunnelID: 1, SessionID: 2, Username: "alice", PeerAddr: "10.0.0.1",
	})
	deadline := time.Now().Add(5 * time.Second)
	for count, _ := authGuard.Panics(taskAccounting); count != before+1; count, _ = authGuard.Panics(taskAccounting) {
		if time.Now().After(deadline) {
			t.Fatalf("Panics(%q) = %d, want %d: the panic was not counted", taskAccounting, count, before+1)
		}
		time.Sleep(10 * time.Millisecond)
	}
	wantOneRecoveredCrash(t, "radius-auth "+taskAccounting+", unit tunnel 1 session 2")

	acct.onSessionIPAssigned(&events.SessionIPAssignedPayload{
		TunnelID: 3, SessionID: 4, Username: "bob", PeerAddr: "10.0.0.2",
	})
	if start := capture.waitN(t, 1)[0]; start.statusType != radius.AcctStatusStart || start.username != "bob" {
		t.Fatalf("after the contained panic: got %+v, want bob's Accounting-Start", start)
	}

	acct.onSessionDown(&events.SessionDownPayload{TunnelID: 1, SessionID: 2})
	recorded := capture.waitN(t, 1)
	if len(recorded) != 2 {
		t.Fatalf("records on the wire: got %+v, want 2 (bob's Start, alice's Stop)", recorded)
	}
	if stop := recorded[1]; stop.statusType != radius.AcctStatusStop || stop.username != "alice" {
		t.Fatalf("session down after the contained panic: got %+v, want alice's Accounting-Stop", stop)
	}
}
