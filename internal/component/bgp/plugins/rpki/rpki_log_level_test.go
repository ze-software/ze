package rpki

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestRPKILogLevelSurvivesPanicGuard drives the plugin's logger the way the engine does: the
// in-process runner hands the plugin slogutil.Logger("bgp.rpki") (ConfigureEngineLogger), then
// builds the panic boundary (ConfigureMetrics reaches rpkiPanics), then the operator runs
// `request log level bgp.rpki <level>`, which is slogutil.SetLevel.
//
// VALIDATES: a runtime level change reaches the logger every RPKI log line goes through, so
// "rtr: sync complete" is logged at info once the operator asks for info.
// PREVENTS: the panic guard calling slogutil.Logger("bgp.rpki") a second time, which registered a
// new level for the subsystem and left the plugin's logger at its startup level: the RPKI terminal
// demo then never saw "rtr: sync complete" after the cache came back.
func TestRPKILogLevelSurvivesPanicGuard(t *testing.T) {
	previous := logger()
	t.Cleanup(func() { setLogger(previous) })

	setLogger(slogutil.Logger("bgp.rpki"))
	guardLog := slog.New(pluginLogHandler{})
	_ = newRPKIPanicTasks()

	ctx := context.Background()
	require.NoError(t, slogutil.SetLevel("bgp.rpki", "error"))
	require.False(t, logger().Enabled(ctx, slog.LevelInfo), "level error still logs info")

	require.NoError(t, slogutil.SetLevel("bgp.rpki", "info"))
	require.True(t, logger().Enabled(ctx, slog.LevelInfo),
		"request log level bgp.rpki info did not reach the plugin's logger")
	require.True(t, guardLog.Enabled(ctx, slog.LevelInfo),
		"the panic guard's logger does not follow the plugin's level")
}
