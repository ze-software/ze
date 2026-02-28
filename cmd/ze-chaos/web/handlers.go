// Design: docs/architecture/chaos-web-dashboard.md — web dashboard UI

package web

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
)

//go:embed assets
var assetsFS embed.FS

// registerRoutes wires all HTTP routes for the dashboard.
func registerRoutes(mux *http.ServeMux, d *Dashboard) error {
	assetsDir, err := fs.Sub(assetsFS, "assets")
	if err != nil {
		return fmt.Errorf("embedded assets sub-fs: %w", err)
	}
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetsDir))))

	// SSE endpoint.
	mux.Handle("GET /events", d.broker)

	// Page and fragments.
	mux.HandleFunc("GET /", d.handleIndex)
	mux.HandleFunc("GET /peers", d.handlePeers)
	mux.HandleFunc("GET /peer/close", d.handlePeerClose)
	mux.HandleFunc("GET /peer/{id}", d.handlePeerDetail)
	mux.HandleFunc("POST /peers/{id}/pin", d.handlePeerPin)

	// Visualization tabs.
	mux.HandleFunc("GET /viz/events", d.handleVizEvents)
	mux.HandleFunc("GET /viz/convergence", d.handleVizConvergence)
	mux.HandleFunc("GET /viz/peer-timeline", d.handleVizPeerTimeline)
	mux.HandleFunc("GET /viz/chaos-events", d.handleVizChaosEvents)
	mux.HandleFunc("GET /viz/chaos-timeline", d.handleVizChaosTimeline)
	mux.HandleFunc("GET /viz/route-matrix", d.handleVizRouteMatrix)
	mux.HandleFunc("GET /viz/route-matrix/cell", d.handleVizRouteMatrixCell)
	mux.HandleFunc("GET /viz/families", d.handleVizFamilies)
	mux.HandleFunc("GET /viz/all-peers", d.handleVizAllPeers)

	// Multi-panel viz layout.
	mux.HandleFunc("GET /viz/panels", d.handleVizPanels)
	mux.HandleFunc("GET /viz/panel-content", d.handleVizPanelContent)

	// Convergence trend (rolling percentile chart).
	mux.HandleFunc("GET /viz/convergence-trend", d.handleVizConvergenceTrend)

	// Sidebar polling fallbacks (supplement SSE).
	mux.HandleFunc("GET /sidebar/stats", d.handleSidebarStats)
	mux.HandleFunc("GET /sidebar/active-set", d.handleSidebarActiveSet)
	mux.HandleFunc("GET /sidebar/events", d.handleSidebarEvents)

	// Peer view modes (grid/table toggle).
	mux.HandleFunc("GET /peers/grid", d.handlePeersGrid)
	mux.HandleFunc("GET /peers/table", d.handlePeersTable)

	// Peer promote (peer picker).
	mux.HandleFunc("POST /peers/promote", d.handlePeerPromote)

	// Active set configuration.
	mux.HandleFunc("POST /active-set/max-visible", d.handleActiveSetMaxVisible)

	// Control endpoints (active only when control channel is configured).
	mux.HandleFunc("POST /control/pause", d.handleControlPause)
	mux.HandleFunc("POST /control/resume", d.handleControlResume)
	mux.HandleFunc("POST /control/rate", d.handleControlRate)
	mux.HandleFunc("POST /control/trigger", d.handleControlTrigger)
	mux.HandleFunc("POST /control/stop", d.handleControlStop)
	mux.HandleFunc("POST /control/restart", d.handleControlRestart)

	// Speed control endpoint (in-process mode only).
	mux.HandleFunc("POST /control/speed", d.handleControlSpeed)

	// Route dynamics control endpoints.
	mux.HandleFunc("POST /control/route/pause", d.handleRouteControlPause)
	mux.HandleFunc("POST /control/route/resume", d.handleRouteControlResume)
	mux.HandleFunc("POST /control/route/rate", d.handleRouteControlRate)
	mux.HandleFunc("POST /control/route/stop", d.handleRouteControlStop)

	return nil
}

// handleIndex serves the full dashboard HTML page.
func (d *Dashboard) handleIndex(w http.ResponseWriter, _ *http.Request) {
	d.state.RLock()
	defer d.state.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writeLayout(w, d)
}

// handlePeers serves the peer table body fragment with sorting and filtering.
// Query params: sort (column), dir (asc/desc), status (up/down/reconnecting/idle), search (text filter).
func (d *Dashboard) handlePeers(w http.ResponseWriter, r *http.Request) {
	d.state.RLock()
	defer d.state.RUnlock()

	sortCol := r.URL.Query().Get("sort")
	sortDir := r.URL.Query().Get("dir")
	statusFilter := r.URL.Query().Get("status")
	search := r.URL.Query().Get("search")

	// Get active set peer indices.
	indices := d.state.Active.Indices()

	indices = filterPeerIndices(d.state, indices, statusFilter, search)

	// Sort.
	sortPeers(indices, d.state, sortCol, sortDir)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h := &htmlWriter{w: w}
	h.write(`<tbody id="peer-tbody">`)
	writePeerRows(w, d.state, indices)
	h.write(`</tbody>`)
}

// filterPeerIndices applies status and search filters to a peer index list.
func filterPeerIndices(state *DashboardState, indices []int, statusFilter, search string) []int {
	if statusFilter == "fault" {
		var filtered []int
		for _, idx := range indices {
			ps := state.Peers[idx]
			if ps != nil && ps.Status != PeerUp && ps.Status != PeerSyncing {
				filtered = append(filtered, idx)
			}
		}
		indices = filtered
	} else if statusFilter != "" {
		var filtered []int
		for _, idx := range indices {
			ps := state.Peers[idx]
			if ps != nil && ps.Status.String() == statusFilter {
				filtered = append(filtered, idx)
			}
		}
		indices = filtered
	}
	if search != "" {
		var filtered []int
		for _, idx := range indices {
			ps := state.Peers[idx]
			if ps != nil && peerMatchesSearch(ps, search) {
				filtered = append(filtered, idx)
			}
		}
		indices = filtered
	}
	return indices
}

// peerMatchesSearch returns true if the peer matches the search text.
//
// Filter syntax (when search contains "," or starts with "-" followed by a digit):
//
//	"1,2,4"     — show only peers 1, 2, 4
//	"1-10"      — show peers 1 through 10
//	"-3,-5"     — show all peers except 3 and 5
//	"1-10,-5"   — show peers 1–10 except 5
//
// Otherwise falls back to text search: index prefix or status substring.
func peerMatchesSearch(ps *PeerState, search string) bool {
	if search == "" {
		return true
	}
	if isFilterSyntax(search) {
		return peerMatchesFilter(ps.Index, search)
	}
	lower := strings.ToLower(search)
	// Match peer index as string prefix.
	if strings.HasPrefix(strconv.Itoa(ps.Index), lower) {
		return true
	}
	// Match status text as case-insensitive substring.
	return strings.Contains(ps.Status.String(), lower)
}

// isFilterSyntax returns true when search looks like a structured filter
// (contains commas, or starts with "-" followed by a digit).
func isFilterSyntax(s string) bool {
	if strings.Contains(s, ",") {
		return true
	}
	if len(s) >= 2 && s[0] == '-' && s[1] >= '0' && s[1] <= '9' {
		return true
	}
	// Check for range: digit-digit pattern.
	for i := 1; i < len(s)-1; i++ {
		if s[i] == '-' && s[i-1] >= '0' && s[i-1] <= '9' && s[i+1] >= '0' && s[i+1] <= '9' {
			return true
		}
	}
	return false
}

// peerMatchesFilter checks whether peerIdx is included by the filter expression.
func peerMatchesFilter(peerIdx int, filter string) bool {
	var includes []bool // true if any include term was seen
	excluded := false

	for tok := range strings.SplitSeq(filter, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}

		// Exclusion: -N
		if len(tok) >= 2 && tok[0] == '-' {
			if n, err := strconv.Atoi(tok[1:]); err == nil {
				if peerIdx == n {
					excluded = true
				}
				continue
			}
		}

		// Range: N-M
		if dashIdx := strings.Index(tok, "-"); dashIdx > 0 && dashIdx < len(tok)-1 {
			lo, errLo := strconv.Atoi(tok[:dashIdx])
			hi, errHi := strconv.Atoi(tok[dashIdx+1:])
			if errLo == nil && errHi == nil {
				includes = append(includes, peerIdx >= lo && peerIdx <= hi)
				continue
			}
		}

		// Single index: N
		if n, err := strconv.Atoi(tok); err == nil {
			includes = append(includes, peerIdx == n)
			continue
		}
	}

	if excluded {
		return false
	}
	// If there were include terms, peer must match at least one.
	if len(includes) > 0 {
		for _, m := range includes {
			if m {
				return true
			}
		}
		return false
	}
	// Only exclusions (no includes) — peer passed exclusion check above.
	return true
}

// handlePeersGrid serves the peer grid view with all peers as colored cells.
// Query params: status (filter by peer status), search (text filter).
func (d *Dashboard) handlePeersGrid(w http.ResponseWriter, r *http.Request) {
	d.state.RLock()
	defer d.state.RUnlock()

	statusFilter := r.URL.Query().Get("status")
	search := r.URL.Query().Get("search")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writePeerGridFiltered(w, d.state, statusFilter, search)
}

// handlePeersTable serves the full peer table container (thead + tbody) for
// toggling back from grid view. The table includes sort headers and active set rows.
// Query params: status (filter by peer status), search (text filter).
func (d *Dashboard) handlePeersTable(w http.ResponseWriter, r *http.Request) {
	d.state.RLock()
	defer d.state.RUnlock()

	statusFilter := r.URL.Query().Get("status")
	search := r.URL.Query().Get("search")

	indices := d.state.Active.Indices()
	indices = filterPeerIndices(d.state, indices, statusFilter, search)
	sortPeers(indices, d.state, "id", "asc")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writePeerTable(w, d.state, indices)
}

// handlePeerClose returns an empty detail div, clearing the detail pane.
func (d *Dashboard) handlePeerClose(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h := &htmlWriter{w: w}
	h.write(`<div id="peer-detail"></div>`)
}

// handlePeerDetail serves the detail pane fragment for a single peer.
func (d *Dashboard) handlePeerDetail(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	idx, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid peer id", http.StatusBadRequest)
		return
	}

	d.state.RLock()
	defer d.state.RUnlock()

	ps := d.state.Peers[idx]
	if ps == nil {
		http.Error(w, "peer not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writePeerDetail(w, ps, d.state.Active.IsPinned(idx), d.state.SortedFamilies())
}

// handlePeerPin toggles the pin state for a peer.
func (d *Dashboard) handlePeerPin(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	idx, err := strconv.Atoi(idStr)
	if err != nil || idx < 0 || idx >= d.state.PeerCount {
		http.Error(w, "invalid peer id", http.StatusBadRequest)
		return
	}

	d.state.mu.Lock()
	if d.state.Active.IsPinned(idx) {
		d.state.Active.Unpin(idx)
	} else {
		d.state.Active.Pin(idx, time.Now())
	}
	d.state.mu.Unlock()

	// Return updated peer row.
	d.state.RLock()
	defer d.state.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	row := d.renderPeerRow(idx)
	if _, err := w.Write([]byte(row)); err != nil {
		d.logger.Debug("write peer row", "error", err)
	}
}

// handlePeerPromote adds a specific peer to the active set (peer picker).
func (d *Dashboard) handlePeerPromote(w http.ResponseWriter, r *http.Request) {
	idStr := r.FormValue("id")
	idx, err := strconv.Atoi(idStr)
	if err != nil || idx < 0 || idx >= d.state.PeerCount {
		http.Error(w, "invalid peer id", http.StatusBadRequest)
		return
	}

	d.state.mu.Lock()
	d.state.Active.Promote(idx, PriorityManual, time.Now())
	d.state.mu.Unlock()

	// Return updated peer table.
	d.state.RLock()
	defer d.state.RUnlock()

	indices := d.state.Active.Indices()
	sortPeers(indices, d.state, "id", "asc")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h := &htmlWriter{w: w}
	h.write(`<tbody id="peer-tbody">`)
	writePeerRows(w, d.state, indices)
	h.write(`</tbody>`)
}

// handleSidebarStats returns the stats card content (polling fallback for SSE).
func (d *Dashboard) handleSidebarStats(w http.ResponseWriter, _ *http.Request) {
	d.state.RLock()
	defer d.state.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := io.WriteString(w, d.renderStats()); err != nil {
		d.logger.Debug("write sidebar stats", "error", err)
	}
}

// handleSidebarActiveSet returns the active set info fragment.
func (d *Dashboard) handleSidebarActiveSet(w http.ResponseWriter, _ *http.Request) {
	d.state.RLock()
	defer d.state.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h := &htmlWriter{w: w}
	h.writef(`<div id="active-set-info" hx-get="/sidebar/active-set" hx-trigger="every 500ms" hx-swap="outerHTML">
  <span class="stat" title="Peers currently shown in the table / maximum visible"><span class="stat-label">Visible </span><span class="stat-value">%d/%d</span></span>
  <span class="stat" title="Time before an inactive peer is removed from the table"><span class="stat-label">TTL </span><span class="stat-value">%s</span></span>
</div>`, d.state.Active.Len(), d.state.Active.MaxVisible, FormatDuration(d.state.Active.AdaptiveTTL()))
}

// handleActiveSetMaxVisible updates the maximum number of visible peers.
func (d *Dashboard) handleActiveSetMaxVisible(w http.ResponseWriter, r *http.Request) {
	nStr := r.FormValue("n")
	n, err := strconv.Atoi(nStr)
	if err != nil || n < 1 {
		http.Error(w, "invalid max-visible value", http.StatusBadRequest)
		return
	}
	if n > d.state.PeerCount {
		n = d.state.PeerCount
	}

	d.state.mu.Lock()
	d.state.Active.SetMaxVisible(n)
	d.state.mu.Unlock()

	// Return updated active set info (same as polling fallback).
	d.state.RLock()
	defer d.state.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h := &htmlWriter{w: w}
	h.writef(`<div id="active-set-info" hx-get="/sidebar/active-set" hx-trigger="every 500ms" hx-swap="outerHTML">
  <span class="stat" title="Peers currently shown in the table / maximum visible"><span class="stat-label">Visible </span><span class="stat-value">%d/%d</span></span>
  <span class="stat" title="Time before an inactive peer is removed from the table"><span class="stat-label">TTL </span><span class="stat-value">%s</span></span>
</div>`, d.state.Active.Len(), d.state.Active.MaxVisible, FormatDuration(d.state.Active.AdaptiveTTL()))
}

// handleSidebarEvents returns the recent events fragment (polling fallback for SSE).
func (d *Dashboard) handleSidebarEvents(w http.ResponseWriter, _ *http.Request) {
	d.state.RLock()
	defer d.state.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h := &htmlWriter{w: w}
	h.write(`<div id="events" class="event-list" sse-swap="events" hx-swap="outerHTML" hx-get="/sidebar/events" hx-trigger="every 500ms">`)
	writeRecentEvents(w, d.state)
	h.write(`</div>`)
}

// sortPeers sorts peer indices by the given column and direction.
func sortPeers(indices []int, state *DashboardState, col, dir string) {
	desc := strings.EqualFold(dir, "desc")

	slices.SortFunc(indices, func(a, b int) int {
		pa, pb := state.Peers[a], state.Peers[b]
		if pa == nil || pb == nil {
			return 0
		}
		var cmp int
		switch col {
		case "status":
			cmp = int(pa.Status) - int(pb.Status)
		case "sent":
			cmp = pa.RoutesSent - pb.RoutesSent
		case "received":
			cmp = pa.RoutesRecv - pb.RoutesRecv
		case "bytes-out":
			cmp = int(pa.BytesSent - pb.BytesSent)
		case "bytes-in":
			cmp = int(pa.BytesRecv - pb.BytesRecv)
		case "rate-out":
			cmp = floatCmp(pa.throughputOut, pb.throughputOut)
		case "rate-in":
			cmp = floatCmp(pa.throughputIn, pb.throughputIn)
		case "chaos":
			cmp = pa.ChaosCount - pb.ChaosCount
		default: // sort by index
			cmp = a - b
		}
		if desc {
			cmp = -cmp
		}
		return cmp
	})
}

// floatCmp returns -1, 0, or 1 for float64 comparison (for sort).
func floatCmp(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
