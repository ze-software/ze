// Design: docs/architecture/plugin/rib-storage-design.md -- pipeline command refusals
package rib

import (
	"strings"
	"testing"
)

// TestPipelineParseErrorsReachCommandVerdict drives both registered command
// handlers: an invalid pipeline must be refused, not returned as successful data.
// VALIDATES: spec-refuse-instead-of-defaulting AC-13, pipeline parse-error row.
// PREVENTS: a parser diagnostic hidden in an error-shaped payload with status done.
func TestPipelineParseErrorsReachCommandVerdict(t *testing.T) {
	for _, command := range []string{"show bgp rib", "show bgp rib best"} {
		t.Run(command, func(t *testing.T) {
			r := newTestRIBManager(t)
			for _, bad := range []struct {
				args []string
				want string
			}{
				{[]string{"unrecognized-stage"}, "unknown keyword: unrecognized-stage"},
				{[]string{"peer"}, "peer requires a value"},
			} {
				status, data, err := r.handleCommand(command, "*", bad.args)
				if status != statusError {
					t.Errorf("%v: status = %q, want error", bad.args, status)
				}
				if err == nil {
					t.Errorf("%v: no refusal reason; data = %#v", bad.args, data)
					continue
				}
				if !strings.Contains(err.Error(), bad.want) {
					t.Errorf("%v: reason = %q, want %q", bad.args, err, bad.want)
				}
				if data != "" {
					t.Errorf("%v: refusal carries successful data %#v", bad.args, data)
				}
			}
			status, _, err := r.handleCommand(command, "*", []string{"count"})
			if status != statusDone || err != nil {
				t.Errorf("valid count: status = %q, error = %v", status, err)
			}
		})
	}
}
