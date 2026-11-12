// VALIDATES: AC-5, the report form of le arch doctor-coverage: every
// uncovered site in the tree, grouped by door area and owner package, with
// counts, and the health rows the audit maps by hand counted apart; and that
// the real checkout scans above the site floor with a door for every file.
// It also validates spec-doctor-coverage-12-plugin-infra AC-46's report form:
// a plugin's rpc.DoctorCheckDiagnostic findings are listed under their owner.
// PREVENTS: an inventory the audit cannot split by area, and a scan of the
// real tree that matched too little to judge it.
package archdoctorcoverage

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/hookruntime"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestDoctorCoverageReportGroupsByOwner is AC-5: the fixture's uncovered
// sites land under their door and owner package with the right counts, the
// covered ones are counted and not listed, and the text names each group.
func TestDoctorCoverageReportGroupsByOwner(t *testing.T) {
	report := treeReport(scanFixture(t))
	if report.Sites != 18 {
		t.Errorf("sites %d, want 18", report.Sites)
	}
	if report.Covered != 8 {
		t.Errorf("covered %d, want 8", report.Covered)
	}
	if report.Uncovered != 9 {
		t.Errorf("uncovered %d, want 9", report.Uncovered)
	}
	if report.HealthRows != 1 {
		t.Errorf("health rows %d, want 1", report.HealthRows)
	}
	if len(report.Doors) != 2 {
		t.Fatalf("door groups %d, want 2: %+v", len(report.Doors), report.Doors)
	}
	want := []struct {
		door, pkg       string
		uncovered, rows int
	}{
		{"ai/doors/fixture-component.md", "internal/component/other", 1, 0},
		{"ai/doors/system-plugins.md", "internal/plugins/fixture", 8, 1},
	}
	for i, group := range report.Doors {
		if group.Door != want[i].door {
			t.Errorf("door group %d is %s, want %s", i, group.Door, want[i].door)
			continue
		}
		if len(group.Packages) != 1 {
			t.Fatalf("%s: packages %d, want 1", group.Door, len(group.Packages))
		}
		owner := group.Packages[0]
		if owner.Package != want[i].pkg {
			t.Errorf("%s: package %s, want %s", group.Door, owner.Package, want[i].pkg)
		}
		if owner.Uncovered != want[i].uncovered {
			t.Errorf("%s: uncovered %d, want %d", owner.Package, owner.Uncovered, want[i].uncovered)
		}
		if owner.HealthRows != want[i].rows {
			t.Errorf("%s: health rows %d, want %d", owner.Package, owner.HealthRows, want[i].rows)
		}
		if group.Uncovered != want[i].uncovered {
			t.Errorf("%s: door uncovered %d, want %d", group.Door, group.Uncovered, want[i].uncovered)
		}
		for _, site := range owner.Sites {
			if site.Status == StatusCovered {
				t.Errorf("%s: a covered site is listed: %+v", owner.Package, site)
			}
		}
	}
	text := report.Text()
	for _, line := range []string{
		"ai/doors/system-plugins.md: 8 uncovered, 1 health row(s)",
		"internal/plugins/fixture: 8 uncovered, 1 health row(s)",
		"ai/doors/fixture-component.md: 1 uncovered",
		"internal/component/other: 1 uncovered",
	} {
		if !strings.Contains(text, line) {
			t.Errorf("report text has no line %q:\n%s", line, text)
		}
	}
}

// TestDoctorCoverageReportListsPluginFindings is AC-46's report form. Goal:
// every rpc.DoctorCheckDiagnostic literal, written through rpc, through the
// SDK alias, as an elided element, address taken or unqualified in rpc
// itself, is a doctor finding judged on its Fix, and report lists the ones
// with no Fix under their owner package. Method: scan pluginFindingTree
// (judge_test.go) and read the inventory and the report.
func TestDoctorCoverageReportListsPluginFindings(t *testing.T) {
	inventory, err := Scan(pluginFindingTree(t), 0, fixtureDoors)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	cases := []struct {
		message string
		file    string
		call    string
		status  Status
	}{
		{"copp: no fix", pluginFindingCopp, "rpc.DoctorCheckDiagnostic", StatusNoFix},
		{"copp: fix set", pluginFindingCopp, "rpc.DoctorCheckDiagnostic", StatusFixSet},
		{"static: no fix", "internal/plugins/static/doctor.go", "sdk.DoctorCheckDiagnostic", StatusNoFix},
		{"static: fix empty", "internal/plugins/static/doctor.go", "sdk.DoctorCheckDiagnostic", StatusNoFix},
		{"rpc: own finding", "pkg/plugin/rpc/types.go", "DoctorCheckDiagnostic", StatusFixSet},
	}
	for _, tc := range cases {
		site := siteWith(t, inventory.Sites, tc.message)
		if site.File != tc.file {
			t.Errorf("%q: file %s, want %s", tc.message, site.File, tc.file)
		}
		if site.Kind != KindDoctorFinding {
			t.Errorf("%q: kind %s, want %s", tc.message, site.Kind, KindDoctorFinding)
		}
		if site.Call != tc.call {
			t.Errorf("%q: call %q, want %q", tc.message, site.Call, tc.call)
		}
		if site.Status != tc.status {
			t.Errorf("%q: status %s, want %s", tc.message, site.Status, tc.status)
		}
	}
	if len(inventory.Sites) != len(cases) {
		t.Errorf("scan found %d sites, want %d (an empty literal is no finding): %+v", len(inventory.Sites), len(cases), inventory.Sites)
	}

	report := treeReport(inventory)
	if report.Findings != 5 {
		t.Errorf("report findings %d, want 5", report.Findings)
	}
	if report.FindingsFixed != 2 {
		t.Errorf("report findings with a Fix %d, want 2", report.FindingsFixed)
	}
	owners := map[string]int{}
	for _, group := range report.Doors {
		for _, owner := range group.Packages {
			owners[owner.Package] = owner.Uncovered
		}
	}
	for pkg, want := range map[string]int{"internal/plugins/copp": 1, "internal/plugins/static": 2} {
		if owners[pkg] != want {
			t.Errorf("owner %s lists %d uncovered finding(s), want %d: %+v", pkg, owners[pkg], want, report.Doors)
		}
	}
	text := report.Text()
	for _, want := range []string{
		"5 doctor finding(s), 2 with a Fix",
		"internal/plugins/copp: 1 uncovered",
		"internal/plugins/static: 2 uncovered",
		"doctor-finding no-fix rpc.DoctorCheckDiagnostic",
		"doctor-finding no-fix sdk.DoctorCheckDiagnostic",
		`"copp: no fix"`, `"static: no fix"`, `"static: fix empty"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("report does not list %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "copp: fix set") {
		t.Errorf("report lists a finding with its Fix:\n%s", text)
	}
}

// TestDoctorCoverageScansTheCheckout scans the real checkout with the real
// covered set and doors: the scan clears its floors, every file maps to a
// door, and the report's counts add up.
func TestDoctorCoverageScansTheCheckout(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("checkout root: %v", err)
	}
	doorsOf, err := hookruntime.DoorsCovering(root)
	if err != nil {
		t.Fatalf("doors: %v", err)
	}
	inventory, err := Scan(root, coveredCodesFloor, doorsOf)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if err := checkSitesFloor(inventory, siteFloor); err != nil {
		t.Fatal(err)
	}
	for _, site := range inventory.Sites {
		if len(site.Doors) == 0 {
			t.Errorf("%s:%d has no door", site.File, site.Line)
		}
	}
	report := treeReport(inventory)
	if report.Covered+report.Uncovered+report.HealthRows != report.Sites {
		t.Errorf("covered %d + uncovered %d + health rows %d != sites %d",
			report.Covered, report.Uncovered, report.HealthRows, report.Sites)
	}
	t.Logf("checkout: %d sites, %d covered, %d uncovered, %d health rows, %d door groups",
		report.Sites, report.Covered, report.Uncovered, report.HealthRows, len(report.Doors))
}

// notProblemPackages are the plugin infrastructure packages whose every problem
// site spec-doctor-coverage-12-plugin-infra AC-44 makes covered.
var notProblemPackages = []string{
	"internal/component/plugin",
	"internal/component/plugin/ipc",
	"internal/component/plugin/process",
	"internal/component/plugin/server",
}

// notProblemSurvivor is the one file whose uncovered sites AC-44 permits: the
// five-stage startup-failure lines that
// plan/release/fixes/spec-doctor-reports-failed-plugin-startup.md owns until
// it lands.
const notProblemSurvivor = "internal/component/plugin/server/startup.go"

// pluginsHealthCode is the code the `plugins` health row's probe reads
// (process/register.go, report.HealthProbeDown).
const pluginsHealthCode = "doctor-plugin-stopped"

// TestNotProblemLinesCarryCodes scans the real checkout and proves that every
// problem site in the plugin infrastructure packages, the not-problem lines
// included, names a code a doctor check declares, apart from the startup
// lines another spec owns; and that the `plugins` health row's code is
// declared. Method: Scan the checkout, keep the sites whose package is one of
// the four, list each uncovered one; read the covered set from the same tree.
//
// VALIDATES: AC-36 to AC-42 and AC-44 of spec-doctor-coverage-12-plugin-infra.
// PREVENTS: a Warn or Error line in the plugin infrastructure that `ze doctor`
// cannot name, or a code on a line that no check declares.
func TestNotProblemLinesCarryCodes(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("checkout root: %v", err)
	}
	doorsOf, err := hookruntime.DoorsCovering(root)
	if err != nil {
		t.Fatalf("doors: %v", err)
	}
	inventory, err := Scan(root, coveredCodesFloor, doorsOf)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	var judged, healthRows int
	for i := range inventory.Sites {
		site := &inventory.Sites[i]
		if !slices.Contains(notProblemPackages, site.Package) {
			continue
		}
		judged++
		if site.Kind == KindHealthRow {
			healthRows++
			if site.Message != "plugins" {
				t.Errorf("%s:%d: health row %s is not the plugins row this test maps", site.File, site.Line, site.Message)
			}
			continue
		}
		if site.Status.Covered() {
			continue
		}
		if site.File == notProblemSurvivor {
			continue
		}
		t.Errorf("%s:%d: %s %q is %s (code %q)", site.File, site.Line, site.Call, site.Message, site.Status, site.CodeExpr)
	}
	if judged == 0 {
		t.Fatal("no site in the plugin infrastructure packages: the scan read nothing")
	}
	if healthRows != 1 {
		t.Errorf("found %d plugins health rows, want 1", healthRows)
	}
	index, err := indexTree(root, nil)
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	if _, declared := index.coveredCodes()[pluginsHealthCode]; !declared {
		t.Errorf("the plugins health row reads %s, which no doctor check declares", pluginsHealthCode)
	}
}
