// VALIDATES: AC-5, the report form of le arch doctor-coverage: every
// uncovered site in the tree, grouped by door area and owner package, with
// counts, and the health rows the audit maps by hand counted apart; and that
// the real checkout scans above the site floor with a door for every file.
// PREVENTS: an inventory the audit cannot split by area, and a scan of the
// real tree that matched too little to judge it.
package archdoctorcoverage

import (
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
