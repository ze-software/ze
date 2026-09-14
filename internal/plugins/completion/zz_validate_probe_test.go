// VALIDATES: `ze completion words run validate ...` still walks the absolute
// tree with the whole path, after runCompletionTree stopped naming `validate`
// in a case list of its own.
// PREVENTS: a silent regression from that removal. `validate` is not a
// canonical verb, so command.IsVerb refuses it and the non-verb branch answers.
// Both the verb-relative tree and the absolute tree are built from the same
// allCLIRPCs() population, and `validate config` registers through
// MustRegisterLocalData rather than as an RPC, so neither tree ever held it:
// the two routes answer the same nothing, and this test pins the path form.
//
// This file is the residue of a probe. Its body belongs beside the other
// runCompletionTree cases in words_test.go, and the file should go when a
// session can approve the deletion this one could not.

package completion

import (
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

func TestRunCompletionTreeKeepsValidateOnThePath(t *testing.T) {
	if command.IsVerb("validate") {
		t.Fatal("validate is not a canonical verb; the registry must not hold it")
	}
	_, rest := runCompletionTree([]string{"validate", "config"})
	if len(rest) != 2 || rest[0] != "validate" || rest[1] != "config" {
		t.Errorf("rest = %v, want [validate config]: a non-verb keeps its whole path", rest)
	}
}
