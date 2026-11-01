package sdk

import (
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// evaluatorView is the engine side of a fast-path call in these tests.
type evaluatorView struct{}

func (evaluatorView) Fields() ([]byte, error) { return []byte(`{"med":7}`), nil }
func (evaluatorView) Raw() []byte             { return nil }
func (evaluatorView) Engine() any             { return nil }

// medEvaluator rejects a route whose fields carry med 7 and accepts others.
func medEvaluator(route *FilterRoute) (*FilterUpdateOutput, error) {
	fields, err := route.Fields()
	if err != nil {
		return nil, err
	}
	var read struct {
		MED uint32 `json:"med"`
	}
	if len(fields) > 0 {
		if err := json.Unmarshal(fields, &read); err != nil {
			return nil, err
		}
	}
	if read.MED == 7 {
		return &FilterUpdateOutput{Action: rpc.FilterReject}, nil
	}
	return &FilterUpdateOutput{Action: rpc.FilterAccept}, nil
}

// TestSDKInstallsEvaluatorPerTransport proves FR-8: one OnFilter
// registration becomes the typed bridge slot when the plugin runs internal
// and the filter-update batch handler when it runs forked, and both judge the
// same route the same way. Method: register on a bridged plugin, activate its
// bridge as Stage 5 does, call the slot; register on a plain plugin, call the
// batch callback with the same fields.
func TestSDKInstallsEvaluatorPerTransport(t *testing.T) {
	internal, _, bridge := newBridgedTestPair(t)
	if !internal.IsInternal() {
		t.Fatal("bridged test plugin is not internal")
	}
	internal.OnFilter(medEvaluator)
	if bridge.HasFilterSlot() {
		t.Fatal("the slot was installed before Stage 5 (FR-11)")
	}
	internal.activateBridge()
	if !bridge.HasFilterSlot() {
		t.Fatal("OnFilter on an internal plugin installed no filter slot")
	}
	header := rpc.FilterUpdateInput{Filter: "f", Direction: "import", Peer: "p"}
	route := rpc.ViewFilterRoute(&header, evaluatorView{})
	fast, err := bridge.CallFilterSlot(&route)
	if err != nil {
		t.Fatal(err)
	}

	forked, _ := newTestPair(t)
	if forked.IsInternal() {
		t.Fatal("plain test plugin is internal")
	}
	forked.OnFilter(medEvaluator)
	params, err := json.Marshal(rpc.FilterUpdateBatchInput{Routes: []rpc.FilterUpdateInput{
		{Filter: "f", Direction: "import", Peer: "p", Fields: json.RawMessage(`{"med":7}`)},
	}})
	if err != nil {
		t.Fatal(err)
	}
	answer, err := forked.callbacks[callbackFilterUpdate](params)
	if err != nil {
		t.Fatal(err)
	}
	var batch rpc.FilterUpdateBatchOutput
	if err := json.Unmarshal(answer, &batch); err != nil {
		t.Fatal(err)
	}
	if len(batch.Verdicts) != 1 {
		t.Fatalf("%d verdicts", len(batch.Verdicts))
	}
	if fast.Action != rpc.FilterReject || batch.Verdicts[0].Action != rpc.FilterReject {
		t.Fatalf("fast %v, forked %v: want reject on both transports", fast.Action, batch.Verdicts[0].Action)
	}
}

// TestFilterUpdateAdapterSerializesAndEncodes proves OnFilterUpdate is the
// adapter over OnFilter: on the fast path its handler receives the route as
// one input with the view's fields encoded.
func TestFilterUpdateAdapterSerializesAndEncodes(t *testing.T) {
	p, _, bridge := newBridgedTestPair(t)
	var got string
	p.OnFilterUpdate(func(in *FilterUpdateInput) (*FilterUpdateOutput, error) {
		got = string(in.Fields)
		return &FilterUpdateOutput{Action: rpc.FilterAccept}, nil
	})
	p.activateBridge()
	header := rpc.FilterUpdateInput{Filter: "f"}
	route := rpc.ViewFilterRoute(&header, evaluatorView{})
	if _, err := bridge.CallFilterSlot(&route); err != nil {
		t.Fatal(err)
	}
	if got != `{"med":7}` {
		t.Fatalf("handler saw fields %q", got)
	}
}
