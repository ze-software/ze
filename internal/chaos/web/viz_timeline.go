// Design: docs/architecture/chaos-web-dashboard.md -- Peer timeline visualization
// Related: viz.go -- event stream, viz_matrix.go -- route matrix

package web

import (
	"io"
	"net/http"
	"slices"
	"strconv"
	"time"
)

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

	h.writef(`<div class="viz-panel" hx-get="/viz/peer-timeline?page=%d" hx-trigger="every 500ms"`+freezePoll+` hx-target="#viz-content" hx-swap="innerHTML">
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
