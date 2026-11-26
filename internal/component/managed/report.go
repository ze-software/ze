// Design: docs/architecture/fleet-config.md — client-side periodic fleet report worker and health producer
// Related: client.go — runConnection starts and joins the per-session reporter
// Related: register_doctor_codes.go — CodeDoctorManagedReportFailed raised on report failures

package managed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	// FleetHealthInterval is the cadence at which a managed client pushes
	// component health reports to its hub after the initial post-connect report.
	FleetHealthInterval = 5 * time.Minute
	// defaultReportCallTimeout bounds each report collection and RPC round-trip
	// when ClientConfig.reportTimeout is zero.
	defaultReportCallTimeout = 30 * time.Second
	// fleetHealthProducerName is the producer identifier registered in pkg/fleet.
	fleetHealthProducerName = "health"
	// unknownMethodPrefix matches the hub's refusal of an unregistered RPC verb
	// (internal/component/plugin/server/managed_serve.go).
	unknownMethodPrefix = "unknown method: "
	// reportFailedFix names the operator action when a fleet report fails.
	reportFailedFix = "ze show doctor; verify hub connectivity and hub version"
)

func collectFleetHealth(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rep, err := ProjectFleetHealth(health.Check())
	if err != nil {
		return nil, err
	}
	return fleet.EncodeHealthReport(rep)
}

// ProjectFleetHealth converts a core health.Report into the public fleet wire
// envelope while preserving the overall status, component rows, timestamps,
// and JSON schema of the daemon /health endpoint. Control characters in
// component reasons are normalized and reasons are bounded to fleet.ReportTextMax.
func ProjectFleetHealth(rep health.Report) (fleet.HealthReport, error) {
	overall, err := projectHealthStatus(rep.Status)
	if err != nil {
		return fleet.HealthReport{}, err
	}
	components := make([]fleet.HealthComponent, 0, len(rep.Components))
	for _, c := range rep.Components {
		status, statusErr := projectHealthStatus(c.Status)
		if statusErr != nil {
			return fleet.HealthReport{}, fmt.Errorf("health component %q: %w", c.Name, statusErr)
		}
		components = append(components, fleet.HealthComponent{
			Name:      c.Name,
			Status:    status,
			Reason:    sanitizeReportText(c.Reason, fleet.ReportTextMax),
			CheckedAt: c.CheckedAt,
		})
	}
	out := fleet.HealthReport{
		Status:     overall,
		Components: components,
		CheckedAt:  rep.CheckedAt,
	}
	if err := out.Validate(); err != nil {
		return fleet.HealthReport{}, err
	}
	return out, nil
}

func projectHealthStatus(s health.Status) (fleet.HealthStatus, error) {
	switch s {
	case health.StatusHealthy:
		return fleet.HealthStatusHealthy, nil
	case health.StatusDegraded:
		return fleet.HealthStatusDegraded, nil
	case health.StatusDown:
		return fleet.HealthStatusDown, nil
	default:
		// Open set: any registered CheckFunc can return an arbitrary health.Status string.
		return fleet.HealthStatusUnspecified, fmt.Errorf("invalid health status %q", string(s))
	}
}

type reportScheduleEntry struct {
	producer   fleet.ReportProducer
	nextDue    time.Time
	suppressed bool
}

type sessionReporter struct {
	mc      *rpc.MuxConn
	cfg     *ClientConfig
	timeout time.Duration
	entries []reportScheduleEntry
	done    chan struct{}
}

// startSessionReporter starts one worker goroutine for an authenticated hub
// session to collect and send every registered fleet report immediately and
// then on each producer's interval. Callers MUST invoke the returned stop
// function before closing mc or returning from runConnection; stop cancels the
// session reporter context and waits for the worker goroutine to exit.
func startSessionReporter(parentCtx context.Context, mc *rpc.MuxConn, cfg *ClientConfig) func() {
	producers := cfg.configuredReportProducers()
	if len(producers) == 0 {
		return func() {}
	}
	entries := make([]reportScheduleEntry, 0, len(producers))
	for _, p := range producers {
		if p.Interval <= 0 {
			continue
		}
		if p.Collect == nil {
			continue
		}
		if p.Method == "" {
			continue
		}
		if p.Name == "" {
			continue
		}
		entries = append(entries, reportScheduleEntry{producer: p})
	}
	if len(entries) == 0 {
		return func() {}
	}

	sessionCtx, cancel := context.WithCancel(parentCtx)
	w := &sessionReporter{
		mc:      mc,
		cfg:     cfg,
		timeout: cfg.reportCallTimeout(),
		entries: entries,
		done:    make(chan struct{}),
	}
	go w.run(sessionCtx)

	var stopOnce sync.Once
	return func() {
		stopOnce.Do(func() {
			cancel()
			<-w.done
		})
	}
}

func (w *sessionReporter) run(ctx context.Context) {
	defer close(w.done)

	// Initial pass: collect and send each producer independently after the
	// session's first config-fetch succeeded.
	for i := range w.entries {
		if w.sessionEnded(ctx) {
			return
		}
		w.sendOnce(ctx, &w.entries[i])
		if !w.entries[i].suppressed {
			w.entries[i].nextDue = time.Now().Add(w.entries[i].producer.Interval)
		}
	}

	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	// Intentional session-scoped loop: exits when ctx is canceled, mc closes,
	// or every registered producer has been suppressed for this connection.
	for {
		nextIdx, nextDue, ok := w.earliestDue()
		if !ok {
			return
		}
		wait := time.Until(nextDue)
		if wait > 0 {
			if timer == nil {
				timer = time.NewTimer(wait)
			} else {
				timer.Reset(wait)
			}
			select {
			case <-timer.C:
			case <-ctx.Done():
				return
			case <-w.mc.Done():
				return
			}
		} else if w.sessionEnded(ctx) {
			return
		}

		w.sendOnce(ctx, &w.entries[nextIdx])
		if !w.entries[nextIdx].suppressed {
			w.entries[nextIdx].nextDue = time.Now().Add(w.entries[nextIdx].producer.Interval)
		}
	}
}

func (w *sessionReporter) earliestDue() (idx int, due time.Time, ok bool) {
	for i := range w.entries {
		if w.entries[i].suppressed {
			continue
		}
		if !ok || w.entries[i].nextDue.Before(due) {
			idx = i
			due = w.entries[i].nextDue
			ok = true
		}
	}
	return idx, due, ok
}

func (w *sessionReporter) sendOnce(ctx context.Context, entry *reportScheduleEntry) {
	p := entry.producer
	callCtx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	payload, collectErr := p.Collect(callCtx)
	if collectErr != nil {
		if w.sessionEnded(ctx) {
			return
		}
		w.reportFailure(p.Name, "collect", collectErr)
		return
	}
	if len(payload) == 0 {
		w.reportFailure(p.Name, "collect", errors.New("empty report payload"))
		return
	}
	if len(payload) > fleet.ReportBodyMax {
		w.reportFailure(p.Name, "encode", fmt.Errorf("report body size %d exceeds max %d bytes", len(payload), fleet.ReportBodyMax))
		return
	}

	_, rpcErr := w.mc.CallRPC(callCtx, p.Method, json.RawMessage(payload))
	if rpcErr == nil {
		w.clearFailure(p.Name)
		return
	}
	if w.sessionEnded(ctx) {
		return
	}
	if isUnknownMethodError(rpcErr, p.Method) {
		entry.suppressed = true
		w.clearFailure(p.Name)
		logger().Info("hub does not support fleet report method; disabling it until reconnect",
			"name", w.cfg.Name, "report", p.Name, "method", p.Method)
		return
	}
	w.reportFailure(p.Name, "send", rpcErr)
}

func (w *sessionReporter) sessionEnded(ctx context.Context) bool {
	if ctx.Err() != nil {
		return true
	}
	select {
	case <-w.mc.Done():
		return true
	default:
		return false
	}
}

func (w *sessionReporter) reportFailure(kind, step string, err error) {
	device := placeholderOr(w.cfg.Name, "<unnamed>")
	var msg textbuf.Buffer
	msg.Str("managed client ").Str(device).
		Str(" failed to ").Str(step).
		Byte(' ').Str(kind).
		Str(" report: ").Str(err.Error()).
		Str("; configuration sync and heartbeats continue")
	text := report.FitMessage(msg.String())
	subject := fitReportSubject(device, kind)

	logger().Warn(text,
		"code", CodeDoctorManagedReportFailed,
		"name", device,
		"report", kind,
		"step", step,
		"error", err,
		"fix", reportFailedFix)
	report.RaiseWarningFix(doctorComponent, CodeDoctorManagedReportFailed, subject, text, reportFailedFix,
		map[string]any{
			pipeOpDevice: device,
			"report":     kind,
			"step":       step,
		})
}

func (w *sessionReporter) clearFailure(kind string) {
	device := placeholderOr(w.cfg.Name, "<unnamed>")
	report.ClearWarning(doctorComponent, CodeDoctorManagedReportFailed, fitReportSubject(device, kind))
}

func isUnknownMethodError(err error, method string) bool {
	rpcErr, ok := errors.AsType[*rpc.RPCCallError](err)
	if !ok {
		return false
	}
	return rpcErr.Message == unknownMethodPrefix+method
}

func fitReportSubject(device, kind string) string {
	var tb textbuf.Buffer
	tb.Str("fleet report ").Str(device).Byte(' ').Str(kind)
	return truncateReportUTF8(tb.String(), report.SubjectMax)
}

func sanitizeReportText(raw string, maxBytes int) string {
	valid := strings.ToValidUTF8(raw, "")
	var b strings.Builder
	b.Grow(min(len(valid), maxBytes))
	for _, r := range valid {
		switch r {
		case '\n', '\r', '\t':
			b.WriteByte(' ')
		default:
			if unicode.IsControl(r) {
				continue
			}
			b.WriteRune(r)
		}
	}
	return truncateReportUTF8(strings.TrimSpace(b.String()), maxBytes)
}

func truncateReportUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
