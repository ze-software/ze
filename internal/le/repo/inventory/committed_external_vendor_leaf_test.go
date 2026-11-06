// Design: docs/architecture/plugin/feature-gates.md -- package identity differs from vendor descendants.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalVendorDirectoryPackage selects an ordinary package whose
// directory happens to be named vendor, then adds an actual dependency below
// it. The package keeps the exact report; unresolved descendants still refuse.
func TestCommittedExternalVendorDirectoryPackage(t *testing.T) {
	fixture := externalEffectsFixture(t, "plugin")
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource("",
		"parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"))
	commitInventoryFixture(t, fixture.root)
	externalEffectsPositive(t, fixture)

	const packageDirectory = "internal/example/vendor"
	fixture.goFile(t, packageDirectory+"/tool.go",
		"package vendortool\nfunc init() {}\nfunc Value() int { return 1 }\n")
	const mainFile = "cmd/ze/main.go"
	fixture.goFile(t, mainFile, strings.Replace(fixture.goSources[mainFile],
		"func main()", "import _ \"github.com/ze-software/ze/"+packageDirectory+"\"\nfunc main()", 1))
	commitInventoryFixture(t, fixture.root)
	externalEffectsPositive(t, fixture)

	fixture.goFile(t, packageDirectory+"/example.invalid/helper/parcel.go",
		"package helper\nvar Callback func()\n")
	commitInventoryFixture(t, fixture.root)
	externalEffectsRefusal(t, fixture, "nested vendor")
}

// TestCommittedExternalVendorDirectoryModuleBoundary selects a package at the
// vendor directory itself with its own module manifest. Its main-module import
// crosses a local module boundary, so the refusal must identify that boundary.
func TestCommittedExternalVendorDirectoryModuleBoundary(t *testing.T) {
	fixture := externalEffectsFixture(t, "plugin")
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource("",
		"parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"))
	commitInventoryFixture(t, fixture.root)
	externalEffectsPositive(t, fixture)

	const packageDirectory = "internal/example/vendor"
	fixture.goFile(t, packageDirectory+"/tool.go", "package vendortool\nfunc init() {}\n")
	write(t, fixture.root, packageDirectory+"/go.mod",
		"module example.invalid/vendortool\n\ngo 1.26\n")
	const mainFile = "cmd/ze/main.go"
	fixture.goFile(t, mainFile, strings.Replace(fixture.goSources[mainFile],
		"func main()", "import _ \"github.com/ze-software/ze/"+packageDirectory+"\"\nfunc main()", 1))
	commitInventoryFixture(t, fixture.root)
	externalEffectsRefusal(t, fixture, "nested module")
}
