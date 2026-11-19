//go:build linux

// Design: docs/architecture/vpp-host-tuning.md -- explicit base-page ownership
// Authored before implementation Source. Runs on MAIN's actual Ze kernel route.
package vpp

import (
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

func TestManagedStartupBasePagesDoNotRequireHugepageReservation(t *testing.T) {
	tree := managedStartupTree(t, map[string]any{"enabled": "true", "memory": map[string]any{"hugepage-size": "default"}})
	params, enabled := hugepageParamsFromTree(tree)
	if !enabled || params.pageSize != "default" {
		t.Fatal("delivered tree did not select explicit base-page mode", params, enabled)
	}
	params.roots = hugepageDoctorRoots{sysfs: filepath.Join(t.TempDir(), "absent")}
	if diags := evaluateVPPHugepages(params); len(diags) != 0 {
		t.Fatal("explicit base pages borrowed a hugepage reservation", diags)
	}
	for _, page := range []string{"2M", "1G"} {
		params.pageSize = page
		diags := evaluateVPPHugepages(params)
		if len(diags) != 1 || diags[0].Severity != diagnostic.SeverityError {
			t.Fatal("explicit hugepages silently fell back to base pages", page, diags)
		}
	}
	params.pageSize = "unrecognized"
	if diags := evaluateVPPHugepages(params); len(diags) != 1 || diags[0].Severity != diagnostic.SeverityError {
		t.Fatal("unknown page mode impersonated the explicit base-page branch", diags)
	}
}
