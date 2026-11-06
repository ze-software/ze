// Design: docs/architecture/plugin/feature-gates.md -- root vendor identity survives nested manifests.
package repoinventory

import "testing"

// TestCommittedExternalVendorModuleManifestAuthority first admits a root-vendored
// package with exact manifest identity. A go.mod inside that package must not
// hide it from the root vendor manifest when its identity entry disappears.
func TestCommittedExternalVendorModuleManifestAuthority(t *testing.T) {
	fixture := externalEffectsFixture(t, "plugin")
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource("",
		"parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"))
	write(t, fixture.root, "go.mod", "module github.com/ze-software/ze\n\ngo 1.26\n\n"+
		"require example.invalid/helper v1.0.0\n")
	write(t, fixture.root, "vendor/modules.txt",
		"# example.invalid/helper v1.0.0\n## explicit; go 1.26\nexample.invalid/helper\n")
	fixture.goFile(t, "vendor/example.invalid/helper/parcel.go",
		"package helper\nvar Callback func()\n")
	commitInventoryFixture(t, fixture.root)
	externalEffectsPositive(t, fixture)

	write(t, fixture.root, "vendor/example.invalid/helper/go.mod",
		"module example.invalid/helper\n\ngo 1.26\n")
	write(t, fixture.root, "vendor/modules.txt", "")
	commitInventoryFixture(t, fixture.root)
	externalEffectsRefusal(t, fixture, "vendor identity")
}

// TestCommittedExternalNestedVendorModuleRefusal prevents a manifest inside an
// unresolved nested vendor tree from making that tree disappear. An independent
// module boundary above vendor remains covered by the example-module positive.
func TestCommittedExternalNestedVendorModuleRefusal(t *testing.T) {
	fixture := externalEffectsFixture(t, "plugin")
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource("",
		"parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"))
	commitInventoryFixture(t, fixture.root)
	externalEffectsPositive(t, fixture)

	const vendorDirectory = "pkg/container/vendor/example.invalid/helper"
	write(t, fixture.root, vendorDirectory+"/go.mod",
		"module example.invalid/helper\n\ngo 1.26\n")
	fixture.goFile(t, vendorDirectory+"/parcel.go", "package helper\nvar Callback func()\n")
	commitInventoryFixture(t, fixture.root)
	externalEffectsRefusal(t, fixture, "nested vendor")
}
