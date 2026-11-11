// Design: docs/architecture/plugin/feature-gates.md -- unsupported callable syntax must not publish partial membership.
package repoinventory

import (
	"reflect"
	"slices"
	"testing"
)

func runCallableOwnerCase(t *testing.T, scenario callableOwnerCase) {
	t.Helper()
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := callableOwnerFixture(t, kind, scenario)
			fixture.goFile(t, callableOwnerCaller, callableOwnerSource(scenario, scenario.call))
			commitInventoryFixture(t, fixture.root)
			callableOwnerOutcome(t, fixture, kind, true)
		})
	}
}

// Named installer aliases, not just aliases of registry.Register itself, must
// reach their defining Go body or cause a named execution-ownership refusal.
func TestCommittedCallableHelperOwnersCannotDisappear(t *testing.T) {
	cases := []callableOwnerCase{
		{name: "local-helper", call: "invoke := install; invoke(ownerProbe())"},
		{name: "package-helper", declarations: "var ownerAlias = install", call: "ownerAlias(ownerProbe())"},
		{name: "local-chain", call: "first := install; second := first; second(ownerProbe())"},
		{name: "parenthesized-local", call: "invoke := install; ((invoke))(ownerProbe())"},
		{name: "chained-parenthesized-package", declarations: "var firstAlias = install\nvar ownerAlias = (firstAlias)", call: "((ownerAlias))(ownerProbe())"},
		{name: "imported-helper", call: "invoke := helper.Install; invoke(ownerProbe())"},
	}
	for _, scenario := range cases {
		t.Run(scenario.name, func(t *testing.T) { runCallableOwnerCase(t, scenario) })
	}
}

// The normalized callee must reach the execution consumer, not merely an alias
// guard. These ordinary Go probes cannot stop at an old assembly/header refusal.
func TestCommittedParenthesizedCallableOwnersCannotDisappear(t *testing.T) {
	cases := []callableOwnerCase{
		{name: "direct", call: "(install)(ownerProbe())"},
		{name: "nested-direct", call: "(((install)))(ownerProbe())"},
		{name: "imported", call: "(helper.Install)(ownerProbe())"},
		{name: "nested-imported", call: "(((helper.Install)))(ownerProbe())"},
		{name: "immediate-closure", call: "(func(_ uint32) { install(0) })(ownerProbe())"},
		{name: "nested-immediate-closure", call: "(((func(_ uint32) { install(0) })))(ownerProbe())"},
	}
	for _, scenario := range cases {
		t.Run(scenario.name, func(t *testing.T) { runCallableOwnerCase(t, scenario) })
	}
}

// Even a closure that ignores its argument must retain registrations performed
// while producing that argument, or explicitly refuse the executable owner.
func TestCommittedCallableOwnerArgumentsCannotDisappear(t *testing.T) {
	cases := []callableOwnerCase{
		{name: "immediate", call: "func(_ uint32) {}(helper.ArgumentInstall())"},
		{name: "parenthesized-immediate", call: "(((func(_ uint32) {})))(helper.ArgumentInstall())"},
	}
	for _, scenario := range cases {
		t.Run(scenario.name, func(t *testing.T) { runCallableOwnerCase(t, scenario) })
	}
}

// Missing import lookup, opaque receivers and generic/indexed/returned callees
// are not external-standard ownership. Local receiver methods carry a real
// installer so ignoring them cannot pass as an ordinary-only complete report.
func TestCommittedOpaqueCallableOwnersCannotDisappear(t *testing.T) {
	method := "type ownerHook struct{}\nfunc (ownerHook) Run(_ uint32) { install(0) }\n"
	cases := []callableOwnerCase{
		{name: "indexed-function", declarations: "var ownerCallbacks = []func(uint32){install}", call: "ownerCallbacks[ownerProbe()](ownerProbe())"},
		{name: "returned-function", declarations: "func ownerFactory() func(uint32) { return install }", call: "ownerFactory()(ownerProbe())"},
		{name: "local-method", declarations: method, call: "local := ownerHook{}; local.Run(ownerProbe())"},
		{name: "local-function-field", declarations: "type ownerFields struct { Run func(uint32) }", call: "local := ownerFields{Run: install}; local.Run(ownerProbe())"},
		{name: "indexed-receiver", declarations: method + "var ownerHooks = []ownerHook{{}}", call: "ownerHooks[ownerProbe()].Run(ownerProbe())"},
		{name: "returned-receiver", declarations: method + "func ownerReceiver() ownerHook { return ownerHook{} }", call: "ownerReceiver().Run(ownerProbe())"},
		{name: "shadowed-standard-import", extraImport: "std \"fmt\"", witnessImports: []string{"fmt"}, declarations: method + "var _ = std.Sprintf", prefix: "_ = std.Sprintf(\"ordinary import control\")\n", call: "std := ownerHook{}; std.Run(ownerProbe())"},
		{name: "shadowed-retained-import", declarations: method, prefix: "_ = helper.Install\n", call: "helper := ownerHook{}; helper.Run(ownerProbe())"},
		{name: "generic-index", declarations: "func ownerGeneric[T any](_ uint32) { install(0) }", call: "ownerGeneric[int](ownerProbe())"},
		{name: "generic-index-list", declarations: "func ownerGeneric[T, U any](_ uint32) { install(0) }", call: "ownerGeneric[int, string](ownerProbe())"},
	}
	for _, scenario := range cases {
		t.Run(scenario.name, func(t *testing.T) { runCallableOwnerCase(t, scenario) })
	}
}

// Dirty source cannot change the old full report. After HEAD moves, the
// captured Plugins/Go paths retain their old owners or precise refusal while a
// fresh full report reads the new ordinary-only commit. No pinned full-report
// API is invented for YANG: its old module is checked by Collect before HEAD
// moves, and the captured public Plugins call still checks traversal/refusal.
func TestCommittedCallableOwnersRetainSnapshot(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			scenario := callableOwnerCase{call: "invoke := install; invoke(ownerProbe())"}
			fixture := callableOwnerFixture(t, kind, scenario)
			fixture.goFile(t, callableOwnerCaller, callableOwnerSource(scenario, scenario.call))
			commitInventoryFixture(t, fixture.root)
			snapshot, plugins, refusal := callableOwnerOutcome(t, fixture, kind, true)
			expected := frozenFixture(fixture)

			fixture.goFile(t, callableOwnerCaller, callableOwnerSource(scenario, "_ = ownerProbe()"))
			fixture.goFile(t, callableOwnerFile, "package ownerhelper\nfunc Install(_ uint32) {}\nfunc ArgumentInstall() uint32 { return 0 }\n")
			marker := ordinaryDir + "/owner_later_head.go"
			fixture.goFile(t, marker, "package ordinary\nconst LaterOwnerHead = true\n")
			callableOwnerOutcome(t, expected, kind, true)
			callableOwnerRetained(t, snapshot, plugins, refusal)

			commitInventoryFixture(t, fixture.root)
			callableOwnerRetained(t, snapshot, plugins, refusal)
			fresh, err := Collect(fixture.root)
			if err != nil {
				t.Fatalf("fresh ordinary-only owner commit: %v", err)
			}
			fixture.checkReport(t, fresh)
			current, err := ReadCommitted(t.Context(), fixture.root)
			if err != nil {
				t.Fatal(err)
			}
			if current.Revision() == snapshot.Revision() {
				t.Fatal("owner HEAD fixture did not move")
			}
			paths := append(callableOwnerPaths(), marker)
			slices.Sort(paths)
			checkSelectedPaths(t, current, paths)
		})
	}
}

// TestCommittedMethodReceiverCallExecutesRegistration requires the
// registration walk to execute a directly called method's receiver, as Go
// evaluates it before the arguments. Method: build() installs the owner and
// returns a value whose plain Register method is empty, so the report holds
// the owner only if build().Register() executed build(). The fixture imports
// no external package, so the registration walk alone decides.
//
// MUTATION: (internal/le/repo/inventory/registrations.go,
// `if ast.Unparen(current.Fun) == callee.value {`,
// `if ast.Unparen(current.Fun) == nil {`): the receiver is never walked, so
// build() never runs and the report publishes without the owner: the plugin
// case fails as the executed callable plugin owner disappeared, the YANG case
// as the executed callable YANG owner disappeared. The parenthesised call is
// the same direct call. Through the method-value alias the receiver already
// ran, unconditionally, where the value was taken, so the alias called inside
// a loop still publishes.
//
// MUTATION: (internal/le/repo/inventory/registrations.go,
// `if ast.Unparen(current.Fun) == callee.value {`, `if true {`): the alias
// call runs build() again inside the loop, and its registration refuses as
// depending on an unresolved condition or iteration.
func TestCommittedMethodReceiverCallExecutesRegistration(t *testing.T) {
	scenario := callableOwnerCase{
		declarations: "type sealed struct { size int }\n" +
			"func build() sealed { install(0); return sealed{size: 0} }\n" +
			"func (s sealed) Register() { _ = s }\n",
	}
	for _, call := range []string{"build().Register()", "(build().Register)()", "f := build().Register; for range 2 { f() }"} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(call+"/"+kind, func(t *testing.T) {
				fixture := callableOwnerFixture(t, kind, scenario)
				fixture.goFile(t, callableOwnerCaller, callableOwnerSource(scenario, call))
				commitInventoryFixture(t, fixture.root)
				if _, _, err := callableOwnerOutcome(t, fixture, kind, false); err != nil {
					t.Fatalf("receiver call refused its registration: %v", err)
				}
			})
		}
	}
}

// TestCommittedReceiverFactoryKeepsCertainRegistration compares a
// statement-separated receiver with the same retained factory evaluated inside
// an opaque method call. The factory installs synchronously before returning
// a receiver whose type is outside Ze; the opaque outer call's receiver
// operand still runs, so that certain registration stays in the report and in
// the declared plugin list.
func TestCommittedReceiverFactoryKeepsCertainRegistration(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := importingFixture(t, kind)
			const factory = "func installedParcel() remote.FileInfo { install(0); parcel, fault := remote.Stat(\".\"); _ = fault; return parcel }\n"
			for _, call := range []string{
				"parcel := installedParcel(); _ = parcel.IsDir()",
				"_ = installedParcel().IsDir()",
			} {
				fixture.goFile(t, callableOwnerCaller, importingSource(factory, call))
				commitInventoryFixture(t, fixture.root)
				inv, err := Collect(fixture.root)
				if err != nil {
					t.Fatalf("receiver factory %q refused certain registration: %v", call, err)
				}
				callableOwnerInstalled(t, fixture, kind, inv)
				plugins, err := DeclaredPlugins(fixture.root)
				if err != nil {
					t.Fatalf("receiver factory %q DeclaredPlugins refused: %v", call, err)
				}
				if !reflect.DeepEqual(plugins, inv.Plugins) {
					t.Fatalf("receiver factory %q public outputs disagree: report %+v, plugins %+v", call, inv.Plugins, plugins)
				}
			}
		})
	}
}
