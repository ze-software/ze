// Design: docs/architecture/plugin/feature-gates.md -- a range over a slice expression has the variables the Go specification gives it.
package repoinventory

import "testing"

// TestCommittedExternalRangeOverSlicePublishes requires a range over a slice
// expression to publish, in the shape that refused at gopls's
// internal/cmd/execute.go:81: `for i, arg := range args[1:]` where args is
// the variadic parameter `args ...string`, both variables reaching external
// code. Method: each row ranges a slice expression and hands both range
// variables to opaque calls, calling a retained method on each element, so a
// lost key or element type refuses as range capability ownership or as an
// unresolved method. The retained package slice row is the control that a
// slice of a declared slice already typed; the variadic row is the gopls
// shape, called with separate arguments, called with a spread slice, and as
// the parameter of a callback handed to an opaque call. Each variadic row has
// a twin with an ordinary []label parameter, called with the slice literal or
// the slice the variadic call passes, because Go makes the two equivalent: a
// variadic row that refuses while its twin publishes is a variadic gap. The
// callback rows do not read their parameter: a parameter read in a callback
// handed to an opaque call refuses for any type, as indirect caller
// provenance.
//
// MUTATION: (internal/le/repo/inventory/methods.go,
// "if variadic, ok := field.Type.(*ast.Ellipsis); ok {",
// "if variadic, ok := field.Type.(*ast.Ellipsis); ok && false {")
// leaves the variadic parameter typed ...T, which slices to nothing: the
// variadic rows refuse.
// MUTATION: (internal/le/repo/inventory/methods.go,
// "return e.sliceType(source, value, depth)", "return sourceExpr{}, false")
// leaves every slice expression untyped: the sliced rows refuse.
// MUTATION: (internal/le/repo/inventory/externalbindings.go,
// "e.types[name.Obj] = sourceExpr{value: parameterType(declaration), file: file.File, pkg: pkg}",
// "e.types[name.Obj] = sourceExpr{value: declaration.Type, file: file.File, pkg: pkg}")
// records a variadic parameter as ...T, so a read of it meets a type walk
// with no ...T arm: the directly called variadic rows refuse.
// MUTATION: (internal/le/repo/inventory/externalbindings.go,
// "datatype := sourceExpr{value: parameterType(field), file: source.file, pkg: source.pkg}",
// "datatype := sourceExpr{value: field.Type, file: source.file, pkg: source.pkg}")
// walks an externally invoked callback's parameter as ...T: the exposed
// callback row refuses.
// MUTATION: (internal/le/repo/inventory/externalcalls.go,
// "spreadBound = e.bindVariadic(field, arguments[min(index, len(arguments)):], spread)",
// "e.problem(source, \"variadic parameter ownership is unresolved\")")
// restores the refusal of a direct call to a variadic function: the directly
// called rows refuse.
// MUTATION: (internal/le/repo/inventory/externalformals.go,
// "e.indexVariadicFormal(field, call.Args[index:], spread, caller, owner)",
// "e.formalInvalid[node] = \"variadic parameter provenance is unsupported\"")
// leaves a variadic parameter without caller provenance: the directly called
// variadic rows refuse as having no grounded retained argument, while their
// twins publish.
// MUTATION: (internal/le/repo/inventory/externalformalproof.go,
// "passed := &externalFormalProof{grounded: true}",
// "passed := &externalFormalProof{}")
// leaves a call passing no arguments ungrounded, since nothing else grounds
// the nil it passes: the no-argument row refuses and its nil twin publishes.
// With arguments the elements' own proofs ground the slice, so the
// separate-argument rows stay green under this break.
// MUTATION: (internal/le/repo/inventory/externalcalls.go,
// "if len(remaining) != 1 {\n\t\t\treturn false",
// "if len(remaining) == 1 {\n\t\t\treturn false")
// refuses a spread of exactly one slice: the spread row refuses.
func TestCommittedExternalRangeOverSlicePublishes(t *testing.T) {
	const exposeCounts = "order.Slice(counts, func(i, j int) bool { return false })"
	for _, shape := range []struct{ name, declarations, call string }{
		{
			name:         "retained slice",
			declarations: "var list = []label{\"a\", \"b\"}\n",
			call: "var names []string; var counts []int; " +
				"for index, value := range list[1:] { counts = append(counts, index); names = append(names, value.text()) }; " +
				"order.Slice(names, func(i, j int) bool { return false }); " + exposeCounts,
		},
		{
			name: "variadic parameter",
			declarations: "func tail(args ...label) []string { var names []string; var counts []int; " +
				"for index, arg := range args[1:] { counts = append(counts, index); names = append(names, arg.text()) }; " +
				exposeCounts + "; return names }\n",
			call: "names := tail(\"a\", \"b\"); order.Slice(names, func(i, j int) bool { return false })",
		},
		{
			name: "whole variadic parameter",
			declarations: "func every(args ...label) []string { var names []string; var counts []int; " +
				"for index, arg := range args { counts = append(counts, index); names = append(names, arg.text()) }; " +
				exposeCounts + "; return names }\n",
			call: "names := every(\"a\", \"b\"); order.Slice(names, func(i, j int) bool { return false })",
		},
		{
			name: "spread argument",
			declarations: "var list = []label{\"a\", \"b\"}\nfunc spread(args ...label) []string { var names []string; var counts []int; " +
				"for index, arg := range args[1:] { counts = append(counts, index); names = append(names, arg.text()) }; " +
				exposeCounts + "; return names }\n",
			call: "names := spread(list...); order.Slice(names, func(i, j int) bool { return false })",
		},
		{
			name: "variadic parameter, no arguments",
			declarations: "func none(args ...label) []string { var names []string; var counts []int; " +
				"for index, arg := range args { counts = append(counts, index); names = append(names, arg.text()) }; " +
				exposeCounts + "; return names }\n",
			call: "names := none(); order.Slice(names, func(i, j int) bool { return false })",
		},
		{
			name: "slice parameter twin, nil argument",
			declarations: "func none(args []label) []string { var names []string; var counts []int; " +
				"for index, arg := range args { counts = append(counts, index); names = append(names, arg.text()) }; " +
				exposeCounts + "; return names }\n",
			call: "names := none(nil); order.Slice(names, func(i, j int) bool { return false })",
		},
		{
			name:         "exposed variadic callback",
			declarations: "",
			call:         "var names []string; order.Slice(names, func(args ...label) bool { return false })",
		},
		{
			name: "slice parameter twin, literal argument",
			declarations: "func tail(args []label) []string { var names []string; var counts []int; " +
				"for index, arg := range args[1:] { counts = append(counts, index); names = append(names, arg.text()) }; " +
				exposeCounts + "; return names }\n",
			call: "names := tail([]label{\"a\", \"b\"}); order.Slice(names, func(i, j int) bool { return false })",
		},
		{
			name: "slice parameter twin, slice argument",
			declarations: "var list = []label{\"a\", \"b\"}\nfunc tail(args []label) []string { var names []string; var counts []int; " +
				"for index, arg := range args[1:] { counts = append(counts, index); names = append(names, arg.text()) }; " +
				exposeCounts + "; return names }\n",
			call: "names := tail(list); order.Slice(names, func(i, j int) bool { return false })",
		},
		{
			name:         "exposed slice callback twin",
			declarations: "",
			call:         "var names []string; order.Slice(names, func(args []label) bool { return false })",
		},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(externalRangeTypeDeclarations+shape.declarations, shape.call))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)
			})
		}
	}
}
