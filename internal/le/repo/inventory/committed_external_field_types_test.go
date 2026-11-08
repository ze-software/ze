// Design: docs/architecture/plugin/feature-gates.md -- a field selector has its field's declared type.
package repoinventory

import (
	"go/ast"
	"testing"
)

// TestCommittedPointerMethodReassignedBindingExecutesOwner requires a pointer
// method called on a reassigned variable to execute: the assignment keeps the
// variable's static type, so the method set is exact. Method: packet is
// declared from makeParcel() and assigned again before packet.Deliver(), and
// Deliver installs the owner; the report publishes with that owner.
//
// MUTATION: (internal/le/repo/inventory/methods.go,
// `// the type its declaration gives it, and no write check applies here.`,
// `if err := e.immutable(source); err != nil { return sourceExpr{}, false }`):
// the written packet has no type, Deliver is unresolved, and the report
// refuses as callable receiver has unresolved execution ownership.
func TestCommittedPointerMethodReassignedBindingExecutesOwner(t *testing.T) {
	scenario := callableOwnerCase{declarations: "type parcel struct{}\nfunc (*parcel) Deliver() { helper.Install(0) }\nfunc makeParcel() *parcel { return &parcel{} }\nvar packet = makeParcel()\n"}
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := pointerOwnerFixture(t, kind, scenario)
			fixture.goFile(t, callableOwnerCaller, callableOwnerSource(scenario, "packet = makeParcel(); packet.Deliver()"))
			commitInventoryFixture(t, fixture.root)
			if _, _, err := callableOwnerOutcome(t, fixture, kind, false); err != nil {
				t.Fatalf("reassigned pointer receiver lost its owner: %v", err)
			}
		})
	}
}

// TestExpressionFieldTypeDerivesRangeVariable requires a field selector to
// have its field's declared type, so the integer fse's bitWriter.flushAlign
// ranges over, and its range variable, are typed. Method: the static type
// resolver is asked for b.nBits, for n := (b.nBits + 7) >> 3, and for i in
// for i := range n, over an authored package; b is a pointer receiver. A
// promoted field and an imported name stay untyped.
//
// MUTATION: (internal/le/repo/inventory/methods.go,
// `return e.fieldType(source, value, depth)`, `return sourceExpr{}, false`):
// b.nBits has no type, and n and i follow it.
// MUTATION: (internal/le/repo/inventory/methods.go,
// `if name.Name == selector.Sel.Name {`, `if name.Name == "_" {`): no field
// matches, and b.nBits has no type.
// MUTATION: (internal/le/repo/inventory/methods.go,
// `holder.value = pointer.X`, `holder.value = pointer`): the pointer
// receiver keeps its star, its holder is no struct, and b.nBits has no type.
// MUTATION: (internal/le/repo/inventory/methods.go,
// `// the type its declaration gives it, and no write check applies here.`,
// `if err := e.immutable(source); err != nil { return sourceExpr{}, false }`):
// flush writes b.out, so the written receiver b has no type and b.nBits
// follows it.
func TestExpressionFieldTypeDerivesRangeVariable(t *testing.T) {
	const directory = "internal/fieldtypeproof"
	const filename = directory + "/proof.go"
	effects := typeIdentitySelectedSource(t, directory, map[string]string{
		"go.mod":            "module github.com/ze-software/ze\n\ngo 1.26\n",
		"feature-gates.txt": "",
		filename: `package fieldtypeproof
import "os"
type writer struct { nBits uint8; out []byte }
type inner struct { depth uint16 }
type outer struct { inner }
func (b *writer) flush(o outer) {
	n := (b.nBits + 7) >> 3
	for i := range n {
		b.out = append(b.out, byte(i))
	}
	_ = o.depth
	_ = os.Args
}
`,
	})
	pkg := effects.source.snapshot.packages[directory]
	file := typeIdentitySelectedFile(t, pkg, filename)
	selectors := make(map[string]ast.Expr)
	var counter ast.Expr
	var bound ast.Expr
	ast.Inspect(file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.SelectorExpr:
			selectors[value.Sel.Name] = value
		case *ast.RangeStmt:
			counter = value.X
			bound = value.Key
		}
		return true
	})
	resolver := effects.source.snapshot.evaluator()
	typed := func(expression ast.Expr) string {
		datatype, found := resolver.valueType(sourceExpr{value: expression, file: file, pkg: pkg}, 0)
		if !found {
			return ""
		}
		name, ok := datatype.value.(*ast.Ident)
		if !ok {
			return "non-name"
		}
		return name.Name
	}
	for _, want := range []struct {
		name       string
		expression ast.Expr
		datatype   string
	}{
		{"field through a pointer receiver", selectors["nBits"], "uint8"},
		{"integer computed from the field", counter, "uint8"},
		{"range variable over that integer", bound, "uint8"},
	} {
		if got := typed(want.expression); got != want.datatype {
			t.Fatalf("%s typed %q, want %q", want.name, got, want.datatype)
		}
	}
	for _, refused := range []string{"depth", "Args"} {
		if got := typed(selectors[refused]); got != "" {
			t.Fatalf("selector %s typed %q, want untyped", refused, got)
		}
	}
}
