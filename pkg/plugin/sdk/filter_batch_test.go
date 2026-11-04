package sdk

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestFilterUpdateAnswersOneVerdictPerRoute proves the SDK half of D-3: a
// filter-update exchange carrying several routes calls the handler once per
// route, in request order, and answers one verdict per route at the same
// position; a handler error fails its own route (a verdict carrying the
// error), never the whole exchange. Method: call the registered callback with
// a batch of three, the middle one failing.
func TestFilterUpdateAnswersOneVerdictPerRoute(t *testing.T) {
	p, _ := newTestPair(t)
	var seen []string
	p.OnFilter(func(route *FilterRoute) (*FilterUpdateOutput, error) {
		seen = append(seen, route.Peer())
		if route.Peer() == "b" {
			return nil, errors.New("no data for b")
		}
		if route.Peer() == "a" {
			return &FilterUpdateOutput{Action: rpc.FilterAccept}, nil
		}
		return &FilterUpdateOutput{Action: rpc.FilterReject}, nil
	})

	params, err := json.Marshal(rpc.FilterUpdateBatchInput{Routes: []rpc.FilterUpdateInput{
		{Filter: "f", Peer: "a"}, {Filter: "f", Peer: "b"}, {Filter: "f", Peer: "c"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	answer, err := p.callbacks[callbackFilterUpdate](params)
	if err != nil {
		t.Fatalf("a batch with one failing route failed as a whole: %v", err)
	}
	var out rpc.FilterUpdateBatchOutput
	if err := json.Unmarshal(answer, &out); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 3 || seen[0] != "a" || seen[1] != "b" || seen[2] != "c" {
		t.Fatalf("handler called for %v, want a, b, c in order", seen)
	}
	if len(out.Verdicts) != 3 {
		t.Fatalf("%d verdicts for 3 routes", len(out.Verdicts))
	}
	if out.Verdicts[0].Action != rpc.FilterAccept || out.Verdicts[0].Error != "" {
		t.Fatalf("verdict a = %+v, want accept", out.Verdicts[0])
	}
	if out.Verdicts[1].Error != "no data for b" {
		t.Fatalf("verdict b = %+v, want its own error", out.Verdicts[1])
	}
	if out.Verdicts[2].Action != rpc.FilterReject || out.Verdicts[2].Error != "" {
		t.Fatalf("verdict c = %+v, want reject", out.Verdicts[2])
	}
}

// TestFilterBatchAllocationBudget bounds allocation pressure for a full batch
// of header-only filters. The evaluator borrows each route, checks its header
// and returns an existing verdict; total allocation pressure, including the
// verdict codec, must stay below two allocations per route while preserving
// all alternating verdicts in order.
func TestFilterBatchAllocationBudget(t *testing.T) {
	input := rpc.FilterUpdateBatchInput{Routes: make([]rpc.FilterUpdateInput, rpc.FilterUpdateBatchMax)}
	for i := range input.Routes {
		input.Routes[i] = rpc.FilterUpdateInput{Filter: "f", PeerAS: uint32(i + 1)}
	}
	params, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	accept := FilterUpdateOutput{Action: rpc.FilterAccept}
	reject := FilterUpdateOutput{Action: rpc.FilterReject}
	seen := 0
	eval := func(route *FilterRoute) (*FilterUpdateOutput, error) {
		seen++
		if route.PeerAS() != uint32(seen) {
			return nil, errors.New("batch route header changed")
		}
		if seen%2 == 0 {
			return &reject, nil
		}
		return &accept, nil
	}
	var answer json.RawMessage
	var callErr error
	allocs := testing.AllocsPerRun(100, func() {
		seen = 0
		answer, callErr = answerFilterBatch(eval, params)
	})
	if callErr != nil {
		t.Fatal(callErr)
	}
	if seen != len(input.Routes) {
		t.Fatalf("judged %d routes, want %d", seen, len(input.Routes))
	}
	var out rpc.FilterUpdateBatchOutput
	if err := json.Unmarshal(answer, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Verdicts) != len(input.Routes) {
		t.Fatalf("%d verdicts for %d routes", len(out.Verdicts), len(input.Routes))
	}
	for i := range out.Verdicts {
		want := rpc.FilterAccept
		if i%2 == 1 {
			want = rpc.FilterReject
		}
		if out.Verdicts[i].Action != want {
			t.Fatalf("route %d action = %v, want %v", i, out.Verdicts[i].Action, want)
		}
		if out.Verdicts[i].Error != "" {
			t.Fatalf("route %d error = %q", i, out.Verdicts[i].Error)
		}
	}
	if allocs >= 2*float64(rpc.FilterUpdateBatchMax) {
		t.Fatalf("full header-only batch allocated %.0f objects, want fewer than %d", allocs, 2*rpc.FilterUpdateBatchMax)
	}
}
