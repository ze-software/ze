// Design: docs/architecture/l2tp/subscriber-session-model.md -- accounting counts the original PPP unit, never a name.
// Related: acct_health.go -- recordCountersOmitted, clearCountersOmitted, checkAccountingHealth
// Related: acct_counter_source_test.go -- the counter-source fixtures these tests drive
package l2tpauthradius

import (
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// counterFindings answers the accounting check's findings under the counters
// code that name one Acct-Session-Id, through the provider's runtime callback.
func counterFindings(t *testing.T, acctSessID string) []rpc.DoctorCheckDiagnostic {
	t.Helper()
	findings, err := checkRuntimeHealth(acctHealthCheckName)
	if err != nil {
		t.Fatal(err)
	}
	var named []rpc.DoctorCheckDiagnostic
	for _, finding := range findings {
		if finding.Code != codeRADIUSAccountingCounters {
			continue
		}
		if !strings.Contains(finding.Message, acctSessID) {
			continue
		}
		named = append(named, finding)
	}
	return named
}

// TestCounterOmissionIsADoctorFinding proves that a Stop sent without traffic
// counters reaches ze doctor under its own code, not only the log.
//
// VALIDATES: when PPP's final measurement does not arrive within
// acctTerminalBound, the runtime check l2tp-radius-accounting answers one
// finding under doctor-radius-accounting-counters that names the session's
// Acct-Session-Id, the cause, and the fix command; the check declares the code.
// PREVENTS: a billing record that lost its octets being visible only to an
// operator who reads the log, with nothing in ze doctor to find it by.
func TestCounterOmissionIsADoctorFinding(t *testing.T) {
	t.Cleanup(func() { report.ClearSource(Name) })
	saved := acctTerminalBound
	acctTerminalBound = 50 * time.Millisecond
	t.Cleanup(func() { acctTerminalBound = saved })
	unit := newAcctUnitCapture(fixedAcctUnit(unitCounters(0, 0, 0, 0), nil))
	useAcctCaptures(t, func(l2tp.SessionSelector) (l2tp.CapturedSession, error) { return unit, nil })
	acct, capture := newCounterAccounting(t, time.Hour)

	acct.onSessionIPAssigned(sourceAssignment("unmeasured-doctor", firstNetworkLifetime(t), measuredSample(100, 200, 1, 2)))
	start := waitSourceRecord(t, capture, radius.AcctStatusStart, "unmeasured-doctor")
	if found := counterFindings(t, start.sessionID); len(found) != 0 {
		t.Fatalf("a finding before any record omitted counters: %+v", found)
	}
	acct.onSessionDown(sourceDown())
	assertCounterAttrs(t, waitSourceRecord(t, capture, radius.AcctStatusStop, "unmeasured-doctor"), nil)

	// The finding is raised while the Stop is built, so it exists once the
	// Stop is on the wire.
	found := counterFindings(t, start.sessionID)
	if len(found) != 1 {
		t.Fatalf("findings for %s = %+v, want one under %s", start.sessionID, found, codeRADIUSAccountingCounters)
	}
	if !strings.Contains(found[0].Message, "final measurement") {
		t.Fatalf("finding %q does not name the cause", found[0].Message)
	}
	if found[0].Fix != acctCountersFix {
		t.Fatalf("finding fix = %q, want %q", found[0].Fix, acctCountersFix)
	}
	if !slices.Contains(acctHealthDeclaration().Codes, codeRADIUSAccountingCounters) {
		t.Fatalf("check %s does not declare %s", acctHealthCheckName, codeRADIUSAccountingCounters)
	}
}

// TestCounterOmissionClearsWhenALaterInterimCarriesCounters proves that an
// Interim omission is settled by the next record that carries counters.
//
// VALIDATES: an Interim whose live read of the PPP unit failed raises the
// session's finding; once reads succeed, the next Interim carries the
// cumulative counters and the finding is gone from the runtime check.
// PREVENTS: a stale finding that outlives its cause and sends an operator to
// investigate a session whose billing record is complete again, and an
// unbounded set of findings for sessions that recovered.
func TestCounterOmissionClearsWhenALaterInterimCarriesCounters(t *testing.T) {
	t.Cleanup(func() { report.ClearSource(Name) })
	var readable atomic.Bool
	unit := newAcctUnitCapture(func() (ppp.SessionCounters, error) {
		if readable.Load() {
			return unitCounters(1500, 2500, 15, 25), nil
		}
		return ppp.SessionCounters{}, errors.New("original unit counter read failed")
	})
	useAcctCaptures(t, func(l2tp.SessionSelector) (l2tp.CapturedSession, error) { return unit, nil })
	acct, capture := newCounterAccounting(t, 200*time.Millisecond)

	acct.onSessionIPAssigned(sourceAssignment("recovers", firstNetworkLifetime(t), measuredSample(100, 200, 1, 2)))
	start := waitSourceRecord(t, capture, radius.AcctStatusStart, "recovers")
	capture.waitForCounterRecord(t, "Interim without counters", func(p capturedAcct) bool {
		return p.statusType == radius.AcctStatusInterimUpdate && p.sessionID == start.sessionID && hasCounterAttrs(p, nil)
	})
	found := counterFindings(t, start.sessionID)
	if len(found) != 1 {
		t.Fatalf("findings after the failed read = %+v, want one under %s", found, codeRADIUSAccountingCounters)
	}
	if !strings.Contains(found[0].Message, "original unit counter read failed") {
		t.Fatalf("finding %q does not carry the read's cause", found[0].Message)
	}

	readable.Store(true)
	// The finding is cleared while the Interim is built, so it is gone once
	// that Interim is on the wire.
	recovered := counterAttrs(iface.InterfaceStats{RxBytes: 1400, TxBytes: 2300, RxPackets: 14, TxPackets: 23})
	capture.waitForCounterRecord(t, "Interim with counters", func(p capturedAcct) bool {
		return p.statusType == radius.AcctStatusInterimUpdate && p.sessionID == start.sessionID && hasCounterAttrs(p, recovered)
	})
	if found := counterFindings(t, start.sessionID); len(found) != 0 {
		t.Fatalf("finding survived an Interim that carried counters: %+v", found)
	}
}
