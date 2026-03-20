// Design: docs/architecture/chaos-web-dashboard.md — web dashboard UI

package web

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/chaos/engine"
)

const (
	statusPaused     = "paused"
	statusRunning    = "running"
	statusStopped    = "stopped"
	statusRestarting = "restarting"
	cssStatusUp      = "status-up"
	cssStatusDown    = "status-down"
	cssReconnecting  = "status-reconnecting"

	// Sort/label column name for chaos events.
	sortChaos = "chaos"
)

// handleControlPause handles POST /control/pause.
func (d *Dashboard) handleControlPause(w http.ResponseWriter, _ *http.Request) {
	if d.control == nil {
		http.Error(w, "no control channel", http.StatusServiceUnavailable)
		return
	}
	select {
	case d.control <- ControlCommand{Type: "pause"}:
		d.state.mu.Lock()
		d.state.Control.Paused = true
		d.state.Control.Status = statusPaused
		d.state.mu.Unlock()
		d.logControl("pause", "")
	default:
		http.Error(w, "busy", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	d.state.RLock()
	defer d.state.RUnlock()
	writeControlStrip(w, &d.state.Control)
}

// logControl writes a control event to the NDJSON log if a ControlLogger is configured.
func (d *Dashboard) logControl(command, value string) {
	if d.controlLogger != nil {
		d.controlLogger.LogControl(command, value, time.Now())
	}
}

// handleControlResume handles POST /control/resume.
func (d *Dashboard) handleControlResume(w http.ResponseWriter, _ *http.Request) {
	if d.control == nil {
		http.Error(w, "no control channel", http.StatusServiceUnavailable)
		return
	}
	select {
	case d.control <- ControlCommand{Type: "resume"}:
		d.state.mu.Lock()
		d.state.Control.Paused = false
		d.state.Control.Status = statusRunning
		d.state.mu.Unlock()
		d.logControl("resume", "")
	default:
		http.Error(w, "busy", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	d.state.RLock()
	defer d.state.RUnlock()
	writeControlStrip(w, &d.state.Control)
}

// handleControlRate handles POST /control/rate.
func (d *Dashboard) handleControlRate(w http.ResponseWriter, r *http.Request) {
	if d.control == nil {
		http.Error(w, "no control channel", http.StatusServiceUnavailable)
		return
	}
	rateStr := r.FormValue("rate")
	rate, err := strconv.ParseFloat(rateStr, 64)
	if err != nil || rate < 0 || rate > 1 {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<div id="control-error" class="event-type event-type-disconnected">invalid rate (0.0-1.0)</div>`)
		return
	}
	select {
	case d.control <- ControlCommand{Type: "rate", Rate: rate}:
		d.state.mu.Lock()
		d.state.Control.Rate = rate
		if rate == 0 {
			d.state.Control.Paused = true
			d.state.Control.Status = statusPaused
		} else {
			d.state.Control.Paused = false
			d.state.Control.Status = statusRunning
		}
		d.state.mu.Unlock()
		d.logControl("rate", fmt.Sprintf("%.2f", rate))
	default:
		http.Error(w, "busy", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	d.state.RLock()
	defer d.state.RUnlock()
	writeControlStrip(w, &d.state.Control)
}

// handleControlTrigger handles POST /control/trigger.
func (d *Dashboard) handleControlTrigger(w http.ResponseWriter, r *http.Request) {
	if d.control == nil {
		http.Error(w, "no control channel", http.StatusServiceUnavailable)
		return
	}
	actionType := r.FormValue("action")
	if actionType == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<div id="trigger-result" class="event-type event-type-disconnected">missing action type</div>`)
		return
	}

	// Parse peer list.
	var peers []int
	if peersStr := r.FormValue("peers"); peersStr != "" {
		for s := range strings.SplitSeq(peersStr, ",") {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			idx, parseErr := strconv.Atoi(s)
			if parseErr != nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = fmt.Fprintf(w, `<div id="trigger-result" class="event-type event-type-disconnected">invalid peer: %s</div>`, escapeHTML(s))
				return
			}
			peers = append(peers, idx)
		}
	}

	// Collect action-specific params.
	params := make(map[string]string)
	for k, v := range r.Form {
		if k != "action" && k != "peers" && len(v) > 0 {
			params[k] = v[0]
		}
	}

	trigger := &ManualTrigger{
		ActionType: actionType,
		Peers:      peers,
		Params:     params,
	}

	select {
	case d.control <- ControlCommand{Type: "trigger", Trigger: trigger}:
		d.logControl("trigger", actionType)
	default:
		http.Error(w, "busy", http.StatusServiceUnavailable)
		return
	}

	targetDesc := "random peer"
	if len(peers) > 0 {
		targetDesc = fmt.Sprintf("peer(s) %v", peers)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<div id="trigger-result" class="trigger-result"><span class="event-type event-type-chaos">triggered</span> %s on %s</div>`,
		escapeHTML(actionType), targetDesc)
}

// handleControlStop handles POST /control/stop.
// Note: stop only halts the chaos scheduler (via the control channel), not the
// entire run. Peers continue running so the user can observe convergence.
// This is intentionally different from restart, which cancels the run context
// via onStop() to tear down everything.
func (d *Dashboard) handleControlStop(w http.ResponseWriter, _ *http.Request) {
	if d.control == nil {
		http.Error(w, "no control channel", http.StatusServiceUnavailable)
		return
	}
	select {
	case d.control <- ControlCommand{Type: "stop"}:
		d.state.mu.Lock()
		d.state.Control.Status = statusStopped
		d.state.mu.Unlock()
		d.logControl("stop", "")
	default:
		http.Error(w, "busy", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	d.state.RLock()
	defer d.state.RUnlock()
	writeControlStrip(w, &d.state.Control)
}

// handleControlSpeed handles POST /control/speed.
func (d *Dashboard) handleControlSpeed(w http.ResponseWriter, r *http.Request) {
	d.state.mu.RLock()
	available := d.state.Control.SpeedAvailable
	d.state.mu.RUnlock()
	if !available {
		http.Error(w, "speed control not available", http.StatusServiceUnavailable)
		return
	}
	var factor int
	switch r.FormValue("factor") {
	case "1":
		factor = 1
	case "10":
		factor = 10
	case "100":
		factor = 100
	case "1000":
		factor = 1000
	default:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		h := &htmlWriter{w: w}
		h.write(`<div id="speed-error" class="event-type event-type-disconnected">invalid speed (1, 10, 100, 1000)</div>`)
		return
	}
	d.SetSpeedFactor(factor) // factor already validated by switch above
	d.logControl("speed", r.FormValue("factor"))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	d.state.RLock()
	defer d.state.RUnlock()
	writeControlStrip(w, &d.state.Control)
}

// handleControlRestart handles POST /control/restart.
// It validates the seed, sends it to the restart channel, and calls onStop
// to cancel the current run.
func (d *Dashboard) handleControlRestart(w http.ResponseWriter, r *http.Request) {
	if d.restartCh == nil {
		http.Error(w, "restart not supported", http.StatusServiceUnavailable)
		return
	}

	seed := parseRestartSeed(r.FormValue("seed"))
	if seed == 0 {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		h := &htmlWriter{w: w}
		h.write(`<div id="control-error" class="event-type event-type-disconnected">invalid seed (must be non-zero integer)</div>`)
		return
	}

	// Send restart seed (non-blocking).
	select {
	case d.restartCh <- seed:
	default:
		http.Error(w, "busy", http.StatusServiceUnavailable)
		return
	}

	// Update state to "restarting".
	d.state.mu.Lock()
	d.state.Control.Status = statusRestarting
	d.state.mu.Unlock()

	d.logControl("restart", fmt.Sprintf("%d", seed))

	// Cancel the current run.
	if d.onStop != nil {
		d.onStop()
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	d.state.RLock()
	defer d.state.RUnlock()
	writeControlStrip(w, &d.state.Control)
}

// parseRestartSeed parses a seed string, returning 0 on any error.
func parseRestartSeed(s string) uint64 {
	v, e := strconv.ParseUint(s, 10, 64)
	if e != nil {
		return 0
	}
	return v
}

// SetPropertyResults updates the dashboard's property badge display.
// Called by the orchestrator when property results change.
func (d *Dashboard) SetPropertyResults(results []PropertyBadge) {
	d.state.mu.Lock()
	d.state.Properties = results
	d.state.dirtyGlobal = true
	d.state.mu.Unlock()
}

// ControlChannel returns the chaos control command channel, or nil if not configured.
func (d *Dashboard) ControlChannel() <-chan ControlCommand {
	return d.control
}

// RouteControlChannel returns the route dynamics control command channel, or nil if not configured.
func (d *Dashboard) RouteControlChannel() <-chan ControlCommand {
	return d.routeControl
}

// writeControlStrip renders the horizontal control strip between header and content.
// Contains: status dot, pause/resume, rate slider, stop, optional speed, optional restart.
func writeControlStrip(w io.Writer, cs *ControlState) {
	statusClass := cssStatusUp
	statusLabel := "Running"
	switch {
	case cs.Status == statusRestarting:
		statusClass = cssReconnecting
		statusLabel = "Restarting..."
	case cs.Status == statusStopped:
		statusClass = cssStatusDown
		statusLabel = "Stopped"
	case cs.Paused:
		statusClass = cssReconnecting
		statusLabel = "Paused"
	}

	h := &htmlWriter{w: w}
	h.writef(`<div id="control-strip" class="control-strip"><span class="dot %s"></span><span class="strip-label">%s</span>`, statusClass, statusLabel)

	if cs.Status != statusStopped && cs.Status != statusRestarting {
		h.write(`<span class="badge btn-stop" hx-post="/control/stop" hx-target="#control-strip" hx-swap="outerHTML" title="Stop chaos">Stop</span>`)

		if cs.Paused {
			h.write(`<span class="badge btn-ctrl" hx-post="/control/resume" hx-target="#control-strip" hx-swap="outerHTML" title="Resume chaos">Resume</span>`)
		} else {
			h.write(`<span class="badge btn-ctrl" hx-post="/control/pause" hx-target="#control-strip" hx-swap="outerHTML" title="Pause chaos">Pause</span>`)
		}
	}

	// Speed buttons inline (when available).
	if cs.SpeedAvailable {
		h.write(`<span class="strip-sep"></span>`)
		for _, f := range []int{1, 10, 100, 1000} {
			active := ""
			if f == cs.SpeedFactor {
				active = ` style="border-color:#22c55e;font-weight:bold"`
			}
			h.writef(`<span class="badge"%s hx-post="/control/speed" hx-target="#control-strip" hx-swap="outerHTML" hx-vals='{"factor":"%d"}' title="%s">%dx</span>`, active, f, speedTitle(f), f)
		}
	}

	// Right-aligned group: rate + seed + restart.
	h.write(`<span style="flex:1"></span>`)
	if cs.Status != statusStopped && cs.Status != statusRestarting {
		h.writef(`<label class="stat-label">Rate</label><input type="range" min="0" max="100" value="%d" class="rate-slider" hx-post="/control/rate" hx-target="#control-strip" hx-swap="outerHTML" hx-trigger="change" name="rate" hx-vals='js:{rate: (parseFloat(event.target.value)/100).toFixed(2)}' title="Chaos event probability (0%% = paused, 100%% = max)"><span class="stat-value">%.0f%%</span>`, int(cs.Rate*100), cs.Rate*100)
		h.write(`<span class="strip-sep"></span>`)
	}
	h.writef(`<span class="strip-label">seed: %d</span>`, cs.Seed)
	if cs.RestartAvailable {
		h.write(`<input type="number" name="seed" min="1" placeholder="new" class="control-input"><span class="badge" hx-post="/control/restart" hx-target="#control-strip" hx-swap="outerHTML" hx-include="[name='seed']">New Seed</span>`)
	}

	h.write(`</div>`)
}

// writeControlSidebar renders the sidebar portion (trigger buttons + param form).
// Called when control is active but the main controls are in the strip.
func writeControlSidebar(w io.Writer, cs *ControlState) {
	if cs.Status == statusStopped || cs.Status == statusRestarting {
		return
	}
	h := &htmlWriter{w: w}
	h.write(`
  <div class="card">
    <h3>Trigger</h3>
    <label class="stat-label">Target Peers</label>`)
	writePeerPicker(h)
	writeTriggerButtons(h, chaosActionTypes())
	h.write(`
    <div id="trigger-result"></div>
  </div>`)
}

// writePeerPicker renders the tag-style peer selector.
// Shows an always-visible number input for adding peers.
// Selected peers appear as badges with [−] to remove individually.
// A hidden input name="peers" holds the comma-separated peer list for the trigger form.
func writePeerPicker(h *htmlWriter) {
	h.write(`<div class="trigger-peers" id="trigger-peers">
<input type="hidden" name="peers" id="trigger-peers-value">
<input type="number" min="0" class="tp-input control-input" id="tp-number-input" placeholder="peer #"
       onkeydown="if(event.key==='Enter'&&this.value!==''){tpAddVal(parseInt(this.value));this.value=''}">
<span class="tp-add" title="Add peer" onclick="var i=document.getElementById('tp-number-input');if(i&&i.value!==''){tpAddVal(parseInt(i.value));i.value='';i.focus()}">+</span>
</div>
<span class="tp-hint" id="tp-hint">All peers (type peer # + Enter to target specific peers)</span>`)
}

// writeTriggerButtons renders individual icon buttons for each chaos action type.
// Clicking a button fires the action immediately via hx-post.
func writeTriggerButtons(h *htmlWriter, actions []string) {
	h.write(`<div class="trigger-grid">`)
	for _, at := range actions {
		h.writef(`<span class="badge trigger-btn" title="%s" hx-post="/control/trigger" hx-target="#trigger-result" hx-swap="outerHTML" hx-include="[name='peers']" hx-vals='{"action":"%s"}'><span class="trigger-icon">%s</span> %s</span>`,
			escapeHTML(chaosActionImpact(at)), escapeJSONInAttr(at), chaosActionIcon(at), chaosActionLabel(at))
	}
	h.write(`</div>`)
}

// writePropertyBadges renders property result badges.
func writePropertyBadges(w io.Writer, badges []PropertyBadge) {
	if len(badges) == 0 {
		return
	}
	_, _ = fmt.Fprint(w, `<div id="property-badges">`)
	for _, b := range badges {
		cssClass := "badge-pass"
		label := "PASS"
		if !b.Pass {
			cssClass = "badge-fail"
			label = "FAIL"
		}
		_, _ = fmt.Fprintf(w, `<div class="property-badge %s" onclick="this.querySelector('.violations').classList.toggle('hidden')">
  <span>%s: %s</span>`, cssClass, escapeHTML(b.Name), label)
		if !b.Pass && len(b.Violations) > 0 {
			_, _ = fmt.Fprint(w, `<div class="violations hidden">`)
			for _, v := range b.Violations {
				_, _ = fmt.Fprintf(w, `<div class="violation-row">%s</div>`, escapeHTML(v))
			}
			_, _ = fmt.Fprint(w, `</div>`)
		}
		_, _ = fmt.Fprint(w, `</div>`)
	}
	_, _ = fmt.Fprint(w, `</div>`)
}

// chaosActionTypes returns all known chaos action type names.
// Route-related actions (partial-withdraw, full-withdraw) are handled by the
// route dynamics scheduler and are NOT included here.
func chaosActionTypes() []string {
	return []string{
		engine.NameTCPDisconnect,
		engine.NameNotificationCease,
		engine.NameHoldTimerExpiry,
		engine.NameDisconnectDuringBurst,
		engine.NameReconnectStorm,
		engine.NameConnectionCollision,
		engine.NameMalformedUpdate,
		engine.NameConfigReload,
		engine.NameSlowRead,
	}
}

// chaosActionImpact returns a short human-readable description of what each
// chaos action does and its impact on BGP sessions and routes.
func chaosActionImpact(action string) string {
	switch action {
	case engine.NameTCPDisconnect:
		return "Drops the TCP connection immediately. Peer loses session and all routes until reconnection."
	case engine.NameNotificationCease:
		return "Sends a BGP NOTIFICATION (Cease) then closes. Graceful shutdown \u2014 peer knows why the session ended."
	case engine.NameHoldTimerExpiry:
		return "Stops sending KEEPALIVEs. Peer detects expiry after hold-time (typically 90s). Slow disruption."
	case engine.NameDisconnectDuringBurst:
		return "Drops connection while routes are still being announced (before EOR). Ze has partial routing state."
	case engine.NameReconnectStorm:
		return "Disconnects and rapidly reconnects 2 times. Stresses session setup and route re-announcement."
	case engine.NameConnectionCollision:
		return "Opens a second TCP connection while the first is active. Tests RFC 4271 collision resolution. No route loss."
	case engine.NameMalformedUpdate:
		return "Sends an UPDATE with invalid attributes. Tests RFC 7606 error handling. Session may or may not drop."
	case engine.NameConfigReload:
		return "Sends SIGHUP to the Ze process. Triggers config re-read. Sessions stay up unless config changed."
	case engine.NameSlowRead:
		return "Toggles slow reading on this peer. TCP backpressure blocks Ze's writes, testing forward pool overflow and congestion callbacks. Click again to restore normal speed."
	default:
		return ""
	}
}

// chaosActionIcon returns a Unicode icon for a chaos action type.
func chaosActionIcon(action string) string {
	switch action {
	case engine.NameTCPDisconnect:
		return "\u26a1" // ⚡
	case engine.NameNotificationCease:
		return "\u26d4" // ⛔
	case engine.NameHoldTimerExpiry:
		return "\u23f3" // ⏳
	case engine.NameDisconnectDuringBurst:
		return "\U0001f4a5" // 💥
	case engine.NameReconnectStorm:
		return "\U0001f300" // 🌀
	case engine.NameConnectionCollision:
		return "\U0001f4a2" // 💢
	case engine.NameMalformedUpdate:
		return "\u26a0" // ⚠
	case engine.NameConfigReload:
		return "\U0001f504" // 🔄
	case engine.NameSlowRead:
		return "\U0001f422" // 🐢
	default:
		return "\u2753" // ❓
	}
}

// chaosActionLabel returns a short label for a chaos action type.
func chaosActionLabel(action string) string {
	switch action {
	case engine.NameTCPDisconnect:
		return "Disconnect"
	case engine.NameNotificationCease:
		return "Cease"
	case engine.NameHoldTimerExpiry:
		return "Hold Expire"
	case engine.NameDisconnectDuringBurst:
		return "Burst Drop"
	case engine.NameReconnectStorm:
		return "Storm"
	case engine.NameConnectionCollision:
		return "Collision"
	case engine.NameMalformedUpdate:
		return "Malformed"
	case engine.NameConfigReload:
		return "Reload"
	case engine.NameSlowRead:
		return "Slow Read"
	default:
		return action
	}
}

// handleRouteControlPause handles POST /control/route/pause.
func (d *Dashboard) handleRouteControlPause(w http.ResponseWriter, _ *http.Request) {
	if d.routeControl == nil {
		http.Error(w, "no route control channel", http.StatusServiceUnavailable)
		return
	}
	select {
	case d.routeControl <- ControlCommand{Type: "pause"}:
		d.state.mu.Lock()
		d.state.Control.RoutePaused = true
		d.state.Control.RouteStatus = statusPaused
		d.state.mu.Unlock()
		d.logControl("route-pause", "")
	default:
		http.Error(w, "busy", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	d.state.RLock()
	defer d.state.RUnlock()
	writeRouteControlPanel(w, &d.state.Control)
}

// handleRouteControlResume handles POST /control/route/resume.
func (d *Dashboard) handleRouteControlResume(w http.ResponseWriter, _ *http.Request) {
	if d.routeControl == nil {
		http.Error(w, "no route control channel", http.StatusServiceUnavailable)
		return
	}
	select {
	case d.routeControl <- ControlCommand{Type: "resume"}:
		d.state.mu.Lock()
		d.state.Control.RoutePaused = false
		d.state.Control.RouteStatus = statusRunning
		d.state.mu.Unlock()
		d.logControl("route-resume", "")
	default:
		http.Error(w, "busy", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	d.state.RLock()
	defer d.state.RUnlock()
	writeRouteControlPanel(w, &d.state.Control)
}

// handleRouteControlRate handles POST /control/route/rate.
func (d *Dashboard) handleRouteControlRate(w http.ResponseWriter, r *http.Request) {
	if d.routeControl == nil {
		http.Error(w, "no route control channel", http.StatusServiceUnavailable)
		return
	}
	rateStr := r.FormValue("rate")
	rate, err := strconv.ParseFloat(rateStr, 64)
	if err != nil || rate < 0 || rate > 1 {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		h := &htmlWriter{w: w}
		h.write(`<div id="route-control-error" class="event-type event-type-disconnected">invalid rate (0.0-1.0)</div>`)
		return
	}
	select {
	case d.routeControl <- ControlCommand{Type: "rate", Rate: rate}:
		d.state.mu.Lock()
		d.state.Control.RouteRate = rate
		if rate == 0 {
			d.state.Control.RoutePaused = true
			d.state.Control.RouteStatus = statusPaused
		} else {
			d.state.Control.RoutePaused = false
			d.state.Control.RouteStatus = statusRunning
		}
		d.state.mu.Unlock()
		d.logControl("route-rate", fmt.Sprintf("%.2f", rate))
	default:
		http.Error(w, "busy", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	d.state.RLock()
	defer d.state.RUnlock()
	writeRouteControlPanel(w, &d.state.Control)
}

// handleRouteControlStop handles POST /control/route/stop.
func (d *Dashboard) handleRouteControlStop(w http.ResponseWriter, _ *http.Request) {
	if d.routeControl == nil {
		http.Error(w, "no route control channel", http.StatusServiceUnavailable)
		return
	}
	select {
	case d.routeControl <- ControlCommand{Type: "stop"}:
		d.state.mu.Lock()
		d.state.Control.RouteStatus = statusStopped
		d.state.mu.Unlock()
		d.logControl("route-stop", "")
	default:
		http.Error(w, "busy", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	d.state.RLock()
	defer d.state.RUnlock()
	writeRouteControlPanel(w, &d.state.Control)
}

// speedTitle returns a human-readable description for a speed factor.
func speedTitle(factor int) string {
	switch factor {
	case 1:
		return "Real-time (1s/step)"
	case 10:
		return "10x (100ms/step)"
	case 100:
		return "100x (10ms/step)"
	case 1000:
		return "1000x (1ms/step)"
	default:
		return fmt.Sprintf("%dx", factor)
	}
}

// writeRouteControlPanel renders the route dynamics control panel HTML fragment.
func writeRouteControlPanel(w io.Writer, cs *ControlState) {
	h := &htmlWriter{w: w}

	statusClass := cssStatusUp
	statusLabel := "Running"
	switch {
	case cs.RouteStatus == statusStopped:
		statusClass = cssStatusDown
		statusLabel = "Stopped"
	case cs.RoutePaused:
		statusClass = cssReconnecting
		statusLabel = "Paused"
	}

	h.writef(`<div id="route-control-panel" class="card">
<h3>Route Dynamics</h3>
<div class="control-row">
  <span class="dot %s"></span> <span>%s</span>
</div>`, statusClass, statusLabel)

	if cs.RouteStatus != statusStopped {
		if cs.RoutePaused {
			h.write(`
<div class="control-row">
  <span class="badge" hx-post="/control/route/resume" hx-target="#route-control-panel" hx-swap="outerHTML" title="Resume route dynamics">Resume</span>
  <span class="badge" hx-post="/control/route/stop" hx-target="#route-control-panel" hx-swap="outerHTML" title="Stop route dynamics entirely">Stop</span>
</div>`)
		} else {
			h.write(`
<div class="control-row">
  <span class="badge" hx-post="/control/route/pause" hx-target="#route-control-panel" hx-swap="outerHTML" title="Pause route dynamics (no new route changes)">Pause</span>
  <span class="badge" hx-post="/control/route/stop" hx-target="#route-control-panel" hx-swap="outerHTML" title="Stop route dynamics entirely">Stop</span>
</div>`)
		}

		// Rate slider.
		h.writef(`
<div class="control-row">
  <label class="stat-label">Rate: </label>
  <input type="range" min="0" max="100" value="%d" class="rate-slider"
         hx-post="/control/route/rate" hx-target="#route-control-panel" hx-swap="outerHTML"
         hx-trigger="change" name="rate"
         hx-vals='js:{rate: (parseFloat(event.target.value)/100).toFixed(2)}'
         title="Adjust route dynamics frequency (0%% = no changes, 100%% = maximum rate)">
  <span class="stat-value">%.0f%%</span>
</div>`, int(cs.RouteRate*100), cs.RouteRate*100)
	}

	h.write("\n</div>")
}
