// Design: docs/architecture/command-ownership.md -- owner command schema
// Related: model_test.go -- persistent profile configuration remains separate

//go:build ze_l2tp

package yang_test

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

// Load the real registered modules, not a source substring or supplied test
// schema. The visible cleanup command must lower to its actual owned wire path.
func TestCoSCleanupCommandIsInRegisteredYANG(t *testing.T) {
	resolved, err := configyang.DefaultLoader()
	if err != nil {
		t.Fatal(err)
	}
	root := configyang.BuildCommandTree(resolved)
	node := command.FindNode(root, strings.Fields("request class-of-service cleanup"))
	if node == nil {
		t.Fatal("the registered command schema has no class-of-service cleanup operation")
	}
	if node.WireMethod != "ze-cos:cleanup" {
		t.Fatalf("cleanup wire method=%q, want ze-cos:cleanup", node.WireMethod)
	}
	if len(node.ArgDefs) != 0 {
		t.Fatalf("cleanup acquired undeclared arguments: %+v", node.ArgDefs)
	}
}
