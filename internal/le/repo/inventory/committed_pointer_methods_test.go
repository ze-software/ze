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
func pointerOwnerFixture(t *testing.T, kind string, scenario callableOwnerCase) *ordinaryFixture {
	t.Helper()
	fixture := callableOwnerFixture(t, kind, scenario)
	fixture.goFile(t, callableOwnerFile, callableOwnerHelper(kind)+pointerOwnerTypes)
	fixture.goFile(t, callableOwnerCaller, callableOwnerSource(scenario, "_ = ownerProbe()"))
	commitInventoryFixture(t, fixture.root)
	checkOrdinaryPositive(t, fixture, callableOwnerPaths())
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
			snapshot := checkOrdinaryPositive(t, fixture, callableOwnerPaths())
			expected := frozenFixture(fixture)
			revision := snapshot.Revision()

			changed := strings.Replace(pointerOwnerTypes, "packet.callback = Install", "Install(0)", 1)
			fixture.goFile(t, callableOwnerFile, callableOwnerHelper(kind)+changed)
			checkOrdinaryPositive(t, expected, callableOwnerPaths())
			commitInventoryFixture(t, fixture.root)
			fresh, _, _ := callableOwnerOutcome(t, fixture, kind, false)
			if fresh.Revision() == revision {
				t.Fatal("pointer method fixture did not advance HEAD")
			}
			if snapshot.Revision() != revision {
				t.Fatal("retained pointer snapshot revision moved")
			}
			plugins, err := snapshot.DeclaredPlugins()
			if err != nil {
				t.Fatalf("later pointer method changed retained membership: %v", err)
			}
			expected.checkPlugins(t, plugins)
			inv, err := snapshot.collect()
			if err != nil {
				t.Fatalf("later pointer method changed retained report: %v", err)
			}
			expected.checkReport(t, inv)
			checkSelectedPaths(t, snapshot, callableOwnerPaths())
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
			checkOrdinaryPositive(t, fixture, callableOwnerPaths())
			changed := strings.Replace(pointerOwnerTypes, "packet.callback = Install", "Install(0)", 1)
			fixture.goFile(t, callableOwnerFile, callableOwnerHelper(kind)+changed)
			commitInventoryFixture(t, fixture.root)
			inv, reportErr := Collect(fixture.root)
			plugins, pluginErr := DeclaredPlugins(fixture.root)
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

// TestCommittedPointerMethodReassignedBindingExecutesOwner requires a pointer
// method called on a reassigned variable to execute: the assignment keeps the
// variable's static type, so the method set is exact. Method: packet is
// declared from makeParcel() and assigned again before packet.Deliver(), and
// Deliver installs the owner; the report publishes with that owner.
//
// MUTATION: (internal/le/repo/inventory/methods.go,
// `// the type its declaration gives it, and no write check applies here.`,
// `if err := e.immutable(source); err != nil { return sourceExpr{}, false }`):
// the written packet has no type, Deliver is unresolved, and the report
// refuses as callable receiver has unresolved execution ownership.
func TestCommittedPointerMethodReassignedBindingExecutesOwner(t *testing.T) {
	scenario := callableOwnerCase{declarations: "type parcel struct{}\nfunc (*parcel) Deliver() { helper.Install(0) }\nfunc makeParcel() *parcel { return &parcel{} }\nvar packet = makeParcel()\n"}
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := pointerOwnerFixture(t, kind, scenario)
			fixture.goFile(t, callableOwnerCaller, callableOwnerSource(scenario, "packet = makeParcel(); packet.Deliver()"))
			commitInventoryFixture(t, fixture.root)
			if _, _, err := callableOwnerOutcome(t, fixture, kind, false); err != nil {
				t.Fatalf("reassigned pointer receiver lost its owner: %v", err)
			}
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

// TestCommittedPointerMethodPromotedExecutesOwner requires a promoted method
// to be executed by the registration walk, so the owner it installs is in the
// report. Method: Deliver, a pointer method of parcel, installs the owner;
// it is called through an embedded *parcel on a nil *envelope, through an
// embedded parcel on an addressable variable, which Go calls on
// &packet.parcel, and through two embedding levels.
//
// MUTATION: (internal/le/repo/inventory/methodpromotion.go,
// `if members == 1 {`, `if members == 2 {`): the level holding Deliver never
// decides, the search runs out of embedded fields, and every shape refuses as
// callable receiver has unresolved execution ownership.
// MUTATION: (internal/le/repo/inventory/methodpromotion.go,
// `if !addressable {`, `if addressable {`): the addressable embedded value
// refuses as callable receiver has unresolved execution ownership.
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
