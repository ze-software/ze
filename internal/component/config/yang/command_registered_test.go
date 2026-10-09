// Design: docs/architecture/api/commands.md — command argument definitions
// Related: command.go — the YANG lowering this test drives

package yang_test

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/yang"
	_ "github.com/ze-software/ze/internal/component/plugin/all" // registers every compiled-in YANG module
)

// TestCommandTreeBuildsFromEveryRegisteredModule lowers every registered
// module into the command tree and checks every argument definition on it.
//
// VALIDATES: BuildCommandTree over the full registered module set raises no
// constructor BUG, and every ArgDef it publishes was built by a constructor
// (AC-20, AC-13).
// PREVENTS: a YANG restriction resolution admits and the ArgDef constructor
// refuses, which would panic the daemon at startup.
func TestCommandTreeBuildsFromEveryRegisteredModule(t *testing.T) {
	loader, err := yang.DefaultLoader()
	if err != nil {
		t.Fatalf("DefaultLoader: %v", err)
	}
	root := yang.BuildCommandTree(loader)
	if root == nil {
		t.Fatal("BuildCommandTree answered no tree")
	}
	defs := 0
	// The tree is the compiled-in command model, so the walk is bounded.
	var walk func(path string, node *command.Node)
	walk = func(path string, node *command.Node) {
		for i := range node.ArgDefs {
			defs++
			// ValidateArgString judges the constructed marker before any
			// token, so ErrArgDef here means no constructor built it.
			if err := command.ValidateArgString("", &node.ArgDefs[i]); errors.Is(err, command.ErrArgDef) {
				t.Errorf("%s: argument %q was not built by a constructor", path, node.ArgDefs[i].Name())
			}
		}
		for name, child := range node.Children {
			walk(path+" "+name, child)
		}
	}
	walk("", root)
	if defs == 0 {
		t.Fatal("the registered module set lowered to no argument definition")
	}
}
