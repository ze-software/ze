// Design: docs/architecture/core-design.md -- native development-tool actions.
package gomodule

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"testing"
)

// TestModuleMoveGeneratorReachesNativeProducer proves the real post-move
// generator consumes the authored package tree, not an obsolete executable Go
// source file. No executable stand-in or injected generator answers this call.
func TestModuleMoveGeneratorReachesNativeProducer(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "go.mod", "module example.test/moved\n\ngo 1.26\n")
	writeFixture(t, root, "feature-gates.txt", "")
	writeFixture(t, root, "internal/plugins/moved/register.go", "package moved\n")
	writeFixture(t, root, generatedAll, "package all\n")
	writeFixture(t, root, pluginGenerator, "package pluginimports\n")

	if code := runPluginGenerator(root); code != 0 {
		t.Fatalf("native post-move generation exited %d", code)
	}
	generated, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, generatedAll), nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	if len(generated.Imports) != 1 {
		t.Fatalf("generated composition root imports = %v, want the moved plugin", generated.Imports)
	}
	path, err := strconv.Unquote(generated.Imports[0].Path.Value)
	if err != nil {
		t.Fatal(err)
	}
	if path != "example.test/moved/internal/plugins/moved" {
		t.Fatalf("generated import = %q, want the moved plugin", path)
	}
	if generated.Imports[0].Name == nil {
		t.Fatal("generated import has no blank identifier, so plugin init is not deliberately linked")
	}
	if generated.Imports[0].Name.Name != "_" {
		t.Fatalf("generated import name = %q, want blank import", generated.Imports[0].Name.Name)
	}
}
