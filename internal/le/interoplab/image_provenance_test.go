// Design: docs/architecture/testing/interop.md -- prove a scenario discriminates.
package interoplab

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestSuiteNarratesTheBuiltImageConsumedByPeers drives eager and selected
// image preparation through Suite.Run with a Docker builder that returns a
// full image ID. It requires the peer to consume that ID and the live progress
// to retain the same logical image name and ID beside the scenario verdict.
func TestSuiteNarratesTheBuiltImageConsumedByPeers(t *testing.T) {
	for _, selected := range []bool{false, true} {
		name := "eager"
		if selected {
			name = "selected"
		}
		t.Run(name, func(t *testing.T) {
			suite, runner := selectedImageFixture(t)
			suite.ImagesOnDemand = selected
			suite.Images = suite.Images[:2]
			imageID := "sha256:" + strings.Repeat("a1", 32)
			runner.run = kernelAnswering(func(command ProcessCommand) (ProcessResult, error) {
				if strings.HasPrefix(strings.Join(command.Arguments, " "), "docker build ") {
					return ProcessResult{Stdout: imageID + "\n"}, nil
				}
				return ProcessResult{}, nil
			})
			var progress bytes.Buffer
			suite.Progress = &progress
			checks := 0
			plan := selectedImagePlan("provenance", []PeerConfig{
				{Name: "ze", Container: "selected-ze", Image: "ze", Host: 2},
				{Name: "frr", Container: "selected-frr", Image: "pinned-frr", Host: 3},
			}, &checks)
			var duringCheck string
			plan.Source.Checker = func(context.Context, *CheckContext) error {
				checks++
				duringCheck = progress.String()
				return nil
			}
			suite.Scenarios = []ScenarioPlan{plan}

			report := suite.Run(t.Context())
			if report.Code != 0 {
				t.Fatalf("suite failed: %+v", report)
			}
			if checks != 1 {
				t.Fatalf("checker calls = %d, want 1", checks)
			}
			if report.Passed != 1 {
				t.Fatalf("passed scenarios = %d, want 1: %+v", report.Passed, report)
			}
			started := 0
			for _, command := range runner.commands {
				arguments := command.Arguments
				if !strings.HasPrefix(strings.Join(arguments, " "), "docker run -d ") {
					continue
				}
				want := imageID
				if strings.Contains(strings.Join(arguments, " "), "--name selected-frr ") {
					want = suite.Images[1].Tag
				}
				if image := arguments[len(arguments)-1]; image != want {
					t.Errorf("peer consumed image %q, want %q", image, want)
				}
				started++
			}
			if started != 2 {
				t.Fatalf("started peers = %d, want 2", started)
			}
			want := "interop: image ze: reference " + imageID + "\n"
			if !strings.Contains(duringCheck, want) {
				t.Errorf("live progress lacks the consumed image %q:\n%s", want, duringCheck)
			}
			if !strings.Contains(progress.String(), "interop: [1/1] provenance: pass (") {
				t.Errorf("progress lacks the scenario verdict:\n%s", progress.String())
			}
		})
	}
}
