// Design: docs/architecture/plugin/feature-gates.md -- an expression has the static type the Go specification gives it.
package repoinventory

import (
	"go/ast"
	"go/token"
	"go/types"
	"path"
	"slices"
	"strings"
	"testing"
)

// selectedSnapshot selects directory from the authored sources as the Linux
// build context does, with no Git repository and no registration walk, so a
// test can ask the static type resolver about the package it selected.
func selectedSnapshot(t testing.TB, directory string, sources map[string]string) *Snapshot {
	t.Helper()
	snapshot := &Snapshot{blobs: make(map[string][]byte), directories: make(map[string][]string), fset: token.NewFileSet(), packages: make(map[string]*sourcePackage)}
	for name, text := range sources {
		snapshot.blobs[name] = []byte(text)
		snapshot.directories[path.Dir(name)] = append(snapshot.directories[path.Dir(name)], name)
	}
	for _, files := range snapshot.directories {
		slices.Sort(files)
	}
	context := linuxContext(nil, snapshot.blobs)
	if err := snapshot.selectPackage(directory, &context); err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func selectedFile(t *testing.T, pkg *sourcePackage, name string) *ast.File {
	t.Helper()
	for _, file := range pkg.files {
		if file.Path == name {
			return file.File
		}
	}
	t.Fatalf("authored selected file %s is unavailable", name)
	return nil
}

// TestRangeVariableTypes pins valueType's answer for each range
// variable of each kind in the Go specification's range clause table,
// function iterators included. Method: a dormant retained
// function holds one loop per row; each variable's derived type is printed
// with types.ExprString, and "?" requires valueType to refuse.
//
// MUTATION: (internal/le/repo/inventory/methods.go,
// "case 0:\n\t\t\treturn at(kind.Key)", "case 0:\n\t\t\treturn at(kind.Value)")
// answers a map key with the element type: the map rows go red.
// MUTATION: (internal/le/repo/inventory/methods.go,
// "if position < len(yielded) {", "if position < len(yielded)-1 {") loses
// an iterator's last yielded type: the iterator rows go red.
// MUTATION: (internal/le/repo/inventory/methods.go,
// "case *ast.FuncLit:\n\t\treturn sourceExpr{value: value.Type, file: source.file, pkg: source.pkg}, true",
// "case *ast.FuncLit:\n\t\treturn sourceExpr{}, false") leaves a literal
// iterator untyped: its row goes red.
// MUTATION: (internal/le/repo/inventory/methods.go,
// "if binding.Kind == ast.Fun {", "if binding.Kind == ast.Bad {") leaves a
// declared iterator untyped: its rows go red.
// MUTATION: (internal/le/repo/inventory/methods.go,
// "if variadic, ok := field.Type.(*ast.Ellipsis); ok {",
// "if variadic, ok := field.Type.(*ast.Ellipsis); ok && false {")
// leaves a variadic parameter typed ...T: both variadic rows go red.
// MUTATION: (internal/le/repo/inventory/methods.go,
// "return e.sliceType(source, value, depth)", "return sourceExpr{}, false")
// leaves every slice expression untyped: the sliced rows go red.
func TestRangeVariableTypes(t *testing.T) {
	const declarations = "var table = map[label]int{\"a\": 1}\nvar list = []label{\"a\"}\nvar row = [1]label{\"a\"}\n" +
		"var named = labels{\"a\"}\nvar word = \"ab\"\nvar limit count = 2\nvar queue chan label = make(chan label, 1)\n" +
		"func pairs() map[string]label { return nil }\nfunc each(yield func(label, count) bool) {}\nfunc keys(yield func(label) bool) {}\n"
	rows := []struct{ loop, key, value string }{
		{"for k, v := range table { _, _ = k, v }", "label", "int"},
		{"for k := range table { _ = k }", "label", ""},
		{"for k, v := range list { _, _ = k, v }", "int", "label"},
		{"for _, v := range list { _ = v }", "", "label"},
		{"for k, v := range row { _, _ = k, v }", "int", "label"},
		{"for k, v := range &row { _, _ = k, v }", "int", "label"},
		{"for k, v := range named { _, _ = k, v }", "int", "label"},
		{"for k, v := range word { _, _ = k, v }", "int", "rune"},
		{"for k, v := range \"ab\" { _, _ = k, v }", "int", "rune"},
		{"for k := range 3 { _ = k }", "int", ""},
		{"for k := range limit { _ = k }", "count", ""},
		{"for k := range queue { _ = k }", "label", ""},
		{"for k, v := range pairs() { _, _ = k, v }", "string", "label"},
		{"for k, v := range (map[string]bool{}) { _, _ = k, v }", "string", "bool"},
		{"for k, v := range each { _, _ = k, v }", "label", "count"},
		{"for k := range keys { _ = k }", "label", ""},
		{"for k, v := range (func(yield func(int, string) bool) {}) { _, _ = k, v }", "int", "string"},
		{"for k, v := range remote.Environ() { _, _ = k, v }", "?", "?"},
		{"for k, v := range list[1:] { _, _ = k, v }", "int", "label"},
		{"for k, v := range list[0:1:1] { _, _ = k, v }", "int", "label"},
		{"for k, v := range named[1:] { _, _ = k, v }", "int", "label"},
		{"for k, v := range row[:] { _, _ = k, v }", "int", "label"},
		{"for k, v := range (&row)[:] { _, _ = k, v }", "int", "label"},
		{"for k, v := range word[1:] { _, _ = k, v }", "int", "rune"},
		{"for k, v := range \"ab\"[1:] { _, _ = k, v }", "int", "rune"},
		{"for k, v := range remote.Environ()[1:] { _, _ = k, v }", "?", "?"},
		{"_ = func(args ...label) { for k, v := range args { _, _ = k, v } }", "int", "label"},
		{"_ = func(args ...label) { for k, v := range args[1:] { _, _ = k, v } }", "int", "label"},
	}
	var loops strings.Builder
	for _, row := range rows {
		loops.WriteString("\t" + row.loop + "\n")
	}
	fixture := callableOwnerFixture(t, "plugin", callableOwnerCase{})
	source := "package ordinary\nimport remote \"os\"\n" + rangeTypeDeclarations + declarations + "func ranges() {\n" + loops.String() + "}\n"
	fixture.goFile(t, ordinaryDir+"/ranges.go", source)
	commitInventoryFixture(t, fixture.root)
	snapshot, err := ReadCommitted(t.Context(), fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	pkg, found := snapshot.packages[ordinaryDir]
	if !found {
		t.Fatalf("retained package %s is missing", ordinaryDir)
	}
	var statements []*ast.RangeStmt
	var file *ast.File
	for _, candidate := range pkg.files {
		for _, declaration := range candidate.File.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if function.Name.Name != "ranges" {
				continue
			}
			file = candidate.File
			ast.Inspect(function.Body, func(node ast.Node) bool {
				if statement, ok := node.(*ast.RangeStmt); ok {
					statements = append(statements, statement)
				}
				return true
			})
		}
	}
	if len(statements) != len(rows) {
		t.Fatalf("found %d range statements, want %d", len(statements), len(rows))
	}
	answer := func(variable ast.Expr) string {
		name, ok := variable.(*ast.Ident)
		if !ok {
			return ""
		}
		if name.Name == "_" {
			return ""
		}
		datatype, known := snapshot.evaluator().valueType(sourceExpr{value: name, file: file, pkg: pkg}, 0)
		if !known {
			return "?"
		}
		return types.ExprString(datatype.value)
	}
	for index, row := range rows {
		statement := statements[index]
		if got := answer(statement.Key); got != row.key {
			t.Errorf("%s: key type %q, want %q", row.loop, got, row.key)
		}
		if got := answer(statement.Value); got != row.value {
			t.Errorf("%s: value type %q, want %q", row.loop, got, row.value)
		}
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
	snapshot := selectedSnapshot(t, directory, map[string]string{
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
	pkg := snapshot.packages[directory]
	file := selectedFile(t, pkg, filename)
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
	resolver := snapshot.evaluator()
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
	snapshot := selectedSnapshot(t, directory, map[string]string{
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
	pkg := snapshot.packages[directory]
	file := selectedFile(t, pkg, filename)
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
	resolver := snapshot.evaluator()
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
	snapshot := selectedSnapshot(t, directory, map[string]string{
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
	pkg := snapshot.packages[directory]
	file := selectedFile(t, pkg, filename)
	declared := make(map[string]*ast.Ident)
	ast.Inspect(file, func(node ast.Node) bool {
		if name, ok := node.(*ast.Ident); ok {
			if _, seen := declared[name.Name]; !seen {
				declared[name.Name] = name
			}
		}
		return true
	})
	resolver := snapshot.evaluator()
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
