// Design: docs/architecture/api/process-protocol.md -- report ownership at process exit.
package server

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestRuntimeCleanupClearsReportRaisedAfterRemoval holds an actual report-issue
// request inside its rate admission, removes its owner, and then lets the
// request raise. Both RPC transports must reach the bus before runtime cleanup
// removes the warning. A later generation keeps its own warning when the old
// generation's cleanup is called again.
//
// The controlled clock makes the removal precede the late raise without a
// timing race. The runtime handler starts after the request finishes and takes
// its stopped/no-connection path. The separate runtime drain regressions cover
// waiting for admitted calls; this test covers the cleanup after that wait.
// A daemon fixture cannot hold this interleaving at report admission.
func TestRuntimeCleanupClearsReportRaisedAfterRemoval(t *testing.T) {
	for _, direct := range []bool{false, true} {
		name := "socket"
		if direct {
			name = "direct"
		}
		t.Run(name, func(t *testing.T) {
			s := reportTestServer(t)
			pm := process.NewProcessManager(nil)
			s.procManager.Store(pm)
			proc := reportingProcess("report-cleanup-" + name)
			pm.AddProcess(proc.Name(), proc)
			p := reportRPCClient(t, s, proc, direct)
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()

			entered := make(chan struct{})
			resume := make(chan struct{})
			var once sync.Once
			release := func() { once.Do(func() { close(resume) }) }
			var calls sync.WaitGroup
			defer calls.Wait()
			defer release()
			s.pluginReports.now = func() time.Time {
				close(entered)
				<-resume
				return time.Now()
			}
			raised := make(chan error, 1)
			calls.Go(func() {
				raised <- p.RaiseIssue(ctx, sdk.Issue{
					Severity: rpc.ReportWarning, Code: diagnostic.CodeDoctorInternalDefect,
					Subject: "late report", Message: "an admitted request completed after removal",
				})
			})
			receiveWithin(t, entered, 5*time.Second, "report-issue never entered admission")

			s.takeAndStopProcess(proc)
			if pm.GetProcess(proc.Name()) != nil {
				t.Fatal("removal kept the process in its manager slot")
			}
			release()
			if err := receiveWithin(t, raised, 5*time.Second, "the admitted report never finished"); err != nil {
				t.Fatalf("the admitted report was refused: %v", err)
			}
			if _, found := findWarning(proc.Name(), diagnostic.CodeDoctorInternalDefect, "late report"); !found {
				t.Fatal("the late report never reached the bus, so cleanup is not being tested")
			}

			// MUTATION: omitting the final report.ClearSource from cleanupProcess
			// leaves the positively observed late warning active here.
			s.handleSingleProcessCommandsRPC(proc)
			if err := proc.WaitRuntimeCleanupContext(ctx); err != nil {
				t.Fatalf("runtime cleanup did not finish: %v", err)
			}
			if _, found := findWarning(proc.Name(), diagnostic.CodeDoctorInternalDefect, "late report"); found {
				t.Error("runtime cleanup left the removed process's late warning active")
			}
			s.pluginReports.mu.Lock()
			_, window := s.pluginReports.windows[proc.Name()]
			s.pluginReports.now = nil
			s.pluginReports.mu.Unlock()
			if window {
				t.Error("runtime cleanup kept the removed process's rate window")
			}

			// Replacement startup is allowed only after the old runtime cleanup.
			// Repeating that cleanup must not clear the replacement's report.
			replacement := reportingProcess(proc.Name())
			pm.AddProcess(replacement.Name(), replacement)
			fresh := reportRPCClient(t, s, replacement, direct)
			if err := fresh.RaiseIssue(ctx, sdk.Issue{
				Severity: rpc.ReportWarning, Code: diagnostic.CodeDoctorInternalDefect,
				Subject: "new generation", Message: "the replacement owns this warning",
			}); err != nil {
				t.Fatalf("replacement report: %v", err)
			}
			s.cleanupProcess(proc)
			if _, found := findWarning(replacement.Name(), diagnostic.CodeDoctorInternalDefect, "new generation"); !found {
				t.Error("the old generation's repeated cleanup removed the replacement's warning")
			}
			if active, _, _ := report.SourceWarnings(replacement.Name(), diagnostic.CodeDoctorInternalDefect, "new generation"); active != 1 {
				t.Errorf("replacement owns %d active warnings, want its one new warning", active)
			}
		})
	}
}
