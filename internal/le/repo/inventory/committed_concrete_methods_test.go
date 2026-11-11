// Design: docs/architecture/plugin/feature-gates.md -- concrete receiver identity retains actual method effects.
package repoinventory

import (
	"reflect"
	"strings"
	"testing"
)

// TestCommittedConcreteReceiverMethodsRetainExecution uses arbitrary names and
// a real method body. Constructor results and range elements must retain their
// declared concrete type without classifying a method by its spelling.
func TestCommittedConcreteReceiverMethodsRetainExecution(t *testing.T) {
	for _, binding := range []string{"constructor-result", "range-element"} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(binding+"/"+kind, func(t *testing.T) {
				declarations := "type parcel struct{}\nfunc makeParcel() parcel { return parcel{} }\nfunc (parcel) Deliver() {}\n"
				call := "packet.Deliver()"
				if binding == "constructor-result" {
					declarations += "var packet = makeParcel()\n"
				} else {
					declarations += "var parcels = []parcel{{}}\n"
					call = "for _, packet := range parcels { packet.Deliver() }"
				}
				scenario := callableOwnerCase{declarations: declarations, call: call}
				fixture := callableOwnerFixture(t, kind, scenario)
				fixture.goFile(t, callableOwnerCaller, callableOwnerSource(scenario, call))
				commitInventoryFixture(t, fixture.root)
				inv, err := Collect(fixture.root)
				if err != nil {
					t.Fatalf("concrete inert method control refused: %v", err)
				}
				fixture.checkReport(t, inv)
				plugins, err := DeclaredPlugins(fixture.root)
				if err != nil {
					t.Fatalf("public Plugins refused concrete inert method: %v", err)
				}
				fixture.checkPlugins(t, plugins)

				scenario.declarations = strings.Replace(declarations, "func (parcel) Deliver() {}", "func (parcel) Deliver() { helper.Install(0) }", 1)
				fixture.goFile(t, callableOwnerCaller, callableOwnerSource(scenario, call))
				commitInventoryFixture(t, fixture.root)
				inv, reportErr := Collect(fixture.root)
				plugins, pluginErr := DeclaredPlugins(fixture.root)
				if binding == "range-element" {
					if reportErr == nil {
						t.Fatalf("conditional method report succeeded: %+v", inv.Plugins)
					}
					if pluginErr == nil {
						t.Fatalf("conditional method Plugins succeeded: %+v", plugins)
					}
					if inv.Plugins != nil {
						t.Fatalf("conditional report leaked partial membership: %+v", inv.Plugins)
					}
					if plugins != nil {
						t.Fatalf("conditional Plugins leaked partial membership: %+v", plugins)
					}
					for _, err := range []error{reportErr, pluginErr} {
						if !strings.Contains(err.Error(), "registration depends on an unresolved condition or iteration") {
							t.Fatalf("conditional method effect masked by unrelated refusal: %v", err)
						}
					}
					return
				}
				if reportErr != nil {
					t.Fatalf("concrete unconditional method report refused: %v", reportErr)
				}
				if pluginErr != nil {
					t.Fatalf("concrete unconditional method Plugins refused: %v", pluginErr)
				}
				callableOwnerInstalled(t, fixture, kind, inv)
				if !reflect.DeepEqual(plugins, inv.Plugins) {
					t.Fatalf("concrete method consumers disagree: %+v / %+v", plugins, inv.Plugins)
				}
			})
		}
	}
}

// TestCommittedConcreteMethodUsesDefiningPackage distinguishes an imported
// concrete result from a same-named local type with different method effects.
func TestCommittedConcreteMethodUsesDefiningPackage(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			scenario := callableOwnerCase{declarations: "type Parcel struct{}\nfunc (Parcel) Deliver() { helper.Install(0) }\n"}
			fixture := callableOwnerFixture(t, kind, scenario)
			helper := callableOwnerHelper(kind) + "type Parcel struct{}\nfunc NewParcel() Parcel { return Parcel{} }\nfunc (Parcel) Deliver() {}\n"
			fixture.goFile(t, callableOwnerFile, helper)
			scenario.declarations += "var packet = helper.NewParcel()\n"
			fixture.goFile(t, callableOwnerCaller, callableOwnerSource(scenario, "packet.Deliver()"))
			commitInventoryFixture(t, fixture.root)
			inv, err := Collect(fixture.root)
			if err != nil {
				t.Fatalf("imported concrete inert method refused: %v", err)
			}
			fixture.checkReport(t, inv)
			plugins, err := DeclaredPlugins(fixture.root)
			if err != nil {
				t.Fatal(err)
			}
			fixture.checkPlugins(t, plugins)

			snapshot, err := ReadCommitted(t.Context(), fixture.root)
			if err != nil {
				t.Fatal(err)
			}
			revision := snapshot.Revision()
			expected := frozenFixture(fixture)
			fixture.goFile(t, callableOwnerFile, strings.Replace(helper, "func (Parcel) Deliver() {}", "func (Parcel) Deliver() { Install(0) }", 1))
			dirty, err := Collect(fixture.root)
			if err != nil {
				t.Fatalf("dirty method body changed committed report: %v", err)
			}
			expected.checkReport(t, dirty)
			commitInventoryFixture(t, fixture.root)
			fresh, _, _ := callableOwnerOutcome(t, fixture, kind, false)
			if fresh.Revision() == revision {
				t.Fatal("method-body fixture did not advance HEAD")
			}
			if snapshot.Revision() != revision {
				t.Fatal("captured method snapshot revision moved")
			}
			retained, err := snapshot.DeclaredPlugins()
			if err != nil {
				t.Fatalf("later method body changed retained Plugins: %v", err)
			}
			expected.checkPlugins(t, retained)
			oldReport, err := snapshot.collect()
			if err != nil {
				t.Fatalf("later method body changed retained full report: %v", err)
			}
			expected.checkReport(t, oldReport)
			checkSelectedPaths(t, snapshot, callableOwnerPaths())
		})
	}
}

// TestCommittedConcreteMethodsDoNotBorrowRegistrationAPIs keeps a local receiver
// that shadows an import from being dispatched as that import's terminal API.
func TestCommittedConcreteMethodsDoNotBorrowRegistrationAPIs(t *testing.T) {
	for _, api := range []struct{ method, directory string }{
		{"Register", registrationPackage},
		{"RegisterModule", "internal/component/config/yang"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(api.method+"/"+kind, func(t *testing.T) {
				scenario := callableOwnerCase{
					declarations: "type parcel struct{}\nfunc (parcel) " + api.method + "() { helper.Install(0) }\n",
					extraImport:  "shadow \"github.com/ze-software/ze/" + api.directory + "\"",
					prefix:       "_ = shadow." + api.method + "\n",
					call:         "shadow := parcel{}; shadow." + api.method + "()",
				}
				fixture := callableOwnerFixture(t, kind, scenario)
				fixture.goFile(t, callableOwnerCaller, callableOwnerSource(scenario, scenario.call))
				commitInventoryFixture(t, fixture.root)
				callableOwnerOutcome(t, fixture, kind, false)
			})
		}
	}
}

// TestCommittedConcreteMethodPromotedValueExecutesOwner requires a value
// method promoted through an embedded value to be executed by the
// registration walk. Method: packet = envelope{}, with envelope embedding
// parcel and Deliver a value method of parcel that installs the owner; the
// report publishes with that owner.
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
