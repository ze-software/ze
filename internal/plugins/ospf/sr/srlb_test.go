package sr

import "testing"

func TestSRLBAllocatorBounds(t *testing.T) {
	a := NewLabelAllocator([]LabelRange{{Base: 40000, Size: 4}})
	if a.Capacity() != 4 {
		t.Fatalf("Capacity = %d want 4", a.Capacity())
	}
	seen := map[uint32]bool{}
	for i := range 4 {
		l, ok := a.Allocate()
		if !ok {
			t.Fatalf("Allocate %d failed early", i)
		}
		if l < 40000 || l > 40003 {
			t.Fatalf("allocated label %d outside SRLB range 40000..40003", l)
		}
		if seen[l] {
			t.Fatalf("duplicate label %d", l)
		}
		seen[l] = true
	}
	if a.InUse() != 4 {
		t.Fatalf("InUse = %d want 4", a.InUse())
	}
}

func TestSRLBAllocatorExhaustion(t *testing.T) {
	a := NewLabelAllocator([]LabelRange{{Base: 40000, Size: 2}})
	if _, ok := a.Allocate(); !ok {
		t.Fatalf("first allocate failed")
	}
	if _, ok := a.Allocate(); !ok {
		t.Fatalf("second allocate failed")
	}
	if _, ok := a.Allocate(); ok {
		t.Fatalf("third allocate must fail (exhausted)")
	}
	// Freeing one label makes a slot available again.
	a.Free(40000)
	l, ok := a.Allocate()
	if !ok || l != 40000 {
		t.Fatalf("after free, Allocate = %d,%v want 40000,true", l, ok)
	}
}

func TestSRLBAllocatorMultiRange(t *testing.T) {
	a := NewLabelAllocator([]LabelRange{{Base: 40000, Size: 2}, {Base: 50000, Size: 2}})
	if a.Capacity() != 4 {
		t.Fatalf("Capacity = %d want 4", a.Capacity())
	}
	got := map[uint32]bool{}
	for i := range 4 {
		l, ok := a.Allocate()
		if !ok {
			t.Fatalf("allocate %d failed", i)
		}
		got[l] = true
	}
	for _, want := range []uint32{40000, 40001, 50000, 50001} {
		if !got[want] {
			t.Fatalf("label %d not allocated across ranges", want)
		}
	}
}

func TestSRLBReserveAndFree(t *testing.T) {
	a := NewLabelAllocator([]LabelRange{{Base: 40000, Size: 4}})
	if !a.Reserve(40002) {
		t.Fatalf("Reserve(40002) should succeed")
	}
	if a.Reserve(40002) {
		t.Fatalf("Reserve of already-used label must fail")
	}
	if a.Reserve(99999) {
		t.Fatalf("Reserve out of range must fail")
	}
	// A reserved label is never handed out by Allocate.
	for range 3 {
		l, ok := a.Allocate()
		if !ok || l == 40002 {
			t.Fatalf("Allocate returned reserved/failed: %d,%v", l, ok)
		}
	}
	a.Free(40002)
	if !a.Reserve(40002) {
		t.Fatalf("after Free, Reserve should succeed again")
	}
}
