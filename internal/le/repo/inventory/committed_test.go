// Design: docs/architecture/plugin/feature-gates.md -- committed inventory population.
package repoinventory

import (
	"os/exec"
	"reflect"
	"testing"
)

// committedGit changes only a test-owned repository, never the shared checkout.
func committedGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture git %v: %v: %s", args, err, output)
	}
	return string(output)
}

// committedFixture models the actual entry/import/registration boundary, including
// an unreferenced literal and a target-Linux file. No fixture source is compiled.
func committedFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "go.mod", "module github.com/ze-software/ze\n\ngo 1.26\n")
	write(t, root, "feature-gates.txt", "ze_example internal/plugins/example\n")
	write(t, root, "cmd/ze/main.go", `//go:build ze_le
package main
import _ "github.com/ze-software/ze/internal/component/plugin/all"
func main() {}
`)
	write(t, root, "internal/component/plugin/all/all.go", `//go:build ze_example
package all
import _ "github.com/ze-software/ze/internal/plugins/example"
`)
	write(t, root, "internal/component/plugin/registry/registry.go", `package registry
type Registration struct { Name, Description, YANG string; Dependencies, OptionalDependencies, StartAfter, ConfigRoots []string; RunEngine func() }
func Register(Registration) error { return nil }
`)
	// Separate witness files survive tests replacing the sink declarations;
	// the embedded-schema and metadata assertions keep their original inputs.
	write(t, root, "internal/component/plugin/registry/external_witness.go", "package registry\nimport _ \"embed\"\n")
	write(t, root, "internal/component/config/yang/external_witness.go", "package yang\nimport _ \"embed\"\n")
	write(t, root, "internal/shared/engine.go", `package shared
const Name = "committed-" + "linux"
const Root = "example"
func Engine() {}
`)
	write(t, root, "internal/schema/schema.go", `package schema
import _ "embed"
//go:embed external.yang
var Document string
`)
	write(t, root, "internal/schema/external.yang", "module external { namespace urn:example; prefix x; }\n")
	write(t, root, "internal/plugins/example/register_linux.go", `package example
import (
 r "github.com/ze-software/ze/internal/component/plugin/registry"
 names "github.com/ze-software/ze/internal/shared"
 schema "github.com/ze-software/ze/internal/schema"
)
var prerequisites = []string{"base"}
func getYANG() string { return schema.Document }
func init() { registerExample() }
func registerExample() {
 reg := r.Registration{
  Name: names.Name, Description: "committed description", RunEngine: names.Engine,
  Dependencies: append([]string{"first"}, prerequisites...), OptionalDependencies: []string{"optional"},
  StartAfter: []string{"ordered"}, ConfigRoots: []string{names.Root}, YANG: getYANG(),
 }
 if err := r.Register(reg); err != nil { panic(err) }
}
func unused() { _ = r.Register(r.Registration{Name: "unreachable", RunEngine: names.Engine}) }
`)
	committedGit(t, root, "init", "--initial-branch=main")
	committedGit(t, root, "add", ".")
	committedGit(t, root, "-c", "user.name=Inventory Fixture", "-c", "user.email=inventory@example.invalid", "commit", "-m", "committed population")
	return root
}

// TestCommittedPluginsIgnoreDirtySourcesAndRetainLinuxRegistration proves the
// public producer reads committed import reachability and metadata, not its host
// registry or checkout. Imported engines and schema files retain their origins.
func TestCommittedPluginsIgnoreDirtySourcesAndRetainLinuxRegistration(t *testing.T) {
	root := committedFixture(t)
	write(t, root, "internal/plugins/example/register_linux.go", "package example\n// Uncommitted replacement removes every registration.\n")
	write(t, root, "feature-gates.txt", "ze_dirty internal/plugins/dirty\n")
	write(t, root, "internal/schema/external.yang", "module dirty { }\n")
	plugins, err := DeclaredPlugins(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 1 {
		t.Fatalf("committed entry registers exactly one plugin; got %d: %v", len(plugins), plugins)
	}
	plugin := plugins[0]
	if plugin.Name != "committed-linux" || plugin.Description != "committed description" {
		t.Fatalf("published host/dirty metadata: %+v", plugin)
	}
	if plugin.SourceDir != "internal/shared" {
		t.Errorf("imported RunEngine origin = %q, want internal/shared", plugin.SourceDir)
	}
	if !reflect.DeepEqual(plugin.YANGFiles, []string{"internal/schema/external.yang"}) {
		t.Errorf("external schema origin = %v", plugin.YANGFiles)
	}
	for field, pair := range map[string][2][]string{
		"dependencies": {plugin.Dependencies, {"first", "base"}},
		"optional":     {plugin.OptionalDependencies, {"optional"}},
		"order":        {plugin.StartAfter, {"ordered"}},
		"roots":        {plugin.ConfigRoots, {"example"}},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			t.Errorf("%s = %v, want %v", field, pair[0], pair[1])
		}
	}
	if !plugin.HasYANG {
		t.Error("committed embedded YANG disappeared")
	}
}
