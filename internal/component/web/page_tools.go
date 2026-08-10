// Design: docs/architecture/web-workbench-pages.md -- Tool page handlers
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
	"context"
	"html/template"
	"net/http"
	"net/netip"
	"regexp"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// toolPageData is the template payload for tool page forms. The form renders
// on GET; Error and Output populate on POST after command dispatch.
type toolPageData struct {
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
		return handleToolPingPage(renderer, r, dispatch), true
	}

	switch path[0] {
	case "ping":
		return handleToolPingPage(renderer, r, dispatch), true
	case "bgp-decode":
		return handleToolBGPDecodePage(renderer, r, dispatch), true
	case "metrics":
		return handleToolMetricsPage(renderer, r, dispatch), true
	case "capture":
		return handleToolCapturePage(renderer, r, dispatch), true
	}

	return "", false
}

// --- Ping ---

// handleToolPingPage returns the rendered HTML for the Ping tool page.
// GET renders the empty form. POST validates, dispatches, and renders results.
func handleToolPingPage(renderer *Renderer, r *http.Request, dispatch CommandDispatcher) template.HTML {
	data := toolPageData{}

	if r.Method == http.MethodPost {
		data = handlePingSubmit(r, dispatch)
	}

	return renderer.RenderFragment("tool_ping", data)
}

// handlePingSubmit validates ping form params and dispatches the command.
func handlePingSubmit(r *http.Request, dispatch CommandDispatcher) toolPageData {
	if err := r.ParseForm(); err != nil {
		return toolPageData{Error: "Invalid form data."}
	}

	dest := strings.TrimSpace(r.PostFormValue("destination"))
	if dest == "" {
		return toolPageData{Error: "Destination is required."}
	}

	if _, err := netip.ParseAddr(dest); err != nil {
		var tb textbuf.Buffer
		return toolPageData{Error: tb.Str("Invalid IP address: ").Str(dest).String()}
	}

	countStr := strings.TrimSpace(r.PostFormValue("count"))
	count := 5
	if countStr != "" {
		v, err := strconv.Atoi(countStr)
		if err != nil || v < 1 || v > 100 {
			return toolPageData{Error: "Count must be between 1 and 100."}
		}
		count = v
	}

	// 65507 is the largest ICMP payload that fits a 65535-byte IP datagram
	// after the IPv4 and ICMP headers; it matches maxPingSize in the ping
	// module and the range on the show/ping size leaf in ze-ping-cmd.yang.
	sizeStr := strings.TrimSpace(r.PostFormValue("size"))
	size := 0
	if sizeStr != "" {
		v, err := strconv.Atoi(sizeStr)
		if err != nil || v < 1 || v > 65507 {
			return toolPageData{Error: "Packet size must be between 1 and 65507 bytes."}
		}
		size = v
	}

	timeoutStr := strings.TrimSpace(r.PostFormValue("timeout"))
	timeout := 5
	if timeoutStr != "" {
		v, err := strconv.Atoi(timeoutStr)
		if err != nil || v < 1 || v > 30 {
			return toolPageData{Error: "Timeout must be between 1 and 30 seconds."}
		}
		timeout = v
	}

	var bCmd textbuf.Buffer
	bCmd.Reset().Str("show ping ").Str(dest).Str(" count ").Int(int64(count))
	if size > 0 {
		bCmd.Str(" size ").Int(int64(size))
	}
	cmd := bCmd.Str(" timeout ").Int(int64(timeout)).Str("s").String()

	return dispatchToolCommand(r, dispatch, cmd)
}

// --- BGP Decode ---

// handleToolBGPDecodePage returns the rendered HTML for the BGP Decode tool page.
func handleToolBGPDecodePage(renderer *Renderer, r *http.Request, dispatch CommandDispatcher) template.HTML {
	data := toolPageData{}

	if r.Method == http.MethodPost {
		data = handleBGPDecodeSubmit(r, dispatch)
	}

	return renderer.RenderFragment("tool_bgp_decode", data)
}

// handleBGPDecodeSubmit validates hex input and dispatches the decode command.
func handleBGPDecodeSubmit(r *http.Request, dispatch CommandDispatcher) toolPageData {
	if err := r.ParseForm(); err != nil {
		return toolPageData{Error: "Invalid form data."}
	}

	hex := strings.TrimSpace(r.PostFormValue("hex"))
	if hex == "" {
		return toolPageData{Error: "Hex input is required."}
	}

	if !validHexPattern.MatchString(hex) {
		return toolPageData{Error: "Input must contain only hexadecimal characters and whitespace."}
	}

	// Collapse whitespace for the command.
	compact := textbuf.Join(strings.Fields(hex), "")
	if len(compact) > 65535*2 {
		return toolPageData{Error: "Hex input exceeds maximum length (65535 bytes)."}
	}

	var tb textbuf.Buffer
	cmd := tb.Str("show bgp decode ").Str(compact).String()

	return dispatchToolCommand(r, dispatch, cmd)
}

// --- Metrics Query ---

// handleToolMetricsPage returns the rendered HTML for the Metrics Query tool page.
func handleToolMetricsPage(renderer *Renderer, r *http.Request, dispatch CommandDispatcher) template.HTML {
	data := toolPageData{}

	if r.Method == http.MethodPost {
		data = handleMetricsSubmit(r, dispatch)
	}

	return renderer.RenderFragment("tool_metrics", data)
}

// handleMetricsSubmit validates metric name and dispatches the query command.
func handleMetricsSubmit(r *http.Request, dispatch CommandDispatcher) toolPageData {
	if err := r.ParseForm(); err != nil {
		return toolPageData{Error: "Invalid form data."}
	}

	name := strings.TrimSpace(r.PostFormValue("name"))
	if name == "" {
		return toolPageData{Error: "Metric name is required."}
	}

	if len(name) > maxMetricNameLen {
		var bErr textbuf.Buffer
		return toolPageData{Error: bErr.Reset().Str("Metric name exceeds maximum length (").Int(int64(maxMetricNameLen)).Str(" characters).").String()}
	}

	if !validMetricNamePattern.MatchString(name) {
		return toolPageData{Error: "Metric name must be alphanumeric with underscores or colons."}
	}

	label := strings.TrimSpace(r.PostFormValue("label"))
	var tb textbuf.Buffer
	tb.Str("show metrics name ").Str(name)
	if label != "" {
		tb.Byte(' ').Str(label)
	}
	cmd := tb.String()

	return dispatchToolCommand(r, dispatch, cmd)
}

// --- Capture ---

// handleToolCapturePage returns the rendered HTML for the Capture tool page.
func handleToolCapturePage(renderer *Renderer, r *http.Request, dispatch CommandDispatcher) template.HTML {
	data := toolPageData{}

	if r.Method == http.MethodPost {
		data = handleCaptureSubmit(r, dispatch)
	}

	return renderer.RenderFragment("tool_capture", data)
}

// handleCaptureSubmit validates capture filters and dispatches the command.
func handleCaptureSubmit(r *http.Request, dispatch CommandDispatcher) toolPageData {
	if err := r.ParseForm(); err != nil {
		return toolPageData{Error: "Invalid form data."}
	}

	var parts []string
	parts = append(parts, "show capture")

	tunnelIDStr := strings.TrimSpace(r.PostFormValue("tunnel-id"))
	if tunnelIDStr != "" {
		v, err := strconv.Atoi(tunnelIDStr)
		if err != nil || v < 0 || v > 65535 {
			return toolPageData{Error: "Tunnel ID must be between 0 and 65535."}
		}
		if v > 0 {
			var bTun textbuf.Buffer
			parts = append(parts, bTun.Reset().Str("tunnel-id ").Int(int64(v)).String())
		}
	}

	peer := strings.TrimSpace(r.PostFormValue("peer"))
	if peer != "" {
		if _, err := netip.ParseAddr(peer); err != nil {
			var tb textbuf.Buffer
			return toolPageData{Error: tb.Str("Invalid peer IP address: ").Str(peer).String()}
		}
		parts = append(parts, "peer "+peer)
	}

	countStr := strings.TrimSpace(r.PostFormValue("count"))
	captureCount := 100
	if countStr != "" {
		v, err := strconv.Atoi(countStr)
		if err != nil || v < 1 || v > 10000 {
			return toolPageData{Error: "Count must be between 1 and 10000."}
		}
		captureCount = v
	}
	var bCount textbuf.Buffer
	parts = append(parts, bCount.Reset().Str("count ").Int(int64(captureCount)).String())

	cmd := textbuf.Join(parts, " ")

	return dispatchToolCommand(r, dispatch, cmd)
}

// --- Shared dispatch ---

// dispatchToolCommand sends a command through the CommandDispatcher and returns
// the result as ToolPageData. Output is ANSI-stripped, HTML-escaped, and capped.
func dispatchToolCommand(r *http.Request, dispatch CommandDispatcher, cmd string) toolPageData {
	if dispatch == nil {
		return toolPageData{Error: "Command dispatch not available."}
	}

	username := GetUsernameFromRequest(r)
	output, err := dispatch.JSON(context.Background(), plugin.CallerIdentity{Username: username, RemoteAddr: r.RemoteAddr}, cmd)
	if err != nil {
		errMsg := err.Error()
		if output != "" {
			errMsg = output
		}
		return toolPageData{Error: errMsg}
	}

	cleaned, truncated := normalizeOutput(output)
	result := template.HTMLEscapeString(cleaned)
	if truncated {
		result += "\n\n[Output truncated at 4 MiB]"
	}

	return toolPageData{Output: result}
}
