// Design: docs/architecture/doctor-and-health-checks.md -- source declarations cover fixed codes
package archdoctorcoverage

import (
	"strings"
	"testing"
)

// TestDoctorCoverageComputedNamesKeepSourceOrigins proves the source scan covers
// a fixed code even when its declaration's name is a parameter, retains the
// declaration's location, and refuses a site whose code is known only at runtime.
func TestDoctorCoverageComputedNamesKeepSourceOrigins(t *testing.T) {
	const file = "internal/plugins/computed/register.go"
	const source = `package computed
import (
	"log/slog"
	"github.com/ze-software/ze/internal/core/diagnostic"
)
func check(name, code string) diagnostic.DoctorCheck {
	return diagnostic.DoctorCheck{Name: name, Codes: []string{"doctor-fixture-computed-name", code}}
}
func warn(code string) {
	slog.Warn("computed name: fixed code", "code", "doctor-fixture-computed-name")
	slog.Warn("computed name: runtime code", "code", code)
}
`
	inventory, err := scan(fixtureTree, map[string]overlayFile{
		file: {text: source, present: true},
	}, fixtureCodesFloor, fixtureDoors)
	if err != nil {
		t.Fatal(err)
	}
	fixed := siteWith(t, inventory.Sites, "computed name: fixed code")
	if fixed.Status != StatusCovered {
		t.Fatalf("fixed code has status %s, want covered: %+v", fixed.Status, fixed)
	}
	if len(fixed.Checks) != 1 {
		t.Fatalf("fixed code names %d declarations, want 1: %+v", len(fixed.Checks), fixed.Checks)
	}
	if !strings.HasPrefix(fixed.Checks[0], file+":") {
		t.Errorf("computed-name declaration %q does not retain its source origin %s", fixed.Checks[0], file)
	}
	if report := checkLines(inventory.Sites, changedAt(fixed)); report.exitCode() != 0 {
		t.Errorf("a fixed code declared under a computed name was refused: %+v", report.Findings)
	}
	dynamic := siteWith(t, inventory.Sites, "computed name: runtime code")
	if dynamic.Status != StatusCodeDynamic {
		t.Fatalf("runtime code has status %s, want dynamic: %+v", dynamic.Status, dynamic)
	}
	report := checkLines(inventory.Sites, changedAt(dynamic))
	if report.exitCode() != 1 {
		t.Fatalf("runtime code answers exit %d, want refusal: %+v", report.exitCode(), report.Findings)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("runtime code answers %d findings, want 1: %+v", len(report.Findings), report.Findings)
	}
}
