// Design: docs/architecture/plugin/feature-gates.md -- unresolved populations refuse.
package repoinventory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCommittedPluginsRefuseUnresolvedMetadata proves a registration the static
// reader cannot resolve never becomes a partial or host-derived successful list.
func TestCommittedPluginsRefuseUnresolvedMetadata(t *testing.T) {
	cases := []struct {
		name string
		before string
		after string
		reason string
	}{
		{"unknown-description", `Description: "committed description"`, `Description: describeAtRuntime()`, "string value requires unsupported executable logic"},
		{"later-name-mutation", `if err := r.Register(reg);`, `reg.Name = "mutated"; if err := r.Register(reg);`, "registration metadata is mutated after declaration"},
		{"conditional-registration", `func init() { registerExample() }`, `func init() { if runtimeCondition() { registerExample() } }`, "plugin registration depends on an unresolved condition or iteration"},
		{"unknown-dependencies", `Dependencies: append([]string{"first"}, prerequisites...)`, `Dependencies: runtimeDependencies()`, "slice requires unsupported executable logic"},
	}
	for _, scenario := range cases {
		t.Run(scenario.name, func(t *testing.T) {
			root := committedFixture(t)
			path := filepath.Join(root, "internal/plugins/example/register_linux.go")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(raw), scenario.before) != 1 {
				t.Fatal("fixture replacement is not unique")
			}
			write(t, root, "internal/plugins/example/register_linux.go", strings.Replace(string(raw), scenario.before, scenario.after, 1))
			committedGit(t, root, "add", ".")
			committedGit(t, root, "-c", "user.name=Inventory Fixture", "-c", "user.email=inventory@example.invalid", "commit", "-m", scenario.name)
			plugins, err := Plugins(root)
			if err == nil {
				t.Fatalf("unresolved committed registration published %d plugins without refusal", len(plugins))
			}
			if !strings.Contains(err.Error(), scenario.reason) {
				t.Fatalf("fixture failed before its intended refusal %q: %v", scenario.reason, err)
			}
			if plugins != nil {
				t.Fatalf("refusal retained partial inventory: %v", plugins)
			}
		})
	}
}
