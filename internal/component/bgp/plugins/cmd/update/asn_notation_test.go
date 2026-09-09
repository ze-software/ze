// Design: docs/architecture/bgp/update-command.md -- AS notation at `update text`
package update

import "testing"

// TestUpdateTextRDReadsEveryNotation proves the `rd` word of `update text`
// takes the AS number in any of the three RFC 5396 spellings. The method
// parses the same route distinguisher in each spelling at the command's own
// entry point and compares the eight wire bytes.
//
// This is the entry point the config file does NOT share: the configuration
// reader accepted `rd 1.10:5` while this path answered "invalid IP in RD".
//
// VALIDATES: parseRDFlat reaches nlri.ParseRDString, which reads asn.Parse.
// PREVENTS: one spelling working in a config file and failing at a command.
func TestUpdateTextRDReadsEveryNotation(t *testing.T) {
	var want parsedAttrs
	if _, err := parseRDFlat([]string{"rd", "65546:5"}, &want); err != nil {
		t.Fatalf("rd 65546:5: %v", err)
	}

	for _, spelling := range []string{"1.10:5", "2:1.10:5"} {
		var got parsedAttrs
		consumed, err := parseRDFlat([]string{"rd", spelling}, &got)
		if err != nil {
			t.Fatalf("rd %s: %v", spelling, err)
		}
		if consumed != 2 {
			t.Errorf("rd %s consumed %d arguments, want 2", spelling, consumed)
		}
		if got.RD != want.RD {
			t.Errorf("rd %s = %v, want %v", spelling, got.RD, want.RD)
		}
	}

	// A dotted token that names no AS number is still refused.
	var bad parsedAttrs
	if _, err := parseRDFlat([]string{"rd", "1.99999:5"}, &bad); err == nil {
		t.Error("rd 1.99999:5 was accepted")
	}
}
