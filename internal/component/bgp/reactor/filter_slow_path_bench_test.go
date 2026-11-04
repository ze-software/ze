// Design: docs/guide/benchmarking.md -- Filter throughput
// Related: filter_answer_transport_test.go -- answerTransport, whose declarations the forked transport keeps
// Related: filter_ordered.go -- runIngressPolicyChainMeta, the import entry each route runs
// Related: filter_library.go -- rewriteLibrary, which rebuilds the modified UPDATE body

package reactor

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
	routepolicy "github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// BenchmarkFilterSlowPathModify measures one forked filter that MODIFIES the
// route, per route, from the received UPDATE to the rebuilt one, at 1, 16 and
// 64 routes per filter-update exchange (spec-filter-wire-0-umbrella A-6).
//
// Method: each route is its own received UPDATE (ORIGIN IGP, AS_PATH [65001],
// NEXT_HOP, MED 7, LOCAL_PREF 100, one IPv4 /24) driven through the real import
// entry, runIngressPolicyChainMeta, by one goroutine per route, as a session
// goroutine per peer does. The library chain's external member projects the
// declared fields (memberView.Fields, through FilterRoute.Input), the
// transport below sends the n projected routes as one filter-update batch on
// a real ipc.PluginConn (PluginConn.CallRPC, MuxConn framing), a real SDK
// plugin started over net.Pipe answers it through OnFilter's batch handler
// (answerFilterBatch) with an evaluator that sets local-preference 200, and
// the engine decodes the verdicts, applies the set (applyExternalSets) and
// rebuilds the body (rewriteLibrary, buildModifiedPayload).
//
// What the transport skips, and why: Server.CallFilterUpdate's process lookup
// and PluginConn.CallFilter's bridge check (one map lookup and one atomic
// load), and filterBatcher's queue and worker handoff. The batcher sends
// whatever is queued when its worker wakes, so the batch size would follow
// the scheduler; the transport instead sends exactly n routes per exchange,
// the same FilterUpdateBatchInput through the same CallRPC, and decodes the
// reply into FilterUpdateBatchOutput as filterBatcher.exchange does. net.Pipe
// stands in for the TLS socket.
//
// ns/route, B/route and allocs/route divide by the routes judged; the
// standard per-op figures are per exchange. Allocation counts include the SDK
// goroutines, which run in this process.
func BenchmarkFilterSlowPathModify(b *testing.B) {
	for _, n := range []int{1, 16, rpc.FilterUpdateBatchMax} {
		b.Run("routes="+strconv.Itoa(n), func(b *testing.B) {
			benchFilterSlowPath(b, n)
		})
	}
}

// slowPathVerdict is the evaluator's answer: modify, local-preference 200.
var slowPathVerdict = rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{
	{Field: "local-preference", Action: rpc.FilterSetActionSet, Value: json.RawMessage("200")}}}

// slowPathEvaluator reads the declared fields, as a filter deciding from
// them does, and answers slowPathVerdict.
func slowPathEvaluator(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	if _, err := route.Fields(); err != nil {
		return nil, err
	}
	return &slowPathVerdict, nil
}

// slowPathMethod is the callback the batch is sent on
// (ipc.methodFilterUpdate, sdk callbackFilterUpdate).
const slowPathMethod = "ze-plugin-callback:filter-update"

// slowPathPayload is the received UPDATE body of route i: ORIGIN IGP, AS_PATH
// [65001], NEXT_HOP 192.0.2.1, MED 7, LOCAL_PREF 100, and 10.0.i.0/24.
func slowPathPayload(i int) []byte {
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN igp
		0x40, 0x02, 0x06, 0x02, 0x01, 0x00, 0x00, 0xFD, 0xE9, // AS_PATH AS_SEQUENCE [65001]
		0x40, 0x03, 0x04, 192, 0, 2, 1, // NEXT_HOP 192.0.2.1
		0x80, 0x04, 0x04, 0, 0, 0, 7, // MULTI_EXIT_DISC 7
		0x40, 0x05, 0x04, 0, 0, 0, 100, // LOCAL_PREF 100
	}
	return buildUpdatePayload(attrs, []byte{24, 10, 0, byte(i)})
}

// slowPathLocalPref answers the LOCAL_PREF value of an UPDATE body with no
// withdrawn routes, ok false when the body carries none.
func slowPathLocalPref(body []byte) (uint32, bool) {
	if len(body) < 4 {
		return 0, false
	}
	attrLen := int(binary.BigEndian.Uint16(body[2:4]))
	if len(body) < 4+attrLen {
		return 0, false
	}
	attrs := body[4 : 4+attrLen]
	for len(attrs) >= 3 {
		flags, code := attrs[0], attrs[1]
		header, size := 3, int(attrs[2])
		if flags&0x10 != 0 {
			if len(attrs) < 4 {
				return 0, false
			}
			header, size = 4, int(binary.BigEndian.Uint16(attrs[2:4]))
		}
		if len(attrs) < header+size {
			return 0, false
		}
		if code == 5 && size == 4 {
			return binary.BigEndian.Uint32(attrs[header:]), true
		}
		attrs = attrs[header+size:]
	}
	return 0, false
}

// forkedBatchTransport is the plugin server for the benchmark: answerTransport
// declares the filter, and CallFilterUpdate sends every size routes as one
// filter-update exchange on engine. Safe for concurrent use: the route that
// completes the batch makes the exchange and answers the routes waiting.
type forkedBatchTransport struct {
	*answerTransport
	engine *ipc.PluginConn
	size   int

	mu      sync.Mutex
	inputs  []rpc.FilterUpdateInput
	replies []chan forkedReply
}

type forkedReply struct {
	out *rpc.FilterUpdateOutput
	err error
}

func newForkedBatchTransport(engine *ipc.PluginConn, size int) *forkedBatchTransport {
	f := &forkedBatchTransport{
		answerTransport: &answerTransport{reads: []string{"med", "local-preference"}, sets: []string{"local-preference"}},
		engine:          engine,
		size:            size,
		inputs:          make([]rpc.FilterUpdateInput, 0, size),
		replies:         make([]chan forkedReply, size),
	}
	for i := range f.replies {
		f.replies[i] = make(chan forkedReply, 1)
	}
	return f
}

// CallFilterUpdate projects route as PluginConn.CallFilter does, then waits
// for the exchange its batch makes.
func (f *forkedBatchTransport) CallFilterUpdate(ctx context.Context, _ string, route *rpc.FilterRoute) (*rpc.FilterUpdateOutput, error) {
	input, err := route.Input()
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	slot := len(f.inputs)
	f.inputs = append(f.inputs, input)
	full := len(f.inputs) == f.size
	f.mu.Unlock()
	if full {
		f.exchange(ctx)
	}
	reply := <-f.replies[slot]
	return reply.out, reply.err
}

// exchange sends the batch and answers each slot. Every other route of the
// batch is blocked on its reply, so the inputs are not touched meanwhile.
func (f *forkedBatchTransport) exchange(ctx context.Context) {
	result, err := f.engine.CallRPC(ctx, slowPathMethod, &rpc.FilterUpdateBatchInput{Routes: f.inputs})
	var out rpc.FilterUpdateBatchOutput
	if err == nil {
		err = json.Unmarshal(result, &out)
	}
	if err == nil && len(out.Verdicts) != f.size {
		err = errors.New("the reply carried " + strconv.Itoa(len(out.Verdicts)) + " verdicts")
	}
	f.mu.Lock()
	clear(f.inputs)
	f.inputs = f.inputs[:0]
	f.mu.Unlock()
	for i, reply := range f.replies {
		if err != nil {
			reply <- forkedReply{err: err}
			continue
		}
		reply <- forkedReply{out: &out.Verdicts[i]}
	}
}

// startForkedFilterPlugin starts a real SDK plugin with slowPathEvaluator
// over net.Pipe, drives its startup to the end, and answers the engine side.
func startForkedFilterPlugin(b *testing.B) *ipc.PluginConn {
	b.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	engineEnd, pluginEnd := net.Pipe()
	engine := ipc.NewMuxPluginConn(rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd)))
	plugin := sdk.NewWithConn("bench-slow-path", pluginEnd)
	plugin.OnFilter(slowPathEvaluator)
	runDone := make(chan error, 1)
	go func() { runDone <- plugin.Run(ctx, &sdk.Registration{}) }()
	b.Cleanup(func() {
		cancel()
		_ = plugin.Close() //nolint:errcheck // the benchmark is over; a close error changes no figure
		_ = engine.Close() //nolint:errcheck // the benchmark is over; a close error changes no figure
		select {
		case <-runDone:
		case <-time.After(5 * time.Second):
			b.Error("the SDK plugin did not stop")
		}
	})

	for _, stage := range []struct{ request, callback string }{
		{rpc.MethodDeclareRegistration, "ze-plugin-callback:configure"},
		{"ze-plugin-engine:declare-capabilities", "ze-plugin-callback:share-registry"},
	} {
		request, err := engine.ReadRequest(ctx)
		if err != nil {
			b.Fatal(err)
		}
		if request.Method != stage.request {
			b.Fatalf("startup request = %q, want %q", request.Method, stage.request)
		}
		if err := engine.SendResult(ctx, request.ID, &rpc.DeclareRegistrationOutput{ProtocolVersion: rpc.ProtocolVersion}); err != nil {
			b.Fatal(err)
		}
		if _, err := engine.CallRPC(ctx, stage.callback, struct{}{}); err != nil {
			b.Fatal(err)
		}
	}
	ready, err := engine.ReadRequest(ctx)
	if err != nil {
		b.Fatal(err)
	}
	if ready.Method != "ze-plugin-engine:ready" {
		b.Fatalf("final startup request = %q", ready.Method)
	}
	if err := engine.SendOK(ctx, ready.ID); err != nil {
		b.Fatal(err)
	}
	return engine
}

// slowPathWorker judges route i once per signal on start, as one peer's
// session goroutine judges what it receives.
func slowPathWorker(r *Reactor, peer *Peer, addr netip.Addr, body []byte, start <-chan struct{}, done chan<- ingressStepResult) {
	for range start {
		done <- r.runIngressPolicyChain(peer, addr, nil, testWireUpdate(body), body)
	}
}

func benchFilterSlowPath(b *testing.B, n int) {
	transport := newForkedBatchTransport(startForkedFilterPlugin(b), n)
	r := &Reactor{attrModHandlers: attrModHandlersWithDefaults(), filterTransportSeam: transport}
	policy := answerPolicy(b, routepolicy.FinalAccept, "set-local-pref")

	starts := make([]chan struct{}, n)
	done := make(chan ingressStepResult, n)
	for i := range n {
		addr := netip.AddrFrom4([4]byte{198, 51, 100, byte(i + 1)})
		peer := NewPeer(&PeerSettings{Address: addr, LocalAS: 65000, PeerAS: 65001, ImportPolicy: policy})
		starts[i] = make(chan struct{}, 1)
		go slowPathWorker(r, peer, addr, slowPathPayload(i), starts[i], done)
	}
	b.Cleanup(func() {
		for _, start := range starts {
			close(start)
		}
	})
	batch := func() []ingressStepResult {
		for _, start := range starts {
			start <- struct{}{}
		}
		results := make([]ingressStepResult, 0, n)
		for range n {
			results = append(results, <-done)
		}
		return results
	}

	// Once, untimed: every route is modified to LOCAL_PREF 200, so the loop
	// cannot be measuring an accept or a refusal.
	for i, res := range batch() {
		if !res.accept {
			b.Fatalf("route %d was not accepted", i)
		}
		pref, ok := slowPathLocalPref(res.modifiedPayload)
		if !ok || pref != 200 {
			b.Fatalf("route %d rebuilt with LOCAL_PREF %d (present %v), want 200", i, pref, ok)
		}
	}

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	b.ReportAllocs()
	for b.Loop() {
		for _, res := range batch() {
			if res.modifiedPayload == nil {
				b.Fatal("a route was not modified")
			}
		}
	}
	runtime.ReadMemStats(&after)
	routes := float64(b.N * n)
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/routes, "ns/route")
	b.ReportMetric(float64(after.TotalAlloc-before.TotalAlloc)/routes, "B/route")
	b.ReportMetric(float64(after.Mallocs-before.Mallocs)/routes, "allocs/route")
}
