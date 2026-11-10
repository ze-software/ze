package probe

import "testing"

// TestDouble proves Double doubles its argument. It is the unit the red
// walk's .ci breaks, so a break of Double MUST turn it red.
func TestDouble(t *testing.T) {
	if got := Double(21); got != 42 {
		t.Fatalf("Double(21) = %d, want 42", got)
	}
}
