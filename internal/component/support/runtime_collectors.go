// Design: docs/architecture/diagnostics/production-diagnostics.md -- core live runtime support bundle collectors
// Related: register.go -- static and core runtime support module registrations

package support

import (
	"context"
	"runtime"
	"time"

	"github.com/ze-software/ze/internal/core/badpacket"
	"github.com/ze-software/ze/internal/core/drift"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/watchdog"
)

const (
	runtimeLogRingMaxEntries = 200
	runtimeReportMaxEntries  = 64
	runtimeDriftTimeout      = 10 * time.Second
	runtimeGoroutineStackMax = 64 * 1024

	keyTruncated  = "truncated"
	keyTotalCount = "total-count"
)

// collectLogRing gathers up to runtimeLogRingMaxEntries newest records from
// the process-wide log ring, honoring opts.SinceTime when non-zero.
func collectLogRing(opts *collectOptions) (any, error) {
	ring := slogutil.GlobalLogRing()
	if ring == nil {
		return map[string]any{
			keyAvailable: false,
			keyReason:    "log ring not initialized",
		}, nil
	}

	var since time.Time
	if opts != nil {
		since = opts.SinceTime
	}

	all := ring.Recent(0)
	entries := make([]map[string]any, 0, min(len(all), runtimeLogRingMaxEntries))
	total := 0
	for i := range all {
		rec := &all[i]
		if !since.IsZero() {
			if rec.Timestamp.Before(since) {
				continue
			}
		}
		total++
		if len(entries) >= runtimeLogRingMaxEntries {
			continue
		}
		entries = append(entries, formatLogRingEntry(rec))
	}

	result := map[string]any{
		keyAvailable:  true,
		"entries":     entries,
		keyCount:      len(entries),
		keyTotalCount: total,
		keyTruncated:  total > len(entries),
	}
	if !since.IsZero() {
		result["since"] = since.UTC().Format(time.RFC3339)
	}
	return result, nil
}

func formatLogRingEntry(rec *slogutil.LogEntry) map[string]any {
	row := map[string]any{
		"timestamp": rec.Timestamp.UTC().Format(time.RFC3339),
		"level":     rec.Level,
		"component": rec.Component,
		"message":   rec.Message,
	}
	if rec.OpID != "" {
		row["operation-id"] = rec.OpID
	}
	attrs, err := rec.Attributes()
	if err != nil {
		row["attributes-error"] = err.Error()
		return row
	}
	if len(attrs) > 0 {
		row["attributes"] = attrs
	}
	return row
}

// collectReport gathers bounded snapshots of operational report-bus errors,
// active warnings, and loss counters.
func collectReport(_ *collectOptions) (any, error) {
	allErrors := report.Errors(0)
	allWarnings := report.Warnings()
	losses := report.Losses()

	errs := capReportIssues(allErrors, runtimeReportMaxEntries)
	warns := capReportIssues(allWarnings, runtimeReportMaxEntries)

	return map[string]any{
		"errors":              errs,
		"error-count":         len(errs),
		"error-total-count":   len(allErrors),
		"errors-truncated":    len(allErrors) > len(errs),
		"warnings":            warns,
		"warning-count":       len(warns),
		"warning-total-count": len(allWarnings),
		"warnings-truncated":  len(allWarnings) > len(warns),
		"losses": map[string]any{
			"warnings-evicted":   losses.WarningsEvicted,
			"warning-cap":        losses.WarningCap,
			"errors-overwritten": losses.ErrorsOverwritten,
			"error-cap":          losses.ErrorCap,
		},
	}, nil
}

func capReportIssues(issues []report.Issue, limit int) []report.Issue {
	if len(issues) == 0 {
		return []report.Issue{}
	}
	if len(issues) > limit {
		return issues[:limit]
	}
	return issues
}

// collectHealth gathers the aggregated component health registry report.
func collectHealth(_ *collectOptions) (any, error) {
	return health.Check(), nil
}

// collectWatchdog gathers the point-in-time daemon liveness watchdog snapshot.
func collectWatchdog(_ *collectOptions) (any, error) {
	return watchdog.Status(), nil
}

// collectBadPackets gathers the bounded ring of recently captured malformed
// wire frames and total recording counters.
func collectBadPackets(_ *collectOptions) (any, error) {
	records := badpacket.Snapshot(0, "")
	return map[string]any{
		"packets":        records,
		keyCount:         len(records),
		"total-recorded": badpacket.Count(),
		"capacity":       badpacket.MaxRecords,
	}, nil
}

// collectDrift runs all registered dataplane drift checkers under a 10-second
// overall timeout cap.
func collectDrift(_ *collectOptions) (any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), runtimeDriftTimeout)
	defer cancel()

	reports := drift.CheckAll(ctx)
	return map[string]any{
		"subsystems": reports,
		keyCount:     len(reports),
	}, nil
}

// collectGoroutines captures the current goroutine count and a stack dump
// capped at runtimeGoroutineStackMax bytes.
func collectGoroutines(_ *collectOptions) (any, error) {
	count := runtime.NumGoroutine()
	buf := make([]byte, runtimeGoroutineStackMax)
	n := runtime.Stack(buf, true)
	return map[string]any{
		keyCount:     count,
		keyTruncated: n == len(buf),
		"stacks":     string(buf[:n]),
	}, nil
}
