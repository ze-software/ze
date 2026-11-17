// Design: docs/architecture/l2tp/bng-5-pppoe.md -- refused admission lifetime

package pppoe

import (
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/internal/core/report"
)

// TestPPPoERejectedExistingSessionKeepsAdmission requeues the same original
// StartSession/Attempt while its real auth wait owns the lease. The genuine
// duplicate rejection must preserve that active admission and its borrower.
func TestPPPoERejectedExistingSessionKeepsAdmission(t *testing.T) {
	fixture := newCoreRetentionFixture(t, 7)
	sub, server := fixture.sub, fixture.server
	admitted, transport, original := fixture.prepare(t, 42, true, nil)
	if server.sessions.takePPPLease(admitted) == nil {
		t.Fatal("original admission did not transfer its exact lease")
	}
	fixture.driver.SessionsIn() <- original
	opened, ok := fixture.next(t).(ppp.EventLCPUp)
	if !ok || opened.Control.Attempt() != original.Attempt {
		t.Fatal("actual original admission lost its attempt")
	}
	sub.handlePPPEvent(opened)
	var request ppp.EventAuthRequest
	select {
	case event := <-fixture.driver.AuthEventsOut():
		request, ok = event.(ppp.EventAuthRequest)
		if !ok || request.Control != opened.Control || !request.Active() {
			t.Fatal("original admission did not reach its exact active auth wait")
		}
	case <-time.After(time.Second):
		t.Fatal("original admission omitted its auth request")
	}
	frames, downs := 0, 0
	server.sendFrameFn = func([]byte) { frames++ }
	subevents.SessionDown.Subscribe(sub.bus, func(*subevents.SessionDownPayload) { downs++ })

	// Requeue the same transfer, not a freshly minted attempt or fabricated
	// event. Driver's comparable-owner guard must classify the original lease.
	fixture.driver.SessionsIn() <- original
	refusal, ok := fixture.next(t).(ppp.EventSessionRejected)
	if !ok || !refusal.ExistingSession || refusal.Attempt != original.Attempt || refusal.Attempt != admitted.Attempt {
		t.Fatal("actual duplicate rejection did not retain the original attempt")
	}
	status, err := refusal.Attempt.CleanupStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !status.Admitted || status.Finished || !status.LeaseAcquired || status.Released || status.Err != nil {
		t.Fatal("duplicate rejection changed authoritative original ownership", status)
	}
	sub.handlePPPEvent(refusal)
	if server.sessions.Lookup(42) != admitted {
		t.Fatal("duplicate refusal removed the admitted session")
	}
	if frames != 0 {
		t.Fatal("duplicate refusal sent PADT for the admitted session")
	}
	if downs != 0 {
		t.Fatal("duplicate refusal published subscriber teardown")
	}
	bound, found := original.Attempt.Control()
	if !found || bound != opened.Control || !bound.Active() || !request.Active() {
		t.Fatal("duplicate refusal retired or rebound the original active worker")
	}
	if transport.owner.closes.Load() != 0 {
		t.Fatal("duplicate rejection closed the original lease")
	}
	select {
	case <-transport.closed:
		t.Fatal("duplicate refusal closed the original physical transport")
	default:
	}
	select {
	case <-admitted.closeDone:
		t.Fatal("duplicate refusal completed the original admission")
	default:
	}
}

func TestPPPoERejectedNewSessionClosesAdmission(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	fixture := newCoreRetentionFixture(t, 7)
	sub, server := fixture.sub, fixture.server
	session, _ := fixture.queue(t, 42, false, nil)
	if err := server.sessions.Add(&Session{SID: 43, MAC: net.HardwareAddr{2, 0, 0, 0, 0, 43}, IfName: "eth0", Opened: fixtureOpened(t, "ppp3")}); err != nil {
		t.Fatal(err)
	}
	var frames [][]byte
	server.sendFrameFn = func(frame []byte) { frames = append(frames, append([]byte(nil), frame...)) }
	var down *subevents.SessionDownPayload
	subevents.SessionDown.Subscribe(sub.bus, func(event *subevents.SessionDownPayload) { down = event })
	refusal, ok := fixture.next(t).(ppp.EventSessionRejected)
	if !ok || refusal.ExistingSession || refusal.Attempt != session.Attempt {
		t.Fatal("actual rejected transfer did not preserve its incoming attempt")
	}
	status, err := refusal.Attempt.CleanupStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.Admitted || !status.Finished || !status.LeaseAcquired || !status.Released || status.Err != nil {
		t.Fatal("new refusal did not confirm cleanup of its own acquired lease", status)
	}
	sub.handlePPPEvent(refusal)
	if server.sessions.Lookup(42) != nil {
		t.Fatal("refused admission remains in the session table")
	}
	if server.sessions.Lookup(43) == nil {
		t.Fatal("refusal removed an unrelated admission")
	}
	if len(frames) != 1 {
		t.Fatalf("teardown frames = %d, want one PADT", len(frames))
	}
	packet, err := ParseDiscovery(frames[0])
	if err != nil {
		t.Fatal(err)
	}
	if packet.Code != CodePADT {
		t.Fatalf("teardown code = %d", packet.Code)
	}
	if packet.SID != 42 {
		t.Fatalf("PADT session = %d", packet.SID)
	}
	if down == nil {
		t.Fatal("refusal did not publish subscriber teardown")
	}
	if down.Session.AccessIfIndex != 7 {
		t.Fatalf("teardown access = %d", down.Session.AccessIfIndex)
	}
	if down.Session.PPPoESID != 42 {
		t.Fatalf("teardown session = %d", down.Session.PPPoESID)
	}
}
