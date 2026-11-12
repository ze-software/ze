// Design: docs/guide/netlab.md -- export live inventory before node startup.
package testnetlab

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// VALIDATES: netlab 26.10's native snapshot protects an existing deployment
// even when that deployment did not request a YAML inventory.
func TestLiveProofRefusesPickleOnlyDeployment(t *testing.T) {
	fixture := newCheckerFixture(t, fixtureConfig)
	marker := filepath.Join(fixture.root, "netlab.snapshot.pickle")
	writeFixtureFile(t, marker, "operator-owned pickled topology", 0o600)
	var commands int
	fixture.checker.Execute = func(Command) CommandResult {
		commands++
		return CommandResult{Code: 1}
	}
	err := fixture.checker.RunLive(context.Background(), fixture.root)
	if err == nil {
		t.Fatal("existing pickle-only deployment accepted")
	}
	if !strings.Contains(err.Error(), "already holds deployment state netlab.snapshot.pickle") {
		t.Errorf("existing deployment was not refused by its native marker: %v", err)
	}
	if commands != 0 {
		t.Errorf("existing deployment received %d commands, including teardown", commands)
	}
	body, readErr := os.ReadFile(marker)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(body) != "operator-owned pickled topology" {
		t.Fatalf("operator deployment changed: %q", body)
	}
}

// VALIDATES: netlab's pickle-only default cannot hide created nodes from failed
// startup diagnostics and cleanup; the YAML inventory is requested before up.
func TestLiveStartExportsYAMLSnapshotBeforeFailureCleanup(t *testing.T) {
	for _, failure := range []string{"up", "initial"} {
		t.Run(failure, func(t *testing.T) {
			fixture := newCheckerFixture(t, fixtureConfig)
			snapshot := "nodes:\n  r1:\n    hostname: clab-owned-r1\n"
			var inspected, logged, stopped, checked bool
			fixture.checker.Execute = func(command Command) CommandResult {
				if command.Argv[0] == fixture.netlab {
					switch command.Argv[1] {
					case "up":
						// netlab 26.10 always writes its normal pickle. Its YAML
						// output module writes a file only when explicitly selected.
						writeFixtureFile(t, filepath.Join(fixture.root, "netlab.snapshot.pickle"), "pickled topology", 0o600)
						setting := slices.Index(command.Argv, "--set")
						if setting >= 0 {
							if setting+1 < len(command.Argv) {
								if command.Argv[setting+1] == "defaults.netlab.create.output.yaml.output=netlab.snapshot.yml" {
									writeFixtureFile(t, filepath.Join(fixture.root, "netlab.snapshot.yml"), snapshot, 0o600)
								}
							}
						}
						if failure == "up" {
							return CommandResult{Code: 1, Stderr: "injected partial startup failure"}
						}
						return CommandResult{}
					case "initial":
						return CommandResult{Code: 1, Stderr: "injected initial configuration failure"}
					case "down":
						stopped = true
						if !inspected {
							t.Error("cleanup started without inspecting the created node")
						}
						if !logged {
							t.Error("cleanup started without retaining the created node's logs")
						}
						return CommandResult{}
					}
				}
				if command.Argv[0] == "docker" {
					switch command.Argv[1] {
					case "inspect":
						inspected = true
						return CommandResult{Stdout: `[{"State":{"Status":"running"}}]`}
					case "logs":
						logged = true
						return CommandResult{Stdout: "created node output\n"}
					case "ps":
						checked = true
						if !stopped {
							t.Error("node absence checked before teardown")
						}
						if !slices.Contains(command.Argv, "name=^/clab-owned-r1$") {
							t.Errorf("cleanup checked the wrong inventory: %v", command.Argv)
						}
						return CommandResult{}
					}
				}
				t.Fatalf("unexpected command: %v", command.Argv)
				return CommandResult{Code: 1}
			}
			err := fixture.checker.RunLive(context.Background(), fixture.root)
			if err == nil {
				t.Fatal("injected startup failure was accepted")
			}
			if !strings.Contains(err.Error(), "injected") {
				t.Fatalf("startup failure was lost: %v", err)
			}
			if !stopped {
				t.Error("failed startup skipped teardown")
			}
			if !checked {
				t.Error("failed startup skipped the created node's absence check")
			}
			directories, globErr := filepath.Glob(filepath.Join(fixture.root, "diagnostics-*"))
			if globErr != nil {
				t.Fatal(globErr)
			}
			if len(directories) != 1 {
				t.Fatalf("diagnostic directories: got %d, want 1", len(directories))
			}
			body, readErr := os.ReadFile(filepath.Join(directories[0], "netlab.snapshot.yml"))
			if readErr != nil {
				t.Fatalf("explicit YAML inventory was not retained: %v", readErr)
			}
			if string(body) != snapshot {
				t.Fatalf("retained inventory: got %q, want %q", body, snapshot)
			}
		})
	}
}
