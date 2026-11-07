// Design: docs/architecture/plugin/feature-gates.md -- a range variable has the type the Go specification gives it.
package repoinventory

import (
	"go/ast"
	"go/types"
	"strings"
	"testing"
)

// externalRangeTypeDeclarations are retained types whose methods a ranged
// value calls, so a range variable whose type is lost leaves its method call
// unresolved instead of publishing.
const externalRangeTypeDeclarations = "type label string\nfunc (l label) text() string { return string(l) }\n" +
	"type count int\nfunc (c count) text() string { return \"\" }\ntype labels []label\n"

// externalRangeExposed hands every accumulator the loops append to an opaque
// call, so each ranged value a loop read reaches external code.
const externalRangeExposed = "order.Slice(names, func(i, j int) bool { return false }); " +
	"order.Slice(counts, func(i, j int) bool { return false }); " +
	"order.Slice(runes, func(i, j int) bool { return false })"

// TestCommittedExternalRangeMapKeysPublish requires the shape that refused in
// internal/exabgp/bridge/bridge_family.go:106 to publish: a package map
// declared from a composite literal with no type, ranged for its keys alone,
// each key appended to a slice that is returned and handed to an opaque call.
// Method: the report and plugins must match the ordinary control's.
//
// MUTATION: (internal/le/repo/inventory/methods.go,
// "case 0:\n\t\t\treturn at(kind.Key)", "case 2:\n\t\t\treturn at(kind.Key)")
// loses the map key's type: the key refuses as range capability ownership.
func TestCommittedExternalRangeMapKeysPublish(t *testing.T) {
	const declarations = "var table = map[string]string{\"unicast\": \"unicast\", \"mpls-vpn\": \"mpls-vpn\"}\n" +
		"func tableKeys() []string { names := make([]string, 0, len(table)); for name := range table { names = append(names, name) }; return names }\n"
	const call = "keys := tableKeys(); order.Slice(keys, func(i, j int) bool { return false })"
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(declarations, call))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
		})
	}
}

// TestCommittedExternalRangeVariableKindsPublish requires every range
// variable of every non-iterator range kind to carry its specified type.
// Method: each loop reads both of its variables into accumulators that reach
// an opaque call, and calls a retained method on each ranged value whose type
// has one, so a lost type refuses either as range capability ownership or as
// an unresolved method. The channel row passed before; it is the control that
// the shared helper still answers a channel's single variable.
//
// MUTATION: (internal/le/repo/inventory/methods.go,
// "case 1:\n\t\t\treturn at(kind.Value)", "case 2:\n\t\t\treturn at(kind.Value)")
// loses the map element: the map rows refuse.
// MUTATION: (internal/le/repo/inventory/methods.go,
// "case 1:\n\t\t\treturn at(kind.Elt)", "case 2:\n\t\t\treturn at(kind.Elt)")
// loses the slice and array element: those rows refuse.
// MUTATION: (internal/le/repo/inventory/methods.go,
// "if array.Len == nil {", "if array.Len != nil {") refuses the pointer
// to an array.
// MUTATION: (internal/le/repo/inventory/methods.go,
// "source = sourceExpr{value: definition.Type, file: file, pkg: owner}",
// "source = sourceExpr{value: definition.Name, file: file, pkg: owner}")
// never reaches a named type's declaration: the named slice and named integer
// rows refuse.
// MUTATION: (internal/le/repo/inventory/methods.go,
// "return universeType(shape, \"rune\")", "return sourceExpr{}, false")
// loses the string's rune: the string rows refuse.
// MUTATION: (internal/le/repo/inventory/methods.go,
// "case token.INT:\n\t\t\treturn universeType(source, \"int\")",
// "case token.INT:\n\t\t\treturn sourceExpr{}, false") leaves an integer
// literal untyped: the integer literal row refuses.
// MUTATION: (internal/le/repo/inventory/methods.go,
// "return named, true", "return shape, true") gives a named integer's
// variable the underlying int, which has no text method: the row refuses.
// MUTATION: (internal/le/repo/inventory/methods.go,
// "case *ast.ChanType:\n\t\tif position == 0 {",
// "case *ast.ChanType:\n\t\tif position == 1 {") loses the received element:
// the channel row refuses.
// MUTATION: (internal/le/repo/inventory/methods.go,
// "if function.decl.Type.Results.NumFields() != 1 {",
// "if function.decl.Type.Results.NumFields() != 2 {") loses a call's declared
// result: the call row refuses.
func TestCommittedExternalRangeVariableKindsPublish(t *testing.T) {
	for _, shape := range []struct{ name, declarations, loop string }{
		{"map key and element", "var table = map[label]label{\"a\": \"b\"}\n",
			"for key, value := range table { names = append(names, key.text(), value.text()) }"},
		{"slice index and element", "var list = []label{\"a\"}\n",
			"for index, value := range list { counts = append(counts, index); names = append(names, value.text()) }"},
		{"named slice", "var named = labels{\"a\"}\n",
			"for index, value := range named { counts = append(counts, index); names = append(names, value.text()) }"},
		{"array", "var row = [1]label{\"a\"}\n",
			"for index, value := range row { counts = append(counts, index); names = append(names, value.text()) }"},
		{"pointer to array", "var row = [1]label{\"a\"}\n",
			"for index, value := range &row { counts = append(counts, index); names = append(names, value.text()) }"},
		{"call result", "func pairs() map[label]label { return nil }\n",
			"for key, value := range pairs() { names = append(names, key.text(), value.text()) }"},
		{"string", "var word = \"ab\"\n",
			"for index, value := range word { counts = append(counts, index); runes = append(runes, value) }"},
		{"string literal", "",
			"for index, value := range \"ab\" { counts = append(counts, index); runes = append(runes, value) }"},
		{"integer literal", "",
			"for index := range 3 { counts = append(counts, index) }"},
		{"named integer", "var limit count = 2\n",
			"for index := range limit { names = append(names, index.text()) }"},
		{"channel", "var queue chan label = make(chan label, 1)\n",
			"for value := range queue { names = append(names, value.text()) }"},
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

// TestCommittedExternalRangedMapElementCarriesRegistration requires the
// element of a map range to be what the map holds. Method: a map literal holds
// a function as any; the element is called inside a callback handed to an
// opaque call, either in the loop or after it through a variable the loop
// assigned. A quiet function publishes and a registering one refuses as
// escaped, so the refusal comes from what the map carried.
//
// MUTATION: (internal/le/repo/inventory/externalexpressions.go,
// "result := child(ranged.X)", "result := externalValue{}") makes a ranged
// element carry nothing: the armed function publishes.
func TestCommittedExternalRangedMapElementCarriesRegistration(t *testing.T) {
	for _, shape := range []struct{ name, call string }{
		{
			name: "in-range",
			call: "hooks := map[string]any{\"a\": FN}; _ = clock.AfterFunc(0, func() { for _, held := range hooks { held.(func())() } })",
		},
		{
			name: "after-range",
			call: "hooks := map[string]any{\"a\": FN}; var last any; for _, held := range hooks { last = held }; _ = clock.AfterFunc(0, func() { last.(func())() })",
		},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				externalChannelArmed(t, kind, "", shape.call)
			})
		}
	}
}

// TestCommittedExternalRangeUntypedOperandRefuses keeps the refusal for a
// range variable whose operand valueType cannot type. Method: the range kind
// is proved (a slice field of a retained struct), so the range itself runs,
// but no declaration gives the selected field's value a static type here; the
// value then reaches an opaque call, and the report must refuse with today's
// reason.
//
// MUTATION: (internal/le/repo/inventory/externalexpressions.go,
// "return externalValue{unknown: \"range capability ownership is unsupported\"}",
// "return externalValue{}") publishes the untyped variable.
func TestCommittedExternalRangeUntypedOperandRefuses(t *testing.T) {
	const declarations = "type holder struct{ values []string }\nvar box = holder{values: []string{\"a\"}}\n"
	const call = "var names []string; for _, value := range box.values { names = append(names, value) }; order.Slice(names, func(i, j int) bool { return false })"
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(declarations, call))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "range capability ownership is unsupported")
		})
	}
}

// TestExternalRangeVariableTypes pins valueType's answer for each range
// variable of each kind in the Go specification's range clause table,
// function iterators included, which the published report never reaches
// because function-range execution refuses first. Method: a dormant retained
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
func TestExternalRangeVariableTypes(t *testing.T) {
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
	}
	var loops strings.Builder
	for _, row := range rows {
		loops.WriteString("\t" + row.loop + "\n")
	}
	fixture := callableOwnerFixture(t, "plugin", callableOwnerCase{})
	source := "package ordinary\nimport remote \"os\"\n" + externalRangeTypeDeclarations + declarations + "func ranges() {\n" + loops.String() + "}\n"
	fixture.goFile(t, asmOrdinaryDir+"/ranges.go", source)
	commitInventoryFixture(t, fixture.root)
	snapshot, err := ReadCommitted(t.Context(), fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	pkg, found := snapshot.packages[asmOrdinaryDir]
	if !found {
		t.Fatalf("retained package %s is missing", asmOrdinaryDir)
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
