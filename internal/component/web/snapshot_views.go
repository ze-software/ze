// Design: docs/architecture/ospf/ospf-13-cli-diag-interop.md -- shared read-only protocol live views.
// Related: handler_isis.go, handler_ospf.go -- the IS-IS/OSPF adapters that wrap this.
//
// The IS-IS and OSPF web surfaces are the same read-only neighbor + database live views
// over dispatched engine snapshots, differing only in the show command, page title,
// stream path, and the <pre> element id. This is the one generic implementation both
// adapters configure, so neither carries a near-duplicate copy.

package web

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
)

// viewSpec describes one read-only live view (neighbor or database) of a protocol.
type viewSpec struct {
	command    string // the show command dispatched to the engine
	title      string // page heading
	streamPath string // SSE endpoint the HTML shell subscribes to
	eventName  string // SSE event name (also the <pre> refresh target)
}

// snapshotHandlers serves the JSON/HTML/SSE live views for one protocol. The adapters
// supply the dispatcher, the per-protocol naming, and the two viewSpecs; the views carry
// no engine state of their own (they are read-only over dispatched snapshots).
type snapshotHandlers struct {
	dispatch       CommandDispatcher
	renderer       *Renderer
	errNoDispatch  error
	unavailableMsg string // "isis engine unavailable" -- the 503 body
	jsonWarnMsg    string // "isis view json write" -- the write-error log message
	dataID         string // "isis-data" / "ospf-data" -- the <pre> element id
	refresh        time.Duration
}

// dispatchJSON runs a show command through the dispatcher and returns the raw JSON,
// degrading gracefully (error) rather than panicking when the engine is unavailable.
func (h *snapshotHandlers) dispatchJSON(ctx context.Context, command, username, remoteAddr string) (json.RawMessage, error) {
	if h.dispatch == nil {
		return nil, h.errNoDispatch
	}
	rendered, err := h.dispatch.JSON(ctx, plugin.CallerIdentity{Username: username, RemoteAddr: remoteAddr}, command)
	defer rendered.TransportComplete()
	if err != nil {
		return nil, err
	}
	if rendered.Output == "" {
		return json.RawMessage("null"), nil
	}
	if json.Valid([]byte(rendered.Output)) {
		return json.RawMessage(rendered.Output), nil
	}
	wrapped, _ := json.Marshal(map[string]string{"raw": rendered.Output})
	return wrapped, nil
}

// handleView renders one view: JSON content negotiation returns the live snapshot; HTML
// returns the page shell that subscribes to the SSE stream for live updates.
func (h *snapshotHandlers) handleView(v viewSpec) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := GetUsernameFromRequest(r)
		payload, err := h.dispatchJSON(r.Context(), v.command, username, r.RemoteAddr)
		if err != nil {
			http.Error(w, h.unavailableMsg, http.StatusServiceUnavailable)
			return
		}
		if NegotiateContentType(r) == formatJSON {
			w.Header().Set("Content-Type", "application/json")
			if _, werr := w.Write(payload); werr != nil {
				serverLogger.Warn(h.jsonWarnMsg, "error", werr)
			}
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		page := snapshotPageHTML(h.renderer, v.title, v.streamPath, v.eventName, string(payload), h.dataID)
		if _, werr := w.Write([]byte(page)); werr != nil {
			return
		}
	}
}

// sse streams a refreshed snapshot for one view over the shared SSE loop, which exits on
// client disconnect so the goroutine never leaks.
func (h *snapshotHandlers) sse(v viewSpec) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sseSnapshotStream(w, r, v.eventName, h.refresh, func(ctx context.Context, username, remoteAddr string) (json.RawMessage, error) {
			return h.dispatchJSON(ctx, v.command, username, remoteAddr)
		})
	}
}
