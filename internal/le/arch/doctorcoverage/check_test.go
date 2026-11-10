// VALIDATES: the check form of le arch doctor-coverage (AC-4, AC-12): a
// problem site on a changed line with no doctor code is refused, naming its
// file and line and the cover; a covered site on a changed line passes; a
// marker comment is no cover; a site on an unchanged line is never refused;
// and the covered codes are read from both doctor registries (A-2).
// PREVENTS: a new Warn or Error line landing with no doctor check that
// answers for it, and a gate that refuses the sites the tree already holds.
package archdoctorcoverage

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// writeTreeFile writes one file under a temporary tree.
func writeTreeFile(t *testing.T, tree, rel, text string) {
	t.Helper()
	full := filepath.Join(tree, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// changedAt is a change set that changed exactly the lines of the given sites.
func changedAt(sites ...Site) repochanged.ChangedLines {
	changed := repochanged.ChangedLines{}
	for i := range sites {
		site := &sites[i]
		changed[site.File] = append(changed[site.File], repochanged.LineSpan{From: site.Line, To: site.EndLine})
	}
	return changed
}

// TestDoctorCoverageRefusesAddedUncoveredSite is AC-4's check form: a Warn
// line in internal/plugins with no doctor code, on a changed line, is refused
// with its file and line, and the refusal prints the cover. The same line
// left unchanged is not refused.
func TestDoctorCoverageRefusesAddedUncoveredSite(t *testing.T) {
	inventory := scanFixture(t)
	uncovered := siteWith(t, inventory.Sites, "fixture: no code")
	untouched := siteWith(t, inventory.Sites, "fixture: dynamic code")

	report := checkLines(inventory.Sites, changedAt(uncovered))
	if report.exitCode() != 1 {
		t.Fatalf("exit %d, want 1: %+v", report.exitCode(), report)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("findings %d, want 1: %+v", len(report.Findings), report.Findings)
	}
	if report.Findings[0].Message != uncovered.Message {
		t.Errorf("refused %q, want %q", report.Findings[0].Message, uncovered.Message)
	}
	text := report.Text()
	where := fixtureSites + ":" + strconv.Itoa(uncovered.Line)
	for _, want := range []string{where, "no code", `"code"`, "report.RaiseError", "./le arch doctor-coverage report"} {
		if !strings.Contains(text, want) {
			t.Errorf("refusal does not name %q:\n%s", want, text)
		}
	}
	// The cover sits on lines of its own, after the label.
	if !strings.Contains(text, "\nFix:") {
		t.Errorf("refusal has no fix line:\n%s", text)
	}
	for _, finding := range report.Findings {
		if finding.Message == untouched.Message {
			t.Errorf("a site on an unchanged line was refused: %+v", finding)
		}
	}

	// Each refusable status is refused on a changed line.
	for _, message := range []string{"fixture: dynamic code", "fixture: orphan code", "fixture: raise uncovered", "fixture: package logger"} {
		site := siteWith(t, inventory.Sites, message)
		refused := checkLines(inventory.Sites, changedAt(site))
		if len(refused.Findings) != 1 {
			t.Errorf("%q (%s) on a changed line: %d findings, want 1", message, site.Status, len(refused.Findings))
		}
	}
}

// TestDoctorCoverageAcceptsCoveredSite proves a site whose code a registered
// check declares passes on a changed line, and a health row (which carries no
// code field) is not refused by the check.
func TestDoctorCoverageAcceptsCoveredSite(t *testing.T) {
	inventory := scanFixture(t)
	var accepted []Site
	for _, site := range inventory.Sites {
		if site.Status == StatusCovered {
			accepted = append(accepted, site)
		}
	}
	if len(accepted) < 6 {
		t.Fatalf("covered sites %d, want at least 6", len(accepted))
	}
	accepted = append(accepted, siteWith(t, inventory.Sites, "fixture-row"))
	report := checkLines(inventory.Sites, changedAt(accepted...))
	if report.exitCode() != 0 {
		t.Fatalf("exit %d, want 0: %+v", report.exitCode(), report.Findings)
	}
	if report.Judged != len(accepted) {
		t.Errorf("judged %d, want %d", report.Judged, len(accepted))
	}
	if !strings.Contains(report.Text(), "doctor-coverage: OK") {
		t.Errorf("clean check text:\n%s", report.Text())
	}
}

// TestDoctorCoverageRefusesMarkedSite is AC-12: a Warn or Error line whose
// comment claims a transient or exempt class is refused like any line with no
// doctor code. No marker is accepted.
func TestDoctorCoverageRefusesMarkedSite(t *testing.T) {
	inventory := scanFixture(t)
	transient := siteWith(t, inventory.Sites, "fixture: marked transient")
	exempt := siteWith(t, inventory.Sites, "fixture: marked exempt")
	report := checkLines(inventory.Sites, repochanged.ChangedLines{fixtureMarked: {{From: 1, To: 1000}}})
	if len(report.Findings) != 2 {
		t.Fatalf("findings %d, want 2: %+v", len(report.Findings), report.Findings)
	}
	for _, site := range []Site{transient, exempt} {
		if !slices.ContainsFunc(report.Findings, func(finding Site) bool { return finding.Line == site.Line }) {
			t.Errorf("marked site %s:%d was not refused", site.File, site.Line)
		}
	}
}

// TestDoctorCoverageReadsPluginCodes is A-2's validation: with the product
// composition root linked, the covered set holds every code a plugin's
// doctor check declares and every code an in-tree doctor check declares, and
// names the check that declares it.
func TestDoctorCoverageReadsPluginCodes(t *testing.T) {
	covered := CoveredCodes()
	plugins := registry.PluginDoctorChecks()
	if len(plugins) == 0 {
		t.Fatal("the linked product declared no plugin doctor check, so the comparison judged nothing")
	}
	for _, check := range plugins {
		name := pluginCheckName(check.PluginName, check.Name)
		for _, code := range check.Codes {
			if !slices.Contains(covered[code], name) {
				t.Errorf("plugin check %s declares %s; covered names %v", name, code, covered[code])
			}
		}
	}
	inTree := diagnostic.DoctorChecks()
	if len(inTree) == 0 {
		t.Fatal("the linked product registered no in-tree doctor check")
	}
	for _, check := range inTree {
		for _, code := range check.Codes {
			if !slices.Contains(covered[code], check.Name) {
				t.Errorf("in-tree check %s declares %s; covered names %v", check.Name, code, covered[code])
			}
		}
	}
	if err := checkCodesFloor(covered, coveredCodesFloor); err != nil {
		t.Error(err)
	}
	t.Logf("A-2: %d plugin doctor checks, %d in-tree doctor checks, %d covered codes", len(plugins), len(inTree), len(covered))
}

// TestDoctorCoverageThinCoveredSetRefused proves a covered set below the
// floor is an error, so a registry the scan no longer reaches cannot answer
// every site as uncovered or, worse, as judged.
func TestDoctorCoverageThinCoveredSetRefused(t *testing.T) {
	if err := checkCodesFloor(fixtureCovered(), 2); err == nil {
		t.Fatal("a covered set of one code passed a floor of two")
	}
}
