// Design: docs/labs/pppoe-interop.md -- the parser proof must fail for its named mutation.
// Related: proof.go -- positive mechanism followed by the isolated metadata oracle.
package testvpppppoe

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// The combined proof is a separately declared action; the existing mechanism
// run keeps its one positive question and does not silently acquire a mutant.
func TestMechanismProofActionRegisters(t *testing.T) {
	if !strings.Contains(actions.Subs(), "proof (writes)") {
		t.Fatal("the combined mechanism/parser proof is not registered")
	}
}

// Pinned vppctl exits zero after receiving a CLI error. Only the exact parser
// metadata refusal proves this mutant, never a transport or startup failure.
func TestMechanismMetadataProofRequiresItsOracle(t *testing.T) {
	oracle := "empty PADT: wrong session ID 0, expected 17"
	for _, test := range []struct {
		name, stdout, stderr string
		exit                 int
		cause                error
		proven               bool
	}{
		{name: "metadata refusal", stdout: "test pppoe frame-bounds: " + oracle + "\n", proven: true},
		{name: "unknown CLI", stdout: "unknown input `test pppoe frame-bounds'\n"},
		{name: "unchanged parser", stdout: "PPPoE frame bounds: 31 cases passed\n"},
		{name: "another assertion", stdout: "empty PADT: wrong protocol 0, expected 49185\n"},
		{name: "transport failure", stdout: oracle, cause: errors.New("Docker transport disconnected")},
		{name: "process failure", stdout: oracle, exit: 1},
		{name: "contradictory success", stdout: oracle + "\nPPPoE frame bounds: 31 cases passed\n"},
		{name: "extra assertion", stdout: "test pppoe frame-bounds: " + oracle + "\ncleanup assertion failed\n"},
		{name: "embedded assertion", stdout: "unexpected failure containing test pppoe frame-bounds: " + oracle + "\n"},
		{name: "stderr failure", stdout: "test pppoe frame-bounds: " + oracle + "\n", stderr: "cleanup failed\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			docker := interoplab.NewFixtureDocker(mechanismProcess(func(context.Context, interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
				return interoplab.ProcessResult{Stdout: test.stdout, Stderr: test.stderr, ExitCode: test.exit}, test.cause
			}))
			err := checkMetadataOracle(t.Context(), mechanismProbeLab{docker: docker})
			if test.proven {
				if err != nil {
					t.Fatalf("the named metadata mutation was not recognized: %v", err)
				}
			} else if err == nil {
				t.Fatal("an unrelated or successful run counted as metadata discrimination")
			}
		})
	}
}
