// Design: docs/architecture/testing/interop.md -- checker and cleanup verdicts.

package interoplab

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"reflect"
	"strings"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// TestPassedAssertionReportsCleanupFailure drives Suite.Run through the real
// scenario teardown and checks its status, progress, text and structured answer.
func TestPassedAssertionReportsCleanupFailure(t *testing.T) {
	for _, checkerFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "passed", true: "failed"}[checkerFails], func(t *testing.T) {
			runner := &recordingRunner{run: kernelAnswering(nil)}
			var progress bytes.Buffer
			checks, cleanups := 0, 0
			suite := Suite{
				Docker: newDocker(runner), StagedZe: kernelCheckZe, NoBuild: true, Progress: &progress,
				Scenarios: []ScenarioPlan{{
					Source: ScenarioSource{Name: "cleanup-proof", Checker: func(context.Context, *CheckContext) error {
						checks++
						if checkerFails {
							return errors.New("checker failure")
						}
						return nil
					}},
					Network:    NetworkSpec{Name: "lab-net", Candidates: []Subnet{{IPv4: netip.MustParsePrefix("172.29.0.0/24")}}},
					Containers: []string{"lab-peer"},
					Prepare: func(context.Context, PrepareContext) (PreparedScenario, error) {
						return PreparedScenario{
							Peers: []PeerConfig{{Name: "peer", Container: "lab-peer", Image: "peer-image", Host: 3}},
							Cleanup: func() error {
								cleanups++
								return errors.New("owned teardown refused")
							},
						}, nil
					},
				}},
			}
			area := leaction.New("test integration", leaction.Action{
				Verb: "cleanup-proof", Why: "Exercise cleanup reporting.",
				Answer: func() (any, int) {
					report := suite.Run(t.Context())
					return report, report.Code
				},
			})
			answer, code := area.AnswerOrSweep([]string{"cleanup-proof"}, leaction.RunEveryAction)
			report, ok := answer.(SuiteReport)
			if !ok {
				t.Fatalf("single action answer = %T, want SuiteReport", answer)
			}
			if code != 1 {
				t.Errorf("single action code = %d, want 1", code)
			}
			if checks != 1 || cleanups != 1 || len(report.Scenarios) != 1 {
				t.Fatalf("checks=%d cleanups=%d report=%#v", checks, cleanups, report)
			}
			if report.Code != 1 || report.SetupError != "" {
				t.Errorf("cleanup must fail the overall run: %#v", report)
			}
			result := report.Scenarios[0]
			if result.Passed == checkerFails || !reflect.DeepEqual(result.CleanupErrors, []string{"owned teardown refused"}) {
				t.Errorf("checker/cleanup verdicts changed: %#v", result)
			}
			if checkerFails {
				if report.Passed != 0 || report.Failed != 1 || !reflect.DeepEqual(report.FailedNames, []string{"cleanup-proof"}) || result.Error != "checker failure" {
					t.Errorf("failed checker changed: %#v", report)
				}
			} else if report.Passed != 1 || report.Failed != 0 || len(report.FailedNames) != 0 || result.Error != "" {
				t.Errorf("passed checker changed: %#v", report)
			}
			for name, text := range map[string]string{"text": report.Text(), "progress": progress.String()} {
				if !strings.Contains(text, "cleanup-proof: cleanup: owned teardown refused") {
					t.Errorf("%s hides cleanup failure: %s", name, text)
				}
			}
			encoded, err := json.Marshal(report)
			if err != nil {
				t.Fatal(err)
			}
			// Decode independently of the report types so a removed JSON field
			// cannot disappear from both sides of the assertion.
			var decoded map[string]any
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatal(err)
			}
			want := map[string]any{
				"scenarios": []any{map[string]any{
					"name":           "cleanup-proof",
					"passed":         true,
					"cleanup-errors": []any{"owned teardown refused"},
				}},
				"passed": float64(1),
				"failed": float64(0),
				"code":   float64(1),
			}
			if checkerFails {
				want = map[string]any{
					"scenarios": []any{map[string]any{
						"name":           "cleanup-proof",
						"passed":         false,
						"error":          "checker failure",
						"cleanup-errors": []any{"owned teardown refused"},
					}},
					"passed":       float64(0),
					"failed":       float64(1),
					"failed-names": []any{"cleanup-proof"},
					"code":         float64(1),
				}
			}
			if !reflect.DeepEqual(decoded, want) {
				t.Fatalf("structured report = %s, want %#v", encoded, want)
			}
			answer, code = area.Sweep([]string{"cleanup-proof"}, leaction.RunEveryAction)
			sweep, ok := answer.(leaction.Sweep)
			if !ok {
				t.Fatalf("combined action answer = %T, want Sweep", answer)
			}
			if code != 1 || !reflect.DeepEqual(sweep.Failed, []string{"cleanup-proof"}) {
				t.Errorf("combined action lost failure: code=%d answer=%#v", code, sweep)
			}
			if !strings.Contains(sweep.Text(), "cleanup-proof: cleanup: owned teardown refused") {
				t.Errorf("combined action hides cleanup failure: %s", sweep.Text())
			}
		})
	}
}
