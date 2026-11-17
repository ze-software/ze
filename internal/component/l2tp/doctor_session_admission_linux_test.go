// Design: docs/architecture/doctor-and-health-checks.md -- transient PPP admission failures

package l2tp

import (
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/doctor"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// Goal: a refused factory is visible through runtime doctor; a successful,
// stale or unclaimed delivery must not invent that historical failure.
// Method: drive the existing handleKernelSuccess descriptor-transfer seam.
func TestKernelPPPAdmissionDoctorEntry(t *testing.T) {
	for _, outcome := range []string{"refused", "success", "stale", "unclaimed"} {
		t.Run(outcome, func(t *testing.T) {
			cause := errors.Join(errors.New("eventfd: too many open files"), errors.New("close unit: input/output error"))
			testKernelPPPAdmissionDoctorOutcome(t, outcome, cause)
		})
	}
}

// Goal: an arbitrarily long backend error cannot make the report bus silently
// reject an actual constructor-refusal event. Method: inject the error through
// the existing factory seam and inspect runtime doctor and the real bus entry.
func TestKernelPPPAdmissionLongCauseRetained(t *testing.T) {
	testKernelPPPAdmissionDoctorOutcome(t, "refused", errors.New(strings.Repeat("x", report.MessageMax+100)))
}

func testKernelPPPAdmissionDoctorOutcome(t *testing.T, outcome string, cause error) {
	t.Helper()
	// This observable code is a contract literal so tests compile before any
	// proposed production code or registration exists.
	expectedCode := "doctor-ppp-session-admission"
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	_, reactor, stop := newUnstartedReactor(t)
	defer stop()
	driver := newFakePPPDriver()
	reactor.setPPPDriver(driver)
	tunnel := mkTunnel(reactor, 100, 200, netip.MustParseAddrPort("192.0.2.7:1701"))
	session := addEstablishedSession(tunnel, 1001, 2001, true)
	fds := &pppSessionFDs{pppoxFD: 30, chanFD: 31, unitFD: 32, unitNum: 7}
	event := kernelSetupSucceeded{localTID: 100, localSID: 1001, session: session, owner: fds, fds: *fds}
	reactor.kernelWorker = &kernelWorker{
		sessions: map[sessionKey]*pppSessionFDs{{100, 1001}: fds},
		eventCh:  make(chan any, 4), stop: make(chan struct{}),
	}
	if outcome == "stale" {
		event.session = &L2TPSession{localSID: 1001}
	}
	if outcome == "unclaimed" {
		event.owner = &pppSessionFDs{}
	}
	factoryCalls := 0
	reactor.newPPPSession = func(cfg pppkernel.SessionConfig) (ppp.FrameChannel, ppp.SessionHandle, error) {
		factoryCalls++
		if outcome == "refused" {
			return nil, nil, cause
		}
		return dispatchProtocolSessionFixture(cfg)
	}
	reactor.handleKernelSuccess(event)
	expectCalls := 1
	if outcome == "stale" || outcome == "unclaimed" {
		expectCalls = 0
	}
	if factoryCalls != expectCalls {
		t.Fatalf("factory calls = %d, want %d", factoryCalls, expectCalls)
	}
	var rows []diagnostic.Diagnostic
	findings := doctor.RunRuntimeChecks(nil, nil)
	for index := range findings {
		finding := &findings[index]
		if finding.Code == expectedCode {
			rows = append(rows, *finding)
		}
	}
	if outcome != "refused" {
		if len(rows) != 0 || len(report.Errors(0)) != 0 {
			t.Fatalf("non-refusal reported an admission error: %+v", rows)
		}
		if outcome == "success" {
			select {
			case start := <-driver.sessionsIn:
				if start.TunnelID != 100 || start.SessionID != 1001 {
					t.Fatalf("successful constructor dispatched another session: %+v", start)
				}
				if err := start.Dataplane.Close(); err != nil {
					t.Fatal(err)
				}
			default:
				t.Fatal("successful constructor did not reach the PPP driver")
			}
		}
		return
	}
	if len(rows) != 1 || len(report.Errors(0)) != 1 {
		t.Fatalf("refusal must produce one doctor row and one event: %+v", rows)
	}
	row := rows[0]
	if row.Fix != "ze show errors" || !strings.Contains(row.Message, "tunnel 100 session 1001") {
		t.Fatalf("missing identity or next command: %+v", row)
	}
	issue := report.Errors(0)[0]
	if issue.Detail["ppp-unit"] != 7 || issue.Detail["error"] != cause.Error() {
		t.Fatalf("missing complete constructor detail: %+v", issue)
	}
	if len(issue.Message) > report.MessageMax {
		t.Fatal("constructor event exceeded the bus message bound")
	}
	if len(cause.Error()) < report.MessageMax && !strings.Contains(row.Message, cause.Error()) {
		t.Fatalf("doctor omitted joined constructor and rollback causes: %+v", row)
	}
	select {
	case <-driver.sessionsIn:
		t.Fatal("refused owner reached PPP driver")
	default:
	}
	if tunnel.lookupSession(1001) != nil {
		t.Fatal("constructor refusal retained the failed L2TP session")
	}
	if len(report.Warnings()) != 0 {
		t.Fatal("admission error invented a cleanup warning")
	}
}
