// Design: docs/labs/pppoe-interop.md -- proof image grammar passes dispatcher startup.
package buildvpp_test

import (
	"testing"

	_ "github.com/ze-software/ze/internal/le"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// Tree runs the same metadata checks as dispatcher startup, including the menu
// text limits that a package-only action dispatch does not exercise.
func TestVPPProofVariantGrammarPassesStartupChecks(t *testing.T) {
	if leroot.Tree() == nil {
		t.Fatal("the proof variant has no command tree")
	}
}
