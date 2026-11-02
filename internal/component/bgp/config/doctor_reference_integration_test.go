// Design: docs/architecture/doctor-and-health-checks.md -- parsed config diagnostics.

//go:build ze_bgp

package bgpconfig_test

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestDoctorReferenceBeforeLoadRefusal proves that a failed load preserves both
// its refusal and the specific reference diagnostic, without running checks that
// require a successfully loaded configuration.
func TestDoctorReferenceBeforeLoadRefusal(t *testing.T) {
	postCalled := false
	const checkName = "test-reference-post-load"
	err := diagnostic.RegisterDoctorCheck(diagnostic.DoctorCheck{
		Name: checkName, Phase: diagnostic.DoctorPhasePostConfig,
		Component: "test", Dependencies: []string{"config-tree"},
		Platforms: []string{diagnostic.DoctorPlatformAny},
		Codes:     []string{"doctor-config-parse"},
		Check: func(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
			postCalled = true
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { diagnostic.UnregisterDoctorCheckForTest(checkName) })
	result := referenceDoctorResult(t, `bgp {
		router-id 1.2.3.4;
		peer peer1 {
			filter { import { filters [ nonexistent-policy ]; final accept; } }
		}
	}`)
	diags := result.Diagnostics
	if result.Ready {
		t.Fatal("doctor accepted an invalid configuration")
	}
	loadRefused := false
	found := false
	for _, d := range diags {
		if d.Code == "doctor-config-parse" {
			loadRefused = true
		}
		if d.Code != "doctor-config-reference" {
			continue
		}
		if d.Severity != diagnostic.SeverityError {
			t.Errorf("reference severity = %s, want error", d.Severity)
		}
		if strings.Contains(d.Message, "references undefined filter 'nonexistent-policy'") {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing undefined-reference diagnostic: %+v", diags)
	}
	if !loadRefused {
		t.Fatal("load validation did not report its refusal")
	}
	if postCalled {
		t.Fatal("post-config check ran after a refused load")
	}
}
