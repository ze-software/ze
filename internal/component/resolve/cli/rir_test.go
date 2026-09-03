// Design: docs/architecture/resolve.md -- wiring test for the RIR lookup CLI
package cli

import (
	"strings"
	"testing"
)

// TestResolveRIRAnswersFromTheEmbeddedSeed proves the operator reaches the
// delegation table through `ze resolve rir <asn>` with no network and no
// daemon. Goal: the whole path, from the argument an operator types to the
// registry name printed on stdout. Method: run the real dispatch in Run and
// read what it wrote.
//
// AS15169 is delegated to ARIN, so a working lookup prints ARIN and its whois
// server and exits zero.
func TestResolveRIRAnswersFromTheEmbeddedSeed(t *testing.T) {
	code, stdout, stderr := captureRun("rir", "15169")
	if code != exitOK {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	if !strings.Contains(stdout, "ARIN") {
		t.Errorf("stdout names no registry: %q", stdout)
	}
	if !strings.Contains(stdout, "whois.arin.net") {
		t.Errorf("stdout names no whois server: %q", stdout)
	}
}

// TestResolveRIRSeparatesNoRangeFromNoTable proves the two failures stay
// distinguishable at the entry point (AC-2). Goal: an operator who asks about
// an AS number nobody holds MUST NOT be told the same thing as an operator
// whose table could not be read. Method: ask about AS0, which no registry is
// delegated, and require the message to name the delegated range rather than
// the table.
func TestResolveRIRSeparatesNoRangeFromNoTable(t *testing.T) {
	code, _, stderr := captureRun("rir", "0")
	if code != exitError {
		t.Fatalf("exit %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr, "no delegated range") {
		t.Errorf("stderr does not report an undelegated AS number: %q", stderr)
	}
}

// TestResolveRIRRefusesAnASNAboveUint32 proves the argument is bounded before
// it reaches the lookup. Goal: 4294967296 is one above the last valid AS
// number and MUST be refused rather than truncated to zero.
func TestResolveRIRRefusesAnASNAboveUint32(t *testing.T) {
	code, _, stderr := captureRun("rir", "4294967296")
	if code != exitError {
		t.Fatalf("exit %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr, "invalid AS number") {
		t.Errorf("stderr does not name the invalid argument: %q", stderr)
	}
}
