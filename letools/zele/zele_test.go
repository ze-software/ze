// These cases verify that ze runs le commands through one engine and root.
// A probe registers like every le tool, then the crossing dispatches it through `run`.
// The tests also compare the two composition lists file by file.

package zele

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/letools/lepath"
	"github.com/ze-software/ze/letools/leroot"
)

// blankImports answers the import paths a Go file blank-imports, which is what
// a composition root is made of.
func blankImports(t *testing.T, path string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	var paths []string
	for _, spec := range file.Imports {
		if spec.Name == nil || spec.Name.Name != "_" {
			continue
		}
		imported, unquoteErr := strconv.Unquote(spec.Path.Value)
		if unquoteErr != nil {
			t.Fatalf("import path %s in %s: %v", spec.Path.Value, path, unquoteErr)
		}
		paths = append(paths, imported)
	}
	slices.Sort(paths)
	return paths
}

// TestTheCrossingLinksEveryToolLeCarries verifies that `ze le` and `le` expose the same tools.
// Two composition lists remain because thirty letools/*/register.go headers name le's root.
// This test requires both lists to match so a tool cannot exist in only one program.
func TestTheCrossingLinksEveryToolLeCarries(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find the checkout: %v", err)
	}

	le := blankImports(t, filepath.Join(root, "cmd", "le", "register.go"))
	crossing := blankImports(t, filepath.Join(root, "letools", "zele", "tools.go"))
	if len(le) == 0 {
		t.Fatal("cmd/le/register.go blank-imports nothing: the comparison would prove nothing")
	}

	for _, path := range le {
		if !slices.Contains(crossing, path) {
			t.Errorf("le carries %s and the crossing does not: add its blank import to letools/zele/tools.go", path)
		}
	}
	for _, path := range crossing {
		if !slices.Contains(le, path) {
			t.Errorf("the crossing carries %s and le does not: remove it from letools/zele/tools.go, or add it to cmd/le/register.go", path)
		}
	}
}

// TestTheCrossingClaimsOneRootName pins the shape: ze gains the single name
// `le` and sub-dispatches under it. The name is ze's, not le's, so it is
// absent from le's own command set -- `le le` is an unknown command.
func TestTheCrossingClaimsOneRootName(t *testing.T) {
	if !registry.HasRootHandler(name) {
		t.Fatalf("the crossing registered no %q root: a ze_le build carries no development command", name)
	}
	if leroot.Owns(name) {
		t.Errorf("%q is registered as one of le's own commands: it is ze's root for them, not one of them", name)
	}

	found := false
	for _, rc := range registry.ListRoot() {
		if rc.Name != name {
			continue
		}
		found = true
		if rc.Meta.Description == "" || rc.Meta.Mode == "" || rc.Meta.Section == "" {
			t.Errorf("%q registered Description=%q Mode=%q Section=%q: all three are required",
				name, rc.Meta.Description, rc.Meta.Mode, rc.Meta.Section)
		}
	}
	if !found {
		t.Errorf("%q has a handler and is not listed: it would appear in no help page", name)
	}
}

// TestTheCrossingDispatchesAnLeCommand drives the whole sub-dispatch: the words
// after the root reach the tool unchanged, and the tool's exit code is what ze
// answers. A flattened code is a behavior change, because a gate reading 3 from
// a command that exited 3 is the contract every caller depends on (AC-8).
func TestTheCrossingDispatchesAnLeCommand(t *testing.T) {
	const probe = "crossing-probe"
	var got []string
	leroot.Register(probe, func(args []string) (any, int) {
		got = args
		return map[string]string{"probe": "ran"}, 3
	}, registry.Meta{Description: "a test probe", Mode: "offline", Section: registry.SectionTest})

	if code := run(nil, []string{probe, "alpha", "beta"}); code != 3 {
		t.Errorf("`ze le %s` answered %d, want the tool's 3", probe, code)
	}
	if slices.Compare(got, []string{"alpha", "beta"}) != 0 {
		t.Errorf("the tool was called with %q, want [alpha beta]", got)
	}
}

// TestTheCrossingRefusesAWordLeDoesNotOwn: a name nobody registered is the
// caller's error, and guessing at it would run something they did not ask for.
func TestTheCrossingRefusesAWordLeDoesNotOwn(t *testing.T) {
	if code := run(nil, []string{"no-such-tool"}); code != 1 {
		t.Errorf("`ze le no-such-tool` answered %d, want 1", code)
	}
}

// TestTheCrossingRefusesZesOwnCommands covers the boundary that tool-name comparisons cannot prove.
// Some le tools link product packages, which register ze root commands in this process.
// `ze le interface` must remain unknown instead of exposing another route to ze's interface editor.
func TestTheCrossingRefusesZesOwnCommands(t *testing.T) {
	product := 0
	for _, rc := range registry.ListRoot() {
		if rc.Name == name || leroot.Owns(rc.Name) {
			continue
		}
		product++
		t.Run(rc.Name, func(t *testing.T) {
			if code := run(nil, []string{rc.Name}); code != 1 {
				t.Errorf("`ze le %s` answered %d: the crossing ran a command that is not le's", rc.Name, code)
			}
		})
	}

	if product == 0 {
		t.Fatal("no command outside le's set is registered in this process, so this test proves nothing: " +
			"le's tools link the product so they can introspect it")
	}
}
