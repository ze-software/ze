// Design: docs/architecture/plugin/feature-gates.md -- external origins retain module identity.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalNestedDotlessNamespaceRefuses distinguishes an inert
// independent checkout from unresolved imports of its declared namespace. A
// dotless name and two authored cycle witnesses cannot establish stdlib ownership
// or select that checkout as a dependency of the root module.
func TestCommittedExternalNestedDotlessNamespaceRefuses(t *testing.T) {
	fixture := externalEffectsFixture(t, "plugin")
	const nested = "examples/independent-parcel"
	const identity = "example/parcel/private"
	write(t, fixture.root, nested+"/go.mod", "module example/parcel\n\ngo 1.26\n")
	fixture.goFile(t, nested+"/private/source.go", "package private\nconst Label = \"independent\"\n")
	caller := externalEffectsSource("", "parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault")
	fixture.goFile(t, callableOwnerCaller, caller)
	commitInventoryFixture(t, fixture.root)
	externalEffectsPositive(t, fixture)

	// Both sink witnesses intentionally satisfy the cycle check. The missing
	// provider must still refuse at identity selection, before effect closure.
	for _, sink := range []struct{ directory, name string }{
		{registrationPackage, "registry"},
		{"internal/component/config/yang", "yang"},
	} {
		fixture.goFile(t, sink.directory+"/nested_namespace_witness.go", "package "+sink.name+"\nimport _ \""+identity+"\"\n")
	}
	fixture.goFile(t, callableOwnerCaller, strings.Replace(caller, "import (", "import (\n _ \""+identity+"\"", 1))
	commitInventoryFixture(t, fixture.root)
	externalEffectsRefusal(t, fixture, "identity")
}

// TestCommittedExternalMalformedNestedIdentityRefuses keeps the independent
// source inert. A malformed manifest cannot certify which import namespace the
// boundary excludes, even though its directory remains outside root sources.
func TestCommittedExternalMalformedNestedIdentityRefuses(t *testing.T) {
	fixture := externalEffectsFixture(t, "plugin")
	const nested = "examples/independent-parcel"
	const manifest = nested + "/go.mod"
	write(t, fixture.root, manifest, "module example/parcel\n\ngo 1.26\n")
	fixture.goFile(t, nested+"/private/source.go", "package private\nconst Label = \"independent\"\n")
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"))
	commitInventoryFixture(t, fixture.root)
	externalEffectsPositive(t, fixture)

	write(t, fixture.root, manifest, "module (\n")
	commitInventoryFixture(t, fixture.root)
	externalEffectsRefusal(t, fixture, "identity")
}
