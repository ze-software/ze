package schema

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"sort"
	"testing"
)

// VALIDATES: handover §5a -- schemaCommands stays in sync with the dispatch
// switch in Run (main.go). The slice is hand-maintained "kept in sync with
// the switch cases in Run"; this test fails if the two diverge.
// PREVENTS: silent drift where a new switch case is added (or removed)
// without updating schemaCommands, leaving Meta.Subs and the unknown-command
// suggestion list stale.
func TestDispatchParity(t *testing.T) {
	// Switch cases intentionally NOT listed in schemaCommands, per the
	// documented design in register.go:
	//   - help / -h / --help: universal, not a discoverable subcommand.
	//   - show: takes a module argument ("ze schema show <module>") and is
	//     deliberately excluded from Meta.Subs.
	excluded := map[string]bool{
		"help": true, "-h": true, "--help": true,
		"show": true,
	}

	switchCases := dispatchSwitchCases(t, "main.go", "Run")

	var wantCommands []string
	for _, c := range switchCases {
		if excluded[c] {
			continue
		}
		wantCommands = append(wantCommands, c)
	}
	sort.Strings(wantCommands)

	gotCommands := append([]string(nil), schemaCommands...)
	sort.Strings(gotCommands)

	for _, c := range wantCommands {
		if !slices.Contains(gotCommands, c) {
			t.Errorf("switch case %q has no matching entry in schemaCommands", c)
		}
	}
	for _, c := range gotCommands {
		if !slices.Contains(switchCases, c) {
			t.Errorf("schemaCommands entry %q has no matching switch case in Run", c)
		}
	}
}

// dispatchSwitchCases parses fileName in the current package and returns the
// string-literal case values of the first switch statement found inside the
// named function. Cases like `case "a", "b":` contribute both "a" and "b".
func dispatchSwitchCases(t *testing.T, fileName, funcName string) []string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, fileName, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", fileName, err)
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
				lit, ok := expr.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				cases = append(cases, mustUnquote(t, lit.Value))
			}
		}
		return false
	})
	if !found {
		t.Fatalf("no switch statement found in %q", funcName)
	}
	return cases
}

// mustUnquote strips the surrounding double quotes from a Go string literal.
func mustUnquote(t *testing.T, lit string) string {
	t.Helper()
	if len(lit) >= 2 && lit[0] == '"' && lit[len(lit)-1] == '"' {
		return lit[1 : len(lit)-1]
	}
	t.Fatalf("unexpected non-quoted string literal %q", lit)
	return ""
}
