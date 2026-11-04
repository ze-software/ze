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
	results := sendQueued(t, engine, routePeers("r", 10))
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

// TestFilterEnqueuesCountOnlyAdmissions proves the admission fence advances
// only after a successful queue send. Method: fill a queue with one request,
// cancel a second wait, then close and refuse a third; the scrape stays at one.
// Not parallel: the registry is package state.
func TestFilterEnqueuesCountOnlyAdmissions(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	SetFilterMetrics(reg)
	t.Cleanup(func() { filterBatchMetricsPtr.Store(nil) })

	b := filterBatcher{queue: make(chan *filterRequest, 1)}
	if err := b.admit(t.Context(), &filterRequest{}); err != nil {
		t.Fatal(err)
	}
	if len(b.queue) != 1 {
		t.Fatal("successful admission did not enqueue its request")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := b.admit(ctx, &filterRequest{}); err == nil {
		t.Fatal("full queue accepted a canceled waiter")
	}
	b.close()
	if err := b.admit(t.Context(), &filterRequest{}); err == nil {
		t.Fatal("closed batcher accepted a request")
	}

	rec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), "GET", "/metrics", nil))
	if text := rec.Body.String(); !strings.Contains(text, "ze_plugin_filter_route_enqueues_total 1\n") {
		t.Errorf("only the successful enqueue must count:\n%s", text)
	}
}
