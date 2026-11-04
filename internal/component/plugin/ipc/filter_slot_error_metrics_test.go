// Design: docs/plugin-development/metrics.md -- completed typed filter judgments.
package ipc

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestFilterSlotEmbeddedErrorsAreNotCompletedJudgments drives the actual
// connection dispatcher: explicit error-bearing replies count no completed
// typed judgment, while an ordinary successful reply counts exactly once.
func TestFilterSlotEmbeddedErrorsAreNotCompletedJudgments(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	previous := filterBatchMetricsPtr.Load()
	SetFilterMetrics(reg)
	t.Cleanup(func() { filterBatchMetricsPtr.Store(previous) })
	bridge := rpc.NewDirectBridge()
	bridge.SetFilterSlot(func(route *rpc.FilterRoute) (*rpc.FilterUpdateOutput, error) {
		if route.Filter() == "good" {
			return &rpc.FilterUpdateOutput{Action: rpc.FilterAccept}, nil
		}
		out := &rpc.FilterUpdateOutput{Action: rpc.FilterAccept, Error: "data unavailable"}
		if route.Filter() == "modify" {
			out.Action = rpc.FilterModify
			out.Raw = []byte{0, 0, 0, 0}
		}
		if route.Filter() == "teardown" {
			out.Teardown = true
		}
		return out, nil
	})
	bridge.SetReady()
	t.Cleanup(bridge.CloseCallbacks)
	conn := &PluginConn{}
	conn.SetBridge(bridge)
	for _, name := range []string{"accept", "modify", "teardown", "good"} {
		route := rpc.InputFilterRoute(&rpc.FilterUpdateInput{Filter: name})
		out, err := conn.CallFilter(context.Background(), &route)
		if name == "good" {
			if err != nil || out == nil || out.Action != rpc.FilterAccept {
				t.Errorf("good route verdict=%+v error=%v, want accept", out, err)
			}
		} else if err == nil || out != nil || !strings.Contains(err.Error(), "data unavailable") {
			t.Errorf("%s error verdict present=%v error=%v, want only its declared failure", name, out != nil, err)
		}
	}
	recorder := httptest.NewRecorder()
	reg.Handler().ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), "GET", "/metrics", nil))
	if !strings.Contains(recorder.Body.String(), "ze_plugin_filter_slot_routes_total 1\n") {
		t.Fatalf("completed judgment counter includes explicit route errors: %s", recorder.Body.String())
	}
}
