package web

import (
	"embed"
	"fmt"
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
	mux.HandleFunc("GET /viz/chaos-timeline", d.handleVizChaosTimeline)
	mux.HandleFunc("GET /viz/route-matrix", d.handleVizRouteMatrix)
	mux.HandleFunc("GET /viz/route-matrix/cell", d.handleVizRouteMatrixCell)

	// Peer promote (peer picker).
	mux.HandleFunc("POST /peers/promote", d.handlePeerPromote)

	// Control endpoints (active only when control channel is configured).
	mux.HandleFunc("POST /control/pause", d.handleControlPause)
	mux.HandleFunc("POST /control/resume", d.handleControlResume)
	mux.HandleFunc("POST /control/rate", d.handleControlRate)
	mux.HandleFunc("POST /control/trigger", d.handleControlTrigger)
	mux.HandleFunc("POST /control/stop", d.handleControlStop)
	mux.HandleFunc("GET /control/trigger-form", d.handleControlTriggerForm)

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
// Query params: sort (column), dir (asc/desc), status (up/down/reconnecting/idle).
func (d *Dashboard) handlePeers(w http.ResponseWriter, r *http.Request) {
	d.state.RLock()
	defer d.state.RUnlock()

	sortCol := r.URL.Query().Get("sort")
	sortDir := r.URL.Query().Get("dir")
	statusFilter := r.URL.Query().Get("status")

	// Get active set peer indices.
	indices := d.state.Active.Indices()

	// Filter by status if requested.
	if statusFilter != "" {
		var filtered []int
		for _, idx := range indices {
			ps := d.state.Peers[idx]
			if ps != nil && ps.Status.String() == statusFilter {
				filtered = append(filtered, idx)
			}
		}
		indices = filtered
	}

	// Sort.
	sortPeers(indices, d.state, sortCol, sortDir)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<tbody id="peer-tbody">`)
	writePeerRows(w, d.state, indices)
	fmt.Fprint(w, `</tbody>`)
}

// handlePeerClose returns an empty detail div, clearing the detail pane.
func (d *Dashboard) handlePeerClose(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<div id="peer-detail"></div>`)
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
	writePeerDetail(w, ps, d.state.Active.IsPinned(idx))
}

// handlePeerPin toggles the pin state for a peer.
func (d *Dashboard) handlePeerPin(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	idx, err := strconv.Atoi(idStr)
	if err != nil {
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
	fmt.Fprint(w, `<tbody id="peer-tbody">`)
	writePeerRows(w, d.state, indices)
	fmt.Fprint(w, `</tbody>`)
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
