// VALIDATES: spec assumption A-1, that a problem site names its doctor code
// statically: the scan finds every Warn and Error log line, report-bus raise
// and health row in the product roots, resolves a code given as a literal, a
// package constant or another package's exported constant, and tells a
// missing code, a code built at run time and a code no check declares apart.
// PREVENTS: a problem site the scan does not see, and a code the scan cannot
// read being counted as covered.
package archdoctorcoverage

import (
	"strings"
	"testing"
)

// fixtureTree holds one call of every site shape, a test file and a tooling
// package the scan skips.
const fixtureTree = "testdata/tree"

const (
	fixtureSites  = "internal/plugins/fixture/sites.go"
	fixtureMarked = "internal/plugins/fixture/marked.go"
	fixtureOther  = "internal/component/other/other.go"
)

// fixtureCodesFloor is the covered-set floor a fixture scan is held to: the
// fixture declares a handful of codes, not the tree's 20.
const fixtureCodesFloor = 1

// fixtureDoors maps the fixture's two owner trees to two door pages.
func fixtureDoors(file string) []string {
	if strings.HasPrefix(file, "internal/plugins/") {
		return []string{"ai/doors/system-plugins.md"}
	}
	if strings.HasPrefix(file, "internal/component/") {
		return []string{"ai/doors/fixture-component.md"}
	}
	return nil
}

// scanFixture scans the fixture tree, whose own doctor check declarations
// give the covered set.
func scanFixture(t *testing.T) Inventory {
	t.Helper()
	inventory, err := Scan(fixtureTree, fixtureCodesFloor, fixtureDoors)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	return inventory
}

// siteWith answers the one site whose message is message, failing the test
// when there is not exactly one.
func siteWith(t *testing.T, sites []Site, message string) Site {
	t.Helper()
	var matched []Site
	for i := range sites {
		site := &sites[i]
		if site.Message == message {
			matched = append(matched, *site)
		}
	}
	if len(matched) != 1 {
		t.Fatalf("sites with message %q = %d, want 1: %+v", message, len(matched), sites)
	}
	return matched[0]
}

// TestDoctorCoverageResolvesStaticCodes is A-1's validation: each fixture
// call answers the kind, status and code its shape implies.
func TestDoctorCoverageResolvesStaticCodes(t *testing.T) {
	inventory := scanFixture(t)
	cases := []struct {
		message string
		file    string
		kind    Kind
		status  Status
		code    string
	}{
		{"fixture: covered constant", fixtureSites, KindLogWarn, StatusCovered, "doctor-fixture-covered"},
		{"fixture: covered attr", fixtureSites, KindLogError, StatusCovered, "doctor-fixture-covered"},
		{"fixture: covered literal", fixtureSites, KindLogWarn, StatusCovered, "doctor-fixture-covered"},
		{"fixture: covered context", fixtureSites, KindLogWarn, StatusCovered, "doctor-fixture-covered"},
		{"fixture: covered level", fixtureSites, KindLogError, StatusCovered, "doctor-fixture-covered"},
		{"refusal.Problem()", fixtureSites, KindLogWarn, StatusCovered, "doctor-fixture-covered"},
		{"fixture: no code", fixtureSites, KindLogWarn, StatusNoCode, ""},
		{"fixture: package logger", fixtureSites, KindLogError, StatusNoCode, ""},
		{"fixture: constant message", fixtureSites, KindLogWarn, StatusNoCode, ""},
		{"fixture: dynamic code", fixtureSites, KindLogWarn, StatusCodeDynamic, ""},
		{"fixture: orphan code", fixtureSites, KindLogError, StatusCodeNotDoctor, "doctor-fixture-orphan"},
		{"fixture: raise covered", fixtureSites, KindReportError, StatusCovered, "doctor-fixture-covered"},
		{"fixture: raise uncovered", fixtureSites, KindReportWarning, StatusCodeNotDoctor, "fixture-unregistered"},
		{"fixture-row", fixtureSites, KindHealthRow, StatusHealthRow, ""},
		{"fixture: marked transient", fixtureMarked, KindLogWarn, StatusNoCode, ""},
		{"fixture: marked exempt", fixtureMarked, KindLogError, StatusNoCode, ""},
		{"other: covered", fixtureOther, KindLogWarn, StatusCovered, "doctor-fixture-covered"},
		{"other: uncovered", fixtureOther, KindLogError, StatusNoCode, ""},
	}
	for _, tc := range cases {
		site := siteWith(t, inventory.Sites, tc.message)
		if site.File != tc.file {
			t.Errorf("%q: file %s, want %s", tc.message, site.File, tc.file)
		}
		if site.Kind != tc.kind {
			t.Errorf("%q: kind %s, want %s", tc.message, site.Kind, tc.kind)
		}
		if site.Status != tc.status {
			t.Errorf("%q: status %s, want %s", tc.message, site.Status, tc.status)
		}
		if site.Code != tc.code {
			t.Errorf("%q: code %q, want %q", tc.message, site.Code, tc.code)
		}
		if site.Line == 0 {
			t.Errorf("%q: no line", tc.message)
		}
	}
	// Every other call in the fixture is no site: Info and Debug lines,
	// http.Error, an error's text, an Info-level Log, the test file and the
	// tooling package.
	if len(inventory.Sites) != len(cases) {
		t.Errorf("scan found %d sites, want %d: %+v", len(inventory.Sites), len(cases), inventory.Sites)
	}
	dynamic := siteWith(t, inventory.Sites, "fixture: dynamic code")
	if dynamic.CodeExpr != "dynamic" {
		t.Errorf("dynamic code expression %q, want %q", dynamic.CodeExpr, "dynamic")
	}
	covered := siteWith(t, inventory.Sites, "fixture: covered constant")
	if strings.Join(covered.Checks, ",") != "fixture-check" {
		t.Errorf("covered site names checks %v, want [fixture-check]", covered.Checks)
	}
	if strings.Join(covered.Doors, ",") != "ai/doors/system-plugins.md" {
		t.Errorf("covered site doors %v, want the plugins door", covered.Doors)
	}
}

// findingsTree holds diagnostic.Diagnostic composite literals: Fix set, Fix
// absent, Fix the empty string and Fix a non-literal expression, written as a
// plain literal, an address-of literal, elided slice elements, through an
// import alias and unqualified inside the diagnostic package, beside a zero
// value that is no finding.
const findingsTree = "testdata/findings"

const (
	findingFile = "internal/plugins/finding/finding.go"
	aliasedFile = "internal/component/aliased/aliased.go"
	coreFile    = "internal/core/diagnostic/types.go"
)

// TestDoctorCoverageListsDiagnosticLiterals is AC-10's report form: every
// diagnostic.Diagnostic literal is a doctor-finding site, covered when its
// Fix is set to anything but the empty string, in every literal shape and
// under internal/core too, where a finding reaches the operator unchanged.
// The zero value is no site, and the report counts the covered findings as
// covered and lists the others.
func TestDoctorCoverageListsDiagnosticLiterals(t *testing.T) {
	inventory, err := Scan(findingsTree, 0, fixtureDoors)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	cases := []struct {
		message string
		file    string
		status  Status
	}{
		{"finding: fix set", findingFile, StatusFixSet},
		{"finding: no fix", findingFile, StatusNoFix},
		{"finding: fix empty", findingFile, StatusNoFix},
		{"finding: fix computed", findingFile, StatusFixSet},
		{"aliased: no fix", aliasedFile, StatusNoFix},
		{"core: own finding", coreFile, StatusFixSet},
	}
	for _, tc := range cases {
		site := siteWith(t, inventory.Sites, tc.message)
		if site.File != tc.file {
			t.Errorf("%q: file %s, want %s", tc.message, site.File, tc.file)
		}
		if site.Kind != KindDoctorFinding {
			t.Errorf("%q: kind %s, want %s", tc.message, site.Kind, KindDoctorFinding)
		}
		if site.Status != tc.status {
			t.Errorf("%q: status %s, want %s", tc.message, site.Status, tc.status)
		}
		if site.Line == 0 {
			t.Errorf("%q: no line", tc.message)
		}
	}
	if len(inventory.Sites) != len(cases) {
		t.Errorf("scan found %d sites, want %d (the zero value is no finding): %+v", len(inventory.Sites), len(cases), inventory.Sites)
	}
	fixed := siteWith(t, inventory.Sites, "finding: fix set")
	if fixed.Call != "diagnostic.Diagnostic" {
		t.Errorf("elided element call %q, want the element type diagnostic.Diagnostic", fixed.Call)
	}
	if fixed.Code != "doctor-finding-fixed" {
		t.Errorf("finding code %q, want doctor-finding-fixed", fixed.Code)
	}
	if fixed.FixExpr != `"ze start"` {
		t.Errorf("finding fix expression %q, want %q", fixed.FixExpr, `"ze start"`)
	}
	if empty := siteWith(t, inventory.Sites, "finding: fix empty"); empty.FixExpr != "" {
		t.Errorf("a Fix of the empty string reads as %q, want none", empty.FixExpr)
	}

	report := treeReport(inventory)
	if report.Covered != 3 {
		t.Errorf("report covered %d, want 3", report.Covered)
	}
	if report.Uncovered != 3 {
		t.Errorf("report uncovered %d, want 3", report.Uncovered)
	}
	if report.Findings != 6 {
		t.Errorf("report findings %d, want 6", report.Findings)
	}
	if report.FindingsFixed != 3 {
		t.Errorf("report findings with a Fix %d, want 3", report.FindingsFixed)
	}
	text := report.Text()
	for _, want := range []string{"6 doctor finding(s), 3 with a Fix", "doctor-finding no-fix diagnostic.Diagnostic", `"finding: no fix"`, `"finding: fix empty"`, `"aliased: no fix"`} {
		if !strings.Contains(text, want) {
			t.Errorf("report does not list %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "finding: fix set") {
		t.Errorf("report lists a covered finding:\n%s", text)
	}
}

// TestDoctorCoverageRefusesUnparsableFile proves a file the scan cannot
// parse is an error, never a file with no site.
func TestDoctorCoverageRefusesUnparsableFile(t *testing.T) {
	tree := t.TempDir()
	writeTreeFile(t, tree, "internal/plugins/broken/broken.go", "package broken\n\nfunc {\n")
	if _, err := Scan(tree, 0, fixtureDoors); err == nil {
		t.Fatal("Scan of an unparsable file answered no error")
	}
}
