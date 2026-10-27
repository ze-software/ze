package process

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// runnerGuardLines counts the runner guard's ERROR lines in the log ring.
func runnerGuardLines(t *testing.T) int {
	t.Helper()
	entries, err := slogutil.GlobalLogRing().Snapshot(0, "ERROR", "plugin")
	require.NoError(t, err)
	lines := 0
	for _, e := range entries {
		if strings.Contains(e.Message, "an internal Ze defect ended this unit") {
			lines++
		}
	}
	return lines
}

// TestInternalPluginRunnerPanicContained: an internal plugin whose engine
// panics ends through the runner's boundary as a plugin that returned does
// (its EngineDone closes), while a second internal plugin started beside it
// keeps running. The panic is counted on the real guard, logged and
// crash-recorded, where the recover it replaced only logged.
func TestInternalPluginRunnerPanicContained(t *testing.T) {
	t.Cleanup(crashlog.SetCrashDirForTest(t.TempDir()))
	beforeCount, declared := runnerPanics().guard.Panics(taskRun)
	require.True(t, declared)
	beforeLines := runnerGuardLines(t)

	registerRunEngine(t, "test-runner-panics", func(net.Conn) int {
		panic("boom: plugin engine defect")
	})
	registerRunEngine(t, "test-runner-healthy", func(conn net.Conn) int {
		drainUntilClosed(conn)
		return 0
	})

	pm := NewProcessManager([]plugin.PluginConfig{
		{Name: "test-runner-panics", Internal: true, Encoder: "json"},
		{Name: "test-runner-healthy", Internal: true, Encoder: "json"},
	})
	require.NoError(t, pm.Start())
	t.Cleanup(pm.Stop)

	select {
	case <-pm.GetProcess("test-runner-panics").EngineDone():
	case <-time.After(5 * time.Second):
		t.Fatal("the panicking internal plugin did not end")
	}
	require.True(t, pm.GetProcess("test-runner-healthy").Running(),
		"the other internal plugin stopped when one plugin panicked")

	afterCount, _ := runnerPanics().guard.Panics(taskRun)
	require.Equal(t, beforeCount+1, afterCount, "the panic was not counted")
	require.Equal(t, beforeLines+1, runnerGuardLines(t), "the panic was not logged")
	crashes := crashlog.ListCrashes()
	require.Len(t, crashes, 1)
	require.Equal(t, crashlog.KindRecovered, crashes[0].Kind)
}
