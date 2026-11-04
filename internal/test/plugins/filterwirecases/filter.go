// Design: docs/plugin-development/protocol.md -- The Route Fields and Sets of filter-update.
// Related: test/plugin/filter-wire-new-fields.ci and filter-wire-set-actions.ci.
// Package filterwirecases is a test-only evaluator linked by the zetest composition root.
package filterwirecases

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const filterWireCasesName = "filter-wire-cases"

// Both startup declarations MUST use this list, so config verification and
// the SDK's live declaration judge the same filters.
func filterWireCasesDecls() []sdk.FilterDecl {
	return []sdk.FilterDecl{
		{Name: "fields-all", Direction: sdk.FilterImport, Reads: []string{"atomic-aggregate", "aggregator", "originator-id", "cluster-list"}, OnError: sdk.OnErrorReject},
		{Name: "fields-only", Direction: sdk.FilterImport, Reads: []string{"atomic-aggregate"}, OnError: sdk.OnErrorReject},
		{Name: "actions", Direction: sdk.FilterImport, Reads: []string{"prefix", "community", "med", "local-preference"}, Sets: []string{"community", "med", "local-preference"}, OnError: sdk.OnErrorReject},
		{Name: "invalid", Direction: sdk.FilterImport, Reads: []string{"prefix", "med"}, Sets: []string{"med"}, OnError: sdk.OnErrorAccept},
		{Name: "check-invalid", Direction: sdk.FilterImport, Reads: []string{"prefix", "med", "local-preference"}, OnError: sdk.OnErrorReject},
		{Name: "check-batch", Direction: sdk.FilterImport, Reads: []string{"prefix", "med"}, OnError: sdk.OnErrorReject},
	}
}

// filterWireCasesRun installs the same evaluator for internal and forked runs.
func filterWireCasesRun(name string, conn net.Conn) int {
	p := sdk.NewWithConn(name, conn)
	p.OnFilter(filterWireCasesEvaluate)
	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, &sdk.Registration{Filters: filterWireCasesDecls()}); err != nil {
		filterWireCasesSaid("filter-wire-cases failed: ", []byte(err.Error()))
		return 1
	}
	return 0
}

// filterWireCasesEvaluate checks actual projected field values, not the
// declaration or a source-code token. Safe for concurrent use: no shared state.
func filterWireCasesEvaluate(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	if route.Filter() == "actions" {
		return filterWireCasesActions(), nil
	}
	if route.Filter() == "invalid" {
		return filterWireCasesInvalid(route)
	}
	if route.Filter() == "check-invalid" {
		return filterWireCasesCheckInvalid(route)
	}
	if route.Filter() == "check-batch" {
		return filterWireCasesCheckBatch(route)
	}
	raw, err := route.Fields()
	if err != nil {
		return nil, fmt.Errorf("read new fields: %w", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("decode new fields: %w", err)
	}
	wants := map[string]string{"atomic-aggregate": "true"}
	if route.Filter() == "fields-all" {
		wants["aggregator"] = `"65010:192.0.2.9"`
		wants["originator-id"] = `"192.0.2.8"`
		wants["cluster-list"] = `["192.0.2.6","192.0.2.7"]`
	}
	if len(fields) != len(wants) {
		return nil, fmt.Errorf("%s received %s: want exactly %d declared fields", route.Filter(), raw, len(wants))
	}
	for field, want := range wants {
		var gotValue, wantValue any
		if err := json.Unmarshal(fields[field], &gotValue); err != nil {
			return nil, fmt.Errorf("%s missing or invalid: %w", field, err)
		}
		if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
			return nil, fmt.Errorf("fixture expectation %s: %w", field, err)
		}
		gotJSON, err := json.Marshal(gotValue)
		if err != nil {
			return nil, fmt.Errorf("encode actual %s: %w", field, err)
		}
		wantJSON, err := json.Marshal(wantValue)
		if err != nil {
			return nil, fmt.Errorf("encode expected %s: %w", field, err)
		}
		if string(gotJSON) != string(wantJSON) {
			return nil, fmt.Errorf("%s is %s, want %s", field, gotJSON, wantJSON)
		}
	}
	transport := "batch"
	if route.Engine() != nil {
		transport = "slot"
	}
	filterWireCasesSaid("OK: new fields "+route.Filter()+" "+transport+" values=", raw)
	return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
}

// All actions fold in order: MED 40+9-3=46, LP 200+20-5=215,
// and community 65001:1 is replaced by 65000:99.
func filterWireCasesActions() *sdk.FilterUpdateOutput {
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Sets: []rpc.FilterSet{
		{Field: "community", Action: rpc.FilterSetActionAdd, Value: json.RawMessage(`"65000:99"`)},
		{Field: "community", Action: rpc.FilterSetActionRemove, Value: json.RawMessage(`"65001:1"`)},
		{Field: "med", Action: rpc.FilterSetActionSet, Value: json.RawMessage(`40`)},
		{Field: "med", Action: rpc.FilterSetActionIncrement, Value: json.RawMessage(`9`)},
		{Field: "med", Action: rpc.FilterSetActionDecrement, Value: json.RawMessage(`3`)},
		{Field: "local-preference", Action: rpc.FilterSetActionSet, Value: json.RawMessage(`200`)},
		{Field: "local-preference", Action: rpc.FilterSetActionIncrement, Value: json.RawMessage(`20`)},
		{Field: "local-preference", Action: rpc.FilterSetActionDecrement, Value: json.RawMessage(`5`)},
	}}
}

// A failed stderr write cannot be reported to a second output channel.
func filterWireCasesSaid(lead string, fields []byte) {
	var line textbuf.Buffer
	line.Str(lead).Str(string(fields)).Byte('\n')
	_ = line.StdErr() //nolint:errcheck // the test runner reads this process's stderr
}
