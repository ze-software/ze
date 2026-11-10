// VALIDATES: AC-7 through `./le commit create`: the package registers its
// check into the changecheck seam the commit flow runs (changeCheckRefusal in
// internal/le/commit reads changecheck.Registered), and that registered check,
// over the real checkout with the real floors, refuses a commit adding a
// plain http.Error to the web package, naming its file and line, its status
// and the fix, while admitting a typed refusal the same commit adds.
// PREVENTS: an error-inventory gate that exists as a command no commit runs,
// and a seam judge that only works on the fixture tree.
//
// The test sits here, as operator-error's and doctor-coverage's do, because a
// test inside internal/le/commit cannot link this package: it imports the
// commit seam, so the import would be a cycle.
package archerrorinventory

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/commit/changecheck"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// commitFixtureFile is the file the commit fixture adds, in the real web
// package. It is never written to disk: only the After side holds it.
const commitFixtureFile = "internal/component/web/zz_error_inventory_fixture.go"

// commitFixtureSource is commitFixtureFile's content: a plain web refusal at
// line 11, and a typed one at line 12 that the check admits.
const commitFixtureSource = "package web\n\n" +
	"import (\n" +
	"\t\"net/http\"\n\n" +
	"\t\"github.com/ze-software/ze/internal/core/opererr\"\n" +
	")\n\n" +
	"// errorInventoryFixture is the commit fixture's two web refusals.\n" +
	"func errorInventoryFixture(w http.ResponseWriter) {\n" +
	"\thttp.Error(w, \"fixture: commit plain\", 401)\n" +
	"\thttp.Error(w, opererr.New(\"fixture: commit typed\", \"the session expired\", \"ze login\").Error(), 401)\n" +
	"}\n"

// registeredCheck answers the error-inventory check the seam holds.
func registeredCheck(t *testing.T) changecheck.Check {
	t.Helper()
	for _, check := range changecheck.Registered() {
		if check.Name == area+" check" {
			return check
		}
	}
	t.Fatalf("%s check is not registered in the commit seam", area)
	return changecheck.Check{}
}

// TestErrorInventoryChangeCheckRegistered holds AC-7's route: the package
// registers its check into the changecheck seam `./le commit create` runs.
func TestErrorInventoryChangeCheckRegistered(t *testing.T) {
	check := registeredCheck(t)
	if check.Judge == nil {
		t.Fatal("the registered check has no judge")
	}
}

// TestCommitCreateRefusesAddedPlainWebSite runs the registered check the way
// the commit flow does (changecheck.Run) over the real checkout: a commit
// adding a file to internal/component/web with one plain http.Error and one
// handed an operator error is refused for the first, at its file and line,
// and not for the second.
func TestCommitCreateRefusesAddedPlainWebSite(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("checkout root: %v", err)
	}
	// The commit carries only the added file, so each side is asked for it
	// alone; any other path read is a judge reading what it does not carry.
	after := func(path string) (string, bool, error) {
		if path != commitFixtureFile {
			t.Errorf("the judge read %s, which the commit does not carry", path)
		}
		return commitFixtureSource, true, nil
	}
	before := func(path string) (string, bool, error) {
		if path != commitFixtureFile {
			t.Errorf("the judge read %s, which the commit does not carry", path)
		}
		return "", false, nil
	}
	change := changecheck.Change{Root: root, Paths: []string{commitFixtureFile}, Before: before, After: after}

	refusals, err := changecheck.Run(change, []changecheck.Check{registeredCheck(t)})
	if err != nil {
		t.Fatalf("the check could not judge: %v", err)
	}
	if len(refusals) != 1 {
		t.Fatalf("refusals %d, want 1: %+v", len(refusals), refusals)
	}
	text := refusals[0].Text
	for _, want := range []string{commitFixtureFile + ":11", "web", "plain", `"fixture: commit plain"`, "\nFix:", "opererr.New"} {
		if !strings.Contains(text, want) {
			t.Errorf("refusal does not name %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, commitFixtureFile+":12") {
		t.Errorf("refusal names the typed site:\n%s", text)
	}
}
