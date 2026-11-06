// Design: docs/architecture/plugin/feature-gates.md -- exported constants retain named scalar methods.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalStaticConstantMethods includes an exported constant's
// private named type in static access roots. Opaque code can invoke its exported
// method without receiving that value as an argument or naming the private type.
func TestCommittedExternalStaticConstantMethods(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			neutral := "package neutralparcel\nconst Label = \"ordinary committed control\"\n" +
				"type token int\nconst Token token = 0\nvar callback func()\n" +
				"func Store(cb func()) { callback = cb }\nfunc (token) Trigger() { callback() }\n"
			fixture.goFile(t, externalNeutralFile, neutral)
			calls := "store.Store(func() { BODY }); parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", strings.Replace(calls, "BODY", "", 1)))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)

			// The private callback is not a static root itself. Its only
			// externally callable reader is Token.Trigger, through the const.
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", strings.Replace(calls, "BODY", "install(0)", 1)))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "registration")
		})
	}
}
