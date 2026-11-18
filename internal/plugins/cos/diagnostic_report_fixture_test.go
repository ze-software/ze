// Design: docs/architecture/traffic/cos-dynamic.md -- explicit unit publisher seam
// Related: shared_access_diagnostic_test.go -- the registered daemon publisher

//go:build ze_l2tp

package cos

import (
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// This callback acknowledges an observed report DTO; it grants no native or
// diagnostic custody. Even pure policy fixtures reserve and publish through
// the actual ResourceWarning episode, so their report store is isolated by
// resetCoSReportsForTest. Registered report questions inspect the real bus.
func discardCoSReportForTest(rpc.ReportIssueInput) error { return nil }

// resetCoSReportsForTest isolates the real diagnostic reservations introduced
// by these fixtures. Tests are not parallel; a fixture's explicit restart
// transfers occur between this setup and its final cleanup. Reset happens
// only after its assertions and original native-target cleanup, never as a
// product recovery result or native release receipt.
func resetCoSReportsForTest(t *testing.T) {
	t.Helper()
	if backend := iface.GetBackend(); backend != nil {
		t.Fatalf("refuse to reset CoS fixture diagnostics while an interface backend is selected: %T", backend)
	}
	report.ResetForTest()
	t.Cleanup(func() {
		if backend := iface.GetBackend(); backend != nil {
			t.Errorf("retain CoS diagnostics after unfinished interface backend cleanup: %T", backend)
			return
		}
		report.ResetForTest()
	})
}
