// Design: docs/architecture/ospf/ospf-1-types.md -- malformed dotted-quad and integer rejection

package types

import "testing"

// VALIDATES: AC-2 - parsers reject malformed dotted quads and invalid area integers.
// PREVENTS: accepting ambiguous or truncated config and CLI identifiers.
func TestParseRejectsMalformed(t *testing.T) {
	badDotted := []string{"", "1.2.3", "1.2.3.4.5", "1..2.3", "1.2.3.256", "1.2.3.-1", "1.2.3.a", "01.2.3.4"}
	for _, input := range badDotted {
		if _, err := ParseRouterID(input); err == nil {
			t.Fatalf("ParseRouterID(%q) succeeded, want error", input)
		}
		if _, err := ParseLinkStateID(input); err == nil {
			t.Fatalf("ParseLinkStateID(%q) succeeded, want error", input)
		}
	}
	badArea := []string{"", "4294967296", "-1", "1.2.3", "1.2.3.256", "area0"}
	for _, input := range badArea {
		if _, err := ParseAreaID(input); err == nil {
			t.Fatalf("ParseAreaID(%q) succeeded, want error", input)
		}
	}
}
