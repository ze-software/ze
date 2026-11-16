// Design: docs/labs/pppoe-interop.md -- require the complete guarded mutation proof.
// Related: proof.go -- exact native ownership result.
package testvpppppoe

import (
	"context"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// The native answer must include all guarded mutation cases. Literal current
// and legacy answers keep the test independent of the checker's success text.
func TestMechanismOwnershipProofRequiresGuardedCases(t *testing.T) {
	for _, answer := range []struct {
		name   string
		stdout string
		proven bool
	}{
		{name: "guarded mutations and route behavior", stdout: "PPPoE ownership: 55 cases passed\n", proven: true},
		{name: "guarded mutations without route behavior", stdout: "PPPoE ownership: 53 cases passed\n"},
		{name: "session and binding cases only", stdout: "PPPoE ownership: 31 cases passed\n"},
		{name: "original ownership cases only", stdout: "PPPoE ownership: 30 cases passed\n"},
	} {
		t.Run(answer.name, func(t *testing.T) {
			docker := interoplab.NewFixtureDocker(mechanismProcess(func(context.Context, interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
				return interoplab.ProcessResult{Stdout: answer.stdout}, nil
			}))
			err := checkOwnershipOracle(t.Context(), mechanismProbeLab{docker: docker})
			if answer.proven {
				if err != nil {
					t.Fatalf("complete guarded mutation proof refused: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("legacy ownership result satisfied the guarded mutation proof")
			}
		})
	}
}
