// Design: docs/architecture/chaos-web-dashboard.md — web dashboard UI

package web

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/peer"
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
	writeConvergenceHistogram(w, d.state.Convergence, d.state.ConvergenceDeadline)
}

// handleVizPeerTimeline serves the peer state timeline tab content.
// Query params: page (1-based), window (time window: "all", "30s", "1m", "5m", "10m").
func (d *Dashboard) handleVizPeerTimeline(w http.ResponseWriter, r *http.Request) {
	d.state.RLock()
	defer d.state.RUnlock()

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}

	window := r.URL.Query().Get("window")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writePeerTimeline(w, d.state, page, window)
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
	h.write(`<div class="viz-panel" hx-get="/viz/events" hx-trigger="every 500ms [!window._frozen]" hx-target="#viz-content" hx-swap="innerHTML"
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
		elapsed := FormatElapsed(time.Since(ev.Time))
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
// The outer div carries sse-swap so SSE broadcasts can update it live.
func writeConvergenceHistogram(w io.Writer, ch *ConvergenceHistogram, deadline time.Duration) {
	hw := &htmlWriter{w: w}
	hw.write(`<div class="viz-panel" id="viz-convergence" sse-swap="convergence" hx-swap="outerHTML">
<h3>Convergence Histogram</h3>
<div class="histogram" style="position:relative">`)

	maxCount := ch.MaxCount()
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

// parseWindowDuration returns the duration for a window string, or 0 for "all".
func parseWindowDuration(window string) time.Duration {
	switch window {
	case "30s":
		return 30 * time.Second
	case "1m":
		return time.Minute
	case "5m":
		return 5 * time.Minute
	case "10m":
		return 10 * time.Minute
	default:
		return 0 // "all" or empty
	}
}

// timelineWindow holds the computed visible time range for the peer timeline.
type timelineWindow struct {
	windowStart time.Time     // Left edge of the visible range.
	windowEnd   time.Time     // Right edge (now).
	visible     time.Duration // windowEnd - windowStart.
}

// timelineWindowFromState computes the visible window for the given window string.
func timelineWindowFromState(s *DashboardState, window string) timelineWindow {
	now := time.Now()
	winDur := parseWindowDuration(window)
	tw := timelineWindow{windowEnd: now}
	if winDur > 0 {
		tw.windowStart = now.Add(-winDur)
		// Don't go before StartTime.
		if tw.windowStart.Before(s.StartTime) {
			tw.windowStart = s.StartTime
		}
	} else {
		tw.windowStart = s.StartTime
	}
	tw.visible = tw.windowEnd.Sub(tw.windowStart)
	if tw.visible <= 0 {
		tw.visible = time.Second
	}
	return tw
}

// pctInWindow returns the percentage position of t within the visible window.
func (tw timelineWindow) pctInWindow(t time.Time) int {
	return pctOfDuration(t.Sub(tw.windowStart), tw.visible)
}

// writePeerTimelineTrack renders one set of peer timeline rows for a given window.
func writePeerTimelineTrack(w io.Writer, s *DashboardState, pagePeers []int, tw timelineWindow) {
	h := &htmlWriter{w: w}
	h.writef(`<div class="timeline-container" style="--timeline-duration:%d">`, int(tw.visible.Seconds()))

	for _, idx := range pagePeers {
		writeTimelineRow(w, s, idx, tw)
	}

	writeTimelineScale(w, tw, s.StartTime)
	h.write(`</div>`)
}

// writePeerTimeline renders two peer state timelines: overall and last 60s.
// Paginated at 30 peers per page.
func writePeerTimeline(w io.Writer, s *DashboardState, page int, _ string) {
	h := &htmlWriter{w: w}
	const peersPerPage = 30

	twAll := timelineWindowFromState(s, "all")
	twRecent := timelineWindowFromState(s, "1m")

	// Build list of peers with transitions (sorted by peer index).
	var peerIndices []int
	for idx := range s.PeerTransitions {
		if len(s.PeerTransitions[idx]) > 0 {
			peerIndices = append(peerIndices, idx)
		}
	}
	// Also include peers with non-idle status even without recorded transitions.
	for idx, ps := range s.Peers {
		if ps.Status != PeerIdle && !slices.Contains(peerIndices, idx) {
			peerIndices = append(peerIndices, idx)
		}
	}

	// Sort peer indices.
	sortIntSlice(peerIndices)

	totalPeers := len(peerIndices)
	totalPages := max((totalPeers+peersPerPage-1)/peersPerPage, 1)
	if page > totalPages {
		page = totalPages
	}

	startIdx := (page - 1) * peersPerPage
	endIdx := min(startIdx+peersPerPage, totalPeers)
	pagePeers := peerIndices[startIdx:endIdx]

	h.writef(`<div class="viz-panel" hx-get="/viz/peer-timeline?page=%d" hx-trigger="every 500ms [!window._frozen]" hx-target="#viz-content" hx-swap="innerHTML">
<div class="viz-header">
  <h3>Peer State Timeline</h3>
  <div class="filters">`, page)

	if totalPages > 1 {
		h.writef(`<span class="stat-label">Page %d/%d</span>`, page, totalPages)
		if page > 1 {
			h.writef(` <span class="badge" hx-get="/viz/peer-timeline?page=%d" hx-target="#viz-content" hx-swap="innerHTML">Prev</span>`, page-1)
		}
		if page < totalPages {
			h.writef(` <span class="badge" hx-get="/viz/peer-timeline?page=%d" hx-target="#viz-content" hx-swap="innerHTML">Next</span>`, page+1)
		}
	}

	h.write(`
  </div>
</div>`)

	// Overall track.
	h.write(`<div class="chaos-timeline-label">Overall</div>`)
	writePeerTimelineTrack(w, s, pagePeers, twAll)

	// Last 60s rolling track.
	h.write(`<div class="chaos-timeline-label">Last 60s</div>`)
	writePeerTimelineTrack(w, s, pagePeers, twRecent)

	h.write(`
<p class="viz-desc">Each row shows one peer's BGP session state over time. Green = established, red = down, yellow = reconnecting, grey = idle. Overall shows the full run; Last 60s is a rolling window of recent activity.</p>
</div>`)
}

// writeTimelineRow renders a single peer's timeline bar within the visible window.
func writeTimelineRow(w io.Writer, s *DashboardState, idx int, tw timelineWindow) {
	ps := s.Peers[idx]
	if ps == nil {
		return
	}

	h := &htmlWriter{w: w}
	h.writef(`<div class="timeline-row"><span class="timeline-label">p%d</span><div class="timeline-bar">`, idx)

	transitions := s.PeerTransitions[idx]
	if len(transitions) == 0 {
		// No transitions — show current status for the full bar.
		color := statusColor(ps.Status)
		h.writef(`<div class="timeline-segment" style="left:0%%;width:100%%;background:%s" title="%s"></div>`, color, ps.Status.String())
	} else {
		// Find the effective state at windowStart: the last transition before the window.
		// This ensures we show the entering state for the left edge when windowed.
		firstVisible := 0
		for i, tr := range transitions {
			if tr.Time.After(tw.windowStart) {
				break
			}
			firstVisible = i
		}

		// Render segments from firstVisible onward, clipped to the window.
		for i := firstVisible; i < len(transitions); i++ {
			tr := transitions[i]

			// Segment start: clip to window left edge.
			segStart := tr.Time
			if segStart.Before(tw.windowStart) {
				segStart = tw.windowStart
			}

			// Segment end: next transition or window right edge.
			segEnd := tw.windowEnd
			if i+1 < len(transitions) {
				segEnd = transitions[i+1].Time
			}

			// Skip segments entirely outside the window.
			if segEnd.Before(tw.windowStart) || segStart.After(tw.windowEnd) {
				continue
			}
			if segEnd.After(tw.windowEnd) {
				segEnd = tw.windowEnd
			}

			startPct := tw.pctInWindow(segStart)
			endPct := tw.pctInWindow(segEnd)
			width := max(endPct-startPct, 1)
			color := statusColor(tr.Status)
			elapsed := FormatDuration(tr.Time.Sub(s.StartTime))
			h.writef(`<div class="timeline-segment" style="left:%d%%;width:%d%%;background:%s" title="%s at %s"></div>`,
				startPct, width, color, tr.Status.String(), elapsed)
		}
	}

	h.write(`</div></div>`)
}

// writeTimelineScale renders tick marks below the timeline bars showing elapsed time.
func writeTimelineScale(w io.Writer, tw timelineWindow, startTime time.Time) {
	h := &htmlWriter{w: w}
	h.write(`<div class="timeline-row timeline-scale"><span class="timeline-label"></span><div class="timeline-bar timeline-axis">`)

	// Choose tick interval based on visible duration.
	tickInterval := chooseTickInterval(tw.visible)
	if tickInterval <= 0 {
		h.write(`</div></div>`)
		return
	}

	// Compute the first tick time: round up from windowStart to the next tick boundary
	// relative to startTime (so ticks align to clean elapsed-time values).
	elapsedAtStart := tw.windowStart.Sub(startTime)
	firstTickElapsed := ((elapsedAtStart + tickInterval - 1) / tickInterval) * tickInterval
	// Handle the special case where windowStart == startTime (elapsed=0).
	if firstTickElapsed == 0 {
		firstTickElapsed = tickInterval
	}
	firstTickTime := startTime.Add(firstTickElapsed)

	for t := firstTickTime; !t.After(tw.windowEnd); t = t.Add(tickInterval) {
		pct := tw.pctInWindow(t)
		if pct < 0 || pct > 100 {
			continue
		}
		label := FormatDuration(t.Sub(startTime))
		h.writef(`<div class="timeline-tick" style="left:%d%%"><span class="timeline-tick-label">%s</span></div>`, pct, label)
	}

	h.write(`</div></div>`)
}

// chooseTickInterval picks a sensible tick spacing for the given visible duration.
func chooseTickInterval(visible time.Duration) time.Duration {
	switch {
	case visible <= 30*time.Second:
		return 5 * time.Second
	case visible <= time.Minute:
		return 10 * time.Second
	case visible <= 5*time.Minute:
		return 30 * time.Second
	case visible <= 10*time.Minute:
		return time.Minute
	case visible <= 30*time.Minute:
		return 5 * time.Minute
	default:
		return 10 * time.Minute
	}
}

// writeChaosEvents renders a scrollable table of recent chaos actions.
func writeChaosEvents(w io.Writer, s *DashboardState) {
	h := &htmlWriter{w: w}
	const maxRows = 200

	h.write(`<div class="viz-panel" hx-get="/viz/chaos-events" hx-trigger="every 500ms [!window._frozen]" hx-target="#viz-content" hx-swap="innerHTML">
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

	h.write(`<div class="viz-panel" hx-get="/viz/chaos-timeline" hx-trigger="every 500ms [!window._frozen]" hx-target="#viz-content" hx-swap="innerHTML">
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

// handleVizRouteMatrixCell serves the detail popup for a single matrix cell.
// Query params: src (source peer index), dst (destination peer index).
func (d *Dashboard) handleVizRouteMatrixCell(w http.ResponseWriter, r *http.Request) {
	src, err1 := strconv.Atoi(r.URL.Query().Get("src"))
	dst, err2 := strconv.Atoi(r.URL.Query().Get("dst"))
	if err1 != nil || err2 != nil {
		http.Error(w, "invalid src/dst", http.StatusBadRequest)
		return
	}

	d.state.RLock()
	defer d.state.RUnlock()

	count := d.state.RouteMatrix.Get(src, dst)
	avg := d.state.RouteMatrix.AvgLatency(src, dst)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h := &htmlWriter{w: w}
	h.writef(`<div class="cell-detail" id="cell-detail">
<h4>p%d → p%d</h4>
<div class="detail-grid">
  <div class="detail-item"><span class="label">Routes: </span><span class="value">%d</span></div>
  <div class="detail-item"><span class="label">Avg latency: </span><span class="value">%s</span></div>
</div>
<span class="close-btn" onclick="this.parentElement.remove()">&times;</span>
</div>`, src, dst, count, FormatDuration(avg))
}

// handleVizRouteMatrix serves the route flow heatmap tab content.
// Query params: top (max peer count), mode (count|latency), family, peers (comma-sep).
func (d *Dashboard) handleVizRouteMatrix(w http.ResponseWriter, r *http.Request) {
	d.state.RLock()
	defer d.state.RUnlock()

	topN := 20
	if t := r.URL.Query().Get("top"); t != "" {
		if v, err := strconv.Atoi(t); err == nil && v > 0 && v <= 100 {
			topN = v
		}
	}

	mode := r.URL.Query().Get("mode") // "latency" or "" (count)
	family := r.URL.Query().Get("family")

	// Custom peer selection: comma-separated indices override top-N.
	var customPeers []int
	if p := r.URL.Query().Get("peers"); p != "" {
		for s := range strings.SplitSeq(p, ",") {
			s = strings.TrimSpace(s)
			if v, err := strconv.Atoi(s); err == nil && v >= 0 {
				customPeers = append(customPeers, v)
			}
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writeRouteMatrix(w, d.state.RouteMatrix, routeMatrixOpts{
		topN:        topN,
		mode:        mode,
		family:      family,
		customPeers: customPeers,
	})
}

// routeMatrixOpts holds parameters for rendering the route flow heatmap.
type routeMatrixOpts struct {
	topN        int
	mode        string // "" (count) or "latency"
	family      string // address family filter (e.g., "ipv4/unicast")
	customPeers []int  // specific peer indices (overrides topN)
}

// writeRouteMatrix renders the N×N heatmap grid for route flow.
func writeRouteMatrix(w io.Writer, m *RouteMatrix, opts routeMatrixOpts) {
	h := &htmlWriter{w: w}
	peers := opts.customPeers
	if len(peers) == 0 {
		peers = m.TopNPeers(opts.topN)
	}

	latencyMode := opts.mode == "latency"

	h.write(`<div class="viz-panel" hx-get="/viz/route-matrix" hx-trigger="every 500ms [!window._frozen]" hx-target="#viz-content" hx-swap="innerHTML"
     hx-include="[name='top'],[name='mode'],[name='family'],[name='peers']">
<div class="viz-header">
  <h3>Route Flow Matrix</h3>
  <div class="filters">
    <label>Top:</label>
    <select hx-get="/viz/route-matrix" hx-target="#viz-content" hx-swap="innerHTML"
            name="top" hx-include="[name='mode'],[name='family'],[name='peers']">`)

	for _, n := range []int{10, 20, 30, 50} {
		h.writef(`<option value="%d"%s>%d</option>`, n, selAttr(n == opts.topN), n)
	}

	h.write(`
    </select>
    <label>Mode:</label>
    <select hx-get="/viz/route-matrix" hx-target="#viz-content" hx-swap="innerHTML"
            name="mode" hx-include="[name='top'],[name='family'],[name='peers']">`)
	h.writef(`<option value=""%s>Count</option>`, selAttr(!latencyMode))
	h.writef(`<option value="latency"%s>Latency</option>`, selAttr(latencyMode))

	h.write(`
    </select>
    <label>Family:</label>
    <select hx-get="/viz/route-matrix" hx-target="#viz-content" hx-swap="innerHTML"
            name="family" hx-include="[name='top'],[name='mode'],[name='peers']">
      <option value="">All</option>`)
	for _, fam := range m.Families() {
		h.writef(`<option value="%s"%s>%s</option>`, escapeAttr(fam), selAttr(fam == opts.family), fam)
	}

	h.write(`
    </select>
    <label>Peers:</label>
    <input type="text" name="peers" placeholder="e.g. 0,1,3" class="control-input"
           hx-get="/viz/route-matrix" hx-target="#viz-content" hx-swap="innerHTML"
           hx-trigger="change" hx-include="[name='top'],[name='mode'],[name='family']"
           value="`)
	if len(opts.customPeers) > 0 {
		for i, p := range opts.customPeers {
			if i > 0 {
				h.write(",")
			}
			h.writef("%d", p)
		}
	}
	h.write(`">
  </div>
</div>`)

	if len(peers) == 0 {
		h.write(`<div class="stat-label" style="padding:16px">No route flow data yet.</div>
<p class="viz-desc">Traffic volume between peers (cumulative). Counts increase after reconnections as routes are re-announced. In latency mode, warmer colors mean slower propagation.</p>
</div>`)
		return
	}

	// Compute scaling value based on mode.
	var maxVal int
	var maxLatency time.Duration
	if latencyMode {
		maxLatency = m.MaxAvgLatency()
	} else {
		if opts.family != "" {
			// Compute max cell for filtered view.
			for _, src := range peers {
				for _, dst := range peers {
					if v := m.GetByFamily(src, dst, opts.family); v > maxVal {
						maxVal = v
					}
				}
			}
		} else {
			maxVal = m.MaxCell()
		}
	}

	// Build the heatmap grid.
	cols := len(peers) + 1 // +1 for row header column
	h.writef(`<div class="heatmap-grid" style="grid-template-columns:40px repeat(%d, 1fr)">`, cols-1)

	// Header row: empty corner + column headers (destinations).
	h.write(`<div class="heatmap-corner"></div>`)
	for _, dst := range peers {
		h.writef(`<div class="heatmap-col-header">p%d</div>`, dst)
	}

	// Data rows: row header (source) + cells.
	for _, src := range peers {
		h.writef(`<div class="heatmap-row-header">p%d</div>`, src)
		for _, dst := range peers {
			if latencyMode {
				writeLatencyCell(w, m, src, dst, maxLatency)
			} else {
				writeCountCell(w, m, src, dst, maxVal, opts.family)
			}
		}
	}

	h.write(`</div>`)

	// Cell detail target.
	h.write(`<div id="cell-detail"></div>`)

	// Stats footer.
	h.writef(`<div class="histogram-stats">
  <span class="stat"><span class="stat-label">Cells </span><span class="stat-value">%d</span></span>`,
		m.Len())
	if latencyMode {
		h.writef(`<span class="stat"><span class="stat-label">Max Avg Latency </span><span class="stat-value">%s</span></span>`,
			FormatDuration(maxLatency))
	} else {
		h.writef(`<span class="stat"><span class="stat-label">Max </span><span class="stat-value">%d</span></span>`, maxVal)
	}
	stats := m.Stats()
	h.writef(`
  <span class="stat"><span class="stat-label">Peers </span><span class="stat-value">%d</span></span>
  <span class="stat" style="margin-left:16px"><span class="stat-label">Sent </span><span class="stat-value">%d</span></span>
  <span class="stat"><span class="stat-label">Recv </span><span class="stat-value">%d</span></span>
  <span class="stat"><span class="stat-label">Direct </span><span class="stat-value">%d</span></span>
  <span class="stat"><span class="stat-label">Credit </span><span class="stat-value">%d</span></span>`,
		len(peers), stats.SentCalls, stats.RecvCalls, stats.DirectMatch,
		stats.CreditMatch)
	if stats.Unmatched > 0 {
		h.writef(`
  <span class="stat"><span class="stat-label">Unmatched </span><span class="stat-value">%d</span></span>`,
			stats.Unmatched)
	}
	h.write(`
</div>
<p class="viz-desc">Traffic volume between peers (cumulative). Counts increase after reconnections as routes are re-announced. In latency mode, warmer colors mean slower propagation.</p>
</div>`)
}

// writeCountCell renders a single heatmap cell in count mode.
func writeCountCell(w io.Writer, m *RouteMatrix, src, dst, maxVal int, family string) {
	h := &htmlWriter{w: w}
	count := m.GetByFamily(src, dst, family)
	intensity := 0
	if maxVal > 0 && count > 0 {
		intensity = max(count*100/maxVal, 5)
	}
	var style string
	if count > 0 {
		style = fmt.Sprintf(` style="background:rgba(88,166,255,%.2f);color:#fff"`, float64(intensity)/100.0)
	}
	title := fmt.Sprintf("p%d→p%d: %d routes", src, dst, count)
	h.writef(`<div class="heatmap-cell"%s title="%s" hx-get="/viz/route-matrix/cell?src=%d&dst=%d" hx-target="#cell-detail" hx-swap="outerHTML">`, style, title, src, dst)
	if count > 0 {
		h.writef(`%d`, count)
	}
	h.write(`</div>`)
}

// writeLatencyCell renders a single heatmap cell in latency mode.
func writeLatencyCell(w io.Writer, m *RouteMatrix, src, dst int, maxLatency time.Duration) {
	h := &htmlWriter{w: w}
	avg := m.AvgLatency(src, dst)
	intensity := 0
	if maxLatency > 0 && avg > 0 {
		intensity = max(int(avg*100/maxLatency), 5)
	}
	// Use warm colors (orange→red) for latency instead of blue.
	var style string
	if avg > 0 {
		style = fmt.Sprintf(` style="background:rgba(219,109,40,%.2f);color:#fff"`, float64(intensity)/100.0)
	}
	title := fmt.Sprintf("p%d→p%d: avg %s", src, dst, FormatDuration(avg))
	h.writef(`<div class="heatmap-cell"%s title="%s" hx-get="/viz/route-matrix/cell?src=%d&dst=%d" hx-target="#cell-detail" hx-swap="outerHTML">`, style, title, src, dst)
	if avg > 0 {
		h.write(FormatDuration(avg))
	}
	h.write(`</div>`)
}

// selAttr returns ` selected` if cond is true, empty string otherwise.
func selAttr(cond bool) string {
	if cond {
		return ` selected`
	}
	return ""
}

// handleVizFamilies serves the per-family route matrix tab content.
func (d *Dashboard) handleVizFamilies(w http.ResponseWriter, _ *http.Request) {
	d.state.RLock()
	defer d.state.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writeFamilyMatrix(w, d.state)
}

// handleVizAllPeers serves the complete peer list tab content.
// Query params: sort (column), dir (asc/desc).
func (d *Dashboard) handleVizAllPeers(w http.ResponseWriter, r *http.Request) {
	d.state.RLock()
	defer d.state.RUnlock()

	sortCol := r.URL.Query().Get("sort")
	sortDir := r.URL.Query().Get("dir")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writeAllPeers(w, d.state, sortCol, sortDir)
}

// writeFamilyMatrix renders a peer × family table showing sent/recv per cell.
// Auto-refreshes so users see propagation counters ticking live.
func writeFamilyMatrix(w io.Writer, s *DashboardState) {
	h := &htmlWriter{w: w}
	families := s.SortedFamilies()

	h.write(`<div class="viz-panel" hx-get="/viz/families" hx-trigger="every 500ms [!window._frozen]" hx-target="#viz-content" hx-swap="innerHTML">
<div class="viz-header">
  <h3>Per-Family Routes</h3>
</div>`)

	if len(families) == 0 {
		h.write(`<div class="stat-label" style="padding:16px">No address families negotiated yet.</div>
<p class="viz-desc">Once peers establish sessions and negotiate address families, this table shows per-family sent/received route counts for every peer.</p>
</div>`)
		return
	}

	// Build sorted list of all peer indices.
	peerIndices := make([]int, 0, len(s.Peers))
	for idx := range s.Peers {
		peerIndices = append(peerIndices, idx)
	}
	sortIntSlice(peerIndices)

	// First pass: compute per-family total sent across all peers.
	// Used to derive "expected" for each peer (total - this peer's sent).
	famTotalSent := make(map[string]int, len(families))
	var grandSent int
	for _, idx := range peerIndices {
		ps := s.Peers[idx]
		if ps == nil {
			continue
		}
		for _, fam := range families {
			famTotalSent[fam] += ps.FamilySent[fam]
		}
		grandSent += ps.RoutesSent
	}

	// Compute per-family total sent-target across all peers.
	// Used for footer totals (sum of all profile targets).
	famTotalTarget := make(map[string]int, len(families))
	var grandTarget int
	for _, idx := range peerIndices {
		ps := s.Peers[idx]
		if ps == nil {
			continue
		}
		for _, fam := range families {
			famTotalTarget[fam] += ps.FamilySentTarget[fam]
		}
		for _, t := range ps.FamilySentTarget {
			grandTarget += t
		}
	}

	// Pre-compute column-wide digit widths for consistent alignment across all rows.
	colWidthL := make(map[string]int, len(families))
	colWidthR := make(map[string]int, len(families))
	var totalWidthL, totalWidthR int
	for _, idx := range peerIndices {
		ps := s.Peers[idx]
		if ps == nil {
			continue
		}
		neg := make(map[string]bool, len(ps.Families))
		for _, f := range ps.Families {
			neg[f] = true
		}
		var pst int
		for _, fam := range families {
			if !neg[fam] {
				continue
			}
			target := ps.FamilySentTarget[fam]
			recv := ps.FamilyRecv[fam]
			expected := famTotalTarget[fam] - target
			pst += target
			if wl := digitCount(max(ps.FamilySent[fam], recv)); wl > colWidthL[fam] {
				colWidthL[fam] = wl
			}
			if wr := digitCount(max(target, expected)); wr > colWidthR[fam] {
				colWidthR[fam] = wr
			}
		}
		totalExpectedTarget := grandTarget - pst
		if wl := digitCount(max(ps.RoutesSent, ps.RoutesRecv)); wl > totalWidthL {
			totalWidthL = wl
		}
		if wr := digitCount(max(pst, totalExpectedTarget)); wr > totalWidthR {
			totalWidthR = wr
		}
	}

	h.write(`<div class="family-matrix-scroll"><table class="family-matrix">
<thead><tr>
  <th class="fm-peer-col">Peer</th>
  <th class="fm-status-col">Status</th>
  <th class="fm-dir-col"></th>`)

	for _, fam := range families {
		h.writef(`<th class="fm-family-col">%s</th>`, escapeHTML(fam))
	}
	h.write(`<th class="fm-total-col">Total</th>
</tr></thead>
<tbody>`)

	// Second pass: render two rows per peer (SEND then RECV).
	var grandRecv int

	for _, idx := range peerIndices {
		ps := s.Peers[idx]
		if ps == nil {
			continue
		}

		// Build negotiated set for this peer.
		negotiated := make(map[string]bool, len(ps.Families))
		for _, f := range ps.Families {
			negotiated[f] = true
		}

		var peerSentTarget int
		for _, fam := range families {
			if negotiated[fam] {
				peerSentTarget += ps.FamilySentTarget[fam]
			}
		}

		// SEND row.
		h.writef(`<tr class="fm-send-row"><td class="fm-peer-col" rowspan="2">%d</td>`, idx)
		h.writef(`<td rowspan="2"><span class="dot %s"></span>%s</td>`, ps.Status.CSSClass(), ps.Status.String())
		h.write(`<td class="fm-dir">SEND</td>`)

		for _, fam := range families {
			if !negotiated[fam] {
				h.write(`<td></td>`)
				continue
			}
			sent := ps.FamilySent[fam]
			target := ps.FamilySentTarget[fam]
			wl := colWidthL[fam]
			wr := colWidthR[fam]
			h.writef(`<td class="fm-cell"><span class="fm-val %s" style="min-width:%dch">%d</span>`,
				familyCellClass(sent, target), wl, sent)
			if target > 0 {
				h.writef(`<span class="fm-dim"> / </span><span class="fm-val fm-dim" style="min-width:%dch">%d</span>`, wr, target)
			}
			h.write(`</td>`)
		}

		recvExpectedTotal := grandTarget - peerSentTarget
		h.writef(`<td class="fm-total fm-cell"><span class="fm-val %s" style="min-width:%dch">%d</span>`,
			familyCellClass(ps.RoutesSent, peerSentTarget), totalWidthL, ps.RoutesSent)
		if peerSentTarget > 0 {
			h.writef(`<span class="fm-dim"> / </span><span class="fm-val fm-dim" style="min-width:%dch">%d</span>`, totalWidthR, peerSentTarget)
		}
		h.write(`</td></tr>`)

		// RECV row.
		h.write(`<tr class="fm-recv-row"><td class="fm-dir">RECV</td>`)

		for _, fam := range families {
			if !negotiated[fam] {
				h.write(`<td></td>`)
				continue
			}
			recv := ps.FamilyRecv[fam]
			expected := famTotalTarget[fam] - ps.FamilySentTarget[fam]
			wl := colWidthL[fam]
			wr := colWidthR[fam]
			h.writef(`<td class="fm-cell"><span class="fm-val %s" style="min-width:%dch">%d</span>`,
				familyCellClass(recv, expected), wl, recv)
			if expected > 0 {
				h.writef(`<span class="fm-dim"> / </span><span class="fm-val fm-dim" style="min-width:%dch">%d</span>`, wr, expected)
			}
			h.write(`</td>`)
		}

		h.writef(`<td class="fm-total fm-cell"><span class="fm-val %s" style="min-width:%dch">%d</span>`,
			familyCellClass(ps.RoutesRecv, recvExpectedTotal), totalWidthL, ps.RoutesRecv)
		if recvExpectedTotal > 0 {
			h.writef(`<span class="fm-dim"> / </span><span class="fm-val fm-dim" style="min-width:%dch">%d</span>`, totalWidthR, recvExpectedTotal)
		}
		h.write(`</td></tr>`)
		grandRecv += ps.RoutesRecv
	}

	// Footer row with per-family totals (same span structure as data rows for / alignment).
	h.write(`</tbody><tfoot><tr class="fm-footer">
  <td colspan="3">Total</td>`)
	for _, fam := range families {
		wl := colWidthL[fam]
		wr := colWidthR[fam]
		h.writef(`<td class="fm-cell"><span class="fm-val" style="min-width:%dch">%d</span>`+
			`<span class="fm-dim"> / </span><span class="fm-val fm-dim" style="min-width:%dch">%d</span></td>`,
			wl, famTotalSent[fam], wr, famTotalTarget[fam])
	}
	h.writef(`<td class="fm-total fm-cell"><span class="fm-val" style="min-width:%dch">%d</span>`+
		`<span class="fm-dim"> / </span><span class="fm-val fm-dim" style="min-width:%dch">%d</span></td>`,
		totalWidthL, grandSent, totalWidthR, grandTarget)
	h.write(`</tr></tfoot></table></div>`)

	h.writef(`<div class="histogram-stats">
  <span class="stat"><span class="stat-label">Peers </span><span class="stat-value">%d</span></span>
  <span class="stat"><span class="stat-label">Families </span><span class="stat-value">%d</span></span>
  <span class="stat"><span class="stat-label">Announced </span><span class="stat-value">%d</span></span>
  <span class="stat"><span class="stat-label">Received </span><span class="stat-value">%d</span></span>
</div>`, len(peerIndices), len(families), grandSent, grandRecv)

	h.write(`<p class="viz-desc">Per-family route propagation for every peer. Each peer has a SEND row (sent/target) and RECV row (received/expected). ` +
		`Green = complete, red = zero, orange = partial. Color applies to the count only. ` +
		`Empty cells mean the peer did not negotiate that family.</p>
</div>`)
}

// familyCellClass returns a CSS class name for coloring a route count span.
// Green when fully propagated, red when nothing received, orange when partial.
func familyCellClass(current, target int) string {
	if target <= 0 {
		return ""
	}
	if current == 0 {
		return "fm-pending"
	}
	if current < target {
		return "fm-partial"
	}
	return "fm-complete"
}

// digitCount returns the number of decimal digits in n (minimum 1).
func digitCount(n int) int {
	if n < 0 {
		n = -n
	}
	d := 1
	for n >= 10 {
		n /= 10
		d++
	}
	return d
}

// writeAllPeers renders a complete sortable list of all peers.
func writeAllPeers(w io.Writer, s *DashboardState, sortCol, sortDir string) {
	h := &htmlWriter{w: w}

	// Build sorted list of all peer indices.
	indices := make([]int, 0, len(s.Peers))
	for idx := range s.Peers {
		indices = append(indices, idx)
	}
	sortPeers(indices, s, sortCol, sortDir)

	h.write(`<div class="viz-panel" hx-get="/viz/all-peers" hx-trigger="every 500ms [!window._frozen]" hx-target="#viz-content" hx-swap="innerHTML"
     hx-include="[name='sort'],[name='dir']">
<div class="viz-header">
  <h3>All Peers</h3>
</div>
<div class="all-peers-scroll"><table class="peer-table">
<thead><tr>
  <th hx-get="/viz/all-peers" hx-target="#viz-content" hx-swap="innerHTML"
      hx-vals='{"sort":"id","dir":"asc"}'>ID</th>
  <th hx-get="/viz/all-peers" hx-target="#viz-content" hx-swap="innerHTML"
      hx-vals='{"sort":"status","dir":"asc"}'>Status</th>
  <th hx-get="/viz/all-peers" hx-target="#viz-content" hx-swap="innerHTML"
      hx-vals='{"sort":"sent","dir":"desc"}'>Sent</th>
  <th hx-get="/viz/all-peers" hx-target="#viz-content" hx-swap="innerHTML"
      hx-vals='{"sort":"received","dir":"desc"}'>Recv</th>
  <th>Missing</th>
  <th hx-get="/viz/all-peers" hx-target="#viz-content" hx-swap="innerHTML"
      hx-vals='{"sort":"chaos","dir":"desc"}'>Chaos</th>
  <th>Reconn</th>
  <th>Families</th>
</tr></thead>
<tbody>`)

	var totalUp, totalDown, totalReconn, totalIdle int

	for _, idx := range indices {
		ps := s.Peers[idx]
		if ps == nil {
			continue
		}

		switch ps.Status {
		case PeerUp:
			totalUp++
		case PeerDown:
			totalDown++
		case PeerReconnecting:
			totalReconn++
		case PeerIdle:
			totalIdle++
		}

		famStr := ""
		if len(ps.Families) > 0 {
			famStr = strings.Join(ps.Families, ", ")
		}

		h.writef(`<tr hx-get="/peer/%d" hx-target="#peer-detail" hx-swap="outerHTML">`, idx)
		h.writef(`<td>%d</td>`, idx)
		h.writef(`<td><span class="dot %s"></span>%s</td>`, ps.Status.CSSClass(), ps.Status.String())
		h.writef(`<td>%d</td>`, ps.RoutesSent)
		h.writef(`<td>%d</td>`, ps.RoutesRecv)
		h.writef(`<td>%d</td>`, ps.Missing)
		h.writef(`<td>%d</td>`, ps.ChaosCount)
		h.writef(`<td>%d</td>`, ps.Reconnects)
		h.writef(`<td class="fm-families">%s</td>`, escapeHTML(famStr))
		h.write(`</tr>`)
	}

	h.write(`</tbody></table></div>`)

	h.writef(`<div class="histogram-stats">
  <span class="stat"><span class="stat-label">Total </span><span class="stat-value">%d</span></span>
  <span class="stat"><span class="stat-label">Up </span><span class="stat-value" style="color:var(--green)">%d</span></span>
  <span class="stat"><span class="stat-label">Down </span><span class="stat-value" style="color:var(--red)">%d</span></span>
  <span class="stat"><span class="stat-label">Reconnecting </span><span class="stat-value" style="color:var(--yellow)">%d</span></span>
  <span class="stat"><span class="stat-label">Idle </span><span class="stat-value">%d</span></span>
</div>`, len(indices), totalUp, totalDown, totalReconn, totalIdle)

	h.write(`<p class="viz-desc">Complete list of all peers, not just the active set. Click a row to view peer details. Sortable by column headers.</p>
</div>`)
}
