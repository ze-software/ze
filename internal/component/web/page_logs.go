// Design: plan/spec-web-8-tools-logs.md -- Log page handlers
// Related: sse.go -- EventBroker for Live Log SSE streaming
// Related: handler_admin.go -- CommandDispatcher type
// Related: workbench_pages.go -- Page dispatch (renderPageContent)
//
// Log pages display operational event data. Live Log uses SSE streaming
// through the existing EventBroker. Warnings and Errors are read-only
// tables populated by dispatching show commands.

package web

import (
	"encoding/json"
	"html/template"
	"net/http"
	"time"
)

// LogTableData is the template payload for warning/error log tables.
type LogTableData struct {
	Title        string
	Columns      []WorkbenchTableColumn
	Rows         []WorkbenchTableRow
	EmptyMessage string
	EmptyHint    string
}

// renderLogPageContent dispatches log sub-pages. The path slice has the
// leading "logs" segment already stripped. Returns (content, true) if a
// page handler matched, or ("", false) to fall through.
func renderLogPageContent(renderer *Renderer, r *http.Request, path []string, dispatch CommandDispatcher, _ *EventBroker) (template.HTML, bool) {
	if len(path) == 0 {
		// /show/logs/ defaults to live.
		return HandleLogLivePage(renderer), true
	}

	switch path[0] {
	case "live":
		return HandleLogLivePage(renderer), true
	case "warnings":
		return HandleLogWarningsPage(renderer, r, dispatch), true
	case "errors":
		return HandleLogErrorsPage(renderer, r, dispatch), true
	}

	return "", false
}

// --- Live Log ---

// HandleLogLivePage returns the rendered HTML for the Live Log toolbar and
// streaming area. The page opens an SSE connection to /logs/live/stream
// on the client side for real-time event display.
func HandleLogLivePage(renderer *Renderer) template.HTML {
	return renderer.RenderFragment("log_live", nil)
}

// HandleLogLiveStream returns an HTTP handler that streams log events via SSE.
// It delegates to the EventBroker's ServeHTTP for subscription management.
// The broker handles client limits, buffering, and heartbeats.
func HandleLogLiveStream(broker *EventBroker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if broker == nil {
			http.Error(w, "SSE not available", http.StatusServiceUnavailable)
			return
		}
		broker.ServeHTTP(w, r)
	}
}

// --- Warnings ---

// HandleLogWarningsPage returns the rendered HTML for the Warnings table.
// Dispatches "show warnings" and renders the response. With no dispatcher
// or no warnings, shows the empty state.
func HandleLogWarningsPage(renderer *Renderer, r *http.Request, dispatch CommandDispatcher) template.HTML {
	data := LogTableData{
		Title: "Warnings",
		Columns: []WorkbenchTableColumn{
			{Key: "time", Label: "Time"},
			{Key: "component", Label: "Component"},
			{Key: "message", Label: "Message"},
			{Key: "duration", Label: "Duration"},
		},
		EmptyMessage: "No active warnings. All systems operating normally.",
	}

	if dispatch != nil {
		username := GetUsernameFromRequest(r)
		output, err := dispatch("show warnings", username, r.RemoteAddr)
		if err == nil && output != "" {
			data.Rows = parseIssueJSON(output, "warnings", true)
		}
	}

	return renderer.RenderFragment("log_table", data)
}

// --- Errors ---

// HandleLogErrorsPage returns the rendered HTML for the Errors table.
// Dispatches "show errors" and renders the response. With no dispatcher
// or no errors, shows the empty state.
func HandleLogErrorsPage(renderer *Renderer, r *http.Request, dispatch CommandDispatcher) template.HTML {
	data := LogTableData{
		Title: "Errors",
		Columns: []WorkbenchTableColumn{
			{Key: "time", Label: "Time"},
			{Key: "component", Label: "Component"},
			{Key: "message", Label: "Message"},
		},
		EmptyMessage: "No recent errors.",
	}

	if dispatch != nil {
		username := GetUsernameFromRequest(r)
		output, err := dispatch("show errors", username, r.RemoteAddr)
		if err == nil && output != "" {
			data.Rows = parseIssueJSON(output, "errors", false)
		}
	}

	return renderer.RenderFragment("log_table", data)
}

// issueJSON matches the report.Issue JSON shape returned by the show
// warnings/errors RPC handlers via serverDispatcher's JSON marshaling.
type issueJSON struct {
	Source  string    `json:"source"`
	Code    string    `json:"code"`
	Subject string    `json:"subject"`
	Message string    `json:"message"`
	Raised  time.Time `json:"raised"`
	Updated time.Time `json:"updated"`
}

// parseIssueJSON parses the JSON response from "show warnings" or "show errors".
// The dispatch returns JSON like {"warnings":[...],"count":N} or
// {"errors":[...],"count":N}. The key parameter selects which array to extract.
// When includeDuration is true, a fourth "Duration" column is computed from
// Raised to Updated (for warnings which are ongoing conditions).
func parseIssueJSON(output, key string, includeDuration bool) []WorkbenchTableRow {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal([]byte(output), &envelope); err != nil {
		return parseLogOutput(output)
	}

	raw, ok := envelope[key]
	if !ok {
		return nil
	}

	var issues []issueJSON
	if err := json.Unmarshal(raw, &issues); err != nil {
		return nil
	}

	var rows []WorkbenchTableRow
	for _, issue := range issues {
		ts := issue.Updated.Format("2006-01-02 15:04:05")
		msg := issue.Message
		if msg == "" {
			msg = issue.Code + ": " + issue.Subject
		}
		cells := []string{ts, issue.Source, template.HTMLEscapeString(msg)}
		if includeDuration {
			cells = append(cells, formatDuration(issue.Updated.Sub(issue.Raised)))
		}
		rows = append(rows, WorkbenchTableRow{Cells: cells})
	}

	return rows
}

// formatDuration returns a short human-readable duration string.
func formatDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return d.Round(time.Second).String()
	case d < time.Hour:
		return d.Round(time.Minute).String()
	default:
		return d.Round(time.Hour).String()
	}
}

// parseLogOutput parses plain-text output as line-based rows. Used as fallback
// when the dispatch output is not structured JSON.
func parseLogOutput(output string) []WorkbenchTableRow {
	cleaned, _ := normalizeOutput(output)
	if cleaned == "" {
		return nil
	}

	var rows []WorkbenchTableRow
	for _, line := range splitLines(cleaned) {
		if line == "" {
			continue
		}
		rows = append(rows, WorkbenchTableRow{
			Cells: []string{"-", "-", template.HTMLEscapeString(line), "-"},
		})
	}

	return rows
}

// splitLines splits a string into lines, handling both \n and \r\n.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := range len(s) {
		if s[i] == '\n' {
			line := s[start:i]
			if line != "" && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
