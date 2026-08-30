package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"
)

// VALIDATES: handover §5a -- bgpCommands stays in sync with the dispatch
// switch in Run (main.go). The slice is hand-maintained "kept in sync with
// the switch"; this test fails if the two diverge.
// PREVENTS: silent drift where a new switch case is added (or removed)
// without updating bgpCommands, leaving Meta.Subs / suggestion hints stale.
func TestDispatchParity(t *testing.T) {
	// Cases handled by the switch that are intentionally NOT user-facing
	// subcommands listed in bgpCommands: help and its flag aliases.
	excluded := map[string]bool{
		"help": true, "-h": true, "--help": true,
	}

	switchCases := dispatchSwitchCases(t, "main.go", "Run", packageStringConsts(t))

	var wantCommands []string
	for _, c := range switchCases {
		if excluded[c] {
			continue
		}
		wantCommands = append(wantCommands, c)
	}
	sort.Strings(wantCommands)

	gotCommands := append([]string(nil), bgpCommands...)
	sort.Strings(gotCommands)

	// Every switch case (minus exclusions) must appear in the slice.
	for _, c := range wantCommands {
		if !slices.Contains(gotCommands, c) {
			t.Errorf("switch case %q has no matching entry in bgpCommands", c)
		}
	}
	// Every slice entry must have a matching switch case.
	for _, c := range gotCommands {
		if !slices.Contains(switchCases, c) {
			t.Errorf("bgpCommands entry %q has no matching switch case in Run", c)
		}
	}
}

// packageStringConsts returns every `const name = "value"` the package's
// non-test files declare. The dispatch switch names its cases through those
// constants, so the parity check has to resolve them before it can compare the
// switch against bgpCommands.
func packageStringConsts(t *testing.T) map[string]string {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}

	consts := make(map[string]string)
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fset, name, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", name, parseErr)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok || len(value.Names) != len(value.Values) {
					continue
				}
				for i, ident := range value.Names {
					lit, ok := value.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					consts[ident.Name] = mustUnquote(t, lit.Value)
				}
			}
		}
	}
	return consts
}

// dispatchSwitchCases parses fileName in the current package and returns the
// case values of the first switch statement found inside the named function.
// Cases like `case "a", "b":` contribute both "a" and "b". A case named by an
// identifier is resolved through consts, and an identifier that is not a
// package string constant fails the test rather than being skipped.
func dispatchSwitchCases(t *testing.T, fileName, funcName string, consts map[string]string) []string {
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
				switch e := expr.(type) {
				case *ast.BasicLit:
					if e.Kind == token.STRING {
						cases = append(cases, mustUnquote(t, e.Value))
					}
				case *ast.Ident:
					value, ok := consts[e.Name]
					if !ok {
						t.Errorf("case %s in %s is not a package string constant", e.Name, funcName)
						continue
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

// mustUnquote strips the surrounding double quotes from a Go string literal.
func mustUnquote(t *testing.T, lit string) string {
	t.Helper()
	if len(lit) >= 2 && lit[0] == '"' && lit[len(lit)-1] == '"' {
		return lit[1 : len(lit)-1]
	}
	t.Fatalf("unexpected non-quoted string literal %q", lit)
	return ""
}
