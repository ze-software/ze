// Design: docs/architecture/api/commands.md -- AS-number notation in the `show bgp rib` path filter
package rib

import "testing"

// TestPathPatternReadsEveryNotation proves the `show bgp rib path <pattern>`
// filter takes the AS numbers in any RFC 5396 spelling. It also proves its
// validator and its matcher agree about which patterns are legal.
//
// The pair is what matters: a pattern the validator accepts and the matcher
// refuses passes verification and then matches nothing, which reads as "no
// routes have that AS in their path".
//
// VALIDATES: validatePathPattern and matchASPath read asplain, asdot and
// asdot+ (AC-1).
// PREVENTS: an operator reading 1.10 on a show output, typing it into the
// path filter, and being told there are no such routes.
func TestPathPatternReadsEveryNotation(t *testing.T) {
	path := []uint32{65546, 100}
	for _, pattern := range []string{"65546", "1.10", "^1.10,100", "0.100"} {
		if msg := validatePathPattern(pattern); msg != "" {
			t.Errorf("validatePathPattern(%q) = %q, want it accepted", pattern, msg)
		}
		if !matchASPath(path, pattern) {
			t.Errorf("matchASPath(%v, %q) = false, want true", path, pattern)
		}
	}

	// A pattern naming an AS number this path does not carry still misses.
	if matchASPath(path, "1.11") {
		t.Errorf("matchASPath(%v, 1.11) = true, want false", path)
	}

	// A token that names no AS number is refused by the validator, and the
	// matcher answers false rather than matching everything.
	for _, bad := range []string{"1.99999", "65536.0", "peer"} {
		if msg := validatePathPattern(bad); msg == "" {
			t.Errorf("validatePathPattern(%q) accepted a token that names no AS number", bad)
		}
		if matchASPath(path, bad) {
			t.Errorf("matchASPath(%v, %q) = true, want false", path, bad)
		}
	}
}
