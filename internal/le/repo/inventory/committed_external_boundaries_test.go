// Design: docs/architecture/plugin/feature-gates.md -- external identity and reference boundaries.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalNamedReferenceMetadata retains the backing location of
// a named slice, rather than recognizing only literal []T syntax as writable.
func TestCommittedExternalNamedReferenceMetadata(t *testing.T) {
	externalReferenceMetadataCase(t, "type labels []string\nvar Labels labels = labels{\"optional\"}\n")
}

// TestCommittedExternalReturnedReferenceMetadata follows an allocation through
// a retained result. The caller's declaration has neither a type nor a literal.
func TestCommittedExternalReturnedReferenceMetadata(t *testing.T) {
	externalReferenceMetadataCase(t, "func labels() []string { return []string{\"optional\"} }\nvar Labels = labels()\n")
}

func externalReferenceMetadataCase(t *testing.T, declarations string) {
	t.Helper()
	fixture := externalEffectsFixture(t, "plugin")
	neutral := "package neutralparcel\nimport (_ \"os\"; _ \"io\"; _ \"sort\"; _ \"time\")\n" +
		"const Label = \"ordinary committed control\"\n" + declarations
	fixture.goFile(t, externalNeutralFile, neutral)
	registration := asmOrdinaryDir + "/register.go"
	source := strings.Replace(fixture.goSources[registration], "import (", "import (\n store \"github.com/ze-software/ze/"+externalNeutralDir+"\"", 1)
	source = strings.Replace(source, "OptionalDependencies: []string{\"optional\"}", "OptionalDependencies: store.Labels", 1)
	fixture.goFile(t, registration, source)
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "order.Slice([]string{\"optional\"}, func(i, j int) bool { return false })"))
	commitInventoryFixture(t, fixture.root)
	externalEffectsPositive(t, fixture)
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "alias := store.Labels; order.Slice(alias, func(i, j int) bool { return false })"))
	commitInventoryFixture(t, fixture.root)
	externalEffectsRefusal(t, fixture, "metadata")
}

// TestCommittedExternalImportOnlyStaticCallback has no explicit opaque call.
// Import-time external execution can access the neutral exported callback and
// retain it until the retained assignment occurs during initialization.
func TestCommittedExternalImportOnlyStaticCallback(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			caller := "package ordinary\nimport (helper \"github.com/ze-software/ze/" + callableOwnerDir + "\"; store \"github.com/ze-software/ze/" + externalNeutralDir + "\"; _ \"os\")\n" +
				"var _ = helper.Install\nfunc init() { store.Callback = func() { BODY } }\n"
			fixture.goFile(t, callableOwnerCaller, strings.Replace(caller, "BODY", "", 1))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
			fixture.goFile(t, callableOwnerCaller, strings.Replace(caller, "BODY", "helper.Install(0)", 1))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "registration")
		})
	}
}

// TestCommittedExternalUnsafeManufactureRefuses gives unsafe both graph
// witnesses deliberately: an import-cycle proof cannot admit pointer fabrication
// that lies outside the separately approved reference-reachability authority.
func TestCommittedExternalUnsafeManufactureRefuses(t *testing.T) {
	fixture := externalEffectsFixture(t, "plugin")
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"))
	commitInventoryFixture(t, fixture.root)
	externalEffectsPositive(t, fixture)
	for _, sink := range []string{registrationPackage, "internal/component/config/yang"} {
		name := sink + "/external_witness.go"
		fixture.goFile(t, name, strings.Replace(fixture.goSources[name], "import (", "import (_ \"unsafe\";", 1))
	}
	source := externalEffectsSource("", "address := raw.Pointer(uintptr(1)); _ = address; parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault")
	source = strings.Replace(source, "import (", "import (raw \"unsafe\";", 1)
	fixture.goFile(t, callableOwnerCaller, source)
	commitInventoryFixture(t, fixture.root)
	externalEffectsRefusal(t, fixture, "unsafe")
}
