// Design: docs/architecture/l2tp/bng-5-pppoe.md -- refused admission lifetime

package pppoe

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
)

func TestPPPoERejectedExistingSessionKeepsAdmission(t *testing.T) {
	bus := newRecordBus()
	sub := newTestSubsystem(t, bus, 7)
	server := sub.servers[7]
	admitted := &Session{SID: 42, MAC: net.HardwareAddr{2, 0, 0, 0, 0, 1}, IfName: "eth0", Opened: fixtureOpened(t, "ppp3")}
	if err := server.sessions.Add(admitted); err != nil {
		t.Fatal(err)
	}
	frames, downs := 0, 0
	server.sendFrameFn = func([]byte) { frames++ }
	subevents.SessionDown.Subscribe(bus, func(*subevents.SessionDownPayload) { downs++ })
	sub.handlePPPEvent(ppp.EventSessionRejected{TunnelID: 7, SessionID: 42, ExistingSession: true, Reason: "duplicate key"})
	if server.sessions.Lookup(42) != admitted {
		t.Fatal("duplicate refusal removed the admitted session")
	}
	if frames != 0 {
		t.Fatal("duplicate refusal sent PADT for the admitted session")
	}
	if downs != 0 {
		t.Fatal("duplicate refusal published subscriber teardown")
	}
}

func TestPPPoERejectedNewSessionClosesAdmission(t *testing.T) {
	bus := newRecordBus()
	sub := newTestSubsystem(t, bus, 7)
	server := sub.servers[7]
	for _, sid := range []uint16{42, 43} {
		if err := server.sessions.Add(&Session{SID: sid, MAC: net.HardwareAddr{2, 0, 0, 0, 0, byte(sid)}, IfName: "eth0", Opened: fixtureOpened(t, "ppp3")}); err != nil {
			t.Fatal(err)
		}
	}
	var frames [][]byte
	server.sendFrameFn = func(frame []byte) { frames = append(frames, append([]byte(nil), frame...)) }
	var down *subevents.SessionDownPayload
	subevents.SessionDown.Subscribe(bus, func(event *subevents.SessionDownPayload) { down = event })
	sub.handlePPPEvent(ppp.EventSessionRejected{TunnelID: 7, SessionID: 42, Reason: "owner admitted under another key"})
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
