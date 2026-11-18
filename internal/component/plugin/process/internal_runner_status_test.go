// Design: docs/architecture/api/process-protocol.md -- internal runner exit and panic containment.
// Related: process.go -- startInternal records the returned status before its panic fallback.
package process

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/crashlog"
)

// TestInternalRunnerPreservesReturnedExitStatus checks real runner completion
// records both clean and failed exits without invoking the panic boundary.
func TestInternalRunnerPreservesReturnedExitStatus(t *testing.T) {
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
	for _, exitCode := range []int{0, 7} {
		t.Run(strconv.Itoa(exitCode), func(t *testing.T) {
			name := "test-runner-returned-status-" + strconv.Itoa(exitCode)
			registerRunEngine(t, name, func(net.Conn) int { return exitCode })
			before, declared := runnerPanics().guard.Panics(taskRun)
			if !declared {
				t.Fatal("internal runner panic task is not declared")
			}
			proc := NewProcess(plugin.PluginConfig{Name: name, Internal: true, Encoder: "json"})
			if err := proc.StartWithContext(t.Context()); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				proc.Stop()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := proc.Wait(ctx); err != nil {
					t.Error("returned internal runner did not join:", err)
				}
			})
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			if err := proc.WaitEngine(ctx); err != nil {
				t.Fatal("internal runner did not finish:", err)
			}
			after, _ := runnerPanics().guard.Panics(taskRun)
			if after != before {
				t.Fatalf("runner returning %d caused %d contained panics", exitCode, after-before)
			}
			want := ExitedWith("exit code " + strconv.Itoa(exitCode))
			if got := proc.ExitStatus(); got != want {
				t.Fatalf("returned exit status = %v, want %v", got, want)
			}
		})
	}
}
