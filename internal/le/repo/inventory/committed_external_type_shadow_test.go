// Design: docs/architecture/plugin/feature-gates.md -- lexical type capabilities.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalCrossFileBuiltinTypeShadow resolves package types before
// the predeclared universe. The use-site parser has no cross-file object link;
// its slice element still carries the package type's retained method set.
func TestCommittedExternalCrossFileBuiltinTypeShadow(t *testing.T) {
	for _, name := range []string{"int", "uint32"} {
		t.Run(name, func(t *testing.T) {
			fixture := externalEffectsFixture(t, "plugin")
			for _, sink := range []string{registrationPackage, "internal/component/config/yang"} {
				file := sink + "/external_witness.go"
				fixture.goFile(t, file, strings.Replace(fixture.goSources[file], "import (", "import (_ \"fmt\";", 1))
			}
			declaration := "package ordinary\ntype " + name + " uint64\nfunc (" + name + ") Fire() { BODY }\n"
			methodFile := asmOrdinaryDir + "/external_shape.go"
			fixture.goFile(t, methodFile, strings.Replace(declaration, "BODY", "", 1))
			caller := externalEffectsSource("", "_ = printer.Sprint([]"+name+"{1, 2})")
			caller = strings.Replace(caller, "import (", "import (printer \"fmt\";", 1)
			fixture.goFile(t, callableOwnerCaller, caller)
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
			fixture.goFile(t, methodFile, strings.Replace(declaration, "BODY", "install(0)", 1))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "registration")
		})
	}
}
