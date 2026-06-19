// Design: plan/spec-isis-13-cli-diag-interop.md -- IS-IS web neighbor + database views.
// Related: handler_l2tp.go -- the dispatcher + SSE ticker pattern reused here
// Related: handler_admin.go -- CommandDispatcher type
//
// The IS-IS engine runs as a managed plugin subprocess, so the web layer reaches
// it the same way the CLI does: through the CommandDispatcher, which forwards
// `show isis neighbor` / `show isis database` to the engine and returns the JSON
// the cmd_show.go proxy relayed. These handlers render that JSON as a live page
// and push refreshed snapshots over SSE on a per-connection ticker (the L2TP CQM
// pattern), closing the stream when the client disconnects (no goroutine leak).

package web

import (
	"encoding/json"
	"net/http"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

// isisRefreshInterval is how often the IS-IS SSE stream re-fetches and pushes a
// fresh neighbor/database snapshot. A few seconds is responsive without
// hammering the engine dispatcher.
const isisRefreshInterval = 3 * time.Second

// ISISHandlers holds the dependencies for the IS-IS web UI handlers. Dispatch is
// the same CommandDispatcher the CLI/admin surfaces use; the IS-IS pages carry no
// engine state of their own (they are a read-only view over dispatched snapshots).
type ISISHandlers struct {
	Renderer *Renderer
	Dispatch CommandDispatcher
}

// isisDispatchJSON runs an IS-IS show command through the dispatcher and returns
// the raw JSON output. It returns an error when the dispatcher is unavailable or
// the command fails, so the page degrades gracefully rather than panicking.
func (h *ISISHandlers) isisDispatchJSON(command, username, remoteAddr string) (json.RawMessage, error) {
	if h.Dispatch == nil {
		return nil, errISISDispatchUnavailable
	}
	out, err := h.Dispatch(command, username, remoteAddr)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return json.RawMessage("null"), nil
	}
	// The dispatcher returns the engine response as JSON text; pass it through
	// verbatim. A non-JSON payload is wrapped so the caller still gets valid JSON.
	if json.Valid([]byte(out)) {
		return json.RawMessage(out), nil
	}
	wrapped, _ := json.Marshal(map[string]string{"raw": out})
	return wrapped, nil
}

// HandleISISNeighbors returns a handler for GET /isis (and /isis/neighbors) that
// renders the IS-IS adjacency view. JSON content negotiation returns the live
// neighbor snapshot; HTML returns the page shell that subscribes to the SSE
// stream for live updates.
func (h *ISISHandlers) HandleISISNeighbors() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := GetUsernameFromRequest(r)
		payload, err := h.isisDispatchJSON("show isis neighbor", username, r.RemoteAddr)
		if err != nil {
			http.Error(w, "isis engine unavailable", http.StatusServiceUnavailable)
			return
		}
		if NegotiateContentType(r) == formatJSON {
			w.Header().Set("Content-Type", "application/json")
			if _, werr := w.Write(payload); werr != nil {
				serverLogger.Warn("isis neighbors json write", "error", werr)
			}
			return
		}
		h.writeISISPage(w, "IS-IS Neighbors", "/isis/neighbors/stream", "neighbors", payload)
	}
}

// HandleISISDatabase returns a handler for GET /isis/database that renders the
// IS-IS link-state database view, with JSON negotiation and an SSE shell like
// HandleISISNeighbors.
func (h *ISISHandlers) HandleISISDatabase() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := GetUsernameFromRequest(r)
		payload, err := h.isisDispatchJSON("show isis database", username, r.RemoteAddr)
		if err != nil {
			http.Error(w, "isis engine unavailable", http.StatusServiceUnavailable)
			return
		}
		if NegotiateContentType(r) == formatJSON {
			w.Header().Set("Content-Type", "application/json")
			if _, werr := w.Write(payload); werr != nil {
				serverLogger.Warn("isis database json write", "error", werr)
			}
			return
		}
		h.writeISISPage(w, "IS-IS Database", "/isis/database/stream", "database", payload)
	}
}

// HandleISISNeighborsSSE returns a handler for GET /isis/neighbors/stream that
// pushes a refreshed neighbor snapshot as an SSE event on a per-connection
// ticker, closing when the client disconnects.
func (h *ISISHandlers) HandleISISNeighborsSSE() http.HandlerFunc {
	return h.isisSSE("show isis neighbor", "neighbors")
}

// HandleISISDatabaseSSE returns a handler for GET /isis/database/stream that
// pushes a refreshed database snapshot as an SSE event on a per-connection
// ticker.
func (h *ISISHandlers) HandleISISDatabaseSSE() http.HandlerFunc {
	return h.isisSSE("show isis database", "database")
}

// isisSSE is the shared SSE loop: it re-dispatches command every
// isisRefreshInterval and emits the JSON as an SSE event named eventName,
// flushing each push and exiting on client disconnect (ctx.Done) so the
// goroutine never leaks (R-5).
func (h *ISISHandlers) isisSSE(command, eventName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		flusher.Flush()

		username := GetUsernameFromRequest(r)
		ctx := r.Context()
		ticker := time.NewTicker(isisRefreshInterval)
		defer ticker.Stop()
		heartbeat := time.NewTicker(30 * time.Second)
		defer heartbeat.Stop()

		// Push an initial snapshot immediately so the page is not blank until the
		// first tick.
		h.pushISISEvent(w, flusher, eventName, command, username, r.RemoteAddr)

		for {
			select {
			case <-ctx.Done():
				return
			case <-heartbeat.C:
				if _, err := w.Write(sseHeartbeat); err != nil {
					return
				}
				flusher.Flush()
			case <-ticker.C:
				if !h.pushISISEvent(w, flusher, eventName, command, username, r.RemoteAddr) {
					return
				}
			}
		}
	}
}

// sseHeartbeat is the SSE comment line that keeps an idle connection alive.
var sseHeartbeat = []byte(": heartbeat\n\n")

// pushISISEvent dispatches command and writes one SSE event built buffer-first.
// It returns false on a write error (client gone) so the caller can stop the
// stream.
func (h *ISISHandlers) pushISISEvent(w http.ResponseWriter, flusher http.Flusher, eventName, command, username, remoteAddr string) bool {
	payload, err := h.isisDispatchJSON(command, username, remoteAddr)
	if err != nil {
		return true // transient; keep the stream open for the next tick
	}
	var tb textbuf.Buffer
	frame := tb.Str("event: ").Str(eventName).Str("\ndata: ").Str(string(payload)).Str("\n\n").String()
	if _, werr := w.Write([]byte(frame)); werr != nil {
		return false
	}
	flusher.Flush()
	return true
}

// writeISISPage writes a minimal HTML shell that shows the initial snapshot and
// subscribes to the SSE stream for live updates. It embeds the snapshot JSON so
// the page renders immediately even before the first SSE push.
func (h *ISISHandlers) writeISISPage(w http.ResponseWriter, title, streamPath, eventName string, initial json.RawMessage) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	page := isisPageHTML(title, streamPath, eventName, string(initial))
	if _, err := w.Write([]byte(page)); err != nil {
		return
	}
}
