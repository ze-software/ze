// Design: docs/architecture/plugin/feature-gates.md -- raw receiver query fallback.
// Synthetic receiver syntax must retain uncached binding resolution.
package repoinventory

import (
	"go/ast"
	"go/token"
	"testing"
)

// TestExternalReceiverRawFallbackCompatibility mutates only synthetic syntax.
// Every query retains its exact source tuple while its binding changes. Raw
// lookup must observe each change without borrowing a selected receiver fact.
// MUTATION: memoizing unindexed receiver queries preserves the first target.
func TestExternalReceiverRawFallbackCompatibility(t *testing.T) {
	const directory = "internal/receiverqueryfallback"
	const filename = directory + "/proof.go"
	effects := typeIdentitySelectedSource(t, directory, map[string]string{
		filename: "package receiverqueryfallback\ntype Root int\ntype Other string\nvar NotType = 1\nfunc (Root) Act() {}\n",
	})
	pkg := effects.source.snapshot.packages[directory]
	file := typeIdentitySelectedFile(t, pkg, filename)
	rootBinding := file.Scope.Lookup("Root")
	root, found := rootBinding.Decl.(*ast.TypeSpec)
	if !found {
		t.Fatal("selected fallback fixture lost Root")
	}
	otherBinding := file.Scope.Lookup("Other")
	other, found := otherBinding.Decl.(*ast.TypeSpec)
	if !found {
		t.Fatal("selected fallback fixture lost Other")
	}
	nonType := file.Scope.Lookup("NotType")
	if nonType == nil {
		t.Fatal("selected fallback fixture lost its non-type binding")
	}
	walk := externalTypeWalk{effects: effects}
	selected := receiverQuerySelectedMethod(t, effects, pkg, "Act")
	receiverQueryAssert(t, &walk, selected, 0, root, false, "")
	for _, scenario := range []struct {
		name string
		file *ast.File
	}{
		{name: "synthetic expression in selected file", file: file},
		{name: "unindexed defining file", file: &ast.File{Name: ast.NewIdent("unindexed")}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			name := ast.NewIdent("Root")
			source := sourceExpr{value: name, file: scenario.file, pkg: pkg}
			name.Obj = rootBinding
			receiverQueryAssert(t, &walk, source, 0, root, false, "")
			name.Obj = otherBinding
			receiverQueryAssert(t, &walk, source, 0, other, false, "")
			name.Obj = nonType
			receiverQueryAssert(t, &walk, source, 0, nil, false, "retained declaration is not a type")
			name.Obj = rootBinding
			receiverQueryAssert(t, &walk, source, 0, root, false, "")
		})
	}
	// Selected syntax remains immutable and retains its independent answer.
	receiverQueryAssert(t, &walk, selected, 0, root, false, "")
	for _, scenario := range []struct {
		name    string
		effects *externalEffects
	}{
		{name: "nil effects"},
		{name: "uninitialized structural index", effects: &externalEffects{}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			query := externalTypeWalk{effects: scenario.effects}
			source := sourceExpr{value: &ast.BasicLit{Kind: token.INT, Value: "0"}}
			for range 2 {
				receiverQueryAssert(t, &query, source, 0, nil, false, "retained type declaration shape is unresolved")
			}
		})
	}
}
