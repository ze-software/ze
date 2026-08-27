package deployment

import (
	"encoding/json"
	"errors"
	"testing"
)

// TestVPPProofVerdictUsesAnInvalidZeroValue validates that an untouched result
// cannot render as proof.
//
// VALIDATES: a VPP proof verdict has no valid zero-value answer.
// PREVENTS: an unexecuted scenario appearing as a pass in structured output.
func TestVPPProofVerdictUsesAnInvalidZeroValue(t *testing.T) {
	if VPPProofUnspecified.String() != "unspecified" {
		t.Fatalf("the zero verdict rendered %q", VPPProofUnspecified.String())
	}
	if VPPProofPass == VPPProofUnspecified || VPPProofFail == VPPProofUnspecified {
		t.Fatal("a valid verdict is the zero value")
	}

	body, err := json.Marshal(VPPProofFail)
	if err != nil {
		t.Fatalf("marshal the verdict: %v", err)
	}
	if string(body) != `"fail"` {
		t.Fatalf("the failed verdict marshaled as %s", body)
	}
}

// TestTheVPPReportIsStructuredData validates every public field and JSON key in
// the action payload.
//
// VALIDATES: the command answers typed scenario and check data.
// PREVENTS: replacing the payload with preformatted prose.
func TestTheVPPReportIsStructuredData(t *testing.T) {
	report := VPPReport{
		Image:     "image",
		Container: "container",
		Version:   "version",
		Interface: "loop7",
		Scenarios: []VPPScenarioReport{{
			Scenario: VPPScenarioIPv4FIB,
			Verdict:  VPPProofPass,
			Checks: []VPPCheck{{
				Check:    "route-installed",
				Verdict:  VPPProofPass,
				Detail:   "real VPP FIB contains 10.20.0.0/24",
				Evidence: []string{"10.20.0.0/24 via 10.0.0.1"},
			}},
		}},
		Passed: true,
	}

	body, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal the report: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal the report: %v", err)
	}
	for _, key := range []string{"image", "container", "vpp-version", "interface", "scenarios", "passed"} {
		if _, ok := got[key]; !ok {
			t.Errorf("the report has no %q key: %s", key, body)
		}
	}

	scenarios, ok := got["scenarios"].([]any)
	if !ok || len(scenarios) != 1 {
		t.Fatalf("the scenarios are %#v", got["scenarios"])
	}
	one, ok := scenarios[0].(map[string]any)
	if !ok {
		t.Fatalf("the scenario is %#v", scenarios[0])
	}
	if one["scenario"] != VPPScenarioIPv4FIB || one["verdict"] != "pass" {
		t.Fatalf("the scenario payload is %#v", one)
	}
}

// TestVPPReportTextRendersEveryCheck validates that the prose view is derived
// from the same checks carried by JSON.
//
// VALIDATES: successful and failed checks have distinct observable renderings.
// PREVENTS: a failed check rendering as an OK line.
func TestVPPReportTextRendersEveryCheck(t *testing.T) {
	report := VPPReport{
		Interface: "loop0",
		Scenarios: []VPPScenarioReport{
			{Scenario: VPPScenarioIPv4FIB, Verdict: VPPProofPass, Checks: []VPPCheck{{
				Check: "installed", Verdict: VPPProofPass, Detail: "route installed",
			}}},
			{Scenario: VPPScenarioFirewall, Verdict: VPPProofFail, Checks: []VPPCheck{{
				Check: "bound", Verdict: VPPProofFail, Detail: "ACL not bound",
				Evidence: []string{"show acl output"}, LogTail: []string{"daemon stopped"},
			}}},
		},
	}
	want := "OK: created real VPP loopback interface loop0\n" +
		"OK: route installed\n" +
		"FAIL: ACL not bound\n" +
		"show acl output\n" +
		"ze log tail:\n" +
		"daemon stopped\n"
	if got := report.Text(); got != want {
		t.Fatalf("Text() =\n%s\nwant\n%s", got, want)
	}
}

// TestVPPActionExitCodeMapping validates operating errors and proof failures
// independently.
//
// VALIDATES: operating errors and proof failures exit 1, and a full proof exits 0.
// PREVENTS: a non-nil report hiding an operating error or failed verdict.
func TestVPPActionExitCodeMapping(t *testing.T) {
	tests := []struct {
		name   string
		passed bool
		err    error
		want   int
	}{
		{name: "pass", passed: true, want: 0},
		{name: "proof failure", passed: false, want: 1},
		{name: "operating error", passed: true, err: errors.New("docker stopped"), want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := vppExitCode(tt.passed, tt.err); got != tt.want {
				t.Fatalf("vppExitCode() = %d, want %d", got, tt.want)
			}
		})
	}
}
