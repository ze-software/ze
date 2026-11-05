// Design: docs/plugin-development/protocol.md -- The filter-update batch.
package ipc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// largeFilterRoute carries 16,300 valid IPv4 /24 NLRIs in both their raw UPDATE
// body and declared prefix field. The wire UPDATE fits BGP's extended maximum;
// a queue of 64 such independently valid filter inputs exceeds the RPC frame.
func largeFilterRoute() rpc.FilterUpdateInput {
	body := []byte{0, 0, 0, 20,
		0x40, 1, 1, 0,
		0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe9,
		0x40, 3, 4, 192, 0, 2, 1}
	fields := []byte(`{"prefix":{"ipv4/unicast":[`)
	for i := range 16300 {
		body = append(body, 24, 10, byte(i/256), byte(i%256))
		if i > 0 {
			fields = append(fields, ',')
		}
		fields = append(fields, `"10.`...)
		fields = strconv.AppendInt(fields, int64(i/256), 10)
		fields = append(fields, '.')
		fields = strconv.AppendInt(fields, int64(i%256), 10)
		fields = append(fields, `.0/24"`...)
	}
	fields = append(fields, `]}}`...)
	return rpc.FilterUpdateInput{Filter: "large", Direction: "import", Peer: "192.0.2.4", Fields: fields, Raw: body}
}

// byteBatchPeer reads actual framed requests and checks both payload fidelity
// and FIFO order, then supplies each route's own MED verdict. It runs for one
// test connection lifecycle and reports completion through the buffered result.
func byteBatchPeer(ctx context.Context, peer *PluginConn, inputs []rpc.FilterUpdateInput, result chan<- error) {
	seen := 0
	batches := 0
	for seen < len(inputs) {
		request, err := peer.ReadRequest(ctx)
		if err != nil {
			result <- fmt.Errorf("read split batch: %w", err)
			return
		}
		if request.Method != methodFilterUpdate {
			result <- fmt.Errorf("batch method %q, want filter-update", request.Method)
			return
		}
		if len(rpc.AppendRequest(nil, request.ID, request.Method, request.Params)) > rpc.MaxMessageSize {
			result <- fmt.Errorf("batch frame exceeds %d bytes", rpc.MaxMessageSize)
			return
		}
		var batch rpc.FilterUpdateBatchInput
		if err := json.Unmarshal(request.Params, &batch); err != nil {
			result <- err
			return
		}
		if len(batch.Routes) == 0 || len(batch.Routes) > rpc.FilterUpdateBatchMax {
			result <- fmt.Errorf("batch carries %d routes", len(batch.Routes))
			return
		}
		out := rpc.FilterUpdateBatchOutput{Verdicts: make([]rpc.FilterUpdateOutput, len(batch.Routes))}
		for i := range batch.Routes {
			index := seen + i
			if index >= len(inputs) {
				result <- fmt.Errorf("unexpected route at index %d", index)
				return
			}
			got, want := &batch.Routes[i], &inputs[index]
			if got.PeerAS != want.PeerAS || !bytes.Equal(got.Fields, want.Fields) || !bytes.Equal(got.Raw, want.Raw) {
				result <- fmt.Errorf("route %d changed or crossed FIFO position: peer-as=%d, want %d", index, got.PeerAS, want.PeerAS)
				return
			}
			out.Verdicts[i] = rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{{
				Field: "med", Action: rpc.FilterSetActionSet, Value: json.RawMessage(strconv.Itoa(int(want.PeerAS))),
			}}}
		}
		if err := peer.SendResult(ctx, request.ID, out); err != nil {
			result <- err
			return
		}
		seen += len(batch.Routes)
		batches++
	}
	if batches < 2 {
		result <- fmt.Errorf("oversized queue used %d exchanges, want at least two", batches)
		return
	}
	result <- nil
}

// TestFilterBatchSplitsAtActualFrameLimit holds the first exchange in flight,
// deterministically admits 64 valid large routes behind it, then releases it.
// Actual IPC framing must split those routes without loss, reordering, payload
// changes or verdict-index contamination; a route-count cap alone fails them.
func TestFilterBatchSplitsAtActualFrameLimit(t *testing.T) {
	engine, peer := newTestPluginConn(t)
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	first := make(chan filterReply, 1)
	go func() {
		out, err := engine.SendFilterUpdate(ctx, &rpc.FilterUpdateInput{Filter: "blocker"})
		first <- filterReply{out: out, err: err}
	}()
	request, err := peer.ReadRequest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	inputs := make([]rpc.FilterUpdateInput, rpc.FilterUpdateBatchMax)
	requests := make([]filterRequest, len(inputs))
	large := largeFilterRoute()
	if len(large.Raw)+19 > 65535 {
		t.Fatal("fixture exceeds BGP's extended message limit")
	}
	for i := range inputs {
		inputs[i] = large
		inputs[i].PeerAS = uint32(i + 1)
		requests[i] = filterRequest{input: &inputs[i], reply: make(chan filterReply, 1)}
		if err := engine.filters.admit(ctx, &requests[i]); err != nil {
			t.Fatal(err)
		}
	}
	encoded, err := json.Marshal(&rpc.FilterUpdateBatchInput{Routes: inputs})
	if err != nil {
		t.Fatal(err)
	}
	if len(rpc.AppendRequest(nil, request.ID+1, methodFilterUpdate, encoded)) <= rpc.MaxMessageSize {
		t.Fatal("fixture's 64 routes do not exceed the actual request frame limit")
	}
	if len(engine.filters.queue) != len(inputs) {
		t.Fatalf("queued routes=%d, want the fenced group of %d", len(engine.filters.queue), len(inputs))
	}
	peerResult := make(chan error, 1)
	go byteBatchPeer(ctx, peer, inputs, peerResult)
	if err := peer.SendResult(ctx, request.ID, rpc.FilterUpdateBatchOutput{Verdicts: []rpc.FilterUpdateOutput{{Action: rpc.FilterAccept}}}); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-first:
		if got.err != nil || got.out == nil || got.out.Action != rpc.FilterAccept {
			t.Fatalf("blocker verdict=%+v error=%v", got.out, got.err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	for i := range requests {
		select {
		case got := <-requests[i].reply:
			if got.err != nil {
				t.Fatalf("individually valid route %d failed instead of splitting the batch: %v", i, got.err)
			}
			if got.out == nil || got.out.Action != rpc.FilterModify || len(got.out.Sets) != 1 || string(got.out.Sets[0].Value) != strconv.Itoa(i+1) {
				t.Fatalf("route %d got another route's verdict: %+v", i, got.out)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	select {
	case err := <-peerResult:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

// TestFilterBatchOversizedRouteDoesNotFailNeighbor queues an individually
// unencodable input beside a small valid input. Only the oversized route may
// fail: its neighbor must still cross the actual transport and receive its own
// verdict, rather than inherit the frame refusal of the combined batch.
func TestFilterBatchOversizedRouteDoesNotFailNeighbor(t *testing.T) {
	engine, peer := newTestPluginConn(t)
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	first := make(chan filterReply, 1)
	go func() {
		out, err := engine.SendFilterUpdate(ctx, &rpc.FilterUpdateInput{Filter: "blocker"})
		first <- filterReply{out: out, err: err}
	}()
	request, err := peer.ReadRequest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	inputs := []rpc.FilterUpdateInput{
		{Filter: "oversized", Raw: make([]byte, rpc.MaxMessageSize)},
		{Filter: "small", PeerAS: 7, Fields: json.RawMessage(`{"med":20}`), Raw: []byte{0, 0, 0, 0}},
	}
	requests := make([]filterRequest, len(inputs))
	for i := range inputs {
		requests[i] = filterRequest{input: &inputs[i], reply: make(chan filterReply, 1)}
		if err := engine.filters.admit(ctx, &requests[i]); err != nil {
			t.Fatal(err)
		}
	}
	peerResult := make(chan error, 1)
	go func() {
		next, err := peer.ReadRequest(ctx)
		if err != nil {
			peerResult <- err
			return
		}
		var batch rpc.FilterUpdateBatchInput
		if err := json.Unmarshal(next.Params, &batch); err != nil {
			peerResult <- err
			return
		}
		if len(batch.Routes) != 1 || batch.Routes[0].Filter != "small" || !bytes.Equal(batch.Routes[0].Fields, inputs[1].Fields) || !bytes.Equal(batch.Routes[0].Raw, inputs[1].Raw) {
			peerResult <- fmt.Errorf("oversized route contaminated its neighbor's exchange: %+v", batch.Routes)
			return
		}
		peerResult <- peer.SendResult(ctx, next.ID, rpc.FilterUpdateBatchOutput{Verdicts: []rpc.FilterUpdateOutput{{Action: rpc.FilterReject}}})
	}()
	if err := peer.SendResult(ctx, request.ID, rpc.FilterUpdateBatchOutput{Verdicts: []rpc.FilterUpdateOutput{{Action: rpc.FilterAccept}}}); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-first:
		if got.err != nil || got.out == nil || got.out.Action != rpc.FilterAccept {
			t.Fatalf("blocker verdict=%+v error=%v", got.out, got.err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	for i := range requests {
		select {
		case got := <-requests[i].reply:
			if i == 0 {
				if got.err == nil || got.out != nil {
					t.Fatalf("oversized route verdict=%+v error=%v, want its own refusal", got.out, got.err)
				}
			} else if got.err != nil || got.out == nil || got.out.Action != rpc.FilterReject {
				t.Fatalf("small neighbor verdict=%+v error=%v, want its own policy reject", got.out, got.err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	select {
	case err := <-peerResult:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

// TestFilterBatchBudgetMatchesCanonicalFrame compares the measured fixed
// budget with the actual appender at the widest request ID, including nonempty
// params and commas. Ordinary inputs must also retain the no-preencoding path.
func TestFilterBatchBudgetMatchesCanonicalFrame(t *testing.T) {
	routes := []rpc.FilterUpdateInput{
		{Filter: "first", Direction: "import", Peer: "192.0.2.4", PeerAS: 65001,
			Fields: json.RawMessage(`{"text":"<>&"}`), Raw: []byte{0, 0, 0, 0}},
		{Filter: "second", Direction: "export", Peer: "192.0.2.5", PeerAS: 65002},
	}
	for count := 1; count <= len(routes); count++ {
		batch := rpc.FilterUpdateBatchInput{Routes: routes[:count]}
		encoded, err := json.Marshal(&batch)
		if err != nil {
			t.Fatal(err)
		}
		wantSize := len(rpc.AppendRequest(nil, math.MaxUint64, methodFilterUpdate, encoded))
		size := filterBatchWireFixed + count - 1
		var worker filterBatcher
		for i := range count {
			input, err := json.Marshal(&routes[i])
			if err != nil {
				t.Fatal(err)
			}
			size += len(input)
			worker.requests = append(worker.requests, &filterRequest{input: &routes[i]})
		}
		if size != wantSize {
			t.Fatalf("%d-route budget=%d, actual widest-ID request=%d", count, size, wantSize)
		}
		if !worker.frameFits() {
			t.Fatalf("%d ordinary routes unnecessarily require exact preencoding", count)
		}
	}
}

// TestFilterBatchSingletonUsesActualRequestID places a canonical singleton
// one byte beyond the conservative maximum-ID budget but within the actual
// first request's frame. Both serial and muxed transports must deliver it
// unchanged, return its verdict and remain usable for the next small route.
func TestFilterBatchSingletonUsesActualRequestID(t *testing.T) {
	input := rpc.FilterUpdateInput{Filter: "boundary", Fields: json.RawMessage(`{"peer-name":""}`)}
	encoded, err := json.Marshal(&input)
	if err != nil {
		t.Fatal(err)
	}
	target := rpc.MaxMessageSize - filterBatchWireFixed + 1
	padding := target - len(encoded)
	fields := []byte(`{"peer-name":"`)
	fields = append(fields, bytes.Repeat([]byte{'x'}, padding)...)
	fields = append(fields, '"', '}')
	input.Fields = fields
	encoded, err = json.Marshal(&rpc.FilterUpdateBatchInput{Routes: []rpc.FilterUpdateInput{input}})
	if err != nil {
		t.Fatal(err)
	}
	if size := len(rpc.AppendRequest(nil, math.MaxUint64, methodFilterUpdate, encoded)); size != rpc.MaxMessageSize+1 {
		t.Fatalf("widest-ID boundary frame=%d, want exactly one byte beyond limit", size)
	}
	if size := len(rpc.AppendRequest(nil, 1, methodFilterUpdate, encoded)); size > rpc.MaxMessageSize {
		t.Fatalf("actual first-ID frame=%d, want it to fit", size)
	}
	for _, muxed := range []bool{false, true} {
		name := "serial"
		if muxed {
			name = "mux"
		}
		t.Run(name, func(t *testing.T) {
			engine, peer := newTestPluginConn(t)
			if muxed {
				engine = NewMuxPluginConn(rpc.NewMuxConn(engine.Conn))
			}
			t.Cleanup(func() {
				if err := engine.Close(); err != nil {
					t.Error(err)
				}
			})
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			peerResult := make(chan error, 1)
			go func() {
				for index := range 2 {
					request, err := peer.ReadRequest(ctx)
					if err != nil {
						peerResult <- err
						return
					}
					var batch rpc.FilterUpdateBatchInput
					if err := json.Unmarshal(request.Params, &batch); err != nil {
						peerResult <- err
						return
					}
					if len(batch.Routes) != 1 {
						peerResult <- fmt.Errorf("boundary exchange carries %d routes", len(batch.Routes))
						return
					}
					if index == 0 {
						if request.ID != 1 || !bytes.Equal(batch.Routes[0].Fields, input.Fields) {
							peerResult <- fmt.Errorf("first-ID boundary route changed: id=%d fields=%d", request.ID, len(batch.Routes[0].Fields))
							return
						}
					} else if batch.Routes[0].Filter != "following" {
						peerResult <- fmt.Errorf("following route changed: %q", batch.Routes[0].Filter)
						return
					}
					err = peer.SendResult(ctx, request.ID, rpc.FilterUpdateBatchOutput{Verdicts: []rpc.FilterUpdateOutput{{Action: rpc.FilterReject}}})
					if err != nil {
						peerResult <- err
						return
					}
				}
				peerResult <- nil
			}()
			for _, route := range []*rpc.FilterUpdateInput{&input, {Filter: "following"}} {
				out, err := engine.SendFilterUpdate(ctx, route)
				if err != nil || out == nil || out.Action != rpc.FilterReject {
					t.Fatalf("fitting singleton %q verdict=%+v error=%v, want its policy reject", route.Filter, out, err)
				}
			}
			select {
			case err := <-peerResult:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		})
	}
}
