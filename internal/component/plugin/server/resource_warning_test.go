// Design: docs/architecture/api/process-protocol.md -- retained resource problems
package server

import (
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// Goal: ordinary report-issue cannot acknowledge deleting or replacing a
// retained resource failure. Method: exercise the actual SDK route over both
// transports, retain the original row, and positively clear an unrelated row.
func TestResourceWarningReportIssueRefusesOrdinaryMutationBothTransports(t *testing.T) {
	for _, direct := range []bool{false, true} {
		name := "wire"
		if direct {
			name = "bridge"
		}
		t.Run(name, func(t *testing.T) {
			server := reportTestServer(t)
			process := reportingProcess("resource-report-" + name)
			client := reportRPCClient(t, server, process, direct)
			code := diagnostic.CodeDoctorInternalDefect
			owner, err := report.NewResourceWarning(process.Name(), "native-original-episode")
			if err != nil {
				t.Fatal(err)
			}
			episode, err := owner.NewEpisode(code, "original retained failure", "request class-of-service cleanup")
			if err != nil {
				t.Fatal(err)
			}
			if err := episode.Publish(); err != nil {
				t.Fatal(err)
			}
			if err := client.ClearIssue(t.Context(), code, "native-original-episode"); err == nil {
				t.Fatal("ordinary SDK clear falsely acknowledged resource clearance")
			}
			if err := client.RaiseIssue(t.Context(), sdk.Issue{Severity: rpc.ReportWarning, Code: code, Subject: "native-original-episode", Message: "replacement guessed cause", Fix: "replacement fix"}); err == nil {
				t.Fatal("ordinary SDK overwrite falsely acknowledged resource mutation")
			}
			if err := client.RaiseIssue(t.Context(), sdk.Issue{Severity: rpc.ReportWarning, Code: code, Subject: "unrelated", Message: "ordinary failure", Fix: "show warnings"}); err != nil {
				t.Fatal(err)
			}
			if err := client.ClearIssue(t.Context(), code, "unrelated"); err != nil {
				t.Fatal(err)
			}
			report.ClearSource(process.Name())
			entries := report.Warnings()
			if len(entries) != 1 {
				t.Fatalf("ordinary cleanup left %d entries, want exact resource issue", len(entries))
			}
			if entries[0].Message != "original retained failure" {
				t.Fatal("ordinary route replaced actual original cause")
			}
			if entries[0].Fix != "request class-of-service cleanup" {
				t.Fatal("ordinary route replaced original repair command")
			}
			if err := episode.Clear(); err != nil {
				t.Fatal(err)
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
