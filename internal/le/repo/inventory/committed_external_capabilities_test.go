// Design: docs/architecture/plugin/feature-gates.md -- transitive external capabilities.
package repoinventory

import (
	"strings"
	"testing"
)

// TestCommittedExternalReturnedPrivateReceiver follows an exported method on
// an unexported returned owner. The package imports an installer, so the import
// cycle excludes direct external access; only the transferred method exposes it.
func TestCommittedExternalReturnedPrivateReceiver(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			neutral := "package neutralparcel\nimport helper \"github.com/ze-software/ze/" + callableOwnerDir + "\"\n" +
				"var _ = helper.Install\nvar Callback func()\nconst Label = \"ordinary committed control\"\n" +
				"type parcel struct{}\nfunc Parcel() parcel { return parcel{} }\nfunc (parcel) Fire() { BODY }\n"
			fixture.goFile(t, externalNeutralFile, strings.Replace(neutral, "BODY", "", 1))
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "parcel := store.Parcel(); _ = clock.AfterFunc(0, parcel.Fire)"))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
			fixture.goFile(t, externalNeutralFile, strings.Replace(neutral, "BODY", "helper.Install(0)", 1))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "registration")
		})
	}
}

// TestCommittedExternalNamedScalarMethods makes scalar elements reachable
// through an opaque operation's interface argument. Their method set, not just
// their integer bits or the direct less callback, determines possible effects.
func TestCommittedExternalNamedScalarMethods(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			declarations := "type parcelNumber uint32\nfunc (parcelNumber) Fire() { BODY }\n"
			call := "order.Slice([]parcelNumber{1, 2}, func(i, j int) bool { return false })"
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(strings.Replace(declarations, "BODY", "", 1), call))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(strings.Replace(declarations, "BODY", "install(0)", 1), call))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "registration")
		})
	}
}

// TestCommittedExternalWritableMetadataAlias excludes static access to the
// neutral package through each external origin. A copied string is inert; a
// pointer to its registration metadata, nested in a slice, remains writable.
func TestCommittedExternalWritableMetadataAlias(t *testing.T) {
	fixture := externalEffectsFixture(t, "plugin")
	neutral := "package neutralparcel\nimport (_ \"os\"; _ \"io\"; _ \"sort\"; _ \"time\")\nvar Callback func()\nvar Label = \"ordinary committed control\"\n"
	fixture.goFile(t, externalNeutralFile, neutral)
	registration := asmOrdinaryDir + "/register.go"
	source := strings.Replace(fixture.goSources[registration], "import (", "import (\n store \"github.com/ze-software/ze/"+externalNeutralDir+"\"", 1)
	source = strings.Replace(source, "Description: \"ordinary committed control\"", "Description: store.Label", 1)
	fixture.goFile(t, registration, source)
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "order.Slice([]string{store.Label}, func(i, j int) bool { return false })"))
	commitInventoryFixture(t, fixture.root)
	externalEffectsPositive(t, fixture)
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "alias := &store.Label; order.Slice([]*string{alias}, func(i, j int) bool { return false })"))
	commitInventoryFixture(t, fixture.root)
	externalEffectsRefusal(t, fixture, "metadata")
}

// TestCommittedExternalUnselectedStaticRoots includes writes outside the root
// import graph. The approved model conservatively includes later retained
// writes: the writer need not itself be importable by the external origin.
func TestCommittedExternalUnselectedStaticRoots(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			const neutral = "pkg/unselectedparcel/parcel.go"
			const writer = "internal/unselectedwriter/write.go"
			fixture.goFile(t, neutral, "package unselectedparcel\nvar Callback func()\n")
			body := "package unselectedwriter\nimport (store \"github.com/ze-software/ze/pkg/unselectedparcel\"; helper \"github.com/ze-software/ze/" + callableOwnerDir + "\")\n" +
				"var _ = helper.Install\nfunc Publish() { store.Callback = func() { BODY } }\n"
			fixture.goFile(t, writer, strings.Replace(body, "BODY", "", 1))
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"))
			commitInventoryFixture(t, fixture.root)
			snapshot := externalEffectsPositive(t, fixture)
			selected, _ := snapshot.Sources()
			for _, source := range selected {
				if source.Path == neutral {
					t.Fatal("unselected static root entered the ordinary import graph")
				}
				if source.Path == writer {
					t.Fatal("unselected writer entered the ordinary import graph")
				}
			}
			fixture.goFile(t, writer, strings.Replace(body, "BODY", "helper.Install(0)", 1))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "registration")
		})
	}
}

// TestCommittedExternalLateMetadataInvalidatesEarlierRegistration places its
// external call in a distinct selected package after ordinary in the reader's
// sorted package traversal. Earlier metadata must not escape final revalidation.
func TestCommittedExternalLateMetadataInvalidatesEarlierRegistration(t *testing.T) {
	fixture := externalEffectsFixture(t, "plugin")
	const later = "internal/zzexternal"
	if strings.Compare(asmOrdinaryDir, later) >= 0 {
		t.Fatal("late external package does not follow the registration package")
	}
	main := "cmd/ze/main.go"
	fixture.goFile(t, main, strings.Replace(fixture.goSources[main], "func main()", "import _ \"github.com/ze-software/ze/"+later+"\"\nfunc main()", 1))
	fixture.goFile(t, later+"/later.go", "package zzexternal\nimport remote \"os\"\nfunc init() { parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault }\n")
	registration := asmOrdinaryDir + "/register.go"
	source := strings.Replace(fixture.goSources[registration], "import (", "import (\n store \"github.com/ze-software/ze/"+externalNeutralDir+"\"", 1)
	source = strings.Replace(source, "Description: \"ordinary committed control\"", "Description: store.Label", 1)
	fixture.goFile(t, registration, source)
	commitInventoryFixture(t, fixture.root)
	externalEffectsPositive(t, fixture)
	fixture.goFile(t, externalNeutralFile, strings.Replace(externalNeutralSource, "const Label", "var Label", 1))
	commitInventoryFixture(t, fixture.root)
	externalEffectsRefusal(t, fixture, "metadata")
}
