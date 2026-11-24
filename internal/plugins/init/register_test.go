// Design: docs/architecture/cli/plugin-modes.md -- offline subcommand flag inventory

package init

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
)

// flagDecl records one flag declared on main.go's Run flag.FlagSet.
type flagDecl struct {
	name       string
	takesValue bool
}

// TestInitRegisteredFlagsMatchFlagSet verifies that every flag registered in
// register.go agrees with main.go's flag.FlagSet in name and value-taking kind.
//
// VALIDATES: Every value-taking flag on `ze init` (including `--sha256`) is
// registered with a non-FlagValueNone ValueHint, every boolean switch is
// registered with FlagValueNone, and the two sets hold the exact same flags.
// PREVENTS: Value-taking flags such as `--sha256` drifting to FlagValueNone so
// shell completion offers flags instead of waiting for a value.
func TestInitRegisteredFlagsMatchFlagSet(t *testing.T) {
	declared := parseInitFlagSetDecls(t, "main.go")
	if len(declared) == 0 {
		t.Fatal("parsed zero flags from main.go Run; AST scan found nothing")
	}

	registered := registry.CommandFlags("init")
	if len(registered) == 0 {
		t.Fatal("registry.CommandFlags(\"init\") returned no flags")
	}

	byName := make(map[string]registry.FlagSpec, len(registered))
	for _, spec := range registered {
		byName[spec.Name] = spec
	}

	declaredNames := make(map[string]bool, len(declared))
	for _, decl := range declared {
		tokenName := "--" + decl.name
		declaredNames[tokenName] = true
		spec, ok := byName[tokenName]
		if !ok {
			t.Errorf("flag %s declared in main.go is missing from register.go", tokenName)
			continue
		}
		if decl.takesValue {
			if spec.ValueHint == registry.FlagValueNone {
				t.Errorf("value flag %s registered with FlagValueNone; want a value hint", tokenName)
			}
			continue
		}
		if spec.ValueHint != registry.FlagValueNone {
			t.Errorf("bool flag %s registered with ValueHint %q; want FlagValueNone", tokenName, spec.ValueHint)
		}
	}

	for _, spec := range registered {
		if !declaredNames[spec.Name] {
			t.Errorf("flag %s registered in register.go is not declared in main.go", spec.Name)
		}
	}

	shaSpec, ok := byName["--sha256"]
	if !ok {
		t.Fatal("--sha256 is not registered for init")
	}
	if shaSpec.ValueHint != registry.FlagValueText {
		t.Errorf("--sha256 ValueHint = %q, want %q", shaSpec.ValueHint, registry.FlagValueText)
	}
}

// parseInitFlagSetDecls parses main.go and extracts every flag declared on the
// flag.FlagSet constructed inside Run. Any unrecognized method on that FlagSet
// fails the test rather than being skipped silently.
func parseInitFlagSetDecls(t *testing.T, filename string) []flagDecl {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", filename, err)
	}

	runBody := findFuncBody(file, "Run")
	if runBody == nil {
		t.Fatalf("%s has no func Run", filename)
	}

	flagSetVar := findFlagSetVar(runBody)
	if flagSetVar == "" {
		t.Fatalf("%s func Run does not construct a flag.NewFlagSet", filename)
	}

	var decls []flagDecl
	ast.Inspect(runBody, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if ident.Name != flagSetVar {
			return true
		}

		decl, matched := classifyFlagSetCall(t, sel.Sel.Name, call.Args)
		if matched {
			decls = append(decls, decl)
		}
		return true
	})
	return decls
}

// findFuncBody returns the body block of the named top-level function in file.
func findFuncBody(file *ast.File, name string) *ast.BlockStmt {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Recv != nil {
			continue
		}
		if fn.Name.Name == name {
			return fn.Body
		}
	}
	return nil
}

// findFlagSetVar finds the local variable initialized by flag.NewFlagSet in body.
func findFlagSetVar(body *ast.BlockStmt) string {
	var name string
	ast.Inspect(body, func(node ast.Node) bool {
		if name != "" {
			return false
		}
		assign, ok := node.(*ast.AssignStmt)
		if !ok {
			return true
		}
		if len(assign.Lhs) != 1 {
			return true
		}
		if len(assign.Rhs) != 1 {
			return true
		}
		if !isFlagNewFlagSetCall(assign.Rhs[0]) {
			return true
		}
		lhs, ok := assign.Lhs[0].(*ast.Ident)
		if !ok {
			return true
		}
		name = lhs.Name
		return false
	})
	return name
}

// isFlagNewFlagSetCall reports whether expr is a call to flag.NewFlagSet.
func isFlagNewFlagSetCall(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	if pkg.Name != "flag" {
		return false
	}
	return sel.Sel.Name == "NewFlagSet"
}

// classifyFlagSetCall maps one method call on a flag.FlagSet to a flagDecl.
// Non-registration calls (Parse, SetOutput, Init) return matched=false; an
// unknown method fails the test so new flag kinds cannot bypass coverage.
func classifyFlagSetCall(t *testing.T, method string, args []ast.Expr) (flagDecl, bool) {
	t.Helper()
	switch method {
	case "Parse", "SetOutput", "Init":
		return flagDecl{}, false
	case "Bool", "BoolFunc":
		return flagDecl{name: mustFlagNameArg(t, method, args, 0), takesValue: false}, true
	case "BoolVar":
		return flagDecl{name: mustFlagNameArg(t, method, args, 1), takesValue: false}, true
	case "String", "Int", "Int64", "Uint", "Uint64", "Float64", "Duration", "Func":
		return flagDecl{name: mustFlagNameArg(t, method, args, 0), takesValue: true}, true
	case "StringVar", "IntVar", "Int64Var", "UintVar", "Uint64Var", "Float64Var", "DurationVar", "TextVar", "Var":
		return flagDecl{name: mustFlagNameArg(t, method, args, 1), takesValue: true}, true
	default:
		t.Fatalf("unhandled flag.FlagSet method %q in main.go Run", method)
		return flagDecl{}, false
	}
}

// mustFlagNameArg extracts the string literal flag name at args[index].
func mustFlagNameArg(t *testing.T, method string, args []ast.Expr, index int) string {
	t.Helper()
	if index >= len(args) {
		t.Fatalf("flag.FlagSet.%s has %d args, need index %d", method, len(args), index)
	}
	lit, ok := args[index].(*ast.BasicLit)
	if !ok {
		t.Fatalf("flag.FlagSet.%s arg %d is not a string literal", method, index)
	}
	if lit.Kind != token.STRING {
		t.Fatalf("flag.FlagSet.%s arg %d is not a string literal", method, index)
	}
	name, err := strconv.Unquote(lit.Value)
	if err != nil {
		t.Fatalf("unquote flag.FlagSet.%s name %s: %v", method, lit.Value, err)
	}
	return name
}
