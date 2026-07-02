package sr

import "testing"

func TestSRLabelFromIndexSingleRange(t *testing.T) {
	g := NewSRGB([]LabelRange{{Base: 16000, Size: 8000}})
	cases := []struct {
		index uint32
		want  uint32
	}{
		{0, 16000},
		{1, 16001},
		{7999, 23999},
	}
	for _, c := range cases {
		got, ok := g.Label(c.index)
		if !ok || got != c.want {
			t.Fatalf("Label(%d) = %d,%v want %d,true", c.index, got, ok, c.want)
		}
	}
}

func TestSRLabelFromIndexMultiRange(t *testing.T) {
	// Two ranges concatenated in advertised order (RFC 8665 §3.2). Range 0 covers
	// indices 0..99 -> labels 16000..16099; range 1 covers indices 100..149 ->
	// labels 20000..20049.
	g := NewSRGB([]LabelRange{
		{Base: 16000, Size: 100},
		{Base: 20000, Size: 50},
	})
	cases := []struct {
		index uint32
		want  uint32
	}{
		{0, 16000},
		{99, 16099},  // last of range 0
		{100, 20000}, // first of range 1
		{149, 20049}, // last of range 1
	}
	for _, c := range cases {
		got, ok := g.Label(c.index)
		if !ok || got != c.want {
			t.Fatalf("Label(%d) = %d,%v want %d,true", c.index, got, ok, c.want)
		}
	}
	if g.TotalSize() != 150 {
		t.Fatalf("TotalSize = %d want 150", g.TotalSize())
	}
}

func TestSRLabelIndexOutOfRange(t *testing.T) {
	g := NewSRGB([]LabelRange{{Base: 16000, Size: 100}, {Base: 20000, Size: 50}})
	if _, ok := g.Label(150); ok {
		t.Fatalf("Label(150) should be rejected (total size 150)")
	}
	if _, ok := g.Label(1000); ok {
		t.Fatalf("Label(1000) should be rejected")
	}
}

func TestSRGBOrderStableAcrossReorigination(t *testing.T) {
	// Range order is significant: swapping the two ranges must change the mapping,
	// proving the SRGB preserves advertised order (RFC 8665 §3.2).
	a := NewSRGB([]LabelRange{{Base: 16000, Size: 100}, {Base: 20000, Size: 50}})
	b := NewSRGB([]LabelRange{{Base: 20000, Size: 50}, {Base: 16000, Size: 100}})
	la, _ := a.Label(0)
	lb, _ := b.Label(0)
	if la == lb {
		t.Fatalf("index 0 mapped identically for reordered ranges: %d", la)
	}
	if la != 16000 || lb != 20000 {
		t.Fatalf("order not preserved: a=%d b=%d", la, lb)
	}
	// Re-building from the same ordered slice yields the identical mapping.
	a2 := NewSRGB(a.Ranges())
	if got, _ := a2.Label(120); got != 20020 {
		t.Fatalf("rebuilt SRGB mapping drifted: %d want 20020", got)
	}
}

func TestSRGBEmptyAndZeroSizeRange(t *testing.T) {
	if !(SRGB{}).Empty() {
		t.Fatalf("zero SRGB must be empty")
	}
	// A zero-size range contributes nothing and is skipped.
	g := NewSRGB([]LabelRange{{Base: 16000, Size: 0}, {Base: 17000, Size: 5}})
	if g.TotalSize() != 5 {
		t.Fatalf("TotalSize = %d want 5", g.TotalSize())
	}
	if got, ok := g.Label(0); !ok || got != 17000 {
		t.Fatalf("Label(0) = %d,%v want 17000,true", got, ok)
	}
}
