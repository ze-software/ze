// Design: docs/architecture/doctor-and-health-checks.md -- event codes, doctor-plugin-group-signal-failed
// Related: process.go -- pluginGroup.signalLocked, the process-group kill path under test
// Related: doctor_codes.go -- CodeDoctorPluginGroupSignalFailed metadata registration

package process

import (
	"bytes"
	"log/slog"
	"sync"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/report"
)

type lockedLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *lockedLogBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf.Reset()
}

func captureProcessWarnLogs(t *testing.T) *lockedLogBuffer {
	t.Helper()
	buf := &lockedLogBuffer{}
	orig := logger
	captured := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	logger = func() *slog.Logger { return captured }
	t.Cleanup(func() { logger = orig })
	return buf
}

func groupSignalEvents() []report.Issue {
	var events []report.Issue
	for _, issue := range report.Errors(0) {
		if issue.Code == CodeDoctorPluginGroupSignalFailed {
			events = append(events, issue)
		}
	}
	return events
}

// TestPluginGroupSignalFailureLogsWarnAndRaisesDoctorCode drives pluginGroup.kill
// against both syscall.ESRCH and a non-ESRCH kernel error (syscall.EPERM).
//
// VALIDATES: a non-ESRCH failure signaling a forked plugin's process group logs
// at WARN with CodeDoctorPluginGroupSignalFailed naming the plugin and cause and
// raises a matching report-bus event with a fix, while ESRCH (group already gone)
// emits zero WARN lines and zero report-bus events.
// PREVENTS: an EPERM or other signal failure dropping below WARN while child
// processes of a stopped plugin remain running.
func TestPluginGroupSignalFailureLogsWarnAndRaisesDoctorCode(t *testing.T) {
	const pluginName = "test-signal-plugin"

	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	logs := captureProcessWarnLogs(t)

	origKill := killProcessGroup
	t.Cleanup(func() { killProcessGroup = origKill })

	t.Run("ESRCH stays silent", func(t *testing.T) {
		report.ResetForTest()
		logs.Reset()
		killProcessGroup = func(int) error { return syscall.ESRCH }

		group := &pluginGroup{name: pluginName, pid: 4242}
		group.kill()

		require.Empty(t, logs.String(), "ESRCH on an already-exited group must not emit a WARN log")
		require.Empty(t, groupSignalEvents(), "ESRCH on an already-exited group must not raise a report event")
	})

	t.Run("non-ESRCH warns and raises doctor code", func(t *testing.T) {
		report.ResetForTest()
		logs.Reset()
		killProcessGroup = func(int) error { return syscall.EPERM }

		group := &pluginGroup{name: pluginName, pid: 4242}
		group.kill()

		gotLog := logs.String()
		require.Contains(t, gotLog, "level=WARN", "non-ESRCH signal failure must log at WARN")
		require.Contains(t, gotLog, "code="+CodeDoctorPluginGroupSignalFailed, "WARN log must carry doctor code")
		require.Contains(t, gotLog, pluginName, "WARN log must name the plugin")
		require.Contains(t, gotLog, syscall.EPERM.Error(), "WARN log must include the kernel error")

		events := groupSignalEvents()
		require.Len(t, events, 1, "non-ESRCH signal failure must raise one report-bus event")
		require.Equal(t, pluginName, events[0].Subject, "report event must name the plugin as subject")
		require.Contains(t, events[0].Message, pluginName, "report event message must name the plugin")
		require.Contains(t, events[0].Message, syscall.EPERM.Error(), "report event message must name the cause")
		require.NotEmpty(t, events[0].Fix, "report event must carry a fix command")
	})
}
