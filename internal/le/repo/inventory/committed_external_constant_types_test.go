// Design: docs/architecture/plugin/feature-gates.md -- a constant and a conversion have the types the Go specification gives them.
package repoinventory

import (
	"go/ast"
	"go/types"
	"testing"
)

// TestCommittedExternalRangeConstantsPublish requires a range over a constant
// or over a converted value to publish. The first row is the shape that
// refused in vendor/github.com/klauspost/compress/huff0/huff0.go:204: a local
// variable declared from a conversion, uint8(...), which shadows a package
// constant of the same name and is ranged, each index reaching external code.
// The other rows range a package constant with no declared type, an iota
// group member with its expression omitted, a typed iota group member whose
// type is repeated, and a variable converted to a retained named type.
// Method: each loop's variables reach an opaque call, and a named variable
// calls a retained method, so a lost type refuses as range capability
// ownership or as an unresolved method.
//
// MUTATION: (internal/le/repo/inventory/methods.go,
// "return e.conversionType(sourceExpr{value: value.Fun, file: source.file, pkg: source.pkg})",
// "return sourceExpr{}, false") leaves a conversion untyped: the huff0 and
// named conversion rows refuse.
// MUTATION: (internal/le/repo/inventory/methods.go,
// "if binding.Kind == ast.Con {\n\t\t\treturn e.constantDefault(source, depth)",
// "if binding.Kind == ast.Bad {\n\t\t\treturn e.constantDefault(source, depth)")
// leaves every declared constant untyped: the constant rows refuse.
// MUTATION: (internal/le/repo/inventory/constanttypes.go,
// "if len(preceding.Values) != 0 {", "if len(preceding.Values) == 0 {")
// loses the implicit repetition: the two iota group rows refuse.
// MUTATION: (internal/le/repo/inventory/constanttypes.go,
// "return sourceExpr{value: spec.Type, file: source.file, pkg: source.pkg}, constantTyped, true",
// "return e.constantType(sourceExpr{value: spec.Values[index], file: source.file, pkg: source.pkg}, depth)")
// drops a constant's declared type for its untyped expression: the typed iota
// row gets int, which has no text method, and refuses.
// MUTATION: (internal/le/repo/inventory/externalranges.go,
// "return walk.constantKind(source, depth+1)",
// "return externalRangeUnresolved, \"range constant type is unresolved\"")
// stops range execution typing a constant: the constant rows refuse as range
// execution ownership.
func TestCommittedExternalRangeConstantsPublish(t *testing.T) {
	for _, shape := range []struct{ name, declarations, loop string }{
		{"huff0 local conversion", "const symbolMax = 255\n",
			"var symbolMax = uint8(3); var widths []uint8; for index := range symbolMax { widths = append(widths, index) }; " +
				"order.Slice(widths, func(i, j int) bool { return false })"},
		{"untyped package constant", "const symbolCount = 255\n",
			"for index := range symbolCount { counts = append(counts, index) }"},
		{"iota group", "const (\n\tlevelFirst = iota\n\tlevelSecond\n)\n",
			"for index := range levelSecond { counts = append(counts, index) }"},
		{"typed iota group", "const (\n\tcountLow count = iota\n\tcountHigh\n)\n",
			"for index := range countHigh { names = append(names, index.text()) }"},
		{"named conversion", "var converted = count(2)\n",
			"for index := range converted { names = append(names, index.text()) }"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				call := "var names []string; var counts []int; var runes []rune; " + shape.loop + "; " + externalRangeExposed
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(externalRangeTypeDeclarations+shape.declarations, call))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)
			})
		}
	}
}

// TestCommittedExternalRangeUnresolvedConstantRefuses keeps the refusal for a
// range over a constant whose value names something no retained declaration
// types: os.PathSeparator, from a package the inventory does not retain.
// Method: the range variable reaches an opaque call, and the report must
// refuse with today's reason.
//
// MUTATION: (internal/le/repo/inventory/constanttypes.go,
// "datatype, found := e.valueType(source, depth+1)\n\treturn datatype, constantTyped, found\n}",
// "_, _ = e.valueType, depth\n\treturn untypedConstant(source, untypedRune)\n}")
// answers every operand constantType cannot type as an untyped rune: the
// unresolved constant publishes.
func TestCommittedExternalRangeUnresolvedConstantRefuses(t *testing.T) {
	const declarations = "const separator = remote.PathSeparator\n"
	const call = "var runes []rune; for index := range separator { runes = append(runes, index) }; order.Slice(runes, func(i, j int) bool { return false })"
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(declarations, call))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "range capability ownership is unsupported")
		})
	}
}

// TestExternalConstantTypes pins valueType's answer for each constant form
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
func TestExternalConstantTypes(t *testing.T) {
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
	source := "package ordinary\nimport remote \"os\"\n" + externalRangeTypeDeclarations + declarations + "func constantForms() {\n" + body + "}\n"
	fixture.goFile(t, asmOrdinaryDir+"/constants.go", source)
	commitInventoryFixture(t, fixture.root)
	snapshot, err := ReadCommitted(t.Context(), fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	pkg, found := snapshot.packages[asmOrdinaryDir]
	if !found {
		t.Fatalf("retained package %s is missing", asmOrdinaryDir)
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
