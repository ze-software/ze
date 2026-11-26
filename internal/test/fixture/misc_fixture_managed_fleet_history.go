// Design: docs/architecture/fleet-config.md -- hub-side fleet metric history reconnect, restart, cap eviction and deletion
// Related: register_managed_fleet_history.go -- registers the two fleet metric history drivers
// Related: misc_fixture_managed_fleet_hold.go -- fleetHoldRunWithHubConfig provisions the hub and device stores
// Related: misc_fixture_managed_fleet_reports.go -- fleetJSONCompactSuffix and hub CLI query pattern
// Related: misc_fixture_managed_fleet_reload.go -- fleetReloadKeepClient and live hub reload via fleetHoldImportBeside

package fixture

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/managed"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/zefs"
)

const (
	fleetHistoryLatestPrefix    = "show fleet sample latest"
	fleetHistorySeriesPrefix    = "show fleet sample series"
	fleetHistoryStorageCmd      = "show fleet sample storage" + fleetJSONCompactSuffix
	fleetHistoryRawSuffix       = " | raw"
	fleetHistoryStatusOK        = "ok"
	fleetHistoryStatusNoReports = "no-reports"
	fleetHistoryFirstCPU        = 42.0
	fleetHistorySecondCPU       = 77.0
	fleetHistoryCapLiveCPU      = 88.5
	fleetHistoryFirstRoutes     = 0.0
	fleetHistorySecondRoutes    = 5.0
	fleetHistoryCapLiveRoutes   = 12.0
	fleetHistoryInitialDevice   = 65536
	fleetHistoryInitialHub      = 131072
	fleetHistoryTightDevice     = 4096
	fleetHistoryTightHub        = 8192
	fleetHistorySampleInterval  = 1
	fleetHistorySeedMinutes     = 45
	fleetHistoryFloatTol        = 1e-6
)

// fleetHistoryClientHandle owns one managed.RunManagedClient session worker and
// its borrowed device store. Callers MUST invoke stop before returning.
type fleetHistoryClientHandle struct {
	cancel context.CancelFunc
	done   chan struct{}
	store  storage.Storage
	once   sync.Once
}

func (h *fleetHistoryClientHandle) run(ctx context.Context, cfg managed.ClientConfig) {
	defer close(h.done)
	managed.RunManagedClient(ctx, cfg)
}

func (h *fleetHistoryClientHandle) stop() error {
	var closeErr error
	h.once.Do(func() {
		h.cancel()
		<-h.done
		if h.store != nil {
			closeErr = h.store.Close()
		}
	})
	return closeErr
}

func fleetHistoryHubConfig(sshPort, pluginPort, managedPort, clients string, deviceBudget, hubBudget int64) string {
	var b textbuf.Buffer
	b.Str(restoreClientSSHListener(sshPort))
	b.Str("bus {\n    hub {\n")
	b.Str("        server local { ip 127.0.0.1; port ").Str(pluginPort).
		Str("; secret \"").Str(caTrustPluginSecret).Str("\"; }\n")
	b.Str("        server remote-fleet { ip 127.0.0.1; port ").Str(managedPort).
		Str("; secret \"").Str(caTrustHubSecret).Str("\";\n")
	b.Str(clients)
	b.Str("        }\n")
	b.Str("        history {\n")
	b.Str("            interval ").Int(fleetHistorySampleInterval).Str(";\n")
	b.Str("            retention-days ").Int(fleet.DefaultHistoryRetentionDays).Str(";\n")
	b.Str("            device-budget-bytes ").Int(deviceBudget).Str(";\n")
	b.Str("            hub-budget-bytes ").Int(hubBudget).Str(";\n")
	b.Str("        }\n")
	b.Str("    }\n}\n")
	b.Str("bgp {\n    session { asn { local 65000; } }\n    router-id 10.0.0.1\n}\n")
	return b.String()
}

func fleetHistoryReconnectHubConfig(sshPort, pluginPort, managedPort string) string {
	clients := "            client " + caTrustClientName + " { secret \"" + caTrustClientSecret +
		"\"; label site { value lon; } }\n            client " + fleetReportPeerName + " { secret \"" + caTrustClientSecret +
		"\"; label site { value par; } }\n"
	return fleetHistoryHubConfig(
		sshPort, pluginPort, managedPort, clients,
		fleet.DefaultHistoryPerDeviceBytes, fleet.DefaultHistoryHubBytes,
	)
}

func fleetHistoryCapHubConfig(sshPort, pluginPort, managedPort string) string {
	clients := "            client " + caTrustClientName + " { secret \"" + caTrustClientSecret +
		"\"; label site { value lon; } }\n            client " + fleetReloadKeepClient + " { secret \"" + fleetReloadKeepSecret +
		"\"; label site { value par; } }\n"
	return fleetHistoryHubConfig(
		sshPort, pluginPort, managedPort, clients,
		fleetHistoryInitialDevice, fleetHistoryInitialHub,
	)
}

func (s *fleetHoldScenario) startControlledSampleClient(
	ctx context.Context,
	readings []fleet.SampleReading,
) (*fleetHistoryClientHandle, error) {
	devStore, err := openDaemonStore(s.device.env, s.device.dir)
	if err != nil {
		return nil, fmt.Errorf("open device store for controlled managed client: %w", err)
	}
	clientCtx, cancel := context.WithCancel(ctx)
	var sentOnce atomic.Bool
	payload := append([]fleet.SampleReading(nil), readings...)

	cfg := managed.ClientConfig{
		Name:        caTrustClientName,
		Server:      s.hubAddress,
		Token:       caTrustClientSecret,
		TLSInsecure: true,
		Handler: &managed.Handler{
			Validate: func(_ []byte) error { return nil },
		},
		Baseline: managed.NewBaselineAccess(devStore),
		CommitOver: func(data []byte, check func(active []byte) error) error {
			if check != nil {
				active, readErr := storage.ReadActiveConfig(devStore, caTrustClientName+".conf")
				if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
					return readErr
				}
				if err := check(active); err != nil {
					return err
				}
			}
			if err := storage.ImportConfig(devStore, caTrustClientName+".conf", data); err != nil {
				return err
			}
			_, err := managed.RecordActiveAsBaseline(devStore)
			return err
		},
		SampleCollector: func(_ context.Context, now time.Time) (fleet.SampleReport, error) {
			if sentOnce.Swap(true) {
				// Emit only the controlled observation for this phase so no extra
				// sample can land in the following UTC minute during disconnect.
				return fleet.SampleReport{SchemaVersion: fleet.SampleSchemaVersion}, nil
			}
			return fleet.SampleReport{
				SchemaVersion: fleet.SampleSchemaVersion,
				DeviceTime:    now.UTC().Format(time.RFC3339Nano),
				Samples:       append([]fleet.SampleReading(nil), payload...),
			}, nil
		},
	}

	handle := &fleetHistoryClientHandle{
		cancel: cancel,
		done:   make(chan struct{}),
		store:  devStore,
	}
	go handle.run(clientCtx, cfg)
	return handle, nil
}

func (s *fleetHoldScenario) queryHistoryLatest(ctx context.Context, pipes string) ([]pluginserver.HistoryLatestRow, string, error) {
	cmd := fleetHistoryLatestPrefix + pipes + fleetHistoryRawSuffix
	raw, err := storageLiveRequest(ctx, s.hubOperator, cmd)
	if err != nil {
		return nil, raw, fmt.Errorf("%s: %w: %s", cmd, err, raw)
	}
	var rows []pluginserver.HistoryLatestRow
	if err := fleetDivergenceDecode(raw, &rows); err != nil {
		return nil, raw, fmt.Errorf("%s: %w: %s", cmd, err, raw)
	}
	return rows, raw, nil
}

func (s *fleetHoldScenario) queryHistorySeries(ctx context.Context, from, until time.Time) ([]pluginserver.HistorySeriesRow, string, error) {
	var b textbuf.Buffer
	b.Str(fleetHistorySeriesPrefix)
	b.Str(" | device ").Str(caTrustClientName)
	b.Str(" | figure ").Str(fleet.FigureCPUUsed)
	b.Str(" | from ").Str(from.UTC().Format(time.RFC3339))
	b.Str(" | until ").Str(until.UTC().Format(time.RFC3339))
	b.Str(fleetHistoryRawSuffix)
	cmd := b.String()
	raw, err := storageLiveRequest(ctx, s.hubOperator, cmd)
	if err != nil {
		return nil, raw, fmt.Errorf("%s: %w: %s", cmd, err, raw)
	}
	var rows []pluginserver.HistorySeriesRow
	if err := fleetDivergenceDecode(raw, &rows); err != nil {
		return nil, raw, fmt.Errorf("%s: %w: %s", cmd, err, raw)
	}
	return rows, raw, nil
}

func (s *fleetHoldScenario) queryHistoryStorage(ctx context.Context) (pluginserver.HistoryStorageStatus, string, error) {
	raw, err := storageLiveRequest(ctx, s.hubOperator, fleetHistoryStorageCmd)
	if err != nil {
		return pluginserver.HistoryStorageStatus{}, raw, fmt.Errorf("%s: %w: %s", fleetHistoryStorageCmd, err, raw)
	}
	var status pluginserver.HistoryStorageStatus
	if err := fleetDivergenceDecode(raw, &status); err != nil {
		return pluginserver.HistoryStorageStatus{}, raw, fmt.Errorf("%s: %w: %s", fleetHistoryStorageCmd, err, raw)
	}
	return status, raw, nil
}

func floatNear(got, want float64) bool {
	return math.Abs(got-want) <= fleetHistoryFloatTol
}

func waitUntilUTC(ctx context.Context, target time.Time) error {
	wait := time.Until(target)
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *fleetHoldScenario) waitLatestRow(
	ctx context.Context,
	predicate func(pluginserver.HistoryLatestRow) bool,
	desc string,
) (pluginserver.HistoryLatestRow, error) {
	var matched pluginserver.HistoryLatestRow
	var lastRaw string
	var lastErr error
	pipes := " | device " + caTrustClientName
	ok := Poll(ctx, fleetHoldDoctorAttempts, fleetStatusPollDelay, func() bool {
		rows, raw, err := s.queryHistoryLatest(ctx, pipes)
		lastRaw = raw
		lastErr = err
		if err != nil {
			return false
		}
		if len(rows) != 1 {
			return false
		}
		if !predicate(rows[0]) {
			return false
		}
		matched = rows[0]
		return true
	})
	if !ok {
		return pluginserver.HistoryLatestRow{}, fmt.Errorf("%s did not reach expected latest state (err=%w): %s", desc, lastErr, lastRaw)
	}
	return matched, nil
}

func verifyMeasuredSeriesBucket(row *pluginserver.HistorySeriesRow, want float64, label, phase string) error {
	if row.Gap {
		return fmt.Errorf("%s: %s bucket unexpectedly marked gap", phase, label)
	}
	if row.Value == nil {
		return fmt.Errorf("%s: %s bucket missing value, want %g", phase, label, want)
	}
	if !floatNear(*row.Value, want) {
		return fmt.Errorf("%s: %s bucket value is %g, want %g", phase, label, *row.Value, want)
	}
	if row.Count < 1 {
		return fmt.Errorf("%s: %s bucket count=%d, want >= 1", phase, label, row.Count)
	}
	if row.ObservedMinutes != 1 {
		return fmt.Errorf("%s: %s bucket observed-minutes=%d, want 1", phase, label, row.ObservedMinutes)
	}
	return nil
}

func verifyGapSeriesBucket(row *pluginserver.HistorySeriesRow, bucketMin int64, phase string) error {
	if !row.Gap {
		return fmt.Errorf("%s: offline outage bucket %d is not marked gap (value=%v count=%d)", phase, bucketMin, row.Value, row.Count)
	}
	if row.Value != nil {
		return fmt.Errorf("%s: offline outage bucket %d unexpectedly carries value %g", phase, bucketMin, *row.Value)
	}
	if row.Count != 0 {
		return fmt.Errorf("%s: offline outage bucket %d count=%d, want 0", phase, bucketMin, row.Count)
	}
	if row.ObservedMinutes != 0 {
		return fmt.Errorf("%s: offline outage bucket %d observed-minutes=%d, want 0", phase, bucketMin, row.ObservedMinutes)
	}
	return nil
}

func verifyReconnectSeriesBuckets(
	rows []pluginserver.HistorySeriesRow,
	firstMinute, secondMinute int64,
	phase string,
) error {
	if len(rows) < 3 {
		return fmt.Errorf("%s: expected at least 3 series buckets across [%d..%d], got %d", phase, firstMinute, secondMinute, len(rows))
	}
	var sawFirst, sawGap, sawSecond bool
	for i := range rows {
		row := &rows[i]
		bucketMin := row.Start.UTC().Unix() / 60
		switch {
		case bucketMin == firstMinute:
			if err := verifyMeasuredSeriesBucket(row, fleetHistoryFirstCPU, "first sample", phase); err != nil {
				return err
			}
			sawFirst = true
		case bucketMin > firstMinute && bucketMin < secondMinute:
			if err := verifyGapSeriesBucket(row, bucketMin, phase); err != nil {
				return err
			}
			sawGap = true
		case bucketMin == secondMinute:
			if err := verifyMeasuredSeriesBucket(row, fleetHistorySecondCPU, "reconnect sample", phase); err != nil {
				return err
			}
			sawSecond = true
		}
	}
	if !sawFirst {
		return fmt.Errorf("%s: series missing first sample bucket %d", phase, firstMinute)
	}
	if !sawGap {
		return fmt.Errorf("%s: series missing offline outage gap bucket between %d and %d", phase, firstMinute, secondMinute)
	}
	if !sawSecond {
		return fmt.Errorf("%s: series missing reconnect sample bucket %d", phase, secondMinute)
	}
	return nil
}

func verifyFigureReading(figures map[string]float64, name string, want float64, contextLabel string) error {
	got, ok := figures[name]
	if !ok {
		return fmt.Errorf("%s: figure %s missing from latest row (%+v)", contextLabel, name, figures)
	}
	if !floatNear(got, want) {
		return fmt.Errorf("%s: figure %s is %g, want %g", contextLabel, name, got, want)
	}
	return nil
}

func latestRowMatchesOnline(r *pluginserver.HistoryLatestRow, wantCPU, wantRoutes float64) bool {
	if !r.Connected {
		return false
	}
	if !r.HasReport {
		return false
	}
	if r.Status != fleetHistoryStatusOK {
		return false
	}
	if r.ReceivedAt.IsZero() {
		return false
	}
	cpu, hasCPU := r.Figures[fleet.FigureCPUUsed]
	if !hasCPU {
		return false
	}
	if !floatNear(cpu, wantCPU) {
		return false
	}
	routes, hasRoutes := r.Figures[fleet.FigureRoutesHeld]
	if !hasRoutes {
		return false
	}
	return floatNear(routes, wantRoutes)
}

// managedFleetHistoryReconnectDriver validates AC-1, AC-3, AC-4, AC-11, and
// AC-12 against a real ze hub daemon: an unreported device reports no-reports,
// a connected managed client pushes a measured reading including explicit zero
// while omitting absent figures, disconnecting preserves latest and leaves an
// actual gap bucket across the offline UTC minute, reconnecting records a fresh
// reading, and restarting the hub preserves latest, series, and gap intact.
//
// Args: <hub-ssh-port> <hub-plugin-port> <hub-managed-port>.
func managedFleetHistoryReconnectDriver(ctx context.Context, args []string) error {
	return fleetHoldRunWithHubConfig(ctx, "managed/fleet-history-reconnect", args, false, fleetHistoryReconnectHubConfig,
		func(ctx context.Context, s *fleetHoldScenario) error {
			// AC-11: declared device that has never sent samples reports no-reports.
			unreportedRows, rawUnrep, err := s.queryHistoryLatest(ctx, " | device "+fleetReportPeerName)
			if err != nil {
				return err
			}
			if len(unreportedRows) != 1 {
				return fmt.Errorf("unreported device %s returned %d rows, want 1: %s", fleetReportPeerName, len(unreportedRows), rawUnrep)
			}
			unrep := unreportedRows[0]
			if unrep.HasReport {
				return fmt.Errorf("unreported device %s unexpectedly has-report=true: %s", fleetReportPeerName, rawUnrep)
			}
			if unrep.Status != fleetHistoryStatusNoReports {
				return fmt.Errorf("unreported device %s status=%q, want %q: %s", fleetReportPeerName, unrep.Status, fleetHistoryStatusNoReports, rawUnrep)
			}

			// Phase 1: connect real managed client and push cpu-used=42, routes-held=0.
			firstReadings := []fleet.SampleReading{
				{Figure: fleet.FigureCPUUsed, Value: fleetHistoryFirstCPU},
				{Figure: fleet.FigureRoutesHeld, Value: fleetHistoryFirstRoutes},
			}
			firstClient, err := s.startControlledSampleClient(ctx, firstReadings)
			if err != nil {
				return err
			}
			firstOnline, err := s.waitLatestRow(ctx, func(r pluginserver.HistoryLatestRow) bool {
				return latestRowMatchesOnline(&r, fleetHistoryFirstCPU, fleetHistoryFirstRoutes)
			}, "first sample report online")
			if err != nil {
				_ = firstClient.stop() //nolint:errcheck // best-effort cleanup on error path
				return err
			}
			if _, hasSub := firstOnline.Figures[fleet.FigureSubscriberSessions]; hasSub {
				_ = firstClient.stop() //nolint:errcheck // best-effort cleanup on error path
				return fmt.Errorf("absent figure %s unexpectedly present in latest row: %+v", fleet.FigureSubscriberSessions, firstOnline.Figures)
			}
			if err := firstClient.stop(); err != nil {
				return fmt.Errorf("stop first managed client: %w", err)
			}

			// Verify latest retains value and receipt time after disconnect.
			firstOffline, err := s.waitLatestRow(ctx, func(r pluginserver.HistoryLatestRow) bool {
				return !r.Connected && r.HasReport && r.Status == fleetHistoryStatusOK
			}, "first sample retained offline")
			if err != nil {
				return err
			}
			if !firstOffline.ReceivedAt.Equal(firstOnline.ReceivedAt) {
				return fmt.Errorf("offline received-at changed from %s to %s", firstOnline.ReceivedAt, firstOffline.ReceivedAt)
			}
			if err := verifyFigureReading(firstOffline.Figures, fleet.FigureCPUUsed, fleetHistoryFirstCPU, "offline after first sample"); err != nil {
				return err
			}
			if firstOffline.AgeSeconds < 0 {
				return fmt.Errorf("offline age-seconds is negative: %d", firstOffline.AgeSeconds)
			}

			// Wait for one full empty UTC minute (firstMinute + 1) to elapse while
			// offline so the hub's 1-minute fine bucket ring records a genuine gap
			// between the first observation and the reconnect observation.
			firstMinute := firstOnline.ReceivedAt.UTC().Unix() / 60
			reconnectTarget := time.Unix((firstMinute+2)*60+1, 0).UTC()
			if err := waitUntilUTC(ctx, reconnectTarget); err != nil {
				return fmt.Errorf("wait for offline UTC gap minute: %w", err)
			}

			// Phase 2: reconnect real managed client and push cpu-used=77, routes-held=5.
			secondReadings := []fleet.SampleReading{
				{Figure: fleet.FigureCPUUsed, Value: fleetHistorySecondCPU},
				{Figure: fleet.FigureRoutesHeld, Value: fleetHistorySecondRoutes},
			}
			secondClient, err := s.startControlledSampleClient(ctx, secondReadings)
			if err != nil {
				return err
			}
			secondOnline, err := s.waitLatestRow(ctx, func(r pluginserver.HistoryLatestRow) bool {
				if !r.ReceivedAt.After(firstOnline.ReceivedAt) {
					return false
				}
				return latestRowMatchesOnline(&r, fleetHistorySecondCPU, fleetHistorySecondRoutes)
			}, "reconnect sample report online")
			if err != nil {
				_ = secondClient.stop() //nolint:errcheck // best-effort cleanup on error path
				return err
			}
			if err := secondClient.stop(); err != nil {
				return fmt.Errorf("stop second managed client: %w", err)
			}
			if _, err := s.waitLatestRow(ctx, func(r pluginserver.HistoryLatestRow) bool {
				return !r.Connected && r.HasReport
			}, "second sample offline before hub restart"); err != nil {
				return err
			}

			secondMinute := secondOnline.ReceivedAt.UTC().Unix() / 60
			if secondMinute < firstMinute+2 {
				return fmt.Errorf("reconnect sample landed in minute %d, want >= %d so an offline gap minute exists", secondMinute, firstMinute+2)
			}
			rangeFrom := time.Unix(firstMinute*60, 0).UTC()
			rangeUntil := time.Unix((secondMinute+1)*60, 0).UTC()

			seriesBefore, rawSeriesBefore, err := s.queryHistorySeries(ctx, rangeFrom, rangeUntil)
			if err != nil {
				return err
			}
			if err := verifyReconnectSeriesBuckets(seriesBefore, firstMinute, secondMinute, "before hub restart"); err != nil {
				return fmt.Errorf("%w: %s", err, rawSeriesBefore)
			}

			// AC-3: restart the hub cleanly (FlushFinal -> LoadFromStore) and verify
			// latest, receipt timestamp, series values, and outage gap all survive.
			if err := s.restartHub(ctx); err != nil {
				return fmt.Errorf("restart hub to verify persisted metric history: %w", err)
			}
			afterRestart, err := s.waitLatestRow(ctx, func(r pluginserver.HistoryLatestRow) bool {
				return !r.Connected && r.HasReport && r.Status == fleetHistoryStatusOK
			}, "latest after hub restart")
			if err != nil {
				return err
			}
			if !afterRestart.ReceivedAt.Equal(secondOnline.ReceivedAt) {
				return fmt.Errorf("after hub restart received-at changed from %s to %s", secondOnline.ReceivedAt, afterRestart.ReceivedAt)
			}
			if err := verifyFigureReading(afterRestart.Figures, fleet.FigureCPUUsed, fleetHistorySecondCPU, "after hub restart"); err != nil {
				return err
			}
			if err := verifyFigureReading(afterRestart.Figures, fleet.FigureRoutesHeld, fleetHistorySecondRoutes, "after hub restart"); err != nil {
				return err
			}

			seriesAfter, rawSeriesAfter, err := s.queryHistorySeries(ctx, rangeFrom, rangeUntil)
			if err != nil {
				return err
			}
			if err := verifyReconnectSeriesBuckets(seriesAfter, firstMinute, secondMinute, "after hub restart"); err != nil {
				return fmt.Errorf("%w: %s", err, rawSeriesAfter)
			}

			return storageLiveOK("fleet metric history retained across disconnect, showed offline gap, updated on reconnect, and persisted across hub restart")
		})
}

func seedHistoricalShardsForCap(s *fleetHoldScenario, slot15Start time.Time) (oldestSeeded time.Time, err error) {
	hubStore, err := openDaemonStore(s.hub.env, s.hub.dir)
	if err != nil {
		return time.Time{}, fmt.Errorf("open hub store to seed historical shards: %w", err)
	}
	defer func() {
		err = errors.Join(err, hubStore.Close())
	}()

	hist := pluginserver.NewManagedHistoryStore(pluginserver.ManagedHistoryStoreConfig{
		Config: plugin.HubHistoryConfig{
			IntervalSeconds:   fleet.DefaultSampleIntervalSeconds,
			RetentionDays:     fleet.DefaultHistoryRetentionDays,
			DeviceBudgetBytes: fleetHistoryInitialDevice,
			HubBudgetBytes:    fleetHistoryInitialHub,
		},
		Store:           hubStore,
		DeclaredClients: 2,
	})

	// Seed 45 minutes strictly before the current 15-minute coarse slot so every
	// seeded point sits in older coarse slots (S-3, S-2, S-1) and never shares a
	// coarse slot with the live sample pushed in slot S. Use deterministic
	// pseudo-random values across all 15 figures so compressed shards exceed
	// 4,096 bytes and trigger real oldest-first eviction when the cap tightens.
	figures := fleet.AllSampleFigures()
	start := slot15Start.Add(-time.Duration(fleetHistorySeedMinutes) * time.Minute)
	var state uint64 = 0x9e3779b97f4a7c15
	nextRand := func() uint64 {
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		return state
	}

	for m := range fleetHistorySeedMinutes {
		ts := start.Add(time.Duration(m)*time.Minute + 15*time.Second)
		readings := make([]fleet.SampleReading, 0, len(figures))
		for _, spec := range figures {
			r := nextRand()
			var val float64
			if spec.HasMax {
				val = float64(r%9900)/100.0 + 0.37
			} else {
				val = float64(r%1_000_000) + float64(r%997)/997.0 + 1.0
			}
			readings = append(readings, fleet.SampleReading{Figure: spec.Name, Value: val})
		}
		rep := &fleet.SampleReport{
			SchemaVersion: fleet.SampleSchemaVersion,
			DeviceTime:    ts.Format(time.RFC3339),
			Samples:       readings,
		}
		if err := hist.Ingest(caTrustClientName, rep, ts); err != nil {
			return time.Time{}, fmt.Errorf("seed minute %d: %w", m, err)
		}
	}
	if err := hist.FlushFinal(); err != nil {
		return time.Time{}, fmt.Errorf("flush seeded history shards: %w", err)
	}
	return start, nil
}

func restartHubStopped(ctx context.Context, s *fleetHoldScenario, whileStopped func() error) error {
	stopManagedProcess(s.hub.command, s.hub.done)
	s.hub.command = &exec.Cmd{}
	if err := whileStopped(); err != nil {
		return err
	}
	if err := caTrustStart(ctx, s.hub); err != nil {
		return err
	}
	var lastAnswer string
	var lastErr error
	answered := Poll(ctx, fleetHoldDoctorAttempts, fleetStatusPollDelay, func() bool {
		lastAnswer, lastErr = storageLiveRequest(ctx, s.hubOperator, "show version")
		if lastErr != nil {
			return false
		}
		return lastAnswer != ""
	})
	if !answered {
		return fmt.Errorf("hub did not answer over SSH after restart (err=%w): %s", lastErr, lastAnswer)
	}
	return nil
}

func storageReachedTightCap(st *pluginserver.HistoryStorageStatus) bool {
	if st.DeviceBudgetBytes != fleetHistoryTightDevice {
		return false
	}
	if st.HubBudgetBytes != fleetHistoryTightHub {
		return false
	}
	if !st.CapacityPressure {
		return false
	}
	if st.UsedBytes > st.HubBudgetBytes {
		return false
	}
	return st.UsedBytes <= fleetHistoryTightDevice
}

// managedFleetHistoryCapDriver validates AC-5, AC-7, and AC-15 against a real
// ze hub daemon: tightening the configured byte cap evicts older buckets first
// while retaining recent live samples and raising capacity-pressure in storage
// status and show doctor; undeclaring the device preserves its history; deleting
// its retained device record erases its history both live and after hub restart.
//
// Args: <hub-ssh-port> <hub-plugin-port> <hub-managed-port>.
func managedFleetHistoryCapDriver(ctx context.Context, args []string) error {
	return fleetHoldRunWithHubConfig(ctx, "managed/fleet-history-cap", args, false, fleetHistoryCapHubConfig,
		func(ctx context.Context, s *fleetHoldScenario) error {
			// Seed 45 minutes of older coarse slots strictly prior to the current
			// 15-minute UTC slot while the hub is stopped, then start the hub.
			curSlot15Start := time.Unix((time.Now().UTC().Unix()/900)*900, 0).UTC()
			var seededStart time.Time
			if err := restartHubStopped(ctx, s, func() error {
				var seedErr error
				seededStart, seedErr = seedHistoricalShardsForCap(s, curSlot15Start)
				return seedErr
			}); err != nil {
				return err
			}

			beforeCap, rawBeforeCap, err := s.queryHistoryStorage(ctx)
			if err != nil {
				return err
			}
			if beforeCap.UsedBytes <= fleetHistoryTightDevice {
				return fmt.Errorf("seeded hub history used-bytes=%d, want > %d so tightening cap triggers real eviction: %s",
					beforeCap.UsedBytes, fleetHistoryTightDevice, rawBeforeCap)
			}
			if beforeCap.CapacityPressure {
				return fmt.Errorf("expected capacity-pressure=false under initial 64 KiB budget: %s", rawBeforeCap)
			}
			oldestBeforeCap := beforeCap.OldestSample

			// Push a fresh live sample from the real managed client into the current
			// 15-minute slot so we prove recent points survive cap eviction.
			liveReadings := []fleet.SampleReading{
				{Figure: fleet.FigureCPUUsed, Value: fleetHistoryCapLiveCPU},
				{Figure: fleet.FigureRoutesHeld, Value: fleetHistoryCapLiveRoutes},
			}
			liveClient, err := s.startControlledSampleClient(ctx, liveReadings)
			if err != nil {
				return err
			}
			liveLatest, err := s.waitLatestRow(ctx, func(r pluginserver.HistoryLatestRow) bool {
				return latestRowMatchesOnline(&r, fleetHistoryCapLiveCPU, fleetHistoryCapLiveRoutes)
			}, "live sample before cap tightening")
			if err != nil {
				_ = liveClient.stop() //nolint:errcheck // best-effort cleanup on error path
				return err
			}
			if err := liveClient.stop(); err != nil {
				return fmt.Errorf("stop live managed client: %w", err)
			}

			// Wait until the device record is persisted on disk so the subsequent
			// delete data key call targets the real persisted registry record.
			recordPersisted := Poll(ctx, fleetHoldDoctorAttempts, fleetStatusPollDelay, func() bool {
				raw, reqErr := storageLiveRequest(ctx, s.hubOperator, "show fleet device name "+caTrustClientName+fleetJSONCompactSuffix)
				if reqErr != nil {
					return false
				}
				var detail struct {
					Connected *bool `json:"connected"`
					Persisted bool  `json:"persisted"`
				}
				if fleetDivergenceDecode(raw, &detail) != nil {
					return false
				}
				if detail.Connected == nil {
					return false
				}
				if *detail.Connected {
					return false
				}
				return detail.Persisted
			})
			if !recordPersisted {
				return fmt.Errorf("device record for %s was not persisted offline before cap tightening", caTrustClientName)
			}

			// AC-5: live reload tightening per-device cap to 4 KiB and hub cap to 8 KiB.
			twoClients := "            client " + caTrustClientName + " { secret \"" + caTrustClientSecret +
				"\"; label site { value lon; } }\n            client " + fleetReloadKeepClient + " { secret \"" + fleetReloadKeepSecret +
				"\"; label site { value par; } }\n"
			tightCfg := fleetHistoryHubConfig(
				args[0], args[1], args[2], twoClients,
				fleetHistoryTightDevice, fleetHistoryTightHub,
			)
			if err := fleetHoldImportBeside(ctx, s.hubImporter, s.hub, caTrustHubName+".conf", []byte(tightCfg)); err != nil {
				return fmt.Errorf("reload hub with tight history budgets: %w", err)
			}

			var afterCap pluginserver.HistoryStorageStatus
			var rawAfterCap string
			pressured := Poll(ctx, fleetHoldDoctorAttempts, fleetStatusPollDelay, func() bool {
				st, raw, qErr := s.queryHistoryStorage(ctx)
				rawAfterCap = raw
				if qErr != nil {
					return false
				}
				if !storageReachedTightCap(&st) {
					return false
				}
				afterCap = st
				return true
			})
			if !pressured {
				return fmt.Errorf("tightening history budget did not signal capacity-pressure within cap: %s", rawAfterCap)
			}
			if !afterCap.OldestSample.After(oldestBeforeCap) {
				return fmt.Errorf("oldest-sample did not advance after cap eviction: before=%s after=%s (%s)",
					oldestBeforeCap, afterCap.OldestSample, rawAfterCap)
			}
			if afterCap.EffectiveRetentionSeconds >= int64(fleet.DefaultHistoryRetentionDays*24*3600) {
				return fmt.Errorf("effective-retention-seconds did not shorten under cap pressure: %d (%s)",
					afterCap.EffectiveRetentionSeconds, rawAfterCap)
			}

			// Verify show doctor surfaces doctor-managed-history-capacity-pressure.
			var doctorOut string
			doctorRaised := Poll(ctx, fleetHoldDoctorAttempts, fleetStatusPollDelay, func() bool {
				doctorOut, _ = storageLiveRequest(ctx, s.hubOperator, "show doctor") //nolint:errcheck // doctor warnings may exit non-zero
				return strings.Contains(doctorOut, pluginserver.CodeDoctorManagedHistoryCapacityPressure)
			})
			if !doctorRaised {
				return fmt.Errorf("show doctor did not report %s:\n%s",
					pluginserver.CodeDoctorManagedHistoryCapacityPressure, doctorOut)
			}

			// Verify oldest seeded bucket was evicted while the recent live sample survived.
			evictedRows, rawEvicted, err := s.queryHistorySeries(ctx, seededStart, seededStart.Add(15*time.Minute))
			if err != nil {
				return err
			}
			for i := range evictedRows {
				if !evictedRows[i].Gap {
					return fmt.Errorf("oldest seeded bucket %s was not evicted under cap pressure: %s", evictedRows[i].Start, rawEvicted)
				}
			}
			liveMinuteStart := time.Unix((liveLatest.ReceivedAt.UTC().Unix()/60)*60, 0).UTC()
			recentRows, rawRecent, err := s.queryHistorySeries(ctx, liveMinuteStart, liveMinuteStart.Add(time.Minute))
			if err != nil {
				return err
			}
			if len(recentRows) != 1 {
				return fmt.Errorf("recent live sample returned %d rows, want 1: %s", len(recentRows), rawRecent)
			}
			if err := verifyMeasuredSeriesBucket(&recentRows[0], fleetHistoryCapLiveCPU, "recent live sample", "after cap eviction"); err != nil {
				return fmt.Errorf("%w: %s", err, rawRecent)
			}

			// AC-7 (part 1): undeclare edge-01 via live reload and verify history is preserved.
			keepOnly := "            client " + fleetReloadKeepClient + " { secret \"" + fleetReloadKeepSecret + "\"; label site { value par; } }\n"
			undeclaredCfg := fleetHistoryHubConfig(
				args[0], args[1], args[2], keepOnly,
				fleetHistoryTightDevice, fleetHistoryTightHub,
			)
			if err := fleetHoldImportBeside(ctx, s.hubImporter, s.hub, caTrustHubName+".conf", []byte(undeclaredCfg)); err != nil {
				return fmt.Errorf("reload hub undeclaring %s: %w", caTrustClientName, err)
			}
			undeclaredLatest, err := s.waitLatestRow(ctx, func(r pluginserver.HistoryLatestRow) bool {
				return !r.Declared && r.HasReport && r.Status == fleetHistoryStatusOK
			}, "history preserved after undeclaration")
			if err != nil {
				return err
			}
			if err := verifyFigureReading(undeclaredLatest.Figures, fleet.FigureCPUUsed, fleetHistoryCapLiveCPU, "undeclared device"); err != nil {
				return err
			}

			// AC-7 (part 2) & AC-15: delete the retained device record on the running hub,
			// confirming history is erased live AND stays gone after hub restart.
			recordKey := strings.Replace(zefs.KeyFleetDeviceRecord.Pattern, "{name}", caTrustClientName, 1)
			if out, rmErr := storageLiveRequest(ctx, s.hubOperator, "delete data key "+recordKey); rmErr != nil {
				return fmt.Errorf("delete data key %s: %w: %s", recordKey, rmErr, out)
			}

			var erasedRaw string
			erasedLive := Poll(ctx, fleetHoldDoctorAttempts, fleetStatusPollDelay, func() bool {
				rows, raw, qErr := s.queryHistoryLatest(ctx, " | device "+caTrustClientName)
				erasedRaw = raw
				if qErr != nil {
					return false
				}
				if len(rows) != 0 {
					return false
				}
				st, stRaw, stErr := s.queryHistoryStorage(ctx)
				erasedRaw = stRaw
				if stErr != nil {
					return false
				}
				if st.DeviceCount != 0 {
					return false
				}
				return len(st.Devices) == 0
			})
			if !erasedLive {
				return fmt.Errorf("deleting device record %s did not erase live history: %s", recordKey, erasedRaw)
			}

			if err := s.restartHub(ctx); err != nil {
				return fmt.Errorf("restart hub after device record deletion: %w", err)
			}
			rowsAfterRestart, rawRowsRestart, err := s.queryHistoryLatest(ctx, " | device "+caTrustClientName)
			if err != nil {
				return err
			}
			if len(rowsAfterRestart) != 0 {
				return fmt.Errorf("deleted device %s reappeared in latest after hub restart: %s", caTrustClientName, rawRowsRestart)
			}
			seriesAfterRestart, rawSeriesRestart, err := s.queryHistorySeries(ctx, liveMinuteStart, liveMinuteStart.Add(time.Minute))
			if err != nil {
				return err
			}
			if len(seriesAfterRestart) != 0 {
				return fmt.Errorf("deleted device %s reappeared in series after hub restart: %s", caTrustClientName, rawSeriesRestart)
			}
			storageAfterRestart, rawStorageRestart, err := s.queryHistoryStorage(ctx)
			if err != nil {
				return err
			}
			if storageAfterRestart.DeviceCount != 0 {
				return fmt.Errorf("deleted device %s still counted in storage device-count=%d after hub restart: %s",
					caTrustClientName, storageAfterRestart.DeviceCount, rawStorageRestart)
			}
			if len(storageAfterRestart.Devices) != 0 {
				return fmt.Errorf("deleted device %s still listed in storage devices after hub restart: %s", caTrustClientName, rawStorageRestart)
			}

			return storageLiveOK("fleet metric history evicted oldest buckets under cap pressure, preserved undeclared history, and stayed erased after deletion and hub restart")
		})
}
