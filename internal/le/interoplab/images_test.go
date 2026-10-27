// Design: docs/architecture/testing/interop.md -- selected scenario image preparation.
package interoplab

import (
	"context"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestSuiteSelectedImagesFollowFinalPeers proves that static and dynamically
// prepared peers build only their dependency closure, share retained builds,
// and start from the built ID or the exact declared pin, never a cache tag.
func TestSuiteSelectedImagesFollowFinalPeers(t *testing.T) {
	for _, dynamic := range []bool{false, true} {
		name := "static"
		if dynamic {
			name = "dynamic"
		}
		t.Run(name, func(t *testing.T) {
			suite, runner := selectedImageFixture(t)
			checks, cleanups := 0, 0
			peers := []PeerConfig{
				{Name: "ze", Container: "selected-ze", Image: "ze", Host: 2},
				{Name: "frr", Container: "selected-frr", Image: "pinned-frr", Host: 3},
				{Name: "relay", Container: "selected-relay", Image: "ze", Host: 4},
			}
			plan := selectedImagePlan("selected", peers, &checks)
			if dynamic {
				plan.Peers = []PeerConfig{{Name: "obsolete", Container: "obsolete-peer", Image: "unrelated", Host: 5}}
				plan.Prepare = func(_ context.Context, prepare PrepareContext) (PreparedScenario, error) {
					if prepare.Network.IPv4 != plan.Network.Candidates[0].IPv4 {
						t.Fatal("image closure did not follow actual network preparation")
					}
					return PreparedScenario{Peers: peers, Cleanup: func() error { cleanups++; return nil }}, nil
				}
			}
			suite.Scenarios = []ScenarioPlan{plan, plan}
			report := suite.Run(t.Context())
			if report.Code != 0 || checks != 2 || report.Passed != 2 {
				t.Fatalf("checks=%d, report=%+v", checks, report)
			}
			if dynamic && cleanups != 2 {
				t.Fatalf("host cleanup calls=%d, want 2", cleanups)
			}
			var names []string
			for _, image := range report.Images {
				names = append(names, image.Name)
			}
			if !reflect.DeepEqual(names, []string{"ze", "pinned-frr"}) {
				t.Fatalf("prepared images=%v, want ze and pinned-frr once", names)
			}
			commands := recordedCommands(runner.commands)
			if strings.Contains(commands, "unrelated:latest") || strings.Contains(commands, "default-frr:latest") {
				t.Fatalf("prepared an unused image:\n%s", commands)
			}
			pull := selectedImagePullCommand(t, runner.commands, "pinned-frr@sha256:fixture")
			if strings.Count(commands, "docker build ") != 1 {
				t.Fatalf("dependencies were not prepared once:\n%s", commands)
			}
			if strings.Count(commands, "docker image rm --no-prune "+report.Images[0].retainedTag) != 1 {
				t.Fatalf("owned image was not released once:\n%s", commands)
			}
			for _, command := range runner.commands {
				if !strings.HasPrefix(strings.Join(command.Arguments, " "), "docker run -d ") {
					continue
				}
				image := command.Arguments[len(command.Arguments)-1]
				if image != "sha256:selected-ze" && image != "pinned-frr@sha256:fixture" {
					t.Errorf("container started from %q instead of prepared dependency", image)
				}
			}
			assertOrder(t, commands, "docker network create", "docker build ", pull, "docker run -d ", "docker network rm selected-net", "docker image rm --no-prune ")
		})
	}
}

// TestSuiteSelectedImageFailureIsScoped proves that an optional pull failure
// fails every dependent scenario without retry or cached-tag fallback, while
// a later independent scenario can use an already retained successful build.
func TestSuiteSelectedImageFailureIsScoped(t *testing.T) {
	suite, runner := selectedImageFixture(t)
	runner.run = kernelAnswering(func(command ProcessCommand) (ProcessResult, error) {
		joined := strings.Join(command.Arguments, " ")
		if selectedImagePull(command, "pinned-frr@sha256:fixture") {
			return ProcessResult{ExitCode: 1, Stderr: "pin unavailable"}, nil
		}
		if strings.HasPrefix(joined, "docker build ") {
			return ProcessResult{Stdout: "sha256:selected-ze\n"}, nil
		}
		return ProcessResult{}, nil
	})
	checks := 0
	dependent := selectedImagePlan("dependent", []PeerConfig{
		{Name: "frr", Container: "selected-frr", Image: "pinned-frr", Host: 3},
	}, &checks)
	independent := selectedImagePlan("independent", []PeerConfig{
		{Name: "ze", Container: "selected-ze", Image: "ze", Host: 2},
	}, &checks)
	// The failed optional pull precedes a successful build in the same batch.
	// That build must still publish its reference for the later independent plan.
	suite.Images[0], suite.Images[1] = suite.Images[1], suite.Images[0]
	dependent.Peers = append(dependent.Peers, independent.Peers...)
	suite.Scenarios = []ScenarioPlan{dependent, dependent, independent}
	report := suite.Run(t.Context())
	if report.SetupError != "" || report.Code != 1 || report.Failed != 2 || report.Passed != 1 || checks != 1 {
		t.Fatalf("checks=%d, report=%+v", checks, report)
	}
	for _, result := range report.Scenarios[:2] {
		if !strings.Contains(result.Error, "pin unavailable") {
			t.Errorf("dependency failure lost: %+v", result)
		}
	}
	commands := recordedCommands(runner.commands)
	selectedImagePullCommand(t, runner.commands, "pinned-frr@sha256:fixture")
	if strings.Count(commands, "docker build ") != 1 {
		t.Fatalf("failed or retained image prepared repeatedly:\n%s", commands)
	}
	if strings.Count(commands, "docker run -d ") != 1 || !strings.Contains(commands, "sha256:selected-ze") {
		t.Fatalf("failed dependency started or independent peer lost its built ID:\n%s", commands)
	}
	if strings.Count(commands, "docker network rm selected-net") != 3 || strings.Count(commands, "docker image rm --no-prune ") != 1 {
		t.Fatalf("owned resources not released:\n%s", commands)
	}
}

// TestSuiteSelectedImagesRejectMissingDependencies proves that closure errors
// stay failures and release the selected network and rendered host resources,
// without starting a peer or running its checker.
func TestSuiteSelectedImagesRejectMissingDependencies(t *testing.T) {
	for _, image := range []string{"unknown", "", "missing-tag"} {
		t.Run(image, func(t *testing.T) {
			suite, runner := selectedImageFixture(t)
			suite.Images = append(suite.Images, ImageBuild{Name: "missing-tag"})
			checks, cleanups := 0, 0
			plan := selectedImagePlan("invalid", nil, &checks)
			plan.Containers = []string{"selected-peer"}
			plan.Prepare = func(context.Context, PrepareContext) (PreparedScenario, error) {
				return PreparedScenario{
					Peers:   []PeerConfig{{Name: "peer", Container: "selected-peer", Image: image, Host: 2}},
					Cleanup: func() error { cleanups++; return nil },
				}, nil
			}
			suite.Scenarios = []ScenarioPlan{plan}
			report := suite.Run(t.Context())
			if report.Code != 1 || report.Failed != 1 || checks != 0 || cleanups != 1 {
				t.Fatalf("checks=%d cleanups=%d report=%+v", checks, cleanups, report)
			}
			commands := recordedCommands(runner.commands)
			if strings.Contains(commands, "docker run -d ") || !strings.Contains(commands, "docker network rm selected-net") {
				t.Fatalf("invalid dependency started a peer or leaked its network:\n%s", commands)
			}
		})
	}
}

// TestSuiteEagerImagesRemainUnfiltered proves that an unselected suite still
// prepares every declaration before creating its first scenario network.
func TestSuiteEagerImagesRemainUnfiltered(t *testing.T) {
	suite, runner := selectedImageFixture(t)
	suite.ImagesOnDemand = false
	checks := 0
	suite.Scenarios = []ScenarioPlan{selectedImagePlan("full", []PeerConfig{
		{Name: "ze", Container: "selected-ze", Image: "ze", Host: 2},
	}, &checks)}
	report := suite.Run(t.Context())
	if report.Code != 0 || len(report.Images) != len(suite.Images) || checks != 1 {
		t.Fatalf("checks=%d report=%+v", checks, report)
	}
	pull := selectedImagePullCommand(t, runner.commands, "unrelated:latest")
	assertOrder(t, recordedCommands(runner.commands), pull, "docker network create", "docker run -d ")
}

// TestSuiteSelectedImageCancellationReleasesOwnership cancels a dependency
// pull after Ze has built, then checks teardown without starting any peer.
func TestSuiteSelectedImageCancellationReleasesOwnership(t *testing.T) {
	suite, runner := selectedImageFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	runner.run = kernelAnswering(func(command ProcessCommand) (ProcessResult, error) {
		joined := strings.Join(command.Arguments, " ")
		if strings.HasPrefix(joined, "docker build ") {
			return ProcessResult{Stdout: "sha256:selected-ze\n"}, nil
		}
		if selectedImagePull(command, "pinned-frr@sha256:fixture") {
			cancel()
			return ProcessResult{}, context.Canceled
		}
		return ProcessResult{}, nil
	})
	suite.Images[1].Required = true
	checks, cleanups := 0, 0
	plan := selectedImagePlan("canceled", nil, &checks)
	plan.Containers = []string{"selected-ze", "selected-frr"}
	plan.Prepare = func(context.Context, PrepareContext) (PreparedScenario, error) {
		return PreparedScenario{
			Peers: []PeerConfig{
				{Name: "ze", Container: "selected-ze", Image: "ze", Host: 2},
				{Name: "frr", Container: "selected-frr", Image: "pinned-frr", Host: 3},
			},
			Cleanup: func() error { cleanups++; return nil },
		}, nil
	}
	suite.Scenarios = []ScenarioPlan{plan}
	report := suite.Run(ctx)
	if report.Code != 1 || report.Failed != 1 || checks != 0 || cleanups != 1 {
		t.Fatalf("checks=%d cleanups=%d report=%+v", checks, cleanups, report)
	}
	if len(report.Images) != 2 || !strings.Contains(report.Scenarios[0].Error, "context canceled") {
		t.Fatalf("lost partial image results or cancellation: %+v", report)
	}
	commands := recordedCommands(runner.commands)
	if strings.Contains(commands, "docker run -d ") {
		t.Fatalf("started a peer after canceled image preparation:\n%s", commands)
	}
	pull := selectedImagePullCommand(t, runner.commands, "pinned-frr@sha256:fixture")
	assertOrder(t, commands, "docker build ", pull, "docker network rm selected-net", "docker image rm --no-prune "+report.Images[0].retainedTag)
}

// selectedImagePull identifies the action and exact dependency, independently
// of Docker.Pull's incidental quiet-output flag.
func selectedImagePull(command ProcessCommand, reference string) bool {
	arguments := command.Arguments
	if len(arguments) < 3 {
		return false
	}
	return arguments[0] == DockerExecutable && arguments[1] == "pull" &&
		arguments[len(arguments)-1] == reference
}

// selectedImagePullCommand requires exactly one pull and returns its original
// argv text so ordering assertions retain the actual command's position.
func selectedImagePullCommand(t *testing.T, commands []ProcessCommand, reference string) string {
	t.Helper()
	count := 0
	var pull string
	for _, command := range commands {
		if selectedImagePull(command, reference) {
			count++
			pull = strings.Join(command.Arguments, " ")
		}
	}
	if count != 1 {
		t.Fatalf("pulls of %q = %d, want 1:\n%s", reference, count, recordedCommands(commands))
	}
	return pull
}

func selectedImageFixture(t *testing.T) (Suite, *recordingRunner) {
	t.Helper()
	directory := t.TempDir()
	dockerfile := filepath.Join(directory, "Dockerfile")
	if err := os.WriteFile(dockerfile, []byte("FROM scratch\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &recordingRunner{run: kernelAnswering(func(command ProcessCommand) (ProcessResult, error) {
		if strings.HasPrefix(strings.Join(command.Arguments, " "), "docker build ") {
			return ProcessResult{Stdout: "sha256:selected-ze\n"}, nil
		}
		return ProcessResult{}, nil
	})}
	return Suite{
		Docker: newDocker(runner), StagedZe: kernelCheckZe, ImagesOnDemand: true,
		Images: []ImageBuild{
			{Name: "ze", Tag: "ze:cache", Dockerfile: dockerfile, Context: directory, Required: true},
			{Name: "pinned-frr", Tag: "pinned-frr@sha256:fixture", Pull: true},
			{Name: "default-frr", Tag: "default-frr:latest", Pull: true, Required: true},
			{Name: "unrelated", Tag: "unrelated:latest", Pull: true, Required: true},
		},
	}, runner
}

func selectedImagePlan(name string, peers []PeerConfig, checks *int) ScenarioPlan {
	return ScenarioPlan{
		Source: ScenarioSource{Name: name, Checker: func(context.Context, *CheckContext) error {
			*checks++
			return nil
		}},
		Network: NetworkSpec{Name: "selected-net", Candidates: []Subnet{{IPv4: netip.MustParsePrefix("172.29.0.0/24")}}},
		Peers:   peers,
	}
}
