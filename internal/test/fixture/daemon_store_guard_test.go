package fixture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestNoFixtureAppendsAConfigRootOntoTheInheritedEnvironment refuses the
// shape that let a fixture's seed and its daemon resolve two roots:
// append(os.Environ(), "ZE_CONFIG_DIR="+x). The runner hands every child a
// dotted ze.config.dir and env.Get prefers a dotted spelling, so an appended
// root, under any spelling, need not be the one the child reads.
// Method: parse every non-test Go file of this package and report each
// append whose first argument is os.Environ() and whose later argument
// starts with a ze.config.dir assignment, written as a literal or through
// envConfigDir / envConfigDirDotted. daemon_store.go is the one place that
// builds a root environment, and it does so with childEnvironment.
func TestNoFixtureAppendsAConfigRootOntoTheInheritedEnvironment(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	scanned := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") || name == "daemon_store.go" {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		scanned++
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || !isIdent(call.Fun, "append") || len(call.Args) < 2 || !isOSEnviron(call.Args[0]) {
				return true
			}
			for _, arg := range call.Args[1:] {
				if assignsConfigRoot(arg) {
					t.Errorf("%s: append(os.Environ(), %s...) sets the config root beside the inherited dotted one: use daemonRootEnvironment or daemonEnvironment (daemon_store.go)", fset.Position(arg.Pos()), environmentKeyReading(envConfigDirDotted))
				}
			}
			return true
		})
	}
	// MUTATION: skip every file, or match only the dotted literal, and the
	// scan still passes here; the self-check below then goes red.
	if scanned == 0 {
		t.Fatal("no fixture source was scanned")
	}
	for _, source := range []string{
		`append(os.Environ(), "ZE_CONFIG_DIR="+dir)`,
		`append(os.Environ(), "x=1", "ze.config.dir="+dir)`,
		`append(os.Environ(), envConfigDir+"="+dir)`,
		`append(os.Environ(), envConfigDirDotted+"="+dir)`,
	} {
		expr, err := parser.ParseExpr(source)
		if err != nil {
			t.Fatal(err)
		}
		call, ok := expr.(*ast.CallExpr)
		if !ok || !isOSEnviron(call.Args[0]) {
			t.Fatalf("%s: not an append onto os.Environ()", source)
		}
		found := false
		for _, arg := range call.Args[1:] {
			found = found || assignsConfigRoot(arg)
		}
		if !found {
			t.Errorf("the scan misses %s", source)
		}
	}
}

// isIdent answers whether expr is the identifier name.
func isIdent(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
}

// isOSEnviron answers whether expr is the call os.Environ().
func isOSEnviron(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && isIdent(selector.X, "os") && selector.Sel.Name == "Environ"
}

// assignsConfigRoot answers whether expr is an environment entry that names
// ze.config.dir under any spelling: its leftmost operand is a string literal
// whose key reads as that variable, or envConfigDir / envConfigDirDotted.
func assignsConfigRoot(expr ast.Expr) bool {
	for {
		binary, ok := expr.(*ast.BinaryExpr)
		if !ok || binary.Op != token.ADD {
			break
		}
		expr = binary.X
	}
	if isIdent(expr, "envConfigDir") || isIdent(expr, "envConfigDirDotted") {
		return true
	}
	literal, ok := expr.(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return false
	}
	value, err := strconv.Unquote(literal.Value)
	if err != nil {
		return false
	}
	key, _, _ := strings.Cut(value, "=")
	return environmentKeyReading(key) == environmentKeyReading(envConfigDirDotted)
}
