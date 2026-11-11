// Design: docs/architecture/plugin/feature-gates.md -- structural identities retain source authority.
package repoinventory

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestExternalTypeIdentitySelectedScopes uses production source selection and
// actual definition queries. It proves structural identity, not Git capture or
// the complete inventory publication boundary covered by committed fixtures.
func TestExternalTypeIdentitySelectedScopes(t *testing.T) {
	const directory = "pkg/alpha"
	const first = directory + "/first.go"
	effects := typeIdentitySelectedSource(t, directory, map[string]string{
		first: `package alpha
import right "github.com/ze-software/ze/pkg/beta"
type T int
type Local int
var Imported right.T
func lexical[T any]() { type Local string; var _ Local }
`,
		directory + "/second.go": "package alpha\ntype Other int\n",
		"pkg/beta/source.go":     "package beta\ntype T string\n",
	})
	pkg := effects.source.snapshot.packages[directory]
	file := typeIdentitySelectedFile(t, pkg, first)
	walk := externalTypeWalk{effects: effects}
	global := file.Scope.Lookup("T")
	name := ast.NewIdent("T")
	name.Obj = global
	definition, owner, reason := walk.definition(sourceExpr{value: name, file: file, pkg: pkg})
	if reason != "" || definition != global.Decl || owner.file != file || owner.pkg != pkg {
		t.Fatalf("selected package type lost exact ownership: %p %+v %s", definition, owner, reason)
	}
	function := pkg.functions["lexical"]
	if function.decl == nil {
		t.Fatal("selected source lost lexical helper")
	}
	statement, declared := function.decl.Body.List[0].(*ast.DeclStmt)
	if !declared {
		t.Fatal("authored lexical helper lost its local declaration")
	}
	group, grouped := statement.Decl.(*ast.GenDecl)
	if !grouped {
		t.Fatal("authored local type lost its declaration group")
	}
	local, typed := group.Specs[0].(*ast.TypeSpec)
	if !typed {
		t.Fatal("authored local type lost its TypeSpec")
	}
	definition, owner, reason = walk.definition(sourceExpr{value: local.Name, file: file, pkg: pkg})
	if reason != "" || definition != local || owner.file != file || owner.value != local.Type {
		t.Fatalf("local same-spelled type borrowed package ownership: %p %+v %s", definition, owner, reason)
	}
	parameter := function.decl.Type.TypeParams.List[0].Names[0]
	if _, _, reason := walk.definition(sourceExpr{value: parameter, file: file, pkg: pkg}); !strings.Contains(reason, "parameter scope") {
		t.Fatalf("lexical type parameter borrowed package T: %q", reason)
	}
	imported := file.Scope.Lookup("Imported")
	value, declared := imported.Decl.(*ast.ValueSpec)
	if !declared {
		t.Fatal("authored imported type lost its value declaration")
	}
	definition, owner, reason = walk.definition(sourceExpr{value: value.Type, file: file, pkg: pkg})
	beta := effects.source.snapshot.packages["pkg/beta"]
	betaFile := typeIdentitySelectedFile(t, beta, "pkg/beta/source.go")
	if reason != "" || definition != betaFile.Scope.Lookup("T").Decl || owner.file != betaFile || owner.pkg != beta {
		t.Fatalf("imported same-spelled type lost its defining package/file: %p %+v %s", definition, owner, reason)
	}
	// This AST is deliberately outside the selected source population. It has
	// the same declaration spelling and its own real parser object.
	unselected, err := parser.ParseFile(token.NewFileSet(), "unselected.go", "package alpha\ntype T int\n", 0)
	if err != nil {
		t.Fatal(err)
	}
	detached := ast.NewIdent("T")
	detached.Obj = unselected.Scope.Lookup("T")
	if _, _, reason := walk.definition(sourceExpr{value: detached, file: unselected, pkg: pkg}); !strings.Contains(reason, "defining file is unavailable") {
		t.Fatalf("unselected same-spelled declaration borrowed a selected owner: %q", reason)
	}
}

func TestExternalTypeIdentityDuplicatePackageBindingRefuses(t *testing.T) {
	const directory = "pkg/duplicate"
	effects := typeIdentitySelectedSource(t, directory, map[string]string{
		directory + "/a.go": "package duplicate\ntype T int\n",
		directory + "/b.go": "package duplicate\ntype T string\n",
	})
	pkg := effects.source.snapshot.packages[directory]
	file := typeIdentitySelectedFile(t, pkg, directory+"/a.go")
	walk := externalTypeWalk{effects: effects}
	if _, _, reason := walk.definition(sourceExpr{value: ast.NewIdent("T"), file: file, pkg: pkg}); !strings.Contains(reason, "ambiguous") {
		t.Fatalf("package fallback silently chose a duplicate type: %q", reason)
	}
	// An exact lexical parser object keeps the existing precedence; the index
	// cannot replace it with the package fallback's other declaration.
	name := ast.NewIdent("T")
	name.Obj = file.Scope.Lookup("T")
	definition, owner, reason := walk.definition(sourceExpr{value: name, file: file, pkg: pkg})
	if reason != "" || definition != name.Obj.Decl || owner.file != file {
		t.Fatalf("exact lexical identity was replaced by a package-name cache: %p %+v %s", definition, owner, reason)
	}
}

func TestExternalTypeIdentityReceiverQueriesKeepTheirDepth(t *testing.T) {
	const directory = "pkg/receivers"
	effects := typeIdentitySelectedSource(t, directory, map[string]string{
		directory + "/source.go": "package receivers\ntype T int\ntype Alias = T\nfunc (Alias) Act() {}\n",
	})
	pkg := effects.source.snapshot.packages[directory]
	file := typeIdentitySelectedFile(t, pkg, directory+"/source.go")
	name := ast.NewIdent("T")
	name.Obj = file.Scope.Lookup("T")
	walk := externalTypeWalk{effects: effects}
	definition, owner, reason := walk.definition(sourceExpr{value: name, file: file, pkg: pkg})
	if reason != "" {
		t.Fatal(reason)
	}
	value := walk.methods(definition, owner)
	if value.unknown != "" || len(value.callables) != 1 {
		t.Fatalf("ordinary alias receiver lost its selected method: %+v", value)
	}
	typeSource := sourceExpr{value: name, file: file, pkg: pkg}
	if value := walk.value(typeSource, nil, true, 128); value.unknown != "type capability depth exceeds 128" {
		t.Fatalf("a query at depth 128 did not refuse: %+v", value)
	}
	if value := walk.value(typeSource, nil, true, 0); value.unknown != "" {
		t.Fatalf("a depth refusal contaminated a shallow query: %+v", value)
	}
	if value := walk.methods(definition, owner); value.unknown != "" || len(value.callables) != 1 {
		t.Fatalf("a depth refusal contaminated the method query: %+v", value)
	}
}

func TestExternalTypeIdentityRetainsReceiverRefusals(t *testing.T) {
	for _, scenario := range []struct{ name, text, cause string }{
		{"bodyless", "type T int\nfunc (T) Opaque()\n", "bodyless"},
		{"unrelated unresolved", "type T int\nfunc (Missing) Other() {}\n", "ownership is unresolved"},
		{"unrelated alias cycle", "type T int\ntype A = B\ntype B = A\nfunc (A) Other() {}\n", "cyclic retained receiver"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			const directory = "pkg/refusals"
			effects := typeIdentitySelectedSource(t, directory, map[string]string{directory + "/source.go": "package refusals\n" + scenario.text})
			pkg := effects.source.snapshot.packages[directory]
			file := typeIdentitySelectedFile(t, pkg, directory+"/source.go")
			name := ast.NewIdent("T")
			name.Obj = file.Scope.Lookup("T")
			walk := externalTypeWalk{effects: effects}
			definition, owner, reason := walk.definition(sourceExpr{value: name, file: file, pkg: pkg})
			if reason != "" {
				t.Fatal(reason)
			}
			if value := walk.methods(definition, owner); !strings.Contains(value.unknown, scenario.cause) {
				t.Fatalf("receiver census dropped its %s refusal: %+v", scenario.cause, value)
			}
		})
	}
}

// TestExternalTypeIdentityExecutesGenericReceiverMethods requires a method
// of a generic type to join the type's capability as an executed body, the
// way a method of any other type does, now that its receiver type parameters
// are bound in the method's scope.
func TestExternalTypeIdentityExecutesGenericReceiverMethods(t *testing.T) {
	const directory = "pkg/generic"
	effects := typeIdentitySelectedSource(t, directory, map[string]string{directory + "/source.go": "package generic\ntype T[P any] struct{}\nfunc (T[P]) Act() {}\n"})
	pkg := effects.source.snapshot.packages[directory]
	file := typeIdentitySelectedFile(t, pkg, directory+"/source.go")
	name := ast.NewIdent("T")
	name.Obj = file.Scope.Lookup("T")
	walk := externalTypeWalk{effects: effects}
	definition, owner, reason := walk.definition(sourceExpr{value: name, file: file, pkg: pkg})
	if reason != "" {
		t.Fatal(reason)
	}
	value := walk.methods(definition, owner)
	if value.unknown != "" {
		t.Fatalf("generic receiver method refused: %s", value.unknown)
	}
	if len(value.callables) != 1 {
		t.Fatalf("generic receiver method count = %d, want 1", len(value.callables))
	}
}

// The names below forward to methods_test.go for the proof-named tests that
// still use them; they go with this file.
func typeIdentitySelectedSource(t testing.TB, directory string, sources map[string]string) *externalEffects {
	t.Helper()
	return newExternalEffects(&externalSource{snapshot: selectedSnapshot(t, directory, sources)})
}

func typeIdentitySelectedFile(t *testing.T, pkg *sourcePackage, name string) *ast.File {
	t.Helper()
	return selectedFile(t, pkg, name)
}
