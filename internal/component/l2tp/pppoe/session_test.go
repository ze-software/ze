package pppoe

import (
	"errors"
	"net"
	"testing"
	"time"
)

func TestSessionIDAlloc(t *testing.T) {
	st := newSessionTable("eth0", 0)
	sid, err := st.AllocSID()
	if err != nil {
		t.Fatalf("AllocSID: %v", err)
	}
	if sid == 0 {
		t.Fatal("AllocSID returned reserved SID 0")
	}
	if sid > maxSID {
		t.Fatalf("AllocSID returned SID %d, want <= %d", sid, maxSID)
	}
}

func TestSessionIDAllocSequential(t *testing.T) {
	st := newSessionTable("eth0", 0)
	seen := make(map[uint16]bool)
	const n = 1000
	for range n {
		sid, err := st.AllocSID()
		if err != nil {
			t.Fatalf("AllocSID: %v", err)
		}
		if seen[sid] {
			t.Fatalf("duplicate SID %d", sid)
		}
		seen[sid] = true
	}
	if len(seen) != n {
		t.Fatalf("got %d unique SIDs, want %d", len(seen), n)
	}
}

func TestSessionIDExhausted(t *testing.T) {
	st := newSessionTable("eth0", 0)
	for range maxSID {
		sid, err := st.AllocSID()
		if err != nil {
			t.Fatalf("AllocSID at count %d: %v", len(st.sessions), err)
		}
		st.sessions[sid] = &Session{SID: sid}
	}
	_, err := st.AllocSID()
	if err == nil {
		t.Fatal("expected error after exhausting all SIDs")
	}
}

func TestSessionIDFree(t *testing.T) {
	st := newSessionTable("eth0", 0)
	sid, err := st.AllocSID()
	if err != nil {
		t.Fatalf("AllocSID: %v", err)
	}
	st.freeSID(sid)

	// After freeing, the SID should be allocatable again.
	// Allocate all remaining and verify the freed SID appears.
	found := false
	for range maxSID {
		s, err := st.AllocSID()
		if err != nil {
			break
		}
		if s == sid {
			found = true
			break
		}
		st.sessions[s] = &Session{SID: s}
	}
	if !found {
		t.Fatalf("freed SID %d was not re-allocated", sid)
	}
}

func TestSessionTableAdd(t *testing.T) {
	st := newSessionTable("eth0", 100)
	mac := net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x01}
	s := &Session{
		SID:       42,
		MAC:       mac,
		IfName:    "eth0",
		State:     StateDiscovery,
		CreatedAt: time.Now(),
	}
	if err := st.Add(s); err != nil {
		t.Fatalf("Add: %v", err)
	}
	got := st.Lookup(42)
	if got == nil {
		t.Fatal("Lookup returned nil for added session")
	}
	if got.SID != 42 {
		t.Fatalf("Lookup SID = %d, want 42", got.SID)
	}

	if err := st.Add(s); !errors.Is(err, ErrSessionExists) {
		t.Fatalf("duplicate Add: got %v, want ErrSessionExists", err)
	}
}

func TestSessionTableRemove(t *testing.T) {
	st := newSessionTable("eth0", 100)
	mac := net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x02}
	s := &Session{SID: 10, MAC: mac, IfName: "eth0"}
	if err := st.Add(s); err != nil {
		t.Fatalf("Add: %v", err)
	}

	st.Remove(10)

	if got := st.Lookup(10); got != nil {
		t.Fatal("Lookup returned non-nil after Remove")
	}
	if got := st.sessionsByMAC(mac); got != nil {
		t.Fatal("sessionsByMAC returned non-nil after Remove")
	}
	if st.Count() != 0 {
		t.Fatalf("Count = %d, want 0", st.Count())
	}
}

func TestSessionTableMaxLimit(t *testing.T) {
	const limit = 5
	st := newSessionTable("eth0", limit)

	for i := range limit {
		sid := uint16(i + 1)
		// Manually mark SID as allocated in bitmap so AllocSID
		// limit check fires based on session count.
		st.bitmap[int(sid)/64] &^= 1 << (uint(sid) % 64)
		s := &Session{SID: sid, IfName: "eth0"}
		if err := st.Add(s); err != nil {
			t.Fatalf("Add session %d: %v", sid, err)
		}
	}

	_, err := st.AllocSID()
	if !errors.Is(err, ErrMaxSessions) {
		t.Fatalf("AllocSID with %d sessions: got %v, want ErrMaxSessions", limit, err)
	}
}

// TestSessionTableSessionsByMAC pins sessionsByMAC's single-session case: one
// session per MAC is still returned correctly once the index is a set.
func TestSessionTableSessionsByMAC(t *testing.T) {
	st := newSessionTable("eth0", 100)
	mac1 := net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
	mac2 := net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, 0x66}

	s1 := &Session{SID: 1, MAC: mac1, IfName: "eth0"}
	s2 := &Session{SID: 2, MAC: mac2, IfName: "eth0"}

	if err := st.Add(s1); err != nil {
		t.Fatalf("Add s1: %v", err)
	}
	if err := st.Add(s2); err != nil {
		t.Fatalf("Add s2: %v", err)
	}

	got := st.sessionsByMAC(mac1)
	if len(got) != 1 || got[0].SID != 1 {
		t.Fatalf("sessionsByMAC(mac1) = %v, want exactly [SID 1]", got)
	}
	got = st.sessionsByMAC(mac2)
	if len(got) != 1 || got[0].SID != 2 {
		t.Fatalf("sessionsByMAC(mac2) = %v, want exactly [SID 2]", got)
	}

	unknown := net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	if got := st.sessionsByMAC(unknown); got != nil {
		t.Fatalf("sessionsByMAC(unknown) = %v, want nil", got)
	}
}

// TestSessionTableIndexesEverySessionOfOneMAC -- AC-3. A MAC opens two
// sessions while the cap is above two: sessionsByMAC must resolve both, and
// the second Add must not orphan the first.
func TestSessionTableIndexesEverySessionOfOneMAC(t *testing.T) {
	st := newSessionTable("eth0", 100)
	mac := net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x10}

	sid1, err := st.AllocSID()
	if err != nil {
		t.Fatalf("AllocSID sid1: %v", err)
	}
	if err := st.Add(&Session{SID: sid1, MAC: mac, IfName: "eth0", State: StateDiscovery}); err != nil {
		t.Fatalf("Add sid1: %v", err)
	}

	sid2, err := st.AllocSID()
	if err != nil {
		t.Fatalf("AllocSID sid2: %v", err)
	}
	if err := st.Add(&Session{SID: sid2, MAC: mac, IfName: "eth0", State: StateSession}); err != nil {
		t.Fatalf("Add sid2: %v", err)
	}

	got := st.sessionsByMAC(mac)
	if len(got) != 2 {
		t.Fatalf("sessionsByMAC returned %d session(s), want 2: a second session for one MAC must not replace the first", len(got))
	}
	sawSID1, sawSID2 := false, false
	for _, s := range got {
		switch s.SID {
		case sid1:
			sawSID1 = true
		case sid2:
			sawSID2 = true
		}
	}
	if !sawSID1 || !sawSID2 {
		t.Fatalf("sessionsByMAC = %v, want both SID %d and %d", got, sid1, sid2)
	}

	// Both sessions must also stay reachable by SID: the MAC index change
	// must not touch the SID-keyed map.
	if st.Lookup(sid1) == nil || st.Lookup(sid2) == nil {
		t.Fatal("both sessions must remain reachable by SID after a second session joins the MAC")
	}
}

// TestSessionTableRemoveKeepsSiblingSessions -- AC-4. Removing one session of
// a multi-session MAC leaves the others reachable, and the MAC entry
// disappears once the last one goes.
func TestSessionTableRemoveKeepsSiblingSessions(t *testing.T) {
	st := newSessionTable("eth0", 100)
	mac := net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x11}

	sid1, err := st.AllocSID()
	if err != nil {
		t.Fatalf("AllocSID sid1: %v", err)
	}
	if err := st.Add(&Session{SID: sid1, MAC: mac, IfName: "eth0", PppoxFD: -1, State: StateSession}); err != nil {
		t.Fatalf("Add sid1: %v", err)
	}
	sid2, err := st.AllocSID()
	if err != nil {
		t.Fatalf("AllocSID sid2: %v", err)
	}
	if err := st.Add(&Session{SID: sid2, MAC: mac, IfName: "eth0", PppoxFD: -1, State: StateSession}); err != nil {
		t.Fatalf("Add sid2: %v", err)
	}

	st.Remove(sid1)

	got := st.sessionsByMAC(mac)
	if len(got) != 1 || got[0].SID != sid2 {
		t.Fatalf("sessionsByMAC after removing sid1 = %v, want exactly [SID %d]", got, sid2)
	}
	if st.Lookup(sid1) != nil {
		t.Error("sid1 must be gone from the SID map after Remove")
	}
	if st.Lookup(sid2) == nil {
		t.Error("sid2 must still be reachable by SID: removing one session of a MAC must not remove its sibling")
	}

	st.Remove(sid2)
	if got := st.sessionsByMAC(mac); got != nil {
		t.Fatalf("sessionsByMAC after removing the last session = %v, want nil", got)
	}
	var key [6]byte
	copy(key[:], mac)
	if _, exists := st.byMAC[key]; exists {
		t.Error("the MAC entry must disappear once its last session is removed")
	}
}

// TestSessionTableChurnLeavesNoIndexEntries -- R-3. Add and remove 1000
// sessions across 10 MACs: both the SID map and the MAC index must return to
// size zero. This is the leak guard: a Remove that forgets to drop an empty
// per-MAC set, or an Add that overwrites instead of joining, leaves byMAC
// non-empty here even though every session was removed.
func TestSessionTableChurnLeavesNoIndexEntries(t *testing.T) {
	st := newSessionTable("eth0", 2000)

	var macs [10]net.HardwareAddr
	for i := range macs {
		macs[i] = net.HardwareAddr{0x00, 0x11, 0x22, 0x33, 0x44, byte(i)}
	}

	for i := range 1000 {
		mac := macs[i%len(macs)]
		sid, err := st.AllocSID()
		if err != nil {
			t.Fatalf("AllocSID at iteration %d: %v", i, err)
		}
		if err := st.Add(&Session{SID: sid, MAC: mac, IfName: "eth0", State: StateSession}); err != nil {
			t.Fatalf("Add at iteration %d: %v", i, err)
		}
		st.Remove(sid)
	}

	if got := len(st.sessions); got != 0 {
		t.Errorf("len(sessions) = %d, want 0 after 1000 add/remove cycles across %d MACs", got, len(macs))
	}
	if got := len(st.byMAC); got != 0 {
		t.Errorf("len(byMAC) = %d, want 0 after 1000 add/remove cycles across %d MACs", got, len(macs))
	}
}

// TestMarkSessionDoesNotResurrectATeardownSession -- R-2's guard from the
// other side. handlePADR sets a session live through markSession after the
// kernel setup it queued has succeeded, and handleSessionDown marks the same
// session StateTeardown from the PPP driver's event-consumer goroutine. When
// the teardown lands first, the later markSession must leave it torn down:
// matchLiveCookie excludes a StateTeardown session, so a markSession that
// overwrote the state would hand a dying session back to the next PADR
// replaying its cookie, which is exactly the outcome R-2 exists to prevent.
func TestMarkSessionDoesNotResurrectATeardownSession(t *testing.T) {
	st := newSessionTable("eth0", 100)
	mac := net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x12}
	cookie := []byte("cookie-bytes-20-long")

	sid, err := st.AllocSID()
	if err != nil {
		t.Fatalf("AllocSID: %v", err)
	}
	if err := st.Add(&Session{
		SID: sid, MAC: mac, IfName: "eth0", PppoxFD: -1,
		State: StateDiscovery, Cookie: cookie,
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// A session still in discovery is what markSession is for.
	st.markSession(sid)
	if got := st.Lookup(sid).State; got != StateSession {
		t.Fatalf("State after markSession on a discovery session = %v, want StateSession", got)
	}

	// The event consumer wins the race and tears the session down.
	if got := st.markTeardown(sid); got == nil {
		t.Fatalf("markTeardown(%d): session not found", sid)
	}

	// handlePADR reaches its own state transition afterwards.
	st.markSession(sid)

	if got := st.Lookup(sid).State; got != StateTeardown {
		t.Errorf("State after a late markSession = %v, want StateTeardown: a torn-down session must not be made live again", got)
	}
	if _, ok := st.matchLiveCookie(mac, cookie); ok {
		t.Error("matchLiveCookie handed back a session marked for teardown: R-2's guard failed open")
	}
}

func TestFreeSIDZeroIsNoop(t *testing.T) {
	st := newSessionTable("eth0", 0)
	st.freeSID(0)
	// SID 0 must remain reserved (bit 0 of word 0 stays clear).
	if st.bitmap[0]&1 != 0 {
		t.Fatal("FreeSID(0) made SID 0 allocatable")
	}
}
