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

// fixtureCovered is the covered set the fixture's registered checks would
// declare: one code, held by one check.
func fixtureCovered() Covered {
	return Covered{"doctor-fixture-covered": {"fixture-check"}}
}

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

// scanFixture scans the fixture tree with the fixture covered set.
func scanFixture(t *testing.T) Inventory {
	t.Helper()
	inventory, err := Scan(fixtureTree, fixtureCovered(), fixtureDoors)
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

// TestDoctorCoverageRefusesUnparsableFile proves a file the scan cannot
// parse is an error, never a file with no site.
func TestDoctorCoverageRefusesUnparsableFile(t *testing.T) {
	tree := t.TempDir()
	writeTreeFile(t, tree, "internal/plugins/broken/broken.go", "package broken\n\nfunc {\n")
	if _, err := Scan(tree, fixtureCovered(), fixtureDoors); err == nil {
		t.Fatal("Scan of an unparsable file answered no error")
	}
}
