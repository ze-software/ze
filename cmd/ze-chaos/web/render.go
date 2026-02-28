// Design: docs/architecture/chaos-web-dashboard.md — web dashboard UI
// Related: viz_panels.go — multi-panel viz layout and panel content handlers

package web

import (
	"fmt"
	"html"
	"io"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/peer"
)

// CSS class constants for pin state.
const (
	cssPinDefault = "pin"
	cssPinPinned  = "pin pinned"
)

// escapeHTML escapes HTML special characters for safe interpolation into templates.
var escapeHTML = html.EscapeString

// htmlWriter wraps an io.Writer and captures the first write error.
// Subsequent writes after an error are no-ops. This avoids per-call error
// checks when rendering HTML fragments to an http.ResponseWriter where
// write failures (client disconnect) are unrecoverable.
type htmlWriter struct {
	w   io.Writer
	err error
}

func (h *htmlWriter) write(s string) {
	if h.err == nil {
		_, h.err = io.WriteString(h.w, s)
	}
}

func (h *htmlWriter) writef(format string, args ...any) {
	if h.err == nil {
		_, h.err = fmt.Fprintf(h.w, format, args...)
	}
}

// writeLayout renders the full HTML page for the dashboard.
func writeLayout(w io.Writer, d *Dashboard) {
	h := &htmlWriter{w: w}
	s := d.state
	uptime := FormatDuration(time.Since(s.StartTime))

	h.write(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Ze Chaos</title>
<link rel="stylesheet" href="/assets/style.css">
<script src="/assets/htmx.min.js"></script>
<script src="/assets/sse.js"></script>
<script>
// Track which peer detail is currently shown (-1 = none).
var _shownPeer=-1;
// Toggle peer detail: if already shown, close it; otherwise let htmx load it.
function togglePeer(idx){
  if(_shownPeer===idx){
    document.getElementById('peer-detail').outerHTML='<div id="peer-detail"></div>';
    _shownPeer=-1;
    return false;
  }
  _shownPeer=idx;
  return true;
}
// [+] cell in grid: re-add an excluded peer by removing -N from search.
function gridAddPeer(){
  var grid=document.getElementById('peer-grid');
  if(!grid)return;
  var ex=grid.querySelector('.gp-input');
  if(ex){ex.focus();return}
  var cell=grid.querySelector('.peer-cell-add');
  if(!cell)return;
  var inp=document.createElement('input');
  inp.type='number';inp.min='0';inp.className='gp-input';
  inp.placeholder='#';
  inp.onkeydown=function(e){
    if(e.key==='Enter'&&this.value!==''){
      gridRestorePeer(parseInt(this.value));
      this.remove();
    } else if(e.key==='Escape'){this.remove()}
  };
  inp.onblur=function(){var self=this;setTimeout(function(){self.remove()},150)};
  cell.after(inp);
  inp.focus();
}
// [-] on a cell: add -N to search bar and refresh grid.
function gridRemovePeer(idx){
  var s=document.querySelector('[name="search"]');
  if(!s)return;
  var v=s.value.trim();
  if(v==='')v='-'+idx;
  else v+=(',-'+idx);
  s.value=v;
  htmx.trigger(s,'keyup');
}
// [+] re-add: remove -N from search bar and refresh grid.
function gridRestorePeer(idx){
  var s=document.querySelector('[name="search"]');
  if(!s)return;
  var parts=s.value.split(',').map(function(t){return t.trim()}).filter(function(t){return t!=='-'+idx});
  s.value=parts.join(',');
  htmx.trigger(s,'keyup');
}
// Trigger peer picker (sidebar).
function tpAddVal(n){
  var c=document.getElementById('trigger-peers');
  if(!c)return;
  if(c.querySelector('[data-peer="'+n+'"]'))return;
  var b=document.createElement('span');
  b.className='trigger-peer-badge';b.setAttribute('data-peer',n);
  var x=document.createElement('span');
  x.className='tp-remove';x.textContent='\u2212';
  x.onclick=function(){b.remove();tpSync()};
  b.appendChild(x);
  b.appendChild(document.createTextNode(' '+n));
  c.insertBefore(b,c.querySelector('.tp-add'));
  tpSync();
}
function tpSync(){
  var bs=document.querySelectorAll('#trigger-peers [data-peer]');
  var v=[];bs.forEach(function(b){v.push(b.getAttribute('data-peer'))});
  var el=document.getElementById('trigger-peers-value');
  if(el)el.value=v.join(',');
  var h=document.getElementById('tp-hint');
  if(h){h.style.display=v.length?'none':''}
}
</script>
</head>
<body>
<div class="layout" hx-ext="sse" sse-connect="/events">

<div class="header">
  <h1>Ze Chaos</h1>
  <span class="run-info">peers: ` + itoa(s.PeerCount) + ` | uptime: <span id="uptime" data-start="` + itoa(int(time.Since(s.StartTime).Seconds())) + `">` + uptime + `</span></span>
</div>`)

	// Control strip (between header and content).
	if s.Control.Status != "" {
		writeControlStrip(w, &s.Control)
	}
	h.write(`
<div class="content">
<div class="sidebar">
  <div class="card">
    <h3>Stats</h3>
    <div id="stats" sse-swap="stats" hx-swap="outerHTML" hx-get="/sidebar/stats" hx-trigger="every 500ms">`)

	// Donut chart showing peer status distribution.
	counts := s.StatusCounts()
	writeDonut(w, counts, s.PeerCount)
	writeDonutLegend(w, counts)
	writeDonutEnd(w)

	h.write(`
      <div class="stat-grid">
        <span></span><span class="stat-grid-header">Out</span><span class="stat-grid-header">In</span>
        <span class="stat-label">Msgs</span><span class="stat-value">` + itoa(s.TotalAnnounced) + `</span><span class="stat-value">` + itoa(s.TotalReceived) + `</span>
        <span class="stat-label">Bytes</span><span class="stat-value">` + FormatBytes(s.TotalBytesSent) + `</span><span class="stat-value">` + FormatBytes(s.TotalBytesRecv) + `</span>
        <span class="stat-label">Rate</span><span class="stat-value">` + FormatBitRate(s.AggregateThroughput(true)) + `</span><span class="stat-value">` + FormatBitRate(s.AggregateThroughput(false)) + `</span>
        <span class="stat-label">Wdraw</span><span class="stat-value">` + itoa(s.TotalWithdrawn) + `</span><span class="stat-value">` + itoa(s.TotalWdrawSent) + `</span>
      </div>
      <span class="stat"><span class="stat-label">Churn </span><span class="stat-value">` + itoa(s.TotalRouteActions) + `</span></span>
      <span class="stat"><span class="stat-label">Chaos </span><span class="stat-value">` + itoa(s.TotalChaos) + `</span></span>
      <span class="stat"><span class="stat-label">Reconn </span><span class="stat-value">` + itoa(s.TotalReconnects) + `</span></span>` +
		syncStat(s.EORCount, s.PeerCount, s.SyncDuration) + `
    </div>
  </div>

`)

	// Trigger sidebar (only when control channel is configured).
	if s.Control.Status != "" {
		writeControlSidebar(w, &s.Control)
	}

	// Route dynamics control panel (only when route control is configured).
	if s.Control.RouteStatus != "" {
		writeRouteControlPanel(w, &s.Control)
	}

	// Property badges.
	if len(s.Properties) > 0 {
		h.write(`
  <div class="card">
    <h3>Properties</h3>`)
		writePropertyBadges(w, s.Properties)
		h.write(`
  </div>`)
	}

	h.write(`
  <div class="card">
    <h3>Recent Events</h3>
    <div id="events" class="event-list" sse-swap="events" hx-swap="outerHTML" hx-get="/sidebar/events" hx-trigger="every 500ms">`)

	writeRecentEvents(w, s)

	h.write(`
    </div>
  </div>
</div>

<div class="main">
  <div id="toast-container" class="toast-container" sse-swap="toast" hx-swap="beforeend"></div>
  <div class="filters">`)
	writeTrackPeerControl(h, s)
	h.write(`<span class="strip-sep"></span>
    <input type="text" name="search" placeholder="Search peers..." autocomplete="off"
           hx-get="/peers/grid" hx-target="#peer-display" hx-swap="innerHTML"
           hx-trigger="keyup changed delay:200ms" hx-include="[name='status']">
    <label>Status:</label>
    <select hx-get="/peers/grid" hx-target="#peer-display" hx-swap="innerHTML" name="status"
            hx-include="[name='search']">
      <option value="" selected>All</option>
      <option value="fault">With Fault</option>
      <option value="up">Up</option>
      <option value="syncing">Syncing</option>
      <option value="down">Down</option>
      <option value="reconnecting">Reconnecting</option>
      <option value="idle">Idle</option>
    </select>`)
	writeActiveSetInfo(h, s)
	h.write(`
    <span class="view-toggle">
      <button class="view-btn" hx-get="/peers/table" hx-target="#peer-display" hx-swap="innerHTML"
              hx-include="[name='search'],[name='status']"
              onclick="document.querySelectorAll('.view-btn').forEach(b=>b.classList.remove('active'));this.classList.add('active')"
              title="Show peers as a detailed table (active set only)">Table</button>
      <button class="view-btn active" hx-get="/peers/grid" hx-target="#peer-display" hx-swap="innerHTML"
              onclick="document.querySelectorAll('.view-btn').forEach(b=>b.classList.remove('active'));this.classList.add('active')"
              title="Show all peers as a compact color-coded grid">Grid</button>
    </span>
  </div>

  <div id="peer-display">`)
	writePeerGrid(w, s)
	h.write(`
  </div>

  <div id="peer-detail"></div>

  <div id="peer-swap" sse-swap="peer-update" hx-swap="innerHTML" style="display:none"></div>
  <div id="peer-remove-swap" sse-swap="peer-remove" hx-swap="innerHTML" style="display:none"></div>
  <div id="peer-add-swap" sse-swap="peer-add" hx-swap="innerHTML" style="display:none"></div>

  <div class="tab-bar">
    <span class="tab-group-label">Peer</span>
    <button class="active" hx-get="/viz/families" hx-target="#viz-content" hx-swap="innerHTML" hx-trigger="load, click"
            onclick="document.querySelectorAll('.tab-bar button').forEach(b=>b.classList.remove('active'));this.classList.add('active')"
            title="Per-family sent/received route counts for all peers">Families</button>
    <button hx-get="/viz/convergence" hx-target="#viz-content" hx-swap="innerHTML"
            onclick="document.querySelectorAll('.tab-bar button').forEach(b=>b.classList.remove('active'));this.classList.add('active')"
            title="Route propagation latency distribution">Convergence</button>
    <button hx-get="/viz/route-matrix" hx-target="#viz-content" hx-swap="innerHTML"
            onclick="document.querySelectorAll('.tab-bar button').forEach(b=>b.classList.remove('active'));this.classList.add('active')"
            title="Heatmap of route announce/withdraw flow between peers">Route Matrix</button>
    <button hx-get="/viz/peer-timeline" hx-target="#viz-content" hx-swap="innerHTML"
            onclick="document.querySelectorAll('.tab-bar button').forEach(b=>b.classList.remove('active'));this.classList.add('active')"
            title="Peer session state changes over time">Timeline</button>
    <button hx-get="/viz/events" hx-target="#viz-content" hx-swap="innerHTML"
            onclick="document.querySelectorAll('.tab-bar button').forEach(b=>b.classList.remove('active'));this.classList.add('active')"
            title="Live feed of all BGP session and routing events">Events</button>
    <button hx-get="/viz/all-peers" hx-target="#viz-content" hx-swap="innerHTML"
            onclick="document.querySelectorAll('.tab-bar button').forEach(b=>b.classList.remove('active'));this.classList.add('active')"
            title="Complete list of all peers (not just the active set)">All Peers</button>
    <span class="tab-separator"></span>
    <span class="tab-group-label">Chaos</span>
    <button hx-get="/viz/chaos-timeline" hx-target="#viz-content" hx-swap="innerHTML"
            onclick="document.querySelectorAll('.tab-bar button').forEach(b=>b.classList.remove('active'));this.classList.add('active')"
            title="Visual timeline of chaos actions over time">Timeline</button>
    <button hx-get="/viz/chaos-events" hx-target="#viz-content" hx-swap="innerHTML"
            onclick="document.querySelectorAll('.tab-bar button').forEach(b=>b.classList.remove('active'));this.classList.add('active')"
            title="Table of chaos actions injected during the run">Events</button>
    <span class="tab-separator"></span>
    <button hx-get="/viz/panels" hx-target="#viz-content" hx-swap="innerHTML"
            onclick="document.querySelectorAll('.tab-bar button').forEach(b=>b.classList.remove('active'));this.classList.add('active')"
            title="Show multiple visualizations simultaneously in a 2×2 grid">Panels</button>
    <label class="freeze-toggle" title="Pause all live updates (for screenshots or copy/paste)">
      <input type="checkbox" id="freeze-updates" onchange="window._frozen=this.checked"> Freeze
    </label>
  </div>
  <div id="viz-content"></div>
</div>

</div>
</div>
<div id="conn-error" style="display:none;position:fixed;bottom:0;left:0;right:0;padding:8px 16px;background:#b91c1c;color:#fff;text-align:center;font-size:14px;z-index:999">
  Server disconnected
</div>
<script>
document.body.addEventListener('htmx:sendError',function(){document.getElementById('conn-error').style.display='block'});
document.body.addEventListener('htmx:responseError',function(){document.getElementById('conn-error').style.display='block'});
document.body.addEventListener('htmx:sseError',function(){document.getElementById('conn-error').style.display='block'});
document.body.addEventListener('htmx:afterRequest',function(e){if(!e.detail.failed)document.getElementById('conn-error').style.display='none'});
document.body.addEventListener('htmx:sseOpen',function(){document.getElementById('conn-error').style.display='none'});
(function(){var el=document.getElementById('uptime');if(!el)return;var s=parseInt(el.dataset.start,10)||0;function fmt(t){var h=Math.floor(t/3600),m=Math.floor((t%3600)/60),sec=t%60;if(h>0)return h+'h'+m+'m'+sec+'s';if(m>0)return m+'m'+sec+'s';return sec+'s';}setInterval(function(){s++;el.textContent=fmt(s)},1000)})();
</script>
</body>
</html>`)
}

// writeTrackPeerControl renders the "Track" input to promote a peer into the table.
func writeTrackPeerControl(h *htmlWriter, s *DashboardState) {
	h.writef(`<label class="stat-label" for="promote-id">Track </label>`+
		`<input type="number" id="promote-id" name="id" min="0" max="%d" placeholder="peer #" class="control-input"`+
		` title="Add a peer to the table by number — tracked peers appear in Table view" onkeydown="if(event.key==='Enter'){event.preventDefault();this.nextElementSibling.click()}">`+
		`<span class="badge" hx-post="/peers/promote" hx-target="#peer-tbody" hx-swap="outerHTML"`+
		` hx-include="#promote-id" title="Start tracking this peer in the table">Add</span>`, s.PeerCount-1)
}

// writeActiveSetInfo renders the active set stats and max-visible control.
func writeActiveSetInfo(h *htmlWriter, s *DashboardState) {
	h.write(`<span class="strip-sep"></span>`)
	h.writef(
		`<div id="active-set-info" hx-get="/sidebar/active-set" hx-trigger="every 500ms" hx-swap="outerHTML">`+
			`<span class="stat" title="Number of peers currently tracked in the table vs maximum allowed"><span class="stat-label">Visible </span><span class="stat-value">%d/%d</span></span>`+
			`<span class="stat" title="Adaptive time-to-live: inactive peers are removed from the table after this duration"><span class="stat-label">TTL </span><span class="stat-value">%s</span></span>`+
			`</div>`, s.Active.Len(), s.Active.MaxVisible, FormatDuration(s.Active.AdaptiveTTL()))
	h.writef(`<label class="stat-label" for="max-visible-input">Max </label>`+
		`<input type="number" id="max-visible-input" name="n" min="1" max="%d" value="%d" class="control-input"`+
		` title="Maximum number of peers tracked in the table">`+
		`<span class="badge" hx-post="/active-set/max-visible" hx-target="#active-set-info" hx-swap="outerHTML"`+
		` hx-include="#max-visible-input" title="Update max visible">Set</span>`, s.PeerCount, s.Active.MaxVisible)
}

// writePeerRows renders table rows for the given peer indices.
func writePeerRows(w io.Writer, state *DashboardState, indices []int) {
	h := &htmlWriter{w: w}
	for _, idx := range indices {
		ps := state.Peers[idx]
		if ps == nil {
			continue
		}
		pinned := state.Active.IsPinned(idx)
		pinClass := cssPinDefault
		if pinned {
			pinClass = cssPinPinned
		}
		h.writef(`<tr id="peer-%d" hx-get="/peer/%d" hx-target="#peer-detail" hx-swap="outerHTML">`, idx, idx)
		h.writef(`<td><span class="%s" hx-post="/peers/%d/pin" hx-swap="none" hx-trigger="click" onclick="event.stopPropagation()"></span></td>`, pinClass, idx)
		h.writef(`<td>%d</td>`, idx)
		h.writef(`<td><span class="dot %s"></span> %s</td>`, ps.Status.CSSClass(), ps.Status.String())
		h.writef(`<td>%d</td>`, ps.RoutesSent)
		h.writef(`<td>%d</td>`, ps.RoutesRecv)
		h.writef(`<td>%s</td>`, FormatBytes(ps.BytesSent))
		h.writef(`<td>%s</td>`, FormatBytes(ps.BytesRecv))
		h.writef(`<td>%s</td>`, FormatBitRate(ps.throughputOut))
		h.writef(`<td>%s</td>`, FormatBitRate(ps.throughputIn))
		h.writef(`<td>%d</td>`, ps.ChaosCount)
		h.write(`</tr>`)
	}
}

// writePeerTable renders the full peer table container with thead and tbody.
// Used by the table toggle to restore the table view from grid mode.
func writePeerTable(w io.Writer, state *DashboardState, indices []int) {
	h := &htmlWriter{w: w}
	h.write(`<div class="peer-table-container">
    <table class="peer-table">
      <thead>
        <tr>
          <th style="width:30px" title="Pin a peer to keep it visible in the table"></th>
          <th hx-get="/peers" hx-target="#peer-tbody" hx-swap="outerHTML"
              hx-vals='{"sort":"id","dir":"asc"}' hx-include="[name='search'],[name='status']" title="Peer index">ID</th>
          <th hx-get="/peers" hx-target="#peer-tbody" hx-swap="outerHTML"
              hx-vals='{"sort":"status","dir":"asc"}' hx-include="[name='search'],[name='status']" title="BGP session state">Status</th>
          <th hx-get="/peers" hx-target="#peer-tbody" hx-swap="outerHTML"
              hx-vals='{"sort":"sent","dir":"desc"}' hx-include="[name='search'],[name='status']" title="BGP messages (routes) sent to Ze">Msgs&#x2192;</th>
          <th hx-get="/peers" hx-target="#peer-tbody" hx-swap="outerHTML"
              hx-vals='{"sort":"received","dir":"desc"}' hx-include="[name='search'],[name='status']" title="BGP messages (routes) received from Ze">Msgs&#x2190;</th>
          <th hx-get="/peers" hx-target="#peer-tbody" hx-swap="outerHTML"
              hx-vals='{"sort":"bytes-out","dir":"desc"}' hx-include="[name='search'],[name='status']" title="Total bytes sent to Ze">Bytes&#x2192;</th>
          <th hx-get="/peers" hx-target="#peer-tbody" hx-swap="outerHTML"
              hx-vals='{"sort":"bytes-in","dir":"desc"}' hx-include="[name='search'],[name='status']" title="Total bytes received from Ze">Bytes&#x2190;</th>
          <th hx-get="/peers" hx-target="#peer-tbody" hx-swap="outerHTML"
              hx-vals='{"sort":"rate-out","dir":"desc"}' hx-include="[name='search'],[name='status']" title="Current send bit rate to Ze">Rate&#x2192;</th>
          <th hx-get="/peers" hx-target="#peer-tbody" hx-swap="outerHTML"
              hx-vals='{"sort":"rate-in","dir":"desc"}' hx-include="[name='search'],[name='status']" title="Current receive bit rate from Ze">Rate&#x2190;</th>
          <th hx-get="/peers" hx-target="#peer-tbody" hx-swap="outerHTML"
              hx-vals='{"sort":"chaos","dir":"desc"}' hx-include="[name='search'],[name='status']" title="Chaos events targeting this peer">Chaos</th>
        </tr>
      </thead>
      <tbody id="peer-tbody">`)
	writePeerRows(w, state, indices)
	h.write(`
      </tbody>
    </table>
  </div>`)
}

// writePeerGrid renders a compact grid of all peers as colored cells.
// Each cell is ~28x28px, colored by status, with a tooltip and click target.
// The grid container has HTMX polling to auto-refresh.
func writePeerGrid(w io.Writer, state *DashboardState) {
	writePeerGridFiltered(w, state, "", "")
}

// writePeerGridFiltered renders the peer grid with optional status and search filters.
// When statusFilter is empty, all statuses are shown.
// When search is empty, all peers are shown.
// The "fault" status filter matches down, reconnecting, and idle peers.
func writePeerGridFiltered(w io.Writer, state *DashboardState, statusFilter, search string) {
	h := &htmlWriter{w: w}
	h.write(`<div id="peer-grid" class="peer-grid" hx-get="/peers/grid" hx-trigger="every 2s" hx-swap="outerHTML" hx-include="[name='search'],[name='status']">`)
	// [+] cell: always first, always visible, opens input for adding a peer by number.
	h.write(`<div class="peer-cell peer-cell-add" onclick="gridAddPeer()" title="Add peer by number"><span class="pc-id">+</span></div>`)
	for idx := range state.PeerCount {
		ps := state.Peers[idx]
		if ps == nil {
			continue
		}
		if statusFilter == "fault" {
			if ps.Status == PeerUp || ps.Status == PeerSyncing {
				continue
			}
		} else if statusFilter != "" && ps.Status.String() != statusFilter {
			continue
		}
		if search != "" && !peerMatchesSearch(ps, search) {
			continue
		}
		pulseClass := ""
		if ps.ChaosActive {
			pulseClass = " pulse"
		}
		// Compute sent percentage from per-family targets.
		sentTarget := 0
		for _, t := range ps.FamilySentTarget {
			sentTarget += t
		}
		sentPct := pctOf(ps.RoutesSent, sentTarget)
		// Compute received percentage: expected = sum of all other peers' targets.
		recvTarget := state.TotalAnnounced - ps.RoutesSent
		recvPct := pctOf(ps.RoutesRecv, recvTarget)

		// onclick togglePeer: if this peer's detail is already shown, close it.
		// Otherwise let htmx fetch the detail.
		// [-] button in top-right corner excludes this peer from the grid via search bar.
		h.writef(`<div id="peer-cell-%d" class="peer-cell %s%s" title="Peer %d: %s | Sent: %d/%d Recv: %d/%d | %s | Chaos: %d" hx-get="/peer/%d" hx-target="#peer-detail" hx-swap="outerHTML" onclick="return togglePeer(%d)">`+
			`<span class="pc-remove" onclick="event.stopPropagation();gridRemovePeer(%d)" title="Hide peer %d">&minus;</span>`+
			`<span class="pc-id">%d</span>`+
			`<span class="pc-row"><span class="pc-label">S</span><span class="pc-pct">%s</span></span>`+
			`<span class="pc-row"><span class="pc-label">R</span><span class="pc-pct">%s</span></span>`+
			`</div>`,
			idx, ps.Status.CSSClass(), pulseClass, idx, ps.Status.String(),
			ps.RoutesSent, sentTarget, ps.RoutesRecv, recvTarget,
			FormatBitRate(ps.throughputOut), ps.ChaosCount, idx,
			idx, idx, idx, idx, sentPct, recvPct)
	}
	h.write(`</div>`)
}

// toastForEvent returns a ToastEntry for toast-worthy events, or false for non-toast events.
func toastForEvent(ev peer.Event) (ToastEntry, bool) {
	switch ev.Type {
	case peer.EventDisconnected:
		return ToastEntry{PeerIndex: ev.PeerIndex, Label: "disconnected", CSSClass: "toast-error", Time: ev.Time}, true
	case peer.EventReconnecting:
		return ToastEntry{PeerIndex: ev.PeerIndex, Label: "reconnecting", CSSClass: "toast-warn", Time: ev.Time}, true
	case peer.EventError:
		detail := ""
		if ev.Err != nil {
			detail = ev.Err.Error()
		}
		return ToastEntry{PeerIndex: ev.PeerIndex, Label: "error", Detail: detail, CSSClass: "toast-error", Time: ev.Time}, true
	case peer.EventChaosExecuted:
		return ToastEntry{PeerIndex: ev.PeerIndex, Label: "chaos", Detail: ev.ChaosAction, CSSClass: "toast-warn", Time: ev.Time}, true
	default:
		return ToastEntry{}, false
	}
}

// renderToast returns an HTML fragment for a single toast notification.
// Uses hx-swap-oob to append to the toast container.
func renderToast(t ToastEntry) string {
	detail := ""
	if t.Detail != "" {
		detail = ` — ` + escapeHTML(t.Detail)
	}
	return `<div class="toast ` + t.CSSClass + `" hx-swap-oob="beforeend:#toast-container" onclick="this.remove()" onanimationend="if(event.animationName==='toast-out')this.remove()">` +
		`<span class="toast-label">p` + itoa(t.PeerIndex) + ` ` + t.Label + detail + `</span>` +
		`<span class="toast-close">&times;</span>` +
		`</div>`
}

// donutStatusOrder defines the rendering order and colors for donut segments.
// Each entry maps a PeerStatus to its CSS color variable.
var donutStatusOrder = []struct {
	Status PeerStatus
	Color  string
	Label  string
}{
	{PeerUp, "var(--green)", "Up"},
	{PeerDown, "var(--red)", "Down"},
	{PeerReconnecting, "var(--yellow)", "Reconn"},
	{PeerSyncing, "var(--accent)", "Syncing"},
	{PeerIdle, "var(--text-muted)", "Idle"},
}

// writeDonut renders an SVG donut ring chart showing peer status distribution.
// counts is indexed by PeerStatus. total is the total peer count.
func writeDonut(w io.Writer, counts [5]int, total int) {
	h := &htmlWriter{w: w}

	// SVG parameters: viewBox 0 0 120 120, circle at center (60,60), radius 50.
	// Circumference = 2 * pi * 50 ≈ 314.159.
	const (
		cx     = 60
		cy     = 60
		radius = 50
		circ   = 314.159
	)

	h.write(`<div class="donut-container"><svg class="donut" viewBox="0 0 120 120">`)

	// Background ring (dim border color).
	h.writef(`<circle cx="%d" cy="%d" r="%d" fill="none" stroke="var(--border)" stroke-width="10"/>`, cx, cy, radius)

	if total > 0 {
		offset := 0.0 // cumulative offset in dasharray units
		for _, entry := range donutStatusOrder {
			count := counts[entry.Status]
			if count == 0 {
				continue
			}
			segLen := float64(count) / float64(total) * circ
			// stroke-dasharray: segment gap; stroke-dashoffset rotates start position.
			// Offset is negative to rotate clockwise from 12 o'clock (-90deg start via rotation).
			h.writef(`<circle cx="%d" cy="%d" r="%d" fill="none" stroke="%s" stroke-width="10" stroke-dasharray="%.2f %.2f" stroke-dashoffset="%.2f" transform="rotate(-90 %d %d)"/>`,
				cx, cy, radius, entry.Color, segLen, circ-segLen, -offset, cx, cy)
			offset += segLen
		}
	}

	// Center text showing total count.
	h.writef(`<text x="%d" y="%d" text-anchor="middle" dominant-baseline="central" class="donut-center">%d</text>`,
		cx, cy, total)

	h.write(`</svg>`)
}

// writeDonutLegend renders the legend to the right of the donut SVG.
// Must be called between writeDonut and writeDonutEnd.
func writeDonutLegend(w io.Writer, counts [5]int) {
	h := &htmlWriter{w: w}
	h.write(`<div class="donut-legend">`)
	for _, entry := range donutStatusOrder {
		h.writef(`<span class="donut-legend-item"><span class="donut-legend-dot" style="background:%s"></span>%s <span class="donut-legend-count">%d</span></span>`,
			entry.Color, entry.Label, counts[entry.Status])
	}
	h.write(`</div>`)
}

// writeDonutEnd closes the donut-container div.
func writeDonutEnd(w io.Writer) {
	h := &htmlWriter{w: w}
	h.write(`</div>`)
}

// writePeerDetail renders the detail pane for a single peer.
// allFamilies is the sorted list of all families seen across all peers.
func writePeerDetail(w io.Writer, ps *PeerState, pinned bool, allFamilies []string) {
	h := &htmlWriter{w: w}
	pinLabel := "Pin"
	if pinned {
		pinLabel = "Unpin"
	}

	h.writef(`<div class="detail-pane" id="peer-detail">
<h2>
  <span>Peer %d</span>
  <span>
    <span class="badge" hx-post="/peers/%d/pin" hx-swap="none">%s</span>
    <span class="close-btn" hx-get="/peer/close" hx-target="#peer-detail" hx-swap="outerHTML">&times;</span>
  </span>
</h2>`, ps.Index, ps.Index, pinLabel)

	h.writef(`
<div class="detail-section">
  <h4>State</h4>
  <div class="detail-grid">
    <div class="detail-item"><span class="label">Status: </span><span class="value"><span class="dot %s"></span>%s</span></div>
    <div class="detail-item"><span class="label">Msgs Sent: </span><span class="value">%d</span></div>
    <div class="detail-item"><span class="label">Msgs Recv: </span><span class="value">%d</span></div>
    <div class="detail-item"><span class="label">Missing: </span><span class="value">%d</span></div>
    <div class="detail-item"><span class="label">Bytes Sent: </span><span class="value">%s</span></div>
    <div class="detail-item"><span class="label">Bytes Recv: </span><span class="value">%s</span></div>
    <div class="detail-item"><span class="label">Rate Out: </span><span class="value">%s</span></div>
    <div class="detail-item"><span class="label">Rate In: </span><span class="value">%s</span></div>
    <div class="detail-item"><span class="label">Chaos: </span><span class="value">%d</span></div>
    <div class="detail-item"><span class="label">Reconnects: </span><span class="value">%d</span></div>
  </div>
</div>`, ps.Status.CSSClass(), ps.Status.String(),
		ps.RoutesSent, ps.RoutesRecv, ps.Missing,
		FormatBytes(ps.BytesSent), FormatBytes(ps.BytesRecv),
		FormatBitRate(ps.throughputOut), FormatBitRate(ps.throughputIn),
		ps.ChaosCount, ps.Reconnects)

	// Per-family route breakdown.
	if len(allFamilies) > 0 {
		// Build set of negotiated families for O(1) lookup.
		negotiated := make(map[string]bool, len(ps.Families))
		for _, f := range ps.Families {
			negotiated[f] = true
		}

		h.write(`
<div class="detail-section">
  <h4>Families</h4>
  <table class="family-table">
    <tr><th>Family</th><th></th><th>Sent</th><th>Recv</th></tr>`)
		for _, fam := range allFamilies {
			sent := ps.FamilySent[fam]
			recv := ps.FamilyRecv[fam]
			if negotiated[fam] {
				h.writef(`<tr><td>%s</td><td class="family-check">&#x2713;</td><td>%d</td><td>%d</td></tr>`,
					escapeHTML(fam), sent, recv)
			} else {
				h.writef(`<tr class="family-disabled"><td>%s</td><td class="family-cross">&#x2717;</td><td>-</td><td>-</td></tr>`,
					escapeHTML(fam))
			}
		}
		h.write(`
  </table>
</div>`)
	}

	// Recent events for this peer.
	h.write(`
<div class="detail-section">
  <h4>Recent Events</h4>
  <div class="event-list">`)

	events := ps.Events.All()
	// Show most recent first.
	for i := len(events) - 1; i >= 0; i-- {
		ev := events[i]
		evClass := eventTypeClass(ev.Type)
		elapsed := FormatElapsed(time.Since(ev.Time))
		label := eventTypeLabel(ev.Type)
		detail := eventDetail(ev)
		h.writef(`<div class="event-row"><span class="event-time">%s ago</span><span class="event-type %s">%s</span><span>%s</span></div>`,
			elapsed, evClass, label, detail)
	}

	h.write(`
  </div>
</div>
</div>`)
}

// writeRecentEvents renders the global recent events list.
func writeRecentEvents(w io.Writer, s *DashboardState) {
	h := &htmlWriter{w: w}
	events := s.GlobalEvents.All()
	// Show most recent first, limit to last 30.
	start := 0
	if len(events) > 30 {
		start = len(events) - 30
	}
	for i := len(events) - 1; i >= start; i-- {
		ev := events[i]
		evClass := eventTypeClass(ev.Type)
		elapsed := FormatElapsed(time.Since(ev.Time))
		label := eventTypeLabel(ev.Type)
		h.writef(`<div class="event-row"><span class="event-time">%s</span><span class="event-type %s">p%d %s</span></div>`,
			elapsed, evClass, ev.PeerIndex, label)
	}
}

// eventTypeClass returns the CSS class for an event type.
func eventTypeClass(t peer.EventType) string {
	switch t {
	case peer.EventEstablished:
		return "event-type-established"
	case peer.EventDisconnected, peer.EventError:
		return "event-type-disconnected"
	case peer.EventChaosExecuted, peer.EventReconnecting:
		return "event-type-chaos"
	case peer.EventRouteSent, peer.EventRouteReceived, peer.EventRouteWithdrawn,
		peer.EventEORSent, peer.EventWithdrawalSent:
		return "event-type-route"
	case peer.EventRouteAction:
		return "event-type-chaos"
	}
	return ""
}

// eventTypeLabel returns a short human-readable label for an event type.
func eventTypeLabel(t peer.EventType) string {
	switch t {
	case peer.EventEstablished:
		return "established"
	case peer.EventDisconnected:
		return "disconnected"
	case peer.EventError:
		return "error"
	case peer.EventChaosExecuted:
		return "chaos"
	case peer.EventReconnecting:
		return "reconnecting"
	case peer.EventRouteSent:
		return "route-sent"
	case peer.EventRouteReceived:
		return "route-recv"
	case peer.EventRouteWithdrawn:
		return "route-withdrawn"
	case peer.EventEORSent:
		return "eor"
	case peer.EventWithdrawalSent:
		return "withdrawal-sent"
	case peer.EventRouteAction:
		return "route-action"
	}
	return "unknown"
}

// eventDetail returns extra detail text for an event (prefix, error, count).
func eventDetail(ev peer.Event) string {
	switch ev.Type {
	case peer.EventRouteSent, peer.EventRouteReceived, peer.EventRouteWithdrawn:
		if ev.Prefix.IsValid() {
			return ev.Prefix.String()
		}
	case peer.EventError:
		if ev.Err != nil {
			return escapeHTML(ev.Err.Error())
		}
	case peer.EventChaosExecuted:
		return escapeHTML(ev.ChaosAction)
	case peer.EventWithdrawalSent, peer.EventEORSent:
		if ev.Count > 0 {
			return itoa(ev.Count)
		}
	case peer.EventRouteAction:
		return escapeHTML(ev.ChaosAction)
	case peer.EventEstablished, peer.EventDisconnected, peer.EventReconnecting:
		// No extra detail.
	}
	return ""
}
