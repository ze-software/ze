// Design: docs/architecture/chaos-web-dashboard.md — convergence trend percentile chart
// Overview: viz.go — writeConvergenceHistogram renders the distribution view
// Related: state.go — ConvergenceTrend RingBuffer and ComputeConvergencePercentiles

package web

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// handleVizConvergenceTrend serves the convergence trend panel content.
func (d *Dashboard) handleVizConvergenceTrend(w http.ResponseWriter, _ *http.Request) {
	d.state.RLock()
	defer d.state.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writeConvergenceTrend(w, d.state.ConvergenceTrend)
}

// renderConvergenceTrend returns the convergence trend HTML fragment for SSE.
func (d *Dashboard) renderConvergenceTrend() string {
	var b strings.Builder
	writeConvergenceTrend(&b, d.state.ConvergenceTrend)
	return b.String()
}

// writeConvergenceTrend renders the rolling percentile chart as CSS-only horizontal bars.
func writeConvergenceTrend(w io.Writer, rb *RingBuffer[time.Duration]) {
	h := &htmlWriter{w: w}

	h.write(`<div class="viz-panel" id="viz-convergence-trend" sse-swap="convergence-trend" hx-swap="outerHTML">
<h3>Convergence Trend</h3>`)

	p := ComputeConvergencePercentiles(rb)
	if p.Count == 0 {
		h.write(`<div class="stat-label" style="padding:16px">Awaiting convergence data...</div>
</div>`)
		return
	}

	// Maximum value determines 100% bar width.
	maxVal := p.P99
	if maxVal == 0 {
		maxVal = 1 // avoid division by zero
	}

	h.write(`<div class="trend-bars">`)
	for _, bar := range []struct {
		label string
		value time.Duration
		class string
	}{
		{"p50", p.P50, "trend-p50"},
		{"p90", p.P90, "trend-p90"},
		{"p99", p.P99, "trend-p99"},
	} {
		pct := int(bar.value * 100 / maxVal)
		if pct < 1 && bar.value > 0 {
			pct = 1 // minimum visible width
		}
		h.writef(`<div class="trend-row"><span class="trend-label">%s</span><div class="trend-track"><div class="%s" style="width:%d%%"></div></div><span class="trend-value">%s</span></div>`,
			bar.label, bar.class, pct, formatTrendDuration(bar.value))
	}
	h.writef(`</div><div class="trend-count">%d samples</div>`, p.Count)
	h.write(`</div>`)
}

// formatTrendDuration formats a duration for display in the trend chart.
func formatTrendDuration(d time.Duration) string {
	switch {
	case d < time.Millisecond:
		return fmt.Sprintf("%dµs", d.Microseconds())
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	default:
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
}
