// Design: docs/architecture/l2tp/subscriber-session-model.md -- accounting baseline before assignment acknowledgment
// Related: session_capture.go -- exact original counters; current_core_driver_fixture_test.go -- actual PPP admission

package l2tp

import (
	"net/netip"
	"testing"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestAccountingBaselineCapturePrecedesSessionUp reads through the production
// capture during the actual assignment publication. The real Driver has not
// received that assignment's acknowledgment and cannot have emitted SessionUp.
// Native I/O is the existing explicit pipe-backed model; its measured zero is
// compared with a direct original Control read, not claimed as kernel evidence.
// MUTATION: minting the network only in handleSessionUp makes this baseline
// unavailable even though the original admitted Control can read its owner.
func TestAccountingBaselineCapturePrecedesSessionUp(t *testing.T) {
	reactor := newReactorForSnapshot(t)
	bus := newTestBus()
	reactor.SetEventBus(bus)
	session := &L2TPSession{localSID: 42, state: L2TPSessionEstablished, pppInterface: "ppp42"}
	insertEstablishedTunnel(t, reactor, 100, 200, netip.MustParseAddrPort("192.0.2.1:1701"), session)
	root := &Subsystem{reactors: []*l2tpReactor{reactor}}
	selector := SessionSelector{SID: 42, TID: 100, Birth: session.Birth()}
	wire := newCurrentCoreAdmission(t, reactor, &currentCoreAdmissionConfig{
		ids: currentCoreSessionIDs{tunnelID: 100, sessionID: 42},
	})
	reactor.handlePPPEvent(wire.opened)
	before, err := root.CaptureSession(selector)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := before.Counters(); err == nil {
		t.Fatal("a pre-network admission supplied an accounting baseline")
	}
	wire.prepareNetwork(t, "baseline", ppp.IPResponseArgs{
		Accept: true, Family: ppp.AddressFamilyIPv4,
		Local: netip.MustParseAddr("192.0.2.1"), Peer: netip.MustParseAddr("192.0.2.42"),
	}, [8]byte{})
	assignment := wire.assign(t, ppp.AddressFamilyIPv4)
	original, err := assignment.Control.Counters()
	if err != nil {
		t.Fatalf("original admitted owner cannot supply the setup measurement: %v", err)
	}
	var observed int
	var baseline ppp.SessionCounters
	var baselineErr error
	var readBeforeAcknowledgment bool
	unsubscribe := l2tpevents.SessionIPAssigned.Subscribe(bus, func(event *l2tpevents.SessionIPAssignedPayload) {
		if event.Birth != session.Birth() {
			return
		}
		observed++
		select {
		case <-assignment.Acknowledged():
		default:
			readBeforeAcknowledgment = true
		}
		captured, captureErr := root.CaptureSession(selector)
		if captureErr != nil {
			baselineErr = captureErr
			return
		}
		baseline, baselineErr = captured.Counters()
	})
	t.Cleanup(unsubscribe)
	reactor.handlePPPEvent(assignment)
	wire.nextUp(t)
	if observed != 1 {
		t.Fatalf("actual assignment publications = %d, want one original publication", observed)
	}
	if !readBeforeAcknowledgment {
		t.Fatal("baseline callback ran after the original assignment was acknowledged")
	}
	if baselineErr != nil {
		t.Fatalf("original accounting baseline unavailable during assignment before SessionUp: %v", baselineErr)
	}
	if baseline != original {
		t.Fatalf("assignment baseline = %+v, want the same original raw measurement %+v", baseline, original)
	}
}
