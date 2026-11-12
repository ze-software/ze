// Design: docs/architecture/doctor-and-health-checks.md -- event codes, doctor-plugin-restart-overlap
// Related: manager.go -- Respawn, the join this test expires
// Related: doctor_records.go -- recordRestartOverlap, the raise under test

package process

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// restartOverlapEvents answers the doctor-plugin-restart-overlap error
// events on the report bus, which the recent-events doctor check renders.
func restartOverlapEvents() []report.Issue {
	var events []report.Issue
	for _, issue := range report.Errors(0) {
		if issue.Code == diagnostic.CodeDoctorPluginRestartOverlap {
			events = append(events, issue)
		}
	}
	return events
}

// TestRespawnRecordsAnExpiredJoin respawns one internal plugin whose engine
// returns once its connection closes, then one whose engine never does.
//
// VALIDATES: Respawn's join of the process it replaces raises
// doctor-plugin-restart-overlap, subject the plugin, with the goroutine dump as
// its fix, when the join expires (manager.go, the `if joinErr != nil` after
// proc.Wait); a process that ends inside the bound raises nothing.
// PREVENTS: an expired join discarded with `_ =`, so a replacement started
// beside a process still running reached no log line and no doctor row.
// METHOD: the respawnJoinMargin seam at 500ms, the margin
// TestStopKeepsTheEnginesThatMissedTheirGrace uses, wide enough that the
// prompt engine ends inside it on a loaded machine. An internal plugin has no
// exit grace, so the margin is the whole bound. The stuck engine is released
// and joined before the test ends.
func TestRespawnRecordsAnExpiredJoin(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)

	stuck := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(stuck) }) }
	t.Cleanup(release)
	registerRunEngine(t, "test-restart-overlap-stuck", func(conn net.Conn) int {
		drainUntilClosed(conn)
		<-stuck
		return 0
	})
	registerRunEngine(t, "test-restart-overlap-prompt", func(conn net.Conn) int {
		drainUntilClosed(conn)
		return 0
	})
	origMargin := respawnJoinMargin
	respawnJoinMargin = 500 * time.Millisecond
	t.Cleanup(func() { respawnJoinMargin = origMargin })

	pm := NewProcessManager([]plugin.PluginConfig{
		{Name: "test-restart-overlap-stuck", Internal: true, Encoder: "json"},
		{Name: "test-restart-overlap-prompt", Internal: true, Encoder: "json"},
	})
	require.NoError(t, pm.Start())

	_, err := pm.Respawn("test-restart-overlap-prompt")
	require.NoError(t, err)
	require.Empty(t, restartOverlapEvents(), "a process that ended inside the bound raised an overlap")

	previous := pm.GetProcess("test-restart-overlap-stuck")
	require.NotNil(t, previous)
	_, err = pm.Respawn("test-restart-overlap-stuck")
	require.NoError(t, err)

	events := restartOverlapEvents()
	require.Len(t, events, 1, "the expired join did not raise exactly one overlap")
	require.Equal(t, "test-restart-overlap-stuck", events[0].Subject, "the event does not name the plugin")
	require.Equal(t, restartOverlapGoroutinesFix, events[0].Fix,
		"an in-daemon engine's overlap does not name the goroutine dump as its fix")
	require.Contains(t, events[0].Message, "was restarted before its previous engine ended")

	release()
	joinCtx, joinCancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer joinCancel()
	require.NoError(t, previous.Wait(joinCtx), "the released engine of the replaced process did not end")
	pm.Stop()
}
