// Design: docs/architecture/plugin/feature-gates.md -- opaque operations reach map keys too.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalMapKeyCapability retains callable methods reachable
// only through an interface-typed map key. Both programs use a comparable key
// and the ordinary fmt API. The external abstraction must include that key's
// method set; it cannot inspect only map values or the static any key type.
func TestCommittedExternalMapKeyCapability(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			for _, sink := range []string{registrationPackage, "internal/component/config/yang"} {
				name := sink + "/external_witness.go"
				fixture.goFile(t, name, strings.Replace(fixture.goSources[name], "import (", "import (_ \"fmt\";", 1))
			}
			const declarations = "type parcelKey struct{}\nfunc (parcelKey) String() string { BODY return \"key\" }\n"
			const call = "_ = printer.Sprint(map[any]int{parcelKey{}: 0})"
			source := externalEffectsSource(strings.Replace(declarations, "BODY", "", 1), call)
			source = strings.Replace(source, "import (", "import (printer \"fmt\";", 1)
			fixture.goFile(t, callableOwnerCaller, source)
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
			source = externalEffectsSource(strings.Replace(declarations, "BODY", "install(0);", 1), call)
			source = strings.Replace(source, "import (", "import (printer \"fmt\";", 1)
			fixture.goFile(t, callableOwnerCaller, source)
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "registration")
		})
	}
}
