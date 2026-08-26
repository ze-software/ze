// The composition root is one blank import per tool, so the only thing to
// check about it is that the imports and the registrations agree, and that a
// name cannot be owned twice.

package main

import (
	"errors"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/letools/leroot"
)

// rootsAtStart is every root command LE OWNS by the time this package's
// variables initialize, which is after every imported tool's init() and before
// any test runs. Later tests register probes of their own, so the set has to be
// taken here rather than inside a test.
//
// It is leroot.Commands rather than registry.ListRoot because le links the
// product: tools that introspect ze load ze's registry to read it, so this
// process's registry carries ze's root commands beside le's (letools/leroot,
// Owned).
var rootsAtStart = leroot.Commands()

// TestEveryPackageRegistersOneRootHandler holds the composition root to its
// one job: every tool it imports is reachable, and every reachable tool was
// imported here. A registration that came from somewhere else is a tool le
// carries without saying so.
func TestEveryPackageRegistersOneRootHandler(t *testing.T) {
	imported := toolImports(t)
	if len(imported) == 0 {
		t.Fatal("cmd/le/register.go imports no tool package")
	}

	roots := rootsAtStart
	if len(roots) != len(imported) {
		names := make([]string, 0, len(roots))
		for _, rc := range roots {
			names = append(names, rc.Name)
		}
		t.Errorf("register.go imports %d tool packages (%v) and %d root commands are registered (%v)",
			len(imported), imported, len(roots), names)
	}
}

// toolImports answers the le tool packages cmd/le/register.go blank-imports.
func toolImports(t *testing.T) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "register.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse register.go: %v", err)
	}

	var paths []string
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatalf("import path %s: %v", spec.Path.Value, err)
		}
		if !strings.HasPrefix(path, leTree) {
			t.Errorf("cmd/le/register.go imports %s, which is not an le tool package", path)
			continue
		}
		if spec.Name == nil || spec.Name.Name != "_" {
			t.Errorf("cmd/le/register.go imports %s by name: the composition root only registers", path)
		}
		paths = append(paths, path)
	}
	return paths
}

// TestNoLeNameCollidesWithZe is A-5's failure mode, made real. The two
// binaries are never linked together, so a name le and ze both use costs
// nothing (`le perf` and `ze perf` can coexist). What matters is what happens
// IF the never-linked rule is ever broken: the registry refuses the second
// owner, loudly, at init, rather than letting one shadow the other.
func TestNoLeNameCollidesWithZe(t *testing.T) {
	const name = "collision-probe"
	handler := func(*registry.RuntimeContext, []string) int { return 0 }
	meta := registry.Meta{Description: "a test probe", Mode: "offline", Section: registry.SectionTest}

	if err := registry.RegisterRootHandler(name, handler, meta); err != nil {
		t.Fatalf("first registration of %q: %v", name, err)
	}

	err := registry.RegisterRootHandler(name, handler, meta)
	if err == nil {
		t.Fatalf("a second owner of %q was accepted: one shadows the other and nothing says so", name)
	}
	if !errors.Is(err, registry.ErrRootHandlerDuplicate) {
		t.Errorf("a duplicate name was refused with %v, want ErrRootHandlerDuplicate", err)
	}

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Error("MustRegisterRootHandler accepted a duplicate name: the failure must stop the process")
		}
	}()
	registry.MustRegisterRootHandler(name, handler, meta)
}
