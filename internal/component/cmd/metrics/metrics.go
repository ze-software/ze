// Design: docs/architecture/api/commands.md — BGP metrics show and list handlers
// Overview: doc.go — bgp-cmd-metrics plugin registration

package metrics

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
	prommetrics "codeberg.org/thomas-mangin/ze/internal/core/metrics"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:metrics-values", Handler: handleMetricsValues},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:metrics-list", Handler: handleMetricsList},
	)
}

// getPrometheusHandler retrieves the Prometheus HTTP handler from the registry.
// Returns the handler, or nil with an error response if metrics are not available.
func getPrometheusHandler() (http.Handler, *plugin.Response) {
	reg := registry.GetMetricsRegistry()
	if reg == nil {
		return nil, &plugin.Response{
			Status: plugin.StatusError,
			Data:   "metrics not available",
		}
	}

	promReg, ok := reg.(*prommetrics.PrometheusRegistry)
	if !ok {
		return nil, &plugin.Response{
			Status: plugin.StatusError,
			Data:   "metrics not available",
		}
	}

	return promReg.Handler(), nil
}

// captureMetricsText invokes the Prometheus handler and returns the text output.
func captureMetricsText(handler http.Handler) (string, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", http.NoBody)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	return recorder.Body.String(), nil
}

// handleMetricsValues returns Prometheus text format output.
func handleMetricsValues(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	handler, errResp := getPrometheusHandler()
	if errResp != nil {
		return errResp, nil
	}

	return captureAndReturnMetrics(handler), nil
}

// handleMetricsList returns a sorted list of metric names.
func handleMetricsList(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	handler, errResp := getPrometheusHandler()
	if errResp != nil {
		return errResp, nil
	}

	return captureAndReturnNames(handler), nil
}

// captureAndReturnMetrics captures Prometheus text and returns a response.
func captureAndReturnMetrics(handler http.Handler) *plugin.Response {
	text, err := captureMetricsText(handler)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("capturing metrics: %v", err),
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"metrics": text,
		},
	}
}

// captureAndReturnNames captures Prometheus text and returns metric names.
func captureAndReturnNames(handler http.Handler) *plugin.Response {
	text, err := captureMetricsText(handler)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Data:   fmt.Sprintf("capturing metrics: %v", err),
		}
	}

	names := extractMetricNames(text)

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: map[string]any{
			"names": names,
			"count": len(names),
		},
	}
}

// extractMetricNames parses Prometheus text format and returns sorted unique metric names.
// Skips HELP and TYPE comment lines, extracts the metric name from each sample line.
func extractMetricNames(text string) []string {
	seen := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(text))

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Metric line format: metric_name{labels} value
		// or: metric_name value
		name := line
		if idx := strings.IndexAny(name, " {"); idx > 0 {
			name = name[:idx]
		}
		if name != "" {
			seen[name] = true
		}
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}
