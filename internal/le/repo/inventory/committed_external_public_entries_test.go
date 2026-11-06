// Design: docs/architecture/plugin/feature-gates.md -- complete public-entry authority.
package repoinventory

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

const externalPublicEntryDir = "pkg/publicentry"
const externalPrivateEntryDir = "internal/publicparcel"

// externalPublicEntryFixture keeps both sinks and their ordinary callers
// internal while removing only the external cycle witnesses. No library name
// can establish the alternative entry certificate.
func externalPublicEntryFixture(t *testing.T, kind string) *asmInventoryFixture {
	t.Helper()
	fixture := externalEffectsFixture(t, kind)
	for _, sink := range []struct{ directory, name string }{
		{registrationPackage, "registry"},
		{"internal/component/config/yang", "yang"},
	} {
		fixture.goFile(t, sink.directory+"/external_witness.go", "package "+sink.name+"\n")
	}
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"))
	return fixture
}

func externalPublicHelperSource(packageName, declarations string) string {
	return "package " + packageName + "\nimport helper \"github.com/ze-software/ze/" + callableOwnerDir + "\"\nfunc keepImport() { _ = helper.Install }\n" + declarations + "\n"
}

// TestCommittedExternalPublicEntryCapabilities pairs every inert public entry
// with the same entry exposing registration through its retained implementation.
// The existing private installer stays present even in each inert control.
func TestCommittedExternalPublicEntryCapabilities(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		for _, route := range []string{"wrapper", "blank-initializer", "returned-callback", "returned-receiver"} {
			t.Run(kind+"/"+route, func(t *testing.T) {
				fixture := externalPublicEntryFixture(t, kind)
				public := ""
				private := ""
				switch route {
				case "wrapper":
					public = externalPublicHelperSource("publicentry", "func Invoke() { EFFECT }")
				case "blank-initializer":
					public = "package publicentry\nimport _ \"github.com/ze-software/ze/" + externalPrivateEntryDir + "\"\n"
					private = externalPublicHelperSource("parcel", "func init() { EFFECT }")
				case "returned-callback":
					public = "package publicentry\nimport parcel \"github.com/ze-software/ze/" + externalPrivateEntryDir + "\"\nfunc New() func() { return parcel.New() }\n"
					private = externalPublicHelperSource("parcel", "func New() func() { return func() { EFFECT } }")
				case "returned-receiver":
					public = "package publicentry\nimport parcel \"github.com/ze-software/ze/" + externalPrivateEntryDir + "\"\nfunc New() interface { Fire() } { return parcel.New() }\n"
					private = externalPublicHelperSource("parcel", "type hidden struct{}\nfunc New() hidden { return hidden{} }\nfunc (hidden) Fire() { EFFECT }")
				}
				fixture.goFile(t, externalPublicEntryDir+"/entry.go", strings.ReplaceAll(public, "EFFECT", ""))
				if private != "" {
					fixture.goFile(t, externalPrivateEntryDir+"/parcel.go", strings.ReplaceAll(private, "EFFECT", ""))
				}
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)

				fixture.goFile(t, externalPublicEntryDir+"/entry.go", strings.ReplaceAll(public, "EFFECT", "helper.Install(0)"))
				if private != "" {
					fixture.goFile(t, externalPrivateEntryDir+"/parcel.go", strings.ReplaceAll(private, "EFFECT", "helper.Install(0)"))
				}
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, "registration")
			})
		}
	}
}

// TestCommittedExternalCertainPublicInitializer counts initialization already
// selected by main exactly once, even though that package is a public entry.
func TestCommittedExternalCertainPublicInitializer(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalPublicEntryFixture(t, kind)
			const mainFile = "cmd/ze/main.go"
			fixture.goFile(t, mainFile, strings.Replace(fixture.goSources[mainFile], "func main()", "import _ \"github.com/ze-software/ze/"+externalPublicEntryDir+"\"\nfunc main()", 1))
			source := externalPublicHelperSource("publicentry", "func init() { EFFECT }")
			fixture.goFile(t, externalPublicEntryDir+"/entry.go", strings.Replace(source, "EFFECT", "", 1))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)

			fixture.goFile(t, externalPublicEntryDir+"/entry.go", strings.Replace(source, "EFFECT", "helper.Install(0)", 1))
			commitInventoryFixture(t, fixture.root)
			inv, err := Collect(fixture.root)
			if err != nil {
				t.Fatalf("certain public initializer refused: %v", err)
			}
			callableOwnerInstalled(t, fixture, kind, inv)
			plugins, err := Plugins(fixture.root)
			if err != nil {
				t.Fatalf("certain public initializer Plugins refused: %v", err)
			}
			if !reflect.DeepEqual(plugins, inv.Plugins) {
				t.Fatalf("certain initialization projections disagree: %+v / %+v", inv.Plugins, plugins)
			}
		})
	}
}

// TestCommittedExternalPublicRootRetainsInternalWrites preserves the explicit
// all-retained-writes model even when the writer is not a static public entry.
func TestCommittedExternalPublicRootRetainsInternalWrites(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalPublicEntryFixture(t, kind)
			source := "package writer\nimport (store \"github.com/ze-software/ze/" + externalNeutralDir + "\"; helper \"github.com/ze-software/ze/" + callableOwnerDir + "\")\nvar _ = helper.Install\nfunc later() { store.Callback = func() { EFFECT } }\n"
			const file = "internal/publicwriter/write.go"
			fixture.goFile(t, file, strings.Replace(source, "EFFECT", "", 1))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
			fixture.goFile(t, file, strings.Replace(source, "EFFECT", "helper.Install(0)", 1))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "registration")
		})
	}
}

// TestCommittedExternalPublicBoundaryRetainsVendorEscapes prevents the parent
// namespace theorem from reclassifying retained vendor bodies as opaque trust.
func TestCommittedExternalPublicBoundaryRetainsVendorEscapes(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		for _, escape := range []string{"unsafe", "linkname"} {
			t.Run(kind+"/"+escape, func(t *testing.T) {
				fixture := externalPublicEntryFixture(t, kind)
				write(t, fixture.root, "go.mod", "module github.com/ze-software/ze\n\ngo 1.26\n\nrequire example.invalid/retained v1.0.0\n")
				write(t, fixture.root, "vendor/modules.txt", "# example.invalid/retained v1.0.0\n## explicit; go 1.26\nexample.invalid/retained\n")
				const file = "vendor/example.invalid/retained/source.go"
				fixture.goFile(t, file, "package retained\nfunc Probe() {}\n")
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)

				source := "package retained\nimport raw \"unsafe\"\nfunc Probe() { _ = raw.Pointer(uintptr(1)) }\n"
				if escape == "linkname" {
					source = "package retained\nimport _ \"unsafe\"\n//go:linkname linked example.invalid/hidden.entry\nfunc linked() {}\nfunc Probe() { linked() }\n"
				}
				fixture.goFile(t, file, source)
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, escape)
			})
		}
	}
}

// TestCommittedExternalPublicConstructorReturnsPrivateMetadata separates a
// writable copy from the private location on which a sink's metadata depends.
func TestCommittedExternalPublicConstructorReturnsPrivateMetadata(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalPublicEntryFixture(t, kind)
			const registration = asmOrdinaryDir + "/register.go"
			source := strings.Replace(fixture.goSources[registration], "import (", "import (\n parcel \"github.com/ze-software/ze/"+externalPrivateEntryDir+"\"", 1)
			value := "ordinary committed control"
			if kind == "plugin" {
				source = strings.Replace(source, "Description: \"ordinary committed control\"", "Description: parcel.Label()", 1)
			} else {
				source = strings.Replace(source, "y.RegisterModule(\"extra.yang\", extraSchema)", "y.RegisterModule(\"extra.yang\", parcel.Label())", 1)
				value = asmExtraSchema
			}
			fixture.goFile(t, registration, source)
			fixture.goFile(t, externalPublicEntryDir+"/entry.go", "package publicentry\nimport parcel \"github.com/ze-software/ze/"+externalPrivateEntryDir+"\"\nfunc Reference() *string { return parcel.Reference() }\n")
			private := "package parcel\nvar label = " + strconv.Quote(value) + "\nfunc Label() string { return label }\nfunc Reference() *string { REFERENCE }\n"
			fixture.goFile(t, externalPrivateEntryDir+"/parcel.go", strings.Replace(private, "REFERENCE", "copy := "+strconv.Quote(value)+"; return &copy", 1))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)

			fixture.goFile(t, externalPrivateEntryDir+"/parcel.go", strings.Replace(private, "REFERENCE", "return &label", 1))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "metadata")
		})
	}
}

// TestCommittedExternalVendorInitializerReachesPublicWrapper retains a vendor
// initializer's ordinary call into Ze. The wrapper is itself a public entry;
// this regression does not claim that the vendor edge is its sole access path.
func TestCommittedExternalVendorInitializerReachesPublicWrapper(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalPublicEntryFixture(t, kind)
			write(t, fixture.root, "go.mod", "module github.com/ze-software/ze\n\ngo 1.26\n\nrequire example.invalid/retained v1.0.0\n")
			write(t, fixture.root, "vendor/modules.txt", "# example.invalid/retained v1.0.0\n## explicit; go 1.26\nexample.invalid/retained\n")
			fixture.goFile(t, "vendor/example.invalid/retained/source.go", "package retained\nimport entry \"github.com/ze-software/ze/"+externalPublicEntryDir+"\"\nfunc init() { entry.Invoke() }\n")
			source := externalPublicHelperSource("publicentry", "func Invoke() { EFFECT }")
			fixture.goFile(t, externalPublicEntryDir+"/entry.go", strings.Replace(source, "EFFECT", "", 1))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)

			fixture.goFile(t, externalPublicEntryDir+"/entry.go", strings.Replace(source, "EFFECT", "helper.Install(0)", 1))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "registration")
		})
	}
}
