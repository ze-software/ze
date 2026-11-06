// Design: docs/architecture/plugin/feature-gates.md -- linked definitions remain external capability roots.
package repoinventory

import (
	"strings"
	"testing"
)

// A private uncalled Go body is inert under ordinary imports, but assigning it
// another linker identity exposes that retained body outside the proved graph.
func TestCommittedExternalLinkedPrivateDefinitionRefuses(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			declaration := "func hiddenParcel() { install(0) }\n"
			call := "parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"
			source := externalEffectsSource(declaration, call)
			fixture.goFile(t, callableOwnerCaller, source)
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)

			linked := "//go:linkname hiddenParcel example.org/parcel.deliver\n" + declaration
			source = externalEffectsSource(linked, call)
			source = strings.Replace(source, "import (", "import (_ \"unsafe\";", 1)
			fixture.goFile(t, callableOwnerCaller, source)
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "linkname")
		})
	}
}
