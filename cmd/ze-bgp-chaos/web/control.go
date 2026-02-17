package web

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const statusStopped = "stopped"

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
		d.state.Control.Status = "paused"
		d.state.mu.Unlock()
		d.logControl("pause", "")
	default:
		http.Error(w, "busy", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	d.state.RLock()
	defer d.state.RUnlock()
	writeControlPanel(w, &d.state.Control)
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
		d.state.Control.Status = "running"
		d.state.mu.Unlock()
		d.logControl("resume", "")
	default:
		http.Error(w, "busy", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	d.state.RLock()
	defer d.state.RUnlock()
	writeControlPanel(w, &d.state.Control)
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
			d.state.Control.Status = "paused"
		} else {
			d.state.Control.Paused = false
			d.state.Control.Status = "running"
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
	writeControlPanel(w, &d.state.Control)
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
	_, _ = fmt.Fprintf(w, `<div id="trigger-result" class="event-row"><span class="event-type event-type-chaos">triggered</span><span>%s on %s</span></div>`,
		escapeHTML(actionType), targetDesc)
}

// handleControlStop handles POST /control/stop.
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
	writeControlPanel(w, &d.state.Control)
}

// handleControlTriggerForm serves the parameter form for a specific action type.
func (d *Dashboard) handleControlTriggerForm(w http.ResponseWriter, r *http.Request) {
	actionType := r.URL.Query().Get("action")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	writeTriggerForm(w, actionType)
}

// SetPropertyResults updates the dashboard's property badge display.
// Called by the orchestrator when property results change.
func (d *Dashboard) SetPropertyResults(results []PropertyBadge) {
	d.state.mu.Lock()
	d.state.Properties = results
	d.state.dirtyGlobal = true
	d.state.mu.Unlock()
}

// ControlChannel returns the control command channel, or nil if not configured.
func (d *Dashboard) ControlChannel() <-chan ControlCommand {
	return d.control
}

// writeControlPanel renders the control panel HTML fragment.
func writeControlPanel(w io.Writer, cs *ControlState) {
	statusClass := "status-up"
	statusLabel := "Running"
	if cs.Paused {
		statusClass = "status-reconnecting"
		statusLabel = "Paused"
	}
	if cs.Status == statusStopped {
		statusClass = "status-down"
		statusLabel = "Stopped"
	}

	_, _ = fmt.Fprintf(w, `<div id="control-panel" class="card">
<h3>Controls</h3>
<div class="control-row">
  <span class="dot %s"></span> <span>%s</span>
</div>`, statusClass, statusLabel)

	if cs.Status != statusStopped {
		if cs.Paused {
			_, _ = fmt.Fprint(w, `
<div class="control-row">
  <span class="badge" hx-post="/control/resume" hx-target="#control-panel" hx-swap="outerHTML">Resume</span>
  <span class="badge" hx-post="/control/stop" hx-target="#control-panel" hx-swap="outerHTML">Stop</span>
</div>`)
		} else {
			_, _ = fmt.Fprint(w, `
<div class="control-row">
  <span class="badge" hx-post="/control/pause" hx-target="#control-panel" hx-swap="outerHTML">Pause</span>
  <span class="badge" hx-post="/control/stop" hx-target="#control-panel" hx-swap="outerHTML">Stop</span>
</div>`)
		}

		// Rate slider.
		_, _ = fmt.Fprintf(w, `
<div class="control-row">
  <label class="stat-label">Rate: </label>
  <input type="range" min="0" max="100" value="%d" class="rate-slider"
         hx-post="/control/rate" hx-target="#control-panel" hx-swap="outerHTML"
         hx-trigger="change" name="rate"
         hx-vals='js:{rate: (parseFloat(event.target.value)/100).toFixed(2)}'>
  <span class="stat-value">%.0f%%</span>
</div>`, int(cs.Rate*100), cs.Rate*100)

		// Trigger dropdown.
		_, _ = fmt.Fprint(w, `
<div class="control-row">
  <select name="action" hx-get="/control/trigger-form" hx-target="#trigger-params" hx-swap="innerHTML"
          hx-trigger="change" hx-include="this">
    <option value="">Trigger...</option>`)
		for _, at := range chaosActionTypes() {
			_, _ = fmt.Fprintf(w, `<option value="%s">%s</option>`, at, at)
		}
		_, _ = fmt.Fprint(w, `
  </select>
</div>
<div id="trigger-params"></div>
<div id="trigger-result"></div>`)
	}

	_, _ = fmt.Fprint(w, `
</div>`)
}

// writeTriggerForm renders the parameter form for a specific action type.
func writeTriggerForm(w io.Writer, actionType string) {
	if actionType == "" {
		return
	}

	_, _ = fmt.Fprint(w, `<div class="control-row">`)

	// Peer selection.
	_, _ = fmt.Fprint(w, `<label class="stat-label">Peers: </label>
<input type="text" name="peers" placeholder="all (or 0,3,7)" class="control-input">`)

	// Action-specific parameters.
	if actionType == "partial-withdraw" {
		_, _ = fmt.Fprint(w, `
<label class="stat-label">Fraction: </label>
<input type="number" name="fraction" value="0.3" step="0.1" min="0.1" max="1.0" class="control-input">`)
	}

	_, _ = fmt.Fprintf(w, `
<span class="badge" hx-post="/control/trigger" hx-target="#trigger-result" hx-swap="outerHTML"
      hx-include="[name='action'],[name='peers'],[name='fraction']"
      hx-vals='{"action":"%s"}'>Execute</span>`, actionType)
	_, _ = fmt.Fprint(w, `</div>`)
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
func chaosActionTypes() []string {
	return []string{
		"tcp-disconnect",
		"notification-cease",
		"hold-timer-expiry",
		"partial-withdraw",
		"full-withdraw",
		"disconnect-during-burst",
		"reconnect-storm",
		"connection-collision",
		"malformed-update",
		"config-reload",
	}
}
