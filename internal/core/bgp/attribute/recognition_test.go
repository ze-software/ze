// Related: attribute.go — RegisterName, RegisterNameOnly, Recognized
//
// VALIDATES: recognition means a parser exists, and naming an attribute for
// display does not claim one.
// PREVENTS: an attribute ze can only name being forwarded as recognized, which
// clears the Partial bit RFC 4271 Section 9 requires on an unrecognized
// optional transitive attribute.
package attribute

import "testing"

// TestRegisterNameOnlyDoesNotClaimRecognition is the discrimination case: the
// two registration functions must differ in exactly this respect, or the split
// buys nothing.
func TestRegisterNameOnlyDoesNotClaimRecognition(t *testing.T) {
	// Codes chosen from the unassigned range so the test cannot disturb a real
	// attribute's registration, which is process-wide and init-time.
	const named = AttributeCode(240)
	const implemented = AttributeCode(241)

	RegisterNameOnly(named, "NAMED_ONLY")
	RegisterName(implemented, "IMPLEMENTED")

	if named.Recognized() {
		t.Error("RegisterNameOnly claimed recognition. An attribute ze can only " +
			"name would then be forwarded with the Partial bit clear, telling the " +
			"next speaker ze understood bytes it never read (RFC 4271 Section 9)")
	}
	if !implemented.Recognized() {
		t.Error("RegisterName did not claim recognition; it is the function that " +
			"asserts a parser exists")
	}

	if got := named.String(); got != "NAMED_ONLY" {
		t.Errorf("display name = %q, want NAMED_ONLY: naming is the whole point of "+
			"the name-only registration", got)
	}
	if got := implemented.String(); got != "IMPLEMENTED" {
		t.Errorf("display name = %q, want IMPLEMENTED", got)
	}
}
