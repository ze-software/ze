// Design: docs/plugin-development/protocol.md -- A route the filter cannot judge.
package sdk

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestConformanceComparesExplicitErrorConsumerSemantics proves that an
// error-bearing typed verdict and the same forked route failure are equivalent
// to the engine, regardless of ignored action, sets, raw or teardown fields.
// It drives the whole harness, including its concurrent and repeat-call checks.
func TestConformanceComparesExplicitErrorConsumerSemantics(t *testing.T) {
	const failure = "filter data is unavailable"
	for _, shape := range []struct {
		name string
		out  FilterUpdateOutput
	}{
		{"accept", FilterUpdateOutput{Action: rpc.FilterAccept, Error: failure}},
		{"modify", FilterUpdateOutput{Action: rpc.FilterModify, Error: failure,
			Sets: []rpc.FilterSet{{Field: "local-preference", Action: rpc.FilterSetActionSet, Value: json.RawMessage("200")}}}},
		{"raw", FilterUpdateOutput{Action: rpc.FilterModify, Error: failure, Raw: []byte{0, 0, 0, 0}}},
		{"teardown", FilterUpdateOutput{Action: rpc.FilterAccept, Error: failure, Teardown: true, NotifyCode: 6, NotifySubcode: 3}},
	} {
		t.Run(shape.name, func(t *testing.T) {
			eval := func(route *FilterRoute) (*FilterUpdateOutput, error) {
				if route.Engine() != nil {
					out := shape.out
					return &out, nil
				}
				return nil, errors.New(failure)
			}
			var reporter recordingReporter
			routes := []ConformanceRoute{{Input: rpc.FilterUpdateInput{Filter: "judge"}, Engine: struct{}{}}}
			if !CheckFilterConformance(&reporter, "error-"+shape.name, eval, routes) || len(reporter.errors) != 0 {
				t.Fatalf("equivalent explicit route failures were rejected: %v", reporter.errors)
			}
		})
	}
}

// TestConformanceStillRejectsSuccessfulTransportMismatch is the positive
// discrimination control: ignoring fields on failed verdicts must not hide
// different successful policy decisions between the two transports.
func TestConformanceStillRejectsSuccessfulTransportMismatch(t *testing.T) {
	eval := func(route *FilterRoute) (*FilterUpdateOutput, error) {
		if route.Engine() != nil {
			return &FilterUpdateOutput{Action: rpc.FilterAccept}, nil
		}
		return &FilterUpdateOutput{Action: rpc.FilterReject}, nil
	}
	var reporter recordingReporter
	routes := []ConformanceRoute{{Input: rpc.FilterUpdateInput{Filter: "judge"}, Engine: struct{}{}}}
	if CheckFilterConformance(&reporter, "successful-mismatch", eval, routes) {
		t.Fatal("different successful verdicts passed conformance")
	}
	if !strings.Contains(strings.Join(reporter.errors, "\n"), "differently by transport") {
		t.Fatalf("successful mismatch report=%v, want the transport difference", reporter.errors)
	}
}
