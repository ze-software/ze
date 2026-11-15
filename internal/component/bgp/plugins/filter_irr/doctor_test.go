package filter_irr

import (
	"strings"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// findingsOf answers the plugin's runtime check and fails the test on an error.
func findingsOf(t *testing.T, plug *irrPlugin) []sdk.DoctorCheckDiagnostic {
	t.Helper()
	findings, err := plug.answerDoctorCheck(storedCopyCheckName)
	if err != nil {
		t.Fatalf("answerDoctorCheck: %v", err)
	}
	return findings
}

// VALIDATES: AC-19 -- a generation the irr plugin raised that the filter has
// not applied is reported with the behind code, both generations and the fix;
// once the notice is applied the finding clears.
// METHOD: read AS-TEST (generation 1), store a new copy without delivering
// its notice (generation 2), ask the check; then deliver it and ask again.
func TestDoctorReportsAGenerationBehind(t *testing.T) {
	state := newFakeConsumerState()
	state.storeList(t, "AS-TEST", "10.0.0.0/24")
	plug := storedPlugin(state, map[uint32]string{65001: "AS-TEST"})
	readStored(plug)
	if findings := findingsOf(t, plug); len(findings) != 0 {
		t.Fatalf("findings with every generation applied = %+v, want none", findings)
	}

	notice := state.storeList(t, "AS-TEST", "10.0.0.0/24 10.0.1.0/24")
	findings := findingsOf(t, plug)
	if len(findings) != 1 || findings[0].Code != codeIRRStoredCopyBehind {
		t.Fatalf("findings with a notice missed = %+v, want one %s", findings, codeIRRStoredCopyBehind)
	}
	if !strings.Contains(findings[0].Message, "generation 1") || !strings.Contains(findings[0].Message, "stored 2") {
		t.Fatalf("message %q, want the applied generation 1 and the stored generation 2", findings[0].Message)
	}
	if findings[0].Fix != "update irr all" {
		t.Fatalf("fix %q, want update irr all", findings[0].Fix)
	}

	deliver(plug, notice)
	if findings := findingsOf(t, plug); len(findings) != 0 {
		t.Fatalf("findings after the notice was applied = %+v, want none", findings)
	}
}

// VALIDATES: AC-19, AC-22 -- a stored copy the filter cannot read is reported
// with the unreadable code, naming the AS number and the set, and the fix
// that fetches the set again; a readable copy clears it.
// METHOD: corrupt the dataset under a new pointer and deliver its notice, ask
// the check; then store a good copy, deliver it and ask again.
func TestDoctorReportsAnUnreadableCopy(t *testing.T) {
	state := newFakeConsumerState()
	state.storeList(t, "AS-TEST", "10.0.0.0/24")
	plug := storedPlugin(state, map[uint32]string{65001: "AS-TEST"})
	readStored(plug)

	notice := state.storeList(t, "AS-TEST", "10.0.0.0/24 10.0.1.0/24")
	state.mu.Lock()
	for key, value := range state.values {
		if strings.HasPrefix(key, "meta/irr-data/") {
			value[len(value)-1] ^= 0xff
		}
	}
	state.mu.Unlock()
	deliver(plug, notice)

	findings := findingsOf(t, plug)
	if len(findings) != 1 || findings[0].Code != codeIRRStoredCopyUnreadable {
		t.Fatalf("findings with an unreadable copy = %+v, want one %s", findings, codeIRRStoredCopyUnreadable)
	}
	if !strings.Contains(findings[0].Message, "AS65001") || !strings.Contains(findings[0].Message, "AS-TEST") {
		t.Fatalf("message %q, want AS65001 and AS-TEST named", findings[0].Message)
	}
	if findings[0].Fix != "update irr as-set AS-TEST" {
		t.Fatalf("fix %q, want update irr as-set AS-TEST", findings[0].Fix)
	}

	deliver(plug, state.storeList(t, "AS-TEST", "10.0.0.0/24 10.0.2.0/24"))
	if findings := findingsOf(t, plug); len(findings) != 0 {
		t.Fatalf("findings after a readable copy = %+v, want none", findings)
	}
}
