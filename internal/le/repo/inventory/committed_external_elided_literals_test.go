// Design: docs/architecture/plugin/feature-gates.md -- elided element literals take their enclosing literal's type.
package repoinventory

import (
	"strconv"
	"strings"
	"testing"
)

// externalElidedLiteralShape is one composite literal whose elements elide
// their type, passed whole to an opaque call. call replaces that call when the
// literal reaches the opaque call through a retained function's parameter.
type externalElidedLiteralShape struct {
	name, declaration, value, call string
}

const externalElidedLiteralTypes = "type point struct{ x int }\ntype label struct{ name string }\ntype renameRule struct{ old, new string }\n"

func externalElidedLiteralShapes() []externalElidedLiteralShape {
	return []externalElidedLiteralShape{
		{name: "htmx-var", declaration: "var renames = []renameRule{\n\t{old: \"htmx:afterOnLoad\", new: \"htmx:load\"},\n\t{old: \"htmx:beforeOnLoad\", new: \"htmx:beforeLoad\"},\n}\n", value: "renames"},
		{name: "htmx-inline", value: `[]renameRule{{old: "a", new: "b"}}`},
		{name: "positional", value: `[]renameRule{{"a", "b"}}`},
		{name: "map-key-value", value: `map[point]label{{x: 1}: {name: "a"}}`},
		{name: "pointer-element", value: `[]*point{{1}}`},
		{name: "nested", value: `[][]point{{{x: 1}}}`},
		{name: "array-index", value: `[...]point{2: {x: 1}}`},
		{name: "named-collection", declaration: "type points []point\n", value: `points{{x: 1}}`},
		{name: "generic-collection", declaration: "type list[E any] []E\n", value: `list[point]{{x: 1}}`},
		{name: "generic-pointer", declaration: "type refs[E any] []*E\n", value: `refs[point]{{1}}`},
		{name: "standard-library", value: `address.Values{"k": {"v"}}`},
		{
			name:        "formal-parameter",
			declaration: "func pass(rules []renameRule) { order.Slice(rules, func(i, j int) bool { return false }) }\n",
			call:        `pass([]renameRule{{old: "a", new: "b"}})`,
		},
	}
}

// externalElidedLiteralFixture adds the import-cycle witness that certifies
// net/url as an opaque origin, beside the fixture's own.
func externalElidedLiteralFixture(t *testing.T, kind string) *asmInventoryFixture {
	t.Helper()
	fixture := externalEffectsFixture(t, kind)
	for _, sink := range []struct{ directory, name string }{
		{registrationPackage, "registry"},
		{"internal/component/config/yang", "yang"},
	} {
		fixture.goFile(t, sink.directory+"/elided_literal_witness.go", "package "+sink.name+"\nimport _ \"net/url\"\n")
	}
	return fixture
}

func externalElidedLiteralSource(shape externalElidedLiteralShape) string {
	call := shape.call
	if call == "" {
		call = "order.Slice(" + shape.value + ", func(i, j int) bool { return false })"
	}
	return callableOwnerSource(callableOwnerCase{
		declarations: externalElidedLiteralTypes + shape.declaration,
		extraImport:  "order \"sort\"\n address \"net/url\"",
		prefix:       "_ = order.Slice; _ = address.Values{}\n",
	}, call)
}

// externalElidedLiteralRefusal requires a refusal caused by the shape itself,
// never by the elided type being unresolved: the elided refusal also names
// "registration", so the reason alone would pass against the defect.
func externalElidedLiteralRefusal(t *testing.T, fixture *asmInventoryFixture, reason string) {
	t.Helper()
	externalEffectsRefusal(t, fixture, reason)
	_, err := Collect(fixture.root)
	if strings.Contains(err.Error(), "elided literal") {
		t.Fatalf("refusal comes from the elided type, not from the element: %v", err)
	}
}

// TestCommittedExternalElidedLiteralsPublish requires an element, key or value
// literal that elides its type to take the type its enclosing literal states,
// so the ordinary report publishes. Method: each shape, internal/le/web/htmx's
// eventRenames among them, is passed whole to an opaque call and must produce
// the same report and plugins as the control.
//
// MUTATION: (internal/le/repo/inventory/externalliteralshapes.go,
// "stated, known := elidedLiteralType(enclosing, mapKey)",
// "stated, known := elidedLiteralType(externalLiteralLayout{}, mapKey)")
// refuses every shape with the elided reason.
// MUTATION: (internal/le/repo/inventory/externalliteralshapes.go,
// "stated = enclosing.key", "stated = enclosing.element") refuses
// map-key-value: the point key is read as a label.
// MUTATION: (internal/le/repo/inventory/externalliteralshapes.go,
// "stated.value = pointer.X", "stated.value = pointer") refuses
// pointer-element with the positional pointer reason.
// MUTATION: (internal/le/repo/inventory/externalliteralshapes.go,
// "\t\t\treturn argument\n", "\t\t\treturn sourceExpr{}\n") refuses
// generic-collection.
// MUTATION: (internal/le/repo/inventory/externalliteralshapes.go,
// "return sourceExpr{value: &ast.StarExpr{X: argument.value}, file: argument.file, pkg: argument.pkg}",
// "return sourceExpr{}") refuses generic-pointer.
// MUTATION: (internal/le/repo/inventory/externalliteralshapes.go,
// "return externalLiteralType{opaque: true}, true",
// "return externalLiteralType{}, false") refuses standard-library.
// MUTATION: (internal/le/repo/inventory/externalformalexpressions.go,
// "return proofs.literal(value, nested, elided, depth+1)",
// "return proofs.expression(value, depth+1)") refuses formal-parameter: the
// field name old is proved as an argument identifier.
func TestCommittedExternalElidedLiteralsPublish(t *testing.T) {
	for _, shape := range externalElidedLiteralShapes() {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalElidedLiteralFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalElidedLiteralSource(shape))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)
			})
		}
	}
}

// TestCommittedExternalElidedLiteralRegisters requires an elided element to be
// judged, not only typed: a field holding a registering function still
// refuses. Method: each shape publishes with a quiet function in the field,
// then must refuse with "registration", and not for the elided type, once the
// field holds a function that installs.
//
// MUTATION: (internal/le/repo/inventory/externalliteralshapes.go,
// "result := e.typedLiteralValue(child, nested, stated, possible, depth+1)",
// "result := e.typeMethods(stated.datatype, depth+1)") publishes every shape:
// the elided element's field values are never evaluated.
func TestCommittedExternalElidedLiteralRegisters(t *testing.T) {
	const declarations = "type hook struct{ fn func() }\ntype list[E any] []E\nfunc quiet() {}\nfunc armed() { install(0) }\n"
	for _, shape := range []struct{ name, value string }{
		{"slice", "[]hook{{fn: FN}}"},
		{"positional", "[]hook{{FN}}"},
		{"pointer", "[]*hook{{fn: FN}}"},
		{"map-value", `map[string]hook{"k": {fn: FN}}`},
		{"nested", "[][]hook{{{fn: FN}}}"},
		{"generic", "list[hook]{{fn: FN}}"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalElidedLiteralFixture(t, kind)
				quiet := externalElidedLiteralShape{declaration: declarations, value: strings.Replace(shape.value, "FN", "quiet", 1)}
				fixture.goFile(t, callableOwnerCaller, externalElidedLiteralSource(quiet))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)
				armed := externalElidedLiteralShape{declaration: declarations, value: strings.Replace(shape.value, "FN", "armed", 1)}
				fixture.goFile(t, callableOwnerCaller, externalElidedLiteralSource(armed))
				commitInventoryFixture(t, fixture.root)
				externalElidedLiteralRefusal(t, fixture, "registration")
			})
		}
	}
}

// TestCommittedExternalElidedLiteralUnstatedStillRefuses keeps the refusal
// where the enclosing type cannot state the elided one: a generic collection
// whose element type names its type parameter inside a larger type
// (map[string]E). The refusal keeps today's message and names the element's
// line. Method: an empty elided element of the same collection publishes,
// since it constructs zero; a nonempty one must refuse at its own line.
//
// MUTATION: (internal/le/repo/inventory/externalliteralshapes.go,
// "if mentioned {", "if mentioned && false {") stops substituting into an
// unread scope, so the refusal changes reason.
// MUTATION: (internal/le/repo/inventory/externalliteralshapes.go,
// "const externalElidedLiteralUnresolved = \"nonempty elided literal layout is unresolved\"",
// "const externalElidedLiteralUnresolved = \"elided literal layout is unresolved\"")
// changes today's message.
// MUTATION: (internal/le/repo/inventory/externalvalues.go,
// "v.unknownAt = source", "v.unknownAt = sourceExpr{}") drops the site.
func TestCommittedExternalElidedLiteralUnstatedStillRefuses(t *testing.T) {
	const grid = "type grid[E any] []map[string]E\n"
	const element = `{"k": {x: 1}},`
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalElidedLiteralFixture(t, kind)
			empty := externalElidedLiteralShape{declaration: grid + "var cells = grid[point]{\n\t{},\n}\n", value: "cells"}
			fixture.goFile(t, callableOwnerCaller, externalElidedLiteralSource(empty))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
			full := externalElidedLiteralShape{declaration: grid + "var cells = grid[point]{\n\t" + element + "\n}\n", value: "cells"}
			source := externalElidedLiteralSource(full)
			fixture.goFile(t, callableOwnerCaller, source)
			commitInventoryFixture(t, fixture.root)
			const reason = "nonempty elided literal layout is unresolved"
			externalEffectsRefusal(t, fixture, reason)
			before, _, found := strings.Cut(source, element)
			if !found {
				t.Fatal("fixture lost its elided element")
			}
			line := strconv.Itoa(strings.Count(before, "\n") + 1)
			_, err := Collect(fixture.root)
			want := ":" + line + ": external registration capability is unknown: " + reason
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("elided refusal is not sited at its element:\n got %v\nwant %s", err, want)
			}
			if !strings.Contains(err.Error(), "callable_owner.go") {
				t.Fatalf("elided refusal does not name its file: %v", err)
			}
		})
	}
}
