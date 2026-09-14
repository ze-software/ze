// Related: pluginimports.go -- pluginDirs, the policy this test keeps honest

package pluginimports

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
)

// treeRoot walks up from the working directory to the checkout that holds
// go.mod. The package's other tests build a fixture tree; this one judges the
// real one, because the thing it guards is a directory somebody adds.
func treeRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("read the working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod above %s", dir)
		}
		dir = parent
	}
}

// VALIDATES: every internal/component/<domain>/plugins directory in the tree is
// named in pluginDirs.
// PREVENTS: a new component plugins subtree that the composition root never
// walks, so its plugins are absent from the built binary with nothing red.
//
// pluginDirs is a policy and stays written out: the tier gate and the
// process-boundary gate judge the tree against it, so a list derived from the
// tree would agree with the tree by construction. This test is the other half
// of that choice. It does not derive the policy; it refuses to let the tree
// hold a plugins root the policy has not decided about.
//
// It replaced nestedPluginDomains, which derived internal/component/<domain>/plugins
// for a hand-written pair of domains. That is a list of the same kind wearing a
// derivation, and it had already gone stale in both directions: it named "ike",
// which has no plugins subtree, while l2tp/plugins was reachable only through it.
func TestEveryComponentPluginsTreeIsNamed(t *testing.T) {
	root := treeRoot(t)
	componentDir := filepath.Join(root, "internal", "component")

	entries, err := os.ReadDir(componentDir)
	if err != nil {
		t.Fatalf("read %s: %v", componentDir, err)
	}

	declared := pluginSearchRoots()
	found := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		rel := filepath.ToSlash(filepath.Join("internal", "component", entry.Name(), "plugins"))
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			continue
		}
		found++
		if !slices.Contains(declared, rel) {
			t.Errorf("%s holds plugins and no search root names it, so the composition root never walks it; add it to pluginDirs", rel)
		}
	}

	if found == 0 {
		t.Fatal("no internal/component/<domain>/plugins directory was found, so this test asserted nothing")
	}
}

// dispatchRoot is the ze_core composition root that hand-lists the packages
// whose init() the binary needs. It is the file this generator exists to empty.
const dispatchRoot = "cmd/ze/ze_core_dispatch.go"

// handListedImports answers the module-internal blank imports of one file,
// which is the hand list the generated composition root has to cover.
func handListedImports(t *testing.T, root, rel string) []string {
	t.Helper()

	module, err := lepath.Module(root)
	if err != nil {
		t.Fatalf("read the module path: %v", err)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Join(root, rel), nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse %s: %v", rel, err)
	}

	var hand []string
	for _, spec := range file.Imports {
		if spec.Name == nil || spec.Name.Name != "_" {
			continue
		}
		path, unquoteErr := strconv.Unquote(spec.Path.Value)
		if unquoteErr != nil {
			t.Fatalf("unquote the import %s in %s: %v", spec.Path.Value, rel, unquoteErr)
		}
		if !strings.HasPrefix(path, module+"/internal/") {
			continue
		}
		hand = append(hand, path)
	}
	slices.Sort(hand)

	return hand
}

// derivedImports answers every package the derivation names, universal and
// gated alike, as one set.
func derivedImports(t *testing.T, root string) map[string]bool {
	t.Helper()

	found, err := derive(root)
	if err != nil {
		t.Fatalf("derive over the checkout: %v", err)
	}

	named := map[string]bool{}
	for _, group := range [][]string{found.Plugins, found.Schemas, found.RPCs, found.Namespaces, found.RootHandlers} {
		for _, imp := range group {
			named[imp] = true
		}
	}
	for _, group := range found.ByTag {
		for _, imp := range group {
			named[imp] = true
		}
	}

	return named
}

// compositionRootDir is the directory name every composition root in the tree
// carries. internal/component/plugin/all is the generated one and
// internal/component/aaa/all the hand-maintained AAA one, which registers
// backends rather than a command, so no discovery kind reaches it.
const compositionRootDir = "all"

// VALIDATES: every package the ze_core dispatch root blank-imports by hand is
// either named by the derivation, or one the derivation CANNOT name for a
// reason the tree itself states.
// PREVENTS: the one operator-visible failure this generator can cause. A
// package dropped from the derivation and from the hand list registers nothing,
// `ze <root>` answers "unknown command", and every gate stays green because the
// binary still builds.
//
// Three reasons excuse a hand import, and each is read out of the tree rather
// than written here. The package is a composition root. It reaches the
// composition root, so naming it there is a cycle the compiler refuses. Or it
// carries codegen:skip, which is the tree's one way of declaring that another
// root owns it, with a reason beside the marker.
//
// A hand import that meets none of them, and that the derivation DOES name, is
// redundant: the generated root already links it. The test refuses that too, so
// the list cannot grow back one line at a time.
func TestGeneratedImportsCoverTodaysHandList(t *testing.T) {
	root := treeRoot(t)

	module, err := lepath.Module(root)
	if err != nil {
		t.Fatalf("read the module path: %v", err)
	}

	cycling, err := compositionRootImporters(root, module)
	if err != nil {
		t.Fatalf("read the import graph: %v", err)
	}

	hand := handListedImports(t, root, dispatchRoot)
	if len(hand) == 0 {
		t.Fatalf("%s holds no module-internal blank import, so this test asserted nothing", dispatchRoot)
	}

	named := derivedImports(t, root)
	if len(named) < len(hand) {
		t.Fatalf("the derivation names %d packages against %d hand imports, so it read almost nothing", len(named), len(hand))
	}

	for _, imp := range hand {
		dir := filepath.Join(root, strings.TrimPrefix(imp, module+"/"))

		if filepath.Base(dir) == compositionRootDir {
			continue
		}
		if cycling[imp] {
			continue
		}
		skips, skipErr := pkgSkipsCodegen(dir)
		if skipErr != nil {
			t.Fatalf("read the codegen marker of %s: %v", imp, skipErr)
		}
		if skips {
			continue
		}

		if !named[imp] {
			t.Errorf("%s hand-imports %s, the derivation does not name it, and nothing in the tree says why; `ze` loses the commands it registers the moment the line goes", dispatchRoot, imp)

			continue
		}
		t.Errorf("%s hand-imports %s and the generated composition root already names it; delete the line", dispatchRoot, imp)
	}
}

// commandRegistryDir declares the registry whose registrars commandRegistrars
// matches. It is the anchor: a directory that stops resolving fails this test
// rather than leaving it asserting nothing.
const commandRegistryDir = "internal/component/command/registry"

// notACommandRegistrar names the exported registrars of that package which
// register something OTHER than a command, so a package calling only one of
// them owns no command and needs no blank import.
//
// This is a list of DECISIONS rather than a copy of the package's contents,
// which is the distinction the enumeration gate itself turns on: each row is a
// judgement a reader can check against the function, and a registrar this list
// does not name is one a command owner can be reached through.
var notACommandRegistrar = map[string]string{
	// RegisterCommandFlags records the flags an EXISTING command path accepts
	// (internal/component/command/registry/flags.go). It creates no command, so
	// a package calling only it registers nothing to dispatch to.
	"RegisterCommandFlags": "records the flags of a command another registrar declared",
}

// VALIDATES: commandRegistrars names every exported registrar of
// internal/component/command/registry that can declare a command.
// PREVENTS: the silent half of the fifth discovery kind. discoverRootHandlers
// matches registrar calls as TEXT, so a registrar added to that package and not
// added here makes every owner using it invisible: the package links nowhere,
// `ze <command>` answers "unknown command", and the build, the lint and every
// gate stay green. This test is what turns that silence into a red.
func TestEveryCommandRegistrarIsMatched(t *testing.T) {
	root := treeRoot(t)
	dir := filepath.Join(root, filepath.FromSlash(commandRegistryDir))

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", commandRegistryDir, err)
	}

	matched := make(map[string]bool, len(commandRegistrars))
	for _, call := range commandRegistrars {
		matched[strings.TrimSuffix(strings.TrimPrefix(call, "."), "(")] = true
	}

	fset := token.NewFileSet()
	registrars := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parse %s/%s: %v", commandRegistryDir, name, parseErr)
		}
		for _, decl := range file.Decls {
			fn, isFunc := decl.(*ast.FuncDecl)
			if !isFunc || fn.Recv != nil || !fn.Name.IsExported() {
				continue
			}
			if !strings.HasPrefix(fn.Name.Name, "Register") && !strings.HasPrefix(fn.Name.Name, "MustRegister") {
				continue
			}
			registrars++
			if matched[fn.Name.Name] {
				continue
			}
			if why, declared := notACommandRegistrar[fn.Name.Name]; declared {
				t.Logf("%s is not a command registrar: %s", fn.Name.Name, why)

				continue
			}
			t.Errorf("%s.%s registers a command and commandRegistrars does not match it; every owner calling it links nowhere and `ze <command>` answers unknown command",
				commandRegistryDir, fn.Name.Name)
		}
	}

	if registrars == 0 {
		t.Fatalf("no exported registrar was found in %s, so this test asserted nothing", commandRegistryDir)
	}
}
