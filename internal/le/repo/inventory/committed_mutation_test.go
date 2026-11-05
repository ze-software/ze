// Design: docs/architecture/plugin/feature-gates.md -- immutable declaration ownership.
package repoinventory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCommittedPluginsRefuseMutatedDeclarationInputs proves slice aliases and
// escaped registration owners cannot turn a supported declaration into stale
// metadata. Each fixture commits the actual mutation before invoking Plugins.
func TestCommittedPluginsRefuseMutatedDeclarationInputs(t *testing.T) {
	for name, mutation := range map[string]string{
		"slice-input": `prerequisites = append(prerequisites, "late");`,
		"registration-alias": `alias := &reg; alias.Name = "late";`,
		"whole-registration": `reg = r.Registration{Name: "late", RunEngine: names.Engine};`,
	} {
		t.Run(name, func(t *testing.T) {
			root := committedFixture(t)
			path := filepath.Join(root, "internal/plugins/example/register_linux.go")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			marker := "if err := r.Register(reg);"
			if strings.Count(string(raw), marker) != 1 {
				t.Fatal("fixture registration marker is not unique")
			}
			write(t, root, "internal/plugins/example/register_linux.go", strings.Replace(string(raw), marker, mutation+marker, 1))
			committedGit(t, root, "add", ".")
			committedGit(t, root, "-c", "user.name=Inventory Fixture", "-c", "user.email=inventory@example.invalid", "commit", "-m", name)
			plugins, err := Plugins(root)
			if err == nil || plugins != nil {
				t.Fatalf("mutated declaration published %+v: %v", plugins, err)
			}
			if !strings.Contains(err.Error(), "mutated") {
				t.Fatalf("fixture failed before declaration mutation refusal: %v", err)
			}
		})
	}
}
