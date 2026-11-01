package sdk

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestFilterUpdateAdapterBoundsSlowHandler proves the OnFilterUpdate adapter
// bounds each call by the filter-update deadline (R-8): a handler that
// blocks, as filter_irr's first-resolution wait does, fails the route that
// is waiting on it and every route queued behind it once the deadline
// passes, rather than holding the engine's goroutine until the handler
// returns. Method: shorten the deadline, block the handler on its first
// call, and time two concurrent callers; then release it and prove the next
// call is answered, so the serialized handler is not lost.
func TestFilterUpdateAdapterBoundsSlowHandler(t *testing.T) {
	saved := filterUpdateWait
	filterUpdateWait = 100 * time.Millisecond
	t.Cleanup(func() { filterUpdateWait = saved })

	p, _, bridge := newBridgedTestPair(t)
	release := make(chan struct{})
	var first sync.Once
	p.OnFilterUpdate(func(in *FilterUpdateInput) (*FilterUpdateOutput, error) {
		blocked := false
		first.Do(func() { blocked = true })
		if blocked {
			<-release
		}
		return &FilterUpdateOutput{Action: rpc.FilterAccept}, nil
	})
	p.activateBridge()
	t.Cleanup(func() { p.Close() }) //nolint:errcheck // test cleanup

	// The handler is released well after the deadline, so an unbounded
	// caller waits for it and the elapsed bound below goes red.
	releaseAfter := time.AfterFunc(2*time.Second, func() { close(release) })
	t.Cleanup(func() { releaseAfter.Stop() })

	call := func() (time.Duration, error) {
		header := rpc.FilterUpdateInput{Filter: "slow"}
		route := rpc.ViewFilterRoute(&header, evaluatorView{})
		start := time.Now()
		_, err := bridge.CallFilterSlot(&route)
		return time.Since(start), err
	}

	type result struct {
		elapsed time.Duration
		err     error
	}
	results := make(chan result, 2)
	go func() { // test helper: the caller the slow handler holds
		elapsed, err := call()
		results <- result{elapsed, err}
	}()
	time.Sleep(20 * time.Millisecond) // let the first call reach the handler
	go func() {                       // test helper: a caller queued behind it
		elapsed, err := call()
		results <- result{elapsed, err}
	}()

	for range 2 {
		r := <-results
		if r.err == nil {
			t.Fatalf("a call held past the deadline answered a verdict after %v", r.elapsed)
		}
		if r.elapsed > time.Second {
			t.Fatalf("a call waited %v, past the %v deadline: %v", r.elapsed, filterUpdateWait, r.err)
		}
	}

	if !releaseAfter.Stop() {
		t.Fatal("the release fired before both callers answered")
	}
	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for {
		_, err := call()
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the handler never answered again after its release: %v", err)
		}
	}
}

// TestFilterUpdateAdapterAfterClose proves a call through the adapter after
// the plugin closed answers rpc.ErrBridgeClosed, which the engine reports as
// "unavailable" and judges under the filter's on-error.
func TestFilterUpdateAdapterAfterClose(t *testing.T) {
	p, _, _ := newBridgedTestPair(t)
	p.OnFilterUpdate(func(in *FilterUpdateInput) (*FilterUpdateOutput, error) {
		return &FilterUpdateOutput{Action: rpc.FilterAccept}, nil
	})
	eval := p.onFilter
	header := rpc.FilterUpdateInput{Filter: "f"}
	route := rpc.InputFilterRoute(&header)
	if _, err := eval(&route); err != nil {
		t.Fatalf("call before close: %v", err)
	}
	p.Close() //nolint:errcheck // the test closes to observe the after-close answer
	if _, err := eval(&route); !errors.Is(err, rpc.ErrBridgeClosed) {
		t.Fatalf("call after close answered %v, want rpc.ErrBridgeClosed", err)
	}
}
