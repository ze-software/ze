// Design: docs/architecture/plugin/feature-gates.md -- syntax nonimportability bounds potential source.
package repoinventory

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

const externalInvalidPotentialDir = "arbitrary/place/parcel"
const externalInvalidPotentialSource = "package parcel\nfunc broken( {\n"

// TestCommittedExternalInvalidPotentialPackage retains every report input while
// excluding a complete syntax-invalid package from possible external imports.
// The valid sibling installer proves that exclusion applies to the package.
func TestCommittedExternalInvalidPotentialPackage(t *testing.T) {
	fixture := externalPotentialFixture(t)
	fixture.goFile(t, externalInvalidPotentialDir+"/broken.go", externalInvalidPotentialSource)
	fixture.goFile(t, externalInvalidPotentialDir+"/installer.go", externalPotentialInstaller("parcel", ""))
	write(t, fixture.root, externalInvalidPotentialDir+"/textflag.h", "committed header-presence control\n")
	commitInventoryFixture(t, fixture.root)
	snapshot := externalEffectsPositive(t, fixture)
	if !snapshot.headerPresence[externalInvalidPotentialDir+"/textflag.h"] {
		t.Fatal("syntax exclusion erased raw assembly-header presence")
	}
	for _, name := range []string{"broken.go", "installer.go"} {
		if !slices.Contains(snapshot.paths, externalInvalidPotentialDir+"/"+name) {
			t.Fatalf("syntax exclusion changed report census: %s", name)
		}
	}
	private, err := newExternalSource(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, included := private.snapshot.packages[externalInvalidPotentialDir]; included {
		t.Fatal("syntax-invalid potential package entered the private graph")
	}
}

// TestCommittedExternalInvalidPotentialDependents proves a valid importer cannot
// acquire an excluded package as an opaque leaf and contribute its installer.
func TestCommittedExternalInvalidPotentialDependents(t *testing.T) {
	fixture := externalPotentialFixture(t)
	fixture.goFile(t, externalInvalidPotentialDir+"/broken.go", externalInvalidPotentialSource)
	const intermediate = "another/arbitrary/path"
	fixture.goFile(t, intermediate+"/source.go", "package relay\nimport _ \"github.com/ze-software/ze/"+externalInvalidPotentialDir+"\"\n")
	const dependent = "unrelated/installer"
	fixture.goFile(t, dependent+"/source.go", externalPotentialInstaller("installer", "_ \"github.com/ze-software/ze/"+intermediate+"\""))
	commitInventoryFixture(t, fixture.root)
	snapshot := externalEffectsPositive(t, fixture)
	private, err := newExternalSource(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{externalInvalidPotentialDir, intermediate, dependent} {
		if _, included := private.snapshot.packages[directory]; included {
			t.Fatalf("nonimportable dependency entered private graph: %s", directory)
		}
	}
}

// TestCommittedExternalInvalidCertainPackage preserves the refusal when a
// certain import requires the same syntax-invalid retained package.
func TestCommittedExternalInvalidCertainPackage(t *testing.T) {
	fixture := externalPotentialFixture(t)
	fixture.goFile(t, externalInvalidPotentialDir+"/broken.go", externalInvalidPotentialSource)
	fixture.goFile(t, "cmd/ze/broken_import.go", "package main\nimport _ \"github.com/ze-software/ze/"+externalInvalidPotentialDir+"\"\n")
	commitInventoryFixture(t, fixture.root)
	inv, reportErr := Collect(fixture.root)
	plugins, pluginErr := Plugins(fixture.root)
	if reportErr == nil {
		t.Fatalf("certain invalid package published report: %+v", inv)
	}
	if pluginErr == nil {
		t.Fatalf("certain invalid package published plugins: %+v", plugins)
	}
	if !reflect.DeepEqual(inv, Inventory{}) {
		t.Fatalf("certain parse refusal leaked report: %+v", inv)
	}
	if plugins != nil {
		t.Fatalf("certain parse refusal leaked plugins: %+v", plugins)
	}
	for _, err := range []error{reportErr, pluginErr} {
		if !strings.Contains(err.Error(), "parse committed source "+externalInvalidPotentialDir+"/broken.go") {
			t.Fatalf("certain invalid syntax cause absent: %v", err)
		}
	}
}

// TestCommittedExternalBuildExcludedMalformedPotentialFile keeps a valid
// package when its malformed sibling is outside the declared target selection.
func TestCommittedExternalBuildExcludedMalformedPotentialFile(t *testing.T) {
	fixture := externalPotentialFixture(t)
	fixture.goFile(t, externalInvalidPotentialDir+"/source.go", "package parcel\nconst Label = \"ordinary\"\n")
	fixture.goFile(t, externalInvalidPotentialDir+"/broken_windows.go", "//go:build windows\n\n"+externalInvalidPotentialSource)
	commitInventoryFixture(t, fixture.root)
	snapshot := externalEffectsPositive(t, fixture)
	private, err := newExternalSource(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if _, included := private.snapshot.packages[externalInvalidPotentialDir]; !included {
		t.Fatal("build-excluded malformed file removed its valid package")
	}
}

// TestCommittedExternalValidPotentialInstaller remains an effect refusal at an
// arbitrary path; the parser certificate cannot become a path exemption.
func TestCommittedExternalValidPotentialInstaller(t *testing.T) {
	fixture := externalPotentialFixture(t)
	fixture.goFile(t, externalInvalidPotentialDir+"/source.go", externalPotentialInstaller("parcel", ""))
	commitInventoryFixture(t, fixture.root)
	externalEffectsRefusal(t, fixture, "registration")
}

// TestCommittedExternalFutureLanguageSyntaxRefuses cannot certify malformed
// syntax under a language newer than the analyzer's parser authority.
func TestCommittedExternalFutureLanguageSyntaxRefuses(t *testing.T) {
	for _, declaration := range []string{"module", "vendor"} {
		t.Run(declaration, func(t *testing.T) {
			fixture := externalPotentialFixture(t)
			directory := externalInvalidPotentialDir
			if declaration == "module" {
				write(t, fixture.root, "go.mod", "module github.com/ze-software/ze\n\ngo 1.999\n")
			}
			if declaration == "vendor" {
				directory = "vendor/example.invalid/parcel"
				write(t, fixture.root, "go.mod", "module github.com/ze-software/ze\n\ngo 1.26\n\nrequire example.invalid/parcel v1.0.0\n")
				write(t, fixture.root, "vendor/modules.txt", "# example.invalid/parcel v1.0.0\n## explicit; go 1.999\nexample.invalid/parcel\n")
			}
			fixture.goFile(t, directory+"/broken.go", externalInvalidPotentialSource)
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "language")
		})
	}
}

// TestCommittedExternalInvalidPotentialModuleIdentity preserves the module
// boundary refusal even when its importing package also has invalid syntax.
func TestCommittedExternalInvalidPotentialModuleIdentity(t *testing.T) {
	fixture := externalPotentialFixture(t)
	const nested = "examples/independent"
	write(t, fixture.root, nested+"/go.mod", "module example.invalid/independent\n\ngo 1.26\n")
	fixture.goFile(t, nested+"/source.go", "package independent\n")
	fixture.goFile(t, externalInvalidPotentialDir+"/broken.go", "package parcel\nimport _ \"github.com/ze-software/ze/"+nested+"\"\nfunc broken( {\n")
	commitInventoryFixture(t, fixture.root)
	externalEffectsRefusal(t, fixture, "nested module")
}

func externalPotentialFixture(t *testing.T) *asmInventoryFixture {
	t.Helper()
	fixture := externalEffectsFixture(t, "plugin")
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource("", "parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"))
	commitInventoryFixture(t, fixture.root)
	externalEffectsPositive(t, fixture)
	return fixture
}

func externalPotentialInstaller(packageName, extraImport string) string {
	return "package " + packageName + "\nimport (helper \"github.com/ze-software/ze/" + callableOwnerDir + "\"; store \"github.com/ze-software/ze/" + externalNeutralDir + "\"; " + extraImport + ")\nfunc install() { store.Callback = func() { helper.Install(0) } }\n"
}
