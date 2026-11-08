// Design: docs/architecture/plugin/feature-gates.md -- a slice expression has the type the Go specification gives it.
package repoinventory

import (
	"go/ast"
	"go/types"
	"testing"
)

// TestExpressionSliceTypeDerivesRangeVariable requires a slice expression to
// have its specified static type, so zstd's dict.go `for i, n := range
// remain[:]` over an array field types both range variables. Method: the
// static type resolver is asked for slices of an array field, a string field,
// a named slice field and a pointer to an array, and for the range variables
// over the sliced array, over an authored package.
//
// MUTATION: (internal/le/repo/inventory/methods.go,
// `return e.sliceType(source, value, depth)`, `return sourceExpr{}, false`):
// no slice expression has a type, and the range variables follow.
// MUTATION: (internal/le/repo/inventory/methods.go,
// `if kind.Len == nil {`, `if false {`): a sliced named slice loses its name.
// MUTATION: (internal/le/repo/inventory/methods.go,
// `if basic.Info()&types.IsString == 0 {`, `if basic.Info()&types.IsString == 0 || true {`):
// a sliced string has no type.
// MUTATION: (internal/le/repo/inventory/methods.go,
// `return sourceExpr{value: &ast.ArrayType{Lbrack: array.Lbrack, Elt: array.Elt}, file: shape.file, pkg: shape.pkg}, true`,
// `return sourceExpr{}, false`): a sliced pointer to an array has no type.
func TestExpressionSliceTypeDerivesRangeVariable(t *testing.T) {
	const directory = "internal/slicetypeproof"
	const filename = directory + "/proof.go"
	effects := typeIdentitySelectedSource(t, directory, map[string]string{
		"go.mod":            "module github.com/ze-software/ze\n\ngo 1.26\n",
		"feature-gates.txt": "",
		filename: `package slicetypeproof
type names []string
type table struct { remain [256]int; label string; parts names; window *[4]byte }
func (t *table) walk() {
	for i, n := range t.remain[:] {
		_, _ = i, n
	}
	_ = t.label[1:]
	_ = t.parts[1:]
	_ = t.window[:]
}
`,
	})
	pkg := effects.source.snapshot.packages[directory]
	file := typeIdentitySelectedFile(t, pkg, filename)
	var slices []ast.Expr
	var key, value ast.Expr
	ast.Inspect(file, func(node ast.Node) bool {
		switch current := node.(type) {
		case *ast.SliceExpr:
			slices = append(slices, current)
		case *ast.RangeStmt:
			key, value = current.Key, current.Value
		}
		return true
	})
	if len(slices) != 4 {
		t.Fatalf("authored slice expressions are unavailable: %d", len(slices))
	}
	resolver := effects.source.snapshot.evaluator()
	typed := func(expression ast.Expr) string {
		datatype, found := resolver.valueType(sourceExpr{value: expression, file: file, pkg: pkg}, 0)
		if !found {
			return ""
		}
		return types.ExprString(datatype.value)
	}
	for _, want := range []struct {
		name       string
		expression ast.Expr
		datatype   string
	}{
		{"sliced array field", slices[0], "[]int"},
		{"range index over the sliced array", key, "int"},
		{"range element over the sliced array", value, "int"},
		{"sliced string", slices[1], "string"},
		{"sliced named slice", slices[2], "names"},
		{"sliced pointer to an array", slices[3], "[]byte"},
	} {
		if got := typed(want.expression); got != want.datatype {
			t.Errorf("%s typed %q, want %q", want.name, got, want.datatype)
		}
	}
}
