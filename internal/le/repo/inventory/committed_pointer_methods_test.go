// Design: docs/architecture/plugin/feature-gates.md -- explicit retained pointer receivers keep concrete method ownership.
package repoinventory

import (
	"strings"
	"testing"
)

const pointerOwnerTypes = `type Parcel struct { callback func(uint32) }
func NewParcel() *Parcel { return &Parcel{} }
func (packet *Parcel) Deliver() { packet.callback = Install }
`

// pointerOwnerFixture retains the existing ordinary/direct-installer controls.
// Its initial call never invokes a pointer method, so an old missing receiver
// implementation cannot mask the actual negative-owner discriminators.
func pointerOwnerFixture(t *testing.T, kind string, scenario callableOwnerCase) *asmInventoryFixture {
	t.Helper()
	fixture := callableOwnerFixture(t, kind, scenario)
	fixture.goFile(t, callableOwnerFile, callableOwnerHelper(kind)+pointerOwnerTypes)
	fixture.goFile(t, callableOwnerCaller, callableOwnerSource(scenario, "_ = ownerProbe()"))
	commitInventoryFixture(t, fixture.root)
	asmHeaderPositive(t, fixture, callableOwnerPaths())
	return fixture
}

// TestCommittedPointerMethodsKeepDefiningIdentityAndEffects distinguishes the
// imported pointer result from a same-named local pointer type. Storing a callback
// is inert; invoking an installer inside the actual retained method is not.
func TestCommittedPointerMethodsKeepDefiningIdentityAndEffects(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			scenario := callableOwnerCase{
				declarations: "type Parcel struct{}\nfunc (*Parcel) Deliver() { helper.Install(0) }\n",
			}
			fixture := pointerOwnerFixture(t, kind, scenario)
			scenario.declarations += "var origin = helper.NewParcel()\nvar packet = origin\n"
			fixture.goFile(t, callableOwnerCaller, callableOwnerSource(scenario, "packet.Deliver()"))
			commitInventoryFixture(t, fixture.root)
			snapshot := asmHeaderPositive(t, fixture, callableOwnerPaths())
			expected := asmHeaderExpected(fixture)
			revision := snapshot.Revision()

			changed := strings.Replace(pointerOwnerTypes, "packet.callback = Install", "Install(0)", 1)
			fixture.goFile(t, callableOwnerFile, callableOwnerHelper(kind)+changed)
			asmHeaderPositive(t, expected, callableOwnerPaths())
			commitInventoryFixture(t, fixture.root)
			fresh, _, _ := callableOwnerOutcome(t, fixture, kind, false)
			if fresh.Revision() == revision {
				t.Fatal("pointer method fixture did not advance HEAD")
			}
			if snapshot.Revision() != revision {
				t.Fatal("retained pointer snapshot revision moved")
			}
			plugins, err := snapshot.Plugins()
			if err != nil {
				t.Fatalf("later pointer method changed retained membership: %v", err)
			}
			expected.checkPlugins(t, plugins)
			inv, err := snapshot.collect()
			if err != nil {
				t.Fatalf("later pointer method changed retained report: %v", err)
			}
			expected.checkReport(t, inv)
			asmHeaderSelectedPaths(t, snapshot, callableOwnerPaths())
		})
	}
}

// TestCommittedPointerMethodIterationRetainsConditionalRefusal permits an inert
// pointer method under range but never treats a registration there as certain.
func TestCommittedPointerMethodIterationRetainsConditionalRefusal(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			scenario := callableOwnerCase{}
			fixture := pointerOwnerFixture(t, kind, scenario)
			scenario.declarations = "var parcels = []*helper.Parcel{helper.NewParcel()}\n"
			call := "for _, packet := range parcels { packet.Deliver() }"
			fixture.goFile(t, callableOwnerCaller, callableOwnerSource(scenario, call))
			commitInventoryFixture(t, fixture.root)
			asmHeaderPositive(t, fixture, callableOwnerPaths())
			changed := strings.Replace(pointerOwnerTypes, "packet.callback = Install", "Install(0)", 1)
			fixture.goFile(t, callableOwnerFile, callableOwnerHelper(kind)+changed)
			commitInventoryFixture(t, fixture.root)
			inv, reportErr := Collect(fixture.root)
			plugins, pluginErr := Plugins(fixture.root)
			if reportErr == nil {
				t.Fatalf("conditional pointer report succeeded: %+v", inv.Plugins)
			}
			if pluginErr == nil {
				t.Fatalf("conditional pointer Plugins succeeded: %+v", plugins)
			}
			if inv.Plugins != nil {
				t.Fatalf("conditional pointer report leaked partial membership: %+v", inv.Plugins)
			}
			if plugins != nil {
				t.Fatalf("conditional pointer Plugins leaked partial membership: %+v", plugins)
			}
			for _, err := range []error{reportErr, pluginErr} {
				if !strings.Contains(err.Error(), "registration depends on an unresolved condition or iteration") {
					t.Fatalf("conditional pointer effect masked by unrelated refusal: %v", err)
				}
			}
		})
	}
}

// TestCommittedPointerMethodUnsupportedOwnersRefuse does not infer
// value-method fallback, dereference expressions, aliases, generic sets or
// interface values. Implicit addressing of a variable, a reassigned receiver
// binding and a promoted method are resolved, and
// TestCommittedPointerMethodImplicitAddressExecutesOwner and
// TestCommittedPointerMethodPromotedExecutesOwner require their owner.
func TestCommittedPointerMethodUnsupportedOwnersRefuse(t *testing.T) {
	for _, scenario := range []callableOwnerCase{
		{name: "value-method-fallback", declarations: "type parcel struct{}\nfunc (parcel) Deliver() { helper.Install(0) }\nfunc makeParcel() *parcel { return &parcel{} }\nvar packet = makeParcel()\n", call: "packet.Deliver()"},
		{name: "double-pointer", declarations: "type parcel struct{}\nfunc (*parcel) Deliver() { helper.Install(0) }\nvar first = &parcel{}\nvar packet = &first\n", call: "(*packet).Deliver()"},
		{name: "type-alias", declarations: "type parcel struct{}\nfunc (*parcel) Deliver() { helper.Install(0) }\ntype alias = parcel\nvar packet *alias\n", call: "packet.Deliver()"},
		{name: "generic", declarations: "type parcel[T any] struct{}\nfunc (*parcel[T]) Deliver() { helper.Install(0) }\nvar packet *parcel[int]\n", call: "packet.Deliver()"},
		{name: "interface", declarations: "type parcel struct{}\nfunc (*parcel) Deliver() { helper.Install(0) }\ntype delivery interface { Deliver() }\nfunc makeDelivery() delivery { return &parcel{} }\nvar packet = makeDelivery()\n", call: "packet.Deliver()"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(scenario.name+"/"+kind, func(t *testing.T) {
				fixture := pointerOwnerFixture(t, kind, scenario)
				fixture.goFile(t, callableOwnerCaller, callableOwnerSource(scenario, scenario.call))
				commitInventoryFixture(t, fixture.root)
				inv, reportErr := Collect(fixture.root)
				plugins, pluginErr := Plugins(fixture.root)
				if reportErr == nil {
					t.Fatalf("unsupported pointer report succeeded: %+v", inv.Plugins)
				}
				if pluginErr == nil {
					t.Fatalf("unsupported pointer Plugins succeeded: %+v", plugins)
				}
				if inv.Plugins != nil {
					t.Fatalf("unsupported pointer report leaked partial membership: %+v", inv.Plugins)
				}
				if plugins != nil {
					t.Fatalf("unsupported pointer Plugins leaked partial membership: %+v", plugins)
				}
				callableOwnerCause(t, reportErr)
				callableOwnerCause(t, pluginErr)
			})
		}
	}
}

// TestCommittedPointerMethodsDoNotBorrowRegistrationAPIs preserves lexical API
// classification when an explicit pointer receiver shadows a registration import.
func TestCommittedPointerMethodsDoNotBorrowRegistrationAPIs(t *testing.T) {
	for _, api := range []struct{ method, directory string }{
		{"Register", registrationPackage},
		{"RegisterModule", "internal/component/config/yang"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(api.method+"/"+kind, func(t *testing.T) {
				scenario := callableOwnerCase{
					declarations: "type parcel struct{}\nfunc (*parcel) " + api.method + "() { helper.Install(0) }\nfunc makeParcel() *parcel { return &parcel{} }\n",
					extraImport:  "shadow \"github.com/ze-software/ze/" + api.directory + "\"",
					prefix:       "_ = shadow." + api.method + "\n",
					call:         "shadow := makeParcel(); shadow." + api.method + "()",
				}
				fixture := pointerOwnerFixture(t, kind, scenario)
				fixture.goFile(t, callableOwnerCaller, callableOwnerSource(scenario, scenario.call))
				commitInventoryFixture(t, fixture.root)
				callableOwnerOutcome(t, fixture, kind, false)
			})
		}
	}
}
