// Design: docs/architecture/web-workbench-pages.md -- L2TP web management UI
// Related: handler_admin.go -- CommandDispatcher type reused for disconnect
// Related: sse.go -- heartbeat/flusher pattern reused for CQM SSE

//go:build ze_l2tp

package web

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/show"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errMissingSessionId     = errors.New("missing session ID")
	errSessionId0IsReserved = errors.New("session ID 0 is reserved")
)

// l2TPHandlers holds the dependencies for L2TP web UI handlers.
type l2TPHandlers struct {
	Renderer *Renderer
	Dispatch CommandDispatcher
}

// handleL2TPList returns a handler for GET /l2tp that renders the
// session/tunnel list page.
func (h *l2TPHandlers) handleL2TPList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		svc := l2tp.LookupService()
		if svc == nil {
			http.Error(w, "l2tp subsystem not running", http.StatusServiceUnavailable)
			return
		}
		snap := svc.Snapshot()

		rows := make([]l2tpSessionRow, 0, snap.SessionCount)
		for i := range snap.Tunnels {
			t := &snap.Tunnels[i]
			for j := range t.Sessions {
				s := &t.Sessions[j]
				addr := ""
				if s.AssignedAddr.IsValid() {
					addr = s.AssignedAddr.String()
				}
				rows = append(rows, l2tpSessionRow{
					LocalSID:     s.LocalSID,
					TunnelTID:    t.LocalTID,
					Username:     s.Username,
					AssignedAddr: addr,
					PeerAddr:     t.PeerAddr.String(),
					State:        s.State,
					Interface:    s.PppInterface,
					CreatedAt:    s.CreatedAt,
				})
			}
		}

		data := map[string]any{
			"Tunnels":      snap.Tunnels,
			"Sessions":     rows,
			"TunnelCount":  snap.TunnelCount,
			"SessionCount": snap.SessionCount,
			"CapturedAt":   snap.CapturedAt,
		}

		if NegotiateContentType(r) == formatJSON {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(data); err != nil {
				serverLogger.Warn("l2tp list json encode", "error", err)
			}
			return
		}

		view := l2tpListView{
			TunnelCount:  snap.TunnelCount,
			SessionCount: snap.SessionCount,
			Sessions:     rows,
		}

		if err := writeComponent(w, "l2tp list", l2tpList(view)); err != nil {
			serverLogger.Warn("l2tp list render", "error", err)
		}
	}
}

// handleL2TPDetail returns a handler for GET /l2tp/{sid} that renders
// the session detail page with chart container and event timeline.
func (h *l2TPHandlers) handleL2TPDetail() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sid, err := parseL2TPID(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		svc := l2tp.LookupService()
		if svc == nil {
			http.Error(w, "l2tp subsystem not running", http.StatusServiceUnavailable)
			return
		}
		ss, ok := svc.LookupSession(sid)
		if !ok {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}
		events := svc.SessionEvents(sid)

		enriched := map[string]any{
			colSessionID: ss.LocalSID,
			colUsername:  ss.Username,
		}
		show.Enrich("show l2tp session detail", enriched)

		body := l2tpDetailBody{
			Enriched: enriched,
			Events:   events,
			Login:    ss.Username,
			Session:  ss,
		}

		if NegotiateContentType(r) == formatJSON {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(body); err != nil {
				serverLogger.Warn("l2tp detail json encode", "error", err)
			}
			return
		}

		if err := writeComponent(w, "l2tp detail", l2tpDetail(l2tpDetailViewOf(body))); err != nil {
			serverLogger.Warn("l2tp detail render", "error", err)
		}
	}
}

// handleL2TPSamplesJSON returns a handler for GET /l2tp/{login}/samples
// that returns CQM buckets as columnar JSON matching uPlot's data shape.
func handleL2TPSamplesJSON() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		login := extractLogin(r)
		if login == "" {
			http.Error(w, "missing login", http.StatusBadRequest)
			return
		}
		svc := l2tp.LookupService()
		if svc == nil {
			http.Error(w, "l2tp subsystem not running", http.StatusServiceUnavailable)
			return
		}
		buckets := svc.LoginSamples(login)
		if buckets == nil {
			http.Error(w, "login not found", http.StatusNotFound)
			return
		}
		buckets = filterBucketsByTime(buckets, r)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(bucketsToColumnar(buckets)); err != nil {
			serverLogger.Warn("l2tp samples json encode", "error", err)
		}
	}
}

// handleL2TPSamplesCSV returns a handler for GET /l2tp/{login}/samples.csv
// that returns CQM buckets as CSV.
func handleL2TPSamplesCSV() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		login := extractLogin(r)
		if login == "" {
			http.Error(w, "missing login", http.StatusBadRequest)
			return
		}
		svc := l2tp.LookupService()
		if svc == nil {
			http.Error(w, "l2tp subsystem not running", http.StatusServiceUnavailable)
			return
		}
		buckets := svc.LoginSamples(login)
		if buckets == nil {
			http.Error(w, "login not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+login+"-cqm.csv\"")
		cw := csv.NewWriter(w)
		if err := cw.Write([]string{"timestamp", colState, "echo_count", "min_rtt_us", "avg_rtt_us", "max_rtt_us"}); err != nil {
			return
		}
		for i := range buckets {
			b := &buckets[i]
			if err := cw.Write([]string{
				strconv.FormatInt(b.Start.Unix(), 10),
				b.State.String(),
				strconv.FormatUint(uint64(b.EchoCount), 10),
				strconv.FormatInt(b.MinRTT.Microseconds(), 10),
				strconv.FormatInt(b.AvgRTT().Microseconds(), 10),
				strconv.FormatInt(b.MaxRTT.Microseconds(), 10),
			}); err != nil {
				return
			}
		}
		cw.Flush()
	}
}

// handleL2TPSamplesSSE returns the SSE route for new CQM buckets.
func handleL2TPSamplesSSE() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		login := extractLogin(r)
		if login == "" {
			http.Error(w, "missing login", http.StatusBadRequest)
			return
		}
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

		ctx := r.Context()
		ticker := time.NewTicker(l2tp.BucketInterval)
		defer ticker.Stop()
		heartbeat := time.NewTicker(30 * time.Second)
		defer heartbeat.Stop()

		var lastTS time.Time
		if svc := l2tp.LookupService(); svc != nil {
			if s := svc.LoginSamples(login); len(s) > 0 {
				lastTS = s[len(s)-1].Start
			}
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-heartbeat.C:
				if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
					return
				}
				flusher.Flush()
			case <-ticker.C:
				svc := l2tp.LookupService()
				if svc == nil {
					continue
				}
				samples := svc.LoginSamples(login)
				if len(samples) == 0 {
					continue
				}
				startIdx := len(samples)
				for i := range samples {
					if samples[i].Start.After(lastTS) {
						startIdx = i
						break
					}
				}
				if startIdx >= len(samples) {
					continue
				}
				newBuckets := samples[startIdx:]
				lastTS = newBuckets[len(newBuckets)-1].Start
				data := bucketsToColumnar(newBuckets)
				jsonBytes, err := json.Marshal(data)
				if err != nil {
					continue
				}
				if _, err := fmt.Fprintf(w, "event: bucket\ndata: %s\n\n", jsonBytes); err != nil { //nolint:errcheck // output
					return
				}
				flusher.Flush()
			}
		}
	}
}

const (
	maxReasonLen = 256
	maxCauseVal  = 65535
)

// handleL2TPDisconnect returns a handler for POST /l2tp/{sid}/disconnect
// that dispatches a session teardown through the CLI.
func (h *l2TPHandlers) handleL2TPDisconnect() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		sid, err := parseL2TPID(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 65536)
		reason := strings.TrimSpace(r.FormValue("reason"))
		if reason == "" {
			http.Error(w, "reason is required", http.StatusBadRequest)
			return
		}
		if utf8.RuneCountInString(reason) > maxReasonLen {
			var bLen textbuf.Buffer
			http.Error(w, bLen.Reset().Str("reason too long (max ").Int(int64(maxReasonLen)).Str(" chars)").String(), http.StatusBadRequest)
			return
		}
		causeStr := r.FormValue("cause")
		username := GetUsernameFromRequest(r)
		actor := username
		if actor == "" {
			actor = "web"
		}
		var bCmd textbuf.Buffer
		cmd := bCmd.Reset().Str("clear l2tp session id ").Int(int64(sid)).Byte(' ').Str("actor ").Str(actor).Str(" reason ").Str(quoteForDispatch(reason)).String()
		if causeStr != "" {
			causeVal, parseErr := strconv.ParseUint(causeStr, 10, 16)
			if parseErr != nil || causeVal > maxCauseVal {
				http.Error(w, "invalid cause code", http.StatusBadRequest)
				return
			}
			var bCause textbuf.Buffer
			cmd += bCause.Reset().Str(" cause ").Int(int64(causeVal)).String()
		}

		if h.Dispatch == nil {
			http.Error(w, "command dispatch not available", http.StatusServiceUnavailable)
			return
		}
		rendered, execErr := h.Dispatch.JSON(r.Context(), plugin.CallerIdentity{Username: username, RemoteAddr: r.RemoteAddr}, cmd)
		defer rendered.TransportComplete()
		output := rendered.Output

		if NegotiateContentType(r) == formatJSON {
			result := map[string]any{
				"command":    cmd,
				"output":     output,
				jsonKeyError: execErr != nil,
				colSessionID: int(sid),
			}
			w.Header().Set("Content-Type", "application/json")
			if execErr != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
			if err := json.NewEncoder(w).Encode(result); err != nil {
				serverLogger.Warn("l2tp disconnect json encode", "error", err)
			}
			return
		}

		if execErr != nil {
			serverLogger.Warn("l2tp disconnect failed", "sid", sid, "error", execErr)
			http.Error(w, "disconnect failed", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/l2tp", http.StatusSeeOther)
	}
}

// quoteForDispatch escapes and double-quotes a string so the CLI
// tokenizer treats it as a single token, preventing keyword injection
// (e.g. reason text containing "cause 999").
func quoteForDispatch(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// bucketsToColumnar transforms CQM buckets into the columnar array format
// that uPlot expects: {timestamps, minRTT, avgRTT, maxRTT, states}.
func bucketsToColumnar(buckets []l2tp.CQMBucket) map[string]any {
	n := len(buckets)
	timestamps := make([]int64, n)
	minRTT := make([]int64, n)
	avgRTT := make([]int64, n)
	maxRTT := make([]int64, n)
	states := make([]string, n)
	for i := range buckets {
		b := &buckets[i]
		timestamps[i] = b.Start.Unix()
		minRTT[i] = b.MinRTT.Microseconds()
		avgRTT[i] = b.AvgRTT().Microseconds()
		maxRTT[i] = b.MaxRTT.Microseconds()
		states[i] = b.State.String()
	}
	return map[string]any{
		"timestamps": timestamps,
		"minRTT":     minRTT,
		"avgRTT":     avgRTT,
		"maxRTT":     maxRTT,
		"states":     states,
	}
}

// filterBucketsByTime filters buckets using optional from/to query params (unix seconds).
func filterBucketsByTime(buckets []l2tp.CQMBucket, r *http.Request) []l2tp.CQMBucket {
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	if fromStr == "" && toStr == "" {
		return buckets
	}
	var from, to time.Time
	if fromStr != "" {
		if ts, err := strconv.ParseInt(fromStr, 10, 64); err == nil && ts >= 0 {
			from = time.Unix(ts, 0)
		}
	}
	if toStr != "" {
		if ts, err := strconv.ParseInt(toStr, 10, 64); err == nil && ts >= 0 {
			to = time.Unix(ts, 0)
		}
	}
	filtered := make([]l2tp.CQMBucket, 0, len(buckets))
	for i := range buckets {
		b := &buckets[i]
		if !from.IsZero() && b.Start.Before(from) {
			continue
		}
		if !to.IsZero() && b.Start.After(to) {
			continue
		}
		filtered = append(filtered, *b)
	}
	return filtered
}

// parseL2TPID extracts a uint16 session/tunnel ID from the URL path.
// Expected path formats: /l2tp/{id} or /l2tp/{id}/disconnect.
func parseL2TPID(r *http.Request) (uint16, error) {
	path := strings.TrimPrefix(r.URL.Path, "/l2tp/")
	path = strings.TrimSuffix(path, "/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		return 0, errMissingSessionId
	}
	n, err := strconv.ParseUint(parts[0], 10, 16)
	if err != nil {
		return 0, fmt.Errorf("invalid session ID: %w", err)
	}
	if n == 0 {
		return 0, errSessionId0IsReserved
	}
	return uint16(n), nil
}

// extractLogin extracts the login from URL paths like /l2tp/{login}/samples*.
// Rejects control characters, path separators, and characters unsafe in
// HTTP headers (quotes, semicolons, backslashes).
func extractLogin(r *http.Request) string {
	path := strings.TrimPrefix(r.URL.Path, "/l2tp/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 {
		return ""
	}
	login := parts[0]
	if login == "" || login == ".." {
		return ""
	}
	for _, c := range login {
		if c < 0x20 || c == 0x7F || c == '/' || c == '"' || c == ';' || c == '\\' {
			return ""
		}
	}
	return login
}

// l2tpDetailBody is what GET /l2tp/{sid} answers a JSON request with, and the
// value the page renders from. ONE construction feeds both, so the API body and
// the page cannot describe the same session differently.
//
// The fields are declared in alphabetical order. The map this replaced
// serialized in sorted key order, and a struct serializes in declaration order.
// The order below is what keeps the response bytes.
//
// The snapshot and the event ring go out whole. An API client keeps every field
// the subsystem published. The page takes the subset it shows, through
// l2tpDetailViewOf.
type l2tpDetailBody struct {
	Enriched map[string]any
	Events   []l2tp.ObserverEvent
	Login    string
	Session  l2tp.SessionSnapshot
}

// l2tpDetailViewOf converts the response body into the view the detail page
// renders. It is the only place the l2tp package's types meet the view models.
// That is what keeps view_l2tp.go free of the import, and its generated
// components free of the ze_l2tp build tag.
func l2tpDetailViewOf(body l2tpDetailBody) l2tpDetailView {
	ss := body.Session

	view := l2tpDetailView{
		Session: l2tpSessionView{
			LocalSID: ss.LocalSID,
			Username: ss.Username,
			State:    ss.State,
			// String() unconditionally, including for the zero address. The
			// page used to read the snapshot's netip.Addr directly, and
			// html/template printed "invalid IP" for a session with no NCP
			// assignment yet. The list page blanks it instead. That difference
			// belongs to the pages, not to the port.
			AssignedAddr:   ss.AssignedAddr.String(),
			PppInterface:   ss.PppInterface,
			TunnelLocalTID: ss.TunnelLocalTID,
			Family:         ss.Family,
		},
		Login: body.Login,
	}

	for _, e := range body.Events {
		row := l2tpEventRow{
			Timestamp: e.Timestamp,
			Type:      e.Type.String(),
			Actor:     e.Actor,
			Reason:    e.Reason,
		}

		// A zero RTT is the one the page skips, and an empty string is how the
		// view says so. Every other duration prints what String gave it.
		if e.RTT != 0 {
			row.RTT = e.RTT.String()
		}

		view.Events = append(view.Events, row)
	}

	return view
}
