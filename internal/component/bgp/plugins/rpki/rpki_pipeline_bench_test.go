package rpki

import (
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

func benchPipelinePlugin(b *testing.B) (*rPKIPlugin, *atomic.Int64, func()) {
	b.Helper()

	var dispatched atomic.Int64
	bridge := rpc.NewDirectBridge()
	bridge.SetBatchValidate(func(decisions []rpc.ValidationDecision) (*rpc.BatchValidateResult, error) {
		dispatched.Add(int64(len(decisions)))
		return &rpc.BatchValidateResult{Accepted: len(decisions)}, nil
	})
	bridge.SetReady()

	pluginEnd, engineEnd := net.Pipe()
	p := sdk.NewWithConn("rpki-bench", rpc.NewBridgedConn(pluginEnd, bridge))

	rp := &rPKIPlugin{
		plugin:     p,
		cache:      newROACache(),
		aspaCache:  newASPACache(),
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
// with batching enabled (up to 128 decisions per BatchValidate call).
// Each iteration sends 128 decisions through validationWorker (channel +
// timer coalescing) -> buildDecisions -> BatchValidate -> DirectBridge
// -> mock handler. Per-decision cost = ns/op / 128.
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
// BatchValidate overhead that batching eliminates.
func BenchmarkDispatchPipelineIndividual(b *testing.B) {
	rp, _, cleanup := benchPipelinePlugin(b)
	defer cleanup()

	single := benchRequests(1)

	b.ResetTimer()
	for b.Loop() {
		rp.dispatchBatch(single)
	}
}
