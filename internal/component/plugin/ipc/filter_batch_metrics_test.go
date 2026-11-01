// Design: docs/plugin-development/metrics.md -- ze_plugin_filter_batch_routes
package ipc

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestFilterBatchSizeObserved proves the slow path's batch-size histogram
// (D-3): each filter-update exchange observes the number of routes it
// carried, once. Method: a Prometheus registry wired into the package; one
// lone route is exchanged, then ten routes queued behind it go as one batch;
// the scrape shows two observations summing to eleven routes, one in the
// bucket of 1 and both under 16. Not parallel: the registry is package state.
func TestFilterBatchSizeObserved(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	SetFilterMetrics(reg)
	t.Cleanup(func() { filterBatchMetricsPtr.Store(nil) })

	engine, plugin := newTestPluginConn(t)
	first := make(chan filterResult, 1)
	go func() {
		out, err := engine.SendFilterUpdate(context.Background(), &rpc.FilterUpdateInput{Filter: "f", Peer: "lone"})
		first <- filterResult{peer: "lone", out: out, err: err}
	}()
	id, batch := readBatch(t, plugin)
	results := sendQueued(t, engine, 0, routePeers("r", 10))
	if err := plugin.SendResult(context.Background(), id, verdictsFor(batch)); err != nil {
		t.Fatal(err)
	}
	<-first
	id, batch = readBatch(t, plugin)
	if err := plugin.SendResult(context.Background(), id, verdictsFor(batch)); err != nil {
		t.Fatal(err)
	}
	checkVerdicts(t, results)

	rec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), "GET", "/metrics", nil))
	text := rec.Body.String()
	for _, want := range []string{
		"ze_plugin_filter_batch_routes_count 2",
		"ze_plugin_filter_batch_routes_sum 11",
		`ze_plugin_filter_batch_routes_bucket{le="1"} 1`,
		`ze_plugin_filter_batch_routes_bucket{le="16"} 2`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("scrape lacks %s:\n%s", want, text)
		}
	}
}
