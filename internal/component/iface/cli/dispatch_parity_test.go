package cli

import (
	"go/ast"
	"go/constant"
	"go/types"
	"path/filepath"
	"slices"
	"testing"

	"golang.org/x/tools/go/packages"
)

// VALIDATES: handover §5a -- ifaceCommands stays in sync with the dispatch
// switch in Run (main.go). The slice is the documented "single source of
// truth shared with the known-subcommand gate"; this test fails if the
// slice and the switch diverge.
// PREVENTS: silent drift where a new switch case is added (or removed)
// without updating ifaceCommands, leaving the known-subcommand gate,
// suggestion hints, and Meta.Subs stale.
func TestDispatchParity(t *testing.T) {
	// help and its flag aliases are handled by an early return before the
	// switch, so they are not switch cases. No exclusions are needed for
	// the switch in this package, but keep the set explicit for clarity.
	excluded := map[string]bool{
		"help": true, "-h": true, "--help": true,
	}

	switchCases := dispatchSwitchCases(t, "main.go", "Run")

	var wantCommands []string
	for _, c := range switchCases {
		if excluded[c] {
			continue
		}
		wantCommands = append(wantCommands, c)
	}
	slices.Sort(wantCommands)

	gotCommands := append([]string(nil), ifaceCommands...)
	slices.Sort(gotCommands)

	for _, c := range wantCommands {
		if !slices.Contains(gotCommands, c) {
			t.Errorf("switch case %q has no matching entry in ifaceCommands", c)
		}
	}
	for _, c := range gotCommands {
		if !slices.Contains(switchCases, c) {
			t.Errorf("ifaceCommands entry %q has no matching switch case in Run", c)
		}
	}
}

// dispatchSwitchCases type-checks the current package and returns the case
// values of the first switch statement found inside the named function of
// fileName. Cases like `case a, b:` contribute both values.
//
// A case value is any constant string expression: a literal, a constant
// declared in this package, or a selector into another package's constant
// (`command.VerbShow`). The type checker resolves each one, so the switch and
// ifaceCommands read from one set of constants: sharing a constant proves the
// two spell a command the same way, and it does not prove either list is
// complete, which is what this test is for. A case that is not a string
// constant fails the test rather than being skipped, because a silently
// dropped case would make the parity check vacuous.
func dispatchSwitchCases(t *testing.T, fileName, funcName string) []string {
	t.Helper()

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedTypes |
			packages.NeedSyntax | packages.NeedTypesInfo,
		Dir: ".",
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		t.Fatalf("load package: %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("load package returned %d packages, want 1", len(pkgs))
	}
	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		t.Fatalf("load package: %v", pkg.Errors)
	}

	var file *ast.File
	for _, syntax := range pkg.Syntax {
		if filepath.Base(pkg.Fset.File(syntax.Pos()).Name()) == fileName {
			file = syntax
			break
		}
	}
	if file == nil {
		t.Fatalf("file %s not found in package %s", fileName, pkg.PkgPath)
	}

	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		if d, ok := decl.(*ast.FuncDecl); ok && d.Name.Name == funcName {
			fn = d
			break
		}
	}
	if fn == nil {
		t.Fatalf("function %q not found in %s", funcName, fileName)
	}

	var cases []string
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		sw, ok := n.(*ast.SwitchStmt)
		if !ok || found {
			return true
		}
		found = true
		for _, stmt := range sw.Body.List {
			cc, ok := stmt.(*ast.CaseClause)
			if !ok || cc.List == nil { // nil List == default clause
				continue
			}
			for _, expr := range cc.List {
				tv, ok := pkg.TypesInfo.Types[expr]
				if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
					t.Fatalf("case %s in %q is not a string constant", types.ExprString(expr), funcName)
				}
				cases = append(cases, constant.StringVal(tv.Value))
			}
		}
		return false
	})
	if !found {
		t.Fatalf("no switch statement found in %q", funcName)
	}
	return cases
}
