// Design: docs/architecture/plugin/feature-gates.md -- committed registration reachability.
package repoinventory

import "testing"

// TestCommittedPluginsIgnoreBlankFunctionDeclarations keeps nonbinding Go
// declarations out of the callable index and out of initialization reachability.
func TestCommittedPluginsIgnoreBlankFunctionDeclarations(t *testing.T) {
	root := committedFixture(t)
	write(t, root, "internal/plugins/example/blank.go", `package example
import r "github.com/ze-software/ze/internal/component/plugin/registry"
func _() { _ = r.Register(r.Registration{Name: "blank-function-is-not-an-initializer"}) }
func _() {}
`)
	commitInventoryFixture(t, root)
	plugins, err := DeclaredPlugins(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 1 || plugins[0].Name != "committed-linux" {
		t.Fatalf("nonbinding declarations changed committed registration reachability: %+v", plugins)
	}
}
