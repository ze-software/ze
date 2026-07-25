// Design: plan/learned/967-ospf-13-cli-diag-interop.md -- shared read-only SSE snapshot loop.
// Related: handler_isis.go, handler_ospf.go -- the IS-IS/OSPF live views that use it.
//
// The IS-IS and OSPF web views are identical read-only streams over dispatched engine
// snapshots, differing only in the show command and event name. This is the one shared
// loop both use, so neither carries a near-duplicate copy.

package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// sseHeartbeat is the SSE comment line that keeps an idle connection alive.
var sseHeartbeat = []byte(": heartbeat\n\n")

// snapshotFetch returns the JSON payload for one SSE push, or an error to skip the tick.
type snapshotFetch func(username, remoteAddr string) (json.RawMessage, error)

// sseSnapshotStream runs the shared read-only SSE loop: it pushes an initial snapshot,
// then re-pushes every refreshInterval, emitting `event: <eventName>\ndata: <json>` and
// exiting on client disconnect (ctx.Done) so the goroutine never leaks. A fetch error is
// transient -- the stream stays open for the next tick.
func sseSnapshotStream(w http.ResponseWriter, r *http.Request, eventName string, refreshInterval time.Duration, fetch snapshotFetch) {
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
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	push := func() bool {
		payload, err := fetch(username, r.RemoteAddr)
		if err != nil {
			return true
		}
		// SSE frames are newline-delimited: a raw newline in the payload would split
		// the frame (or forge an event). Continue any embedded newline as a fresh
		// `data:` line so a multi-line (or hostile newline-padded) JSON payload stays
		// one event. Engine snapshots are compact JSON today; this is defense in depth.
		data := string(payload)
		if strings.IndexByte(data, '\n') >= 0 {
			data = strings.ReplaceAll(data, "\n", "\ndata: ")
		}
		var tb textbuf.Buffer
		frame := tb.Str("event: ").Str(eventName).Str("\ndata: ").Str(data).Str("\n\n").String()
		if _, werr := w.Write([]byte(frame)); werr != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	// Push an initial snapshot immediately so the page is not blank until the first tick.
	push()
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
			if !push() {
				return
			}
		}
	}
}
