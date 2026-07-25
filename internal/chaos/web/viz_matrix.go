// Design: docs/architecture/chaos-web-dashboard.md -- Route matrix and family visualization
// Related: viz.go -- event stream, viz_timeline.go -- peer timeline

package web

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

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
	fam := r.URL.Query().Get("family")

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
		family:      fam,
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
		var totalExpectedTarget int
		for _, fam := range families {
			if neg[fam] {
				totalExpectedTarget += famTotalTarget[fam] - ps.FamilySentTarget[fam]
			}
		}
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

		var recvExpectedTotal int
		for _, fam := range families {
			if negotiated[fam] {
				recvExpectedTotal += famTotalTarget[fam] - ps.FamilySentTarget[fam]
			}
		}
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
      hx-vals='{"sort":"sent","dir":"desc"}' title="BGP messages (routes) sent to Ze">Msgs&#x2192;</th>
  <th hx-get="/viz/all-peers" hx-target="#viz-content" hx-swap="innerHTML"
      hx-vals='{"sort":"received","dir":"desc"}' title="BGP messages (routes) received from Ze">Msgs&#x2190;</th>
  <th hx-get="/viz/all-peers" hx-target="#viz-content" hx-swap="innerHTML"
      hx-vals='{"sort":"bytes-out","dir":"desc"}' title="Total bytes sent to Ze">Bytes&#x2192;</th>
  <th hx-get="/viz/all-peers" hx-target="#viz-content" hx-swap="innerHTML"
      hx-vals='{"sort":"bytes-in","dir":"desc"}' title="Total bytes received from Ze">Bytes&#x2190;</th>
  <th hx-get="/viz/all-peers" hx-target="#viz-content" hx-swap="innerHTML"
      hx-vals='{"sort":"rate-out","dir":"desc"}' title="Current send bit rate to Ze">Rate&#x2192;</th>
  <th hx-get="/viz/all-peers" hx-target="#viz-content" hx-swap="innerHTML"
      hx-vals='{"sort":"rate-in","dir":"desc"}' title="Current receive bit rate from Ze">Rate&#x2190;</th>
  <th>Missing</th>
  <th hx-get="/viz/all-peers" hx-target="#viz-content" hx-swap="innerHTML"
      hx-vals='{"sort":"chaos","dir":"desc"}'>Chaos</th>
  <th>Reconn</th>
  <th>Families</th>
</tr></thead>
<tbody>`)

	var totalUp, totalSyncing, totalDown, totalReconn, totalIdle int

	for _, idx := range indices {
		ps := s.Peers[idx]
		if ps == nil {
			continue
		}

		switch ps.Status {
		case PeerUp:
			totalUp++
		case PeerSyncing:
			totalSyncing++
		case PeerDown:
			totalDown++
		case PeerReconnecting:
			totalReconn++
		case PeerIdle:
			totalIdle++
		}

		famStr := ""
		if len(ps.Families) > 0 {
			famStr = textbuf.Join(ps.Families, ", ")
		}

		h.writef(`<tr hx-get="/peer/%d" hx-target="#peer-detail" hx-swap="outerHTML">`, idx)
		h.writef(`<td>%d</td>`, idx)
		h.writef(`<td><span class="dot %s"></span>%s</td>`, ps.Status.CSSClass(), ps.Status.String())
		h.writef(`<td>%d</td>`, ps.RoutesSent)
		h.writef(`<td>%d</td>`, ps.RoutesRecv)
		h.writef(`<td>%s</td>`, FormatBytes(ps.BytesSent))
		h.writef(`<td>%s</td>`, FormatBytes(ps.BytesRecv))
		h.writef(`<td>%s</td>`, FormatBitRate(ps.throughputOut))
		h.writef(`<td>%s</td>`, FormatBitRate(ps.throughputIn))
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
  <span class="stat"><span class="stat-label">Syncing </span><span class="stat-value" style="color:var(--accent)">%d</span></span>
  <span class="stat"><span class="stat-label">Down </span><span class="stat-value" style="color:var(--red)">%d</span></span>
  <span class="stat"><span class="stat-label">Reconnecting </span><span class="stat-value" style="color:var(--yellow)">%d</span></span>
  <span class="stat"><span class="stat-label">Idle </span><span class="stat-value">%d</span></span>
</div>`, len(indices), totalUp, totalSyncing, totalDown, totalReconn, totalIdle)

	h.write(`<p class="viz-desc">Complete list of all peers, not just the active set. Click a row to view peer details. Sortable by column headers.</p>
</div>`)
}
