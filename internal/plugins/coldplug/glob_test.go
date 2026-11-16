package coldplug

import "testing"

// TestAliasMatch proves the alias glob reads modules.alias patterns the way
// kmod's fnmatch(pattern, modalias, 0) does.
//
// Method: one table of pattern, modalias and the answer fnmatch gives, each
// row named for the rule it pins, with a refused neighbor beside every
// accepted row so a matcher that answers true for everything fails.
// MUTATION: treating '/' as a separator (path.Match) fails "star crosses a
// slash"; dropping the '!' negation fails "negated class refuses a member";
// a class that ignores ranges fails "range class"; a '*' with no backtrack
// fails "star backtracks"; reading '\' as an ordinary byte fails "escaped
// star is literal".
func TestAliasMatch(t *testing.T) {
	const e1000 = "pci:v00008086d0000100Esv*sd*bc*sc*i*"
	tests := []struct {
		name     string
		pattern  string
		modalias string
		want     bool
	}{
		{"pci driver matches its device", e1000, "pci:v00008086d0000100Esv00008086sd0000001Ebc02sc00i00", true},
		{"pci driver refuses another device id", e1000, "pci:v00008086d0000100Fsv00008086sd0000001Ebc02sc00i00", false},
		{"question mark takes one byte", "usb:v1234p?678d*", "usb:v1234p5678d0100", true},
		{"question mark refuses no byte", "usb:v1234p?678d*", "usb:v1234p678d0100", false},
		{"range class", "usb:v0BDAp8179d0[0-1]*", "usb:v0BDAp8179d01FFdc00", true},
		{"range class refuses outside", "usb:v0BDAp8179d0[0-1]*", "usb:v0BDAp8179d02FFdc00", false},
		{"negated class takes a non-member", "x[!0-9]", "xa", true},
		{"negated class refuses a member", "x[!0-9]", "x5", false},
		{"caret negates too", "x[^0-9]", "x5", false},
		{"close bracket first is a member", "x[]a]", "x]", true},
		{"dash before close is a member", "x[a-]", "x-", true},
		{"unterminated bracket is literal", "ab[c", "ab[c", true},
		{"star crosses a slash", "of:N*Tfsl,ifc", "of:Nsoc/ifc@0Tfsl,ifc", true},
		{"leading dot is ordinary", "*.x", ".x", true},
		{"escaped star is literal", `a\*b`, "a*b", true},
		{"escaped star refuses another byte", `a\*b`, "axb", false},
		{"star backtracks", "*a*b", "xaybzb", true},
		{"trailing byte refused", "a*b", "acbd", false},
		{"star matches the empty run", "pci:*", "pci:", true},
		{"pattern longer than alias", "pci:v1", "pci:v", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := aliasMatch(tt.pattern, tt.modalias); got != tt.want {
				t.Errorf("aliasMatch(%q, %q) = %v, want %v", tt.pattern, tt.modalias, got, tt.want)
			}
		})
	}
}
