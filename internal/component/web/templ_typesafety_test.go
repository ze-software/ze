// Related: render.go -- LayoutData, the view model this test renames
// Related: page_layout.templ -- the ported markup that reads v.Title

package web

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// templViewFile declares the view model the test renames a field on.
const templViewFile = "render.go"

// templMarkupFile is the ported component that reads that field.
const templMarkupFile = "page_layout.templ"

// templMarkupRead is how the markup reads it. The test refuses to run when the
// markup stops reading the field, because the rename would then prove nothing.
const templMarkupRead = "{ v.Title }"

// templViewField matches the declaration of LayoutData.Title. gofmt aligns the
// type, so the pattern reads the padding rather than pinning a column count.
var templViewField = regexp.MustCompile(`(?m)^\tTitle(\s+)string$`)

// TestTemplComponentTypeSafety proves that a PORTED templ component reads its
// view model at compile time.
//
// VALIDATES: AC-1 of spec-web-templ-migration. A view-model field
// renamed without a matching markup edit fails the build.
// PREVENTS: the silent blank panel html/template gave. RenderFragment and
// RenderField discarded the execution error and returned "", so a renamed field
// reached the operator as an empty region and the log as nothing.
//
// THE ASSERTION IS THE GENERATED FILE, not the count of errors. A rename that
// broke only hand-written Go would prove nothing about the markup: the point is
// that page_layout.templ's read of v.Title is compiled. So the test requires an
// error positioned inside a *_templ.go, which no engine but templ produces.
//
// THE MECHANISM. A Go file that must not compile takes its whole package down,
// and a package that does not build reports no test result at all. So the
// broken form is never written to disk: the rename is a go/packages overlay.
// Nothing on disk changes and two sessions can run this test at once.
//
// The clean load runs FIRST and must report no error. Without it a package that
// was already broken would satisfy the second load for the wrong reason, and
// the test would pass while checking nothing.
func TestTemplComponentTypeSafety(t *testing.T) {
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve the package directory: %v", err)
	}
	markup, err := os.ReadFile(filepath.Join(dir, templMarkupFile))
	if err != nil {
		t.Fatalf("read %s: %v", templMarkupFile, err)
	}
	if !strings.Contains(string(markup), templMarkupRead) {
		t.Fatalf("%s no longer reads %s, so renaming the field would prove nothing", templMarkupFile, templMarkupRead)
	}
	viewFile := filepath.Join(dir, templViewFile)
	source, err := os.ReadFile(viewFile)
	if err != nil {
		t.Fatalf("read %s: %v", viewFile, err)
	}
	if got := len(templViewField.FindAllString(string(source), -1)); got != 1 {
		t.Fatalf("%s declares %d fields matching %v, want exactly 1: this test cannot name the field to rename", templViewFile, got, templViewField)
	}

	clean := loadWebPackage(t, dir, nil)
	if len(clean.Errors) != 0 {
		t.Fatalf("the package must build as written, got %v", clean.Errors)
	}
	if clean.Types == nil || clean.Types.Scope().Lookup("pageLayout") == nil {
		t.Fatal("the loaded package declares no pageLayout component, so a rename could not break it")
	}

	renamed := templViewField.ReplaceAllString(string(source), "\tHeading${1}string")
	broken := loadWebPackage(t, dir, map[string][]byte{viewFile: []byte(renamed)})
	if len(broken.Errors) == 0 {
		t.Fatal("renaming LayoutData.Title left the build clean, so the ported component checked nothing")
	}
	for _, e := range broken.Errors {
		if strings.Contains(e.Pos, "_templ.go") && strings.Contains(e.Msg, "Title") {
			return
		}
	}
	t.Fatalf("no generated component failed on the renamed field, so the markup does not read it at compile time: %v", broken.Errors)
}

// loadWebPackage type-checks the package in dir and returns it. overlay maps an
// absolute file path to the content to use in place of the file on disk.
func loadWebPackage(t *testing.T, dir string, overlay map[string][]byte) *packages.Package {
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
