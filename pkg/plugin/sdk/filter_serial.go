// Design: docs/plugin-development/handlers.md — Route filters (the OnFilterUpdate adapter)
// Related: sdk_callbacks.go — OnFilter and OnFilterUpdate

package sdk

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// filterUpdateWait bounds one call through the OnFilterUpdate adapter: the
// deadline of one filter-update exchange, which bounded the handler's caller
// when the handler ran on the plugin's callback loop. A test shortens it.
var filterUpdateWait = rpc.FilterUpdateTimeout

// filterSerializer runs a FilterUpdateHandler on one long-lived worker, so
// its calls are serialized as the plugin's callback loop serialized them
// before the fast path, while each caller waits at most filterUpdateWait. A
// handler that blocks (filter_irr's first-resolution wait) then fails the
// route that waits on it, and every route queued behind it, under the
// filter's on-error once the deadline passes, as an exchange past its
// deadline did; it never holds the engine's goroutine for longer (R-8).
//
// Lifecycle: the worker starts on the first call and stops when the plugin
// closes (stop, from Plugin.Close); a call after that answers
// rpc.ErrBridgeClosed. The request channel is unbuffered, so a caller whose
// deadline passes before the worker takes its route leaves nothing queued:
// its route is never judged late. Safe for concurrent use.
type filterSerializer struct {
	fn       FilterUpdateHandler
	requests chan *filterSerialCall
	done     chan struct{}
	start    sync.Once
	stopOnce sync.Once
	calls    sync.Pool
}

// filterSerialCall is one route handed to the worker. The worker writes
// output and err, then signals reply (capacity 1, so a caller that stopped
// waiting never blocks the worker).
type filterSerialCall struct {
	input  FilterUpdateInput
	output *FilterUpdateOutput
	err    error
	reply  chan struct{}
}

// newFilterSerializer returns the serializer for fn. MUST be stopped with
// stop when the plugin closes.
func newFilterSerializer(fn FilterUpdateHandler) *filterSerializer {
	s := &filterSerializer{
		fn:       fn,
		requests: make(chan *filterSerialCall),
		done:     make(chan struct{}),
	}
	s.calls.New = func() any { return &filterSerialCall{reply: make(chan struct{}, 1)} }
	return s
}

// call judges route through fn on the worker, waiting at most
// filterUpdateWait. The route's slices are borrowed for the engine's call
// only, and the worker may still run fn after this returns on expiry, so the
// input it hands over owns its bytes.
func (s *filterSerializer) call(route *FilterRoute) (*FilterUpdateOutput, error) {
	input, err := route.Input()
	if err != nil {
		return nil, err
	}
	input.Fields = bytes.Clone(input.Fields)
	input.Raw = bytes.Clone(input.Raw)

	s.start.Do(s.launch)
	c, ok := s.calls.Get().(*filterSerialCall)
	if !ok {
		panic("BUG: filterSerializer pool holds a value that is not a *filterSerialCall")
	}
	c.input = input

	timer := time.NewTimer(filterUpdateWait)
	defer timer.Stop()

	select {
	case s.requests <- c:
	case <-s.done:
		s.calls.Put(c)
		return nil, rpc.ErrBridgeClosed
	case <-timer.C:
		s.calls.Put(c)
		return nil, s.expired(route)
	}
	select {
	case <-c.reply:
		output, err := c.output, c.err
		*c = filterSerialCall{reply: c.reply}
		s.calls.Put(c)
		return output, err
	case <-s.done:
		return nil, rpc.ErrBridgeClosed // the worker owns c until it replies
	case <-timer.C:
		return nil, s.expired(route) // the worker owns c until it replies
	}
}

// expired answers a call whose deadline passed: the handler is still judging
// this route or one queued before it.
func (s *filterSerializer) expired(route *FilterRoute) error {
	return fmt.Errorf("filter %q gave no verdict within %v: its handler is still busy with an earlier route; the filter's on-error applies",
		route.Filter(), filterUpdateWait)
}

// launch starts the worker; the first call runs it once.
func (s *filterSerializer) launch() {
	go s.work()
}

// work is the worker: it judges one route at a time until stop. It loops
// until done closes, by design.
func (s *filterSerializer) work() {
	for {
		select {
		case <-s.done:
			return
		case c := <-s.requests:
			s.judge(c)
			c.reply <- struct{}{}
		}
	}
}

// judge runs fn for one route. A panic in fn fails that route alone, as a
// panic in a slot handler does (rpc.DirectBridge.CallFilterSlot): the worker
// runs off the engine's goroutine, so nothing else would recover it.
func (s *filterSerializer) judge(c *filterSerialCall) {
	defer func() {
		if r := recover(); r != nil {
			c.output, c.err = nil, fmt.Errorf("filter %q handler panicked: %v", c.input.Filter, r)
		}
	}()
	c.output, c.err = s.fn(&c.input)
}

// stop ends the worker once the route in hand, if any, is judged. Safe to
// call more than once.
func (s *filterSerializer) stop() {
	s.stopOnce.Do(func() { close(s.done) })
}
