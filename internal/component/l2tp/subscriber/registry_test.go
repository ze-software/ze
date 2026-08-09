// Design: docs/architecture/l2tp/subscriber-session-model.md -- registry tests

package subscriber

import (
	"sync"
	"testing"
)

func TestRegistryAddRemoveCount(t *testing.T) {
	r := NewRegistry()

	s1 := &Session{ID: "pppoe-1", AccessType: AccessPPPoE}
	s2 := &Session{ID: "l2tp-1", AccessType: AccessL2TP}
	s3 := &Session{ID: "l2tp-2", AccessType: AccessL2TP}

	r.Add(s1)
	r.Add(s2)
	r.Add(s3)

	c := r.Count()
	if c.Total != 3 {
		t.Fatalf("total: got %d, want 3", c.Total)
	}
	if c.PPPoE != 1 {
		t.Fatalf("pppoe: got %d, want 1", c.PPPoE)
	}
	if c.L2TP != 2 {
		t.Fatalf("l2tp: got %d, want 2", c.L2TP)
	}

	all := r.All()
	if len(all) != 3 {
		t.Fatalf("all: got %d, want 3", len(all))
	}

	pppoe := r.ByAccessType(AccessPPPoE)
	if len(pppoe) != 1 {
		t.Fatalf("by-pppoe: got %d, want 1", len(pppoe))
	}

	got, ok := r.Get("pppoe-1")
	if !ok {
		t.Fatal("get pppoe-1: not found")
	}
	if got.AccessType != AccessPPPoE {
		t.Fatalf("get pppoe-1: access type = %s, want pppoe", got.AccessType)
	}

	r.Remove("l2tp-1")
	c = r.Count()
	if c.Total != 2 {
		t.Fatalf("after remove: total = %d, want 2", c.Total)
	}
	if c.L2TP != 1 {
		t.Fatalf("after remove: l2tp = %d, want 1", c.L2TP)
	}

	_, ok = r.Get("l2tp-1")
	if ok {
		t.Fatal("get l2tp-1 after remove: should not be found")
	}
}

func TestRegistryAddReplacesExisting(t *testing.T) {
	r := NewRegistry()
	r.Add(&Session{ID: "s1", Username: "alice"})
	r.Add(&Session{ID: "s1", Username: "bob"})

	got, ok := r.Get("s1")
	if !ok {
		t.Fatal("get s1: not found")
	}
	if got.Username != "bob" {
		t.Fatalf("username: got %q, want bob", got.Username)
	}
	c := r.Count()
	if c.Total != 1 {
		t.Fatalf("total after replace: got %d, want 1", c.Total)
	}
}

func TestRegistryLookupByAcctSessionID(t *testing.T) {
	r := NewRegistry()
	r.Add(&Session{ID: "s1", AcctSessionID: "acct-123"})
	r.Add(&Session{ID: "s2", AcctSessionID: "acct-456"})

	got, ok := r.LookupByAcctSessionID("acct-456")
	if !ok {
		t.Fatal("lookup acct-456: not found")
	}
	if got.ID != "s2" {
		t.Fatalf("lookup acct-456: got ID %q, want s2", got.ID)
	}

	_, ok = r.LookupByAcctSessionID("acct-999")
	if ok {
		t.Fatal("lookup acct-999: should not be found")
	}

	_, ok = r.LookupByAcctSessionID("")
	if ok {
		t.Fatal("lookup empty: should not be found")
	}
}

func TestRegistrySnapshotIsolation(t *testing.T) {
	r := NewRegistry()
	r.Add(&Session{ID: "s1", Username: "alice"})

	got, _ := r.Get("s1")
	got.Username = "mutated"

	orig, _ := r.Get("s1")
	if orig.Username != "alice" {
		t.Fatalf("snapshot mutation leaked: got %q, want alice", orig.Username)
	}
}

func TestRegistryConcurrent(t *testing.T) {
	r := NewRegistry()
	var wg sync.WaitGroup

	for i := range 100 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := "s" + string(rune('A'+n%26))
			r.Add(&Session{ID: id, AccessType: AccessPPPoE})
			r.Get(id)
			r.All()
			r.Count()
			r.ByAccessType(AccessPPPoE)
			r.Remove(id)
		}(i)
	}
	wg.Wait()
}
