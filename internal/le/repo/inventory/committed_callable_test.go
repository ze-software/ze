// Design: docs/architecture/plugin/feature-gates.md -- init-callable declarations cannot disappear.
package repoinventory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCommittedPluginsFollowImmediateInitClosures proves a syntactically known
// immediate closure is executed membership, not an uncalled metadata callback.
func TestCommittedPluginsFollowImmediateInitClosures(t *testing.T) {
	root := committedFixture(t)
	name := filepath.Join(root, "internal/plugins/example/register_linux.go")
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	before := "func init() { registerExample() }"
	if strings.Count(string(raw), before) != 1 {
		t.Fatal("fixture init marker is not unique")
	}
	write(t, root, "internal/plugins/example/register_linux.go", strings.Replace(string(raw), before, "func init() { func() { registerExample() }() }", 1))
	committedGit(t, root, "add", ".")
	committedGit(t, root, "-c", "user.name=Inventory Fixture", "-c", "user.email=inventory@example.invalid", "commit", "-m", "immediate init closure")
	plugins, err := Plugins(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 1 || plugins[0].Name != "committed-linux" {
		t.Fatalf("immediate init closure lost registration: %+v", plugins)
	}
}
