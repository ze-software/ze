package bufpool

import (
	"runtime/debug"
	"testing"
)

func TestGetReturnsFullCapacity(t *testing.T) {
	p := New(4, 128, "test")
	b := p.Get()
	if cap(b) != 128 {
		t.Fatalf("Get cap = %d, want 128", cap(b))
	}
	if len(b) != 128 {
		t.Fatalf("Get len = %d, want 128", len(b))
	}
}

func TestPutRestoresFullCapacity(t *testing.T) {
	p := New(1, 64, "test")
	b := p.Get()
	p.Put(b[:10])
	b2 := p.Get()
	if len(b2) != 64 || cap(b2) != 64 {
		t.Fatalf("post-Put Get len/cap = %d/%d, want 64/64", len(b2), cap(b2))
	}
}

func TestPutDropsWrongSize(t *testing.T) {
	p := New(0, 64, "test")
	bad := make([]byte, 32)
	p.Put(bad)
	b := p.Get()
	if cap(b) != 64 {
		t.Fatalf("Get after bad Put cap = %d, want 64 (bad slice must not poison pool)", cap(b))
	}
}

func TestSeedsProducePooledBuffers(t *testing.T) {
	p := New(3, 32, "test")
	// First 3 Gets should all come from the seed without invoking New.
	for i := range 3 {
		b := p.Get()
		if cap(b) != 32 {
			t.Fatalf("seeded Get[%d] cap = %d, want 32", i, cap(b))
		}
	}
}

func TestSizeAndName(t *testing.T) {
	p := New(1, 256, "mypool")
	if p.Size() != 256 {
		t.Fatalf("Size = %d, want 256", p.Size())
	}
	if p.Name() != "mypool" {
		t.Fatalf("Name = %q, want %q", p.Name(), "mypool")
	}
}

func TestPutDropsLargerBuffer(t *testing.T) {
	p := New(0, 64, "test")
	big := make([]byte, 128)
	p.Put(big)
	b := p.Get()
	if cap(b) != 64 {
		t.Fatalf("Get after oversized Put cap = %d, want 64", cap(b))
	}
}

// VALIDATES: Put hands the buffer back to the pool for reuse instead of
// dropping it on the floor.
// PREVENTS: Put silently becoming a no-op (or storing a copy), which would turn
// every Get into a fresh allocation -- the pool still "works", just without
// pooling anything, so no other test in this file would notice.
//
// A single Put/Get pair used to assert this and flaked in full `ze-precommit-verify` runs
// (plan/known-failures/syncpool-capacity-identity-flakes.md). That assertion was
// not about our code: sync.Pool explicitly does NOT promise Get returns what was
// just Put. It is emptied at every GC, and an item parked in one P's private
// slot cannot be stolen by another P, so under the memory pressure and
// rescheduling of the parallel suite the marker legitimately went missing.
//
// GC off plus retries removes that environmental variable WITHOUT weakening what
// is being checked: a fresh make([]byte) is always zeroed and every marker here
// is non-zero, so if Put stopped pooling, zero iterations would see their
// marker and this still fails.
func TestGetReturnsSameBufferAfterPut(t *testing.T) {
	defer debug.SetGCPercent(debug.SetGCPercent(-1))

	p := New(0, 64, "test")
	const attempts = 100
	reused := 0
	for i := range attempts {
		// Distinct non-zero marker per iteration: a buffer still carrying an
		// EARLIER iteration's marker must not read as this iteration's reuse.
		// attempts < 255 keeps them unique.
		mark := byte(i + 1)
		b := p.Get()
		b[0] = mark
		p.Put(b)
		b2 := p.Get()
		if b2[0] == mark {
			reused++
		}
		p.Put(b2)
	}
	if reused == 0 {
		t.Fatalf("no Get in %d attempts returned a buffer Put had pooled; Put is not returning buffers to the pool", attempts)
	}
}
