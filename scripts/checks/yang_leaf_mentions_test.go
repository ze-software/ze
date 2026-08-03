package main

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// TestYANGLeafMentionReport runs the advisory leaf mention-check in self-test
// mode.
//
// The check is a HEURISTIC: plugins parse config out of map[string]any with
// string-literal keys, so a YANG leaf whose kebab name appears in no literal of
// the owning package is probably never read. Probably is not certainly, which
// is why the report is advisory and is not wired into any verify stage. The
// self-test is what keeps it honest: over a fixture whose answer is known, it
// must report the unread leaf and must stay silent on the read one.
//
// VALIDATES: AC-8 (spec improve-7) -- a YANG leaf under a claimed root whose
// kebab name appears nowhere in the owning plugin package is listed with its
// module, leaf path, and owning package.
// PREVENTS: the report going quietly empty after a parser or discovery change,
// which would read as "every leaf is consumed".
func TestYANGLeafMentionReport(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", "scripts/checks/yang_leaf_mentions.go", "--selftest")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("leaf mention-check self-test failed:\n%s", out)
	}
	if !strings.Contains(string(out), "yang-leaf-mentions: selftest OK") {
		t.Fatalf("self-test did not report OK:\n%s", out)
	}
}

// TestYANGLeafMentionReportRunsOverTheTree runs the check over the repository
// and asserts it produced a report it could act on.
//
// The check is advisory and exits 0 whatever it finds, so a broken discovery
// walk would look exactly like a clean tree. This asserts it actually read
// modules, which is the only part of an advisory check that can be wrong
// without anybody noticing.
//
// VALIDATES: AC-8 -- the report covers the real tree, not an empty walk.
// PREVENTS: a silent zero-module report being mistaken for zero findings.
func TestYANGLeafMentionReportRunsOverTheTree(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "run", "scripts/checks/yang_leaf_mentions.go", "--json")
	cmd.Dir = repoRoot(t)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("leaf mention-check failed: %v", err)
	}

	var report struct {
		Modules  int `json:"modules"`
		Leaves   int `json:"leaves"`
		Findings []struct {
			Module  string `json:"module"`
			Package string `json:"package"`
			Path    string `json:"path"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(out, &report); err != nil {
		t.Fatalf("report is not JSON: %v\n%s", err, out)
	}

	// Floors, not counts: 68 config modules on 2026-08-03.
	const minModules = 40
	if report.Modules < minModules {
		t.Errorf("the check read only %d config modules (floor %d): discovery no longer matches the tree layout, so the report covers almost nothing", report.Modules, minModules)
	}
	if report.Leaves == 0 {
		t.Error("the check found no YANG leaves at all: the leaf parser matched nothing, so every module would look fully consumed")
	}
	for _, f := range report.Findings {
		if f.Module == "" || f.Package == "" || f.Path == "" {
			t.Errorf("a finding is missing its module, package, or leaf path: %+v", f)
		}
	}
}
