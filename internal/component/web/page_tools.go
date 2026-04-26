// Design: plan/spec-web-8-tools-logs.md -- Tool page handlers
// Related: handler_tools.go -- Related-tool overlay handler (separate from these)
// Related: handler_admin.go -- CommandDispatcher type
// Related: workbench_pages.go -- Page dispatch (renderPageContent)
//
// Each tool page renders a purpose-built form inside the workbench shell.
// GET returns the form; POST validates input, dispatches a show command
// through the standard CommandDispatcher, and renders the result inline.
// All output is ANSI-stripped, HTML-escaped, and capped at 4 MiB.

package web

import (
	"fmt"
	"html/template"
	"net/http"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
)

// ToolPageData is the template payload for tool page forms. The form renders
// on GET; Error and Output populate on POST after command dispatch.
type ToolPageData struct {
	Error  string
	Output string
}

// validHexPattern matches strings containing only hex digits and whitespace.
var validHexPattern = regexp.MustCompile(`^[0-9a-fA-F\s]+$`)

// validMetricNamePattern matches Prometheus metric names: alphanumeric, underscore, colon.
var validMetricNamePattern = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)

// maxMetricNameLen is the maximum length for a metric name input.
const maxMetricNameLen = 256

// renderToolPageContent dispatches tool sub-pages. The path slice has the
// leading "tools" segment already stripped. Returns (content, true) if a
// page handler matched, or ("", false) to fall through.
func renderToolPageContent(renderer *Renderer, r *http.Request, path []string, dispatch CommandDispatcher) (template.HTML, bool) {
	if len(path) == 0 {
		// /show/tools/ defaults to ping.
		return HandleToolPingPage(renderer, r, dispatch), true
	}

	switch path[0] {
	case "ping":
		return HandleToolPingPage(renderer, r, dispatch), true
	case "bgp-decode":
		return HandleToolBGPDecodePage(renderer, r, dispatch), true
	case "metrics":
		return HandleToolMetricsPage(renderer, r, dispatch), true
	case "capture":
		return HandleToolCapturePage(renderer, r, dispatch), true
	}

	return "", false
}

// --- Ping ---

// HandleToolPingPage returns the rendered HTML for the Ping tool page.
// GET renders the empty form. POST validates, dispatches, and renders results.
func HandleToolPingPage(renderer *Renderer, r *http.Request, dispatch CommandDispatcher) template.HTML {
	data := ToolPageData{}

	if r.Method == http.MethodPost {
		data = handlePingSubmit(r, dispatch)
	}

	return renderer.RenderFragment("tool_ping", data)
}

// handlePingSubmit validates ping form params and dispatches the command.
func handlePingSubmit(r *http.Request, dispatch CommandDispatcher) ToolPageData {
	if err := r.ParseForm(); err != nil {
		return ToolPageData{Error: "Invalid form data."}
	}

	dest := strings.TrimSpace(r.PostFormValue("destination"))
	if dest == "" {
		return ToolPageData{Error: "Destination is required."}
	}

	if _, err := netip.ParseAddr(dest); err != nil {
		return ToolPageData{Error: fmt.Sprintf("Invalid IP address: %s", dest)}
	}

	countStr := strings.TrimSpace(r.PostFormValue("count"))
	count := 5
	if countStr != "" {
		v, err := strconv.Atoi(countStr)
		if err != nil || v < 1 || v > 100 {
			return ToolPageData{Error: "Count must be between 1 and 100."}
		}
		count = v
	}

	timeoutStr := strings.TrimSpace(r.PostFormValue("timeout"))
	timeout := 5
	if timeoutStr != "" {
		v, err := strconv.Atoi(timeoutStr)
		if err != nil || v < 1 || v > 30 {
			return ToolPageData{Error: "Timeout must be between 1 and 30 seconds."}
		}
		timeout = v
	}

	cmd := fmt.Sprintf("show ping %s count %d timeout %ds", dest, count, timeout)

	return dispatchToolCommand(r, dispatch, cmd)
}

// --- BGP Decode ---

// HandleToolBGPDecodePage returns the rendered HTML for the BGP Decode tool page.
func HandleToolBGPDecodePage(renderer *Renderer, r *http.Request, dispatch CommandDispatcher) template.HTML {
	data := ToolPageData{}

	if r.Method == http.MethodPost {
		data = handleBGPDecodeSubmit(r, dispatch)
	}

	return renderer.RenderFragment("tool_bgp_decode", data)
}

// handleBGPDecodeSubmit validates hex input and dispatches the decode command.
func handleBGPDecodeSubmit(r *http.Request, dispatch CommandDispatcher) ToolPageData {
	if err := r.ParseForm(); err != nil {
		return ToolPageData{Error: "Invalid form data."}
	}

	hex := strings.TrimSpace(r.PostFormValue("hex"))
	if hex == "" {
		return ToolPageData{Error: "Hex input is required."}
	}

	if !validHexPattern.MatchString(hex) {
		return ToolPageData{Error: "Input must contain only hexadecimal characters and whitespace."}
	}

	// Collapse whitespace for the command.
	compact := strings.Join(strings.Fields(hex), "")
	if len(compact) > 65535*2 {
		return ToolPageData{Error: "Hex input exceeds maximum length (65535 bytes)."}
	}

	cmd := fmt.Sprintf("show bgp/decode %s", compact)

	return dispatchToolCommand(r, dispatch, cmd)
}

// --- Metrics Query ---

// HandleToolMetricsPage returns the rendered HTML for the Metrics Query tool page.
func HandleToolMetricsPage(renderer *Renderer, r *http.Request, dispatch CommandDispatcher) template.HTML {
	data := ToolPageData{}

	if r.Method == http.MethodPost {
		data = handleMetricsSubmit(r, dispatch)
	}

	return renderer.RenderFragment("tool_metrics", data)
}

// handleMetricsSubmit validates metric name and dispatches the query command.
func handleMetricsSubmit(r *http.Request, dispatch CommandDispatcher) ToolPageData {
	if err := r.ParseForm(); err != nil {
		return ToolPageData{Error: "Invalid form data."}
	}

	name := strings.TrimSpace(r.PostFormValue("name"))
	if name == "" {
		return ToolPageData{Error: "Metric name is required."}
	}

	if len(name) > maxMetricNameLen {
		return ToolPageData{Error: fmt.Sprintf("Metric name exceeds maximum length (%d characters).", maxMetricNameLen)}
	}

	if !validMetricNamePattern.MatchString(name) {
		return ToolPageData{Error: "Metric name must be alphanumeric with underscores or colons."}
	}

	label := strings.TrimSpace(r.PostFormValue("label"))
	cmd := "show metrics-query " + name
	if label != "" {
		cmd += " " + label
	}

	return dispatchToolCommand(r, dispatch, cmd)
}

// --- Capture ---

// HandleToolCapturePage returns the rendered HTML for the Capture tool page.
func HandleToolCapturePage(renderer *Renderer, r *http.Request, dispatch CommandDispatcher) template.HTML {
	data := ToolPageData{}

	if r.Method == http.MethodPost {
		data = handleCaptureSubmit(r, dispatch)
	}

	return renderer.RenderFragment("tool_capture", data)
}

// handleCaptureSubmit validates capture filters and dispatches the command.
func handleCaptureSubmit(r *http.Request, dispatch CommandDispatcher) ToolPageData {
	if err := r.ParseForm(); err != nil {
		return ToolPageData{Error: "Invalid form data."}
	}

	var parts []string
	parts = append(parts, "show capture")

	tunnelIDStr := strings.TrimSpace(r.PostFormValue("tunnel-id"))
	if tunnelIDStr != "" {
		v, err := strconv.Atoi(tunnelIDStr)
		if err != nil || v < 0 || v > 65535 {
			return ToolPageData{Error: "Tunnel ID must be between 0 and 65535."}
		}
		if v > 0 {
			parts = append(parts, fmt.Sprintf("tunnel-id %d", v))
		}
	}

	peer := strings.TrimSpace(r.PostFormValue("peer"))
	if peer != "" {
		if _, err := netip.ParseAddr(peer); err != nil {
			return ToolPageData{Error: fmt.Sprintf("Invalid peer IP address: %s", peer)}
		}
		parts = append(parts, "peer "+peer)
	}

	countStr := strings.TrimSpace(r.PostFormValue("count"))
	captureCount := 100
	if countStr != "" {
		v, err := strconv.Atoi(countStr)
		if err != nil || v < 1 || v > 10000 {
			return ToolPageData{Error: "Count must be between 1 and 10000."}
		}
		captureCount = v
	}
	parts = append(parts, fmt.Sprintf("count %d", captureCount))

	cmd := strings.Join(parts, " ")

	return dispatchToolCommand(r, dispatch, cmd)
}

// --- Shared dispatch ---

// dispatchToolCommand sends a command through the CommandDispatcher and returns
// the result as ToolPageData. Output is ANSI-stripped, HTML-escaped, and capped.
func dispatchToolCommand(r *http.Request, dispatch CommandDispatcher, cmd string) ToolPageData {
	if dispatch == nil {
		return ToolPageData{Error: "Command dispatch not available."}
	}

	username := GetUsernameFromRequest(r)
	output, err := dispatch(cmd, username, r.RemoteAddr)
	if err != nil {
		errMsg := err.Error()
		if output != "" {
			errMsg = output
		}
		return ToolPageData{Error: errMsg}
	}

	cleaned, truncated := normalizeOutput(output)
	result := template.HTMLEscapeString(cleaned)
	if truncated {
		result += "\n\n[Output truncated at 4 MiB]"
	}

	return ToolPageData{Output: result}
}
