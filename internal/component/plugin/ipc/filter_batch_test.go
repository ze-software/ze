package ipc

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// filterResult is what one SendFilterUpdate caller got back.
type filterResult struct {
	peer string
	out  *rpc.FilterUpdateOutput
	err  error
}

// sendQueued starts one caller per peer, in order, each only once the one
// before it is queued, so the queue holds them in exactly that order: one
// source producing routes in sequence. It answers their results in peer order.
func sendQueued(t *testing.T, conn *PluginConn, alreadyQueued int, peers []string) []chan filterResult {
	t.Helper()
	results := make([]chan filterResult, len(peers))
	for i, peer := range peers {
		results[i] = make(chan filterResult, 1)
		go func(peer string, result chan filterResult) {
			out, err := conn.SendFilterUpdate(context.Background(), &rpc.FilterUpdateInput{Filter: "f", Peer: peer})
			result <- filterResult{peer: peer, out: out, err: err}
		}(peer, results[i])
		waitQueued(t, conn, alreadyQueued+i+1)
	}
	return results
}

// waitQueued waits until n routes sit in conn's filter queue.
func waitQueued(t *testing.T, conn *PluginConn, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for len(conn.filters.queue) < n {
		if time.Now().After(deadline) {
			t.Fatalf("queue holds %d routes, want %d", len(conn.filters.queue), n)
		}
		time.Sleep(time.Millisecond)
	}
}

// readBatch reads one filter-update request on the plugin side.
func readBatch(t *testing.T, plugin *PluginConn) (uint64, rpc.FilterUpdateBatchInput) {
	t.Helper()
	req, err := plugin.ReadRequest(context.Background())
	if err != nil {
		t.Fatalf("read request: %v", err)
	}
	if req.Method != methodFilterUpdate {
		t.Fatalf("method = %q, want %q", req.Method, methodFilterUpdate)
	}
	var batch rpc.FilterUpdateBatchInput
	if err := json.Unmarshal(req.Params, &batch); err != nil {
		t.Fatalf("params: %v", err)
	}
	return req.ID, batch
}

func peersOf(batch rpc.FilterUpdateBatchInput) []string {
	peers := make([]string, len(batch.Routes))
	for i := range batch.Routes {
		peers[i] = batch.Routes[i].Peer
	}
	return peers
}

// verdictsFor answers each route with its own verdict: reject for an even
// index, modify with a med set of the index for an odd one.
func verdictsFor(batch rpc.FilterUpdateBatchInput) rpc.FilterUpdateBatchOutput {
	out := rpc.FilterUpdateBatchOutput{Verdicts: make([]rpc.FilterUpdateOutput, len(batch.Routes))}
	for i := range batch.Routes {
		if i%2 == 0 {
			out.Verdicts[i] = rpc.FilterUpdateOutput{Action: rpc.FilterReject}
			continue
		}
		out.Verdicts[i] = rpc.FilterUpdateOutput{Action: rpc.FilterModify, Sets: []rpc.FilterSet{{
			Field: "med", Action: rpc.FilterSetActionSet, Value: json.RawMessage(strconv.Itoa(i)),
		}}}
	}
	return out
}

func routePeers(prefix string, n int) []string {
	peers := make([]string, n)
	for i := range peers {
		peers[i] = prefix + strconv.Itoa(i)
	}
	return peers
}

// checkVerdicts asserts each caller got the verdict verdictsFor gave its
// position in the batch.
func checkVerdicts(t *testing.T, results []chan filterResult) {
	t.Helper()
	for i, result := range results {
		got := <-result
		if got.err != nil {
			t.Fatalf("route %s: %v", got.peer, got.err)
		}
		if i%2 == 0 {
			if got.out.Action != rpc.FilterReject || len(got.out.Sets) != 0 {
				t.Fatalf("route %s got %+v, want its own reject", got.peer, got.out)
			}
			continue
		}
		if got.out.Action != rpc.FilterModify || len(got.out.Sets) != 1 || string(got.out.Sets[0].Value) != strconv.Itoa(i) {
			t.Fatalf("route %s got %+v, want its own med set %d", got.peer, got.out, i)
		}
	}
}

// TestFilterBatchOrderAndVerdicts proves AC-12 at the plugin connection (D-3):
// routes one source queued while an exchange was in flight go together in the
// next exchange, in queue order, and each caller gets the verdict at its own
// position, with its own sets. Past rpc.FilterUpdateBatchMax they split into
// consecutive exchanges and the order holds across them. A lone route goes at
// once, in an exchange of one.
func TestFilterBatchOrderAndVerdicts(t *testing.T) {
	t.Parallel()
	engine, plugin := newTestPluginConn(t)

	// A lone route is sent at once, alone.
	first := make(chan filterResult, 1)
	go func() {
		out, err := engine.SendFilterUpdate(context.Background(), &rpc.FilterUpdateInput{Filter: "f", Peer: "lone"})
		first <- filterResult{peer: "lone", out: out, err: err}
	}()
	id, batch := readBatch(t, plugin)
	if got := peersOf(batch); len(got) != 1 || got[0] != "lone" {
		t.Fatalf("lone route sent as %v, want [lone]", got)
	}

	// Ten routes queue behind the exchange in flight.
	ten := routePeers("r", 10)
	results := sendQueued(t, engine, 0, ten)
	if err := plugin.SendResult(context.Background(), id, verdictsFor(batch)); err != nil {
		t.Fatal(err)
	}
	if got := <-first; got.err != nil || got.out.Action != rpc.FilterReject {
		t.Fatalf("lone route = %+v, want reject", got)
	}
	id, batch = readBatch(t, plugin)
	if got := peersOf(batch); strings.Join(got, ",") != strings.Join(ten, ",") {
		t.Fatalf("ten queued routes sent as %v, want one exchange %v", got, ten)
	}
	if err := plugin.SendResult(context.Background(), id, verdictsFor(batch)); err != nil {
		t.Fatal(err)
	}
	checkVerdicts(t, results)

	// Past the bound: a blocker in flight, then bound+6 routes queued.
	blocker := make(chan filterResult, 1)
	go func() {
		out, err := engine.SendFilterUpdate(context.Background(), &rpc.FilterUpdateInput{Filter: "f", Peer: "blocker"})
		blocker <- filterResult{out: out, err: err}
	}()
	id, batch = readBatch(t, plugin)
	many := routePeers("m", rpc.FilterUpdateBatchMax+6)
	results = sendQueued(t, engine, 0, many)
	if err := plugin.SendResult(context.Background(), id, verdictsFor(batch)); err != nil {
		t.Fatal(err)
	}
	<-blocker
	var sent []string
	var answers []rpc.FilterUpdateOutput
	for len(sent) < len(many) {
		id, batch = readBatch(t, plugin)
		if len(batch.Routes) > rpc.FilterUpdateBatchMax {
			t.Fatalf("an exchange carried %d routes, above the bound %d", len(batch.Routes), rpc.FilterUpdateBatchMax)
		}
		sent = append(sent, peersOf(batch)...)
		out := rpc.FilterUpdateBatchOutput{Verdicts: make([]rpc.FilterUpdateOutput, len(batch.Routes))}
		for i := range out.Verdicts {
			out.Verdicts[i] = rpc.FilterUpdateOutput{Action: rpc.FilterModify, Update: batch.Routes[i].Peer}
		}
		answers = append(answers, out.Verdicts...)
		if err := plugin.SendResult(context.Background(), id, out); err != nil {
			t.Fatal(err)
		}
	}
	if strings.Join(sent, ",") != strings.Join(many, ",") {
		t.Fatalf("routes past the bound sent in order %v, want %v", sent, many)
	}
	if len(answers) != len(many) {
		t.Fatalf("%d verdicts for %d routes", len(answers), len(many))
	}
	for _, result := range results {
		got := <-result
		if got.err != nil || got.out.Update != got.peer {
			t.Fatalf("route %s got %+v, %v; want the verdict naming it", got.peer, got.out, got.err)
		}
	}
}

// TestFilterBatchFailureAppliesOnError proves AC-19 and the batch half of
// AC-15 at the plugin connection (D-3): an exchange that fails as a whole (an
// error reply, a reply with fewer or more verdicts than routes) answers an
// error to every route of the batch, so each caller applies the filter's
// on-error to its own route, and no verdict of the malformed reply reaches a
// caller; a verdict carrying an error fails its route alone; the next batch is
// unaffected.
func TestFilterBatchFailureAppliesOnError(t *testing.T) {
	t.Parallel()
	engine, plugin := newTestPluginConn(t)

	failures := []struct {
		name   string
		answer func(id uint64, batch rpc.FilterUpdateBatchInput) error
	}{
		{"error reply", func(id uint64, _ rpc.FilterUpdateBatchInput) error {
			return plugin.SendError(context.Background(), id, "plugin broke")
		}},
		{"fewer verdicts", func(id uint64, batch rpc.FilterUpdateBatchInput) error {
			out := verdictsFor(batch)
			out.Verdicts = out.Verdicts[:len(out.Verdicts)-1]
			return plugin.SendResult(context.Background(), id, out)
		}},
		{"more verdicts", func(id uint64, batch rpc.FilterUpdateBatchInput) error {
			out := verdictsFor(batch)
			out.Verdicts = append(out.Verdicts, rpc.FilterUpdateOutput{Action: rpc.FilterAccept})
			return plugin.SendResult(context.Background(), id, out)
		}},
	}
	for _, failure := range failures {
		blocker := make(chan filterResult, 1)
		go func() {
			out, err := engine.SendFilterUpdate(context.Background(), &rpc.FilterUpdateInput{Filter: "f", Peer: "blocker"})
			blocker <- filterResult{out: out, err: err}
		}()
		id, batch := readBatch(t, plugin)
		results := sendQueued(t, engine, 0, routePeers("x", 3))
		if err := plugin.SendResult(context.Background(), id, verdictsFor(batch)); err != nil {
			t.Fatal(err)
		}
		<-blocker
		id, batch = readBatch(t, plugin)
		if len(batch.Routes) != 3 {
			t.Fatalf("%s: batch of %d routes, want 3", failure.name, len(batch.Routes))
		}
		if err := failure.answer(id, batch); err != nil {
			t.Fatal(err)
		}
		for _, result := range results {
			got := <-result
			if got.err == nil {
				t.Fatalf("%s: route %s got verdict %+v, want the batch's error", failure.name, got.peer, got.out)
			}
		}
	}

	// A verdict carrying an error fails its route alone, and the batch after
	// the failures above is judged normally.
	blocker := make(chan filterResult, 1)
	go func() {
		out, err := engine.SendFilterUpdate(context.Background(), &rpc.FilterUpdateInput{Filter: "f", Peer: "blocker"})
		blocker <- filterResult{out: out, err: err}
	}()
	id, batch := readBatch(t, plugin)
	results := sendQueued(t, engine, 0, routePeers("y", 3))
	if err := plugin.SendResult(context.Background(), id, verdictsFor(batch)); err != nil {
		t.Fatal(err)
	}
	if got := <-blocker; got.err != nil {
		t.Fatalf("the batch after the failures failed: %v", got.err)
	}
	id, batch = readBatch(t, plugin)
	out := verdictsFor(batch)
	out.Verdicts[1] = rpc.FilterUpdateOutput{Action: rpc.FilterReject, Error: "no data for this route"}
	if err := plugin.SendResult(context.Background(), id, out); err != nil {
		t.Fatal(err)
	}
	for i, result := range results {
		got := <-result
		if i == 1 {
			if got.err == nil || !strings.Contains(got.err.Error(), "no data for this route") {
				t.Fatalf("route %s = %+v, %v; want its own error", got.peer, got.out, got.err)
			}
			continue
		}
		if got.err != nil || got.out.Action != rpc.FilterReject {
			t.Fatalf("route %s = %+v, %v; want its own reject", got.peer, got.out, got.err)
		}
	}

	// Close answers a queued route, and refuses a new one.
	go func() {
		_, _ = engine.SendFilterUpdate(context.Background(), &rpc.FilterUpdateInput{Filter: "f", Peer: "inflight"}) //nolint:errcheck // the exchange fails at Close
	}()
	readBatch(t, plugin)
	queued := sendQueued(t, engine, 0, []string{"queued"})
	engine.Close() //nolint:errcheck // the test closes the engine side to stop the batcher
	if got := <-queued[0]; got.err == nil {
		t.Fatalf("a route queued at close got %+v, want an error", got.out)
	}
	if _, err := engine.SendFilterUpdate(context.Background(), &rpc.FilterUpdateInput{Filter: "f"}); err == nil {
		t.Fatal("a route sent after close was accepted")
	}
}
