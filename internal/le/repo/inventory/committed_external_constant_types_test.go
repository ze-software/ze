// Design: docs/architecture/plugin/feature-gates.md -- a constant and a conversion have the types the Go specification gives them.
package repoinventory

import (
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
