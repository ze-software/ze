//go:build integration && linux

// Design: docs/architecture/l2tp/bng-5-pppoe.md -- SID ownership through PADT publication
// Related: current_core_fixture_test.go -- actual Driver admissions and protocol peer
package pppoe

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"

	"golang.org/x/sys/unix"
)

// A genuine terminal event can arrive while the original PADT writer is still
// running. Drive that event through the real consumer and keep an independent
// admitted worker available to authenticate after the first publication ends.
func TestPADTPublicationRetainsAdmissionThroughCompetingTerminal(t *testing.T) {
	const ifIndex = 473
	local := [EthALen]byte{2, 0, 0, 0, 4, 73}
	peer := [EthALen]byte{2, 0, 0, 0, 4, 74}
	otherPeer := [EthALen]byte{2, 0, 0, 0, 4, 75}
	sub := newTestSubsystem(t, newRecordBus(), ifIndex)
	server := sub.servers[ifIndex]
	server.hwAddr = local
	server.sessions.setMaxSessions(2)
	sid, fd := newPADTSessionWithIdentity(t, server, peer, "padt473a")
	otherSID, otherFD := newPADTSessionWithIdentity(t, server, otherPeer, "padt473b")
	session := server.sessions.Lookup(sid)
	other := server.sessions.Lookup(otherSID)
	owner := fixtureCurrentStart(t, server, session, &fixtureCurrentStartConfig{method: ppp.AuthMethodNone})
	owner.authenticate(t, &fixtureCurrentAuthConfig{})
	owner.up(t)
	survivor := fixtureCurrentStart(t, server, other, &fixtureCurrentStartConfig{method: ppp.AuthMethodPAP})

	sent := 0
	server.sendFrameFn = func(frame []byte) {
		sent++
		packet, err := ParseDiscovery(frame)
		if err != nil {
			t.Fatal(err)
		}
		if packet.Code != CodePADT {
			t.Fatalf("sent discovery code %d, want PADT", packet.Code)
		}
		if packet.SID != sid {
			t.Fatalf("PADT named SID %d, want %d", packet.SID, sid)
		}
		if _, err := unix.Write(fd, []byte("PPP after PADT")); !errors.Is(err, unix.EBADF) {
			t.Fatalf("original transport remains writable during PADT: %v", err)
		}
		// handleSessionDown canceled this exact worker before publishing PADT.
		// Consume its genuine Down without manufacturing a terminal receipt.
		sub.handlePPPEvent(owner.down(t))
		if got := server.sessions.Lookup(sid); got != session {
			t.Fatal("terminal consumer released the original admission during PADT publication")
		}
		if _, err := server.sessions.AllocSID(); !errors.Is(err, ErrMaxSessions) {
			t.Fatalf("replacement admitted before original PADT returned: %v", err)
		}
		if _, err := unix.Write(otherFD, []byte("independent PPP")); err != nil {
			t.Fatalf("competing terminal closed the independent transport: %v", err)
		}
	}
	server.handleSessionDown(session)
	if sent != 1 {
		t.Fatalf("sent %d PADTs, want one", sent)
	}
	if server.sessions.Lookup(sid) != nil {
		t.Fatal("completed original publication retained its finished admission")
	}
	if server.sessions.Lookup(otherSID) != other {
		t.Fatal("original teardown replaced the independent admission")
	}
	replacement, err := server.sessions.AllocSID()
	if err != nil {
		t.Fatalf("publication did not release its original capacity: %v", err)
	}
	server.sessions.freeSID(replacement)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	status, err := owner.control.WaitRetired(ctx)
	if err != nil {
		t.Fatalf("original worker did not retire: %v", err)
	}
	if !status.Released {
		t.Fatal("original worker retired without releasing its owner")
	}
	if status.Err != nil {
		t.Fatalf("original native cleanup failed: %v", status.Err)
	}
	survivor.authenticate(t, &fixtureCurrentAuthConfig{username: "survivor"})
	up := survivor.up(t)
	if up.Control != survivor.control {
		t.Fatal("independent PAP completion changed its admitted worker")
	}
	if _, err := unix.Write(otherFD, []byte("PPP after original retirement")); err != nil {
		t.Fatalf("original retirement closed the independent transport: %v", err)
	}
}
