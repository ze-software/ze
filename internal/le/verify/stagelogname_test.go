// VALIDATES: a namespaced stage writes the log file name its hyphenated
// spelling wrote, so no verification artifact path moved when the twenty-one
// commands gained a space (spec-le-command-namespaces, AC-14).
package verify

import (
	"context"
	"path"
	"strings"
	"testing"

	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// TestStageLogNamesAreUnchangedByNamespacing pins the flattening every
// verification artifact depends on.
//
// `stageLogPath` turns both separators into a hyphen, so `verify lint/run`
// writes 01-verify-lint-run.log, which is the name it wrote when the command
// was spelled `verify-lint`. That was luck rather than design at the rename,
// and the failure index, every rerun line and the functional fixtures read
// those paths. This test is what makes it design.
func TestStageLogNamesAreUnchangedByNamespacing(t *testing.T) {
	repo := newFixtureRepo(t)
	repo.commit(t, "fixture", "one")

	runner := func(_ context.Context, _ string, identity verifyengine.Identity) verifyengine.ActionResult {
		return verifyengine.ActionResult{Identity: identity, Registered: true, Completed: true, Output: "ok\n"}
	}
	report := runCurrent(context.Background(), repo.root, "full", runner)
	if report.Code != 0 || !report.Completed {
		t.Fatalf("report = %#v", report)
	}

	// One row per namespaced family that owns a stage, named in full so a
	// changed name is a changed expectation rather than a silent pass.
	want := map[string]string{
		"verify lint/run":                "01-verify-lint-run.log",
		"verify deps/unit-cached":        "verify-deps-unit-cached.log",
		"doc wiring":                     "doc-wiring.log",
		"doc check/verify":               "doc-check-verify.log",
		"repository tracked-build/check": "repository-tracked-build-check.log",
		"plugin boundary/check":          "plugin-boundary-check.log",
		"site facts/check":               "site-facts-check.log",
	}

	seen := 0
	for _, stage := range report.Stages {
		expected, named := want[stage.Identity.Name]
		if !named {
			continue
		}
		seen++
		base := path.Base(stage.Log)
		if !strings.Contains(base, expected) {
			t.Errorf("stage %q wrote %q, want a name holding %q: an artifact path moved", stage.Identity.Name, base, expected)
		}
		if strings.ContainsAny(base, " /") {
			t.Errorf("stage %q wrote %q, which holds a separator no reader of these paths expects", stage.Identity.Name, base)
		}
	}
	if seen == 0 {
		t.Fatalf("no stage of the full population carried a name this test knows; it asserted nothing")
	}
}
