// Design: docs/architecture/api/commands.md — BGP metrics show and list handlers
// Overview: doc.go — bgp-cmd-metrics plugin registration

package metrics

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	prommetrics "github.com/ze-software/ze/internal/core/metrics"
)

func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:metrics-values", Handler: handleMetricsValues},
		pluginserver.RPCRegistration{WireMethod: "ze-bgp:metrics-list", Handler: handleMetricsList},
	)
}

// getPrometheusRegistry retrieves the registry that owns both metric
// declarations and their current samples.
func getPrometheusRegistry() (*prommetrics.PrometheusRegistry, *plugin.Response) {
	reg := registry.GetMetricsRegistry()
	promReg, ok := reg.(*prommetrics.PrometheusRegistry)
	if !ok || promReg == nil {
		return nil, &plugin.Response{
			Status: plugin.StatusError,
			Error:  "metrics not available",
		}
	}
	return promReg, nil
}

func getPrometheusHandler() (http.Handler, *plugin.Response) {
	promReg, errResp := getPrometheusRegistry()
	if errResp != nil {
		return nil, errResp
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

// handleMetricsList returns a sorted list of every registered metric name.
func handleMetricsList(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	promReg, errResp := getPrometheusRegistry()
	if errResp != nil {
		return errResp, nil
	}
	names, err := promReg.Names()
	if err != nil {
		return &plugin.Response{Status: plugin.StatusError, Error: fmt.Sprintf("listing metrics: %v", err)}, nil
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{"names": names, "count": len(names)},
	}, nil
}

// captureAndReturnMetrics captures Prometheus text and returns a response.
func captureAndReturnMetrics(handler http.Handler) *plugin.Response {
	text, err := captureMetricsText(handler)
	if err != nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  fmt.Sprintf("capturing metrics: %v", err),
		}
	}

	return &plugin.Response{
		Status: plugin.StatusDone,
		Data: plugin.Map{
			"metrics": text,
		},
	}
}
