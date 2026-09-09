// Design: docs/architecture/wire/attributes.md -- AS notation in an extended community
package attribute

import "testing"

// TestExtCommunityAdminReadsEveryNotation proves the administrator of an
// extended community takes any of the three RFC 5396 spellings. The bytes do
// not depend on which one the operator wrote.
//
// RFC 5668 Section 2 gives the 4-octet AS specific form a "4-octet Autonomous
// System number" in its Global Administrator. A dotted spelling fits it, and no
// RFC pins the text form to decimal.
//
// VALIDATES: ParseSingleExtCommunity reads asplain, asdot and asdot+ (AC-1).
// PREVENTS: an operator reading 1.10 on a show output and having
// `target:1.10:5` refused, or worse read as an address.
func TestExtCommunityAdminReadsEveryNotation(t *testing.T) {
	want, err := ParseSingleExtCommunity("target:65546:5")
	if err != nil {
		t.Fatalf("target:65546:5: %v", err)
	}
	if want[0] != 0x02 {
		t.Fatalf("type = %#x, want 0x02 for a four-byte AS number", want[0])
	}

	for _, spelling := range []string{"target:1.10:5", "origin:1.10:5"} {
		got, err := ParseSingleExtCommunity(spelling)
		if err != nil {
			t.Fatalf("%s: %v", spelling, err)
		}
		if got[0] != 0x02 {
			t.Errorf("%s type = %#x, want 0x02", spelling, got[0])
		}
	}
	dotted, err := ParseSingleExtCommunity("target:1.10:5")
	if err != nil {
		t.Fatalf("target:1.10:5: %v", err)
	}
	if dotted != want {
		t.Errorf("target:1.10:5 = %v, want %v", dotted, want)
	}

	// The IPv4 administrator is decided by netip before the AS branch, so it
	// keeps type 0x01 and a dotted AS number does not reach it.
	ipForm, err := ParseSingleExtCommunity("target:192.0.2.1:5")
	if err != nil {
		t.Fatalf("target:192.0.2.1:5: %v", err)
	}
	if ipForm[0] != 0x01 {
		t.Errorf("target:192.0.2.1:5 type = %#x, want 0x01", ipForm[0])
	}

	for _, bad := range []string{"target:1.99999:5", "target:65536.0:5", "target:1.2.3:5"} {
		if _, err := ParseSingleExtCommunity(bad); err == nil {
			t.Errorf("%s was accepted", bad)
		}
	}
}

// TestExtCommunityLSuffixIsOrthogonalToTheSpelling proves the `L` suffix means
// the same thing in every spelling: encode as four octets whatever the value.
//
// For a value of more than 65535 it is redundant, because the value forces that
// encoding by itself. Redundant is accepted rather than refused.
//
// VALIDATES: ParseExtCommunityAdmin reads the suffix after any spelling.
// PREVENTS: `0.100L` and `100L` disagreeing, and `1.10L` being an error while
// `1.10` is not.
func TestExtCommunityLSuffixIsOrthogonalToTheSpelling(t *testing.T) {
	// The suffix forces the 4-octet form for a value that does not need it.
	// The bytes are pinned rather than compared to another producer: two
	// wrong producers agree with each other.
	//
	// RFC 5668 Section 2: type 0x02, sub-type 0x02 for a route target, then
	// the 4-octet Global Administrator (AS 100) and the 2-octet Local
	// Administrator (5).
	wantForced := ExtendedCommunity{0x02, 0x02, 0x00, 0x00, 0x00, 0x64, 0x00, 0x05}
	forced, err := ParseSingleExtCommunity("target:100L:5")
	if err != nil {
		t.Fatalf("target:100L:5: %v", err)
	}
	if forced != wantForced {
		t.Errorf("target:100L:5 = %v, want %v", forced, wantForced)
	}
	dottedForced, err := ParseSingleExtCommunity("target:0.100L:5")
	if err != nil {
		t.Fatalf("target:0.100L:5: %v", err)
	}
	if dottedForced != wantForced {
		t.Errorf("target:0.100L:5 = %v, want %v", dottedForced, wantForced)
	}

	// For more than 65535 the suffix is redundant, and says nothing new.
	// AS 65546 is 0x0001000a in four octets.
	wantRedundant := ExtendedCommunity{0x02, 0x02, 0x00, 0x01, 0x00, 0x0a, 0x00, 0x05}
	plain, err := ParseSingleExtCommunity("target:1.10:5")
	if err != nil {
		t.Fatalf("target:1.10:5: %v", err)
	}
	if plain != wantRedundant {
		t.Errorf("target:1.10:5 = %v, want %v", plain, wantRedundant)
	}
	redundant, err := ParseSingleExtCommunity("target:1.10L:5")
	if err != nil {
		t.Fatalf("target:1.10L:5: %v", err)
	}
	if redundant != wantRedundant {
		t.Errorf("target:1.10L:5 = %v, want %v", redundant, wantRedundant)
	}
}

// TestFlowSpecRedirectReadsEveryNotation proves the redirect administrator, the
// fourth reader of this field, agrees with the other three.
//
// VALIDATES: FlowSpecRedirect calls ParseExtCommunityAdmin (AC-1).
// PREVENTS: `redirect 1.10:5` refused while `target:1.10:5` is accepted.
func TestFlowSpecRedirectReadsEveryNotation(t *testing.T) {
	want, err := FlowSpecRedirect("65546", "5")
	if err != nil {
		t.Fatalf("redirect 65546:5: %v", err)
	}
	got, err := FlowSpecRedirect("1.10", "5")
	if err != nil {
		t.Fatalf("redirect 1.10:5: %v", err)
	}
	if got != want {
		t.Errorf("redirect 1.10 = %v, want %v", got, want)
	}

	// The suffix forces the 4-octet form here too, which the 2-octet value
	// width then proves.
	forced, err := FlowSpecRedirect("100L", "5")
	if err != nil {
		t.Fatalf("redirect 100L:5: %v", err)
	}
	if forced[0] != 0x82 {
		t.Errorf("redirect 100L type = %#x, want 0x82, the four-octet AS form", forced[0])
	}

	// The IPv4 administrator still wins its branch, and a malformed token is
	// still refused.
	if _, err := FlowSpecRedirect("192.0.2.1", "5"); err != nil {
		t.Errorf("redirect 192.0.2.1:5: %v", err)
	}
	if _, err := FlowSpecRedirect("1.99999", "5"); err == nil {
		t.Error("redirect 1.99999 was accepted")
	}
}
