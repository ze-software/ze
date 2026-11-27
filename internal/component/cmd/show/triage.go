// Design: docs/architecture/diagnostics/production-diagnostics.md -- single-call live runtime triage snapshot
// Related: system.go -- querySystemMap and sibling show system handlers
// Related: show.go -- ze-cmd:show-system-triage registration and shared response keys

package show

import (
	"maps"
	"runtime"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/badpacket"
	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/slogutil"
	zeversion "github.com/ze-software/ze/internal/core/version"
	"github.com/ze-software/ze/internal/core/watchdog"
)

// Stated bounds for each list section in a single triage snapshot.
const (
	triageMaxErrors   = 32
	triageMaxWarnings = 64
	triageMaxCrashes  = 16
	triageMaxLogs     = 32
)

// Repeated JSON keys shared across triage sections.
const (
	triageKeyAvailable  = "available"
	triageKeyReason     = "reason"
	triageKeyTotalCount = "total-count"
	triageKeyTruncated  = "truncated"
	triageKeyStatus     = "status"
	triageKeyName       = "name"
	triageKeyError      = "error"
	triageKeyErrors     = "errors"
	triageKeyWarnings   = "warnings"
	triageKeyEntries    = "entries"
	triageKeyTotal      = "total"
	triageKeyRunning    = "running"
)

// handleShowSystemTriage returns a single bounded live runtime triage snapshot
// combining build provenance, component health, daemon doctor findings,
// operational report-bus issues, recent crashes, plugin health summary, reload
// state, recent warning/error logs, and process runtime vitals.
func handleShowSystemTriage(ctx *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"version":         triageVersion(),
			"health":          triageHealth(),
			"doctor":          triageDoctor(ctx),
			triageKeyErrors:   triageErrors(),
			triageKeyWarnings: triageWarnings(),
			"crashes":         triageCrashes(),
			"plugins":         triagePlugins(),
			"reload":          triageReload(ctx),
			"recent-logs":     triageRecentLogs(),
			"runtime":         triageRuntime(ctx),
		},
	}, nil
}

// triageVersion returns structured build and appliance provenance from
// version.BuildIdentity.
func triageVersion() map[string]any {
	id := zeversion.BuildIdentity()
	out := map[string]any{
		"release":    id.Release,
		"build-date": id.BuildDate,
		"go":         id.Go,
		"os":         id.OS,
		"arch":       id.Arch,
	}
	if id.Commit != "" {
		out["commit"] = id.Commit
	}
	if id.Modified {
		out["modified"] = true
	}
	if id.VCSTime != "" {
		out["vcs-time"] = id.VCSTime
	}
	if id.CGO != "" {
		out["cgo"] = id.CGO
	}
	if id.Image != nil {
		out["image"] = id.Image
	}
	return out
}

// triageHealth returns the live component health registry snapshot.
func triageHealth() map[string]any {
	rep := health.Check()
	components := make([]map[string]any, 0, len(rep.Components))
	for i := range rep.Components {
		c := &rep.Components[i]
		entry := map[string]any{
			triageKeyName:   c.Name,
			triageKeyStatus: string(c.Status),
		}
		if c.Reason != "" {
			entry[triageKeyReason] = c.Reason
		}
		components = append(components, entry)
	}
	return map[string]any{
		triageKeyStatus: string(rep.Status),
		"checked-at":    rep.CheckedAt,
		keyCount:        len(components),
		"components":    components,
	}
}

// triageDoctor queries `show doctor` over the caller's dispatcher without
// failing the enclosing triage call if the dispatcher or check is unavailable.
func triageDoctor(ctx *pluginserver.CommandContext) map[string]any {
	doc, err := querySystemMap(ctx, "show doctor")
	if err != nil {
		return map[string]any{
			triageKeyAvailable: false,
			triageKeyError:     err.Error(),
		}
	}
	out := maps.Clone(doc)
	if out == nil {
		out = map[string]any{}
	}
	out[triageKeyAvailable] = true
	return out
}

// triageErrors returns the newest triageMaxErrors error events from the report
// bus alongside total count, truncation flag, and ring overwrite count.
func triageErrors() map[string]any {
	all := report.Errors(0)
	losses := report.Losses()
	total := len(all)
	capped := all
	if len(capped) > triageMaxErrors {
		capped = capped[:triageMaxErrors]
	}
	if capped == nil {
		capped = []report.Issue{}
	}
	return map[string]any{
		triageKeyErrors:     capped,
		keyCount:            len(capped),
		triageKeyTotalCount: total,
		triageKeyTruncated:  total > len(capped),
		"overwritten":       losses.ErrorsOverwritten,
	}
}

// triageWarnings returns the newest triageMaxWarnings active warnings from the
// report bus alongside total count, truncation flag, and eviction count.
func triageWarnings() map[string]any {
	all := report.Warnings()
	losses := report.Losses()
	total := len(all)
	capped := all
	if len(capped) > triageMaxWarnings {
		capped = capped[:triageMaxWarnings]
	}
	if capped == nil {
		capped = []report.Issue{}
	}
	return map[string]any{
		triageKeyWarnings:   capped,
		keyCount:            len(capped),
		triageKeyTotalCount: total,
		triageKeyTruncated:  total > len(capped),
		"evicted":           losses.WarningsEvicted,
	}
}

// triageCrashes returns up to triageMaxCrashes crash artifacts newest-first.
func triageCrashes() map[string]any {
	all := crashlog.CrashListFields()
	dir := crashlog.CrashDir()
	total := len(all)
	capped := all
	if len(capped) > triageMaxCrashes {
		capped = capped[:triageMaxCrashes]
	}
	if capped == nil {
		capped = []map[string]any{}
	}
	out := map[string]any{
		"crashes":           capped,
		keyCount:            len(capped),
		triageKeyTotalCount: total,
		triageKeyTruncated:  total > len(capped),
	}
	if dir != "" {
		out["dir"] = dir
	}
	return out
}

// triagePlugins summarizes published plugin lifecycle and delivery state,
// expanding only unhealthy or noteworthy plugins to keep payloads bounded.
func triagePlugins() map[string]any {
	snap, err := pluginserver.PluginDoctorSnapshot()
	if err != nil {
		return map[string]any{
			triageKeyAvailable: false,
			triageKeyReason:    err.Error(),
		}
	}

	configured := 0
	running := 0
	down := 0
	disabled := 0
	unhealthy := make([]map[string]any, 0)

	for i := range snap.Plugins {
		p := &snap.Plugins[i]
		if p.Configured {
			configured++
		}
		if p.Running {
			running++
		}
		if p.Down {
			down++
		}
		if p.Disabled {
			disabled++
		}
		if pluginIsUnhealthy(p) {
			unhealthy = append(unhealthy, triageUnhealthyPluginRow(p))
		}
	}

	out := map[string]any{
		triageKeyAvailable:         true,
		triageKeyTotal:             len(snap.Plugins),
		"configured":               configured,
		triageKeyRunning:           running,
		"down":                     down,
		"disabled":                 disabled,
		"unhealthy":                unhealthy,
		"delivery-disagreements":   len(snap.DeliveryDisagreements),
		"dormant-receptions":       len(snap.DormantReceptions),
		"unbacked-claims":          len(snap.UnbackedClaims),
		"handler-panics":           len(snap.HandlerPanics),
		"handler-panics-truncated": snap.HandlerPanicsTruncated,
	}
	if snap.LastCommittedTransaction != "" {
		out["last-committed-transaction"] = snap.LastCommittedTransaction
	}
	return out
}

// pluginIsUnhealthy reports whether p carries any lifecycle, startup, cleanup,
// enricher, registration, or delivery fault worth surfacing in triage.
func pluginIsUnhealthy(p *pluginserver.PluginDoctorState) bool {
	if !p.Running {
		return true
	}
	if p.Down {
		return true
	}
	if p.Disabled {
		return true
	}
	if p.LastError != "" {
		return true
	}
	if p.CleanupOwed {
		return true
	}
	if len(p.Enrichers) > 0 {
		return true
	}
	return doctorRecordHasProblems(&p.Records)
}

// doctorRecordHasProblems reports whether r records any refusal, dropped
// subscription, failed post-startup, projection error, delivery problem, or
// failed config commit.
func doctorRecordHasProblems(r *process.DoctorRecord) bool {
	if len(r.CommandRefusals) > 0 {
		return true
	}
	if r.CommandRefusalsDropped > 0 {
		return true
	}
	if len(r.SubscriptionDrops) > 0 {
		return true
	}
	if r.SubscriptionDropsDropped > 0 {
		return true
	}
	if r.PostStartup == process.PostStartupUndelivered {
		return true
	}
	if r.PostStartupError != "" {
		return true
	}
	if r.ProjectionError != "" {
		return true
	}
	if r.Delivery.Failures > 0 {
		return true
	}
	if r.Delivery.Slow > 0 {
		return true
	}
	if r.Delivery.BackpressureWaits > 0 {
		return true
	}
	if r.Delivery.LastError != "" {
		return true
	}
	return r.ConfigCommit.Error != ""
}

// triageUnhealthyPluginRow builds one compact triage entry for an unhealthy
// plugin without copying unbounded per-process lists.
func triageUnhealthyPluginRow(p *pluginserver.PluginDoctorState) map[string]any {
	row := map[string]any{
		triageKeyName:    p.Name,
		"configured":     p.Configured,
		triageKeyRunning: p.Running,
		"down":           p.Down,
		"disabled":       p.Disabled,
	}
	if p.HasProcess {
		row["stage"] = p.Stage.String()
	}
	if p.LastError != "" {
		row["last-error"] = p.LastError
	}
	if p.CleanupOwed {
		row["cleanup-owed"] = true
	}
	if len(p.Enrichers) > 0 {
		row["enricher-failures"] = len(p.Enrichers)
	}
	if p.Records.ProjectionError != "" {
		row["projection-error"] = p.Records.ProjectionError
	}
	if p.Records.Delivery.Failures > 0 {
		row["delivery-failures"] = p.Records.Delivery.Failures
	}
	if p.Records.Delivery.Slow > 0 {
		row["delivery-slow"] = p.Records.Delivery.Slow
	}
	if p.Records.Delivery.BackpressureWaits > 0 {
		row["delivery-backpressure-waits"] = p.Records.Delivery.BackpressureWaits
	}
	if p.Records.ConfigCommit.Error != "" {
		row["config-commit-error"] = p.Records.ConfigCommit.Error
	}
	if len(p.Records.CommandRefusals) > 0 {
		row["command-refusals"] = len(p.Records.CommandRefusals)
	}
	if len(p.Records.SubscriptionDrops) > 0 {
		row["subscription-drops"] = len(p.Records.SubscriptionDrops)
	}
	if p.Records.PostStartupError != "" {
		row["post-startup-error"] = p.Records.PostStartupError
	}
	return row
}

// triageReload returns the daemon's reload fence status when a server is wired,
// or an explicit unavailable marker offline.
func triageReload(ctx *pluginserver.CommandContext) map[string]any {
	if ctx == nil {
		return map[string]any{
			triageKeyAvailable: false,
			triageKeyReason:    msgDaemonNotRunning,
		}
	}
	if ctx.Server == nil {
		return map[string]any{
			triageKeyAvailable: false,
			triageKeyReason:    msgDaemonNotRunning,
		}
	}
	generation, outcome, at, opID := ctx.Server.ReloadStatusWithOp()
	lastReloadAt := ""
	if !at.IsZero() {
		lastReloadAt = at.UTC().Format(time.RFC3339)
	}
	out := map[string]any{
		triageKeyAvailable: true,
		"generation":       generation,
		"last-outcome":     outcome,
		"last-reload-at":   lastReloadAt,
	}
	if opID != "" {
		out["last-operation-id"] = opID
	}
	return out
}

// triageRecentLogs returns up to triageMaxLogs newest warn and error entries
// from the global log ring. Info and debug records are excluded.
func triageRecentLogs() map[string]any {
	ring := slogutil.GlobalLogRing()
	if ring == nil {
		return map[string]any{
			triageKeyAvailable:  false,
			triageKeyReason:     "log ring unavailable",
			triageKeyEntries:    []map[string]any{},
			keyCount:            0,
			triageKeyTotalCount: 0,
			triageKeyTruncated:  false,
		}
	}

	all := ring.Recent(0)
	entries := make([]map[string]any, 0, min(len(all), triageMaxLogs))
	total := 0
	for i := range all {
		rec := &all[i]
		if !isTriageLogLevel(rec.Level) {
			continue
		}
		total++
		if len(entries) >= triageMaxLogs {
			continue
		}
		row := map[string]any{
			"timestamp": rec.Timestamp.UTC().Format(time.RFC3339),
			"level":     rec.Level,
			"component": rec.Component,
			"message":   rec.Message,
		}
		attrs, err := rec.Attributes()
		if err != nil {
			row["attributes-error"] = err.Error()
		} else if len(attrs) > 0 {
			row["attributes"] = attrs
		}
		entries = append(entries, row)
	}
	return map[string]any{
		triageKeyEntries:    entries,
		keyCount:            len(entries),
		triageKeyTotalCount: total,
		triageKeyTruncated:  total > len(entries),
	}
}

// isTriageLogLevel reports whether level is warn or error.
func isTriageLogLevel(level string) bool {
	if strings.EqualFold(level, "warn") {
		return true
	}
	return strings.EqualFold(level, triageKeyError)
}

// triageRuntime returns compact Go runtime memory, CPU, goroutine, watchdog,
// and optional reactor uptime statistics.
func triageRuntime(ctx *pluginserver.CommandContext) map[string]any {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	st := watchdog.Status()
	hungCount := 0
	if ctx != nil {
		if ctx.Server != nil {
			hungCount = ctx.Server.HungHandlerCount()
		}
	}
	out := map[string]any{
		keyNumGoroutines:     runtime.NumGoroutine(),
		keyNumCPU:            runtime.NumCPU(),
		keyMaxProcs:          runtime.GOMAXPROCS(0),
		keyHeapAlloc:         m.HeapAlloc,
		keyHeapInUse:         m.HeapInuse,
		keySys:               m.Sys,
		keyNumGC:             m.NumGC,
		"watchdog-running":   st.Running,
		"stall-count":        st.StallCount,
		"hung-handler-count": hungCount,
		"bad-packet-count":   badpacket.Count(),
	}
	if ctx == nil {
		return out
	}
	r := ctx.Reactor()
	if r == nil {
		return out
	}
	stats := r.Stats()
	out["start-time"] = stats.StartTime.Format(time.RFC3339)
	out["uptime"] = stats.Uptime.Truncate(time.Second).String()
	return out
}
