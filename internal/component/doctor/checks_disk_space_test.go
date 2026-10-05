// Design: docs/architecture/doctor-and-health-checks.md -- usable disk accounting.

package doctor

import (
	"math"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/diskspace"
)

// TestCheckDiskSpaceBlocks pins the zero-total refusal and low-space boundary
// using signed availability, including the deficit FreeBSD can report.
func TestCheckDiskSpaceBlocks(t *testing.T) {
	for _, tc := range []struct {
		name      string
		total     uint64
		available int64
		severity  diagnostic.Severity
		actual    any
	}{
		{"zero total", 0, 0, diagnostic.SeverityError, nil},
		{"negative", 100, -1, diagnostic.SeverityWarning, "0%"},
		{"zero", 100, 0, diagnostic.SeverityWarning, "0%"},
		{"below five", 1000, 49, diagnostic.SeverityWarning, "4%"},
		{"five", 1000, 50, "", ""},
		{"normal", 1000, 750, "", ""},
		{"overflow below five", math.MaxUint64, math.MaxUint64 / 20, diagnostic.SeverityWarning, "4%"},
		{"overflow above five", math.MaxUint64, math.MaxUint64/20 + 1, "", ""},
		{"large normal", math.MaxUint64, math.MaxInt64, "", ""},
		{"available above total", 100, 101, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := checkDiskSpaceBlocks("config", tc.total, diskspace.UsableBlocks(tc.available))
			if tc.severity == "" {
				if len(got) != 0 {
					t.Fatalf("healthy capacity produced %v", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("diagnostics = %v, want one", got)
			}
			if got[0].Severity != tc.severity || got[0].Actual != tc.actual {
				t.Errorf("diagnostic = %+v, want severity %v and actual %v", got[0], tc.severity, tc.actual)
			}
			if got[0].Code != diagnostic.CodeDoctorDiskSpace || got[0].Path != "config" {
				t.Errorf("diagnostic lost identity: %+v", got[0])
			}
		})
	}
}
