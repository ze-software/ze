// VALIDATES: every stage writes a log file named after its command, with both
// separators flattened to a hyphen, so a renamed command renames its log file
// (AC-18 of spec-le-subject-first-command-tree).
package verify

import (
	"context"
	"path"
	"strings"
	"testing"

	verifyengine "github.com/ze-software/ze/internal/le/verify/engine"
)

// TestStageLogNamesFollowTheCommandName pins the flattening every verification
// artifact depends on.
//
// `stageLogPath` turns both separators into a hyphen, so `arch tier/check`
// writes NN-arch-tier-check.log. The failure index, every rerun line and the
// functional fixtures read those paths, so a stage rename is a changed
// expectation here, never a silent move.
func TestStageLogNamesFollowTheCommandName(t *testing.T) {
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
		"go lint/run":                      "01-go-lint-run.log",
		"go version-pin/check":             "go-version-pin-check.log",
		"go vet-platforms/darwin/freebsd":  "go-vet-platforms-darwin-freebsd.log",
		"go staticcheck/check/part/1/of/6": "go-staticcheck-check-part-1-of-6.log",
		"verify deps/unit-cached":          "verify-deps-unit-cached.log",
		"doc wiring":                       "doc-wiring.log",
		"doc check/verify":                 "doc-check-verify.log",
		"repo compiles/check":              "repo-compiles-check.log",
		"repo/tree-check":                  "repo-tree-check.log",
		"arch tier/check":                  "arch-tier-check.log",
		"cli stdio/check":                  "cli-stdio-check.log",
		"config ports/check":               "config-ports-check.log",
		"plugin boundary/check":            "plugin-boundary-check.log",
		"site facts/check":                 "site-facts-check.log",
		"web vendor/check":                 "web-vendor-check.log",
		"doc index/check":                  "doc-index-check.log",
		"ai rules/lint":                    "ai-rules-lint.log",
		"ai hooks/unit":                    "ai-hooks-unit.log",
		"test sensitivity/check":           "test-sensitivity-check.log",
		"test weakened/check":              "test-weakened-check.log",
		"test health/check":                "test-health-check.log",
		"test functional/gating":           "test-functional-gating.log",
		"test functional/exabgp-test":      "test-functional-exabgp-test.log",
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
