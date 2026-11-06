// Design: docs/architecture/plugin/feature-gates.md -- public initializer imports grow ambient authority.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalPublicBlankImportAddsOrigin excludes the installer from
// every initial opaque origin. Only the unselected public entry's blank import
// creates an origin with no cycle back from that installer.
func TestCommittedExternalPublicBlankImportAddsOrigin(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalPublicEntryFixture(t, kind)
			fixture.goFile(t, externalPublicEntryDir+"/entry.go", "package publicentry\nimport _ \"encoding/hex\"\n")
			const file = "pkg/origininstaller/install.go"
			source := "package origininstaller\nimport (_ \"os\"; _ \"io\"; _ \"sort\"; _ \"time\"; helper \"github.com/ze-software/ze/" + callableOwnerDir + "\")\nfunc Install() { _ = helper.Install; EFFECT }\n"
			fixture.goFile(t, file, strings.Replace(source, "EFFECT", "", 1))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)

			fixture.goFile(t, file, strings.Replace(source, "EFFECT", "helper.Install(0)", 1))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "registration")
		})
	}
}
