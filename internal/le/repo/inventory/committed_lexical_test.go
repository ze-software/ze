// Design: docs/architecture/plugin/feature-gates.md -- committed consumers share retained lexical ownership.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedLexicalBindingsPreserveInventory drives ReadCommitted through
// cross-file constants, import aliases, closure execution and mutation queries.
// A later commit must change only a new snapshot, including its refusal result.
func TestCommittedLexicalBindingsPreserveInventory(t *testing.T) {
	root := committedFixture(t)
	const directory = "internal/plugins/example"
	const source = directory + "/register_linux.go"
	write(t, root, source, `package example
import r "github.com/ze-software/ze/internal/component/plugin/registry"
import names "github.com/ze-software/ze/internal/shared"
import schema "github.com/ze-software/ze/internal/schema"
var metadata = r.Registration{ // metadata-declaration
 Name: lexicalName, Description: "lexical declaration", RunEngine: names.Engine,
 ConfigRoots: []string{names.Root}, YANG: schema.Document,
}
var initialized = install()
func install() int {
 register := func() { _ = r.Register(metadata) }
 register()
 return 1
}
func localShadow() { metadata := "local"; metadata = "changed"; _ = metadata }
`)
	write(t, root, directory+"/names.go", `package example
import remote "github.com/ze-software/ze/internal/shared"
const lexicalName = remote.Name
`)
	committedGit(t, root, "add", ".")
	committedGit(t, root, "-c", "user.name=Inventory Fixture", "-c", "user.email=inventory@example.invalid", "commit", "-m", "lexical consumer population")
	snapshot, err := ReadCommitted(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	check := func(reader *Snapshot) {
		t.Helper()
		plugins, err := reader.DeclaredPlugins()
		if err != nil {
			t.Fatal(err)
		}
		if len(plugins) != 1 {
			t.Fatalf("retained closure registration produced %d plugins: %+v", len(plugins), plugins)
		}
		plugin := plugins[0]
		if plugin.Name != "committed-linux" {
			t.Fatalf("cross-file constant resolved to %q", plugin.Name)
		}
		if plugin.Description != "lexical declaration" {
			t.Fatalf("retained metadata description is %q", plugin.Description)
		}
		if plugin.SourceDir != "internal/shared" {
			t.Fatalf("imported engine origin is %q", plugin.SourceDir)
		}
		if len(plugin.YANGFiles) != 1 {
			t.Fatalf("retained schema provenance is %v", plugin.YANGFiles)
		}
		if plugin.YANGFiles[0] != "internal/schema/external.yang" {
			t.Fatalf("retained schema origin is %v", plugin.YANGFiles)
		}
	}
	check(snapshot)
	file, identifier := lexicalIdent(t, snapshot, source, "metadata-declaration", "metadata", 0)
	declaration, known := snapshot.binding(file, identifier)
	if !known {
		t.Fatal("ReadCommitted returned without its lexical index")
	}
	input := sourceExpr{value: identifier, file: file, pkg: snapshot.packages[directory]}
	if err := snapshot.evaluator().immutable(input); err != nil {
		t.Fatalf("a local shadow wrote the package declaration: %v", err)
	}
	write(t, root, directory+"/mutate.go", "package example\nfunc mutate() { metadata.Name = \"changed\" }\n")
	committedGit(t, root, "add", ".")
	committedGit(t, root, "-c", "user.name=Inventory Fixture", "-c", "user.email=inventory@example.invalid", "commit", "-m", "later lexical mutation")
	check(snapshot)
	if retained, held := snapshot.binding(file, identifier); !held || retained != declaration {
		t.Fatal("HEAD advance changed a retained declaration identity")
	}
	fresh, err := ReadCommitted(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, held := fresh.binding(file, identifier); held {
		t.Fatal("new snapshot accepted the earlier snapshot's identifier")
	}
	_, err = fresh.DeclaredPlugins()
	if err == nil {
		t.Fatal("fresh snapshot certified mutated registration metadata")
	}
	if !strings.Contains(err.Error(), "mutated") {
		t.Fatalf("fresh snapshot lost its mutation cause: %v", err)
	}
}
