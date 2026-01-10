package context

import "testing"

// TestAPIContextID verifies APIContextID is registered with ASN4=true.
//
// VALIDATES: API wire input uses modern 4-byte AS encoding.
// PREVENTS: Wire mode using legacy 2-byte AS encoding.
func TestAPIContextID(t *testing.T) {
	// APIContextID should be non-zero (valid registration)
	if APIContextID == 0 {
		t.Fatal("APIContextID not registered")
	}

	// Should be able to look up the context
	ctx := Registry.Get(APIContextID)
	if ctx == nil {
		t.Fatal("APIContextID lookup returned nil")
	}

	// ASN4 must be true for API input (modern encoding)
	if !ctx.ASN4 {
		t.Error("APIContextID: want ASN4=true, got false")
	}
}
