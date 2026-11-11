// Design: docs/architecture/plugin/feature-gates.md -- the committed source selection refuses what it cannot select.
package repoinventory

import (
	"reflect"
	"strings"
	"testing"
)

const invalidPackageDir = "arbitrary/place/parcel"
const invalidPackageSource = "package parcel\nfunc broken( {\n"
const absentPackageTarget = "unrelated/missing/target"

// TestCommittedInvalidCertainPackage preserves the parse refusal when an
// import the initialization graph selects names a syntax-invalid package.
// Method: cmd/ze imports the broken package; the report and the declared
// list both refuse, leak nothing, and name the broken file.
func TestCommittedInvalidCertainPackage(t *testing.T) {
	fixture := statCallerFixture(t)
	fixture.goFile(t, invalidPackageDir+"/broken.go", invalidPackageSource)
	fixture.goFile(t, "cmd/ze/broken_import.go", "package main\nimport _ \"github.com/ze-software/ze/"+invalidPackageDir+"\"\n")
	commitInventoryFixture(t, fixture.root)
	inv, reportErr := Collect(fixture.root)
	plugins, pluginErr := DeclaredPlugins(fixture.root)
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
		if !strings.Contains(err.Error(), "parse committed source "+invalidPackageDir+"/broken.go") {
			t.Fatalf("certain invalid syntax cause absent: %v", err)
		}
	}
}

// TestCommittedAbsentCertainDependency preserves the source refusal when
// initialization imports a package the tree holds no Go source for. Method:
// cmd/ze imports an absent path; the report and the declared list both
// refuse, leak nothing, and name the import.
func TestCommittedAbsentCertainDependency(t *testing.T) {
	fixture := statCallerFixture(t)
	fixture.goFile(t, "cmd/ze/absent_import.go", "package main\nimport _ \"github.com/ze-software/ze/"+absentPackageTarget+"\"\n")
	commitInventoryFixture(t, fixture.root)
	inv, reportErr := Collect(fixture.root)
	plugins, pluginErr := DeclaredPlugins(fixture.root)
	if reportErr == nil {
		t.Fatalf("certain absent package published report: %+v", inv)
	}
	if pluginErr == nil {
		t.Fatalf("certain absent package published plugins: %+v", plugins)
	}
	if !reflect.DeepEqual(inv, Inventory{}) {
		t.Fatalf("certain absence refusal leaked report: %+v", inv)
	}
	if plugins != nil {
		t.Fatalf("certain absence refusal leaked plugins: %+v", plugins)
	}
	for _, err := range []error{reportErr, pluginErr} {
		if !strings.Contains(err.Error(), "imports "+absentPackageTarget+" but selects no Go source") {
			t.Fatalf("certain absent source cause absent: %v", err)
		}
	}
}

// TestCommittedVendorDirectoryPackage selects an ordinary Ze package whose
// directory happens to be named vendor, through its module-prefixed import.
// Method: cmd/ze imports internal/example/vendor; the report and the declared
// list keep the exact ordinary oracle.
func TestCommittedVendorDirectoryPackage(t *testing.T) {
	fixture := importingFixture(t, "plugin")
	fixture.goFile(t, callableOwnerCaller, importingSource("",
		"parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"))
	commitInventoryFixture(t, fixture.root)
	importingPositive(t, fixture)

	const packageDirectory = "internal/example/vendor"
	fixture.goFile(t, packageDirectory+"/tool.go",
		"package vendortool\nfunc init() {}\nfunc Value() int { return 1 }\n")
	const mainFile = "cmd/ze/main.go"
	fixture.goFile(t, mainFile, strings.Replace(fixture.goSources[mainFile],
		"func main()", "import _ \"github.com/ze-software/ze/"+packageDirectory+"\"\nfunc main()", 1))
	commitInventoryFixture(t, fixture.root)
	importingPositive(t, fixture)
}

// statCallerFixture is importingFixture whose caller makes a standard library
// call, proven to keep the ordinary report first.
func statCallerFixture(t *testing.T) *ordinaryFixture {
	t.Helper()
	fixture := importingFixture(t, "plugin")
	fixture.goFile(t, callableOwnerCaller, importingSource("", "parcel, fault := remote.Stat(\".\"); _ = parcel; _ = fault"))
	commitInventoryFixture(t, fixture.root)
	importingPositive(t, fixture)
	return fixture
}
