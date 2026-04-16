package bufpool

import "testing"

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
