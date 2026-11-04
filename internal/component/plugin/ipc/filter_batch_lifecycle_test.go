package ipc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestFilterBatchCloseBeforeFirstSend proves a stale PluginConn refuses routes
// without allocating a batch queue, scratch, or a worker after Close.
func TestFilterBatchCloseBeforeFirstSend(t *testing.T) {
	engine, _ := newTestPluginConn(t)
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := engine.SendFilterUpdate(ctx, &rpc.FilterUpdateInput{Filter: "f"})
	if !errors.Is(err, errFilterConnClosed) {
		t.Fatalf("send after Close = %+v, %v; want connection-closed refusal", out, err)
	}
	if out != nil {
		t.Fatalf("send after Close returned verdict %+v", out)
	}
	b := &engine.filters
	// Stop a leaked worker on the unfixed producer, without hiding the failure.
	if b.done != nil {
		select {
		case <-b.done:
		default:
			close(b.done)
		}
	}
	if b.queue != nil {
		t.Error("send after Close allocated a persistent queue")
	}
	if b.done != nil {
		t.Error("send after Close started a worker lifecycle")
	}
	if b.routes != nil {
		t.Error("send after Close allocated route scratch")
	}
	if b.requests != nil {
		t.Error("send after Close allocated request scratch")
	}
}

// TestFilterBatchFirstSendConcurrentClose races the first route against the real
// PluginConn.Close entry point. Both calls must settle, any started lifecycle
// must be stopped, and a later route must never enter the final drain.
func TestFilterBatchFirstSendConcurrentClose(t *testing.T) {
	for attempt := range 128 {
		engine, _ := newTestPluginConn(t)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		start := make(chan struct{})
		result := make(chan filterReply, 1)
		closed := make(chan error, 1)
		go func() {
			<-start
			out, err := engine.SendFilterUpdate(ctx, &rpc.FilterUpdateInput{Filter: "f"})
			result <- filterReply{out: out, err: err}
		}()
		go func() {
			<-start
			closed <- engine.Close()
		}()
		close(start)
		select {
		case err := <-closed:
			if err != nil {
				t.Fatalf("attempt %d: Close: %v", attempt, err)
			}
		case <-ctx.Done():
			cancel()
			t.Fatalf("attempt %d: Close did not settle", attempt)
		}
		select {
		case got := <-result:
			if got.err == nil {
				t.Fatalf("attempt %d: closed transport returned verdict %+v", attempt, got.out)
			}
		case <-ctx.Done():
			cancel()
			t.Fatalf("attempt %d: first send did not settle", attempt)
		}
		if done := engine.filters.done; done != nil {
			select {
			case <-done:
			default:
				// Both public calls have returned, so cleanup owns this signal.
				close(done)
				t.Errorf("attempt %d: first send left a worker running after Close", attempt)
			}
		}
		out, err := engine.SendFilterUpdate(ctx, &rpc.FilterUpdateInput{Filter: "f"})
		if !errors.Is(err, errFilterConnClosed) {
			t.Fatalf("attempt %d: late send = %+v, %v; want connection-closed refusal", attempt, out, err)
		}
		if got := len(engine.filters.queue); got != 0 {
			t.Fatalf("attempt %d: %d routes remain queued after settlement", attempt, got)
		}
		cancel()
	}
}

// TestFilterBatchCloseSettlesWorker proves an initialized worker returns its
// positive verdict, settles every admitted request at Close, and actually exits.
// The fixture wraps the existing worker only to observe its terminal return.
func TestFilterBatchCloseSettlesWorker(t *testing.T) {
	engine, plugin := newTestPluginConn(t)
	b := &engine.filters
	exited := make(chan struct{})
	b.start.Do(func() {
		b.queue = make(chan *filterRequest, filterQueueDepth)
		b.done = make(chan struct{})
		b.routes = make([]rpc.FilterUpdateInput, 0, rpc.FilterUpdateBatchMax)
		b.requests = make([]*filterRequest, 0, rpc.FilterUpdateBatchMax)
		go func() {
			b.run(engine)
			close(exited)
		}()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
		select {
		case <-exited:
		case <-time.After(5 * time.Second):
			t.Error("filter worker did not exit during cleanup")
		}
	})
	result := make(chan filterReply, 1)
	send := func() {
		out, err := engine.SendFilterUpdate(ctx, &rpc.FilterUpdateInput{Filter: "f", Peer: "positive"})
		result <- filterReply{out: out, err: err}
	}
	go send()
	req, err := plugin.ReadRequest(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := plugin.SendResult(ctx, req.ID, rpc.FilterUpdateBatchOutput{
		Verdicts: []rpc.FilterUpdateOutput{{Action: rpc.FilterAccept}},
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-result:
		if got.err != nil {
			t.Fatal(got.err)
		}
		if got.out == nil {
			t.Fatal("admitted route received no verdict")
		}
		if got.out.Action != rpc.FilterAccept {
			t.Fatalf("admitted verdict = %+v; want accept", got.out)
		}
	case <-ctx.Done():
		t.Fatal("positive route did not settle")
	}

	// Hold an exchange in flight, so the requests below are certainly queued.
	go send()
	if _, err := plugin.ReadRequest(ctx); err != nil {
		t.Fatal(err)
	}
	queued := make([]filterRequest, 3)
	for i := range queued {
		queued[i].input = &rpc.FilterUpdateInput{Filter: "f"}
		queued[i].reply = make(chan filterReply, 1)
		if err := b.admit(ctx, &queued[i]); err != nil {
			t.Fatal(err)
		}
	}
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-result:
		if got.err == nil {
			t.Fatalf("in-flight route survived Close with %+v", got.out)
		}
	case <-ctx.Done():
		t.Fatal("in-flight route did not settle at Close")
	}
	for i := range queued {
		select {
		case got := <-queued[i].reply:
			if got.err == nil {
				t.Fatalf("queued route %d survived Close with %+v", i, got.out)
			}
		case <-ctx.Done():
			t.Fatalf("queued route %d did not settle at Close", i)
		}
	}
	select {
	case <-exited:
	case <-ctx.Done():
		t.Fatal("worker did not exit after settling its queue")
	}
	if out, err := engine.SendFilterUpdate(ctx, &rpc.FilterUpdateInput{Filter: "f"}); !errors.Is(err, errFilterConnClosed) {
		t.Fatalf("send after worker exit = %+v, %v; want connection-closed refusal", out, err)
	}
	if got := len(b.queue); got != 0 {
		t.Fatalf("final drain left %d queued routes", got)
	}
}
