// VALIDATES: AC-4 through the commit seam's judge: a commit that adds a
// problem site with no doctor code is refused, naming the file and line and
// the cover; a commit that adds a covered site is admitted; an uncovered site
// the parent already held is not refused (report lists it); an empty change
// set is admitted without reading either side; and a thin scan is an error.
// PREVENTS: `./le commit create` landing a Warn line no doctor check answers
// for, and a commit refused for the uncovered sites the tree already holds.
package archdoctorcoverage

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/commit/changecheck"
)

// fixtureAnchor is the fixture line a commit side adds a new line after.
const fixtureAnchor = "\tlog.Warn(\"fixture: no code\", \"peer\", peer)\n"

// fixtureSide reads the fixture tree from disk, with old replaced by
// replacement in path.
func fixtureSide(t *testing.T, path, old, replacement string) changecheck.Side {
	t.Helper()
	return func(read string) (string, bool, error) {
		content, err := os.ReadFile(filepath.Join(fixtureTree, filepath.FromSlash(read)))
		if err != nil {
			return "", false, err
		}
		text := string(content)
		if read == path {
			if !strings.Contains(text, old) {
				t.Fatalf("fixture %s holds no %q", read, old)
			}
			text = strings.Replace(text, old, replacement, 1)
		}
		return text, true, nil
	}
}

// addingLine is a commit side whose sites.go holds line after the anchor,
// and the line number line lands on.
func addingLine(t *testing.T, line string) (changecheck.Side, int) {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(fixtureTree, filepath.FromSlash(fixtureSites)))
	if err != nil {
		t.Fatal(err)
	}
	before, _, found := bytes.Cut(content, []byte(fixtureAnchor))
	if !found {
		t.Fatalf("fixture %s holds no %q", fixtureSites, fixtureAnchor)
	}
	lineNumber := bytes.Count(before, []byte("\n")) + 2
	return fixtureSide(t, fixtureSites, fixtureAnchor, fixtureAnchor+"\t"+line+"\n"), lineNumber
}

// TestDoctorCoverageCommitRefusesAddedUncoveredSite holds AC-4's commit
// judge: a commit adding a Warn line with no code to an internal/plugins file
// is refused naming file:line and the cover, and the refusal names none of
// the uncovered sites the parent already held.
func TestDoctorCoverageCommitRefusesAddedUncoveredSite(t *testing.T) {
	after, line := addingLine(t, `log.Warn("fixture: added uncovered", "peer", peer)`)
	change := changecheck.Change{
		Root:   fixtureTree,
		Paths:  []string{fixtureSites},
		Before: fixtureSide(t, "", "", ""),
		After:  after,
	}
	text, err := judgeChangeAt(change, fixtureCovered(), 1)
	if err != nil {
		t.Fatalf("judgeChangeAt: %v", err)
	}
	where := fixtureSites + ":" + strconv.Itoa(line)
	for _, want := range []string{where, "fixture: added uncovered", "no code attribute", "\nFix:", `"code"`, "report.RaiseError", "./le arch doctor-coverage report"} {
		if !strings.Contains(text, want) {
			t.Errorf("refusal does not name %q:\n%s", want, text)
		}
	}
	for _, held := range []string{"fixture: no code\"", "fixture: dynamic code", "fixture: orphan code", "fixture: raise uncovered", "fixture: package logger"} {
		if strings.Contains(text, held) {
			t.Errorf("refusal names %q, a site the parent already held:\n%s", held, text)
		}
	}
}

// TestDoctorCoverageCommitAdmitsAddedCoveredSite holds that a commit adding a
// Warn line whose code a registered check declares is admitted.
func TestDoctorCoverageCommitAdmitsAddedCoveredSite(t *testing.T) {
	after, _ := addingLine(t, `log.Warn("fixture: added covered", "code", diagnostic.CodeDoctorFixtureCovered)`)
	change := changecheck.Change{
		Root:   fixtureTree,
		Paths:  []string{fixtureSites},
		Before: fixtureSide(t, "", "", ""),
		After:  after,
	}
	text, err := judgeChangeAt(change, fixtureCovered(), 1)
	if err != nil {
		t.Fatalf("judgeChangeAt: %v", err)
	}
	if text != "" {
		t.Errorf("a commit adding a covered site was refused:\n%s", text)
	}
}

// TestDoctorCoverageCommitLeavesHeldSitesToReport holds that a commit
// carrying a file whose uncovered sites the parent already held is admitted,
// while report still lists those sites: the tree's sites are the audit's.
// A commit that strips the code from a covered site adds an uncovered one.
func TestDoctorCoverageCommitLeavesHeldSitesToReport(t *testing.T) {
	unchanged := fixtureSide(t, "", "", "")
	change := changecheck.Change{Root: fixtureTree, Paths: []string{fixtureSites}, Before: unchanged, After: unchanged}
	text, err := judgeChangeAt(change, fixtureCovered(), 1)
	if err != nil {
		t.Fatalf("judgeChangeAt: %v", err)
	}
	if text != "" {
		t.Errorf("a commit adding no site was refused for the sites its parent held:\n%s", text)
	}
	listed := treeReport(scanFixture(t)).Text()
	if !strings.Contains(listed, `"fixture: no code"`) {
		t.Errorf("report does not list the held uncovered site:\n%s", listed)
	}

	stripped := fixtureSide(t, fixtureSites,
		`log.Warn("fixture: covered literal", "code", "doctor-fixture-covered")`,
		`log.Warn("fixture: covered literal")`)
	stripping := changecheck.Change{Root: fixtureTree, Paths: []string{fixtureSites}, Before: unchanged, After: stripped}
	text, err = judgeChangeAt(stripping, fixtureCovered(), 1)
	if err != nil {
		t.Fatalf("judgeChangeAt: %v", err)
	}
	if !strings.Contains(text, "fixture: covered literal") {
		t.Errorf("a commit removing a site's code was admitted:\n%s", text)
	}
}

// TestDoctorCoverageCommitAdmitsEmptyChange holds that a change set carrying
// no shipped file under the site roots is admitted without reading a side,
// and that a carried file below a thin scan is an error, never an admit.
func TestDoctorCoverageCommitAdmitsEmptyChange(t *testing.T) {
	refuseRead := func(path string) (string, bool, error) {
		t.Errorf("an empty change set read %s", path)
		return "", false, nil
	}
	for _, paths := range [][]string{nil, {"docs/guide/x.md", "internal/plugins/fixture/sites_test.go"}} {
		change := changecheck.Change{Root: fixtureTree, Paths: paths, Before: refuseRead, After: refuseRead}
		text, err := judgeChangeAt(change, fixtureCovered(), 1)
		if err != nil {
			t.Fatalf("judgeChangeAt %v: %v", paths, err)
		}
		if text != "" {
			t.Errorf("change set %v was refused:\n%s", paths, text)
		}
	}

	unchanged := fixtureSide(t, "", "", "")
	clean := changecheck.Change{Root: fixtureTree, Paths: []string{fixtureSites}, Before: unchanged, After: unchanged}
	if _, err := judgeChangeAt(clean, fixtureCovered(), 1000); err == nil {
		t.Error("a commit judged over a thin scan was admitted")
	}
}
