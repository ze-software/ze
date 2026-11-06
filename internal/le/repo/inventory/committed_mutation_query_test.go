// Design: docs/architecture/plugin/feature-gates.md -- mutation query authority stays reader-local.
package repoinventory

import (
	"go/ast"
	"strings"
	"testing"
)

// TestCommittedMutationQueriesRetainPolicyAndIdentity exercises actual selected
// assignments and addresses through one resolver. It does not inspect its cache.
func TestCommittedMutationQueriesRetainPolicyAndIdentity(t *testing.T) {
	root := committedFixture(t)
	const directory = "internal/plugins/example"
	const name = directory + "/query_controls.go"
	const declarations = `package example
import names "github.com/ze-software/ze/internal/shared"
var Callback = func() {}
func address() {
 callback := func() {}
 _ = &callback
 _ = &Callback
 names.Callback = func() {}
}
func intact() { callback := func() {}; _ = callback }
`
	write(t, root, name, declarations)
	write(t, root, "internal/shared/query_controls.go", "package shared\nvar Callback = func() {}\n")
	committedGit(t, root, "add", ".")
	committedGit(t, root, "-c", "user.name=Inventory Fixture", "-c", "user.email=inventory@example.invalid", "commit", "-m", "mutation query controls")
	snapshot, err := ReadCommitted(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	reader := snapshot.evaluator()
	global := mutationQueryGlobal(t, snapshot, directory, "Callback")
	if err := reader.immutable(global); err != nil {
		t.Fatalf("address-only global lost its all-writes control: %v", err)
	}
	failure := reader.callableImmutable(global)
	if failure == nil {
		t.Fatal("a prior clean all-writes query certified an addressed callable")
	}
	if !strings.Contains(failure.Error(), "escaping address") {
		t.Fatalf("callable policy lost its address cause: %v", failure)
	}
	if err := reader.immutable(global); err != nil {
		t.Fatalf("callable-address refusal contaminated the other policy: %v", err)
	}
	for _, scenario := range []struct {
		function string
		escaped  bool
	}{
		{"address", true},
		{"intact", false},
	} {
		source := mutationQueryLocal(t, snapshot.packages[directory], name, scenario.function)
		if err := reader.immutable(source); err != nil {
			t.Fatalf("local %s lost its assignment control: %v", scenario.function, err)
		}
		err := reader.callableImmutable(source)
		if (err != nil) != scenario.escaped {
			t.Fatalf("distinct local %s borrowed another callback's query: %v", scenario.function, err)
		}
	}
	foreign := mutationQueryGlobal(t, snapshot, "internal/shared", "Callback")
	failure = reader.immutable(foreign)
	if failure == nil {
		t.Fatal("same-named imported declaration lost its selected importing-file write")
	}
	if !strings.Contains(failure.Error(), "mutated") {
		t.Fatalf("imported write lost its mutation cause: %v", failure)
	}

	// Neither a dirty edit nor a different HEAD changes this reader's source.
	changed := strings.Replace(declarations, "_ = &Callback", "Callback = func() {}", 1)
	write(t, root, name, changed)
	if err := reader.immutable(global); err != nil {
		t.Fatalf("dirty source changed the retained query: %v", err)
	}
	committedGit(t, root, "add", ".")
	committedGit(t, root, "-c", "user.name=Inventory Fixture", "-c", "user.email=inventory@example.invalid", "commit", "-m", "changed mutation query source")
	if err := reader.immutable(global); err != nil {
		t.Fatalf("HEAD advance changed the old retained query: %v", err)
	}
	fresh, err := ReadCommitted(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := fresh.evaluator().immutable(mutationQueryGlobal(t, fresh, directory, "Callback")); err == nil {
		t.Fatal("a fresh reader reused another snapshot's clean declaration query")
	}
}

func mutationQueryGlobal(t *testing.T, snapshot *Snapshot, directory, name string) sourceExpr {
	t.Helper()
	pkg := snapshot.packages[directory]
	for _, file := range pkg.files {
		if object := file.File.Scope.Lookup(name); object != nil {
			identifier := ast.NewIdent(name)
			identifier.Obj = object
			return sourceExpr{value: identifier, file: file.File, pkg: pkg}
		}
	}
	t.Fatalf("authored global %s.%s was not selected", directory, name)
	return sourceExpr{}
}

func mutationQueryLocal(t *testing.T, pkg *sourcePackage, name, function string) sourceExpr {
	t.Helper()
	for _, file := range pkg.files {
		if file.Path != name {
			continue
		}
		for _, declaration := range file.File.Decls {
			owner, declared := declaration.(*ast.FuncDecl)
			if !declared {
				continue
			}
			if owner.Name.Name != function {
				continue
			}
			assignment, assigned := owner.Body.List[0].(*ast.AssignStmt)
			if !assigned {
				t.Fatalf("authored local %s.callback has no defining assignment", function)
			}
			return sourceExpr{value: assignment.Lhs[0], file: file.File, pkg: pkg}
		}
	}
	t.Fatalf("authored local %s.callback was not selected", function)
	return sourceExpr{}
}
