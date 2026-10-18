package interoplab

import (
	"context"
	"net/netip"
	"strings"
	"testing"
)

// VALIDATES: the wiring row "any lab verb -> Suite.Run -> Docker.CleanupOrphans
// before Preflight": the labeled listing follows the probe and precedes both
// the Preflight and the first create.
// PREVENTS: a lab start that never collects a killed run's leftovers.
func TestSuiteRunCleansOrphansBeforePreflight(t *testing.T) {
	runner := &recordingRunner{}
	runner.run = kernelAnswering(nil)
	preflightAt := -1
	suite := Suite{
		Docker:   newDocker(runner),
		StagedZe: kernelCheckZe,
		NoBuild:  true,
		Preflight: func(context.Context, *Docker) error {
			preflightAt = len(runner.commands)
			return nil
		},
		Scenarios: []ScenarioPlan{{
			Source:  ScenarioSource{Name: "one", Directory: "/scenarios/one", Checker: func(context.Context, *CheckContext) error { return nil }},
			Network: NetworkSpec{Name: "lab-net", Candidates: []Subnet{{IPv4: netip.MustParsePrefix("172.29.0.0/24")}}},
			Peers:   []PeerConfig{{Name: "peer", Container: "lab-peer", Image: "peer-image", Host: 3}},
		}},
	}
	report := suite.Run(t.Context())
	if report.Code != 0 {
		t.Fatalf("Suite code = %d: %#v", report.Code, report)
	}
	listedAt := -1
	createdAt := -1
	for index, command := range runner.commands {
		joined := strings.Join(command.Arguments, " ")
		if listedAt < 0 && strings.HasPrefix(joined, "docker ps -a --no-trunc --filter label="+labelMarker) {
			listedAt = index
		}
		if createdAt < 0 && strings.HasPrefix(joined, "docker network create") {
			createdAt = index
		}
	}
	if listedAt < 0 {
		t.Fatalf("Suite.Run never listed the labeled containers:\n%s", recordedCommands(runner.commands))
	}
	if listedAt == 0 {
		t.Error("the orphan listing ran before docker info")
	}
	if preflightAt < 0 || listedAt >= preflightAt {
		t.Errorf("orphan listing at %d, Preflight at %d: the sweep must precede Preflight", listedAt, preflightAt)
	}
	if createdAt < 0 || listedAt >= createdAt {
		t.Errorf("orphan listing at %d, first create at %d", listedAt, createdAt)
	}
}

// VALIDATES: AC-7: preClean removes a same-name container only when its labels
// prove the owner gone, and refuses a live owner's or an unlabeled one with an
// error naming the container, its owner and the fix.
// PREVENTS: a run force-removing another live run's same-name container.
func TestPreCleanRefusesLiveOrUnlabeledSameName(t *testing.T) {
	for _, row := range []struct {
		name    string
		labels  string
		lookup  string
		removed bool
		message []string
	}{
		{name: "gone owner is removed", labels: labelJSON(fixtureLabels(nil)), removed: true},
		{name: "live owner is refused", labels: labelJSON(fixtureLabels(nil)), lookup: "100",
			message: []string{"lab-peer", "4242", "_INTEROP_SUFFIX", "./le test integration cleanup"}},
		{name: "unlabeled is refused", labels: "null",
			message: []string{"lab-peer", "no Ze lab owner label", "_INTEROP_SUFFIX"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			runner := &recordingRunner{run: func(command processCommand) (processResult, error) {
				joined := strings.Join(command.Arguments, " ")
				switch {
				case strings.HasPrefix(joined, "docker ps -a --no-trunc --filter name="):
					return processResult{Stdout: fixtureContainerID + "\tlab-peer-other\n" + fixtureContainerID + "\tlab-peer\n"}, nil
				case strings.HasPrefix(joined, "docker container inspect"):
					return processResult{Stdout: row.labels + "\n"}, nil
				}
				return processResult{}, nil
			}}
			docker := newDocker(runner)
			docker.judge.lookup = lookupAnswering(row.lookup)
			lab := newLab(docker, nil)
			err := lab.preClean(t.Context(), "", []PeerConfig{{Name: "peer", Container: "lab-peer"}})
			removed := strings.Contains(recordedCommands(runner.commands), "docker rm -f "+fixtureContainerID)
			if removed != row.removed {
				t.Errorf("removed = %v, want %v:\n%s", removed, row.removed, recordedCommands(runner.commands))
			}
			if row.removed {
				if err != nil {
					t.Errorf("preClean refused a gone owner: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("preClean accepted a container it cannot prove dead")
			}
			for _, want := range row.message {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal %q does not name %q", err, want)
				}
			}
		})
	}
}
