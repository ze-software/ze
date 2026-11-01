// Design: docs/architecture/api/process-protocol.md — the filter-update batch
// Related: rpc.go — PluginConn, CallRPC

package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const methodFilterUpdate = "ze-plugin-callback:filter-update"

// filterQueueDepth bounds the routes waiting for a plugin's next filter-update
// exchange. A caller that finds it full waits for room (the worker drains it a
// batch at a time), which holds the producing goroutine back rather than
// dropping a route.
const filterQueueDepth = 4 * rpc.FilterUpdateBatchMax

var errFilterConnClosed = errors.New("filter-update: the plugin connection closed before the route was judged")

// filterBatcher sends the routes queued for one plugin in filter-update
// exchanges of up to rpc.FilterUpdateBatchMax routes (D-3). One worker
// goroutine per plugin connection owns the exchanges: the first route starts
// it, PluginConn.Close stops it, and it answers every route it was given
// before it exits. Safe for concurrent use.
type filterBatcher struct {
	start sync.Once
	// mu orders admission against close: a route enters the queue under the
	// read lock and close takes the write lock, so no route enters after the
	// worker's final drain.
	mu     sync.RWMutex
	closed bool
	queue  chan *filterRequest
	done   chan struct{}
	// routes and requests are the worker's per-exchange scratch.
	routes   []rpc.FilterUpdateInput
	requests []*filterRequest
}

// filterRequest is one route waiting for its verdict. reply is buffered, so
// the worker never blocks answering it.
type filterRequest struct {
	input *rpc.FilterUpdateInput
	reply chan filterReply
}

type filterReply struct {
	out *rpc.FilterUpdateOutput
	err error
}

var filterRequests = sync.Pool{New: func() any {
	return &filterRequest{reply: make(chan filterReply, 1)}
}}

// SendFilterUpdate sends input to the plugin in its next filter-update
// exchange and answers the route's verdict. ctx bounds only the wait for a
// place in the queue: once queued, the route is answered within the exchange
// in flight plus its own, each bounded by rpc.FilterUpdateTimeout, and input
// is not read after the return. A failed exchange (timeout, transport error,
// malformed reply) answers its error for every route of the batch; a verdict
// carrying Error answers that error for its route alone.
func (pc *PluginConn) SendFilterUpdate(ctx context.Context, input *rpc.FilterUpdateInput) (*rpc.FilterUpdateOutput, error) {
	b := &pc.filters
	b.start.Do(func() {
		b.queue = make(chan *filterRequest, filterQueueDepth)
		b.done = make(chan struct{})
		b.routes = make([]rpc.FilterUpdateInput, 0, rpc.FilterUpdateBatchMax)
		b.requests = make([]*filterRequest, 0, rpc.FilterUpdateBatchMax)
		go b.run(pc)
	})

	req, ok := filterRequests.Get().(*filterRequest)
	if !ok {
		return nil, errors.New("BUG: filter-update: the request pool holds another type")
	}
	req.input = input
	if err := b.admit(ctx, req); err != nil {
		req.input = nil
		filterRequests.Put(req)
		return nil, err
	}

	reply := <-req.reply
	req.input = nil
	filterRequests.Put(req)
	return reply.out, reply.err
}

// admit queues req, or answers why it cannot.
func (b *filterBatcher) admit(ctx context.Context, req *filterRequest) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.closed {
		return errFilterConnClosed
	}
	select {
	case b.queue <- req:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("filter-update: waiting for room in the plugin's queue: %w", ctx.Err())
	}
}

// close stops the worker: a route not yet queued is refused, and every route
// already queued is answered errFilterConnClosed. A batcher that never
// started has no worker, and only refuses.
func (b *filterBatcher) close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	if b.done != nil {
		close(b.done)
	}
}

// run is the worker loop: it waits for a route, takes every route queued
// behind it up to the bound, and exchanges them. It ends when close is
// called, after answering what is still queued.
func (b *filterBatcher) run(pc *PluginConn) {
	for {
		select {
		case first := <-b.queue:
			b.requests = append(b.requests[:0], first)
			b.takeQueued()
			b.exchange(pc)
		case <-b.done:
			b.answerQueued()
			return
		}
	}
}

// takeQueued adds the routes already queued, up to rpc.FilterUpdateBatchMax,
// without waiting for more: a lone route goes at once.
func (b *filterBatcher) takeQueued() {
	for len(b.requests) < rpc.FilterUpdateBatchMax {
		select {
		case req := <-b.queue:
			b.requests = append(b.requests, req)
		default:
			return
		}
	}
}

// answerQueued answers every queued route after close. close holds the
// write lock while it closes done, so no route is queued after it, and this
// drain empties the queue for good.
func (b *filterBatcher) answerQueued() {
	for {
		select {
		case req := <-b.queue:
			req.reply <- filterReply{err: errFilterConnClosed}
		default:
			return
		}
	}
}

// exchange sends the taken routes as one filter-update under one
// rpc.FilterUpdateTimeout and answers each request.
func (b *filterBatcher) exchange(pc *PluginConn) {
	b.routes = b.routes[:0]
	for _, req := range b.requests {
		b.routes = append(b.routes, *req.input)
	}
	observeFilterBatch(len(b.routes))
	ctx, cancel := context.WithTimeout(context.Background(), rpc.FilterUpdateTimeout)
	result, err := pc.CallRPC(ctx, methodFilterUpdate, &rpc.FilterUpdateBatchInput{Routes: b.routes})
	cancel()
	// The routes borrow the callers' bytes: drop them before any caller returns.
	clear(b.routes)
	if err != nil {
		b.answerAll(fmt.Errorf("filter-update: the batch of %d routes failed: %w", len(b.requests), err))
		return
	}
	var out rpc.FilterUpdateBatchOutput
	if err := json.Unmarshal(result, &out); err != nil {
		b.answerAll(fmt.Errorf("filter-update: the reply to a batch of %d routes is not valid: %w", len(b.requests), err))
		return
	}
	if len(out.Verdicts) != len(b.requests) {
		b.answerAll(errors.New("filter-update: the reply carried " + strconv.Itoa(len(out.Verdicts)) +
			" verdicts for a batch of " + strconv.Itoa(len(b.requests)) +
			" routes, so none is applied; the plugin must answer one verdict per route, in request order"))
		return
	}
	for i, req := range b.requests {
		verdict := &out.Verdicts[i]
		if verdict.Error != "" {
			req.reply <- filterReply{err: errors.New("filter-update: the filter could not judge the route: " + verdict.Error)}
			continue
		}
		req.reply <- filterReply{out: verdict}
	}
	clear(b.requests)
}

// answerAll answers err for every route of the failed exchange.
func (b *filterBatcher) answerAll(err error) {
	for _, req := range b.requests {
		req.reply <- filterReply{err: err}
	}
	clear(b.requests)
}
