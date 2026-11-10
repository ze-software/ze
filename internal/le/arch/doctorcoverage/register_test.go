// VALIDATES: AC-4 through `./le commit create`: the package registers its
// check into the changecheck seam the commit flow runs, and that registered
// check, over the real checkout with the real covered set, refuses a commit
// adding a Warn line with no doctor code in an internal/plugins package while
// admitting a covered line the same commit adds.
// PREVENTS: a doctor-coverage gate that exists as a command no commit runs,
// and a seam judge that only works on the fixture tree.
package archdoctorcoverage

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/commit/changecheck"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// commitFixtureFile is the file the commit fixture adds, in an existing
// plugin package. It is never written to disk: only the After side holds it.
const commitFixtureFile = "internal/plugins/ntp/zz_doctor_coverage_fixture.go"

// registeredCheck answers the doctor-coverage check the seam holds.
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

// TestDoctorChangeCheckRegistered holds AC-4 through the seam: the package
// registers its check into the changecheck seam `./le commit create` runs.
func TestDoctorChangeCheckRegistered(t *testing.T) {
	check := registeredCheck(t)
	if check.Judge == nil {
		t.Fatal("the registered check has no judge")
	}
}

// TestCommitCreateRefusesUncoveredProblemSite runs the registered check the
// way the commit flow does (changecheck.Run) over the real checkout: a commit
// adding a file to internal/plugins/ntp with one Warn line naming no code and
// one naming a code a doctor check in the checkout declares is refused for the
// first, at its file and line, and not for the second.
func TestCommitCreateRefusesUncoveredProblemSite(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("checkout root: %v", err)
	}
	index, err := indexTree(root, nil)
	if err != nil {
		t.Fatalf("indexTree: %v", err)
	}
	covered := index.coveredCodes()
	codes := make([]string, 0, len(covered))
	for code := range covered {
		codes = append(codes, code)
	}
	if len(codes) == 0 {
		t.Fatal("the checkout declared no doctor code")
	}
	slices.Sort(codes)

	added := "package ntp\n\n" +
		"import \"log/slog\"\n\n" +
		"func doctorCoverageFixture(log *slog.Logger) {\n" +
		"\tlog.Warn(\"fixture: commit covered\", \"code\", \"" + codes[0] + "\")\n" +
		"\tlog.Warn(\"fixture: commit uncovered\")\n" +
		"}\n"
	// The commit carries only the added file, so each side is asked for it
	// alone; any other path read is a judge reading what it does not carry.
	after := func(path string) (string, bool, error) {
		if path != commitFixtureFile {
			t.Errorf("the judge read %s, which the commit does not carry", path)
		}
		return added, true, nil
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
	for _, want := range []string{commitFixtureFile + ":7", "fixture: commit uncovered", "no code attribute", "\nFix:"} {
		if !strings.Contains(text, want) {
			t.Errorf("refusal does not name %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "fixture: commit covered") {
		t.Errorf("refusal names the covered site:\n%s", text)
	}
}
