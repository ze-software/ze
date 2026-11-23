// Design: docs/architecture/plugin/feature-gates.md -- parsed syntax retains declaration ownership.
// Related: parsed_bindings.go -- bindings without committed-source claims.
package repoinventory

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// parsedBindingFiles parses caller-owned syntax once, as enumeration does.
func parsedBindingFiles(t *testing.T, sources ...string) []*ast.File {
	t.Helper()
	fset := token.NewFileSet()
	files := make([]*ast.File, 0, len(sources))
	for _, source := range sources {
		file, err := parser.ParseFile(fset, "bindings.go", source, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	return files
}

// mustAs asserts that value has concrete type T in a parsed fixture AST.
func mustAs[T any](t *testing.T, value any) T {
	t.Helper()
	got, ok := value.(T)
	if !ok {
		t.Fatalf("unexpected AST node type %T", value)
	}
	return got
}

// TestParsedBindingsRetainDeclarationIdentity compares authored definitions
// and uses across files, local shadows and package-initializer closures.
func TestParsedBindingsRetainDeclarationIdentity(t *testing.T) {
	files := parsedBindingFiles(t, `package example
var first, second = 1, 2
var prepared = func() int { first = 3; return first }()
`, `package example
func use() { first := second; first = 4; _ = first }
`)
	view := NewParsedBindings(files, ParsedBindingOptions{})
	group := mustAs[*ast.ValueSpec](t, mustAs[*ast.GenDecl](t, files[0].Decls[0]).Specs[0])
	first, known := view.Binding(files[0], group.Names[0])
	if !known {
		t.Fatal("package definition is unknown")
	}
	second, known := view.Binding(files[0], group.Names[1])
	if !known {
		t.Fatal("second package definition is unknown")
	}
	if first == second {
		t.Fatal("two names in one ValueSpec share an identity")
	}
	if first.Ident() != group.Names[0] {
		t.Fatal("defining identifier was replaced")
	}
	if first.Node() != group {
		t.Fatal("declaration node was replaced")
	}
	if !first.Package() {
		t.Fatal("package declaration became local")
	}
	if first.Kind() != ParsedBindingVariable {
		t.Fatal("variable kind was lost")
	}
	prepared := mustAs[*ast.ValueSpec](t, mustAs[*ast.GenDecl](t, files[0].Decls[1]).Specs[0])
	closure := mustAs[*ast.FuncLit](t, mustAs[*ast.CallExpr](t, prepared.Values[0]).Fun)
	write := mustAs[*ast.Ident](t, mustAs[*ast.AssignStmt](t, closure.Body.List[0]).Lhs[0])
	bound, known := view.Binding(files[0], write)
	if !known {
		t.Fatal("initializer closure write is unknown")
	}
	if bound != first {
		t.Fatal("initializer closure write lost package ownership")
	}
	function := mustAs[*ast.FuncDecl](t, files[1].Decls[0])
	local := mustAs[*ast.AssignStmt](t, function.Body.List[0])
	bound, known = view.Binding(files[1], mustAs[*ast.Ident](t, local.Rhs[0]))
	if !known {
		t.Fatal("cross-file reference is unknown")
	}
	if bound != second {
		t.Fatal("cross-file reference borrowed another declaration")
	}
	shadow, known := view.Binding(files[1], mustAs[*ast.Ident](t, local.Lhs[0]))
	if !known {
		t.Fatal("local definition is unknown")
	}
	if shadow == first {
		t.Fatal("local shadow borrowed the package declaration")
	}
	if shadow.Package() {
		t.Fatal("local shadow became package-scoped")
	}
	assignment := mustAs[*ast.AssignStmt](t, function.Body.List[1])
	bound, known = view.Binding(files[1], mustAs[*ast.Ident](t, assignment.Lhs[0]))
	if !known {
		t.Fatal("local write is unknown")
	}
	if bound != shadow {
		t.Fatal("local write lost shadow ownership")
	}
}

// TestCommittedImplicitImportsRetainDeclaredOwnership reads a committed fixture
// whose internal package name differs from its directory, plus a standard-library
// import. Metadata and exact ImportSpec references both depend on those bindings.
func TestCommittedImplicitImportsRetainDeclaredOwnership(t *testing.T) {
	root := committedFixture(t)
	write(t, root, "internal/shared/engine.go", `package actualnames
const Name = "implicit-package-owner"
func Engine() {}
`)
	const registrationPath = "internal/plugins/example/register_linux.go"
	write(t, root, registrationPath, `package example
import (
 "github.com/ze-software/ze/internal/component/plugin/registry"
 "github.com/ze-software/ze/internal/shared"
 "os"
)
func init() {
 _ = os.Args
 _ = registry.Register(registry.Registration{Name: actualnames.Name, RunEngine: actualnames.Engine})
}
`)
	commitInventoryFixture(t, root)
	snapshot, err := ReadCommitted(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	plugins, err := snapshot.DeclaredPlugins()
	if err != nil {
		t.Fatal(err)
	}
	if len(plugins) != 1 {
		t.Fatalf("implicit import produced %d plugins, want one", len(plugins))
	}
	if plugins[0].Name != "implicit-package-owner" {
		t.Fatalf("imported name constant resolved to %q", plugins[0].Name)
	}
	if plugins[0].SourceDir != "internal/shared" {
		t.Fatalf("imported engine origin is %q", plugins[0].SourceDir)
	}
	sources, _ := snapshot.Sources()
	var registration SourceFile
	for _, source := range sources {
		if source.Path == registrationPath {
			registration = source
		}
	}
	if registration.File == nil {
		t.Fatal("committed registration syntax is missing")
	}
	if len(registration.File.Imports) != 3 {
		t.Fatal("registration fixture does not retain its three imports")
	}
	wanted := map[string]*ast.ImportSpec{
		"actualnames": registration.File.Imports[1],
		"os":          registration.File.Imports[2],
	}
	seen := map[string]bool{}
	ast.Inspect(registration.File, func(node ast.Node) bool {
		selector, selected := node.(*ast.SelectorExpr)
		if !selected {
			return true
		}
		identifier, named := selector.X.(*ast.Ident)
		if !named {
			return true
		}
		imported, expected := wanted[identifier.Name]
		if !expected {
			return true
		}
		same, known := snapshot.DeclarationReference(registration, identifier, imported)
		if !known {
			t.Fatalf("qualifier %s has unknown import ownership", identifier.Name)
		}
		if !same {
			t.Fatalf("qualifier %s lost its exact ImportSpec", identifier.Name)
		}
		same, known = snapshot.DeclarationReference(registration, identifier, registration.File.Imports[0])
		if !known {
			t.Fatalf("qualifier %s lost its negative ownership control", identifier.Name)
		}
		if same {
			t.Fatalf("qualifier %s borrowed the registry ImportSpec", identifier.Name)
		}
		seen[identifier.Name] = true
		return true
	})
	for name := range wanted {
		if !seen[name] {
			t.Fatalf("fixture never exercised implicit qualifier %s", name)
		}
	}
}

// TestParsedBindingsRetainCallerKeyRoles checks caller-classified expression
// keys, field labels and unknown shapes against an actual package constant.
func TestParsedBindingsRetainCallerKeyRoles(t *testing.T) {
	files := parsedBindingFiles(t, `package example
const key = "value"
type Keys map[string]int
type Alias = Keys
type Fields struct { key int }
var mapping = Alias{key: 1}
var record = Fields{key: 1}
var unknown = foreign.Fields{key: 1}
`)
	view := NewParsedBindings(files, ParsedBindingOptions{CompositeKeys: func(_ *ParsedBindings, _ *ast.File, literal *ast.CompositeLit) ParsedCompositeKeys {
		name, named := literal.Type.(*ast.Ident)
		if !named {
			return ParsedKeysUnknown
		}
		if name.Name == "Alias" {
			return ParsedKeysExpressions
		}
		if name.Name == "Fields" {
			return ParsedKeysFields
		}
		return ParsedKeysUnknown
	}})
	definition := mustAs[*ast.ValueSpec](t, mustAs[*ast.GenDecl](t, files[0].Decls[0]).Specs[0]).Names[0]
	want, known := view.Binding(files[0], definition)
	if !known {
		t.Fatal("constant definition is unknown")
	}
	for index, state := range []string{"map", "struct", "unknown"} {
		valueSpec := mustAs[*ast.ValueSpec](t, mustAs[*ast.GenDecl](t, files[0].Decls[index+4]).Specs[0])
		literal := mustAs[*ast.CompositeLit](t, valueSpec.Values[0])
		key := mustAs[*ast.Ident](t, mustAs[*ast.KeyValueExpr](t, literal.Elts[0]).Key)
		binding, known := view.Binding(files[0], key)
		switch state {
		case "map":
			if !known {
				t.Fatal("named map key is unknown")
			}
			if binding != want {
				t.Fatal("named map key lost its constant")
			}
		case "struct":
			if !known {
				t.Fatal("struct label is not classified")
			}
			if binding != nil {
				t.Fatal("struct label borrowed a constant")
			}
		case "unknown":
			if known {
				t.Fatal("unknown key shape acquired declaration credit")
			}
		}
	}
}

// TestParsedBindingsRefuseForeignSyntax checks node ownership, uninitialized
// views and blank nonbindings without borrowing another syntax tree's names.
func TestParsedBindingsRefuseForeignSyntax(t *testing.T) {
	files := parsedBindingFiles(t, "package example; var value = 1; func use() { _ = value }")
	view := NewParsedBindings(files, ParsedBindingOptions{})
	assignment := mustAs[*ast.AssignStmt](t, mustAs[*ast.FuncDecl](t, files[0].Decls[1]).Body.List[0])
	blank := mustAs[*ast.Ident](t, assignment.Lhs[0])
	bound, known := view.Binding(files[0], blank)
	if !known {
		t.Fatal("blank identifier is not classified")
	}
	if bound != nil {
		t.Fatal("blank identifier acquired a declaration")
	}
	if _, known := view.Binding(files[0], &ast.Ident{Name: "value"}); known {
		t.Fatal("fabricated syntax acquired a declaration")
	}
	foreign := parsedBindingFiles(t, "package example; var value = 1")[0]
	valueRef := mustAs[*ast.Ident](t, assignment.Rhs[0])
	if _, known := view.Binding(foreign, valueRef); known {
		t.Fatal("foreign file acquired a declaration")
	}
	for _, test := range []struct {
		name string
		view *ParsedBindings
	}{
		{name: "nil"},
		{name: "zero", view: &ParsedBindings{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			binding, known := test.view.Binding(files[0], valueRef)
			if known {
				t.Fatal("an uninitialized view classified retained syntax")
			}
			if binding != nil {
				t.Fatal("an uninitialized view returned a declaration")
			}
		})
	}
}

// TestParsedBindingsKeepUnresolvedImportsConservative proves an unknown import
// cannot manufacture a universe binding, while valid package declarations remain
// visible and an explicit alias retains its authored declaration.
func TestParsedBindingsKeepUnresolvedImportsConservative(t *testing.T) {
	files := parsedBindingFiles(t, `package example
import "example.invalid/syntax/v2"
import named "example.invalid/explicit"
var codes = 1
func use() { _ = codes; _ = append; _ = named.Value }
`)
	view := NewParsedBindings(files, ParsedBindingOptions{})
	body := mustAs[*ast.FuncDecl](t, files[0].Decls[3]).Body
	use := mustAs[*ast.Ident](t, mustAs[*ast.AssignStmt](t, body.List[0]).Rhs[0])
	bound, known := view.Binding(files[0], use)
	if !known {
		t.Fatal("unknown import hid a valid package declaration")
	}
	if !bound.Package() {
		t.Fatal("package reference became local")
	}
	builtin := mustAs[*ast.Ident](t, mustAs[*ast.AssignStmt](t, body.List[1]).Rhs[0])
	if _, known := view.Binding(files[0], builtin); known {
		t.Fatal("unknown import borrowed universe append")
	}
	selected := mustAs[*ast.SelectorExpr](t, mustAs[*ast.AssignStmt](t, body.List[2]).Rhs[0])
	selectedRoot := mustAs[*ast.Ident](t, selected.X)
	bound, known = view.Binding(files[0], selectedRoot)
	if !known {
		t.Fatal("explicit import alias became unknown")
	}
	if bound.Kind() != ParsedBindingImport {
		t.Fatal("explicit alias lost import ownership")
	}
	if binding, known := view.Binding(files[0], selected.Sel); !known {
		t.Fatal("selector member is not classified")
	} else if binding != nil {
		t.Fatal("selector member borrowed a binding")
	}
	resolved := NewParsedBindings(files, ParsedBindingOptions{ImportName: func(name string) (string, bool) {
		if name == "example.invalid/syntax/v2" {
			return "syntax", true
		}
		return "", false
	}})
	bound, known = resolved.Binding(files[0], builtin)
	if !known {
		t.Fatal("known package name hid universe append")
	}
	if bound.Kind() != ParsedBindingUniverse {
		t.Fatal("resolved imports lost universe provenance")
	}
	definition := mustAs[*ast.ValueSpec](t, mustAs[*ast.GenDecl](t, files[0].Decls[2]).Specs[0]).Names[0]
	for _, test := range []struct {
		name        string
		packageName string
		known       bool
	}{
		{name: "false", packageName: "syntax"},
		{name: "empty", known: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate := NewParsedBindings(files, ParsedBindingOptions{ImportName: func(string) (string, bool) {
				return test.packageName, test.known
			}})
			binding, known := candidate.Binding(files[0], builtin)
			if known {
				t.Fatal("an unresolved import callback supplied universe ownership")
			}
			if binding != nil {
				t.Fatal("an unresolved import callback returned a universe declaration")
			}
			binding, known = candidate.Binding(files[0], use)
			if !known {
				t.Fatal("an unresolved import callback hid the package declaration")
			}
			if binding == nil {
				t.Fatal("the package declaration became a nonbinding")
			}
			if !binding.Package() {
				t.Fatal("the package declaration became local")
			}
			if binding.Ident() != definition {
				t.Fatal("the package reference lost its authored declaration")
			}
			binding, known = candidate.Binding(files[0], selectedRoot)
			if !known {
				t.Fatal("an unresolved import callback hid the explicit alias")
			}
			if binding == nil {
				t.Fatal("the explicit alias became a nonbinding")
			}
			if binding.Kind() != ParsedBindingImport {
				t.Fatal("the explicit alias lost its import declaration")
			}
		})
	}
}
