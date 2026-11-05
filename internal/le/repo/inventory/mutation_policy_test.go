// Design: docs/architecture/plugin/feature-gates.md -- unknown mutation policy cannot certify declarations.
package repoinventory

import (
	"strings"
	"testing"
)

// TestMutationCheckRefusesUnspecifiedAndUnknown verifies that an absent or
// unknown policy refuses before the scan accesses any source population.
func TestMutationCheckRefusesUnspecifiedAndUnknown(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode mutationCheck
	}{
		{"unspecified", mutationCheckUnspecified},
		{"unknown", mutationCheck(255)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := expression{}
			err := reader.writes(declarationReference{}, tc.mode)
			if err == nil {
				t.Fatal("unsupported mutation policy certified a declaration")
			}
			if !strings.Contains(err.Error(), "unsupported declaration mutation scan mode") {
				t.Fatalf("mutation policy refusal lost its cause: %v", err)
			}
		})
	}
}
