// Design: docs/architecture/plugin/feature-gates.md -- indexed lexical scopes retain span authority.
package repoinventory

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestExternalLexicalParameterIndexRetainsScopes exercises selected source and
// actual scope/type queries. It does not inspect the structural index's maps or
// replace committed inventory publication proofs.
func TestExternalLexicalParameterIndexRetainsScopes(t *testing.T) {
	const directory = "pkg/lexical"
	const filename = directory + "/source.go"
	effects := typeIdentitySelectedSource(t, directory, map[string]string{
		filename: `package lexical
type T int
type Top[T any] struct { Value T }
func Generic[T any]() {
 type Local struct { Value T }
 var _ Local
}
`,
	})
	pkg := effects.source.snapshot.packages[directory]
	file := typeIdentitySelectedFile(t, pkg, filename)
	top, typed := file.Scope.Lookup("Top").Decl.(*ast.TypeSpec)
	if !typed {
		t.Fatal("selected generic declaration lost its TypeSpec")
	}
	function := pkg.functions["Generic"]
	if function.decl == nil {
		t.Fatal("selected generic function is unavailable")
	}
	statement, declared := function.decl.Body.List[0].(*ast.DeclStmt)
	if !declared {
		t.Fatal("selected local type lost its declaration")
	}
	group, grouped := statement.Decl.(*ast.GenDecl)
	if !grouped {
		t.Fatal("selected local type lost its declaration group")
	}
	local, typed := group.Specs[0].(*ast.TypeSpec)
	if !typed {
		t.Fatal("selected local declaration is not a type")
	}
	topParameter := top.TypeParams.List[0].Names[0]
	functionParameter := function.decl.Type.TypeParams.List[0].Names[0]
	topSource := sourceExpr{value: top.Type, file: file, pkg: pkg}
	topScope := effects.typeIdentities.lexicalScope(topSource)
	lexicalParameterFound(t, topScope, topParameter, true)
	lexicalParameterFound(t, topScope, functionParameter, false)
	localSource := sourceExpr{value: local.Type, file: file, pkg: pkg}
	localScope := effects.typeIdentities.lexicalScope(localSource)
	lexicalParameterFound(t, localScope, functionParameter, true)
	lexicalParameterFound(t, localScope, topParameter, false)

	// A positioned synthetic expression is not a member of this AST. Its
	// span still selects the enclosing function's T, rather than package T.
	synthetic := &ast.Ident{Name: "T", NamePos: local.Type.Pos()}
	syntheticSource := sourceExpr{value: synthetic, file: file, pkg: pkg}
	lexicalParameterFound(t, effects.typeIdentities.lexicalScope(syntheticSource), functionParameter, true)
	value := effects.typeMethods(syntheticSource, 0)
	if value.unknown != "" || !value.references {
		t.Fatalf("positioned parameter borrowed the same-spelled scalar package type: %+v", value)
	}
	packageType, typed := file.Scope.Lookup("T").Decl.(*ast.TypeSpec)
	if !typed {
		t.Fatal("selected package scalar lost its type declaration")
	}
	global := &ast.Ident{Name: "T", NamePos: packageType.Name.Pos()}
	globalValue := effects.typeMethods(sourceExpr{value: global, file: file, pkg: pkg}, 0)
	if globalValue.unknown != "" || globalValue.references {
		t.Fatalf("package scalar borrowed a different declaration's parameter scope: %+v", globalValue)
	}

	// Both independent predicates apply, including the original ancestor
	// pruning: this query ends beyond the containing file/function.
	wide := &ast.ParenExpr{Lparen: local.Type.Pos(), X: synthetic, Rparen: file.End() + 1}
	if scope := effects.typeIdentities.lexicalScope(sourceExpr{value: wide, file: file, pkg: pkg}); scope != nil {
		t.Fatal("a wide synthetic query escaped the original ancestor containment")
	}
	// The original helper does not normalize inverted synthetic spans. Both
	// non-ancestor declarations match, and their original preorder is retained.
	inverted := &ast.StarExpr{Star: function.decl.Body.Rbrace, X: topParameter}
	invertedSource := sourceExpr{value: inverted, file: file, pkg: pkg}
	first := effects.typeIdentities.lexicalScope(invertedSource)
	second := effects.typeIdentities.lexicalScope(invertedSource)
	lexicalParameterFound(t, first, functionParameter, true)
	// The nearer same-spelled function T shadows Top.T: an object mismatch
	// at that binding refuses rather than searching through its parent.
	lexicalParameterFound(t, first, topParameter, false)
	if first == nil || second == nil {
		t.Fatal("matching structural entries lost their scope chain")
	}
	if first.parent == nil || second.parent == nil {
		t.Fatal("inverted query lost its original declaration chain")
	}
	if first == second || first.parent == second.parent {
		t.Fatal("separate queries interned scope identities used by type recursion")
	}
	lexicalParameterFound(t, first.parent, topParameter, true)
	lexicalParameterFound(t, first.parent, functionParameter, false)
}

func TestExternalLexicalParameterIndexFallbackAndEmpty(t *testing.T) {
	const directory = "pkg/lexicalempty"
	const filename = directory + "/source.go"
	const emptyName = directory + "/empty.go"
	effects := typeIdentitySelectedSource(t, directory, map[string]string{
		filename:  "package lexicalempty\nfunc Generic[T any]() { var _ T }\n",
		emptyName: "package lexicalempty\nvar Plain int\n",
	})
	pkg := effects.source.snapshot.packages[directory]
	file := typeIdentitySelectedFile(t, pkg, filename)
	parameter := pkg.functions["Generic"].decl.Type.TypeParams.List[0].Names[0]
	for _, source := range []sourceExpr{
		{value: parameter, pkg: pkg},
		{file: file, pkg: pkg},
		{value: ast.NewIdent("T"), file: file, pkg: pkg},
	} {
		if scope := effects.typeIdentities.lexicalScope(source); scope != nil {
			t.Fatal("nil or unpositioned input acquired a lexical scope")
		}
	}
	empty := typeIdentitySelectedFile(t, pkg, emptyName)
	plain, declared := empty.Scope.Lookup("Plain").Decl.(*ast.ValueSpec)
	if !declared {
		t.Fatal("selected zero-parameter file lost its plain declaration")
	}
	if scope := effects.typeIdentities.lexicalScope(sourceExpr{value: plain.Type, file: empty, pkg: pkg}); scope != nil {
		t.Fatal("selected zero-parameter file borrowed another file's declarations")
	}

	// The same filename in a different parser AST is unindexed. It must use
	// its own original traversal rather than an empty/matching-filename fact.
	unselected, err := parser.ParseFile(token.NewFileSet(), filename, "package lexicalempty\nfunc Loose[U any]() { var _ U }\n", 0)
	if err != nil {
		t.Fatal(err)
	}
	function, declared := unselected.Scope.Lookup("Loose").Decl.(*ast.FuncDecl)
	if !declared {
		t.Fatal("unindexed source lost its actual generic function")
	}
	unknownParameter := function.Type.TypeParams.List[0].Names[0]
	source := sourceExpr{value: unknownParameter, file: unselected, pkg: pkg}
	first := effects.typeIdentities.lexicalScope(source)
	second := effects.typeIdentities.lexicalScope(source)
	lexicalParameterFound(t, first, unknownParameter, true)
	lexicalParameterFound(t, first, parameter, false)
	if first == second {
		t.Fatal("unindexed fallback interned a lexical scope result")
	}
}

func lexicalParameterFound(t *testing.T, scope *externalTypeScope, parameter *ast.Ident, want bool) {
	t.Helper()
	_, found := scope.argument(parameter)
	if found != want {
		t.Fatalf("parameter %s object %p: found %t, want %t", parameter.Name, parameter.Obj, found, want)
	}
}
