// Design: docs/architecture/plugin/feature-gates.md -- pinned HEAD and complete target context.
package repoinventory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestCommittedSnapshotKeepsMembershipMetadataAndSchemaAcrossHEADMovement reads
// one snapshot, changes all three committed inputs, then resolves the old one.
func TestCommittedSnapshotKeepsMembershipMetadataAndSchemaAcrossHEADMovement(t *testing.T) {
	root := committedFixture(t)
	snapshot, err := ReadCommitted(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	revision := snapshot.Revision()
	write(t, root, "feature-gates.txt", "ze_other internal/plugins/other\n")
	write(t, root, "internal/schema/external.yang", "module replaced { }\n")
	write(t, root, "internal/plugins/example/register_linux.go", "package example\n")
	committedGit(t, root, "add", ".")
	committedGit(t, root, "-c", "user.name=Inventory Fixture", "-c", "user.email=inventory@example.invalid", "commit", "-m", "advance HEAD")
	plugins, err := snapshot.Plugins()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Revision() != revision || len(plugins) != 1 || plugins[0].Name != "committed-linux" || !plugins[0].HasYANG {
		t.Fatalf("snapshot mixed HEADs: revision %s plugins %+v", snapshot.Revision(), plugins)
	}
	if got := snapshot.Gates().Requirement("internal/plugins/example"); len(got) != 1 || got[0] != "ze_example" {
		t.Fatalf("snapshot mixed feature manifests: %v", got)
	}
}

// TestCommittedSourceUsesTheCompleteLinuxBuildContext selects architecture,
// compiler, cgo, release and tool constraints independently of a simulated Mac.
func TestCommittedSourceUsesTheCompleteLinuxBuildContext(t *testing.T) {
	t.Setenv("GOOS", "darwin")
	t.Setenv("GOARCH", "arm64")
	t.Setenv("CGO_ENABLED", "1")
	root := committedFixture(t)
	name := filepath.Join(root, "internal/plugins/example/register_linux.go")
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "internal/plugins/example/register_linux.go", "//go:build ze_le && ze_example && linux && amd64 && gc && !cgo && amd64.v1 && go1.26\n\n"+string(raw))
	write(t, root, "internal/plugins/example/register_darwin.go", `package example
import r "github.com/ze-software/ze/internal/component/plugin/registry"
func init() { _ = r.Register(r.Registration{Name: "darwin-only", RunEngine: func() {}}) }
`)
	write(t, root, "internal/plugins/example/register_absent.go", `//go:build !ze_example
package example
import r "github.com/ze-software/ze/internal/component/plugin/registry"
func init() { _ = r.Register(r.Registration{Name: "feature-off", RunEngine: func() {}}) }
`)
	committedGit(t, root, "add", ".")
	committedGit(t, root, "-c", "user.name=Inventory Fixture", "-c", "user.email=inventory@example.invalid", "commit", "-m", "context constraints")
	plugins, err := Plugins(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 1 || plugins[0].Name != "committed-linux" {
		t.Fatalf("target build population = %+v", plugins)
	}
}
