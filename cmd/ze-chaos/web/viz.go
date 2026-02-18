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
// Query params: page (1-based).
func (d *Dashboard) handleVizPeerTimeline(w http.ResponseWriter, r *http.Request) {
	d.state.RLock()
	defer d.state.RUnlock()

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writePeerTimeline(w, d.state, page)
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
	h.write(`<div class="viz-panel">
<div class="viz-header">
  <h3>Event Stream</h3>
  <div class="filters">
    <label>Peer:</label>
    <select hx-get="/viz/events" hx-target="#viz-content" hx-swap="outerHTML"
            name="peer" hx-include="[name='type']">
      <option value="">All</option>`)

	for i := range s.PeerCount {
		h.writef(`<option value="%d"%s>Peer %d</option>`, i, selAttr(peerFilter == itoa(i)), i)
	}

	h.write(`
    </select>
    <label>Type:</label>
    <select hx-get="/viz/events" hx-target="#viz-content" hx-swap="outerHTML"
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
		elapsed := FormatDuration(time.Since(ev.Time))
		label := eventTypeLabel(ev.Type)
		detail := eventDetail(ev)
		h.writef(`<div class="event-row"><span class="event-time">%s ago</span><span class="event-type %s">p%d</span><span class="event-type %s">%s</span><span>%s</span></div>`,
			elapsed, evClass, ev.PeerIndex, evClass, label, detail)
	}

	h.write(`</div></div>`)
}

// writeConvergenceHistogram renders the CSS bar chart for convergence latency.
func writeConvergenceHistogram(w io.Writer, ch *ConvergenceHistogram, deadline time.Duration) {
	hw := &htmlWriter{w: w}
	hw.write(`<div class="viz-panel" id="viz-convergence">
<h3>Convergence Histogram</h3>
<div class="histogram" style="position:relative">`)

	maxCount := ch.MaxCount()
	bucketColors := []string{
		"#3fb950", "#3fb950", "#7cc647", // green (fast)
		"#b8cc3e", "#d29922", "#db8928", // yellow (moderate)
		"#db6d28", "#f85149", "#f85149", // red (slow)
	}

	for i, b := range ch.Buckets {
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
		for i, b := range ch.Buckets {
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
		hw.writef(`<span class="stat"><span class="stat-label">Slow (&gt;1s) </span><span class="stat-value" style="color:var(--red)">%d</span></span>`, ch.SlowCount)
	}

	hw.write(`</div></div>`)
}

// writePeerTimeline renders horizontal bars showing per-peer state over time.
// Paginated at 30 peers per page.
func writePeerTimeline(w io.Writer, s *DashboardState, page int) {
	h := &htmlWriter{w: w}
	const peersPerPage = 30
	elapsed := time.Since(s.StartTime)
	if elapsed == 0 {
		elapsed = time.Second // Avoid division by zero.
	}

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

	h.write(`<div class="viz-panel">
<div class="viz-header">
  <h3>Peer State Timeline</h3>
  <div class="filters">`)

	if totalPages > 1 {
		h.writef(`<span class="stat-label">Page %d/%d</span>`, page, totalPages)
		if page > 1 {
			h.writef(` <span class="badge" hx-get="/viz/peer-timeline?page=%d" hx-target="#viz-content" hx-swap="outerHTML">Prev</span>`, page-1)
		}
		if page < totalPages {
			h.writef(` <span class="badge" hx-get="/viz/peer-timeline?page=%d" hx-target="#viz-content" hx-swap="outerHTML">Next</span>`, page+1)
		}
	}

	h.writef(`
  </div>
</div>
<div class="timeline-container" style="--timeline-duration:%d">`, int(elapsed.Seconds()))

	pagePeers := peerIndices[startIdx:endIdx]
	for _, idx := range pagePeers {
		writeTimelineRow(w, s, idx, elapsed)
	}

	h.write(`</div></div>`)
}

// writeTimelineRow renders a single peer's timeline bar.
func writeTimelineRow(w io.Writer, s *DashboardState, idx int, elapsed time.Duration) {
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
		// Render segments between transitions.
		for i, tr := range transitions {
			startPct := pctOfDuration(tr.Time.Sub(s.StartTime), elapsed)
			var endPct int
			if i+1 < len(transitions) {
				endPct = pctOfDuration(transitions[i+1].Time.Sub(s.StartTime), elapsed)
			} else {
				endPct = 100
			}
			width := max(endPct-startPct, 1)
			color := statusColor(tr.Status)
			h.writef(`<div class="timeline-segment" style="left:%d%%;width:%d%%;background:%s" title="%s at %s"></div>`,
				startPct, width, color, tr.Status.String(), FormatDuration(tr.Time.Sub(s.StartTime)))
		}
	}

	h.write(`</div></div>`)
}

// writeChaosTimeline renders horizontal timeline with chaos event markers.
func writeChaosTimeline(w io.Writer, s *DashboardState, warmup time.Duration) {
	h := &htmlWriter{w: w}
	elapsed := time.Since(s.StartTime)
	if elapsed == 0 {
		elapsed = time.Second
	}

	h.write(`<div class="viz-panel">
<h3>Chaos Timeline</h3>
<div class="chaos-timeline">
<div class="chaos-timeline-track" style="position:relative">`)

	// Warmup region shading.
	if warmup > 0 {
		warmupPct := pctOfDuration(warmup, elapsed)
		if warmupPct > 0 {
			h.writef(`<div class="warmup-region" style="width:%d%%" title="Warmup: %s"></div>`,
				warmupPct, FormatDuration(warmup))
		}
	}

	// Action colors — ordered slice for deterministic legend rendering.
	type actionColor struct {
		name  string
		color string
	}
	actionColors := []actionColor{
		{"blackhole", "#8b949e"},
		{"corrupt", "#bc8cff"},
		{"delay", "#d29922"},
		{"disconnect", "#f85149"},
		{"flap", "#ffa657"},
		{"partition", "#ff7b72"},
		{"prefix-hijack", "#d2a8ff"},
		{"reset", "#f85149"},
		{"route-leak", "#79c0ff"},
		{"withdraw", "#db6d28"},
	}

	colorMap := make(map[string]string, len(actionColors))
	for _, ac := range actionColors {
		colorMap[ac.name] = ac.color
	}

	for _, entry := range s.ChaosHistory {
		leftPct := pctOfDuration(entry.Time.Sub(s.StartTime), elapsed)
		color, ok := colorMap[entry.Action]
		if !ok {
			color = "#8b949e"
		}
		h.writef(`<div class="chaos-marker" style="left:%d%%;background:%s" title="p%d: %s at %s" hx-get="/peer/%d" hx-target="#peer-detail" hx-swap="outerHTML"></div>`,
			leftPct, color, entry.PeerIndex, escapeAttr(entry.Action), FormatDuration(entry.Time.Sub(s.StartTime)), entry.PeerIndex)
	}

	h.write(`</div></div>
<div class="chaos-legend">`)

	for _, ac := range actionColors {
		h.writef(`<span class="legend-item"><span class="legend-swatch" style="background:%s"></span>%s</span>`, ac.color, ac.name)
	}

	h.writef(`</div>
<div class="histogram-stats">
  <span class="stat"><span class="stat-label">Total actions </span><span class="stat-value">%d</span></span>
  <span class="stat"><span class="stat-label">Duration </span><span class="stat-value">%s</span></span>
</div></div>`, len(s.ChaosHistory), FormatDuration(elapsed))
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

	h.write(`<div class="viz-panel">
<div class="viz-header">
  <h3>Route Flow Matrix</h3>
  <div class="filters">
    <label>Top:</label>
    <select hx-get="/viz/route-matrix" hx-target="#viz-content" hx-swap="outerHTML"
            name="top" hx-include="[name='mode'],[name='family'],[name='peers']">`)

	for _, n := range []int{10, 20, 30, 50} {
		h.writef(`<option value="%d"%s>%d</option>`, n, selAttr(n == opts.topN), n)
	}

	h.write(`
    </select>
    <label>Mode:</label>
    <select hx-get="/viz/route-matrix" hx-target="#viz-content" hx-swap="outerHTML"
            name="mode" hx-include="[name='top'],[name='family'],[name='peers']">`)
	h.writef(`<option value=""%s>Count</option>`, selAttr(!latencyMode))
	h.writef(`<option value="latency"%s>Latency</option>`, selAttr(latencyMode))

	h.write(`
    </select>
    <label>Family:</label>
    <select hx-get="/viz/route-matrix" hx-target="#viz-content" hx-swap="outerHTML"
            name="family" hx-include="[name='top'],[name='mode'],[name='peers']">
      <option value="">All</option>`)
	for _, fam := range m.Families() {
		h.writef(`<option value="%s"%s>%s</option>`, escapeAttr(fam), selAttr(fam == opts.family), fam)
	}

	h.write(`
    </select>
    <label>Peers:</label>
    <input type="text" name="peers" placeholder="e.g. 0,1,3" class="control-input"
           hx-get="/viz/route-matrix" hx-target="#viz-content" hx-swap="outerHTML"
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
		h.write(`<div class="stat-label" style="padding:16px">No route flow data yet.</div></div>`)
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
	h.writef(`
  <span class="stat"><span class="stat-label">Peers </span><span class="stat-value">%d</span></span>
</div></div>`, len(peers))
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
		style = fmt.Sprintf(` style="background:rgba(88,166,255,%.2f)"`, float64(intensity)/100.0)
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
		style = fmt.Sprintf(` style="background:rgba(219,109,40,%.2f)"`, float64(intensity)/100.0)
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
