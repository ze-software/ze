// Design: docs/architecture/testing/interop.md -- automatic cleanup ownership.

package interoplab

import (
	"context"
	"net/netip"
	"strings"
	"testing"
)

// TestSuiteRunPreservesOtherRunsResources checks the automatic suite entry
// against a dead same-host foreign owner, while proving that the checker runs
// and this scenario's own container and network are still removed.
// The ownership boundary is ai/META-DECISIONS.md, "How agents work".
func TestSuiteRunPreservesOtherRunsResources(t *testing.T) {
	for _, row := range []struct {
		name   string
		start  string
		mutate func(map[string]string)
	}{
		{name: "absent owner"},
		{name: "reused pid", start: "999"},
		{name: "previous boot", start: "100", mutate: func(labels map[string]string) {
			labels[labelBootID] = otherBootID
		}},
	} {
		t.Run(row.name, func(t *testing.T) {
			labels := otherRunLabels()
			if row.mutate != nil {
				row.mutate(labels)
			}
			runner := &recordingRunner{run: kernelAnswering(orphanDaemon(labels, func(command ProcessCommand) (ProcessResult, bool) {
				joined := strings.Join(command.Arguments, " ")
				if strings.Contains(joined, "--filter name=") {
					return ProcessResult{}, true
				}
				return ProcessResult{}, false
			}))}
			docker := newDocker(runner)
			docker.judge.lookup = lookupAnswering(row.start)
			checked := false
			suite := Suite{
				Docker:   docker,
				StagedZe: kernelCheckZe,
				NoBuild:  true,
				Scenarios: []ScenarioPlan{{
					Source: ScenarioSource{Name: "owned-scenario", Checker: func(context.Context, *CheckContext) error {
						checked = true
						return nil
					}},
					Network: NetworkSpec{Name: "this-run-network", Candidates: []Subnet{{IPv4: netip.MustParsePrefix("172.29.0.0/24")}}},
					Peers:   []PeerConfig{{Name: "peer", Container: "this-run-peer", Image: "peer-image", Host: 3}},
				}},
			}
			report := suite.Run(t.Context())
			if report.Code != 0 {
				t.Fatalf("Suite.Run code = %d: %#v", report.Code, report)
			}
			if !checked {
				t.Fatal("Suite.Run never reached the scenario checker")
			}
			commands := recordedCommands(runner.commands)
			for _, foreign := range []string{
				"docker rm -f " + fixtureContainerID,
				"docker network rm " + fixtureNetworkID,
				"docker image rm --no-prune ze-interop-retained:abc",
			} {
				if strings.Contains(commands, foreign) {
					t.Errorf("Suite.Run removed another run's resource with %q:\n%s", foreign, commands)
				}
			}
			for _, own := range []string{"docker rm -f this-run-peer", "docker network rm this-run-network"} {
				if !strings.Contains(commands, own) {
					t.Errorf("Suite.Run did not clean its own resource with %q:\n%s", own, commands)
				}
			}
		})
	}
}

// TestPreCleanPreservesDeadOtherRunsNames checks both actual name-claim
// boundaries. A dead owner's same-name resource is still another run's, so
// preClean must refuse it with a usable suffix instruction and no removal.
func TestPreCleanPreservesDeadOtherRunsNames(t *testing.T) {
	for _, kind := range []resourceKind{resourceContainer, resourceNetwork} {
		t.Run(kind.String(), func(t *testing.T) {
			const name = "other-runs-name"
			runner := &recordingRunner{run: func(command ProcessCommand) (ProcessResult, error) {
				joined := strings.Join(command.Arguments, " ")
				if strings.Contains(joined, "--filter name=") {
					return ProcessResult{Stdout: fixtureContainerID + "\t" + name + "\n"}, nil
				}
				if strings.Contains(joined, " inspect ") {
					return ProcessResult{Stdout: labelJSON(withKind(otherRunLabels(), kind)) + "\n"}, nil
				}
				return ProcessResult{}, nil
			}}
			lab := newLab(newDocker(runner), nil)
			var peers []PeerConfig
			var network string
			if kind == resourceContainer {
				peers = []PeerConfig{{Name: "peer", Container: name}}
			} else {
				network = name
			}
			err := lab.preClean(t.Context(), network, peers, "ZE_TEST_INTEROP_SUFFIX")
			if err == nil {
				t.Error("preClean accepted another dead run's name")
			} else {
				for _, want := range []string{name, "ZE_TEST_INTEROP_SUFFIX"} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("preClean refusal %q lacks %q", err, want)
					}
				}
			}
			commands := recordedCommands(runner.commands)
			for _, removal := range []string{"docker rm -f ", "docker network rm "} {
				if strings.Contains(commands, removal) {
					t.Errorf("preClean removed another run's resource:\n%s", commands)
				}
			}
		})
	}
}
