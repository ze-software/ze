package bgp

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// peerStopsReadingCheckerBranches drives both polarities of every predicate
// of the peer-stops-reading-bird checker on text shaped like what ze, BIRD
// and the kernel print, with no container. TestBespokeCheckerBranches runs it.
func peerStopsReadingCheckerBranches(t *testing.T) {
	t.Run("close window", peerStopsReadingCloseWindowBranches)
	t.Run("blocked writer", peerStopsReadingBlockedWriterBranches)
	t.Run("socket tables", peerStopsReadingSocketTableBranches)
	t.Run("route count", peerStopsReadingRouteCountBranches)
	t.Run("sessions", peerStopsReadingSessionBranches)
}

// peerStopsReadingCloseWindowBranches checks the window at both of its
// edges, and that a close at the write budget is refused: that is the close
// of a build whose hold timer waits for the blocked writer.
func peerStopsReadingCloseWindowBranches(t *testing.T) {
	if peerStopsReadingCloseEarliest != 5*time.Second {
		t.Fatalf("earliest close = %s, want 5s", peerStopsReadingCloseEarliest)
	}
	if peerStopsReadingCloseLatest != 20500*time.Millisecond {
		t.Fatalf("latest close = %s, want 20.5s", peerStopsReadingCloseLatest)
	}
	for _, accepted := range []time.Duration{
		peerStopsReadingCloseEarliest,
		peerStopsReadingHold + peerStopsReadingCloseBound,
		peerStopsReadingCloseLatest - time.Millisecond,
	} {
		if err := peerStopsReadingCloseVerdict(accepted); err != nil {
			t.Errorf("a close %s after the freeze was refused: %v", accepted, err)
		}
	}
	early := peerStopsReadingCloseVerdict(peerStopsReadingCloseEarliest - time.Millisecond)
	if early == nil {
		t.Fatal("a close before the hold timer could expire was accepted")
	}
	if !strings.Contains(early.Error(), "something other than the hold timer") {
		t.Errorf("the early refusal does not name its cause: %v", early)
	}
	for _, refused := range []time.Duration{peerStopsReadingCloseLatest, peerStopsReadingWriteBudget, peerStopsReadingWriteBudget + 2*time.Second} {
		late := peerStopsReadingCloseVerdict(refused)
		if late == nil {
			t.Fatalf("a close %s after the freeze was accepted", refused)
		}
		if !strings.Contains(late.Error(), "waited for the blocked write") {
			t.Errorf("the late refusal does not name its cause: %v", late)
		}
	}
}

// peerStopsReadingBlockedWriterBranches checks each way the frozen phase can
// fail to put a blocked writer in front of the hold timer.
func peerStopsReadingBlockedWriterBranches(t *testing.T) {
	const quietLog = "level=INFO msg=\"session closed\" peer=172.30.44.4 reason=\"hold timer expired\"\n"
	const notifiedLog = "level=WARN msg=\"notification sent\" peer=172.30.44.4 code=\"Hold Timer Expired\" subcode=0\n" + quietLog

	blocked := peerStopsReadingFrozen{zeLogs: quietLog, socketSeen: true, unsentOctetsMax: peerStopsReadingQueueFloor}
	if err := blocked.blockedWriterVerdict(); err != nil {
		t.Fatalf("a full send queue and no NOTIFICATION were refused: %v", err)
	}

	unread := peerStopsReadingFrozen{zeLogs: quietLog, socketErr: errors.New("exec refused")}
	if err := unread.blockedWriterVerdict(); err == nil || !strings.Contains(err.Error(), "exec refused") {
		t.Errorf("socket tables that were never read: verdict = %v, want the read failure", err)
	}
	absent := peerStopsReadingFrozen{zeLogs: quietLog}
	if err := absent.blockedWriterVerdict(); err == nil || !strings.Contains(err.Error(), "no BGP connection") {
		t.Errorf("socket tables with no BGP connection: verdict = %v, want a refusal", err)
	}
	idle := peerStopsReadingFrozen{zeLogs: quietLog, socketSeen: true, unsentOctetsMax: peerStopsReadingQueueFloor - 1}
	if err := idle.blockedWriterVerdict(); err == nil || !strings.Contains(err.Error(), "never filled the socket") {
		t.Errorf("a send queue below the floor: verdict = %v, want a refusal", err)
	}
	notified := peerStopsReadingFrozen{zeLogs: notifiedLog, socketSeen: true, unsentOctetsMax: 4 << 20}
	if err := notified.blockedWriterVerdict(); err == nil || !strings.Contains(err.Error(), "needs a free write path") {
		t.Errorf("a delivered Hold Timer Expired NOTIFICATION: verdict = %v, want a refusal", err)
	}
}

// peerStopsReadingSocketTableBranches reads the send queue out of rows shaped
// like /proc/net/tcp and /proc/net/tcp6, and keeps what is not a BGP
// connection out of the answer.
func peerStopsReadingSocketTableBranches(t *testing.T) {
	const header = "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n"
	const listener = "   0: 00000000:00B3 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 4101 1 0000000000000000 100 0 0 10 0\n"
	const cli = "   1: 022C1EAC:0016 0A2C1EAC:D431 01 00000FFF:00000000 00:00000000 00000000     0        0 4102 1 0000000000000000 20 4 30 10 -1\n"
	const dialed = "   2: 022C1EAC:C350 042C1EAC:00B3 01 0025A000:00000000 01:00000014 00000000     0        0 4103 2 0000000000000000 20 4 30 10 -1\n"
	const accepted = "   0: 0000000000000000FFFF0000022C1EAC:00B3 0000000000000000FFFF0000042C1EAC:8D2C 01 00001000:00000000 00:00000000 00000000     0        0 4104 1 0000000000000000 20 4 30 10 -1\n"

	if octets, found := bgpSocketUnsentOctets(header + listener + cli); found {
		t.Errorf("tables with a listener and another connection answered a BGP connection holding %d octets", octets)
	}
	if octets, found := bgpSocketUnsentOctets(""); found {
		t.Errorf("empty tables answered a BGP connection holding %d octets", octets)
	}
	octets, found := bgpSocketUnsentOctets(header + listener + cli + dialed)
	if !found || octets != 0x25A000 {
		t.Errorf("a connection ze dialed: %d octets, found %v; want %d, true", octets, found, 0x25A000)
	}
	octets, found = bgpSocketUnsentOctets(header + listener + header + accepted)
	if !found || octets != 0x1000 {
		t.Errorf("a connection ze accepted on the IPv6 table: %d octets, found %v; want %d, true", octets, found, 0x1000)
	}
	octets, found = bgpSocketUnsentOctets(accepted + dialed)
	if !found || octets != 0x25A000 {
		t.Errorf("two BGP connections: %d octets, found %v; want the larger queue %d", octets, found, 0x25A000)
	}

	var frozen peerStopsReadingFrozen
	frozen.recordSockets(header+dialed, nil)
	frozen.recordSockets(header+listener, nil)
	if !frozen.socketSeen || frozen.unsentOctetsMax != 0x25A000 {
		t.Errorf("a later read without the connection lost the queue: seen %v, %d octets", frozen.socketSeen, frozen.unsentOctetsMax)
	}
}

// peerStopsReadingRouteCountBranches reads the shown count out of birdc's
// answer and refuses an answer without the count line.
func peerStopsReadingRouteCountBranches(t *testing.T) {
	const counted = "BIRD 3.4.0 ready.\n131074 of 131076 routes for 131076 networks in table master4\n"
	held, err := birdShownRouteCount(counted)
	if err != nil || held != peerStopsReadingInitialRoutes+peerStopsReadingReloadRoutes {
		t.Errorf("shown count = %d, %v; want %d", held, err, peerStopsReadingInitialRoutes+peerStopsReadingReloadRoutes)
	}
	held, err = birdShownRouteCount("BIRD 3.4.0 ready.\n0 of 2 routes for 2 networks in table master4\n")
	if err != nil || held != 0 {
		t.Errorf("a count of zero = %d, %v; want 0 read from the line", held, err)
	}
	if _, err := birdShownRouteCount("BIRD 3.4.0 ready.\n"); err == nil {
		t.Error("an answer without a count line was read as a count")
	}
	if _, err := birdShownRouteCount("many of 4 routes for 4 networks in table master4\n"); err == nil {
		t.Error("a count line that opens with no number was read as a count")
	}
}

// peerStopsReadingSessionBranches counts sessions in both logs and judges
// them before the freeze and after the return.
func peerStopsReadingSessionBranches(t *testing.T) {
	const address = "172.30.44.4"
	const established = "level=INFO msg=\"session established\" peer=172.30.44.4 localAS=65001 peerAS=65002\n"
	const closed = "level=INFO msg=\"session closed\" peer=172.30.44.4 reason=\"hold timer expired\"\n"
	const other = "level=INFO msg=\"session closed\" peer=172.30.44.9 reason=\"teardown\"\n"
	const up = "bird: ze_peer: State changed to up\n"
	const start = "bird: ze_peer: State changed to start\n"

	before := countPeerStopsReadingSessions(established+other, start+up, address)
	if before != (peerStopsReadingSessions{zeEstablished: 1, zeClosed: 0, birdUp: 1}) {
		t.Fatalf("counts before the freeze = %+v", before)
	}
	if err := before.openVerdict(); err != nil {
		t.Errorf("one open session was refused: %v", err)
	}
	if err := (peerStopsReadingSessions{birdUp: 1}).openVerdict(); err == nil {
		t.Error("a ze log with no establishment was accepted")
	}
	if err := (peerStopsReadingSessions{zeEstablished: 1}).openVerdict(); err == nil {
		t.Error("a BIRD log with no state change was accepted")
	}
	if err := (peerStopsReadingSessions{zeEstablished: 2, zeClosed: 2, birdUp: 2}).openVerdict(); err == nil {
		t.Error("a ze log whose last session is closed was accepted")
	}

	after := countPeerStopsReadingSessions(established+other+closed+established, start+up+start+up, address)
	if err := before.returnVerdict(after); err != nil {
		t.Errorf("one session ended and one returned was refused: %v", err)
	}
	if err := before.returnVerdict(before); err == nil {
		t.Error("a session that never ended was accepted as returned")
	}
	flapped := countPeerStopsReadingSessions(established+closed+established+closed+established, start+up+start+up+start+up, address)
	if err := before.returnVerdict(flapped); err == nil {
		t.Error("a second session that ended again was accepted")
	}
	if err := before.returnVerdict(peerStopsReadingSessions{zeEstablished: 2, zeClosed: 1, birdUp: 1}); err == nil {
		t.Error("a return BIRD never logged was accepted")
	}
}
