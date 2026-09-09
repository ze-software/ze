// Design: docs/architecture/resolve.md -- AS-number notation at the resolve CLI
package cli

import (
	"strings"
	"testing"
)

// TestResolveRIRReadsEveryNotation proves `ze resolve rir` takes the AS number
// in the notation the operator read it in. Goal: the whole path, from the
// argument typed to the registry printed. Method: ask for AS15169 in each RFC
// 5396 spelling through the real dispatch and read what it wrote.
//
// AS15169 is delegated to ARIN. asdot writes an AS number less than 65536 in
// plain, so 15169 and 0.15169 are the two spellings this AS number has.
//
// VALIDATES: the rir command word reads asplain and asdot+ (AC-1).
// PREVENTS: "error: invalid AS number: 0.15169" for an allocated AS number.
func TestResolveRIRReadsEveryNotation(t *testing.T) {
	for _, spelling := range []string{"15169", "0.15169"} {
		code, stdout, stderr := captureRun("rir", spelling)
		if code != exitOK {
			t.Fatalf("resolve rir %s: exit %d, stderr %q", spelling, code, stderr)
		}
		if !strings.Contains(stdout, "ARIN") {
			t.Errorf("resolve rir %s: stdout names no registry: %q", spelling, stdout)
		}
	}

	// A dotted token whose low field overflows names no AS number, so the
	// command still refuses it rather than looking up something else.
	if code, _, _ := captureRun("rir", "0.65536"); code != exitError {
		t.Errorf("resolve rir 0.65536: exit %d, want %d", code, exitError)
	}
}

// TestResolveCymruReadsADottedASNumber proves the `cymru asn-name` word parses
// the AS number before it reaches DNS, so a dotted token is not refused as
// malformed. The method points the command at a closed local port, so the
// lookup fails after the parse, and reads which failure it reported.
//
// A dotted token that reaches DNS has been read. A dotted token refused at the
// parse never gets there, and that is the defect this guards.
//
// VALIDATES: cmdCymru reads asplain, asdot and asdot+ (AC-1).
// PREVENTS: "error: invalid ASN: 1.10" for an AS number an operator read on a
// show output.
func TestResolveCymruReadsADottedASNumber(t *testing.T) {
	code, _, stderr := captureRun("cymru", "--dns-server", "127.0.0.1:1", "asn-name", "1.10")
	if code != exitError {
		t.Fatalf("exit %d, want %d: the closed port must fail the lookup", code, exitError)
	}
	if strings.Contains(stderr, "invalid ASN") {
		t.Errorf("1.10 was refused at the parse: %q", stderr)
	}

	// A token that names no AS number is still refused at the parse.
	_, _, stderr = captureRun("cymru", "--dns-server", "127.0.0.1:1", "asn-name", "peer")
	if !strings.Contains(stderr, "invalid ASN") {
		t.Errorf("stderr for a malformed token = %q, want it to name the invalid ASN", stderr)
	}
}

// TestResolvePeeringDBReadsADottedASNumber proves the `peeringdb` word parses
// the AS number before it reaches the API, so a dotted token is not refused as
// malformed. The method points the command at a closed local port, so the
// lookup fails after the parse, and reads which failure it reported.
//
// VALIDATES: cmdPeeringDB reads asplain, asdot and asdot+ (AC-1).
// PREVENTS: "error: invalid ASN: 1.10" for an AS number an operator read on a
// show output.
func TestResolvePeeringDBReadsADottedASNumber(t *testing.T) {
	code, _, stderr := captureRun("peeringdb", "--url", "http://127.0.0.1:1", "max-prefix", "1.10")
	if code != exitError {
		t.Fatalf("exit %d, want %d: the closed port must fail the lookup", code, exitError)
	}
	if strings.Contains(stderr, "invalid ASN") {
		t.Errorf("1.10 was refused at the parse: %q", stderr)
	}

	_, _, stderr = captureRun("peeringdb", "--url", "http://127.0.0.1:1", "max-prefix", "peer")
	if !strings.Contains(stderr, "invalid ASN") {
		t.Errorf("stderr for a malformed token = %q, want it to name the invalid ASN", stderr)
	}
}
