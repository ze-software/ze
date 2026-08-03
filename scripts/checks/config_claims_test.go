package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// TestConfigClaimsGate runs scripts/checks/config_claims.go over the tree.
//
// The gate proves every config subtree an operator can write is delivered to
// some plugin config root, some hub handler path, or a recorded exception. The
// daemon accepts an unclaimed subtree in silence: reloadConfig logs Info "config
// reload: no affected plugins, updating config" and stores it
// (internal/component/plugin/server/reload.go).
//
// It runs the gate with the full feature tag set, because a reduced set
// compiles modules out and shrinks what the gate can see.
//
// VALIDATES: AC-1, AC-2, AC-3 (spec improve-7) from the make-target entry point,
// which is the one a verify stage runs.
// PREVENTS: a YANG config module landing without the matching ConfigRoots entry.
func TestConfigClaimsGate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "make", "ze-config-claims-check")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("config-claims gate failed:\n%s", out)
	}
	if !strings.Contains(string(out), "config-claims: OK") {
		t.Fatalf("config_claims.go did not report OK:\n%s", out)
	}
}

// TestConfigClaimsGateReportsItsInventory asserts the gate says what it read.
//
// An advisory-looking pass and a real pass are the same text when the walk
// enumerated nothing. The gate carries its own floors, so this asserts the
// counts reach the reader rather than re-deriving them.
//
// VALIDATES: AC-6 support -- the JSON report carries the enumerated root and
// claim counts.
// PREVENTS: a broken enumeration passing as a clean tree.
func TestConfigClaimsGateReportsItsInventory(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "make", "ze-config-claims-check-json")
	cmd.Dir = repoRoot(t)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("config-claims gate failed: %v", err)
	}

	start := bytes.IndexByte(out, '{')
	if start < 0 {
		t.Fatalf("no JSON object in the gate output:\n%s", out)
	}

	var report struct {
		Roots       int      `json:"roots"`
		Claims      int      `json:"claims"`
		Allowlisted []string `json:"allowlisted"`
		Findings    []string `json:"findings"`
	}
	if err := json.Unmarshal(out[start:], &report); err != nil {
		t.Fatalf("gate output is not JSON: %v\n%s", err, out)
	}
	if report.Roots < 25 {
		t.Errorf("gate reported only %d config roots: the schema walk covers almost nothing", report.Roots)
	}
	if report.Claims < 50 {
		t.Errorf("gate reported only %d claims: the plugin registry did not populate", report.Claims)
	}
	if len(report.Allowlisted) == 0 {
		t.Error("gate reported no allowlisted paths: the recorded exceptions are not reaching the report, so a reader cannot see what was skipped")
	}
	if len(report.Findings) != 0 {
		t.Errorf("gate reported findings: %v", report.Findings)
	}
}
