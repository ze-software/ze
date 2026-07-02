// VALIDATES: AC-5 Shannon entropy -- 0 for empty/single-bucket distributions,
// log2(n) for n equal buckets, and correct handling of a skewed distribution.
// PREVENTS: log2(0) producing NaN, and counting non-positive weights.

package stats

import (
	"math"
	"testing"
)

func TestEntropyBounds(t *testing.T) {
	if h := Entropy(nil); h != 0 {
		t.Errorf("Entropy(nil) = %v, want 0", h)
	}
	if h := Entropy([]float64{5}); h != 0 {
		t.Errorf("Entropy(single) = %v, want 0", h)
	}
	if h := Entropy([]float64{0, 0, 0}); h != 0 {
		t.Errorf("Entropy(all-zero) = %v, want 0", h)
	}
	// two equal buckets -> 1 bit
	if h := Entropy([]float64{3, 3}); math.Abs(h-1) > 1e-9 {
		t.Errorf("Entropy(2 equal) = %v, want 1", h)
	}
	// four equal buckets -> 2 bits
	if h := Entropy([]float64{1, 1, 1, 1}); math.Abs(h-2) > 1e-9 {
		t.Errorf("Entropy(4 equal) = %v, want 2", h)
	}
	// negative weights ignored; two effective equal buckets -> 1 bit
	if h := Entropy([]float64{4, -1, 4}); math.Abs(h-1) > 1e-9 {
		t.Errorf("Entropy(with negative) = %v, want 1", h)
	}
	// skewed distribution is between 0 and log2(2)=1
	if h := Entropy([]float64{9, 1}); h <= 0 || h >= 1 {
		t.Errorf("Entropy(skewed) = %v, want in (0,1)", h)
	}
}
