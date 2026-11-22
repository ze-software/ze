// Design: docs/architecture/l2tp/subscriber-session-model.md -- accounting counts the original PPP unit, never a name.
// Related: acct_unit_capture_test.go -- the capture fixture
// Related: acct_counter_lifetime_test.go -- the UDP accountant and counter assertions
package l2tpauthradius

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/radius"
)

// sourceAssignment is the assignment PPP publishes for the network it measured.
func sourceAssignment(username string, network l2tpevents.NetworkLifetime, baseline l2tpevents.TrafficSample) *l2tpevents.SessionIPAssignedPayload {
	return &l2tpevents.SessionIPAssignedPayload{
		SubscriberID: "l2tp-2030-49", TunnelID: 2030, SessionID: 49, Username: username,
		PeerAddr: "198.51.100.42", PppInterface: "ppp4242",
		NetworkLifetime: network, BaselineCounters: baseline,
	}
}

func waitSourceRecord(t *testing.T, capture *acctCapture, statusType uint8, username string) capturedAcct {
	t.Helper()
	return capture.waitForCounterRecord(t, username+" record", func(p capturedAcct) bool {
		return p.statusType == statusType && p.username == username
	})
}

func sourceDown() *l2tpevents.SessionDownPayload {
	return &l2tpevents.SessionDownPayload{TunnelID: 2030, SessionID: 49, Cause: l2tpevents.TerminateCauseUserRequest}
}

// TestAccountingCountsTheOriginalUnitNotTheName proves that the Interim and the
// Stop count the session's original PPP unit, through the capture bound to its
// network, while the interface name now answers for another subscriber.
//
// VALIDATES: the capture is bound to the session's SID, TID and network
// lifetime; the Interim reports the live unit reading minus the payload's
// baseline; the Stop reports the terminal measurement minus that baseline.
// PREVENTS: a renamed pppN, or a name reused by another subscriber, billing
// that subscriber's traffic to this session. The interface backend answers
// ppp4242 with far larger counters, so a name read fails every assertion.
func TestAccountingCountsTheOriginalUnitNotTheName(t *testing.T) {
	if !isolatedAcctCounters(t) {
		return
	}
	backend := installAcctCounterBackend(t)
	backend.set(iface.InterfaceStats{RxBytes: 900000, TxBytes: 800000, RxPackets: 9000, TxPackets: 8000}, nil)
	unit := newAcctUnitCapture(fixedAcctUnit(unitCounters(1500, 2500, 15, 25), nil))
	var mu sync.Mutex
	var selectors []l2tp.SessionSelector
	useAcctCaptures(t, func(selector l2tp.SessionSelector) (l2tp.CapturedSession, error) {
		mu.Lock()
		selectors = append(selectors, selector)
		mu.Unlock()
		return unit, nil
	})
	acct, capture := newCounterAccounting(t, 200*time.Millisecond)
	network := firstNetworkLifetime(t)

	acct.onSessionIPAssigned(sourceAssignment("renamed", network, measuredSample(1000, 2000, 10, 20)))
	start := waitSourceRecord(t, capture, radius.AcctStatusStart, "renamed")
	mu.Lock()
	bound := append([]l2tp.SessionSelector(nil), selectors...)
	mu.Unlock()
	if len(bound) != 1 || bound[0].SID != 49 || bound[0].TID != 2030 || bound[0].NetworkLifetime != network {
		t.Fatalf("captures bound = %+v, want one for SID 49, TID 2030 and the assigned network", bound)
	}
	interim := counterAttrs(iface.InterfaceStats{RxBytes: 500, TxBytes: 500, RxPackets: 5, TxPackets: 5})
	capture.waitForCounterRecord(t, "Interim from the original unit", func(p capturedAcct) bool {
		return p.statusType == radius.AcctStatusInterimUpdate && p.sessionID == start.sessionID && hasCounterAttrs(p, interim)
	})

	unit.measure(measuredSample(4000, 7000, 40, 70))
	acct.onSessionDown(sourceDown())
	stop := waitSourceRecord(t, capture, radius.AcctStatusStop, "renamed")
	assertCounterAttrs(t, stop, counterAttrs(iface.InterfaceStats{RxBytes: 3000, TxBytes: 5000, RxPackets: 30, TxPackets: 50}))
}

// TestAccountingStopAwaitsTheTerminalMeasurementAfterTheDown proves the order
// of a peer hang-up: L2TP publishes the Down before PPP has measured the
// network, and the Stop waits for that measurement and reports it.
//
// VALIDATES: after the Down the task waits on TerminalCounters and no Stop has
// left; once PPP measures, the Stop reports terminal minus baseline.
// PREVENTS: a Stop sent from the Down's payload, which on this path carries no
// measurement, losing the traffic counters of most sessions.
func TestAccountingStopAwaitsTheTerminalMeasurementAfterTheDown(t *testing.T) {
	unit := newAcctUnitCapture(fixedAcctUnit(unitCounters(0, 0, 0, 0), nil))
	useAcctCaptures(t, func(l2tp.SessionSelector) (l2tp.CapturedSession, error) { return unit, nil })
	acct, capture := newCounterAccounting(t, time.Hour)

	acct.onSessionIPAssigned(sourceAssignment("hangup", firstNetworkLifetime(t), measuredSample(100, 200, 1, 2)))
	waitSourceRecord(t, capture, radius.AcctStatusStart, "hangup")
	acct.onSessionDown(sourceDown())
	select {
	case <-unit.awaiting:
	case <-time.After(5 * time.Second):
		t.Fatal("the accounting task never awaited the terminal measurement")
	}
	capture.mu.Lock()
	for _, packet := range capture.packets {
		if packet.statusType == radius.AcctStatusStop {
			t.Errorf("a Stop left before the terminal measurement: %+v", packet)
		}
	}
	capture.mu.Unlock()

	unit.measure(measuredSample(1100, 2200, 11, 22))
	stop := waitSourceRecord(t, capture, radius.AcctStatusStop, "hangup")
	assertCounterAttrs(t, stop, counterAttrs(iface.InterfaceStats{RxBytes: 1000, TxBytes: 2000, RxPackets: 10, TxPackets: 20}))
}

// TestAccountingStopOmitsCountersWhenTheTerminalWaitEnds proves that a Stop
// whose terminal measurement never arrives within acctTerminalBound is still
// sent, with its cause and session time and without traffic counters.
//
// VALIDATES: the bound ends the wait; the Stop omits all six counter
// attributes and keeps Acct-Session-Time and Acct-Terminate-Cause.
// PREVENTS: a Stop held forever, or one reporting zero octets it never read.
func TestAccountingStopOmitsCountersWhenTheTerminalWaitEnds(t *testing.T) {
	saved := acctTerminalBound
	acctTerminalBound = 50 * time.Millisecond
	t.Cleanup(func() { acctTerminalBound = saved })
	unit := newAcctUnitCapture(fixedAcctUnit(unitCounters(0, 0, 0, 0), nil))
	useAcctCaptures(t, func(l2tp.SessionSelector) (l2tp.CapturedSession, error) { return unit, nil })
	acct, capture := newCounterAccounting(t, time.Hour)

	acct.onSessionIPAssigned(sourceAssignment("unmeasured", firstNetworkLifetime(t), measuredSample(100, 200, 1, 2)))
	waitSourceRecord(t, capture, radius.AcctStatusStart, "unmeasured")
	acct.onSessionDown(sourceDown())
	stop := waitSourceRecord(t, capture, radius.AcctStatusStop, "unmeasured")
	assertCounterAttrs(t, stop, nil)
	if value := stop.numericAttrs[radius.AttrAcctSessionTime]; len(value) != 4 {
		t.Fatalf("Stop lost its session time: %x", value)
	}
	if stop.terminateCause != uint32(l2tpevents.TerminateCauseUserRequest) {
		t.Fatalf("Stop cause = %d, want User Request", stop.terminateCause)
	}
}

// TestAccountingStopOmitsCountersOfAnUnavailableMeasurement proves that a
// terminal sample PPP could not take is reported as absent, not as zero.
//
// VALIDATES: an unavailable terminal sample leaves the Stop without counters.
// PREVENTS: an unavailable measurement read as a measured zero.
func TestAccountingStopOmitsCountersOfAnUnavailableMeasurement(t *testing.T) {
	unit := newAcctUnitCapture(fixedAcctUnit(unitCounters(0, 0, 0, 0), nil))
	useAcctCaptures(t, func(l2tp.SessionSelector) (l2tp.CapturedSession, error) { return unit, nil })
	acct, capture := newCounterAccounting(t, time.Hour)

	acct.onSessionIPAssigned(sourceAssignment("unreadable", firstNetworkLifetime(t), measuredSample(100, 200, 1, 2)))
	waitSourceRecord(t, capture, radius.AcctStatusStart, "unreadable")
	unit.measure(l2tpevents.UnavailableTrafficCounters(errors.New("original unit counter read failed")))
	acct.onSessionDown(sourceDown())
	assertCounterAttrs(t, waitSourceRecord(t, capture, radius.AcctStatusStop, "unreadable"), nil)
}

// TestAccountingAssignmentWithoutNetworkBindsNoCapture proves that an
// assignment naming no network lifetime seeks no capture, and that its records
// carry no traffic counters rather than a reading of some other network.
//
// VALIDATES: the capture seam is never called; the Start, an Interim and the
// Stop are still sent, and the Interim and the Stop omit all six counters.
// PREVENTS: an unqualified selector capturing the logical session across
// renegotiation, so one network's octets are billed against another's baseline.
func TestAccountingAssignmentWithoutNetworkBindsNoCapture(t *testing.T) {
	useAcctCaptures(t, func(l2tp.SessionSelector) (l2tp.CapturedSession, error) {
		t.Error("no capture may be sought for an assignment that named no network")
		return nil, errors.New("unexpected capture")
	})
	acct, capture := newCounterAccounting(t, 200*time.Millisecond)

	acct.onSessionIPAssigned(sourceAssignment("unqualified", l2tpevents.NetworkLifetime{}, measuredSample(100, 200, 1, 2)))
	start := waitSourceRecord(t, capture, radius.AcctStatusStart, "unqualified")
	interim := capture.waitForCounterRecord(t, "Interim of an unqualified assignment", func(p capturedAcct) bool {
		return p.statusType == radius.AcctStatusInterimUpdate && p.sessionID == start.sessionID
	})
	assertCounterAttrs(t, interim, nil)
	acct.onSessionDown(sourceDown())
	assertCounterAttrs(t, waitSourceRecord(t, capture, radius.AcctStatusStop, "unqualified"), nil)
}

// TestAccountingRetirementStopReadsTheLiveUnit proves that a Stop sent because
// accounting retired under a continuing network reports the live reading of
// the original unit, and does not wait for a terminal measurement that will
// never come.
//
// VALIDATES: the retirement Stop carries live reading minus baseline, with
// Administrative Reboot as its cause, and no terminal wait happened.
// PREVENTS: retirement Stops waiting out acctTerminalBound and then dropping
// the counters of every live session.
func TestAccountingRetirementStopReadsTheLiveUnit(t *testing.T) {
	unit := newAcctUnitCapture(fixedAcctUnit(unitCounters(700, 900, 7, 9), nil))
	useAcctCaptures(t, func(l2tp.SessionSelector) (l2tp.CapturedSession, error) { return unit, nil })
	acct, capture := newCounterAccounting(t, time.Hour)

	acct.onSessionIPAssigned(sourceAssignment("retired", firstNetworkLifetime(t), measuredSample(100, 200, 1, 2)))
	waitSourceRecord(t, capture, radius.AcctStatusStart, "retired")
	acct.Stop()
	stop := waitSourceRecord(t, capture, radius.AcctStatusStop, "retired")
	assertCounterAttrs(t, stop, counterAttrs(iface.InterfaceStats{RxBytes: 600, TxBytes: 700, RxPackets: 6, TxPackets: 7}))
	if stop.terminateCause != uint32(l2tpevents.TerminateCauseAdminReboot) {
		t.Fatalf("Stop cause = %d, want Admin Reboot", stop.terminateCause)
	}
	select {
	case <-unit.awaiting:
		t.Fatal("retirement waited for a terminal measurement of a continuing network")
	default:
	}
}
