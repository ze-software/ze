// Design: docs/architecture/plugin/feature-gates.md -- the generated gated-plugin table
package gatedtable

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

	"github.com/ze-software/ze/internal/component/plugin/registry"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	pluginimports "github.com/ze-software/ze/internal/le/plugin/imports"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
	repoinventory "github.com/ze-software/ze/internal/le/repo/inventory"
)

// registryImport is the package a plugin registers through.
const registryImport = "github.com/ze-software/ze/internal/component/plugin/registry"

// repositoryRoot answers the checkout these tests read.
func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// VALIDATES: the committed table is exactly what the manifest and every
// registration owe (AC-18 on the real tree), and the binary that derived it saw
// every gated registration (A-2).
//
// Method: run Check over the checkout and require no stale reason. Then walk
// every feature-gates.txt package, find each one whose own Go source calls
// registry.Register, and require a table row from that package. The second half
// is what A-2 rests on: the table comes from registrations compiled into the
// running binary, so a binary built without some gate tag would omit that
// gate's plugins and Check alone, comparing against the same blind binary,
// could not see it. The source walk does not depend on build tags.
func TestGatedPluginTableMatchesManifest(t *testing.T) {
	root := repositoryRoot(t)

	report, err := Check(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.Reason != "" {
		t.Fatalf("%s %s: run ./le plugin gated-table write", report.File, report.Reason)
	}
	if len(report.Rows) == 0 {
		t.Fatal("the table holds no gated plugin, so the test binary linked no gated registration")
	}

	fromSource := map[string]bool{}
	for _, row := range report.Rows {
		fromSource[row.SourceDir] = true
	}

	gates, err := repofeaturetags.Gates(root)
	if err != nil {
		t.Fatal(err)
	}
	registering := 0
	for _, gate := range gates {
		if !packageRegistersPlugin(t, filepath.Join(root, filepath.FromSlash(gate.Package))) {
			continue
		}
		registering++
		if !fromSource[gate.Package] {
			t.Errorf("%s (%s) calls registry.Register but no table row comes from it: the binary did not link it", gate.Package, gate.Tag)
		}
	}
	if registering == 0 {
		t.Fatal("no gated package registers a plugin, so the source walk proved nothing")
	}
}

// VALIDATES: a nested gate yields the compound requirement (AC-19), both in the
// derived rows and in the table compiled into this binary.
//
// Method: bgp-bmp lives in internal/component/bgp/plugins/bmp under ze_bmp,
// inside the ze_bgp tree, so it needs both tags, enclosing gate first.
func TestNestedGateYieldsCompoundRequirement(t *testing.T) {
	rows, err := Derive(repositoryRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ze_bgp", "ze_bmp"}

	at := slices.IndexFunc(rows, func(r Row) bool { return r.Name == "bgp-bmp" })
	if at < 0 {
		t.Fatal("bgp-bmp is not a gated plugin row")
	}
	if !slices.Equal(rows[at].Tags, want) {
		t.Errorf("derived bgp-bmp tags = %v, want %v", rows[at].Tags, want)
	}

	compiled, gated := registry.GatedPluginTags("bgp-bmp")
	if !gated {
		t.Fatal("registry.GatedPluginTags answers bgp-bmp as ungated")
	}
	if !slices.Equal(compiled, want) {
		t.Errorf("compiled bgp-bmp tags = %v, want %v", compiled, want)
	}
}

// VALIDATES: a gate added to feature-gates.txt without regenerating the table
// makes the check report it stale (AC-18, R-3).
//
// Method: copy the real manifest into a scratch tree, add a gate over the
// package of one plugin that is ungated today, derive the rows over the real
// registrations, and judge the committed table against them with the same
// verdict function Check uses. The new gate must produce a row for that plugin,
// and the committed table must then read as stale.
func TestNewGateWithoutRegenerationIsStale(t *testing.T) {
	root := repositoryRoot(t)

	plugins, err := repoinventory.Plugins(root)
	if err != nil {
		t.Fatal(err)
	}
	current, err := pluginimports.LoadGateIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	at := slices.IndexFunc(plugins, func(p repoinventory.Plugin) bool { return len(current.Requirement(p.SourceDir)) == 0 })
	if at < 0 {
		t.Fatal("every plugin is gated, so no ungated one can gain a gate")
	}
	ungated := plugins[at]

	// The manifest is re-rendered from its parsed rows rather than copied, so
	// repofeaturetags stays the only Go reader of feature-gates.txt.
	rowsDeclared, err := repofeaturetags.Gates(root)
	if err != nil {
		t.Fatal(err)
	}
	var widened strings.Builder
	for _, gate := range rowsDeclared {
		widened.WriteString(gate.Tag + " " + gate.Package + "\n")
	}
	widened.WriteString("ze_testgate " + ungated.SourceDir + "\n")
	scratch := t.TempDir()
	if err := os.WriteFile(filepath.Join(scratch, "feature-gates.txt"), []byte(widened.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	gates, err := pluginimports.LoadGateIndex(scratch)
	if err != nil {
		t.Fatal(err)
	}

	rows := rowsOf(gates, plugins)
	got := slices.IndexFunc(rows, func(r Row) bool { return r.Name == ungated.Name })
	if got < 0 {
		t.Fatalf("the new gate over %s produced no row for %s", ungated.SourceDir, ungated.Name)
	}
	if !slices.Contains(rows[got].Tags, "ze_testgate") {
		t.Errorf("%s tags = %v, want ze_testgate among them", ungated.Name, rows[got].Tags)
	}

	want, err := render(rows)
	if err != nil {
		t.Fatal(err)
	}
	reason, err := verdictOf(filepath.Join(root, filepath.FromSlash(tableFile)), want)
	if err != nil {
		t.Fatal(err)
	}
	if reason != reasonStale {
		t.Errorf("verdict = %q, want %q", reason, reasonStale)
	}
}

// VALIDATES: a table that was never generated is reported missing, not current.
func TestAbsentTableIsMissing(t *testing.T) {
	reason, err := verdictOf(filepath.Join(t.TempDir(), "gated_plugins_gen.go"), []byte("package registry\n"))
	if err != nil {
		t.Fatal(err)
	}
	if reason != reasonMissing {
		t.Errorf("verdict = %q, want %q", reason, reasonMissing)
	}
}

// packageRegistersPlugin answers whether any non-test Go file directly in dir
// calls Register on the plugin registry, under whatever name it imports it.
func packageRegistersPlugin(t *testing.T, dir string) bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if fileRegistersPlugin(t, filepath.Join(dir, name)) {
			return true
		}
	}
	return false
}

// fileRegistersPlugin answers whether one Go file calls <registry>.Register.
func fileRegistersPlugin(t *testing.T, path string) bool {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	local := ""
	for _, spec := range file.Imports {
		imported, unquoteErr := strconv.Unquote(spec.Path.Value)
		if unquoteErr != nil {
			t.Fatal(unquoteErr)
		}
		if imported != registryImport {
			continue
		}
		local = "registry"
		if spec.Name != nil {
			local = spec.Name.Name
		}
	}
	if local == "" {
		return false
	}
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}
		selector, isSelector := call.Fun.(*ast.SelectorExpr)
		if !isSelector {
			return true
		}
		receiver, isIdent := selector.X.(*ast.Ident)
		if isIdent && receiver.Name == local && selector.Sel.Name == "Register" {
			found = true
		}
		return true
	})
	return found
}
