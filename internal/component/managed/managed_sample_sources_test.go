// VALIDATES: fleet-8 AC-1, AC-9, AC-11, AC-14 device-to-hub metric sampling
// pipeline. Exercises the real builtin host source (ze-host:show-fleet-source-host,
// with telemetry export disabled) alongside a forked plugin source over a real
// net.Pipe PluginConn, collected via CollectFleetSampleReportFromServer, sent
// by sessionSampler over authenticated TLS to a real ManagedServer, and queried
// back through ManagedHistoryLatest and ManagedHistorySeries. Also verifies
// absent optional owners supply no data (never zero), present owners preserve
// real measured zero, and standalone mode makes zero calls to FleetSampleMetrics.
// PREVENTS: broken builtin+forked discovery or process-boundary collection,
// absent optional features fabricating zero readings, or standalone daemons
// starting fleet sampling timers.

package managed

import (
	"context"
	"encoding/json"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	_ "github.com/ze-software/ze/internal/plugins/host-cmd/cmd"  // Link real builtin show fleet source host without telemetry.
	_ "github.com/ze-software/ze/internal/plugins/host-cmd/yang" // Link ze-host-cmd.yang so NewServer maps ze-host:show-fleet-source-host.
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func startForkedFleetSourcePlugin(
	t *testing.T,
	d *pluginserver.Dispatcher,
	procName string,
	cmdName string,
	answer func() fleet.SourceSnapshot,
) *atomic.Int32 {
	t.Helper()

	engineEnd, pluginEnd := net.Pipe()
	mux := rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd))
	proc := process.NewProcess(plugin.PluginConfig{Name: procName})
	proc.SetConn(pluginipc.NewMuxPluginConn(mux))
	proc.SetRunning(true)

	results := d.Registry().Register(proc, []pluginserver.CommandDef{{
		Name:   cmdName,
		Hidden: true,
	}})
	require.Len(t, results, 1)
	require.True(t, results[0].OK, results[0].Error)

	var calls atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn := rpc.NewConn(pluginEnd, pluginEnd)
		for {
			req, err := conn.ReadRequest(t.Context())
			if err != nil {
				return
			}
			calls.Add(1)
			payload, err := json.Marshal(answer())
			if err != nil {
				return
			}
			if err := rpc.WriteDocumentAnswer(conn.AnswerWriter(t.Context()), req.ID, rpc.AnswerTail{}, payload); err != nil {
				return
			}
		}
	}()

	t.Cleanup(func() {
		proc.SetRunning(false)
		_ = mux.Close()       //nolint:errcheck // test teardown
		_ = engineEnd.Close() //nolint:errcheck // test teardown
		_ = pluginEnd.Close() //nolint:errcheck // test teardown
		<-done
	})

	return &calls
}

// TestManagedSampleSourcesMixedBuiltinAndForkedEndToEnd proves that a device
// server with the real builtin host source and a forked plugin source over
// PluginConn collects both without telemetry enabled, pushes reports via
// sessionSampler to a real ManagedServer, preserves real measured zeros while
// omitting absent optional owners, and serves both latest and series queries.
//
// VALIDATES: AC-1, AC-11 mixed builtin/forked collection and absent-vs-zero.
// PREVENTS: forked plugin sources failing over PluginConn, hidden sources
// leaking into visible discovery, or absent owners appearing as zero.
func TestManagedSampleSourcesMixedBuiltinAndForkedEndToEnd(t *testing.T) {
	// 1. Device-side pluginserver.Server with real builtin host source (linked
	// via host-cmd/cmd) and a forked bgp-rib source over net.Pipe PluginConn.
	deviceServer := registryDispatcher(t)

	var forkedRoutes atomic.Int64
	forkedRoutes.Store(0) // Tick 1 reports a real measured zero.
	forkedCalls := startForkedFleetSourcePlugin(
		t,
		deviceServer.Dispatcher(),
		"bgp-rib-forked",
		"show fleet source bgp-rib",
		func() fleet.SourceSnapshot {
			return fleet.SourceSnapshot{
				Owner: fleet.SampleOwnerBGPRIB,
				Readings: []fleet.SourceReading{{
					Figure: fleet.FigureRoutesHeld,
					Unit:   fleet.SampleUnitCount,
					Value:  float64(forkedRoutes.Load()),
				}},
			}
		},
	)

	// Hidden forked source must stay out of operator-visible plugin discovery.
	assert.Empty(t, deviceServer.Dispatcher().Registry().VisibleCommandEntries())

	// 2. Hub-side ManagedServer with deterministic step clock (1-minute slots).
	base := time.Now().UTC().Truncate(time.Minute)
	clk := newTestStepClock(base)
	addr := startHistoryCommandsServer(t, clk, map[string]map[string]string{
		"edge-mixed": {"region": "west"},
	})
	mc := connectReportCommandClient(t, addr, "edge-mixed")

	// 3. Managed client sessionSampler wired to real CollectFleetSampleReportFromServer.
	// Gate tick 2 behind allowTick2 so tick 2 never races ahead of the
	// forkedRoutes update and clock advance below.
	var tickNum atomic.Int32
	allowTick2 := make(chan struct{})
	cfg := &ClientConfig{
		Name:               "edge-mixed",
		sampleIntervalUnit: 20 * time.Millisecond,
		sampleNow:          clk.Now,
		SampleCollector: func(ctx context.Context, now time.Time) (fleet.SampleReport, error) {
			if tickNum.Add(1) >= 2 {
				select {
				case <-allowTick2:
				case <-ctx.Done():
					return fleet.SampleReport{}, ctx.Err()
				}
			}
			return pluginserver.CollectFleetSampleReportFromServer(ctx, deviceServer, now)
		},
	}
	sampler := newSessionSampler(t.Context(), mc, cfg)
	t.Cleanup(sampler.Stop)
	sampler.UpdateInterval(1)

	allowAll := pluginserver.ResolveFleetDeviceScope(&pluginserver.CommandContext{})

	// Wait for tick 1 to reach the hub's ManagedHistoryStore.
	var firstRow pluginserver.HistoryLatestRow
	require.Eventually(t, func() bool {
		rows, err := pluginserver.ManagedHistoryLatest(allowAll, pluginserver.HistoryLatestFilter{Device: "edge-mixed"})
		if err != nil || len(rows) != 1 || !rows[0].HasReport {
			return false
		}
		firstRow = rows[0]
		return true
	}, 5*time.Second, 20*time.Millisecond, "first sample report did not reach hub")

	assert.GreaterOrEqual(t, forkedCalls.Load(), int32(1))
	assert.Equal(t, "ok", firstRow.Status)

	// Present builtin host figures arrived without telemetry export.
	for _, fig := range []string{
		fleet.FigureMemoryUsed,
		fleet.FigureMemoryTotal,
		fleet.FigureStorageUsed,
		fleet.FigureStorageTotal,
		fleet.FigureLoad1m,
		fleet.FigureUptime,
	} {
		_, present := firstRow.Figures[fig]
		assert.True(t, present, "real host builtin figure %q must be present", fig)
	}
	assert.Greater(t, firstRow.Figures[fleet.FigureMemoryTotal], 0.0)
	assert.Greater(t, firstRow.Figures[fleet.FigureStorageTotal], 0.0)

	// Present forked plugin figure preserved a real measured zero.
	routesVal, hasRoutes := firstRow.Figures[fleet.FigureRoutesHeld]
	assert.True(t, hasRoutes, "forked source measured zero routes-held must be present")
	assert.Equal(t, 0.0, routesVal)

	// Absent/compiled-out optional owners supplied NO data (never zero).
	for _, absentFig := range []string{
		fleet.FigureInterfacesUp,
		fleet.FigureInterfacesConfigured,
		fleet.FigureTrafficIn,
		fleet.FigureTrafficOut,
		fleet.FigureBGPPeersEstablished,
		fleet.FigureBGPPeersConfigured,
		fleet.FigureSubscriberSessions,
	} {
		_, present := firstRow.Figures[absentFig]
		assert.False(t, present, "absent optional figure %q must not appear in latest", absentFig)
	}

	// 4. Update forked plugin measurement to 42, advance hub clock by 1 minute,
	// then release tick 2 so it deterministically lands in the next UTC slot.
	forkedRoutes.Store(42)
	clk.Advance(time.Minute)
	close(allowTick2)

	require.Eventually(t, func() bool {
		rows, err := pluginserver.ManagedHistoryLatest(allowAll, pluginserver.HistoryLatestFilter{Device: "edge-mixed"})
		if err != nil || len(rows) != 1 || !rows[0].HasReport {
			return false
		}
		v, ok := rows[0].Figures[fleet.FigureRoutesHeld]
		return ok && v == 42
	}, 5*time.Second, 20*time.Millisecond, "second sample report with routes-held=42 did not reach hub")

	sampler.Stop()

	// 5. Verify ManagedHistorySeries records both ticks (0 then 42) and leaves
	// absent optional figures as explicit gaps with nil values.
	routesSeries, err := pluginserver.ManagedHistorySeries(allowAll, pluginserver.HistorySeriesQuery{
		Device: "edge-mixed",
		Figure: fleet.FigureRoutesHeld,
		From:   base,
		Until:  base.Add(2 * time.Minute),
	})
	require.NoError(t, err)
	require.Len(t, routesSeries, 2)
	assert.False(t, routesSeries[0].Gap)
	require.NotNil(t, routesSeries[0].Value)
	assert.Equal(t, 0.0, *routesSeries[0].Value)
	assert.False(t, routesSeries[1].Gap)
	require.NotNil(t, routesSeries[1].Value)
	assert.Equal(t, 42.0, *routesSeries[1].Value)

	memSeries, err := pluginserver.ManagedHistorySeries(allowAll, pluginserver.HistorySeriesQuery{
		Device: "edge-mixed",
		Figure: fleet.FigureMemoryUsed,
		From:   base,
		Until:  base.Add(2 * time.Minute),
	})
	require.NoError(t, err)
	require.Len(t, memSeries, 2)
	assert.False(t, memSeries[0].Gap)
	assert.False(t, memSeries[1].Gap)

	absentSeries, err := pluginserver.ManagedHistorySeries(allowAll, pluginserver.HistorySeriesQuery{
		Device: "edge-mixed",
		Figure: fleet.FigureSubscriberSessions,
		From:   base,
		Until:  base.Add(2 * time.Minute),
	})
	require.NoError(t, err)
	for _, bucket := range absentSeries {
		assert.True(t, bucket.Gap, "absent figure series bucket must be a gap")
		assert.Nil(t, bucket.Value, "absent figure series bucket must not have a zero value")
	}
}

// TestStandaloneModeZeroFleetSampleCalls proves that a standalone daemon
// answering ordinary local commands (including show fleet status) performs zero
// fleet metric sampling calls and leaves FleetSampleMetrics completely unchanged.
//
// VALIDATES: AC-9 standalone mode zero calls to fleet sampling.
// PREVENTS: standalone startup activating timers or invoking sample collectors.
func TestStandaloneModeZeroFleetSampleCalls(t *testing.T) {
	// Not parallel: reads process-wide FleetSampleMetrics deltas and ze.config.dir.
	fleetStatusStore(t, nil, nil)
	before := FleetSampleMetrics()

	standaloneServer := registryDispatcher(t)
	resp, err := standaloneServer.Dispatcher().Dispatch(
		&pluginserver.CommandContext{Server: standaloneServer},
		"show fleet status",
	)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, plugin.StatusDone, resp.Status, "%s", resp.Error)

	raw, err := plugin.ResponseJSON(resp, nil)
	require.NoError(t, err)
	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &status))
	assert.Equal(t, false, status["managed"])
	assert.Equal(t, false, status["hub-declared"])

	after := FleetSampleMetrics()
	assert.Equal(t, before, after, "standalone device reads must make zero fleet sampling calls")
}
