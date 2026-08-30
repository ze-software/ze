package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"sort"
	"testing"
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
	sort.Strings(wantCommands)

	gotCommands := append([]string(nil), ifaceCommands...)
	sort.Strings(gotCommands)

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

// dispatchSwitchCases parses fileName in the current package and returns the
// case values of the first switch statement found inside the named function.
// Cases like `case a, b:` contribute both values.
//
// A case value is either a string literal or the name of a string constant
// declared in the same file, and a name is resolved through that declaration.
// Resolving is what lets the switch and ifaceCommands read from one set of
// constants: sharing a constant proves the two spell a command the same way,
// and it does not prove either list is complete, which is what this test is
// for. An unresolvable name fails the test rather than being skipped, because
// a silently dropped case would make the parity check vacuous.
func dispatchSwitchCases(t *testing.T, fileName, funcName string) []string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, fileName, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", fileName, err)
	}

	constValues := stringConstants(t, file)

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
				switch e := expr.(type) {
				case *ast.BasicLit:
					if e.Kind != token.STRING {
						continue
					}
					cases = append(cases, mustUnquote(t, e.Value))
				case *ast.Ident:
					value, ok := constValues[e.Name]
					if !ok {
						t.Fatalf("case %q in %q is not a string constant declared in %s", e.Name, funcName, fileName)
					}
					cases = append(cases, value)
				}
			}
		}
		return false
	})
	if !found {
		t.Fatalf("no switch statement found in %q", funcName)
	}
	return cases
}

// stringConstants returns every untyped string constant declared at the top
// level of file, keyed by name. Only a `name = "literal"` spec is collected: an
// iota or an expression is not a spelling this test can compare, and leaving it
// out makes dispatchSwitchCases fail loudly on it.
func stringConstants(t *testing.T, file *ast.File) map[string]string {
	t.Helper()

	values := make(map[string]string)
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Names) != len(vs.Values) {
				continue
			}
			for i, name := range vs.Names {
				lit, ok := vs.Values[i].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				values[name.Name] = mustUnquote(t, lit.Value)
			}
		}
	}
	return values
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
