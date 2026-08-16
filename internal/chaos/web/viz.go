// Design: docs/architecture/chaos-web-dashboard.md — web dashboard UI
// Detail: viz_panels.go — multi-panel viz layout and panel content handlers
// Detail: viz_convergence_trend.go — convergence trend line chart
// Related: viz_timeline.go — peer timeline visualization
// Related: viz_matrix.go — route matrix and family visualization

package web

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/chaos/peer"
)

// escapeAttr escapes a string for safe use in HTML attributes.
var escapeAttr = html.EscapeString

// escapeJSONInAttr escapes a string for safe interpolation as a JSON value
// inside an HTML attribute. Two layers: JSON-escape (\" and \\) so the value
// survives browser entity decoding + JSON parsing, then HTML-escape so the
// attribute boundary isn't broken.
func escapeJSONInAttr(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return html.EscapeString(s)
}

// handleVizEvents serves the event stream tab content.
// Query params: peer (index), type (event type name).
func (d *Dashboard) handleVizEvents(w http.ResponseWriter, r *http.Request) {
	d.state.RLock()
	defer d.state.RUnlock()

	peerFilter := r.URL.Query().Get("peer")
	typeFilter := r.URL.Query().Get("type")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writeEventStream(w, d.state, peerFilter, typeFilter)
}

// handleVizConvergence serves the convergence histogram tab content.
func (d *Dashboard) handleVizConvergence(w http.ResponseWriter, _ *http.Request) {
	d.state.RLock()
	defer d.state.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writeConvergenceHistogram(w, d.state.Convergence, d.state.ConvergenceDeadline, pagePanel)
}

// handleVizChaosEvents serves the chaos events table tab content.
func (d *Dashboard) handleVizChaosEvents(w http.ResponseWriter, _ *http.Request) {
	d.state.RLock()
	defer d.state.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writeChaosEvents(w, d.state)
}

// handleVizChaosTimeline serves the chaos event timeline tab content.
func (d *Dashboard) handleVizChaosTimeline(w http.ResponseWriter, _ *http.Request) {
	d.state.RLock()
	defer d.state.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writeChaosTimeline(w, d.state, d.state.WarmupDuration)
}

// writeEventStream renders the event stream feed with optional filtering.
func writeEventStream(w io.Writer, s *DashboardState, peerFilter, typeFilter string) {
	h := &htmlWriter{w: w}
	h.write(`<div class="viz-panel" hx-get="/viz/events" hx-trigger="every 500ms"` + freezePoll + ` hx-target="#viz-content" hx-swap="innerHTML"
     hx-include="[name='peer'],[name='type']">
<div class="viz-header">
  <h3>Event Stream</h3>
  <div class="filters">
    <label>Peer:</label>
    <select hx-get="/viz/events" hx-target="#viz-content" hx-swap="innerHTML"
            name="peer" hx-include="[name='type']">
      <option value="">All</option>`)

	for i := range s.PeerCount {
		h.writef(`<option value="%d"%s>Peer %d</option>`, i, selAttr(peerFilter == itoa(i)), i)
	}

	h.write(`
    </select>
    <label>Type:</label>
    <select hx-get="/viz/events" hx-target="#viz-content" hx-swap="innerHTML"
            name="type" hx-include="[name='peer']">
      <option value="">All</option>`)

	for _, name := range eventTypeNames() {
		h.writef(`<option value="%s"%s>%s</option>`, name, selAttr(typeFilter == name), name)
	}

	h.write(`
    </select>
    <label class="auto-scroll-toggle">
      <input type="checkbox" id="auto-scroll" checked onchange="window._autoScroll=this.checked"> Auto-scroll
    </label>
  </div>
</div>
<div class="event-feed" id="event-feed">`)

	events := s.GlobalEvents.All()
	// Show most recent first.
	for i := len(events) - 1; i >= 0; i-- {
		ev := events[i]

		// Apply peer filter.
		if peerFilter != "" {
			if pidx, err := strconv.Atoi(peerFilter); err == nil && ev.PeerIndex != pidx {
				continue
			}
		}

		// Apply type filter.
		if typeFilter != "" && eventTypeLabel(ev.Type) != typeFilter {
			continue
		}

		evClass := eventTypeClass(ev.Type)
		elapsed := formatElapsed(time.Since(ev.Time))
		label := eventTypeLabel(ev.Type)
		detail := eventDetail(ev)
		detailStyle := chaosDetailStyle(ev)
		h.writef(`<div class="event-row"><span class="event-time">%s ago</span><span class="event-peer %s">p%d</span><span class="event-type %s">%s</span><span class="event-detail"%s>%s</span></div>`,
			elapsed, evClass, ev.PeerIndex, evClass, label, detailStyle, detail)
	}

	h.write(`</div>
<p class="viz-desc">Live feed of BGP session and routing events across all peers. Filter by peer index or event type. Timestamps show how long ago each event occurred.</p>
</div>`)
}

// writeConvergenceHistogram renders the CSS bar chart for convergence latency.
// Pass streamPanel when the fragment is broadcast, so it names itself out of
// band, and pagePanel when it is rendered in place.
func writeConvergenceHistogram(w io.Writer, ch *ConvergenceHistogram, deadline time.Duration, oob string) {
	hw := &htmlWriter{w: w}
	hw.write(`<div class="viz-panel" id="viz-convergence"` + oob + ` hx-swap="outerHTML">
<h3>Convergence Histogram</h3>
<div class="histogram" style="position:relative">`)

	maxCount := ch.maxCount()
	bucketColors := []string{
		"#3fb950", "#3fb950", "#7cc647", // green (fast)
		"#b8cc3e", "#d29922", "#db8928", // yellow (moderate)
		"#db6d28", "#f85149", // orange-red (slow)
		"#f85149", "#da3633", "#b62324", // red (very slow)
		"#8b1a1a", "#6e1212", // dark red (extremely slow)
	}

	for i, b := range &ch.Buckets {
		pct := 0
		if maxCount > 0 {
			pct = b.Count * 100 / maxCount
		}
		if pct < 2 && b.Count > 0 {
			pct = 2 // Minimum visible height.
		}
		color := bucketColors[i]
		hw.writef(`<div class="histogram-bar-wrapper">
  <div class="histogram-bar" style="height:%d%%;background:%s" title="%s: %d routes"></div>
  <div class="histogram-label">%s</div>
  <div class="histogram-count">%d</div>
</div>`, pct, color, b.Label, b.Count, b.Label, b.Count)
	}

	// Deadline marker: vertical dashed line at the bucket position matching the deadline.
	if deadline > 0 {
		// Find which bucket the deadline falls in (as a percentage across the 9 buckets).
		deadlinePct := 0
		for i, b := range &ch.Buckets {
			if deadline >= b.Min && (b.Max == 0 || deadline < b.Max) {
				// Interpolate within the bucket.
				bucketWidth := 100 / len(ch.Buckets)
				deadlinePct = i*bucketWidth + bucketWidth/2
				break
			}
			if b.Max > 0 && deadline >= b.Max {
				continue
			}
		}
		if deadlinePct > 0 && deadlinePct <= 100 {
			hw.writef(`<div class="deadline-marker" style="left:%d%%" title="Deadline: %s"></div>`,
				deadlinePct, FormatDuration(deadline))
		}
	}

	hw.write(`</div>
<div class="histogram-stats">`)

	hw.writef(`<span class="stat"><span class="stat-label">Total </span><span class="stat-value">%d</span></span>`, ch.Total)
	hw.writef(`<span class="stat"><span class="stat-label">Avg </span><span class="stat-value">%s</span></span>`, FormatDuration(ch.Avg()))
	if ch.Total > 0 {
		hw.writef(`<span class="stat"><span class="stat-label">Min </span><span class="stat-value">%s</span></span>`, FormatDuration(ch.Min))
		hw.writef(`<span class="stat"><span class="stat-label">Max </span><span class="stat-value">%s</span></span>`, FormatDuration(ch.Max))
	}
	if ch.SlowCount > 0 {
		hw.writef(`<span class="stat"><span class="stat-label">Slow (&gt;1s) </span><span class="stat-value" style="color:var(--yellow)">%d</span></span>`, ch.SlowCount)
	}

	hw.write(`</div>
<p class="viz-desc">Distribution of route propagation latency — time from when a route is announced by one peer until it is received by another. Bars show how many routes converged within each time bucket. The dashed line marks the convergence deadline.</p>
</div>`)
}

// writeChaosEvents renders a scrollable table of recent chaos actions.
func writeChaosEvents(w io.Writer, s *DashboardState) {
	h := &htmlWriter{w: w}
	const maxRows = 200

	h.write(`<div class="viz-panel" hx-get="/viz/chaos-events" hx-trigger="every 500ms"` + freezePoll + ` hx-target="#viz-content" hx-swap="innerHTML">
<h3>Chaos Events</h3>`)

	if len(s.ChaosHistory) == 0 {
		h.write(`<div class="stat-label" style="padding:16px">No chaos actions recorded yet.</div>
<p class="viz-desc">Table of chaos actions injected during the run. Shows the most recent actions with timestamps, target peer, and action type. Chaos events appear once the warmup period elapses and the chaos scheduler is active.</p>
</div>`)
		return
	}

	h.write(`<div class="chaos-events-table">
<table class="peer-table">
  <thead><tr>
    <th>Time</th>
    <th>Peer</th>
    <th>Action</th>
  </tr></thead>
  <tbody>`)

	// Show most recent first, capped at maxRows.
	_, colorMap := chaosActionColors()
	start := 0
	if len(s.ChaosHistory) > maxRows {
		start = len(s.ChaosHistory) - maxRows
	}
	for i := len(s.ChaosHistory) - 1; i >= start; i-- {
		entry := s.ChaosHistory[i]
		elapsed := FormatDuration(entry.Time.Sub(s.StartTime))
		color := colorMap[entry.Action]
		if color == "" {
			color = "var(--text-secondary)"
		}
		h.writef(`<tr><td>%s</td><td>p%d</td><td style="color:%s">%s</td></tr>`,
			elapsed, entry.PeerIndex, color, escapeHTML(entry.Action))
	}

	h.writef(`</tbody></table></div>
<div class="histogram-stats">
  <span class="stat"><span class="stat-label">Total </span><span class="stat-value">%d</span></span>
  <span class="stat"><span class="stat-label">Showing </span><span class="stat-value">%d</span></span>
</div>
<p class="viz-desc">Table of chaos actions injected during the run. Shows the most recent %d actions with timestamps relative to run start, target peer, and action type.</p>
</div>`, len(s.ChaosHistory), min(len(s.ChaosHistory), maxRows), maxRows)
}

// chaosDetailStyle returns an inline style attribute for chaos/route-action events,
// coloring the detail text to match the chaos timeline markers. Returns empty string
// for non-chaos events.
func chaosDetailStyle(ev peer.Event) string {
	if ev.Type != peer.EventChaosExecuted && ev.Type != peer.EventRouteAction {
		return ""
	}
	_, colorMap := chaosActionColors()
	if color := colorMap[ev.ChaosAction]; color != "" {
		return fmt.Sprintf(` style="color:%s"`, color)
	}
	return ""
}

// chaosActionColors returns the ordered action→color mapping for chaos timeline
// markers and legend. Defined once, shared by writeChaosTimeline and tests.
func chaosActionColors() ([]struct{ name, color string }, map[string]string) {
	type ac struct{ name, color string }
	ordered := []ac{
		{"config-reload", "#79c0ff"},
		{"connection-collision", "#d2a8ff"},
		{"disconnect-during-burst", "#ff7b72"},
		{"hold-timer-expiry", "#d29922"},
		{"malformed-update", "#bc8cff"},
		{"notification-cease", "#ffa657"},
		{"reconnect-storm", "#db6d28"},
		{"tcp-disconnect", "#f85149"},
	}
	m := make(map[string]string, len(ordered))
	for _, a := range ordered {
		m[a.name] = a.color
	}
	// Convert to exported-friendly type (same layout).
	out := make([]struct{ name, color string }, len(ordered))
	for i, a := range ordered {
		out[i] = struct{ name, color string }{a.name, a.color}
	}
	return out, m
}

// rollingWindow is the duration of the "Last 60s" rolling track.
const rollingWindow = 60 * time.Second

// writeChaosTrack renders a single chaos timeline track from windowStart to now.
// It filters entries outside the window and positions markers as percentages.
// warmup is drawn only when > 0 and the window includes s.StartTime.
func writeChaosTrack(w io.Writer, s *DashboardState, windowStart, now time.Time, colorMap map[string]string, warmup time.Duration) int {
	h := &htmlWriter{w: w}
	windowDur := now.Sub(windowStart)
	if windowDur <= 0 {
		windowDur = time.Second
	}

	h.write(`<div class="chaos-timeline">
<div class="chaos-timeline-track" style="position:relative">`)

	// Warmup shading — only when the track starts at run start.
	if warmup > 0 && !windowStart.After(s.StartTime) {
		warmupPct := pctOfDuration(warmup, windowDur)
		if warmupPct > 0 {
			h.writef(`<div class="warmup-region" style="width:%d%%" title="Warmup: %s"></div>`,
				warmupPct, FormatDuration(warmup))
		}
	}

	var count int
	for _, entry := range s.ChaosHistory {
		if entry.Time.Before(windowStart) {
			continue
		}
		leftPct := pctOfDuration(entry.Time.Sub(windowStart), windowDur)
		color := colorMap[entry.Action]
		if color == "" {
			color = "#8b949e"
		}
		h.writef(`<div class="chaos-marker" style="left:%d%%;background:%s" title="p%d: %s at %s" hx-get="/peer/%d" hx-target="#peer-detail" hx-swap="outerHTML"></div>`,
			leftPct, color, entry.PeerIndex, escapeAttr(entry.Action), FormatDuration(entry.Time.Sub(s.StartTime)), entry.PeerIndex)
		count++
	}

	h.write(`</div></div>`)
	return count
}

// writeChaosTimeline renders two horizontal timelines with chaos event markers:
// an overall timeline spanning the full run, and a rolling last-60s window.
func writeChaosTimeline(w io.Writer, s *DashboardState, warmup time.Duration) {
	h := &htmlWriter{w: w}
	now := time.Now()
	elapsed := now.Sub(s.StartTime)
	if elapsed == 0 {
		elapsed = time.Second
	}

	h.write(`<div class="viz-panel" hx-get="/viz/chaos-timeline" hx-trigger="every 500ms"` + freezePoll + ` hx-target="#viz-content" hx-swap="innerHTML">
<h3>Chaos Timeline</h3>`)

	if len(s.ChaosHistory) == 0 {
		h.write(`<p class="viz-desc">No chaos actions recorded yet. Chaos events appear here once the warmup period elapses and the chaos scheduler is active (--chaos-rate &gt; 0).</p></div>`)
		return
	}

	actionColors, colorMap := chaosActionColors()

	// Overall track.
	h.write(`<div class="chaos-timeline-label">Overall</div>`)
	writeChaosTrack(w, s, s.StartTime, now, colorMap, warmup)

	// Last 60s rolling track.
	rollingStart := now.Add(-rollingWindow)
	if rollingStart.Before(s.StartTime) {
		rollingStart = s.StartTime
	}
	h.write(`<div class="chaos-timeline-label">Last 60s</div>`)
	recentCount := writeChaosTrack(w, s, rollingStart, now, colorMap, warmup)

	// Shared legend.
	h.write(`<div class="chaos-legend">`)
	for _, ac := range actionColors {
		h.writef(`<span class="legend-item"><span class="legend-swatch" style="background:%s"></span>%s</span>`, ac.color, ac.name)
	}

	h.writef(`</div>
<div class="histogram-stats">
  <span class="stat"><span class="stat-label">Total actions </span><span class="stat-value">%d</span></span>
  <span class="stat"><span class="stat-label">Last 60s </span><span class="stat-value">%d</span></span>
  <span class="stat"><span class="stat-label">Duration </span><span class="stat-value">%s</span></span>
</div>
<p class="viz-desc">Two timelines: Overall shows all chaos actions across the full run; Last 60s is a rolling window showing recent activity. Each vertical mark is one action; color indicates the action type. The shaded region on the overall track is the warmup period.</p>
</div>`, len(s.ChaosHistory), recentCount, FormatDuration(elapsed))
}

// eventTypeNames returns all known event type labels for filter dropdowns.
func eventTypeNames() []string {
	return []string{
		"established", "disconnected", "error", "chaos", "reconnecting",
		"route-sent", "route-recv", "route-withdrawn", "eor", "withdrawal-sent",
	}
}

// statusColor returns a CSS color for a peer status.
func statusColor(s PeerStatus) string {
	switch s {
	case PeerUp:
		return "#3fb950"
	case PeerSyncing:
		return "#58a6ff"
	case PeerDown:
		return "#f85149"
	case PeerReconnecting:
		return "#d29922"
	case PeerIdle:
		return "#6e7681"
	}
	return "#6e7681"
}

// pctOfDuration returns the percentage position of d within total.
func pctOfDuration(d, total time.Duration) int {
	if total <= 0 {
		return 0
	}
	pct := int(d * 100 / total)
	if pct < 0 {
		return 0
	}
	if pct > 100 {
		return 100
	}
	return pct
}

// sortIntSlice sorts an int slice in ascending order (simple insertion sort for small slices).
func sortIntSlice(s []int) {
	for i := 1; i < len(s); i++ {
		key := s[i]
		j := i - 1
		for j >= 0 && s[j] > key {
			s[j+1] = s[j]
			j--
		}
		s[j+1] = key
	}
}
