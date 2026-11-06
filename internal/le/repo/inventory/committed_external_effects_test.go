// Design: docs/architecture/plugin/feature-gates.md -- Git-only external capability closure.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalOpaqueResults retain both result slots and interface
// receivers without naming a privileged library or method in the evaluator.
// Nilness is unknown; source traversal analyzes both branches without executing.
func TestCommittedExternalOpaqueResults(t *testing.T) {
	for _, operation := range []struct{ name, call string }{
		{"file-interface", "parcel, fault := remote.Stat(\".\"); if fault != nil { _ = fault.Error() }; if parcel != nil { _ = parcel.IsDir() }"},
		{"pipe-receivers", "reader, writer := conduit.Pipe(); _ = reader.Close(); _ = writer.Close()"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(operation.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", operation.call))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)
			})
		}
	}
}

// TestCommittedExternalRequiresBothSinkWitnesses removes one graph witness at
// a time after an exact positive. Public wrappers expose each semantic sink
// when its cycle witness disappears; a package-name exemption cannot pass it.
func TestCommittedExternalRequiresBothSinkWitnesses(t *testing.T) {
	for _, sink := range []struct{ directory, name string }{
		{registrationPackage, "registry"},
		{"internal/component/config/yang", "yang"},
	} {
		t.Run(sink.name, func(t *testing.T) {
			fixture := externalEffectsFixture(t, "plugin")
			fixture.goFile(t, "pkg/publicregistry/entry.go", "package publicregistry\nimport api \"github.com/ze-software/ze/"+registrationPackage+"\"\nfunc engine() {}\nfunc Install() { _ = api.Register(api.Registration{Name: \"possible-public-plugin\", RunEngine: engine}) }\n")
			fixture.goFile(t, "pkg/publicyang/entry.go", "package publicyang\nimport api \"github.com/ze-software/ze/internal/component/config/yang\"\nfunc Install() { api.RegisterModule(\"possible-public.yang\", \"module possible { namespace urn:possible; prefix p; }\") }\n")
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
			fixture.goFile(t, sink.directory+"/external_witness.go", "package "+sink.name+"\n")
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "registration")
		})
	}
}

// TestCommittedExternalCallbackEffects requires conservative callback execution,
// captured ownership, and binding-sensitive summaries for repeated helper calls.
func TestCommittedExternalCallbackEffects(t *testing.T) {
	for _, operation := range []struct{ name, declarations, control, effect string }{
		{"closure", "", "order.Slice([]int{1, 2}, func(i, j int) bool { return false })", "order.Slice([]int{1, 2}, func(i, j int) bool { install(0); return false })"},
		{"retained-later", "", "_ = clock.AfterFunc(0, func() {}); parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault", "_ = clock.AfterFunc(0, func() { install(0) }); parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"},
		{"inert-first", "func passParcel(callback func()) { _ = clock.AfterFunc(0, callback) }\n", "passParcel(func() {}); passParcel(func() {})", "passParcel(func() {}); passParcel(func() { install(0) })"},
		{"captured", "func passParcel(callback func()) { _ = clock.AfterFunc(0, func() { callback() }) }\n", "passParcel(func() {})", "passParcel(func() { install(0) })"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(operation.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(operation.declarations, operation.control))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(operation.declarations, operation.effect))
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, "registration")
			})
		}
	}
}

// TestCommittedExternalNeutralCallbackClosure transfers no installer argument
// externally. A possibly imported neutral global supplies the capability, even
// when the assignment is discovered after a previously inert external call.
func TestCommittedExternalNeutralCallbackClosure(t *testing.T) {
	for _, assignment := range []string{
		"store.Callback = func() { install(0) }; parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault",
		"parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault; store.Callback = func() { install(0) }",
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(kind+"/"+assignment, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				control := strings.ReplaceAll(assignment, "install(0)", "")
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", control))
				commitInventoryFixture(t, fixture.root)
				snapshot := externalEffectsPositive(t, fixture)
				expected := asmHeaderExpected(fixture)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", assignment))
				externalEffectsPositive(t, expected)
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, "registration")
				old, err := snapshot.collect()
				if err != nil {
					t.Fatalf("new callback contaminated retained snapshot: %v", err)
				}
				expected.checkReport(t, old)
				plugins, err := snapshot.Plugins()
				if err != nil {
					t.Fatal(err)
				}
				expected.checkPlugins(t, plugins)
			})
		}
	}
}

// TestCommittedExternalNeutralMetadata rejects a mutable static-access root,
// including metadata collected before another package's external operation.
func TestCommittedExternalNeutralMetadata(t *testing.T) {
	fixture := externalEffectsFixture(t, "plugin")
	name := asmOrdinaryDir + "/register.go"
	source := strings.Replace(fixture.goSources[name], "import (", "import (\n store \"github.com/ze-software/ze/"+externalNeutralDir+"\"", 1)
	source = strings.Replace(source, "Description: \"ordinary committed control\"", "Description: store.Label", 1)
	fixture.goFile(t, name, source)
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"))
	commitInventoryFixture(t, fixture.root)
	externalEffectsPositive(t, fixture)
	fixture.goFile(t, externalNeutralFile, strings.Replace(externalNeutralSource, "const Label", "var Label", 1))
	commitInventoryFixture(t, fixture.root)
	externalEffectsRefusal(t, fixture, "metadata")
}
