// Related: configclaims.go -- the claim gate these tests drive from its entry point
//
// Every test here calls the tool as a function. The gate used to be reachable
// only as a subprocess, so what it proved about the tree cost a `go run` per
// assertion and could say nothing about a case the tree does not hold.

package configclaims

import (
	"encoding/json"
	"strings"
	"testing"
)

// VALIDATES: the audit reads the LIVE registry, so the counts it reports are
// the daemon's own inventory rather than a list written down twice.
// PREVENTS: an enumeration that broke reporting a clean tree.
//
// This is where TestConfigClaimsGate and TestConfigClaimsGateReportsItsInventory
// (scripts/checks/config_claims_test.go) now live. Those two forked
// `make ze-config-claims-check` and `-json` and asserted, between them, that the
// tree passes, that the page says OK, and that the JSON carries at least 25
// roots, at least 50 claims, a non-empty exception list and no findings. All
// four facts are asserted below, from a function call.
func TestAuditReadsTheLiveRegistry(t *testing.T) {
	report, err := Audit()
	if err != nil {
		t.Fatalf("audit the live registry: %v", err)
	}

	if report.Roots < minRoots {
		t.Errorf("the audit enumerated %d top-level config roots, floor %d: the schema walk covers almost nothing", report.Roots, minRoots)
	}
	if report.Claims < minClaims {
		t.Errorf("the audit enumerated %d claims, floor %d: the plugin registry did not populate", report.Claims, minClaims)
	}
	if len(report.Allowlisted) == 0 {
		t.Error("the audit reported no allowlisted paths, so a reader cannot see which subtrees were skipped")
	}
	if len(report.Findings) != 0 {
		t.Errorf("the audit reported findings, so a config subtree reaches nobody: %v", report.Findings)
	}
}

// VALIDATES: the floors fire when the inventory comes back too small to have
// checked anything, and they say which half broke.
// PREVENTS: the vacuous pass -- an empty walk and a clean tree print the same
// page, and the gate's whole subject is delivery to nobody.
func TestFloorsFireOnAnInventoryTooSmallToMeanAnything(t *testing.T) {
	cases := []struct {
		name    string
		roots   int
		claims  int
		wantSub []string
	}{
		{"both at their floor", minRoots, minClaims, nil},
		{"one root short", minRoots - 1, minClaims, []string{"top-level config roots enumerated"}},
		{"one claim short", minRoots, minClaims - 1, []string{"claims enumerated"}},
		{"nothing enumerated at all", 0, 0, []string{"top-level config roots enumerated", "claims enumerated"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := floorFindings(tc.roots, tc.claims)
			if len(findings) != len(tc.wantSub) {
				t.Fatalf("%d roots and %d claims draw %d findings, want %d: %v", tc.roots, tc.claims, len(findings), len(tc.wantSub), findings)
			}
			for i, want := range tc.wantSub {
				if !strings.Contains(findings[i], want) {
					t.Errorf("finding %d is %q, want it to name %q", i, findings[i], want)
				}
				if !strings.HasPrefix(findings[i], "unclassifiable: ") {
					t.Errorf("finding %d is %q, want it to open with the unclassifiable verdict", i, findings[i])
				}
			}
		})
	}
}

// VALIDATES: the page a person reads carries the inventory, the exceptions and
// the verdict, and it ends with OK only when there is nothing to report.
// PREVENTS: a page that says OK over a report holding findings.
func TestTextCarriesTheInventoryAndTheVerdict(t *testing.T) {
	clean := Report{Roots: 36, Claims: 72, Allowlisted: []string{"bgp/hidden"}}
	text := clean.Text()

	for _, want := range []string{
		"# Config Claim Completeness Gate",
		"Top-level config roots: 36",
		"Claims: 72",
		"Allowlisted: 1",
		"  allowlisted bgp/hidden",
		"config-claims: OK\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the clean page does not carry %q:\n%s", want, text)
		}
	}

	failed := Report{Roots: 36, Claims: 72, Findings: []string{"unclaimed: ntp"}}
	text = failed.Text()
	if strings.Contains(text, "config-claims: OK") {
		t.Errorf("a page holding a finding still says OK:\n%s", text)
	}
	if !strings.Contains(text, "## Findings (1)") || !strings.Contains(text, "  unclaimed: ntp") {
		t.Errorf("the failing page does not carry its finding:\n%s", text)
	}
}

// VALIDATES: the payload is structured data under the keys the script's --json
// emitted, so `| json` renders it with no rendering code in the tool.
// PREVENTS: a tool that answers finished text, which is what the command
// contract forbids and what would make `| yaml` impossible.
func TestReportIsStructuredData(t *testing.T) {
	raw, err := json.Marshal(Report{Roots: 3, Claims: 4, Allowlisted: []string{"a"}, Findings: []string{"b"}})
	if err != nil {
		t.Fatalf("marshal the report: %v", err)
	}

	var back map[string]any
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("the report does not round-trip: %v", err)
	}
	for _, key := range []string{"roots", "claims", "allowlisted", "findings"} {
		if _, ok := back[key]; !ok {
			t.Errorf("the payload has no %q key, so it is not the shape the script published: %s", key, raw)
		}
	}
	if len(back) != 4 {
		t.Errorf("the payload carries %d keys, want the script's 4: %s", len(back), raw)
	}
}

// VALIDATES: the command takes no argument, and says so rather than ignoring
// one.
// PREVENTS: a path positional creeping in, which would break keyword-before-value
// and would make the tree the command judges depend on what a caller typed.
func TestAnswerRefusesAnArgument(t *testing.T) {
	payload, code := Answer([]string{"internal"})
	if code != 1 {
		t.Errorf("a stray argument answers %d, want 1", code)
	}
	if payload != nil {
		t.Errorf("a refused command answered a payload: %#v", payload)
	}
}

// VALIDATES: a clean tree answers 0 and a tree holding a finding answers 1, so
// the gate's verdict reaches the exit status.
// PREVENTS: a gate that reports a finding and exits 0, which is the failure
// every check in this directory exists to prevent, applied to itself.
func TestAnswerCarriesTheVerdictToTheExitCode(t *testing.T) {
	payload, code := Answer(nil)
	if payload == nil {
		t.Fatal("the command answered no payload")
	}
	report, ok := payload.(Report)
	if !ok {
		t.Fatalf("the command answered %T, want a Report", payload)
	}
	if code != verdict(report) {
		t.Errorf("the command answered %d over a report the verdict scores %d", code, verdict(report))
	}
}

// VALIDATES: one finding is enough to fail the gate, and an empty report passes.
// PREVENTS: a gate that reports what it found and exits 0. This checkout is
// clean, so the failing half of the verdict is reachable by no test that drives
// the live registry.
func TestVerdictFailsOnASingleFinding(t *testing.T) {
	if got := verdict(Report{Roots: 36, Claims: 72}); got != 0 {
		t.Errorf("a report with no findings scores %d, want 0", got)
	}
	if got := verdict(Report{Roots: 36, Claims: 72, Findings: []string{"unclaimed: ntp"}}); got != 1 {
		t.Errorf("a report holding one finding scores %d, want 1", got)
	}
}
