// Design: docs/architecture/doctor-and-health-checks.md -- event codes, doctor-plugin-stop-unfinished
// Related: doctor_records.go -- RecordStopUnfinished, the raise under test

package process

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// stopUnfinishedEvents answers the doctor-plugin-stop-unfinished error events
// on the report bus, which the recent-events doctor check renders.
func stopUnfinishedEvents() []report.Issue {
	var events []report.Issue
	for _, issue := range report.Errors(0) {
		if issue.Code == diagnostic.CodeDoctorPluginStopUnfinished {
			events = append(events, issue)
		}
	}
	return events
}

// TestRecordStopUnfinishedRaisesTheEvent proves a stop whose join expired
// reaches the report bus as one doctor-plugin-stop-unfinished error event
// from the plugin source, subject the plugin, with the text and the fix
// stopUnfinishedText answers. No fork: the forked shape is a Process whose
// config is not Internal with Running set by hand, which is all the text reads.
func TestRecordStopUnfinishedRaisesTheEvent(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	proc := NewProcess(plugin.PluginConfig{Name: "test-stop-unfinished"})
	proc.running.Store(true)
	wantMessage, wantFix := stopUnfinishedText(proc, 2*time.Second)

	RecordStopUnfinished(proc, 2*time.Second, context.DeadlineExceeded)

	events := stopUnfinishedEvents()
	require.Len(t, events, 1, "the expired join is raised once")
	require.Equal(t, reportSourcePlugin, events[0].Source)
	require.Equal(t, "test-stop-unfinished", events[0].Subject)
	require.Equal(t, wantMessage, events[0].Message)
	require.Equal(t, wantFix, events[0].Fix)
}

// TestStopUnfinishedTextNamesWhatWasUnfinished builds the text for each of
// the four states an expired join can find the stopped process in: it names
// the plugin and the bound, says ze removed the process and takes no further
// step, claims a kill only for a forked process that had not exited, and its
// fix is a ze command that names no pid.
func TestStopUnfinishedTextNamesWhatWasUnfinished(t *testing.T) {
	cases := []struct {
		name     string
		internal bool
		running  bool
		says     string
		killed   bool
		fix      string
		bound    time.Duration
	}{
		{"engine in the daemon had not returned", true, true,
			"its engine had not ended: that engine runs in the daemon and had not returned", false,
			restartOverlapGoroutinesFix, time.Second},
		{"engine returned, delivery unfinished", true, false,
			"its engine had returned, but its event delivery had not finished", false,
			restartOverlapGoroutinesFix, time.Second},
		{"forked process had not exited", false, true,
			"its process had not exited: ze closed its connection and sent its process group the kill", true,
			restartOverlapRelayLogFix, 2 * time.Second},
		{"forked process exited, delivery or relay unfinished", false, false,
			"its process had exited, but ze had not finished its event delivery or reading its stderr", false,
			restartOverlapRelayLogFix, 2 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proc := NewProcess(plugin.PluginConfig{Name: "test-stop-text", Internal: tc.internal})
			proc.running.Store(tc.running)

			message, fix := stopUnfinishedText(proc, tc.bound)

			require.Contains(t, message, "plugin test-stop-text was stopped at boot")
			require.Contains(t, message, tc.says)
			require.Contains(t, message, tc.bound.String()+" after ze stopped it")
			require.Contains(t, message, "ze removed it from the plugins it runs and takes no further step to end")
			if tc.killed {
				require.Contains(t, message, "so it may still be running")
			} else {
				require.NotContains(t, message, "the kill",
					"a kill is claimed for a process that had exited or that runs in the daemon")
			}
			require.Equal(t, tc.fix, fix)
			require.NotContains(t, message, "pgrep")
		})
	}
}
