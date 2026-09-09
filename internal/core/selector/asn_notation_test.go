// Design: docs/architecture/core-design.md -- AS notation in a peer selector
package selector

import "testing"

// TestASNSelectorReadsEveryNotation proves the `as<N>` peer selector takes the
// AS number in any of the three RFC 5396 spellings.
//
// The failure this guards is SILENT. ParseDefault turns a refusal into
// PeerName, so `as1.10` became a peer name and matched no peer. It reported
// nothing: no error, no empty-result message, no log line.
//
// VALIDATES: ParseASNSelector reads asplain, asdot and asdot+ (AC-1).
// PREVENTS: a peer command selecting nothing for an AS number the operator
// read on a show output.
func TestASNSelectorReadsEveryNotation(t *testing.T) {
	for _, spelling := range []string{"as65546", "AS65546", "as1.10", "AS1.10"} {
		number, ok := ParseASNSelector(spelling)
		if !ok {
			t.Errorf("ParseASNSelector(%q) refused a valid selector", spelling)
			continue
		}
		if number != 65546 {
			t.Errorf("ParseASNSelector(%q) = %d, want 65546", spelling, number)
		}
	}

	// asdot+ writes an AS number less than 65536 with a zero high field.
	if number, ok := ParseASNSelector("as0.100"); !ok || number != 100 {
		t.Errorf("ParseASNSelector(as0.100) = %d/%v, want 100/true", number, ok)
	}

	// A token that names no AS number is still not a selector, and Parse then
	// answers a peer name rather than an AS number..
	for _, bad := range []string{"as1.99999", "as65536.0", "aspath", "as"} {
		if number, ok := ParseASNSelector(bad); ok {
			t.Errorf("ParseASNSelector(%q) = %d, want it refused", bad, number)
		}
	}
}

// TestParseReachesTheASNSelector proves the whole path, from the string an
// operator types to the typed selector every peer command resolves.
//
// VALIDATES: Parse answers KindASN for a dotted selector.
// PREVENTS: the notation reaching ParseASNSelector but not the entry point.
func TestParseReachesTheASNSelector(t *testing.T) {
	sel, err := Parse("as1.10")
	if err != nil {
		t.Fatalf("Parse(as1.10): %v", err)
	}
	if sel.SelectorKind() != KindASN {
		t.Fatalf("Parse(as1.10) kind = %v, want %v", sel.SelectorKind(), KindASN)
	}
	if sel.ASNValue() != 65546 {
		t.Errorf("Parse(as1.10) ASN = %d, want 65546", sel.ASNValue())
	}

	// ParseDefault must not turn a readable selector into a peer name, which
	// is the silent failure this whole test exists for.
	if got := ParseDefault("as1.10"); got.SelectorKind() != KindASN {
		t.Errorf("ParseDefault(as1.10) kind = %v, want %v", got.SelectorKind(), KindASN)
	}
}
