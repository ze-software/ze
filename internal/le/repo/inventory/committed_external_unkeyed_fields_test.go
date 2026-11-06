// Design: docs/architecture/plugin/feature-gates.md -- positional fields retain exact capabilities.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalUnkeyedStructFieldCapabilities separates an inert scalar
// projection from the callable held in a different field of the same value.
// Only the later callback projection transfers that capability to a public root.
func TestCommittedExternalUnkeyedStructFieldCapabilities(t *testing.T) {
	for _, shape := range []struct{ name, declarations, datatype, middle string }{
		{"named", "type parcelBox struct { Callback func(); Label string }\n", "parcelBox", ""},
		{"anonymous", "", "struct { Callback func(); Label string }", ""},
		{"alias", "type originalBox struct { Callback func(); Label string }\ntype parcelBox = originalBox\n", "parcelBox", ""},
		{"defined", "type originalBox struct { Callback func(); Label string }\ntype parcelBox originalBox\n", "parcelBox", ""},
		{"grouped", "type parcelBox struct { Callback, Other func(); Label string }\n", "parcelBox", "nil, "},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				// The installer remains in the unselected callback field. A value
				// copy of Label cannot grant authority to that other field.
				scalar := "order.Slice([]string{(" + shape.datatype + "{func() { install(0) }, " + shape.middle + "\"copy\"}).Label}, func(i, j int) bool { return false })"
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(shape.declarations, scalar))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)

				callback := "store.Callback = (" + shape.datatype + "{func() { BODY }, " + shape.middle + "\"copy\"}).Callback; parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(shape.declarations, strings.Replace(callback, "BODY", "", 1)))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)

				// Change only the stored callback's body. Opaque code may call
				// store.Callback through its ordinary static import of neutralparcel.
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(shape.declarations, strings.Replace(callback, "BODY", "install(0)", 1)))
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, "registration")
			})
		}
	}
}
