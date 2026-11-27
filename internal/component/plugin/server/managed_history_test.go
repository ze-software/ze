// VALIDATES: hub-side fleet metric history ingestion, dual-resolution fine/coarse
// rings with packed minute contributions, aligned set maxima, UTC-slot admission
// and backwards clock-step guards, offline gap preservation, checkpoint/restart
// durability, deletion fencing, per-device/hub capacity caps, reload validation,
// doctor reporting, and 2,000/4,096 estate sizing (AC-1..AC-8, AC-11..AC-15).
// PREVENTS: double-counting across the 1-day boundary, naive peak overcounting,
// rate-limit bypass across reconnects, resurrecting forgotten devices from late
// flushes, unbounded output or storage growth, and preallocating offline rings.

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math/rand/v2"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/storage"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type historyFailStore struct {
	storage.Storage
	mu        sync.Mutex
	failWrite func(key string) error
	failDel   func(key string) error
}

func newHistoryFailStore() *historyFailStore {
	return &historyFailStore{Storage: storage.CreateMemory()}
}

func (f *historyFailStore) WriteKey(key string, data []byte) error {
	f.mu.Lock()
	fn := f.failWrite
	f.mu.Unlock()
	if fn != nil {
		if err := fn(key); err != nil {
			return err
		}
	}
	return f.Storage.WriteKey(key, data)
}

func (f *historyFailStore) RemoveKey(key string) error {
	f.mu.Lock()
	fn := f.failDel
	f.mu.Unlock()
	if fn != nil {
		if err := fn(key); err != nil {
			return err
		}
	}
	return f.Storage.RemoveKey(key)
}

func allowAllFleetScope() FleetDeviceScope {
	return FleetDeviceScope{mode: fleetScopeAllowAll}
}

type allowSetScopeAuth struct {
	allowed map[string]bool
}

func (a allowSetScopeAuth) AuthorizeLocalScope(_, _ string, args []string, _ bool) bool {
	if len(args) == 2 && args[0] == "name" {
		return a.allowed[args[1]]
	}
	return false
}

func allowDevicesFleetScope(names ...string) FleetDeviceScope {
	allowed := make(map[string]bool, len(names))
	for _, n := range names {
		allowed[n] = true
	}
	return FleetDeviceScope{
		mode:      fleetScopeEvaluate,
		username:  "operator",
		scopeAuth: allowSetScopeAuth{allowed: allowed},
	}
}

func startHistoryTestServer(t *testing.T, store storage.Storage, clk *reportTestClock, historyCfg plugin.HubHistoryConfig, clients map[string]string, labels map[string]map[string]string) (*ManagedServer, string) {
	t.Helper()
	srv, err := NewManagedServer(ManagedServerConfig{
		Authority: newTestCA(t),
		Store:     store,
		History:   historyCfg,
		Listeners: []ManagedListener{{
			Addr:         "127.0.0.1:0",
			Clients:      clients,
			ClientLabels: labels,
		}},
		ReadConfig: func(name string) ([]byte, error) {
			if _, ok := clients[name]; ok {
				return []byte(testClientConfig), nil
			}
			return nil, ErrClientConfigNotFound
		},
	})
	require.NoError(t, err)
	if clk != nil {
		srv.now = clk.Now
	}
	require.NoError(t, srv.Start(context.Background()))
	t.Cleanup(srv.Stop)
	addrs := srv.Addrs()
	require.Len(t, addrs, 1)
	return srv, addrs[0].String()
}

func makeSampleReport(deviceTime string, samples ...fleet.SampleReading) fleet.SampleReport {
	return fleet.SampleReport{
		SchemaVersion: fleet.SampleSchemaVersion,
		DeviceTime:    deviceTime,
		Samples:       samples,
	}
}

// TestManagedHistoryIngestLatestAndZeroVsAbsent validates AC-1 and AC-11:
// config-fetch interval advertisement on full and current replies, sample-report
// ingestion, measured zero vs absent figure distinction, unreported vs
// unsupported devices, ranking, reduction, and caller device visibility scope.
func TestManagedHistoryIngestLatestAndZeroVsAbsent(t *testing.T) {
	t0 := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	clk := newReportTestClock(t0)
	store := storage.CreateMemory()
	clients := map[string]string{
		"edge-01": testClientSecret,
		"edge-02": testClient2Secret,
		"edge-03": "edge03-secret-that-is-at-least-32ch",
	}
	labels := map[string]map[string]string{
		"edge-01": {"region": "west", "role": "edge"},
		"edge-02": {"region": "west", "role": "edge"},
		"edge-03": {"region": "east", "role": "core"},
	}
	srv, addr := startHistoryTestServer(t, store, clk, plugin.DefaultHubHistoryConfig(), clients, labels)

	mc1 := dialTestClient(t, addr, "edge-01", testClientSecret)

	// Both full and current config-fetch replies advertise SampleIntervalSeconds.
	fetchFull := fetch(t, mc1, "")
	require.Equal(t, fleet.DefaultSampleIntervalSeconds, fetchFull.SampleIntervalSeconds)
	fetchCurrent := fetch(t, mc1, fetchFull.Version)
	require.Equal(t, "current", fetchCurrent.Status)
	require.Equal(t, fleet.DefaultSampleIntervalSeconds, fetchCurrent.SampleIntervalSeconds)

	// edge-01 reports measured zero cpu-used and memory figures, but NO BGP figures.
	rep1 := makeSampleReport(
		t0.Format(time.RFC3339),
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 0},
		fleet.SampleReading{Figure: fleet.FigureMemoryUsed, Value: 512},
		fleet.SampleReading{Figure: fleet.FigureMemoryTotal, Value: 1024},
	)
	res, err := callReportRPC(t, mc1, fleet.VerbSampleReport, rep1)
	require.NoError(t, err)
	require.Equal(t, json.RawMessage(`{}`), res)

	// Mark edge-03 explicitly unsupported; leave edge-02 never reported.
	srv.History().MarkUnsupported("edge-03", t0)

	clk.Advance(45 * time.Second)
	rows, err := srv.HistoryLatest(allowAllFleetScope(), HistoryLatestFilter{})
	require.NoError(t, err)
	require.Len(t, rows, 3)

	require.Equal(t, "edge-01", rows[0].Device)
	require.True(t, rows[0].HasReport)
	require.Equal(t, historyStatusOK, rows[0].Status)
	require.Equal(t, int64(45), rows[0].AgeSeconds)
	cpuVal, hasCPU := rows[0].Figures[fleet.FigureCPUUsed]
	require.True(t, hasCPU, "measured zero must stay present")
	require.Zero(t, cpuVal)
	_, hasBGP := rows[0].Figures[fleet.FigureBGPPeersEstablished]
	require.False(t, hasBGP, "unreported owner figure must stay absent")

	require.Equal(t, "edge-02", rows[1].Device)
	require.False(t, rows[1].HasReport)
	require.False(t, rows[1].Unsupported)
	require.Equal(t, historyStatusNoReports, rows[1].Status)

	require.Equal(t, "edge-03", rows[2].Device)
	require.False(t, rows[2].HasReport)
	require.True(t, rows[2].Unsupported)
	require.Equal(t, historyStatusUnsupported, rows[2].Status)

	// Caller scope hides unauthorized devices before reduction or ranking.
	scopedRows, err := srv.HistoryLatest(allowDevicesFleetScope("edge-01"), HistoryLatestFilter{})
	require.NoError(t, err)
	require.Len(t, scopedRows, 1)
	require.Equal(t, "edge-01", scopedRows[0].Device)

	// Denied device and absent device yield the exact same empty answer.
	deniedRows, err := srv.HistoryLatest(allowDevicesFleetScope("edge-01"), HistoryLatestFilter{Device: "edge-02"})
	require.NoError(t, err)
	require.Empty(t, deniedRows)
	absentRows, err := srv.HistoryLatest(allowDevicesFleetScope("edge-01"), HistoryLatestFilter{Device: "no-such-device"})
	require.NoError(t, err)
	require.Empty(t, absentRows)

	// Reduce latest across authorized west devices: edge-01 reported, edge-02 missing.
	westRows, err := srv.HistoryLatest(allowAllFleetScope(), HistoryLatestFilter{Labels: map[string]string{"region": "west"}})
	require.NoError(t, err)
	require.Len(t, westRows, 2)
	reduced, err := ReduceLatestRows(westRows, fleet.FigureMemoryUsed)
	require.NoError(t, err)
	require.Len(t, reduced, 1)
	require.Equal(t, 1, reduced[0].Contributors)
	require.Equal(t, 1, reduced[0].Missing)
	require.NotNil(t, reduced[0].Value)
	require.InDelta(t, 512.0, *reduced[0].Value, 1e-9)
	require.NotNil(t, reduced[0].Ratio)
	require.InDelta(t, 0.5, *reduced[0].Ratio, 1e-9)

	// Rank drops rows missing the figure and breaks ties ascending by device name.
	ranked, err := RankLatestRows(rows, fleet.FigureCPUUsed)
	require.NoError(t, err)
	require.Len(t, ranked, 1)
	require.Equal(t, "edge-01", ranked[0].Device)
}

// TestManagedHistoryDualResolutionBoundaryAndAlignedMaxima validates AC-2 and
// AC-14: fine 1m + coarse 15m rings, exact 1-day boundary transition without
// overlap or double-counting, aligned estate peaks from packed minute
// contributions, weighted means, summable counters, paired capacity ratios, and
// points rebucketing.
func TestManagedHistoryDualResolutionBoundaryAndAlignedMaxima(t *testing.T) {
	// Align query time to a clean UTC hour boundary.
	end := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	start := end.Add(-48 * time.Hour)
	h := NewManagedHistoryStore(ManagedHistoryStoreConfig{
		Config:          plugin.DefaultHubHistoryConfig(),
		Now:             func() time.Time { return end },
		DeclaredClients: 3,
	})
	meta := map[string]HistoryDeviceMeta{
		"edge-01": {Name: "edge-01", Declared: true, Connected: true},
		"edge-02": {Name: "edge-02", Declared: true, Connected: true},
		"edge-03": {Name: "edge-03", Declared: true, Connected: true},
	}

	// Pick one specific coarse 15-minute window 30 hours ago (strictly in coarse range):
	// edge-01 peaks at minute offset 2, edge-02 peaks at minute offset 7.
	peakSlotStart := end.Add(-30 * time.Hour)
	for m := range 48 * 60 {
		ts := start.Add(time.Duration(m) * time.Minute)
		cpu1, cpu2 := 10.0, 10.0
		tr1, tr2 := 100.0, 100.0
		if !ts.Before(peakSlotStart) && ts.Before(peakSlotStart.Add(15*time.Minute)) {
			off := int(ts.Sub(peakSlotStart) / time.Minute)
			if off == 2 {
				cpu1 = 90.0
				tr1 = 1000.0
			}
			if off == 7 {
				cpu2 = 90.0
				tr2 = 1000.0
			}
		}
		require.NoError(t, h.Ingest("edge-01", &fleet.SampleReport{
			SchemaVersion: fleet.SampleSchemaVersion,
			Samples: []fleet.SampleReading{
				{Figure: fleet.FigureCPUUsed, Value: cpu1},
				{Figure: fleet.FigureTrafficIn, Value: tr1},
				{Figure: fleet.FigureMemoryUsed, Value: 400},
				{Figure: fleet.FigureMemoryTotal, Value: 1000},
			},
		}, ts))
		require.NoError(t, h.Ingest("edge-02", &fleet.SampleReport{
			SchemaVersion: fleet.SampleSchemaVersion,
			Samples: []fleet.SampleReading{
				{Figure: fleet.FigureCPUUsed, Value: cpu2},
				{Figure: fleet.FigureTrafficIn, Value: tr2},
				{Figure: fleet.FigureMemoryUsed, Value: 600},
				{Figure: fleet.FigureMemoryTotal, Value: 3000},
			},
		}, ts))
		// edge-03 reports memory-used WITHOUT memory-total: must NOT pollute paired ratio!
		require.NoError(t, h.Ingest("edge-03", &fleet.SampleReport{
			SchemaVersion: fleet.SampleSchemaVersion,
			Samples: []fleet.SampleReading{
				{Figure: fleet.FigureMemoryUsed, Value: 900},
			},
		}, ts))
	}

	// 1. Aligned set maxima on coarse peak slot (CPU mean: aligned peak = (90+10)/2 = 50, NOT 90).
	cpuPeakRows, err := h.QuerySeries(allowAllFleetScope(), meta, HistorySeriesQuery{
		Figure: fleet.FigureCPUUsed,
		From:   peakSlotStart,
		Until:  peakSlotStart.Add(15 * time.Minute),
		Reduce: true,
	}, end)
	require.NoError(t, err)
	require.Len(t, cpuPeakRows, 1)
	require.Equal(t, historyResolutionCoarse, cpuPeakRows[0].Resolution)
	require.False(t, cpuPeakRows[0].Gap)
	require.Equal(t, 2, cpuPeakRows[0].Contributors)
	require.Equal(t, 1, cpuPeakRows[0].Missing)
	require.Equal(t, uint32(15), cpuPeakRows[0].ObservedMinutes)
	require.Equal(t, uint32(15), cpuPeakRows[0].TotalMinutes)
	require.InDelta(t, 1.0, cpuPeakRows[0].Coverage, 1e-9)
	require.NotNil(t, cpuPeakRows[0].Max)
	require.InDelta(t, 50.0, *cpuPeakRows[0].Max, 1e-9, "aligned estate peak must not combine distinct-minute peaks")
	// Weighted mean across 30 observations: (28*10 + 2*90) / 30 = 460/30.
	require.InDelta(t, 460.0/30.0, *cpuPeakRows[0].Value, 1e-9)

	// 2. Aligned set maxima on summable figure (traffic-in: aligned peak = 1000+100 = 1100, NOT 2000).
	trPeakRows, err := h.QuerySeries(allowAllFleetScope(), meta, HistorySeriesQuery{
		Figure: fleet.FigureTrafficIn,
		From:   peakSlotStart,
		Until:  peakSlotStart.Add(15 * time.Minute),
		Reduce: true,
	}, end)
	require.NoError(t, err)
	require.Len(t, trPeakRows, 1)
	require.NotNil(t, trPeakRows[0].Max)
	require.InDelta(t, 1100.0, *trPeakRows[0].Max, 1e-9)

	// 3. Capacity ratio uses summed numerator and denominator from the SAME reporting devices only.
	memRows, err := h.QuerySeries(allowAllFleetScope(), meta, HistorySeriesQuery{
		Figure: fleet.FigureMemoryUsed,
		From:   peakSlotStart,
		Until:  peakSlotStart.Add(15 * time.Minute),
		Reduce: true,
	}, end)
	require.NoError(t, err)
	require.Len(t, memRows, 1)
	require.Equal(t, 3, memRows[0].Contributors)
	require.NotNil(t, memRows[0].Value)
	require.InDelta(t, 1900.0, *memRows[0].Value, 1e-9)
	require.NotNil(t, memRows[0].Ratio)
	require.InDelta(t, (400.0+600.0)/(1000.0+3000.0), *memRows[0].Ratio, 1e-9)

	// Zero installed capacity denominator returns unavailable (nil Ratio).
	zeroDenStore := NewManagedHistoryStore(ManagedHistoryStoreConfig{
		Config: plugin.DefaultHubHistoryConfig(),
		Now:    func() time.Time { return end },
	})
	require.NoError(t, zeroDenStore.Ingest("edge-z", &fleet.SampleReport{
		SchemaVersion: fleet.SampleSchemaVersion,
		Samples: []fleet.SampleReading{
			{Figure: fleet.FigureMemoryUsed, Value: 0},
			{Figure: fleet.FigureMemoryTotal, Value: 0},
		},
	}, end.Add(-5*time.Minute)))
	zeroRows, err := zeroDenStore.QuerySeries(allowAllFleetScope(), map[string]HistoryDeviceMeta{
		"edge-z": {Name: "edge-z", Declared: true},
	}, HistorySeriesQuery{
		Device: "edge-z",
		Figure: fleet.FigureMemoryUsed,
		From:   end.Add(-10 * time.Minute),
		Until:  end,
	}, end)
	require.NoError(t, err)
	var measuredZeroRow *HistorySeriesRow
	for i := range zeroRows {
		if !zeroRows[i].Gap {
			measuredZeroRow = &zeroRows[i]
			break
		}
	}
	require.NotNil(t, measuredZeroRow)
	require.Nil(t, measuredZeroRow.Ratio, "zero denominator must return unavailable ratio")

	// 4. Crossing the 1-day boundary: [end-36h, end) -> 12h coarse (48 slots) + 24h fine (1440 slots).
	crossFrom := end.Add(-36 * time.Hour)
	crossRows, err := h.QuerySeries(allowAllFleetScope(), meta, HistorySeriesQuery{
		Device: "edge-01",
		Figure: fleet.FigureCPUUsed,
		From:   crossFrom,
		Until:  end,
	}, end)
	require.NoError(t, err)
	require.Len(t, crossRows, 48+1440)
	var totalMins uint32
	for i := range crossRows {
		if i > 0 {
			require.Equal(t, crossRows[i-1].End, crossRows[i].Start, "adjacent buckets must abut with zero overlap and zero gap")
		}
		totalMins += crossRows[i].TotalMinutes
	}
	require.Equal(t, uint32(36*60), totalMins, "crossing 1-day boundary must count every minute exactly once")
	require.Equal(t, historyResolutionCoarse, crossRows[0].Resolution)
	require.Equal(t, historyResolutionFine, crossRows[len(crossRows)-1].Resolution)

	// 5. Points rebucketing reduces to requested point count preserving weighted mean and peak.
	rebucketed, err := h.QuerySeries(allowAllFleetScope(), meta, HistorySeriesQuery{
		Figure: fleet.FigureCPUUsed,
		From:   crossFrom,
		Until:  end,
		Reduce: true,
		Points: 36,
	}, end)
	require.NoError(t, err)
	require.Len(t, rebucketed, 36)
	require.Equal(t, crossFrom, rebucketed[0].Start)
	require.Equal(t, end, rebucketed[len(rebucketed)-1].End)
}

// TestManagedHistoryOfflineGapAndReconnect validates AC-4 and AC-12: offline
// devices retain latest + receipt time across disconnect and hub restart, series
// has explicit gaps during outage, and reconnect stamps fresh receipt time.
func TestManagedHistoryOfflineGapAndReconnect(t *testing.T) {
	t0 := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	clk := newReportTestClock(t0)
	store := storage.CreateMemory()
	clients := map[string]string{"edge-01": testClientSecret}

	// Hub #1 receives 80% CPU at 10:00, device disconnects, hub stops cleanly.
	func() {
		srv1, addr1 := startHistoryTestServer(t, store, clk, plugin.DefaultHubHistoryConfig(), clients, nil)
		mc := dialTestClient(t, addr1, "edge-01", testClientSecret)
		_, err := callReportRPC(t, mc, fleet.VerbSampleReport, makeSampleReport(
			t0.Format(time.RFC3339),
			fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 80},
		))
		require.NoError(t, err)
		require.NoError(t, mc.Close())
		srv1.Stop()
	}()

	// Hub #2 restarts 1 hour later while edge-01 is still offline.
	t1 := clk.Advance(time.Hour)
	srv2, addr2 := startHistoryTestServer(t, store, clk, plugin.DefaultHubHistoryConfig(), clients, nil)

	latestOffline, err := srv2.HistoryLatest(allowAllFleetScope(), HistoryLatestFilter{Device: "edge-01"})
	require.NoError(t, err)
	require.Len(t, latestOffline, 1)
	require.True(t, latestOffline[0].HasReport)
	require.False(t, latestOffline[0].Connected)
	require.Equal(t, t0, latestOffline[0].ReceivedAt)
	require.Equal(t, int64(3600), latestOffline[0].AgeSeconds)
	require.InDelta(t, 80.0, latestOffline[0].Figures[fleet.FigureCPUUsed], 1e-9)

	// Reconnect at 11:00 and report 25% CPU.
	mc2 := dialTestClient(t, addr2, "edge-01", testClientSecret)
	_, err = callReportRPC(t, mc2, fleet.VerbSampleReport, makeSampleReport(
		t1.Format(time.RFC3339),
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 25},
	))
	require.NoError(t, err)

	latestReconnected, err := srv2.HistoryLatest(allowAllFleetScope(), HistoryLatestFilter{Device: "edge-01"})
	require.NoError(t, err)
	require.Len(t, latestReconnected, 1)
	require.Equal(t, t1, latestReconnected[0].ReceivedAt)
	require.Equal(t, int64(0), latestReconnected[0].AgeSeconds)
	require.InDelta(t, 25.0, latestReconnected[0].Figures[fleet.FigureCPUUsed], 1e-9)

	// Series over [10:00, 11:01) has measured points at 10:00 and 11:00 and 59 explicit gap minutes in between.
	series, err := srv2.HistorySeries(allowAllFleetScope(), HistorySeriesQuery{
		Device: "edge-01",
		Figure: fleet.FigureCPUUsed,
		From:   t0,
		Until:  t1.Add(time.Minute),
	})
	require.NoError(t, err)
	require.Len(t, series, 61)
	require.False(t, series[0].Gap)
	require.InDelta(t, 80.0, *series[0].Value, 1e-9)
	for i := 1; i < 60; i++ {
		require.True(t, series[i].Gap, "offline minute %d must be an explicit gap", i)
		require.Nil(t, series[i].Value)
		require.Zero(t, series[i].Count)
	}
	require.False(t, series[60].Gap)
	require.InDelta(t, 25.0, *series[60].Value, 1e-9)
}

// TestManagedHistoryRateLimitClockStepAndWholeRejection validates AC-8 and AC-13:
// 1 report per UTC interval across reconnects, backwards wall-clock step guard,
// whole-submission rejection on malformed/duplicate/out-of-domain/oversized/
// superseded-session reports, and disabled sampling (interval=0).
func TestManagedHistoryRateLimitClockStepAndWholeRejection(t *testing.T) {
	t0 := time.Date(2026, 11, 20, 10, 0, 10, 0, time.UTC)
	clk := newReportTestClock(t0)
	store := storage.CreateMemory()
	clients := map[string]string{
		"edge-01": testClientSecret,
		"edge-02": testClient2Secret,
	}
	srv, addr := startHistoryTestServer(t, store, clk, plugin.DefaultHubHistoryConfig(), clients, nil)

	mc1 := dialTestClient(t, addr, "edge-01", testClientSecret)
	mc2 := dialTestClient(t, addr, "edge-02", testClient2Secret)

	good1 := makeSampleReport(t0.Format(time.RFC3339), fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 42})
	_, err := callReportRPC(t, mc1, fleet.VerbSampleReport, good1)
	require.NoError(t, err)

	// Same UTC interval 30s later -> rate limited.
	clk.Advance(30 * time.Second)
	_, err = callReportRPC(t, mc1, fleet.VerbSampleReport, makeSampleReport("", fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 99}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "rate limited")

	// Disconnect and reconnect still within the SAME UTC minute -> STILL rate limited!
	srv.mu.Lock()
	doneCh := srv.devices["edge-01"].sessionDone
	srv.mu.Unlock()
	require.NoError(t, mc1.Close())
	<-doneCh
	mc1 = dialTestClient(t, addr, "edge-01", testClientSecret)
	_, err = callReportRPC(t, mc1, fleet.VerbSampleReport, makeSampleReport("", fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 99}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "rate limited")

	// Backwards wall-clock step -> rejected without touching latest or rings.
	clk.Advance(-2 * time.Minute)
	_, err = callReportRPC(t, mc1, fleet.VerbSampleReport, makeSampleReport("", fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 77}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "backwards")

	// Advance into next valid UTC minute (10:01:10) and test malformed/bad reports FIRST.
	clk.Advance(2*time.Minute + 30*time.Second)

	// Malformed/unknown field, duplicate figure, out-of-domain, wrong schema version, oversize.
	_, err = callReportRPC(t, mc1, fleet.VerbSampleReport, map[string]any{"schema-version": 1, "samples": []any{}, "extra": 1})
	require.Error(t, err)
	_, err = callReportRPC(t, mc1, fleet.VerbSampleReport, makeSampleReport("",
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 10},
		fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 20},
	))
	require.Error(t, err)
	_, err = callReportRPC(t, mc1, fleet.VerbSampleReport, makeSampleReport("", fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 150}))
	require.Error(t, err)
	_, err = callReportRPC(t, mc1, fleet.VerbSampleReport, fleet.SampleReport{SchemaVersion: 99})
	require.Error(t, err)
	_, err = callReportRPC(t, mc1, fleet.VerbSampleReport, map[string]any{
		"schema-version": 1,
		"device-time":    strings.Repeat("x", fleet.SampleReportBodyMax+128),
		"samples":        []any{},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "exceeds max")

	// Superseded/stale session rejected without mutating history.
	staleMC := &rpc.MuxConn{}
	require.Error(t, srv.recordSampleReport("edge-01", staleMC, &good1))

	// Previous valid sample (42) must still be intact AND the 10:01 slot must still be open!
	rows, err := srv.HistoryLatest(allowAllFleetScope(), HistoryLatestFilter{Device: "edge-01"})
	require.NoError(t, err)
	require.InDelta(t, 42.0, rows[0].Figures[fleet.FigureCPUUsed], 1e-9)
	require.Equal(t, t0, rows[0].ReceivedAt)

	// Valid report in 10:01 succeeds, and edge-02 is unaffected.
	_, err = callReportRPC(t, mc1, fleet.VerbSampleReport, makeSampleReport("", fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 55}))
	require.NoError(t, err)
	_, err = callReportRPC(t, mc2, fleet.VerbSampleReport, makeSampleReport("", fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 12}))
	require.NoError(t, err)

	status, err := srv.HistoryStorage(allowAllFleetScope())
	require.NoError(t, err)
	require.GreaterOrEqual(t, status.RejectedReports, uint64(8))
	require.GreaterOrEqual(t, status.RateLimitedReports, uint64(3))

	// Disabled sampling (IntervalSeconds == 0) advertises 0 and refuses sample-report.
	disabledCfg := plugin.DefaultHubHistoryConfig()
	disabledCfg.IntervalSeconds = 0
	srvDis, addrDis := startHistoryTestServer(t, storage.CreateMemory(), clk, disabledCfg, map[string]string{"edge-01": testClientSecret}, nil)
	_ = srvDis
	mcDis := dialTestClient(t, addrDis, "edge-01", testClientSecret)
	respDis := fetch(t, mcDis, "")
	require.Zero(t, respDis.SampleIntervalSeconds)
	_, err = callReportRPC(t, mcDis, fleet.VerbSampleReport, good1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "disabled")
}

// TestManagedHistoryCheckpointRestartAndFailureRecovery validates AC-3 and
// AC-15: checkpoint failure keeps dirty data for retry, exposes failure + last
// success age in doctor, keeps healthy committed shards readable, recovers when
// store heals, and skips corrupt shards on startup without losing healthy data.
func TestManagedHistoryCheckpointRestartAndFailureRecovery(t *testing.T) {
	t0 := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	clk := newReportTestClock(t0)
	fstore := newHistoryFailStore()

	h := NewManagedHistoryStore(ManagedHistoryStoreConfig{
		Config:          plugin.DefaultHubHistoryConfig(),
		Store:           fstore,
		Now:             clk.Now,
		DeclaredClients: 2,
	})
	require.NoError(t, h.Ingest("edge-01", &fleet.SampleReport{
		SchemaVersion: fleet.SampleSchemaVersion,
		Samples:       []fleet.SampleReading{{Figure: fleet.FigureCPUUsed, Value: 30}},
	}, t0))
	require.NoError(t, h.CheckpointOnce())

	healthOK := h.HealthSnapshot(t0)
	require.False(t, healthOK.CheckpointFailed)
	require.Equal(t, t0, healthOK.LastCheckpointOKAt)

	// Advance 10 minutes, ingest edge-01 and edge-02, and fail ONLY edge-02 writes.
	t1 := clk.Advance(10 * time.Minute)
	require.NoError(t, h.Ingest("edge-01", &fleet.SampleReport{
		SchemaVersion: fleet.SampleSchemaVersion,
		Samples:       []fleet.SampleReading{{Figure: fleet.FigureCPUUsed, Value: 45}},
	}, t1))
	require.NoError(t, h.Ingest("edge-02", &fleet.SampleReport{
		SchemaVersion: fleet.SampleSchemaVersion,
		Samples:       []fleet.SampleReading{{Figure: fleet.FigureCPUUsed, Value: 60}},
	}, t1))

	fstore.mu.Lock()
	fstore.failWrite = func(key string) error {
		if strings.Contains(key, "edge-02") {
			return errors.New("simulated disk write failure")
		}
		return nil
	}
	fstore.mu.Unlock()

	err := h.CheckpointOnce()
	require.Error(t, err)
	require.Contains(t, err.Error(), "simulated disk write failure")

	t2 := clk.Advance(2 * time.Minute)
	healthFail := h.HealthSnapshot(t2)
	require.True(t, healthFail.CheckpointFailed)
	require.Contains(t, healthFail.LastCheckpointError, "simulated disk write failure")
	require.Equal(t, t0, healthFail.LastCheckpointOKAt)
	require.Equal(t, int64(12*60), healthFail.LastCheckpointAgeSeconds)

	// Healthy edge-01 shard was committed and is readable by a restarted store!
	reloadedPartial := NewManagedHistoryStore(ManagedHistoryStoreConfig{
		Config: plugin.DefaultHubHistoryConfig(),
		Store:  fstore,
		Now:    clk.Now,
	})
	require.NoError(t, reloadedPartial.LoadFromStore())
	rowsPartial, err := reloadedPartial.QueryLatest(allowAllFleetScope(), map[string]HistoryDeviceMeta{
		"edge-01": {Name: "edge-01", Declared: true},
	}, HistoryLatestFilter{Device: "edge-01"}, t2)
	require.NoError(t, err)
	require.Len(t, rowsPartial, 1)
	require.InDelta(t, 45.0, rowsPartial[0].Figures[fleet.FigureCPUUsed], 1e-9)
	require.Equal(t, t1, rowsPartial[0].ReceivedAt)

	// Clear store failure; retry checkpoint recovers cleanly and persists edge-02.
	fstore.mu.Lock()
	fstore.failWrite = nil
	fstore.mu.Unlock()
	require.NoError(t, h.CheckpointOnce())

	healthRecovered := h.HealthSnapshot(t2)
	require.False(t, healthRecovered.CheckpointFailed)
	require.Empty(t, healthRecovered.LastCheckpointError)
	require.Equal(t, t2, healthRecovered.LastCheckpointOKAt)

	// Inject a corrupt shard for a third device; restart must skip it, report
	// it in doctor health, and still load edge-01 and edge-02 cleanly.
	require.NoError(t, fstore.WriteKey(historyShardKey("edge-bad", historyShardNameFine), []byte("not-a-valid-shard")))
	reloaded := NewManagedHistoryStore(ManagedHistoryStoreConfig{
		Config: plugin.DefaultHubHistoryConfig(),
		Store:  fstore,
		Now:    clk.Now,
	})
	require.NoError(t, reloaded.LoadFromStore())
	healthCorrupt := reloaded.HealthSnapshot(t2)
	require.Len(t, healthCorrupt.CorruptRecords, 1)
	require.Equal(t, "edge-bad", healthCorrupt.CorruptRecords[0].Name)

	reloadedRows, err := reloaded.QueryLatest(allowAllFleetScope(), map[string]HistoryDeviceMeta{
		"edge-01": {Name: "edge-01", Declared: true},
		"edge-02": {Name: "edge-02", Declared: true},
	}, HistoryLatestFilter{}, t2)
	require.NoError(t, err)
	require.Len(t, reloadedRows, 2)
	require.InDelta(t, 45.0, reloadedRows[0].Figures[fleet.FigureCPUUsed], 1e-9)
	require.InDelta(t, 60.0, reloadedRows[1].Figures[fleet.FigureCPUUsed], 1e-9)
}

// TestManagedHistoryUndeclareVsDeleteFencingAndRestart validates AC-7 and
// AC-15: undeclaring preserves remembered history, actual deletion fences an
// in-flight checkpoint write without resurrecting data, interrupted deletion
// intents replay on restart, and recreating the name starts empty.
func TestManagedHistoryUndeclareVsDeleteFencingAndRestart(t *testing.T) {
	t0 := time.Date(2026, 11, 20, 14, 0, 0, 0, time.UTC)
	clk := newReportTestClock(t0)
	store := heldTestStore(t)
	clients := map[string]string{
		"edge-01": testClientSecret,
		"edge-02": testClient2Secret,
	}
	srv, addr := startHistoryTestServer(t, store, clk, plugin.DefaultHubHistoryConfig(), clients, nil)

	mc1 := dialTestClient(t, addr, "edge-01", testClientSecret)
	_, err := callReportRPC(t, mc1, fleet.VerbSampleReport, makeSampleReport("", fleet.SampleReading{Figure: fleet.FigureCPUUsed, Value: 71}))
	require.NoError(t, err)
	require.NoError(t, srv.History().CheckpointOnce())

	// 1. Undeclare edge-01: remembered record and metric history MUST survive!
	plan, err := srv.PrepareManagedListenersWithHistory([]ManagedListener{{
		Addr:    addr,
		Clients: map[string]string{"edge-02": testClient2Secret},
	}}, "", plugin.DefaultHubHistoryConfig())
	require.NoError(t, err)
	srv.ApplyManagedListeners(plan)
	require.True(t, srv.History().HasDevice("edge-01"))

	// Re-declare edge-01: history is still intact!
	plan2, err := srv.PrepareManagedListenersWithHistory([]ManagedListener{{
		Addr:    addr,
		Clients: clients,
	}}, "", plugin.DefaultHubHistoryConfig())
	require.NoError(t, err)
	srv.ApplyManagedListeners(plan2)
	rowsAfterRedeploy, err := srv.HistoryLatest(allowAllFleetScope(), HistoryLatestFilter{Device: "edge-01"})
	require.NoError(t, err)
	require.Len(t, rowsAfterRedeploy, 1)
	require.InDelta(t, 71.0, rowsAfterRedeploy[0].Figures[fleet.FigureCPUUsed], 1e-9)

	// 2. Race an in-flight checkpoint flush against actual device deletion!
	t1 := clk.Advance(time.Minute)
	require.NoError(t, srv.History().Ingest("edge-01", &fleet.SampleReport{
		SchemaVersion: fleet.SampleSchemaVersion,
		Samples:       []fleet.SampleReading{{Figure: fleet.FigureCPUUsed, Value: 82}},
	}, t1))

	entered := make(chan struct{})
	release := make(chan struct{})
	srv.History().beforeShardWrite = func(dev, shard string) {
		if dev == "edge-01" && shard == historyShardNameFine {
			close(entered)
			<-release
		}
	}

	cpDone := make(chan error, 1)
	go func() {
		cpDone <- srv.History().CheckpointOnce()
	}()
	<-entered

	// Delete the device record while the fine shard flush is paused mid-flight!
	srv.drainDeviceRecords()
	require.NoError(t, store.RemoveKey(deviceRecordKey("edge-01")))
	close(release)
	require.NoError(t, <-cpDone)
	srv.History().beforeShardWrite = nil

	// CheckpointOnce itself drains trailing pending deletes before returning so
	// edge-01 is completely erased from memory AND store without a manual drain.
	require.False(t, srv.History().HasDevice("edge-01"))
	_, err = store.ReadKey(historyLatestKey("edge-01"))
	require.ErrorIs(t, err, fs.ErrNotExist)
	leftoverShards, err := store.ListKeys(historyDeviceShardPrefix("edge-01"))
	require.NoError(t, err)
	require.Empty(t, leftoverShards)
	_, err = store.ReadKey(historyDeleteIntentKey("edge-01"))
	require.ErrorIs(t, err, fs.ErrNotExist)

	// 3. Interrupted deletion recovery: seed delete-intent + leftover shards for edge-02.
	require.NoError(t, srv.History().Ingest("edge-02", &fleet.SampleReport{
		SchemaVersion: fleet.SampleSchemaVersion,
		Samples:       []fleet.SampleReading{{Figure: fleet.FigureCPUUsed, Value: 33}},
	}, t1))
	require.NoError(t, srv.History().CheckpointOnce())
	intentRaw, err := json.Marshal(persistedHistoryDeleteIntent{
		SchemaVersion: historyPersistSchemaVersion,
		Device:        "edge-02",
		Generation:    99,
		DeletedAt:     t1,
	})
	require.NoError(t, err)
	require.NoError(t, store.WriteKey(historyDeleteIntentKey("edge-02"), intentRaw))

	// Restarting reloads the store: replays edge-02 deletion intent and keeps edge-01 gone!
	restarted := NewManagedHistoryStore(ManagedHistoryStoreConfig{
		Config: plugin.DefaultHubHistoryConfig(),
		Store:  store,
		Now:    clk.Now,
	})
	require.NoError(t, restarted.LoadFromStore())
	require.False(t, restarted.HasDevice("edge-01"))
	require.False(t, restarted.HasDevice("edge-02"))
	_, err = store.ReadKey(historyDeleteIntentKey("edge-02"))
	require.ErrorIs(t, err, fs.ErrNotExist)

	// Recreating edge-01 starts empty with fresh history only.
	t2 := clk.Advance(time.Minute)
	require.NoError(t, restarted.Ingest("edge-01", &fleet.SampleReport{
		SchemaVersion: fleet.SampleSchemaVersion,
		Samples:       []fleet.SampleReading{{Figure: fleet.FigureCPUUsed, Value: 15}},
	}, t2))
	recreatedSeries, err := restarted.QuerySeries(allowAllFleetScope(), map[string]HistoryDeviceMeta{
		"edge-01": {Name: "edge-01", Declared: true},
	}, HistorySeriesQuery{
		Device: "edge-01",
		Figure: fleet.FigureCPUUsed,
		From:   t0,
		Until:  t2.Add(time.Minute),
	}, t2)
	require.NoError(t, err)
	measuredCount := 0
	for i := range recreatedSeries {
		if !recreatedSeries[i].Gap {
			measuredCount++
			require.InDelta(t, 15.0, *recreatedSeries[i].Value, 1e-9)
		}
	}
	require.Equal(t, 1, measuredCount)
}

// TestManagedHistoryCapEvictionReloadAndOutputBounds validates AC-5, reload
// budget validation, capacity pressure doctor recovery, and the 4,096-row query
// output bound.
func TestManagedHistoryCapEvictionReloadAndOutputBounds(t *testing.T) {
	t0 := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	clk := newReportTestClock(t0)
	store := storage.CreateMemory()
	tightCfg := plugin.HubHistoryConfig{
		IntervalSeconds:   60,
		RetentionDays:     7,
		DeviceBudgetBytes: 6 << 10,  // 6 KiB per device
		HubBudgetBytes:    16 << 10, // 16 KiB hub total
	}
	clients := map[string]string{
		"edge-01": testClientSecret,
		"edge-02": testClient2Secret,
	}
	srv, addr := startHistoryTestServer(t, store, clk, tightCfg, clients, nil)

	// Feed 18 hours of varied data across 6 figures for edge-01 to trigger per-device cap eviction.
	rng := rand.New(rand.NewPCG(1, 2))
	var lastVal float64
	for m := range 18 * 60 {
		ts := t0.Add(time.Duration(m) * time.Minute)
		clk.Advance(time.Minute)
		lastVal = rng.Float64() * 100
		require.NoError(t, srv.History().Ingest("edge-01", &fleet.SampleReport{
			SchemaVersion: fleet.SampleSchemaVersion,
			Samples: []fleet.SampleReading{
				{Figure: fleet.FigureCPUUsed, Value: lastVal},
				{Figure: fleet.FigureLoad1m, Value: rng.Float64() * 16},
				{Figure: fleet.FigureMemoryUsed, Value: rng.Float64() * 1e9},
				{Figure: fleet.FigureMemoryTotal, Value: 16e9},
				{Figure: fleet.FigureTrafficIn, Value: rng.Float64() * 1e9},
				{Figure: fleet.FigureTrafficOut, Value: rng.Float64() * 1e9},
			},
		}, ts))
	}

	status, err := srv.HistoryStorage(allowAllFleetScope())
	require.NoError(t, err)
	require.LessOrEqual(t, status.UsedBytes, tightCfg.HubBudgetBytes, "hub total bytes must never exceed hub cap")
	require.Len(t, status.Devices, 1)
	require.LessOrEqual(t, status.Devices[0].UsedBytes, tightCfg.DeviceBudgetBytes, "device bytes must never exceed per-device cap")
	require.True(t, status.CapacityPressure, "cap eviction must raise capacity pressure")
	require.Less(t, status.EffectiveRetentionSeconds, int64(7*24*3600), "effective retention must shorten under cap")

	// Latest observation is NEVER evicted under cap pressure!
	latest, err := srv.HistoryLatest(allowAllFleetScope(), HistoryLatestFilter{Device: "edge-01"})
	require.NoError(t, err)
	require.Len(t, latest, 1)
	require.True(t, latest[0].HasReport)
	require.InDelta(t, lastVal, latest[0].Figures[fleet.FigureCPUUsed], 1e-9)

	// Doctor runtime health reports capacity pressure.
	docRec, running := ManagedRuntimeHealth()
	require.True(t, running)
	require.NotNil(t, docRec.History)
	require.True(t, docRec.History.CapacityPressure)

	// Impossible reload budget (too small for 2 declared devices: needs >= 8192 bytes) is refused!
	impossible := tightCfg
	impossible.HubBudgetBytes = 4096
	_, err = srv.PrepareManagedListenersWithHistory([]ManagedListener{{Addr: addr, Clients: clients}}, "", impossible)
	require.Error(t, err)
	opErr, isOp := errors.AsType[*opererr.Error](err)
	require.True(t, isOp)
	require.Contains(t, opErr.Error(), "8192")

	// Valid reload raising budgets clears capacity pressure.
	relaxed := plugin.DefaultHubHistoryConfig()
	plan, err := srv.PrepareManagedListenersWithHistory([]ManagedListener{{Addr: addr, Clients: clients}}, "", relaxed)
	require.NoError(t, err)
	srv.ApplyManagedListeners(plan)
	docRec2, _ := ManagedRuntimeHealth()
	require.False(t, docRec2.History.CapacityPressure)

	// Output bound (> 4,096 rows) returns explicit refinement operator error.
	_, err = srv.HistorySeries(allowAllFleetScope(), HistorySeriesQuery{
		From:  clk.Now().Add(-7 * 24 * time.Hour),
		Until: clk.Now(),
	})
	require.Error(t, err)
	boundErr, ok := errors.AsType[*opererr.Error](err)
	require.True(t, ok)
	require.Contains(t, boundErr.Fix(), "| points")
}

// TestManagedHistoryEstateSizingAndSparse4096 validates spec-fleet-8 Sizing and
// Recovery Evidence: 4,096 declared devices with sparse data do not preallocate
// offline rings; 2,000 connected devices stay within budget with measured
// checkpoint/reload times; worst-case 7-day incompressible 15-figure density
// enforces the 1 MiB hard ceiling and reports shortened effective retention.
func TestManagedHistoryEstateSizingAndSparse4096(t *testing.T) {
	t.Run("4096 declared devices with sparse data do not preallocate offline rings", func(t *testing.T) {
		t0 := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
		store := storage.CreateMemory()
		h := NewManagedHistoryStore(ManagedHistoryStoreConfig{
			Config:          plugin.DefaultHubHistoryConfig(),
			Store:           store,
			Now:             func() time.Time { return t0.Add(15 * time.Minute) },
			DeclaredClients: plugin.ManagedClientsMax, // 4,096 declared
		})
		require.NoError(t, plugin.ValidateHubHistoryConfig(plugin.DefaultHubHistoryConfig(), plugin.ManagedClientsMax))

		// Only 128 of 4,096 declared devices send 10 samples each; 3,968 remain offline.
		allFigs := fleet.AllSampleFigures()
		readings := make([]fleet.SampleReading, len(allFigs))
		for i, spec := range allFigs {
			readings[i] = fleet.SampleReading{Figure: spec.Name, Value: float64(i + 1)}
		}
		for d := range 128 {
			name := fmt.Sprintf("edge-%04d", d)
			for m := range 10 {
				require.NoError(t, h.Ingest(name, &fleet.SampleReport{
					SchemaVersion: fleet.SampleSchemaVersion,
					Samples:       readings,
				}, t0.Add(time.Duration(m)*time.Minute)))
			}
		}

		// Offline devices MUST NOT allocate any entries in h.devices!
		h.mu.Lock()
		allocatedDevices := len(h.devices)
		h.mu.Unlock()
		require.Equal(t, 128, allocatedDevices, "offline declared devices must not preallocate rings")

		cpStart := time.Now()
		require.NoError(t, h.CheckpointOnce())
		cpDur := time.Since(cpStart)

		reloaded := NewManagedHistoryStore(ManagedHistoryStoreConfig{
			Config:          plugin.DefaultHubHistoryConfig(),
			Store:           store,
			Now:             func() time.Time { return t0.Add(15 * time.Minute) },
			DeclaredClients: plugin.ManagedClientsMax,
		})
		loadStart := time.Now()
		require.NoError(t, reloaded.LoadFromStore())
		loadDur := time.Since(loadStart)

		status, err := reloaded.QueryStorage(allowAllFleetScope(), nil, t0.Add(15*time.Minute))
		require.NoError(t, err)
		require.Equal(t, 128, status.DeviceCount)
		require.Less(t, status.UsedBytes, int64(2<<20), "sparse 4096 estate must stay far below 2 GiB")
		require.False(t, status.CapacityPressure)
		t.Logf("4096 declared (128 active x 10m x 15 figs): used=%d bytes, checkpoint=%v, reload=%v",
			status.UsedBytes, cpDur, loadDur)
	})

	t.Run("2000 connected devices checkpoint, reload, and budget compliance", func(t *testing.T) {
		t0 := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
		store := storage.CreateMemory()
		now := t0.Add(2 * time.Hour)
		h := NewManagedHistoryStore(ManagedHistoryStoreConfig{
			Config:          plugin.DefaultHubHistoryConfig(),
			Store:           store,
			Now:             func() time.Time { return now },
			DeclaredClients: 2000,
		})

		allFigs := fleet.AllSampleFigures()
		readings := make([]fleet.SampleReading, len(allFigs))
		var memBefore, memAfter runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&memBefore)

		// 2,000 connected devices x 30 minutes x 15 figures = 900,000 measurements.
		for d := range 2000 {
			name := fmt.Sprintf("router-%04d", d)
			for m := range 30 {
				for i, spec := range allFigs {
					v := float64((d+m+i)%90) + 1.5
					if spec.HasMax && v > spec.Max {
						v = spec.Max
					}
					readings[i] = fleet.SampleReading{Figure: spec.Name, Value: v}
				}
				require.NoError(t, h.Ingest(name, &fleet.SampleReport{
					SchemaVersion: fleet.SampleSchemaVersion,
					Samples:       readings,
				}, t0.Add(time.Duration(m)*time.Minute)))
			}
		}

		runtime.ReadMemStats(&memAfter)
		cpStart := time.Now()
		require.NoError(t, h.CheckpointOnce())
		cpDur := time.Since(cpStart)

		reloaded := NewManagedHistoryStore(ManagedHistoryStoreConfig{
			Config:          plugin.DefaultHubHistoryConfig(),
			Store:           store,
			Now:             func() time.Time { return now },
			DeclaredClients: 2000,
		})
		loadStart := time.Now()
		require.NoError(t, reloaded.LoadFromStore())
		loadDur := time.Since(loadStart)

		status, err := reloaded.QueryStorage(allowAllFleetScope(), nil, now)
		require.NoError(t, err)
		require.Equal(t, 2000, status.DeviceCount)
		require.LessOrEqual(t, status.UsedBytes, int64(fleet.DefaultHistoryHubBytes))
		require.False(t, status.CapacityPressure)
		t.Logf("2000 connected devices (30m x 15 figs): used=%d bytes (avg %d B/dev), heapAllocDelta=%d MiB, checkpoint=%v, reload=%v",
			status.UsedBytes, status.UsedBytes/2000, int64(memAfter.HeapAlloc-memBefore.HeapAlloc)/(1<<20), cpDur, loadDur)
	})

	t.Run("worst-case 7-day incompressible 15-figure density honors 1 MiB hard cap and reports shortened retention", func(t *testing.T) {
		end := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
		start := end.Add(-7 * 24 * time.Hour)
		h := NewManagedHistoryStore(ManagedHistoryStoreConfig{
			Config:          plugin.DefaultHubHistoryConfig(), // 1 MiB per-device cap, 7 days
			Now:             func() time.Time { return end },
			DeclaredClients: 1,
		})
		allFigs := fleet.AllSampleFigures()
		readings := make([]fleet.SampleReading, len(allFigs))
		rng := rand.New(rand.NewPCG(42, 99))

		// Ingest 7 full days (10,080 minutes) of high-entropy random floats across all 15 figures.
		for m := range 7 * 24 * 60 {
			ts := start.Add(time.Duration(m) * time.Minute)
			for i, spec := range allFigs {
				v := rng.Float64() * 1e6
				if spec.HasMax {
					v = rng.Float64() * spec.Max
				}
				readings[i] = fleet.SampleReading{Figure: spec.Name, Value: v}
			}
			require.NoError(t, h.Ingest("dense-01", &fleet.SampleReport{
				SchemaVersion: fleet.SampleSchemaVersion,
				Samples:       readings,
			}, ts))
		}

		status, err := h.QueryStorage(allowAllFleetScope(), nil, end)
		require.NoError(t, err)
		require.Len(t, status.Devices, 1)
		require.LessOrEqual(t, status.Devices[0].UsedBytes, int64(fleet.DefaultHistoryPerDeviceBytes),
			"hard 1 MiB per-device cap must never be exceeded even on incompressible 7-day data")
		require.True(t, status.CapacityPressure, "incompressible 7-day 15-figure stream exceeds 1 MiB and must report capacity pressure")
		require.Less(t, status.EffectiveRetentionSeconds, int64(7*24*3600), "effective retention must reflect shortened span")
		require.Greater(t, status.EffectiveRetentionSeconds, int64(24*3600), "1 MiB cap should still retain well over 1 day even under worst-case entropy")
		t.Logf("worst-case 7d x 15 figs incompressible under 1 MiB cap: used=%d bytes, effectiveRetention=%v",
			status.Devices[0].UsedBytes, time.Duration(status.EffectiveRetentionSeconds)*time.Second)
	})
}

// TestManagedHistorySeriesHorizonClippingAndSubMinuteBounds validates that
// unbounded [from, until) ranges clip cleanly to the retained horizon without
// allocating unbounded slot slices, ranges outside the horizon return empty,
// and fine buckets clip Start/End to sub-minute query bounds.
func TestManagedHistorySeriesHorizonClippingAndSubMinuteBounds(t *testing.T) {
	now := time.Date(2026, 11, 20, 10, 5, 0, 0, time.UTC)
	h := NewManagedHistoryStore(ManagedHistoryStoreConfig{
		Config:          plugin.DefaultHubHistoryConfig(),
		Now:             func() time.Time { return now },
		DeclaredClients: 1,
	})
	meta := map[string]HistoryDeviceMeta{
		"edge-01": {Name: "edge-01", Declared: true, Connected: true},
	}
	t0 := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	require.NoError(t, h.Ingest("edge-01", &fleet.SampleReport{
		SchemaVersion: fleet.SampleSchemaVersion,
		Samples:       []fleet.SampleReading{{Figure: fleet.FigureCPUUsed, Value: 20}},
	}, t0))
	require.NoError(t, h.Ingest("edge-01", &fleet.SampleReport{
		SchemaVersion: fleet.SampleSchemaVersion,
		Samples:       []fleet.SampleReading{{Figure: fleet.FigureCPUUsed, Value: 40}},
	}, t0.Add(time.Minute)))

	// Unbounded ancient..future query must clip to retained horizon and return quickly.
	startExec := time.Now()
	rows, err := h.QuerySeries(allowAllFleetScope(), meta, HistorySeriesQuery{
		Device: "edge-01",
		Figure: fleet.FigureCPUUsed,
		From:   time.Date(1, 1, 2, 0, 0, 0, 0, time.UTC),
		Until:  time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC),
		Points: 10,
	}, now)
	require.NoError(t, err)
	require.Less(t, time.Since(startExec), 5*time.Second)
	require.Len(t, rows, 10)
	require.Equal(t, now.Add(-7*24*time.Hour), rows[0].Start)
	require.Equal(t, now.Add(time.Minute), rows[len(rows)-1].End)

	// Omitted From with far-future Until still defaults From to retained horizon.
	openFromRows, err := h.QuerySeries(allowAllFleetScope(), meta, HistorySeriesQuery{
		Device: "edge-01",
		Figure: fleet.FigureCPUUsed,
		Until:  time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC),
		Points: 10,
	}, now)
	require.NoError(t, err)
	require.Len(t, openFromRows, 10)
	require.Equal(t, now.Add(-7*24*time.Hour), openFromRows[0].Start)
	require.Equal(t, now.Add(time.Minute), openFromRows[len(openFromRows)-1].End)

	// Ranges completely outside retained horizon return empty without error.
	oldRows, err := h.QuerySeries(allowAllFleetScope(), meta, HistorySeriesQuery{
		Device: "edge-01",
		Figure: fleet.FigureCPUUsed,
		From:   time.Date(1, 1, 2, 0, 0, 0, 0, time.UTC),
		Until:  time.Date(2, 1, 1, 0, 0, 0, 0, time.UTC),
	}, now)
	require.NoError(t, err)
	require.Empty(t, oldRows)

	futureRows, err := h.QuerySeries(allowAllFleetScope(), meta, HistorySeriesQuery{
		Device: "edge-01",
		Figure: fleet.FigureCPUUsed,
		From:   time.Date(9000, 1, 1, 0, 0, 0, 0, time.UTC),
		Until:  time.Date(9001, 1, 1, 0, 0, 0, 0, time.UTC),
	}, now)
	require.NoError(t, err)
	require.Empty(t, futureRows)

	// Sub-minute bounds clip fine bucket Start and End in both device and reduced modes.
	subFrom := t0.Add(15 * time.Second)
	subUntil := t0.Add(time.Minute + 45*time.Second)
	for _, reduced := range []bool{false, true} {
		subRows, subErr := h.QuerySeries(allowAllFleetScope(), meta, HistorySeriesQuery{
			Device: "edge-01",
			Figure: fleet.FigureCPUUsed,
			From:   subFrom,
			Until:  subUntil,
			Reduce: reduced,
		}, now)
		require.NoError(t, subErr)
		require.Len(t, subRows, 2)
		require.Equal(t, subFrom, subRows[0].Start)
		require.Equal(t, t0.Add(time.Minute), subRows[0].End)
		require.Equal(t, t0.Add(time.Minute), subRows[1].Start)
		require.Equal(t, subUntil, subRows[1].End)
	}
}

// TestManagedHistoryIntervalReloadNotifiesConnectedDevices validates that
// reloading bus hub history interval notifies already-connected devices via
// config-changed so they re-fetch and pick up the new SampleIntervalSeconds.
func TestManagedHistoryIntervalReloadNotifiesConnectedDevices(t *testing.T) {
	t0 := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	clk := newReportTestClock(t0)
	store := storage.CreateMemory()
	clients := map[string]string{"edge-01": testClientSecret}
	srv, addr := startHistoryTestServer(t, store, clk, plugin.DefaultHubHistoryConfig(), clients, nil)

	mc := dialTestClient(t, addr, "edge-01", testClientSecret)
	notifyCh := make(chan struct{}, 8)
	go func() {
		for req := range mc.Requests() {
			if req.Method == fleet.VerbConfigChanged {
				_ = mc.SendOK(context.Background(), req.ID)
				notifyCh <- struct{}{}
			}
		}
	}()

	// Initial fetch confirms default 60s.
	resp0 := fetch(t, mc, "")
	require.Equal(t, 60, resp0.SampleIntervalSeconds)

	for _, nextInterval := range []int{120, 0, 60} {
		cfg := plugin.DefaultHubHistoryConfig()
		cfg.IntervalSeconds = nextInterval
		plan, err := srv.PrepareManagedListenersWithHistory([]ManagedListener{{
			Addr:    "127.0.0.1:0",
			Clients: clients,
		}}, "", cfg)
		require.NoError(t, err)
		srv.ApplyManagedListeners(plan)

		select {
		case <-notifyCh:
		case <-time.After(3 * time.Second):
			t.Fatalf("expected config-changed notification for interval reload to %d", nextInterval)
		}

		refetched := fetch(t, mc, resp0.Version)
		require.Equal(t, "current", refetched.Status)
		require.Equal(t, nextInterval, refetched.SampleIntervalSeconds)
	}
}

// TestManagedHistoryConcurrentEvictionKeepsDirtyDuringFlush validates that a
// cap eviction occurring while a checkpoint flush is in flight keeps dirty=true
// and preserves post-eviction accounting so the next checkpoint persists the
// evicted state.
func TestManagedHistoryConcurrentEvictionKeepsDirtyDuringFlush(t *testing.T) {
	t0 := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	clk := newReportTestClock(t0)
	store := storage.CreateMemory()
	h := NewManagedHistoryStore(ManagedHistoryStoreConfig{
		Config:          plugin.DefaultHubHistoryConfig(),
		Store:           store,
		Now:             clk.Now,
		DeclaredClients: 1,
	})
	rng := rand.New(rand.NewPCG(7, 11))
	for m := range 120 {
		ts := t0.Add(time.Duration(m) * time.Minute)
		clk.Advance(time.Minute)
		require.NoError(t, h.Ingest("edge-01", &fleet.SampleReport{
			SchemaVersion: fleet.SampleSchemaVersion,
			Samples: []fleet.SampleReading{
				{Figure: fleet.FigureCPUUsed, Value: rng.Float64() * 100},
				{Figure: fleet.FigureTrafficIn, Value: rng.Float64() * 1e9},
				{Figure: fleet.FigureTrafficOut, Value: rng.Float64() * 1e9},
			},
		}, ts))
	}

	entered := make(chan struct{})
	release := make(chan struct{})
	h.beforeShardWrite = func(dev, shard string) {
		if dev == "edge-01" && shard == historyShardNameFine {
			close(entered)
			<-release
		}
	}

	cpDone := make(chan error, 1)
	go func() {
		cpDone <- h.CheckpointOnce()
	}()
	<-entered

	// Tighten per-device budget mid-flush to trigger cap eviction while flush is paused.
	tight := plugin.HubHistoryConfig{
		IntervalSeconds:   60,
		RetentionDays:     7,
		DeviceBudgetBytes: 5 << 10,
		HubBudgetBytes:    16 << 10,
	}
	h.UpdateConfig(tight, 1)
	close(release)
	require.NoError(t, <-cpDone)
	h.beforeShardWrite = nil

	// Device MUST remain dirty so the post-eviction state is persisted on next checkpoint!
	h.mu.Lock()
	stillDirty := h.devices["edge-01"].dirty
	postEvictBytes := h.deviceAccountedBytesLocked(h.devices["edge-01"])
	h.mu.Unlock()
	require.True(t, stillDirty, "concurrent eviction during flush must keep device dirty")
	require.LessOrEqual(t, postEvictBytes, tight.DeviceBudgetBytes)

	// Next checkpoint persists the evicted state and clears dirty.
	require.NoError(t, h.CheckpointOnce())
	h.mu.Lock()
	afterSecondFlushDirty := h.devices["edge-01"].dirty
	h.mu.Unlock()
	require.False(t, afterSecondFlushDirty)
}

// TestManagedHistoryCorruptDomainValidation validates that out-of-domain stored
// figure values in latest or shard records are rejected on load under
// CodeDoctorManagedHistoryCorrupt.
func TestManagedHistoryCorruptDomainValidation(t *testing.T) {
	t0 := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	store := storage.CreateMemory()
	// Out-of-domain cpu-used (150 > 100) in persisted latest record.
	badLatest, err := json.Marshal(persistedHistoryLatest{
		SchemaVersion: historyPersistSchemaVersion,
		Device:        "edge-bad-domain",
		HasReport:     true,
		ReceivedAt:    t0,
		Values:        map[string]float64{fleet.FigureCPUUsed: 150},
	})
	require.NoError(t, err)
	require.NoError(t, store.WriteKey(historyLatestKey("edge-bad-domain"), badLatest))

	h := NewManagedHistoryStore(ManagedHistoryStoreConfig{
		Config: plugin.DefaultHubHistoryConfig(),
		Store:  store,
		Now:    func() time.Time { return t0 },
	})
	require.NoError(t, h.LoadFromStore())
	require.False(t, h.HasDevice("edge-bad-domain"))
	health := h.HealthSnapshot(t0)
	require.Len(t, health.CorruptRecords, 1)
	require.Equal(t, "edge-bad-domain", health.CorruptRecords[0].Name)
	require.Contains(t, health.CorruptRecords[0].Error, "exceeds maximum")
}

// TestManagedHistoryTightCapPreservesNewestLiveSlot validates that tightening
// device-budget-bytes to 4 KiB on a device with 3 dense older coarse slots and
// 1 recent live sample evicts the oldest slots, raises capacity pressure, stays
// within 4096 bytes, and preserves the recent live sample bucket as measured
// (never blind-batching past the final live slot).
func TestManagedHistoryTightCapPreservesNewestLiveSlot(t *testing.T) {
	base := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	liveTS := base.Add(45 * time.Minute) // 4th coarse slot [10:45, 11:00)
	h := NewManagedHistoryStore(ManagedHistoryStoreConfig{
		Config:          plugin.DefaultHubHistoryConfig(),
		Now:             func() time.Time { return liveTS },
		DeclaredClients: 1,
	})

	allFigs := fleet.AllSampleFigures()
	readings := make([]fleet.SampleReading, len(allFigs))
	rng := rand.New(rand.NewPCG(19, 23))

	// Populate 3 full old coarse slots (45 minutes x 15 figures of high-entropy data).
	for m := range 45 {
		ts := base.Add(time.Duration(m) * time.Minute)
		for i, spec := range allFigs {
			v := rng.Float64() * 1e6
			if spec.HasMax {
				v = rng.Float64() * spec.Max
			}
			readings[i] = fleet.SampleReading{Figure: spec.Name, Value: v}
		}
		require.NoError(t, h.Ingest("edge-01", &fleet.SampleReport{
			SchemaVersion: fleet.SampleSchemaVersion,
			Samples:       readings,
		}, ts))
	}

	// Add 1 brand-new live sample in the 4th coarse slot [10:45, 11:00).
	const liveCPU = 42.5
	require.NoError(t, h.Ingest("edge-01", &fleet.SampleReport{
		SchemaVersion: fleet.SampleSchemaVersion,
		Samples: []fleet.SampleReading{
			{Figure: fleet.FigureCPUUsed, Value: liveCPU},
		},
	}, liveTS))

	// Tighten cap to the minimum 4 KiB per device.
	tight := plugin.HubHistoryConfig{
		IntervalSeconds:   60,
		RetentionDays:     7,
		DeviceBudgetBytes: fleet.MinHistoryLatestReservePerDeviceBytes,
		HubBudgetBytes:    fleet.MinHistoryLatestReservePerDeviceBytes * 4,
	}
	h.UpdateConfig(tight, 1)

	status, err := h.QueryStorage(allowAllFleetScope(), nil, liveTS)
	require.NoError(t, err)
	require.Len(t, status.Devices, 1)
	require.LessOrEqual(t, status.Devices[0].UsedBytes, tight.DeviceBudgetBytes)
	require.True(t, status.CapacityPressure)

	// The newest live sample bucket at 10:45 MUST remain measured (not evicted into a gap).
	series, err := h.QuerySeries(allowAllFleetScope(), map[string]HistoryDeviceMeta{
		"edge-01": {Name: "edge-01", Declared: true, Connected: true},
	}, HistorySeriesQuery{
		Device: "edge-01",
		Figure: fleet.FigureCPUUsed,
		From:   liveTS,
		Until:  liveTS.Add(time.Minute),
	}, liveTS)
	require.NoError(t, err)
	require.Len(t, series, 1)
	require.False(t, series[0].Gap, "newest live sample bucket must survive cap tightening when it fits within limit")
	require.NotNil(t, series[0].Value)
	require.InDelta(t, liveCPU, *series[0].Value, 1e-9)
}

func TestPrepareManagedListenersWithHistoryNormalizesZeroConfig(t *testing.T) {
	store := storage.CreateMemory()
	srv, err := NewManagedServer(ManagedServerConfig{
		Store:      store,
		ReadConfig: func(string) ([]byte, error) { return nil, nil },
	})
	require.NoError(t, err)

	// Reloading a config with no bus.hub block passes zero-value HubHistoryConfig{};
	// prepareManagedListeners must normalize it to DefaultHubHistoryConfig() just
	// like NewManagedServer does.
	plan, err := srv.PrepareManagedListenersWithHistory(nil, "", plugin.HubHistoryConfig{})
	require.NoError(t, err)
	require.Equal(t, plugin.DefaultHubHistoryConfig(), plan.history)

	// Explicitly invalid non-zero history config is still refused.
	bad := plugin.DefaultHubHistoryConfig()
	bad.RetentionDays = 0
	_, err = srv.PrepareManagedListenersWithHistory(nil, "", bad)
	require.Error(t, err)
	require.Contains(t, err.Error(), "retention-days 0")
}
