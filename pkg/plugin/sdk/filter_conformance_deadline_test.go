// Design: docs/plugin-development/handlers.md — Route filters (the conformance harness)

package sdk

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestFastPathConformanceDeadlineEveryClause holds one exercised call until
// cleanup, proving that every clause refuses it without waiting for its return.
// The concurrent case also requires another invocation to run while it is held.
func TestFastPathConformanceDeadlineEveryClause(t *testing.T) {
	cases := []struct {
		name      string
		blockCall int64
	}{
		{name: "initial", blockCall: 1},
		{name: "repeat", blockCall: 2},
		{name: "borrowed", blockCall: 3},
		{name: "forked", blockCall: 4},
		{name: "concurrent", blockCall: 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			entered := make(chan struct{})
			release := make(chan struct{})
			returned := make(chan struct{})
			overlapped := make(chan struct{}, 1)
			finished := make(chan struct{})
			type result struct {
				ok     bool
				errors []string
			}
			answers := make(chan result, 1)
			var calls atomic.Int64
			var eval FilterEvaluator = func(*FilterRoute) (*FilterUpdateOutput, error) {
				call := calls.Add(1)
				if call == tc.blockCall {
					close(entered)
					<-release
					close(returned)
				}
				if tc.name == "concurrent" {
					if call > tc.blockCall {
						select {
						case overlapped <- struct{}{}:
						default:
						}
					}
				}
				return &FilterUpdateOutput{Action: rpc.FilterAccept}, nil
			}
			t.Cleanup(func() {
				close(release)
				if !conformanceDeadlineSignal(returned) {
					t.Error("the released evaluator did not return")
				}
				if !conformanceDeadlineSignal(finished) {
					t.Error("the conformance harness did not finish after cleanup released the evaluator")
				}
			})
			go func() {
				defer close(finished)
				var rep recordingReporter
				ok := CheckFilterConformance(&rep, tc.name, eval, []ConformanceRoute{
					{Input: rpc.FilterUpdateInput{Filter: "deadline"}},
				})
				answers <- result{ok: ok, errors: rep.errors}
			}()
			if !conformanceDeadlineSignal(entered) {
				t.Fatal("the harness did not exercise the selected evaluator call")
			}
			if tc.name == "concurrent" {
				if !conformanceDeadlineSignal(overlapped) {
					t.Fatal("no independent concurrent evaluation ran while the slow call was held")
				}
			}
			guard := time.NewTimer(5 * time.Second)
			defer guard.Stop()
			select {
			case answer := <-answers:
				if answer.ok {
					t.Fatal("the harness accepted an evaluator whose exercised call never returned")
				}
				joined := strings.Join(answer.errors, "\n")
				for _, want := range []string{"non-blocking clause", "route %d", "[\"" + tc.name + "\",0,"} {
					if !strings.Contains(joined, want) {
						t.Fatalf("deadline report %q does not name %q", joined, want)
					}
				}
			case <-guard.C:
				t.Fatal("the harness waited for a blocked evaluator instead of refusing the non-blocking clause")
			}
		})
	}
}

// conformanceDeadlineSignal bounds test coordination, not evaluator latency.
func conformanceDeadlineSignal(signal <-chan struct{}) bool {
	guard := time.NewTimer(5 * time.Second)
	defer guard.Stop()
	select {
	case <-signal:
		return true
	case <-guard.C:
		return false
	}
}
