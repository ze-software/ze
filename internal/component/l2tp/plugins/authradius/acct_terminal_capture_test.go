// Design: docs/architecture/l2tp/subscriber-session-model.md -- the Stop reports the end of service it was handed.
// Related: acct_original_task_test.go -- the held RADIUS server these cases share
package l2tpauthradius

import (
	"encoding/binary"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/report"
)

// TestAccountingStopSurvivesDefectWhileCapturingEnd proves that a Ze defect
// met while the Down callback reads the final counters loses neither the
// session's Accounting-Stop nor the bound on shutdown.
//
// METHOD: the counter seam answers real values for the Start's baseline, then
// panics on the read the end of service makes. The Down must return to its
// publisher, the panic must be counted on the accounting boundary, the Stop
// must still reach the wire with the original Acct-Session-Id and cause and
// without counters, and Stop must return well inside its bound.
// PREVENTS: a panic in the end-of-service capture leaving the task waiting
// forever, so the Stop is never sent and retirement never returns.
func TestAccountingStopSurvivesDefectWhileCapturingEnd(t *testing.T) {
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
	// Installed before the accountant, so it is restored after the tasks join.
	var armed atomic.Bool
	saved := acctGetStats
	acctGetStats = func(string) (*iface.InterfaceStats, error) {
		if armed.Load() {
			panic("boom")
		}
		return &iface.InterfaceStats{RxBytes: 10, TxBytes: 20, RxPackets: 1, TxPackets: 2}, nil
	}
	t.Cleanup(func() { acctGetStats = saved })
	acct, capture := capturedAccountant(t)
	before, _ := authGuard.Panics(taskAccounting)

	acct.onSessionIPAssigned(&l2tpevents.SessionIPAssignedPayload{
		SubscriberID: "l2tp-5-6",
		TunnelID:     5, SessionID: 6, Username: "dora", PeerAddr: "10.0.0.6", PppInterface: "ppp6",
	})
	start := capture.waitN(t, 1)[0]
	armed.Store(true)
	acct.onSessionDown(&l2tpevents.SessionDownPayload{
		TunnelID: 5, SessionID: 6, Cause: l2tpevents.TerminateCauseUserRequest,
	})

	recorded := capture.waitN(t, 1)
	stop := recorded[len(recorded)-1]
	if stop.statusType != radius.AcctStatusStop || stop.sessionID != start.sessionID {
		t.Fatalf("after the defect: got %+v, want dora's Stop for %q", stop, start.sessionID)
	}
	if stop.terminateCause != uint32(l2tpevents.TerminateCauseUserRequest) {
		t.Fatalf("Stop cause = %d, want User Request", stop.terminateCause)
	}
	if _, reported := stop.numericAttrs[radius.AttrAcctInputOctets]; reported {
		t.Fatal("the Stop reports counters the defect left unread")
	}
	if _, reported := stop.numericAttrs[radius.AttrAcctSessionTime]; !reported {
		t.Fatal("the Stop omits the elapsed time recorded before the defect")
	}
	if count, _ := authGuard.Panics(taskAccounting); count != before+1 {
		t.Fatalf("Panics(%q) = %d, want %d", taskAccounting, count, before+1)
	}

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		acct.Stop()
	}()
	select {
	case <-stopped:
	case <-time.After(acctRecordBound):
		t.Fatal("shutdown did not return after the defect")
	}
}

// TestAccountingStopReportsTheEndOfService proves that a Stop waiting behind
// an unacknowledged Interim reports the instant service ended as its
// Event-Timestamp, not the later instant its task sent it.
//
// METHOD: the clock seam reads a value the test moves. The Interim is held at
// the server, the session ends at one instant, the clock moves on, and only
// then is the Interim answered and the Stop sent.
// PREVENTS: RFC 2869 Section 5.3, "to record the time that this event
// occurred on the NAS", read as the time the record left the NAS.
func TestAccountingStopReportsTheEndOfService(t *testing.T) {
	const ended, sent = int64(1756900100), int64(1756900200)
	var clock atomic.Int64
	clock.Store(1756900000)
	saved := acctNow
	acctNow = func() time.Time { return time.Unix(clock.Load(), 0) }
	t.Cleanup(func() { acctNow = saved })

	acct, server := newOrderedAccounting(t, radius.AcctStatusInterimUpdate, 30*time.Millisecond)
	assigned := orderedAccountingAssignment(t, "end-of-service")
	acct.onSessionIPAssigned(assigned)
	server.waitHeld(t)
	clock.Store(ended)
	done := orderedAccountingEnd(t, acct, orderedAccountingDown(assigned), server)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SessionDown waits for remote RADIUS while Interim is held")
	}
	clock.Store(sent)
	server.release()
	server.capture.waitForCounterRecord(t, "the queued Stop", func(packet capturedAcct) bool {
		return packet.statusType == radius.AcctStatusStop
	})
	acct.Stop()

	server.capture.mu.Lock()
	defer server.capture.mu.Unlock()
	for index, packet := range server.capture.packets {
		if packet.statusType != radius.AcctStatusStop {
			continue
		}
		stamp := server.eventTimes[index]
		if len(stamp) != 4 {
			t.Fatalf("Stop Event-Timestamp is %d octets, want 4", len(stamp))
		}
		if got := int64(binary.BigEndian.Uint32(stamp)); got != ended {
			t.Fatalf("Stop Event-Timestamp = %d, want the end of service %d (sent at %d)", got, ended, sent)
		}
		return
	}
	t.Fatal("no Stop on the wire")
}

// TestUnacknowledgedAccountingRecordIsADoctorFinding proves that a record no
// server acknowledged reaches ze doctor, not only the log.
//
// METHOD: a session starts against a server that never answers. The
// provider's runtime callback, asked for the accounting check, must answer one
// finding under the declared code that names the session's Acct-Session-Id and
// a fix command, and an unknown check name must be refused.
// PREVENTS: a billing record lost to an unreachable server being visible only
// to an operator who reads the log.
func TestUnacknowledgedAccountingRecordIsADoctorFinding(t *testing.T) {
	t.Cleanup(func() { report.ClearSource(Name) })
	acct := newDeadServerAcct(t)
	t.Cleanup(acct.Stop)
	acct.onSessionIPAssigned(&l2tpevents.SessionIPAssignedPayload{
		SubscriberID: "l2tp-8-9",
		TunnelID:     8, SessionID: 9, Username: "erin", PeerAddr: "10.0.0.9",
	})
	acctID := acct.currentSessionIDsByLabels(t, 8, 9)

	deadline := time.Now().Add(5 * time.Second)
	for {
		findings, err := checkRuntimeHealth(acctHealthCheckName)
		if err != nil {
			t.Fatal(err)
		}
		if len(findings) == 1 {
			finding := findings[0]
			if finding.Code != codeRADIUSAccountingUnacknowledged {
				t.Fatalf("finding code = %q, want %q", finding.Code, codeRADIUSAccountingUnacknowledged)
			}
			if !strings.Contains(finding.Message, acctID) || finding.Fix == "" {
				t.Fatalf("finding %+v does not name %q with a fix", finding, acctID)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no accounting finding after the unanswered Start: %+v", findings)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := checkRuntimeHealth("l2tp-radius-unknown"); err == nil {
		t.Fatal("an undeclared runtime check name was answered")
	}
}

// currentSessionIDsByLabels answers the issued Acct-Session-Id of the one live
// record holding these wire labels.
func (a *radiusAcct) currentSessionIDsByLabels(t *testing.T, tunnelID, sessionID uint16) string {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	sess := a.sessions[sessionKey{tunnelID: tunnelID, sessionID: sessionID}]
	if sess == nil {
		t.Fatalf("no accounting record for %d/%d", tunnelID, sessionID)
	}
	return sess.acctSessID
}
