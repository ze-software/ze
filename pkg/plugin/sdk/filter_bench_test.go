package sdk

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// The throughput harness of FR-7: one evaluator judged on the fast path
// (the typed bridge slot) and on the forked path (one filter-update batch
// marshaled, answered and decoded) at several batch sizes. Thomas runs it on
// his Mac; an agent runs it with -benchtime=1x only, to prove it executes.

var benchFields = json.RawMessage(`{"med":7,"local-preference":100}`)

func benchEvaluator(route *FilterRoute) (*FilterUpdateOutput, error) {
	if _, err := route.Fields(); err != nil {
		return nil, err
	}
	return &FilterUpdateOutput{Action: rpc.FilterAccept}, nil
}

// BenchmarkFilterFastPath measures one route through the typed slot.
func BenchmarkFilterFastPath(b *testing.B) {
	bridge := rpc.NewDirectBridge()
	bridge.SetFilterSlot(rpc.FilterSlotHandler(benchEvaluator))
	bridge.SetReady()
	view := &conformanceView{fields: benchFields}
	header := rpc.FilterUpdateInput{Filter: "f", Direction: "import", Peer: "192.0.2.1", PeerAS: 65001}
	b.ReportAllocs()
	for b.Loop() {
		route := rpc.ViewFilterRoute(&header, view)
		if _, err := bridge.CallFilterSlot(&route); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFilterForkedBatch measures the forked path per batch of n routes:
// the engine's marshal, the SDK's unmarshal, evaluation and answer, and the
// engine's decode. ns/op divided by n is the cost per route.
func BenchmarkFilterForkedBatch(b *testing.B) {
	for _, n := range []int{1, 16, rpc.FilterUpdateBatchMax} {
		b.Run("routes="+strconv.Itoa(n), func(b *testing.B) {
			batch := rpc.FilterUpdateBatchInput{Routes: make([]rpc.FilterUpdateInput, n)}
			for i := range batch.Routes {
				batch.Routes[i] = rpc.FilterUpdateInput{Filter: "f", Direction: "import",
					Peer: "192.0.2.1", PeerAS: 65001, Fields: benchFields}
			}
			b.ReportAllocs()
			for b.Loop() {
				params, err := json.Marshal(batch)
				if err != nil {
					b.Fatal(err)
				}
				answer, err := answerFilterBatch(benchEvaluator, params)
				if err != nil {
					b.Fatal(err)
				}
				var out rpc.FilterUpdateBatchOutput
				if err := json.Unmarshal(answer, &out); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
