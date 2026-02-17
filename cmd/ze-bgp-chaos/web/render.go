package web

import (
	"fmt"
	"html"
	"io"
	"time"

	"codeberg.org/thomas-mangin/ze/cmd/ze-bgp-chaos/peer"
)

// escapeHTML escapes HTML special characters for safe interpolation into templates.
var escapeHTML = html.EscapeString

// writeLayout renders the full HTML page for the dashboard.
func writeLayout(w io.Writer, d *Dashboard) {
	s := d.state
	uptime := FormatDuration(time.Since(s.StartTime))

	fmt.Fprint(w, `<!DOCTYPE html>
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
  <span class="run-info">peers: `+itoa(s.PeerCount)+` | uptime: `+uptime+`</span>
</div>

<div class="content">
<div class="sidebar">
  <div class="card">
    <h3>Stats</h3>
    <div id="stats" sse-swap="stats" hx-swap="outerHTML">
      <span class="stat"><span class="stat-label">Peers </span><span class="stat-value">`+itoa(s.PeersUp)+`/`+itoa(s.PeerCount)+`</span></span>
      <span class="stat"><span class="stat-label">Announced </span><span class="stat-value">`+itoa(s.TotalAnnounced)+`</span></span>
      <span class="stat"><span class="stat-label">Received </span><span class="stat-value">`+itoa(s.TotalReceived)+`</span></span>
      <span class="stat"><span class="stat-label">Withdrawn </span><span class="stat-value">`+itoa(s.TotalWithdrawn)+`</span></span>
      <span class="stat"><span class="stat-label">Chaos </span><span class="stat-value">`+itoa(s.TotalChaos)+`</span></span>
      <span class="stat"><span class="stat-label">Reconnects </span><span class="stat-value">`+itoa(s.TotalReconnects)+`</span></span>
    </div>
  </div>

  <div class="card">
    <h3>Active Set</h3>
    <div id="active-set-info">
      <span class="stat"><span class="stat-label">Visible </span><span class="stat-value">`+itoa(s.Active.Len())+`/`+itoa(s.Active.MaxVisible)+`</span></span>
      <span class="stat"><span class="stat-label">TTL </span><span class="stat-value">`+FormatDuration(s.Active.AdaptiveTTL())+`</span></span>
    </div>
  </div>

  <div class="card">
    <h3>Peer Picker</h3>
    <div class="control-row">
      <input type="number" id="promote-id" name="id" min="0" max="`+itoa(s.PeerCount-1)+`" placeholder="peer #" class="control-input">
      <span class="badge" hx-post="/peers/promote" hx-target="#peer-tbody" hx-swap="outerHTML"
            hx-include="#promote-id">Add</span>
    </div>
  </div>`)

	// Control panel (only when control channel is configured).
	if s.Control.Status != "" {
		writeControlPanel(w, &s.Control)
	}

	// Property badges.
	if len(s.Properties) > 0 {
		fmt.Fprint(w, `
  <div class="card">
    <h3>Properties</h3>`)
		writePropertyBadges(w, s.Properties)
		fmt.Fprint(w, `
  </div>`)
	}

	fmt.Fprint(w, `
  <div class="card">
    <h3>Recent Events</h3>
    <div id="events" class="event-list" sse-swap="events" hx-swap="outerHTML">`)

	writeRecentEvents(w, s)

	fmt.Fprint(w, `
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
          <th style="width:30px"></th>
          <th hx-get="/peers" hx-target="#peer-tbody" hx-swap="outerHTML"
              hx-vals='{"sort":"id","dir":"asc"}'>ID</th>
          <th hx-get="/peers" hx-target="#peer-tbody" hx-swap="outerHTML"
              hx-vals='{"sort":"status","dir":"asc"}'>Status</th>
          <th hx-get="/peers" hx-target="#peer-tbody" hx-swap="outerHTML"
              hx-vals='{"sort":"sent","dir":"desc"}'>Sent</th>
          <th hx-get="/peers" hx-target="#peer-tbody" hx-swap="outerHTML"
              hx-vals='{"sort":"received","dir":"desc"}'>Recv</th>
          <th hx-get="/peers" hx-target="#peer-tbody" hx-swap="outerHTML"
              hx-vals='{"sort":"chaos","dir":"desc"}'>Chaos</th>
        </tr>
      </thead>
      <tbody id="peer-tbody">`)

	indices := s.Active.Indices()
	sortPeers(indices, s, "id", "asc")
	writePeerRows(w, s, indices)

	fmt.Fprint(w, `
      </tbody>
    </table>
  </div>

  <div id="peer-detail"></div>

  <div class="tab-bar">
    <button class="active" hx-get="/viz/events" hx-target="#viz-content" hx-swap="outerHTML"
            onclick="document.querySelectorAll('.tab-bar button').forEach(b=>b.classList.remove('active'));this.classList.add('active')">Events</button>
    <button hx-get="/viz/convergence" hx-target="#viz-content" hx-swap="outerHTML"
            onclick="document.querySelectorAll('.tab-bar button').forEach(b=>b.classList.remove('active'));this.classList.add('active')">Convergence</button>
    <button hx-get="/viz/peer-timeline" hx-target="#viz-content" hx-swap="outerHTML"
            onclick="document.querySelectorAll('.tab-bar button').forEach(b=>b.classList.remove('active'));this.classList.add('active')">Peer Timeline</button>
    <button hx-get="/viz/chaos-timeline" hx-target="#viz-content" hx-swap="outerHTML"
            onclick="document.querySelectorAll('.tab-bar button').forEach(b=>b.classList.remove('active'));this.classList.add('active')">Chaos Timeline</button>
    <button hx-get="/viz/route-matrix" hx-target="#viz-content" hx-swap="outerHTML"
            onclick="document.querySelectorAll('.tab-bar button').forEach(b=>b.classList.remove('active'));this.classList.add('active')">Route Matrix</button>
  </div>
  <div id="viz-content"></div>
</div>

</div>
</div>
</body>
</html>`)
}

// writePeerRows renders table rows for the given peer indices.
func writePeerRows(w io.Writer, state *DashboardState, indices []int) {
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
		fmt.Fprintf(w, `<tr id="peer-%d" hx-get="/peer/%d" hx-target="#peer-detail" hx-swap="outerHTML">`, idx, idx)
		fmt.Fprintf(w, `<td><span class="%s" hx-post="/peers/%d/pin" hx-swap="none" hx-trigger="click" onclick="event.stopPropagation()"></span></td>`, pinClass, idx)
		fmt.Fprintf(w, `<td>%d</td>`, idx)
		fmt.Fprintf(w, `<td><span class="dot %s"></span> %s</td>`, ps.Status.CSSClass(), ps.Status.String())
		fmt.Fprintf(w, `<td>%d</td>`, ps.RoutesSent)
		fmt.Fprintf(w, `<td>%d</td>`, ps.RoutesRecv)
		fmt.Fprintf(w, `<td>%d</td>`, ps.ChaosCount)
		fmt.Fprint(w, `</tr>`)
	}
}

// writePeerDetail renders the detail pane for a single peer.
func writePeerDetail(w io.Writer, ps *PeerState, pinned bool) {
	pinLabel := "Pin"
	if pinned {
		pinLabel = "Unpin"
	}

	fmt.Fprintf(w, `<div class="detail-pane" id="peer-detail">
<h2>
  <span>Peer %d</span>
  <span>
    <span class="badge" hx-post="/peers/%d/pin" hx-swap="none">%s</span>
    <span class="close-btn" hx-get="/peer/close" hx-target="#peer-detail" hx-swap="outerHTML">&times;</span>
  </span>
</h2>`, ps.Index, ps.Index, pinLabel)

	fmt.Fprintf(w, `
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
	fmt.Fprint(w, `
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
		fmt.Fprintf(w, `<div class="event-row"><span class="event-time">%s ago</span><span class="event-type %s">%s</span><span>%s</span></div>`,
			elapsed, evClass, label, detail)
	}

	fmt.Fprint(w, `
  </div>
</div>
</div>`)
}

// writeRecentEvents renders the global recent events list.
func writeRecentEvents(w io.Writer, s *DashboardState) {
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
		fmt.Fprintf(w, `<div class="event-row"><span class="event-time">%s</span><span class="event-type %s">p%d %s</span></div>`,
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
