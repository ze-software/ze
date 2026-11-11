// Design: docs/architecture/testing/runner-architecture.md -- suite selection refusals.
// Related: cmd_bgp_refusal_test.go -- the exit status of the same unknown verb.

package cli

import (
	"strings"
	"testing"
)

// TestBgpUnknownVerbRefusalNamesIt proves the refusal of an unknown suite verb
// names the verb the operator typed. Method: parse one unknown verb through the
// suite's own argument parser and read the returned error.
//
// VALIDATES: zeTestParseRunCLI refuses an unknown suite verb with an error
// whose text carries that verb.
// PREVENTS: a refusal that says only that something was wrong, leaving the
// operator to guess which word of the command line the parser rejected.
func TestBgpUnknownVerbRefusalNamesIt(t *testing.T) {
	_, err := zeTestParseRunCLI([]string{"ac1-unknown-verb"})
	if err == nil {
		t.Fatal("unknown suite verb was accepted")
	}
	if !strings.Contains(err.Error(), "ac1-unknown-verb") {
		t.Fatalf("refusal %q does not name the unknown verb", err.Error())
	}
}
