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
	p.OnFilterUpdate(func(input *FilterUpdateInput) (*FilterUpdateOutput, error) {
		seen = append(seen, input.Peer)
		if input.Peer == "b" {
			return nil, errors.New("no data for b")
		}
		if input.Peer == "a" {
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
