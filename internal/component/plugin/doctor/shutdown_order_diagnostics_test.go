// Design: docs/architecture/doctor-and-health-checks.md -- plugin lifecycle checks
// Related: check_lifecycle_local.go -- the persisted shutdown-order failure row
package doctor

import (
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestShutdownOrderDoctorRowNamesTheDistinctFailure proves local doctor reads
// the new persisted ordering reason without falsely calling it late cleanup.
// MUTATION: remove LastStopShutdownOrder from fatalStopRows.
func TestShutdownOrderDoctorRowNamesTheDistinctFailure(t *testing.T) {
	at := time.Date(2026, 11, 17, 14, 0, 0, 0, time.UTC)
	rows := fatalStopRows(&server.LastStop{Entries: []server.LastStopEntry{{
		Reason: server.LastStopShutdownOrder, Detail: "selected dependency graph contains a cycle",
		Fix: "journalctl -u ze", Time: at,
	}}})
	if len(rows) != 1 {
		t.Fatalf("rows = %+v", rows)
	}
	row := rows[0]
	if row.Code != diagnostic.CodeDoctorPluginShutdownOrder {
		t.Fatalf("row code = %s", row.Code)
	}
	if row.Severity != diagnostic.SeverityWarning {
		t.Fatalf("row severity = %v", row.Severity)
	}
	for _, want := range []string{"2026-11-17T14:00:00Z", "dependency order", "contains a cycle"} {
		if !strings.Contains(row.Message, want) {
			t.Fatalf("row %q lacks %q", row.Message, want)
		}
	}
	if row.Fix != "journalctl -u ze" {
		t.Fatalf("fix = %q", row.Fix)
	}
	if strings.Contains(row.Message, "went on before") {
		t.Fatalf("ordering failure misreported as late cleanup: %q", row.Message)
	}
	if strings.Contains(row.Message, "state it installs may remain") {
		t.Fatalf("ordering failure claims installed state remains: %q", row.Message)
	}
}
