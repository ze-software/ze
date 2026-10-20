package interoplab

import (
	"context"
	"errors"
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

// VALIDATES: a lab run's start sweep prints its summary after its per-resource
// lines, so a sweep that met nothing still says it ran. Method: run the suite
// against a fake daemon, stop it at Preflight, and read the progress output.
// PREVENTS: a clean daemon's sweep that prints nothing, which an operator
// cannot tell from a sweep that never ran.
func TestSuiteRunPrintsTheSweepSummary(t *testing.T) {
	oneContainer := func(command ProcessCommand) (ProcessResult, bool) {
		joined := strings.Join(command.Arguments, " ")
		if strings.HasPrefix(joined, "docker network ls") {
			return ProcessResult{}, true
		}
		if strings.HasPrefix(joined, "docker image ls") {
			return ProcessResult{}, true
		}
		return ProcessResult{}, false
	}
	for _, row := range []struct {
		name  string
		run   func(ProcessCommand) (ProcessResult, error)
		lines []string
	}{
		{name: "clean daemon", run: kernelAnswering(nil),
			lines: []string{"interop: cleanup: no Ze lab resource on this Docker daemon"}},
		{name: "one removable orphan", run: orphanDaemon(fixtureLabels(nil), oneContainer),
			lines: []string{
				"interop: cleanup: removed container lab-peer-4242",
				"interop: cleanup: 1 removed, 0 kept, 0 failed, 0 unknown",
			}},
	} {
		t.Run(row.name, func(t *testing.T) {
			var progress strings.Builder
			suite := Suite{
				Docker:   newDocker(&recordingRunner{run: row.run}),
				Progress: &progress,
				NoBuild:  true,
				Preflight: func(context.Context, *Docker) error {
					return errors.New("stop after the sweep")
				},
				Scenarios: []ScenarioPlan{{Source: ScenarioSource{Name: "one"}}},
			}
			suite.Run(t.Context())
			output := progress.String()
			from := 0
			for _, want := range row.lines {
				at := strings.Index(output[from:], want)
				if at < 0 {
					t.Fatalf("progress lacks %q after offset %d:\n%s", want, from, output)
				}
				from += at + len(want)
			}
		})
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
		{name: "gone owner is removed", labels: labelJSON(otherRunLabels()), removed: true},
		{name: "live owner is refused", labels: labelJSON(otherRunLabels()), lookup: "100",
			message: []string{"lab-peer", "4242", "_INTEROP_SUFFIX", "./le test integration cleanup"}},
		{name: "unlabeled is refused", labels: "null",
			message: []string{"lab-peer", "no Ze lab owner label", "_INTEROP_SUFFIX"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			runner := &recordingRunner{run: func(command ProcessCommand) (ProcessResult, error) {
				joined := strings.Join(command.Arguments, " ")
				switch {
				case strings.HasPrefix(joined, "docker ps -a --no-trunc --filter name="):
					return ProcessResult{Stdout: fixtureContainerID + "\tlab-peer-other\n" + fixtureContainerID + "\tlab-peer\n"}, nil
				case strings.HasPrefix(joined, "docker container inspect"):
					return ProcessResult{Stdout: row.labels + "\n"}, nil
				}
				return ProcessResult{}, nil
			}}
			docker := newDocker(runner)
			docker.judge.lookup = lookupAnswering(row.lookup)
			lab := newLab(docker, nil)
			err := lab.preClean(t.Context(), "", []PeerConfig{{Name: "peer", Container: "lab-peer"}}, "ZE_TEST_INTEROP_SUFFIX")
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

// VALIDATES: R-1, AC-7: a scenario whose preClean refuses a same-name
// container or network removes nothing by that name when it tears down, and
// removes no network it never created. Method: one live-owned resource of the
// scenario's name, refused, then the recorded argv searched for any removal.
// PREVENTS: the deferred teardown force-removing a live run's container and
// network after claimName refused them.
func TestRunScenarioRefusedClaimRemovesNothingForeign(t *testing.T) {
	for _, row := range []struct {
		name    string
		listing string
		inspect string
		refused string
	}{
		{name: "refused container", listing: "docker ps -a --no-trunc --filter name=",
			inspect: "docker container inspect", refused: "lab-peer"},
		{name: "refused network", listing: "docker network ls --no-trunc --filter name=",
			inspect: "docker network inspect", refused: "lab-net"},
	} {
		t.Run(row.name, func(t *testing.T) {
			runner := &recordingRunner{}
			runner.run = kernelAnswering(func(command ProcessCommand) (ProcessResult, error) {
				joined := strings.Join(command.Arguments, " ")
				if strings.HasPrefix(joined, row.listing) {
					return ProcessResult{Stdout: fixtureContainerID + "\t" + row.refused + "\n"}, nil
				}
				if strings.HasPrefix(joined, row.inspect) {
					return ProcessResult{Stdout: labelJSON(otherRunLabels()) + "\n"}, nil
				}
				return ProcessResult{}, nil
			})
			docker := newDocker(runner)
			docker.judge.lookup = lookupAnswering("100")
			suite := Suite{
				Docker: docker, StagedZe: kernelCheckZe, NoBuild: true,
				Scenarios: []ScenarioPlan{{
					Source:  ScenarioSource{Name: "one", Directory: "/scenarios/one", Checker: func(context.Context, *CheckContext) error { return nil }},
					Network: NetworkSpec{Name: "lab-net", Candidates: []Subnet{{IPv4: netip.MustParsePrefix("172.29.0.0/24")}}},
					Peers:   []PeerConfig{{Name: "peer", Container: "lab-peer", Image: "peer-image", Host: 3}},
				}},
			}
			report := suite.Run(t.Context())
			if report.Code != 1 {
				t.Fatalf("Suite code = %d, want 1: %#v", report.Code, report)
			}
			if !strings.Contains(report.Scenarios[0].Error, row.refused) {
				t.Errorf("scenario error %q does not name %s", report.Scenarios[0].Error, row.refused)
			}
			joined := recordedCommands(runner.commands)
			for _, removal := range []string{"docker rm -f", "docker network rm", "docker network create"} {
				if strings.Contains(joined, removal) {
					t.Errorf("a refused claim still ran %q:\n%s", removal, joined)
				}
			}
		})
	}
}

// VALIDATES: R-1: a create the daemon refuses because another run took the
// name after preClean claimed it records nothing for teardown, while what this
// run did create (the network, in the container row) is still removed.
// PREVENTS: teardown removing a container or network a concurrent run created.
func TestRunScenarioTakenNameIsNotTornDown(t *testing.T) {
	for _, row := range []struct {
		name     string
		refuse   string
		stderr   string
		absent   []string
		required []string
	}{
		{name: "container name taken", refuse: "docker run -d --name lab-peer",
			stderr:   `Conflict. The container name "/lab-peer" is already in use by container "c0".`,
			absent:   []string{"docker rm -f lab-peer"},
			required: []string{"docker network rm lab-net"}},
		{name: "network name taken", refuse: "docker network create",
			stderr: "network with name lab-net already exists",
			absent: []string{"docker rm -f lab-peer", "docker network rm lab-net"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			runner := &recordingRunner{}
			runner.run = kernelAnswering(func(command ProcessCommand) (ProcessResult, error) {
				if strings.HasPrefix(strings.Join(command.Arguments, " "), row.refuse) {
					return ProcessResult{ExitCode: 125, Stderr: row.stderr}, nil
				}
				return ProcessResult{}, nil
			})
			suite := Suite{
				Docker: newDocker(runner), StagedZe: kernelCheckZe, NoBuild: true,
				Scenarios: []ScenarioPlan{{
					Source:  ScenarioSource{Name: "one", Directory: "/scenarios/one", Checker: func(context.Context, *CheckContext) error { return nil }},
					Network: NetworkSpec{Name: "lab-net", Candidates: []Subnet{{IPv4: netip.MustParsePrefix("172.29.0.0/24")}}},
					Peers:   []PeerConfig{{Name: "peer", Container: "lab-peer", Image: "peer-image", Host: 3}},
				}},
			}
			if report := suite.Run(t.Context()); report.Code != 1 {
				t.Fatalf("Suite code = %d, want 1: %#v", report.Code, report)
			}
			joined := recordedCommands(runner.commands)
			for _, removal := range row.absent {
				if strings.Contains(joined, removal) {
					t.Errorf("teardown ran %q for a name another run took:\n%s", removal, joined)
				}
			}
			for _, removal := range row.required {
				if !strings.Contains(joined, removal) {
					t.Errorf("teardown never ran %q for what this run created:\n%s", removal, joined)
				}
			}
		})
	}
}

// VALIDATES: user-facing-errors, AC-7: a pre-clean refusal names the lab's
// real suffix variable from Suite.SuffixVariable, never a placeholder, and a
// suite with none names the cleanup command and the owner pid instead.
// PREVENTS: an operator told to set `ZE_<LAB>_INTEROP_SUFFIX`, which no lab reads.
func TestPreCleanRefusalNamesTheRealSuffixVariable(t *testing.T) {
	for _, row := range []struct {
		name     string
		variable string
		want     []string
		absent   []string
	}{
		{name: "lab with a suffix variable", variable: "ZE_L2TP_INTEROP_SUFFIX",
			want: []string{"set ZE_L2TP_INTEROP_SUFFIX to", "./le test integration cleanup"}, absent: []string{"<LAB>"}},
		{name: "suite with none", want: []string{"stop pid 4242", "./le test integration cleanup"},
			absent: []string{"<LAB>", "INTEROP_SUFFIX"}},
	} {
		t.Run(row.name, func(t *testing.T) {
			runner := &recordingRunner{}
			runner.run = kernelAnswering(func(command ProcessCommand) (ProcessResult, error) {
				joined := strings.Join(command.Arguments, " ")
				if strings.HasPrefix(joined, "docker ps -a --no-trunc --filter name=") {
					return ProcessResult{Stdout: fixtureContainerID + "\tlab-peer\n"}, nil
				}
				if strings.HasPrefix(joined, "docker container inspect") {
					return ProcessResult{Stdout: labelJSON(otherRunLabels()) + "\n"}, nil
				}
				return ProcessResult{}, nil
			})
			docker := newDocker(runner)
			docker.judge.lookup = lookupAnswering("100")
			suite := Suite{
				Docker: docker, StagedZe: kernelCheckZe, NoBuild: true, SuffixVariable: row.variable,
				Scenarios: []ScenarioPlan{{
					Source:  ScenarioSource{Name: "one", Directory: "/scenarios/one", Checker: func(context.Context, *CheckContext) error { return nil }},
					Network: NetworkSpec{Name: "lab-net", Candidates: []Subnet{{IPv4: netip.MustParsePrefix("172.29.0.0/24")}}},
					Peers:   []PeerConfig{{Name: "peer", Container: "lab-peer", Image: "peer-image", Host: 3}},
				}},
			}
			report := suite.Run(t.Context())
			if len(report.Scenarios) != 1 {
				t.Fatalf("Suite ran %d scenarios: %#v", len(report.Scenarios), report)
			}
			refusal := report.Scenarios[0].Error
			for _, want := range row.want {
				if !strings.Contains(refusal, want) {
					t.Errorf("refusal %q does not name %q", refusal, want)
				}
			}
			for _, absent := range row.absent {
				if strings.Contains(refusal, absent) {
					t.Errorf("refusal %q names %q", refusal, absent)
				}
			}
		})
	}
}
