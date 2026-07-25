// VALIDATES: spec-ospf-ext-14 AC-25 -- `ze doctor` emits a Warning (a NEW ext-14 code,
// doctor-ospf-debug-enabled) while debug LSA injection is left enabled, and nothing when it
// is off; the two config-sanity codes are unaffected.
// PREVENTS: a router silently left able to originate crafted test LSAs into production.
package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

func TestDebugEnabledDoctorWarning(t *testing.T) {
	t.Cleanup(func() { setDebugInjectEnabled(false) })

	setDebugInjectEnabled(false)
	if diags := checkOSPFDebugEnabled(diagnostic.DoctorCheckContext{}); len(diags) != 0 {
		t.Fatalf("debug disabled should yield no diagnostics, got %+v", diags)
	}

	setDebugInjectEnabled(true)
	diags := checkOSPFDebugEnabled(diagnostic.DoctorCheckContext{})
	if len(diags) != 1 {
		t.Fatalf("debug enabled diagnostics = %d, want 1", len(diags))
	}
	if diags[0].Code != codeOSPFDebugEnabled {
		t.Fatalf("code = %q, want %q", diags[0].Code, codeOSPFDebugEnabled)
	}
	if diags[0].Severity != diagnostic.SeverityWarning {
		t.Fatalf("severity = %q, want warning", diags[0].Severity)
	}
}
