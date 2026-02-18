package web

import (
	"fmt"
	"html"
	"io"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-chaos/peer"
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
<title>Ze BGP Chaos</title>
<link rel="stylesheet" href="/assets/style.css">
<script src="/assets/htmx.min.js"></script>
<script src="/assets/sse.js"></script>
</head>
<body>
<div class="layout" hx-ext="sse" sse-connect="/events">

<div class="header">
  <h1>Ze BGP Chaos</h1>
  <span class="run-info">peers: ` + itoa(s.PeerCount) + ` | uptime: ` + uptime + `</span>
</div>

<div class="content">
<div class="sidebar">
  <div class="card">
    <h3>Stats</h3>
    <div id="stats" sse-swap="stats" hx-swap="outerHTML">
      <span class="stat" title="BGP sessions currently established / total configured"><span class="stat-label">Peers </span><span class="stat-value">` + itoa(s.PeersUp) + `/` + itoa(s.PeerCount) + `</span></span>
      <span class="stat" title="Total routes announced to peers"><span class="stat-label">Announced </span><span class="stat-value">` + itoa(s.TotalAnnounced) + `</span></span>
      <span class="stat" title="Total routes received from peers"><span class="stat-label">Received </span><span class="stat-value">` + itoa(s.TotalReceived) + `</span></span>
      <span class="stat" title="Total routes withdrawn by peers"><span class="stat-label">Withdrawn </span><span class="stat-value">` + itoa(s.TotalWithdrawn) + `</span></span>
      <span class="stat" title="Total withdrawal messages sent to peers"><span class="stat-label">Wdraw Sent </span><span class="stat-value">` + itoa(s.TotalWdrawSent) + `</span></span>
      <span class="stat" title="Total chaos actions executed (disconnects, route drops, etc.)"><span class="stat-label">Chaos </span><span class="stat-value">` + itoa(s.TotalChaos) + `</span></span>
      <span class="stat" title="Total peer reconnections after chaos events"><span class="stat-label">Reconnects </span><span class="stat-value">` + itoa(s.TotalReconnects) + `</span></span>
    </div>
  </div>

  <div class="card">
    <h3 title="The table shows only the most active peers. Peers rotate in/out based on activity.">Active Set</h3>
    <div id="active-set-info">
      <span class="stat" title="Peers currently shown in the table / maximum visible"><span class="stat-label">Visible </span><span class="stat-value">` + itoa(s.Active.Len()) + `/` + itoa(s.Active.MaxVisible) + `</span></span>
      <span class="stat" title="Time before an inactive peer is removed from the table"><span class="stat-label">TTL </span><span class="stat-value">` + FormatDuration(s.Active.AdaptiveTTL()) + `</span></span>
    </div>
  </div>

  <div class="card">
    <h3 title="Manually add a peer to the table by entering its index number">Peer Picker</h3>
    <div class="control-row">
      <input type="number" id="promote-id" name="id" min="0" max="` + itoa(s.PeerCount-1) + `" placeholder="peer #" class="control-input"
             title="Enter a peer index (0 to ` + itoa(s.PeerCount-1) + `) to add it to the table">
      <span class="badge" hx-post="/peers/promote" hx-target="#peer-tbody" hx-swap="outerHTML"
            hx-include="#promote-id" title="Add this peer to the visible table">Add</span>
    </div>
  </div>`)

	// Control panel (only when control channel is configured).
	if s.Control.Status != "" {
		writeControlPanel(w, &s.Control)
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
    <div id="events" class="event-list" sse-swap="events" hx-swap="outerHTML">`)

	writeRecentEvents(w, s)

	h.write(`
    </div>
  </div>
</div>

<div class="main">
  <div class="filters">
    <label>Status:</label>
    <select hx-get="/peers" hx-target="#peer-tbody" hx-swap="outerHTML" name="status"
            hx-include="[name='sort'],[name='dir']">
      <option value="">All</option>
      <option value="up">Up</option>
      <option value="down">Down</option>
      <option value="reconnecting">Reconnecting</option>
      <option value="idle">Idle</option>
    </select>
  </div>

  <div class="peer-table-container">
    <table class="peer-table">
      <thead>
        <tr>
          <th style="width:30px" title="Pin a peer to keep it visible in the table"></th>
          <th hx-get="/peers" hx-target="#peer-tbody" hx-swap="outerHTML"
              hx-vals='{"sort":"id","dir":"asc"}' title="Peer index — click to sort">ID</th>
          <th hx-get="/peers" hx-target="#peer-tbody" hx-swap="outerHTML"
              hx-vals='{"sort":"status","dir":"asc"}' title="BGP session state — click to sort">Status</th>
          <th hx-get="/peers" hx-target="#peer-tbody" hx-swap="outerHTML"
              hx-vals='{"sort":"sent","dir":"desc"}' title="Routes announced to this peer — click to sort">Sent</th>
          <th hx-get="/peers" hx-target="#peer-tbody" hx-swap="outerHTML"
              hx-vals='{"sort":"received","dir":"desc"}' title="Routes received from this peer — click to sort">Recv</th>
          <th hx-get="/peers" hx-target="#peer-tbody" hx-swap="outerHTML"
              hx-vals='{"sort":"chaos","dir":"desc"}' title="Chaos events targeting this peer — click to sort">Chaos</th>
        </tr>
      </thead>
      <tbody id="peer-tbody">`)

	indices := s.Active.Indices()
	sortPeers(indices, s, "id", "asc")
	writePeerRows(w, s, indices)

	h.write(`
      </tbody>
    </table>
  </div>

  <div id="peer-detail"></div>

  <div id="peer-swap" sse-swap="peer-update" hx-swap="innerHTML" style="display:none"></div>
  <div id="peer-remove-swap" sse-swap="peer-remove" hx-swap="innerHTML" style="display:none"></div>

  <div class="tab-bar">
    <button class="active" hx-get="/viz/events" hx-target="#viz-content" hx-swap="innerHTML" hx-trigger="load, click"
            onclick="document.querySelectorAll('.tab-bar button').forEach(b=>b.classList.remove('active'));this.classList.add('active')"
            title="Live feed of all BGP and chaos events">Events</button>
    <button hx-get="/viz/convergence" hx-target="#viz-content" hx-swap="innerHTML"
            onclick="document.querySelectorAll('.tab-bar button').forEach(b=>b.classList.remove('active'));this.classList.add('active')"
            title="Route propagation latency distribution">Convergence</button>
    <button hx-get="/viz/peer-timeline" hx-target="#viz-content" hx-swap="innerHTML"
            onclick="document.querySelectorAll('.tab-bar button').forEach(b=>b.classList.remove('active'));this.classList.add('active')"
            title="Peer session state changes over time">Peer Timeline</button>
    <button hx-get="/viz/chaos-timeline" hx-target="#viz-content" hx-swap="innerHTML"
            onclick="document.querySelectorAll('.tab-bar button').forEach(b=>b.classList.remove('active'));this.classList.add('active')"
            title="History of chaos actions executed during the run">Chaos Timeline</button>
    <button hx-get="/viz/route-matrix" hx-target="#viz-content" hx-swap="innerHTML"
            onclick="document.querySelectorAll('.tab-bar button').forEach(b=>b.classList.remove('active'));this.classList.add('active')"
            title="Heatmap of route announce/withdraw flow between peers">Route Matrix</button>
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
</script>
</body>
</html>`)
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
		pinClass := "pin"
		if pinned {
			pinClass = "pin pinned"
		}
		h.writef(`<tr id="peer-%d" hx-get="/peer/%d" hx-target="#peer-detail" hx-swap="outerHTML">`, idx, idx)
		h.writef(`<td><span class="%s" hx-post="/peers/%d/pin" hx-swap="none" hx-trigger="click" onclick="event.stopPropagation()"></span></td>`, pinClass, idx)
		h.writef(`<td>%d</td>`, idx)
		h.writef(`<td><span class="dot %s"></span> %s</td>`, ps.Status.CSSClass(), ps.Status.String())
		h.writef(`<td>%d</td>`, ps.RoutesSent)
		h.writef(`<td>%d</td>`, ps.RoutesRecv)
		h.writef(`<td>%d</td>`, ps.ChaosCount)
		h.write(`</tr>`)
	}
}

// writePeerDetail renders the detail pane for a single peer.
func writePeerDetail(w io.Writer, ps *PeerState, pinned bool) {
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
    <div class="detail-item"><span class="label">Sent: </span><span class="value">%d</span></div>
    <div class="detail-item"><span class="label">Recv: </span><span class="value">%d</span></div>
    <div class="detail-item"><span class="label">Missing: </span><span class="value">%d</span></div>
    <div class="detail-item"><span class="label">Chaos: </span><span class="value">%d</span></div>
    <div class="detail-item"><span class="label">Reconnects: </span><span class="value">%d</span></div>
  </div>
</div>`, ps.Status.CSSClass(), ps.Status.String(),
		ps.RoutesSent, ps.RoutesRecv, ps.Missing,
		ps.ChaosCount, ps.Reconnects)

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
		elapsed := FormatDuration(time.Since(ev.Time))
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
		elapsed := FormatDuration(time.Since(ev.Time))
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
	case peer.EventEstablished, peer.EventDisconnected, peer.EventReconnecting:
		// No extra detail.
	}
	return ""
}
