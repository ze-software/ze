// VALIDATES: spec-ospf-ext-5 AC-21 -- the SR doctor check reports an SRGB/SRLB
// overlap (or other unsound range) before runtime install.
// PREVENTS: a double-claimed label reaching the data plane undetected.
package ospf

import (
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/sr"
)

func TestSRDoctorReportsOverlap(t *testing.T) {
	// SRGB overlapping SRLB yields a diagnostic with the registered code.
	bad := &sr.SRConfig{
		Enabled: true,
		SRGB:    []sr.LabelRange{{Base: 16000, Size: 8000}},
		SRLB:    []sr.LabelRange{{Base: 20000, Size: 1000}},
	}
	diags := srConfigDiagnostics(bad)
	if len(diags) != 1 || diags[0].Code != codeOSPFSegmentRoutingOverlap {
		t.Fatalf("expected one overlap diagnostic, got %+v", diags)
	}
}

func TestSRDoctorSilentWhenValid(t *testing.T) {
	good := &sr.SRConfig{
		Enabled: true,
		SRGB:    []sr.LabelRange{{Base: 16000, Size: 8000}},
		SRLB:    []sr.LabelRange{{Base: 40000, Size: 1000}},
	}
	if diags := srConfigDiagnostics(good); len(diags) != 0 {
		t.Fatalf("valid SR config must produce no diagnostics: %+v", diags)
	}
	// A nil (unconfigured) SR block is silent.
	if diags := srConfigDiagnostics(nil); len(diags) != 0 {
		t.Fatalf("nil SR config must produce no diagnostics: %+v", diags)
	}
	// A disabled SR block is silent.
	if diags := srConfigDiagnostics(&sr.SRConfig{Enabled: false}); len(diags) != 0 {
		t.Fatalf("disabled SR config must produce no diagnostics: %+v", diags)
	}
}
