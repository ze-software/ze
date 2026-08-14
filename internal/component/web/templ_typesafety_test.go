// Related: testdata/templtypesafety/view.go -- the view model this test renames
// Related: testdata/templtypesafety/page.templ -- the markup that reads it

package web

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// templFixtureDir holds the package TestTemplComponentTypeSafety loads.
const templFixtureDir = "testdata/templtypesafety"

// templFixtureField is the view-model field the test renames. The rename must
// break the build, because page.templ reads this name.
const templFixtureField = "Title string"

// TestTemplComponentTypeSafety proves that a templ component reads its view
// model at compile time.
//
// VALIDATES: AC-1 of plan/spec-web-templ-migration.md. A view-model field
// renamed without a matching markup edit fails the build.
// PREVENTS: the silent blank panel html/template gives today. RenderFragment
// and RenderField discard the execution error and return "". A renamed field
// therefore reaches the operator as an empty region, and the log as nothing.
//
// THE MECHANISM, chosen in phase 1 because phase 4 needs it. A Go file that
// must not compile takes its whole package down, and a package that does not
// build reports no test result at all. So the broken form is never written to
// disk. The fixture sits under testdata, where no build and no lint run reaches
// it, and the rename is applied as a go/packages overlay. Nothing on disk
// changes, and two sessions can run this test at the same time.
func TestTemplComponentTypeSafety(t *testing.T) {
	dir, err := filepath.Abs(templFixtureDir)
	if err != nil {
		t.Fatalf("resolve %s: %v", templFixtureDir, err)
	}
	viewFile := filepath.Join(dir, "view.go")
	source, err := os.ReadFile(viewFile)
	if err != nil {
		t.Fatalf("read %s: %v", viewFile, err)
	}
	if !strings.Contains(string(source), templFixtureField) {
		t.Fatalf("%s no longer declares %q, so this test cannot rename it", viewFile, templFixtureField)
	}

	pkg := loadTemplFixture(t, dir, nil)
	if len(pkg.Errors) != 0 {
		t.Fatalf("the fixture must build as written, got %v", pkg.Errors)
	}
	if pkg.Types == nil || pkg.Types.Scope().Lookup("page") == nil {
		t.Fatal("the fixture built without a page component, so a rename could not break it")
	}

	renamed := strings.Replace(string(source), templFixtureField, "Heading string", 1)
	broken := loadTemplFixture(t, dir, map[string][]byte{viewFile: []byte(renamed)})
	if len(broken.Errors) == 0 {
		t.Fatal("renaming the view-model field left the build clean, so templ checked nothing")
	}
	for _, e := range broken.Errors {
		if strings.Contains(e.Msg, "Title") {
			return
		}
	}
	t.Fatalf("the build failed for some other reason than the renamed field: %v", broken.Errors)
}

// loadTemplFixture type-checks the fixture package and returns it. overlay maps
// an absolute file path to the content to use in place of the file on disk.
func loadTemplFixture(t *testing.T, dir string, overlay map[string][]byte) *packages.Package {
	t.Helper()
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedTypes |
			packages.NeedSyntax | packages.NeedTypesInfo,
		Dir:     dir,
		Overlay: overlay,
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		t.Fatalf("load %s: %v", dir, err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("load %s returned %d packages, want 1", dir, len(pkgs))
	}
	return pkgs[0]
}
