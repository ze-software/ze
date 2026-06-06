package rpki

import (
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

func benchPipelinePlugin(b *testing.B) (*RPKIPlugin, *atomic.Int64, func()) {
	b.Helper()

	var dispatched atomic.Int64
	bridge := rpc.NewDirectBridge()
	bridge.SetDispatchCommandArgs(func(_ string, args []string, _ string) (*rpc.DispatchCommandOutput, error) {
		dispatched.Add(int64(len(args) / 6))
		return &rpc.DispatchCommandOutput{Status: rpc.StatusDone}, nil
	})
	bridge.SetReady()

	pluginEnd, engineEnd := net.Pipe()
	p := sdk.NewWithConn("rpki-bench", rpc.NewBridgedConn(pluginEnd, bridge))

	rp := &RPKIPlugin{
		plugin:     p,
		cache:      NewROACache(),
		aspaCache:  NewASPACache(),
		validateCh: make(chan validationRequest, 4096),
		stopCh:     make(chan struct{}),
	}

	var wg sync.WaitGroup
	wg.Go(rp.validationWorker)

	cleanup := func() {
		close(rp.stopCh)
		wg.Wait()
		p.Close()         //nolint:errcheck // bench cleanup
		engineEnd.Close() //nolint:errcheck // bench cleanup
	}
	return rp, &dispatched, cleanup
}

func benchRequests(n int) []validationRequest {
	reqs := make([]validationRequest, n)
	for i := range n {
		reqs[i] = validationRequest{
			peerAddr: "10.0.0.1",
			family:   "ipv4/unicast",
			prefix:   "10.0.0.0/24",
			pathID:   uint32(i),
			state:    ValidationValid,
		}
	}
	return reqs
}

// BenchmarkDispatchPipelineBatch measures the full rpki-to-adj-rib-in pipeline
// with batching enabled (up to 128 decisions per DispatchCommandArgs call).
// Each iteration sends 128 decisions through validationWorker (channel +
// timer coalescing) -> buildBatchArgs -> DispatchCommandArgs -> DirectBridge
// -> mock handler. Per-decision cost = ns/op / 128.
//
// Note: includes channel and timer overhead that the Individual benchmark
// skips (it calls dispatchBatch directly). The real per-call savings from
// batching are larger than the raw ratio suggests.
func BenchmarkDispatchPipelineBatch(b *testing.B) {
	rp, dispatched, cleanup := benchPipelinePlugin(b)
	defer cleanup()

	const batchSize = 128
	reqs := benchRequests(batchSize)

	b.ResetTimer()
	for b.Loop() {
		before := dispatched.Load()
		for i := range batchSize {
			rp.validateCh <- reqs[i]
		}
		for dispatched.Load() < before+batchSize {
			runtime.Gosched()
		}
	}
}

// BenchmarkDispatchPipelineIndividual measures the per-call cost of
// dispatchBatch with a single decision (no channel, no coalescing worker).
// Compare per-decision cost with BenchmarkDispatchPipelineBatch to see the
// DispatchCommandArgs overhead that batching eliminates.
func BenchmarkDispatchPipelineIndividual(b *testing.B) {
	rp, _, cleanup := benchPipelinePlugin(b)
	defer cleanup()

	single := benchRequests(1)

	b.ResetTimer()
	for b.Loop() {
		rp.dispatchBatch(single)
	}
}
