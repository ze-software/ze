// Design: docs/architecture/plugin/feature-gates.md -- all retained writes contribute formal provenance.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalLateFormalCallbackBindings enforces the conservative
// later-write contract. The dormant function is deliberately not invoked;
// this is not a claim that its write executes during actual initialization.
func TestCommittedExternalLateFormalCallbackBindings(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			declarations := "func stash(cb func()) { store.Callback = cb }\nfunc later() { stash(func() { LATER_BODY }) }\n"
			calls := "stash(func() {})\nparcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(strings.Replace(declarations, "LATER_BODY", "", 1), calls))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)

			// The reached call remains inert. Only a later retained call's
			// argument gains the installer capability written to the same root.
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(strings.Replace(declarations, "LATER_BODY", "install(0)", 1), calls))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "registration")
		})
	}
}

// TestCommittedExternalUnboundFormalCallbackWrite distinguishes an unknown
// formal's retained write from a proven nil value. No ordering or dead-write
// certificate excludes this assignment under the approved later-write model.
func TestCommittedExternalUnboundFormalCallbackWrite(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			calls := "parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", calls))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)

			fixture.goFile(t, callableOwnerCaller, externalEffectsSource("func stash(cb func()) { store.Callback = cb }", calls))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "unknown")
		})
	}
}
