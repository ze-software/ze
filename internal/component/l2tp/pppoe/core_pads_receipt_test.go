// Design: docs/architecture/l2tp/bng-5-pppoe.md -- monotonic PADS publication receipt
package pppoe

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/core/report"
)

// TestCorePPPoEPADSPublicationSurvivesTeardown distinguishes the authoritative
// table receipt from current session phase and from the void sender hook.
func TestCorePPPoEPADSPublicationSurvivesTeardown(t *testing.T) {
	for _, sender := range []string{"returned observer", "panicked observer", "kernel error"} {
		t.Run(sender, func(t *testing.T) {
			fixture := newCoreRetentionFixture(t, 7)
			session := &Session{SID: 89, SubscriberID: pppoeSessionID(7, 89), AccessIfIndex: 7, State: StateDiscovery}
			if err := fixture.server.sessions.Add(session); err != nil {
				t.Fatal(err)
			}
			fixture.server.admitting = admission{session: session}
			publication, current := fixture.server.sessions.PADSPublication(session)
			if !current || publication != PADSNotAttempted {
				t.Fatal("Add did not explicitly initialize fresh publication", publication, current)
			}
			observed := false
			fixture.server.sendFrameFn = func([]byte) {
				observed = true
				state, match := fixture.server.sessions.PADSPublication(session)
				if !match || state != PADSAttempted {
					t.Error("PADS may escape before its attempted receipt exists")
				}
				if sender == "panicked observer" {
					panic("uncertain PADS send")
				}
			}
			if sender == "kernel error" {
				fixture.server.sendFrameFn = nil
			}
			var frameBuffer [EthMaxLen]byte
			request := Packet{SrcMAC: [EthALen]byte{2, 0, 0, 0, 0, 89}, DstMAC: fixture.server.hwAddr}
			frame := BuildPADS(frameBuffer[:], fixture.server.hwAddr, &request, fixture.server.acName, session.SID)
			if frame == nil {
				t.Fatal("PADS fixture did not encode")
			}
			func() {
				if sender == "panicked observer" {
					defer func() {
						if recover() == nil {
							t.Error("test sender did not panic")
						}
					}()
				}
				fixture.server.sendPADS(session.SID, frame)
			}()
			publication, current = fixture.server.sessions.PADSPublication(session)
			if !current || publication != PADSAttempted {
				t.Fatal("uncertain/void send was promoted to kernel submission", publication, current)
			}
			if sender != "kernel error" && !observed {
				t.Fatal("actual sender callback was not reached")
			}
			if sender == "returned observer" && session.State != StateSession {
				t.Fatal("completed observer did not reach the original established phase")
			}
			fixture.server.sessions.markTeardown(session)
			captured := false
			for _, row := range fixture.server.sessions.admissions() {
				if row.session == session {
					captured = true
					if row.publication != PADSAttempted {
						t.Fatal("teardown erased captured PADS history")
					}
				}
			}
			if !captured {
				t.Fatal("teardown omitted the original admission snapshot")
			}
			snapshot, found := fixture.server.sessions.LookupSnapshot(89)
			if !found || snapshot.PADSPublication != PADSAttempted || snapshot.State != StateTeardown {
				t.Fatal("former publication was derived from current teardown phase", snapshot, found)
			}
			fixture.server.closeAdmitted(session)
			if state, match := fixture.server.sessions.PADSPublication(session); match || state != PADSUnspecified {
				t.Fatal("departed pointer was reported as confirmed absent/current", state, match)
			}
		})
	}
}

func TestCorePPPoEProducerCleanupSnapshotRequiresCurrentOwner(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	fixture := newCoreRetentionFixture(t, 7)
	cause := errors.New("producer snapshot cleanup failure")
	session, transport, _ := fixture.prepare(t, 90, true, cause)
	fixture.server.closeAdmitted(session)
	status, current := fixture.server.sessions.CleanupStatus(session)
	if !current || status.InProgress || !errors.Is(status.Err, cause) {
		t.Fatal("current cleanup snapshot lost its actual failure", status, current)
	}
	transport.owner.clearFailure()
	fixture.server.closeAdmitted(session)
	if _, current := fixture.server.sessions.CleanupStatus(session); current {
		t.Fatal("departed producer was reported as current")
	}
	select {
	case <-session.closeDone:
	default:
		t.Fatal("recovered original cleanup did not finish")
	}
}
