// Design: docs/plugin-development/handlers.md — Route filters (the fast-path contract)
// Related: sdk_callbacks.go — OnFilter, which installs the evaluator on both transports

package sdk

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// FilterCallDeadline is the per-call deadline of the conformance harness: a
// fast-path evaluator runs on the engine's goroutines, so one call past it
// fails the filter's test (FR-9).
const FilterCallDeadline = 50 * time.Millisecond

// filterConcurrency is how many goroutines the harness calls an evaluator
// from at once, each judging every route.
const filterConcurrency = 8

// ConformanceReporter is the part of testing.TB the harness reports through.
type ConformanceReporter interface {
	Helper()
	Errorf(format string, args ...any)
}

// ConformanceRoute is one route the harness judges: Input names the header,
// and its Fields and Raw are what the fast path's view answers. Engine is the
// engine's typed view the fast path hands over, nil for a filter that reads
// only Fields and Raw.
type ConformanceRoute struct {
	Input  rpc.FilterUpdateInput
	Engine any
}

// CheckFilterConformance runs eval through the clauses of the fast-path
// contract (FR-9) and both transports (FR-8), and reports each broken clause
// naming the filter and the clause. It answers whether every clause held.
// Every fast-path filter's test MUST pass it, under -race: the concurrent
// clause is what lets the race detector see an evaluator that writes shared
// state without a snapshot.
//
// The clauses, in order: each call returns within FilterCallDeadline; a
// repeat call answers what the first did; no call reads a view after its
// return, and poisoning the bytes of the views it read before changes no
// answer; the
// forked transport answers what the fast path did; and concurrent calls
// answer what sequential ones did. A slice kept and never read again changes
// no answer, so no harness sees it.
func CheckFilterConformance(r ConformanceReporter, name string, eval FilterEvaluator, routes []ConformanceRoute) bool {
	r.Helper()
	reference := make([]string, len(routes))
	for i := range routes {
		answer, ok := timedFastCall(eval, &routes[i])
		if !ok {
			r.Errorf("filter %s breaks the non-blocking clause: route %d ran past the %v per-call deadline",
				name, i, FilterCallDeadline)
			return false
		}
		reference[i] = answer
	}
	for i := range routes {
		if again := fastCall(eval, &routes[i], nil); again != reference[i] {
			r.Errorf("filter %s breaks the snapshot clause: a repeat call on route %d answered %s after %s",
				name, i, again, reference[i])
			return false
		}
	}
	var kept atomic.Bool
	held := true
	for i := range routes {
		if answer := fastCall(eval, &routes[i], &kept); answer != reference[i] {
			r.Errorf("filter %s breaks the borrowed-view clause: route %d answered %s once the views it read before were poisoned, %s before",
				name, i, answer, reference[i])
			held = false
		}
	}
	if kept.Load() {
		r.Errorf("filter %s breaks the borrowed-view clause: it read a view after its call returned", name)
		held = false
	}
	if !held {
		return false
	}
	for i := range routes {
		if forked := forkedCall(eval, &routes[i]); forked != reference[i] {
			r.Errorf("filter %s answers route %d differently by transport: fast path %s, forked %s",
				name, i, reference[i], forked)
			held = false
		}
	}
	if !held {
		return false
	}
	return concurrentCalls(r, name, eval, routes, reference)
}

// concurrentCalls judges every route from filterConcurrency goroutines at once.
func concurrentCalls(r ConformanceReporter, name string, eval FilterEvaluator, routes []ConformanceRoute, reference []string) bool {
	r.Helper()
	var wg sync.WaitGroup
	var mismatch atomic.Int64
	mismatch.Store(-1)
	for range filterConcurrency {
		wg.Go(func() {
			for i := range routes {
				if fastCall(eval, &routes[i], nil) != reference[i] {
					mismatch.Store(int64(i))
				}
			}
		})
	}
	wg.Wait()
	if i := mismatch.Load(); i >= 0 {
		r.Errorf("filter %s breaks the concurrent-call clause: route %d answered otherwise under concurrent calls", name, i)
		return false
	}
	return true
}

// conformanceView is the fast-path view the harness hands an evaluator. Its
// fields and raw body are copies the harness poisons once the call returns,
// so a slice the evaluator kept reads the poison on its next use.
type conformanceView struct {
	fields  []byte
	raw     []byte
	engine  any
	expired atomic.Bool
	kept    *atomic.Bool
}

func (v *conformanceView) touch() {
	if v.expired.Load() && v.kept != nil {
		v.kept.Store(true)
	}
}

func (v *conformanceView) Fields() ([]byte, error) { v.touch(); return v.fields, nil }
func (v *conformanceView) Raw() []byte             { v.touch(); return v.raw }
func (v *conformanceView) Engine() any             { v.touch(); return v.engine }

// poison overwrites the view's bytes and marks it expired.
func (v *conformanceView) poison() {
	v.expired.Store(true)
	for i := range v.fields {
		v.fields[i] = 'x'
	}
	for i := range v.raw {
		v.raw[i] = 0xff
	}
}

// fastCall judges route on the fast path. With kept set, the view is
// poisoned after the call and kept, so a later call that reads it is seen.
func fastCall(eval FilterEvaluator, route *ConformanceRoute, kept *atomic.Bool) string {
	view := &conformanceView{
		fields: append([]byte(nil), route.Input.Fields...),
		raw:    append([]byte(nil), route.Input.Raw...),
		engine: route.Engine,
		kept:   kept,
	}
	header := route.Input
	call := rpc.ViewFilterRoute(&header, view)
	out, err := eval(&call)
	if kept != nil {
		view.poison()
	}
	return answerText(out, err)
}

// timedFastCall is fastCall bounded by FilterCallDeadline. A call past it is
// left running: the harness reports it and does not wait.
func timedFastCall(eval FilterEvaluator, route *ConformanceRoute) (string, bool) {
	done := make(chan string, 1)
	go runFastCall(eval, route, done)
	timer := time.NewTimer(FilterCallDeadline)
	defer timer.Stop()
	select {
	case answer := <-done:
		return answer, true
	case <-timer.C:
		return "", false
	}
}

func runFastCall(eval FilterEvaluator, route *ConformanceRoute, done chan<- string) {
	done <- fastCall(eval, route, nil)
}

// forkedCall judges route the way a forked plugin does: one filter-update
// batch marshaled, answered by OnFilter's batch handler, and decoded.
func forkedCall(eval FilterEvaluator, route *ConformanceRoute) string {
	params, err := json.Marshal(rpc.FilterUpdateBatchInput{Routes: []rpc.FilterUpdateInput{route.Input}})
	if err != nil {
		return answerText(nil, err)
	}
	answer, err := answerFilterBatch(eval, params)
	if err != nil {
		return answerText(nil, err)
	}
	var batch rpc.FilterUpdateBatchOutput
	if err := json.Unmarshal(answer, &batch); err != nil {
		return answerText(nil, err)
	}
	if len(batch.Verdicts) != 1 {
		return "a batch of one answered " + string(answer)
	}
	verdict := batch.Verdicts[0]
	if verdict.Error != "" {
		return "error: " + verdict.Error
	}
	return answerText(&verdict, nil)
}

// answerText renders a verdict for comparison: the forked transport's JSON
// form, so both transports compare on what the engine reads.
func answerText(out *rpc.FilterUpdateOutput, err error) string {
	if err != nil {
		return "error: " + err.Error()
	}
	if out == nil {
		return "error: the filter handler answered no verdict"
	}
	text, err := json.Marshal(out)
	if err != nil {
		return "error: " + err.Error()
	}
	return string(text)
}
