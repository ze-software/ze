// Design: docs/plugin-development/metrics.md -- completed typed filter judgments.

package ipc

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestFilterSlotCountsOnlyCompletedJudgments exercises the actual connection
// dispatcher, not the counter helper: successful slot returns count once,
// evaluator errors, nil verdicts and closed slots count nothing, and none of
// these typed calls enters the encoded batch path. Not parallel: metrics are
// package state.
func TestFilterSlotCountsOnlyCompletedJudgments(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	previous := filterBatchMetricsPtr.Load()
	SetFilterMetrics(reg)
	t.Cleanup(func() { filterBatchMetricsPtr.Store(previous) })
	broken := errors.New("evaluator failed")
	for _, test := range []struct {
		name   string
		out    *rpc.FilterUpdateOutput
		err    error
		closed bool
	}{
		{name: "accepted", out: &rpc.FilterUpdateOutput{Action: rpc.FilterAccept}},
		{name: "rejected", out: &rpc.FilterUpdateOutput{Action: rpc.FilterReject}},
		{name: "error", err: broken},
		{name: "nil verdict"},
		{name: "closed", out: &rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, closed: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			bridge := rpc.NewDirectBridge()
			bridge.SetFilterSlot(func(*rpc.FilterRoute) (*rpc.FilterUpdateOutput, error) {
				return test.out, test.err
			})
			bridge.SetReady()
			if test.closed {
				bridge.CloseCallbacks()
			}
			conn := &PluginConn{}
			conn.SetBridge(bridge)
			route := rpc.InputFilterRoute(&rpc.FilterUpdateInput{Filter: "judge"})
			out, err := conn.CallFilter(context.Background(), &route)
			if test.closed {
				if !errors.Is(err, rpc.ErrBridgeClosed) {
					t.Fatalf("closed slot error=%v", err)
				}
				return
			}
			if test.out == nil && test.err == nil {
				// No verdict fails the route, as the batched transport does.
				if err == nil || !strings.Contains(err.Error(), "answered no verdict") {
					t.Fatalf("nil verdict error=%v, want the no-verdict refusal", err)
				}
			} else if !errors.Is(err, test.err) {
				t.Fatalf("error=%v, want %v", err, test.err)
			}
			if out != test.out {
				t.Fatalf("verdict=%v, want %v", out, test.out)
			}
		})
	}
	recorder := httptest.NewRecorder()
	reg.Handler().ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), "GET", "/metrics", nil))
	for _, want := range []string{
		"ze_plugin_filter_slot_routes_total 2\n",
		"ze_plugin_filter_batch_routes_sum 0\n",
		"ze_plugin_filter_batch_routes_count 0\n",
	} {
		if !strings.Contains(recorder.Body.String(), want) {
			t.Errorf("scrape lacks %q: %s", want, recorder.Body.String())
		}
	}
}
