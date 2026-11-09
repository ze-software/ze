// Design: docs/architecture/plugin/feature-gates.md -- a method promoted through embedded fields resolves on the embedded field.
package repoinventory

import (
	"strings"
	"testing"
)

// promotedNeutralTypes is the vendored protojson shape in the neutral
// package: Decoder's pointer method Read returns two values, the first a
// Token whose value method is then called.
const promotedNeutralTypes = "type Token struct { Kind uint8 }\n" +
	"type Decoder struct { Size int }\n" +
	"func (d *Decoder) Read() (Token, error) { _ = d; return Token{Kind: 0}, nil }\n" +
	"func (t Token) Parsed() string { _ = t; return \"\" }\n"

// promotedDecoder embeds a pointer to the neutral package's Decoder, as
// protojson's decoder embeds *json.Decoder.
const promotedDecoder = "type decoder struct { *store.Decoder; size int }\n"

// TestCommittedExternalPromotedMethodPublishes requires a method promoted
// through an embedded pointer to another package's type to resolve, and the
// first result of its two to type the variable it declares. Method: the
// protojson unmarshalTimestamp shape, tok, err := d.Read() then
// tok.Parsed(), runs once in an initializer and once inside a value method
// of decoder, as protojson calls it; every method is inert, so the only
// possible refusal is an unresolved receiver. Before promotion and tuple
// typing, the actual root refused here as returned root field ownership is
// unresolved.
//
// MUTATION: (internal/le/repo/inventory/methodpromotion.go,
// `if pointerMethod == holdsPointer {`, `if pointerMethod != holdsPointer {`):
// d.Read no longer resolves, is projected as a field, and both shapes refuse
// on root field ownership.
// MUTATION: (internal/le/repo/inventory/methods.go,
// `return e.tupleType(source, declaration.Rhs, len(declaration.Lhs), index, depth+1)`,
// `return sourceExpr{}, false`): tok is untyped, tok.Parsed is projected as a
// field, and both shapes refuse as returned root field ownership is
// unresolved, the actual root's refusal.
func TestCommittedExternalPromotedMethodPublishes(t *testing.T) {
	for _, shape := range []struct{ name, declarations, call string }{
		{"initializer", promotedDecoder,
			"d := decoder{Decoder: &store.Decoder{Size: 0}, size: 0}; tok, err := d.Read(); _ = err; _ = tok.Parsed()"},
		{"value method", promotedDecoder +
			"func (d decoder) timestamp() string { tok, err := d.Read(); if err != nil { return \"\" }; return tok.Parsed() }\n",
			"d := decoder{Decoder: &store.Decoder{Size: 0}, size: 0}; _ = d.timestamp()"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				fixture.goFile(t, externalNeutralFile, externalNeutralSource+promotedNeutralTypes)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(shape.declarations, shape.call))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)
			})
		}
	}
}

// TestCommittedPointerMethodPromotedExecutesOwner requires a promoted method
// to be executed by the registration walk, so the owner it installs is in the
// report. Method: Deliver, a pointer method of parcel, installs the owner;
// it is called through an embedded *parcel on a nil *envelope (the scenario
// TestCommittedPointerMethodUnsupportedOwnersRefuse pinned as unsupported),
// through an embedded parcel on an addressable variable, which Go calls on
// &packet.parcel, and through two embedding levels.
//
// MUTATION: (internal/le/repo/inventory/methodpromotion.go,
// `if members == 1 {`, `if members == 2 {`): the level holding Deliver never
// decides, the search runs out of embedded fields, and every shape refuses as
// callable receiver has unresolved execution ownership.
// MUTATION: (internal/le/repo/inventory/methodpromotion.go,
// `if !addressable {`, `if addressable {`): the addressable embedded value
// refuses as callable receiver has unresolved execution ownership, and the
// unaddressable one in TestCommittedExternalPromotedMethodUnresolvedRefuses
// resolves to an address of a call result, which refuses on address
// ownership instead of root field ownership.
func TestCommittedPointerMethodPromotedExecutesOwner(t *testing.T) {
	const parcel = "type parcel struct{}\nfunc (*parcel) Deliver() { helper.Install(0) }\n"
	for _, shape := range []struct{ name, declarations string }{
		{"embedded pointer", "type envelope struct { *parcel }\nvar packet *envelope\n"},
		{"embedded value", "type envelope struct { parcel }\nvar packet envelope\n"},
		{"two levels", "type inner struct { *parcel }\ntype envelope struct { inner }\nvar packet envelope\n"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				scenario := callableOwnerCase{declarations: parcel + shape.declarations}
				fixture := pointerOwnerFixture(t, kind, scenario)
				fixture.goFile(t, callableOwnerCaller, callableOwnerSource(scenario, "packet.Deliver()"))
				commitInventoryFixture(t, fixture.root)
				if _, _, err := callableOwnerOutcome(t, fixture, kind, false); err != nil {
					t.Fatalf("promoted method lost its owner: %v", err)
				}
			})
		}
	}
}

// TestCommittedExternalPromotedMethodBindsEmbeddedField requires the
// receiver of a promoted method to be the embedded field, so the method body
// reads that field's values. Method: Deliver, a pointer method of parcel,
// hands p.hook to sort.Slice; it is called on an envelope embedding *parcel
// and on one embedding parcel by value. An inert hook publishes and a
// registering one refuses as registration reaching external code, so the
// body was walked with the embedded parcel bound to p.
//
// MUTATION: (internal/le/repo/inventory/externalexpressions.go,
// `operand := call.receiver`,
// `operand := sourceExpr{value: value.X, file: source.file, pkg: source.pkg}`):
// p is bound to the envelope, which holds no hook field, so p.hook refuses on
// root field value ownership and the inert case no longer publishes.
func TestCommittedExternalPromotedMethodBindsEmbeddedField(t *testing.T) {
	const parcel = "type parcel struct { hook func() }\n" +
		"func (p *parcel) Deliver() { order.Slice([]any{p.hook}, func(i, j int) bool { return false }) }\n"
	for _, shape := range []struct{ name, declarations, call string }{
		{"embedded pointer", "type envelope struct { *parcel; size int }\n",
			"box := envelope{parcel: &parcel{hook: func() { BODY }}, size: 0}; box.Deliver()"},
		{"embedded value", "type envelope struct { parcel; size int }\n",
			"box := envelope{parcel: parcel{hook: func() { BODY }}, size: 0}; box.Deliver()"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(parcel+shape.declarations, strings.Replace(shape.call, "BODY", "", 1)))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(parcel+shape.declarations, strings.Replace(shape.call, "BODY", "install(0)", 1)))
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, "registration")
			})
		}
	}
}

// TestCommittedExternalPromotedMethodExposesMetadata requires a promoted
// pointer method's receiver to reach the embedded field's storage, so code
// the method hands it to can write that storage. Method: the neutral
// package's Holder.Expose, a pointer method, passes its receiver to
// sort.Slice. Wrapped embeds a Holder by value whose Label is a plugin's
// registration description, and Go calls Expose on &Wrapped.Holder, so it
// refuses as metadata reaching external code; Spare, another Wrapper,
// publishes. No embedded-pointer row is here: an embedded *Holder needs an
// &Box in the source, and expression.callableImmutable already refuses Box's
// metadata for that escaping address whatever is called, so the row would
// have no control. TestCommittedExternalPromotedMethodBindsEmbeddedField
// proves the embedded-pointer receiver.
//
// MUTATION: (internal/le/repo/inventory/externalexpressions.go,
// `receiver = e.addressValue(operand, source, possible, depth)`,
// `receiver = child(operand.value)`): the addressed receiver is
// Wrapped.Holder's value, which names no location, so Wrapped.Expose()
// publishes while the metadata is writable.
func TestCommittedExternalPromotedMethodExposesMetadata(t *testing.T) {
	for _, shape := range []struct {
		name, description, call string
		metadata                bool
	}{
		{"embedded value", "store.Wrapped.Holder.Label", "Wrapped.Expose()", true},
		{"other storage", "store.Wrapped.Holder.Label", "Spare.Expose()", false},
	} {
		t.Run(shape.name, func(t *testing.T) {
			fixture := externalEffectsFixture(t, "plugin")
			neutral := "package neutralparcel\nimport (\"sort\"; _ \"os\"; _ \"io\"; _ \"time\")\n" +
				"type Holder struct { Label string }\n" +
				"func (h *Holder) Expose() { sort.Slice([]*Holder{h}, func(i, j int) bool { return false }) }\n" +
				"type Wrapper struct { Holder }\n" +
				"var Wrapped = Wrapper{Holder: Holder{Label: \"ordinary committed control\"}}\n" +
				"var Spare = Wrapper{Holder: Holder{Label: \"unrelated\"}}\n" +
				"func Touch() { " + shape.call + " }\n"
			fixture.goFile(t, externalNeutralFile, neutral)
			registration := asmOrdinaryDir + "/register.go"
			source := strings.Replace(fixture.goSources[registration], "import (", "import (\n store \"github.com/ze-software/ze/"+externalNeutralDir+"\"", 1)
			source = strings.Replace(source, "Description: \"ordinary committed control\"", "Description: "+shape.description, 1)
			fixture.goFile(t, registration, source)
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "store.Touch()"))
			commitInventoryFixture(t, fixture.root)
			if shape.metadata {
				externalEffectsRefusal(t, fixture, "metadata")
				return
			}
			externalEffectsPositive(t, fixture)
		})
	}
}

// TestCommittedExternalPromotedMethodUnresolvedRefuses keeps refusing a
// promoted method the shallowest-depth rule does not settle to one retained
// method Go can call. Method: each shape calls Present in an initializer the
// external effects walk reads; the selector then falls back to a field
// projection, which refuses on root field ownership. Some shapes are not
// valid Go: inventory reads syntax and must refuse them rather than choose.
//
// MUTATION: (internal/le/repo/inventory/methodpromotion.go,
// `members += len(methods)`, `members = len(methods)`): the level counts only
// its last embedded field, and the ambiguous case publishes right's Present.
// MUTATION: (internal/le/repo/inventory/methodpromotion.go,
// `if declaresField(embeddedDefinition, selector.Sel.Name) {`, `if false {`):
// rival's Present field no longer decides its level, and the shadowed case
// publishes inner's method from one level deeper.
// MUTATION: (internal/le/repo/inventory/methodpromotion.go,
// "if !retained {\n\t\t\t\t\treturn methodCall{}, false",
// "if !retained {\n\t\t\t\t\tcontinue"): the embedded interface is skipped,
// and the interface case publishes inner's Present from one level deeper.
// MUTATION: (internal/le/repo/inventory/methodpromotion.go,
// `const methodPromotionDepth = 4`, `const methodPromotionDepth = 5`): the
// method five levels down resolves and the depth case publishes.
// MUTATION: (internal/le/repo/inventory/methodpromotion.go,
// "// A value method on an embedded *E stays unresolved.\n\tif holdsPointer {",
// "// A value method on an embedded *E stays unresolved.\n\tif false {"): the
// value method on the embedded pointer is bound to the field's address, and
// that case publishes.
// The unaddressable case is broken by the `if !addressable {` note of
// TestCommittedPointerMethodPromotedExecutesOwner.
func TestCommittedExternalPromotedMethodUnresolvedRefuses(t *testing.T) {
	const presence = "type presence struct { size int }\n"
	for _, shape := range []struct{ name, declarations, call string }{
		{"ambiguous",
			"type left struct { size int }\nfunc (l left) Present() bool { return l.size == 0 }\n" +
				"type right struct { size int }\nfunc (r right) Present() bool { return r.size == 0 }\n" +
				"type both struct { left; right }\n",
			"b := both{left: left{size: 0}, right: right{size: 0}}; _ = b.Present()"},
		{"field shadows method",
			"type inner struct { size int }\nfunc (i inner) Present() bool { return i.size == 0 }\n" +
				"type middle struct { inner }\ntype rival struct { Present bool }\ntype outer struct { middle; rival }\n",
			"o := outer{middle: middle{inner: inner{size: 0}}, rival: rival{Present: false}}; _ = o.Present()"},
		{"embedded interface",
			"type presenter interface { Present() bool }\n" +
				"type inner struct { size int }\nfunc (i inner) Present() bool { return i.size == 0 }\n" +
				"type middle struct { inner }\ntype outer struct { presenter; middle }\n",
			"o := outer{presenter: nil, middle: middle{inner: inner{size: 0}}}; _ = o.Present()"},
		{"beyond depth bound",
			"type l5 struct { size int }\nfunc (v l5) Present() bool { return v.size == 0 }\n" +
				"type l4 struct { l5 }\ntype l3 struct { l4 }\ntype l2 struct { l3 }\ntype l1 struct { l2 }\ntype l0 struct { l1 }\n",
			"v := l0{l1: l1{l2: l2{l3: l3{l4: l4{l5: l5{size: 0}}}}}}; _ = v.Present()"},
		{"value method on embedded pointer",
			presence + "func (q presence) Present() bool { return q.size == 0 }\ntype envelope struct { *presence }\n",
			"e := envelope{presence: &presence{size: 0}}; _ = e.Present()"},
		{"unaddressable embedded value",
			presence + "func (q *presence) Present() bool { return q.size == 0 }\ntype envelope struct { presence }\n" +
				"func fresh() envelope { return envelope{presence: presence{size: 0}} }\n",
			"_ = fresh().Present()"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(shape.declarations, shape.call))
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, "root field ownership")
			})
		}
	}
}

// TestCommittedConcreteMethodPromotedValueExecutesOwner requires a value
// method promoted through an embedded value to be executed by the
// registration walk. Method: the input of the promoted case of
// TestCommittedUnsupportedReceiverOwnersRefuse, packet = envelope{} with
// envelope embedding parcel and Deliver a value method of parcel that
// installs the owner; the report publishes with that owner.
//
// MUTATION: (internal/le/repo/inventory/methodpromotion.go,
// `if members == 1 {`, `if members == 2 {`): Deliver no longer resolves and
// the report refuses as callable receiver has unresolved execution ownership.
func TestCommittedConcreteMethodPromotedValueExecutesOwner(t *testing.T) {
	scenario := callableOwnerCase{declarations: "type parcel struct{}\nfunc (parcel) Deliver() { helper.Install(0) }\ntype envelope struct { parcel }\nvar packet = envelope{}\n", call: "packet.Deliver()"}
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := callableOwnerFixture(t, kind, scenario)
			fixture.goFile(t, callableOwnerCaller, callableOwnerSource(scenario, scenario.call))
			commitInventoryFixture(t, fixture.root)
			if _, _, err := callableOwnerOutcome(t, fixture, kind, false); err != nil {
				t.Fatalf("promoted value method lost its owner: %v", err)
			}
		})
	}
}
