// Design: docs/architecture/plugin/feature-gates.md -- a pointer method on an addressable value is called on its address.
package repoinventory

import (
	"strings"
	"testing"
)

// externalImplicitAddressCell is the vendored huff0 nodeElt shape: a named
// scalar whose methods all take a pointer receiver.
const externalImplicitAddressCell = "type cell uint64\n" +
	"func (c *cell) bits() uint8 { return uint8(*c) }\n"

// TestCommittedExternalImplicitAddressPublishes requires a pointer method
// called on an addressable value to resolve, as Go calls it on the value's
// address. Method: huff0's compress.go calls nbBits() on a nodeElt value;
// here a variable and a parenthesised variable make the same call, and the
// method is inert, so the only possible refusal is an
// unresolved receiver. huff0's own receiver is a range variable, whose
// element still has no retained source equation to ground the receiver
// formal, so that shape is not here.
//
// MUTATION: (internal/le/repo/inventory/methods.go,
// `return binding.Kind == ast.Var`, `return false`): no variable receiver is
// addressable, the selector is projected as a field, and the variable cases
// refuse on root field ownership.
func TestCommittedExternalImplicitAddressPublishes(t *testing.T) {
	for _, shape := range []struct{ name, call string }{
		{"variable", "var c cell; _ = c.bits()"},
		{"parenthesised", "var c cell; _ = (c).bits()"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(externalImplicitAddressCell, shape.call))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)
			})
		}
	}
}

// TestCommittedExternalImplicitAddressKeepsRegistration requires an
// implicitly addressed pointer method to carry what its variable holds, as
// the explicit form does. Method: fill, a pointer method, passes &p.hook to
// binary.Read; it is called on the struct variable box and on &box. An inert
// callback publishes and a registering one refuses.
func TestCommittedExternalImplicitAddressBindsStorage(t *testing.T) {
	for _, call := range []string{
		"box := parcel{hook: func() { BODY }, size: 0}; box.fill(nil)",
		"box := parcel{hook: func() { BODY }, size: 0}; (&box).fill(nil)",
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(call+"/"+kind, func(t *testing.T) {
				fixture := externalAddressFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalAddressSource(externalAddressParcel, strings.Replace(call, "BODY", "", 1)))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)
				fixture.goFile(t, callableOwnerCaller, externalAddressSource(externalAddressParcel, strings.Replace(call, "BODY", "install(0)", 1)))
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, "registration")
			})
		}
	}
}

// TestCommittedExternalImplicitAddressExposesMetadata requires the receiver of
// an implicitly addressed pointer method to be the variable's address, so
// code the method hands it to can write that variable. Method: the neutral
// package's Box.Label is a plugin's registration description; Expose, a
// pointer method of Box's type, passes its receiver to sort.Slice. Touch
// calls it on Box, once as Box.Expose() and once as (&Box).Expose(): both
// refuse as metadata reaching external code. Called on another variable of
// the same type, it publishes.
//
// MUTATION: (internal/le/repo/inventory/externalexpressions.go,
// `receiver = e.addressValue(operand, source, possible, depth)`,
// `receiver = e.value(operand, possible, depth+1)`): the implicit receiver
// is Box's value, which names no location, so Box.Expose() publishes while
// the metadata is writable.
func TestCommittedExternalImplicitAddressExposesMetadata(t *testing.T) {
	for _, shape := range []struct {
		name, call string
		metadata   bool
	}{
		{"implicit", "Box.Expose()", true},
		{"explicit", "(&Box).Expose()", true},
		{"other variable", "Other.Expose()", false},
	} {
		t.Run(shape.name, func(t *testing.T) {
			fixture := externalEffectsFixture(t, "plugin")
			neutral := "package neutralparcel\nimport (\"sort\"; _ \"os\"; _ \"io\"; _ \"time\")\n" +
				"type Holder struct { Label string }\n" +
				"func (h *Holder) Expose() { sort.Slice([]*Holder{h}, func(i, j int) bool { return false }) }\n" +
				"var Box = Holder{Label: \"ordinary committed control\"}\nvar Other = Holder{Label: \"unrelated\"}\n" +
				"func Touch() { " + shape.call + " }\n"
			fixture.goFile(t, externalNeutralFile, neutral)
			registration := asmOrdinaryDir + "/register.go"
			source := strings.Replace(fixture.goSources[registration], "import (", "import (\n store \"github.com/ze-software/ze/"+externalNeutralDir+"\"", 1)
			source = strings.Replace(source, "Description: \"ordinary committed control\"", "Description: store.Box.Label", 1)
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

// TestCommittedPointerMethodImplicitAddressExecutesOwner requires a pointer
// method called on a package variable of the value type to execute, as Go
// calls it on &packet. Method: Deliver installs the owner; the implicit call
// and its explicit (&packet).Deliver() form both publish with that owner.
//
// MUTATION: (internal/le/repo/inventory/methods.go,
// `if !e.addressable(receiver) {`, `if !e.addressable(receiver) || true {`):
// no value receiver is addressed, the implicit call's callee is unresolved,
// and its report refuses as callable receiver has unresolved execution
// ownership.
func TestCommittedPointerMethodImplicitAddressExecutesOwner(t *testing.T) {
	for _, call := range []string{"packet.Deliver()", "(&packet).Deliver()"} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(call+"/"+kind, func(t *testing.T) {
				scenario := callableOwnerCase{declarations: "type parcel struct{}\nfunc (*parcel) Deliver() { helper.Install(0) }\nvar packet parcel\n"}
				fixture := pointerOwnerFixture(t, kind, scenario)
				fixture.goFile(t, callableOwnerCaller, callableOwnerSource(scenario, call))
				commitInventoryFixture(t, fixture.root)
				if _, _, err := callableOwnerOutcome(t, fixture, kind, false); err != nil {
					t.Fatalf("implicitly addressed pointer method lost its owner: %v", err)
				}
			})
		}
	}
}

// TestCommittedExternalImplicitAddressKeepsUnaddressableRefusal keeps a
// pointer method on a value Go cannot address unresolved. Method: a retained
// call's result and a composite literal are values with no address, so
// cell's pointer method cannot be called on them and the selector refuses
// on root field ownership. The analyzer reads syntax, so it must refuse this
// invalid Go rather than invent an address.
//
// MUTATION: (internal/le/repo/inventory/methods.go,
// `if !e.addressable(receiver) {`, `if false {`): every value receiver is
// addressed, and both cases publish.
func TestCommittedExternalImplicitAddressKeepsUnaddressableRefusal(t *testing.T) {
	for _, call := range []string{"_ = fresh().bits()", "_ = cell(1).bits()"} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(call+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(externalImplicitAddressCell+"func fresh() cell { return 1 }\n", call))
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, "ownership")
			})
		}
	}
}
