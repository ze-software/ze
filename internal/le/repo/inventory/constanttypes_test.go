// Design: docs/architecture/plugin/feature-gates.md -- a constant and a conversion have the types the Go specification gives them.
package repoinventory

import (
	"go/ast"
	"go/types"
	"testing"
)

// rangeTypeDeclarations are retained types whose methods a ranged
// value calls, so a range variable whose type is lost leaves its method call
// unresolved instead of publishing.
const rangeTypeDeclarations = "type label string\nfunc (l label) text() string { return string(l) }\n" +
	"type count int\nfunc (c count) text() string { return \"\" }\ntype labels []label\n"

// TestConstantTypes pins valueType's answer for each constant form
// and conversion the Go specification types: a typed constant, iota groups
// with omitted expressions (package and local), typed repetition, constant
// expressions mixing kinds, shifts, comparisons, unary operators, rune and
// string constants, and conversions. Method: a dormant retained function
// assigns each expression to the blank identifier; the derived type of each
// right-hand side is printed with types.ExprString, and "?" requires
// valueType to refuse.
//
// MUTATION: (internal/le/repo/inventory/constanttypes.go,
// "return untypedConstant(source, max(leftKind, rightKind))",
// "return untypedConstant(source, min(leftKind, rightKind))") ranks the
// kinds backwards: the mixed integer and float row goes red.
// MUTATION: (internal/le/repo/inventory/constanttypes.go,
// "if leftKind == constantTyped {", "if leftKind == constantUnspecified {")
// lets a typed left operand lose: the small + 1 row goes red.
// MUTATION: (internal/le/repo/inventory/constanttypes.go,
// "if rightKind == constantTyped {", "if rightKind == constantUnspecified {")
// lets a typed right operand lose: the 1 + small row goes red.
// MUTATION: (internal/le/repo/inventory/constanttypes.go,
// "case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:",
// "case token.EQL, token.NEQ, token.LEQ, token.GTR, token.GEQ:") types 1 < 2
// as its operands: the comparison row goes red.
// MUTATION: (internal/le/repo/inventory/constanttypes.go,
// "if leftKind == untypedFloat {", "if leftKind == untypedComplex {") keeps
// a float shift a float: the 1.0 << 3 row goes red.
// MUTATION: (internal/le/repo/inventory/constanttypes.go,
// "return \"rune\", true", "return \"int32\", true") spells the rune
// default by its alias target: the rune rows go red.
// MUTATION: (internal/le/repo/inventory/constanttypes.go,
// "case types.UntypedInt:\n\t\treturn untypedInt, true",
// "case types.UntypedFloat:\n\t\treturn untypedInt, true") leaves iota
// untyped: the iota rows go red.
// MUTATION: (internal/le/repo/inventory/constanttypes.go,
// "return true\n\t})\n\treturn group, group != nil",
// "return false\n\t})\n\treturn group, group != nil") never finds a local
// const group: the local iota row goes red.
// MUTATION: (internal/le/repo/inventory/constanttypes.go,
// "case token.ADD, token.SUB, token.XOR, token.NOT:",
// "case token.ADD, token.SUB, token.XOR:") leaves a negation untyped: the
// !truthy row goes red.
// MUTATION: (internal/le/repo/inventory/constanttypes.go,
// "if leftKind == rightKind {", "if leftKind != rightKind {") refuses two
// untyped operands of one kind: the string concatenation row goes red.
// MUTATION: (internal/le/repo/inventory/methods.go,
// "// (*T)(x) converts to a pointer type when T names a type.\n\t\t\tif !e.namesType(",
// "// (*T)(x) converts to a pointer type when T names a type.\n\t\t\tif e.namesType(")
// refuses a pointer conversion: the (*label)(nil) row goes red.
func TestConstantTypes(t *testing.T) {
	const declarations = "const small uint8 = 4\n" +
		"const (\n\tfirst = iota\n\tsecond\n\tthird\n)\n" +
		"const (\n\tlow count = iota\n\thigh\n)\n" +
		"const mask = 1<<8 - 1\nconst letter = 'a'\nconst greeting = \"ab\"\nconst truthy = true\n" +
		"const far = remote.PathSeparator\n"
	rows := []struct{ expression, want string }{
		{"small", "uint8"},
		{"first", "int"},
		{"third", "int"},
		{"high", "count"},
		{"mask", "int"},
		{"-mask", "int"},
		{"letter", "rune"},
		{"letter + 1", "rune"},
		{"greeting", "string"},
		{"greeting + \"c\"", "string"},
		{"2.5", "float64"},
		{"1 + 2.5", "float64"},
		{"1i", "complex128"},
		{"1.0 << 3", "int"},
		{"small + 1", "uint8"},
		{"1 + small", "uint8"},
		{"1 < 2", "bool"},
		{"truthy", "bool"},
		{"!truthy", "bool"},
		{"localSecond", "int"},
		{"uint8(3)", "uint8"},
		{"count(2)", "count"},
		{"(*label)(nil)", "*label"},
		{"far", "?"},
		{"far + 1", "?"},
	}
	body := "\tconst (\n\t\tlocalFirst = iota\n\t\tlocalSecond\n\t)\n"
	for _, row := range rows {
		body += "\t_ = " + row.expression + "\n"
	}
	fixture := callableOwnerFixture(t, "plugin", callableOwnerCase{})
	source := "package ordinary\nimport remote \"os\"\n" + rangeTypeDeclarations + declarations + "func constantForms() {\n" + body + "}\n"
	fixture.goFile(t, ordinaryDir+"/constants.go", source)
	commitInventoryFixture(t, fixture.root)
	snapshot, err := ReadCommitted(t.Context(), fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	pkg, found := snapshot.packages[ordinaryDir]
	if !found {
		t.Fatalf("retained package %s is missing", ordinaryDir)
	}
	var expressions []ast.Expr
	var file *ast.File
	for _, candidate := range pkg.files {
		for _, declaration := range candidate.File.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if function.Name.Name != "constantForms" {
				continue
			}
			file = candidate.File
			for _, statement := range function.Body.List {
				assignment, ok := statement.(*ast.AssignStmt)
				if !ok {
					continue
				}
				expressions = append(expressions, assignment.Rhs[0])
			}
		}
	}
	if len(expressions) != len(rows) {
		t.Fatalf("found %d blank assignments, want %d", len(expressions), len(rows))
	}
	for index, row := range rows {
		got := "?"
		datatype, known := snapshot.evaluator().valueType(sourceExpr{value: expressions[index], file: file, pkg: pkg}, 0)
		if known {
			got = types.ExprString(datatype.value)
		}
		if got != row.want {
			t.Errorf("%s: type %q, want %q", row.expression, got, row.want)
		}
	}
}
