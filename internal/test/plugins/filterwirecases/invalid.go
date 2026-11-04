// Design: docs/plugin-development/protocol.md -- The Sets of a filter-update answer.
// Related: test/plugin/filter-wire-undeclared-set.ci -- refused-set isolation.
package filterwirecases

import (
	"encoding/json"
	"fmt"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The source changes NEXT_HOP per UPDATE so the session cannot coalesce the
// five cases. Every bad verdict starts with a valid MED set: the downstream
// checker proves that a later refusal rolls that first change back as well.
func filterWireCasesInvalid(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	prefix, _, _, err := filterWireCasesNumbers(route)
	if err != nil {
		return nil, err
	}
	bad := map[string]rpc.FilterSet{
		"10.0.1.0/24": {Field: "local-preference", Action: rpc.FilterSetActionSet, Value: json.RawMessage(`999`)},
		"10.0.2.0/24": {Field: "not-a-route-field", Action: rpc.FilterSetActionSet, Value: json.RawMessage(`999`)},
		"10.0.3.0/24": {Field: "prefix", Action: rpc.FilterSetActionSet, Value: json.RawMessage(`"10.9.0.0/24"`)},
		"10.0.4.0/24": {Field: "med", Action: rpc.FilterSetActionSet, Value: json.RawMessage(`{"not":"a number"}`)},
	}
	if prefix == "10.0.5.0/24" {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Sets: []rpc.FilterSet{{Field: "med", Action: rpc.FilterSetActionSet, Value: json.RawMessage(`9`)}}}, nil
	}
	set, exists := bad[prefix]
	if !exists {
		return nil, fmt.Errorf("invalid-set case for unexpected prefix %s", prefix)
	}
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Sets: []rpc.FilterSet{
		{Field: "med", Action: rpc.FilterSetActionSet, Value: json.RawMessage(`999`)}, set,
	}}, nil
}

func filterWireCasesCheckInvalid(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	prefix, med, local, err := filterWireCasesNumbers(route)
	if err != nil {
		return nil, err
	}
	want := uint32(7)
	if prefix == "10.0.5.0/24" {
		want = 9
	}
	if med != want {
		return nil, fmt.Errorf("%s MED is %d, want %d: refused sets partially applied or valid route lost its verdict", prefix, med, want)
	}
	if local != 100 {
		return nil, fmt.Errorf("%s LP is %d, want unchanged 100", prefix, local)
	}
	filterWireCasesSaid("OK: isolated set verdict "+prefix+" numeric fields checked", nil)
	return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
}

// filterWireCasesNumbers decodes only declared fields, without retaining the
// callback's buffer. A missing numeric field is an error, not zero.
func filterWireCasesNumbers(route *sdk.FilterRoute) (string, uint32, uint32, error) {
	raw, err := route.Fields()
	if err != nil {
		return "", 0, 0, err
	}
	var fields struct {
		Prefix map[string][]string `json:"prefix"`
		MED    *uint32             `json:"med"`
		Local  *uint32             `json:"local-preference"`
	}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return "", 0, 0, fmt.Errorf("decode set-case fields: %w", err)
	}
	prefixes := fields.Prefix["ipv4/unicast"]
	if len(prefixes) != 1 {
		return "", 0, 0, fmt.Errorf("set case needs exactly one prefix: %s", raw)
	}
	if fields.MED == nil {
		return "", 0, 0, fmt.Errorf("set case lacks MED: %s", raw)
	}
	local := uint32(0)
	if fields.Local != nil {
		local = *fields.Local
	}
	return prefixes[0], *fields.MED, local, nil
}
