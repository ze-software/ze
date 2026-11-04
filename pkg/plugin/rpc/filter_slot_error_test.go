// Design: docs/plugin-development/protocol.md -- A route the filter cannot judge.
package rpc

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestBridgeFilterSlotErrorOverridesVerdictFields proves an explicit route
// failure wins over every other verdict field, including raw ownership and
// size processing. A failed verdict must never reach a policy consumer.
func TestBridgeFilterSlotErrorOverridesVerdictFields(t *testing.T) {
	const failure = "filter data is unavailable"
	for _, test := range []struct {
		name string
		out  FilterUpdateOutput
	}{
		{"accept", FilterUpdateOutput{Action: FilterAccept, Error: failure}},
		{"modify", FilterUpdateOutput{Action: FilterModify, Error: failure,
			Sets: []FilterSet{{Field: "local-preference", Action: FilterSetActionSet, Value: json.RawMessage("200")}}}},
		{"teardown", FilterUpdateOutput{Action: FilterAccept, Error: failure, Teardown: true, NotifyCode: 6, NotifySubcode: 3}},
		{"raw before ownership", FilterUpdateOutput{Action: FilterModify, Error: failure, Raw: make([]byte, MaxMessageSize+1)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			bridge := NewDirectBridge()
			bridge.SetFilterSlot(func(*FilterRoute) (*FilterUpdateOutput, error) {
				return &test.out, nil
			})
			bridge.SetReady()
			t.Cleanup(bridge.CloseCallbacks)
			route := InputFilterRoute(&FilterUpdateInput{Filter: "judge"})
			out, err := bridge.CallFilterSlot(&route)
			if out != nil || err == nil || !strings.Contains(err.Error(), failure) {
				t.Fatalf("failed verdict present=%v error=%v, want only the declared route failure", out != nil, err)
			}
		})
	}
}
