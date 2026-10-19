package interoplab

import (
	"context"
	"net/netip"
	"strings"
	"testing"
)

// VALIDATES: a peer whose run fails before docker runs (here an address the
// lab cannot compute) records no container for teardown, while the network
// this run did create is still removed.
// PREVENTS: teardown rm -f'ing a name this run never created.
func TestRunScenarioPreRunFailureRemovesNoContainer(t *testing.T) {
	runner := &recordingRunner{}
	runner.run = kernelAnswering(func(ProcessCommand) (ProcessResult, error) { return ProcessResult{}, nil })
	suite := Suite{
		Docker: newDocker(runner), StagedZe: kernelCheckZe, NoBuild: true,
		Scenarios: []ScenarioPlan{{
			Source:  ScenarioSource{Name: "one", Directory: "/scenarios/one", Checker: func(context.Context, *CheckContext) error { return nil }},
			Network: NetworkSpec{Name: "lab-net", Candidates: []Subnet{{IPv4: netip.MustParsePrefix("172.29.0.0/24")}}},
			Peers:   []PeerConfig{{Name: "peer", Container: "lab-peer", Image: "peer-image", Host: 0}},
		}},
	}
	if report := suite.Run(t.Context()); report.Code != 1 {
		t.Fatalf("Suite code = %d, want 1: %#v", report.Code, report)
	}
	joined := recordedCommands(runner.commands)
	if strings.Contains(joined, "docker rm -f lab-peer") {
		t.Errorf("teardown removed a container docker never ran:\n%s", joined)
	}
	if !strings.Contains(joined, "docker network rm lab-net") {
		t.Errorf("teardown never removed the network this run created:\n%s", joined)
	}
}
