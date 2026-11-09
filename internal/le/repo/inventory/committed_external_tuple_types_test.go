// Design: docs/architecture/plugin/feature-gates.md -- a variable declared from a multi-valued call has its result's type.
package repoinventory

import (
	"go/ast"
	"testing"
)

// TestExpressionTupleTypeDerivesCallResults requires a variable declared or
// assigned from one multi-valued call to have the type of the callee's result
// at its position, as protojson's tok, err := d.Read() gives tok the type
// json.Token. Method: the static type resolver is asked for each variable of
// an authored package: a method's two results, a function's named result list
// (left, right uint16, fail error), a package var declared from it, and a
// promoted method's results through an embedded pointer. A result count that
// differs from the variables, an unknown callee, and the comma-ok map index
// stay untyped. The package is read as syntax, so the invalid count case is
// authored on purpose: inventory must refuse it rather than type a prefix.
//
// MUTATION: (internal/le/repo/inventory/methods.go,
// `return e.tupleType(source, declaration.Rhs, len(declaration.Lhs), index, depth+1)`,
// `return sourceExpr{}, false`): a short variable declaration from a call
// leaves tok untyped.
// MUTATION: (internal/le/repo/inventory/methods.go,
// `return e.tupleType(source, declaration.Values, len(declaration.Names), index, depth+1)`,
// `return sourceExpr{}, false`): the package var first is untyped.
// MUTATION: (internal/le/repo/inventory/methods.go,
// `if function.decl.Type.Results.NumFields() != operands {`,
// `if function.decl.Type.Results.NumFields() < operands {`): two variables
// from three results type as the first two results, and short is a uint8.
// MUTATION: (internal/le/repo/inventory/methods.go,
// `count := max(len(result.Names), 1)`, `count := 1`): the named result
// field (left, right uint16) counts once, so high takes fail's type, error.
// MUTATION: (internal/le/repo/inventory/methodpromotion.go,
// `if pointerMethod == holdsPointer {`, `if pointerMethod != holdsPointer {`):
// read, a pointer method on the embedded *reader, no longer resolves through
// scanner, and held is untyped.
// No single-line break admits the unknown callee or the comma-ok index:
// either branch removed leaves a nil declaration the next line reads.
func TestExpressionTupleTypeDerivesCallResults(t *testing.T) {
	const directory = "internal/tupletypeproof"
	const filename = directory + "/proof.go"
	effects := typeIdentitySelectedSource(t, directory, map[string]string{
		"go.mod":            "module github.com/ze-software/ze\n\ngo 1.26\n",
		"feature-gates.txt": "",
		filename: `package tupletypeproof
import "os"
type token struct { kind uint8 }
type reader struct { size int }
func (r *reader) read() (token, error) { _ = r; return token{kind: 0}, nil }
type scanner struct { *reader }
func pair() (left, right uint16, fail error) { return 0, 0, nil }
func three() (uint8, uint16, uint32) { return 0, 0, 0 }
var first, second, third = pair()
func use(r *reader, s scanner) {
	tok, err := r.read()
	low, high, bad := pair()
	short, long := three()
	vague, other := missing()
	held, through := s.read()
	table := map[string]uint8{}
	entry, present := table["key"]
	_, _, _, _, _, _, _, _, _, _, _, _, _ = tok, err, low, high, bad, short, long, vague, other, held, through, entry, present
	_ = os.Args
}
`,
	})
	pkg := effects.source.snapshot.packages[directory]
	file := typeIdentitySelectedFile(t, pkg, filename)
	declared := make(map[string]*ast.Ident)
	ast.Inspect(file, func(node ast.Node) bool {
		if name, ok := node.(*ast.Ident); ok {
			if _, seen := declared[name.Name]; !seen {
				declared[name.Name] = name
			}
		}
		return true
	})
	resolver := effects.source.snapshot.evaluator()
	typed := func(variable string) string {
		datatype, found := resolver.valueType(sourceExpr{value: declared[variable], file: file, pkg: pkg}, 0)
		if !found {
			return ""
		}
		name, ok := datatype.value.(*ast.Ident)
		if !ok {
			return "non-name"
		}
		return name.Name
	}
	for _, want := range []struct{ variable, datatype string }{
		{"tok", "token"},
		{"err", "error"},
		{"low", "uint16"},
		{"high", "uint16"},
		{"bad", "error"},
		{"first", "uint16"},
		{"second", "uint16"},
		{"held", "token"},
		{"through", "error"},
	} {
		if got := typed(want.variable); got != want.datatype {
			t.Fatalf("%s typed %q, want %q", want.variable, got, want.datatype)
		}
	}
	for _, refused := range []string{"short", "long", "vague", "other", "entry", "present"} {
		if got := typed(refused); got != "" {
			t.Fatalf("%s typed %q, want untyped", refused, got)
		}
	}
}
