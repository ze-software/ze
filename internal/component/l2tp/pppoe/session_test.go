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
	if got := st.lookupByMAC(mac); got != nil {
		t.Fatal("LookupByMAC returned non-nil after Remove")
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

func TestSessionTableLookupByMAC(t *testing.T) {
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

	got := st.lookupByMAC(mac1)
	if got == nil || got.SID != 1 {
		t.Fatalf("LookupByMAC(mac1) = %v, want SID 1", got)
	}
	got = st.lookupByMAC(mac2)
	if got == nil || got.SID != 2 {
		t.Fatalf("LookupByMAC(mac2) = %v, want SID 2", got)
	}

	unknown := net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	if got := st.lookupByMAC(unknown); got != nil {
		t.Fatalf("LookupByMAC(unknown) = %v, want nil", got)
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
