// Design: docs/guide/netlab.md -- retain failed node diagnostics before teardown.
package testnetlab

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: a node that exits before initial configuration leaves its actual
// Docker state, process logs and generated input available after full teardown.
func TestFailedLiveStartRetainsNodeDiagnosticsBeforeCleanup(t *testing.T) {
	fixture := newCheckerFixture(t, fixtureConfig)
	snapshot := "nodes:\n  r1:\n    hostname: clab-owned-r1\n"
	generated := filepath.Join(fixture.root, "node_files", "r1", "ze.conf")
	var inspected, logged, stopped bool
	fixture.checker.Execute = func(command Command) CommandResult {
		if command.Argv[0] == fixture.netlab {
			switch command.Argv[1] {
			case "up":
				writeFixtureFile(t, filepath.Join(fixture.root, "netlab.snapshot.yml"), snapshot, 0o600)
				writeFixtureFile(t, generated, "rendered node configuration", 0o600)
				return CommandResult{Stdout: "all containers created\n"}
			case "initial":
				return CommandResult{Code: 1, Stderr: "container is not running"}
			case "down":
				stopped = true
				if !inspected || !logged {
					t.Error("teardown removed the node before its state and logs were captured")
				}
				if err := os.Remove(filepath.Join(fixture.root, "netlab.snapshot.yml")); err != nil {
					t.Fatal(err)
				}
				if err := os.RemoveAll(filepath.Join(fixture.root, "node_files")); err != nil {
					t.Fatal(err)
				}
				return CommandResult{}
			}
		}
		if command.Argv[0] == "docker" {
			switch command.Argv[1] {
			case "exec":
				if len(command.Argv) < 3 {
					t.Fatalf("diagnostic command has no node: %v", command.Argv)
				}
				if command.Argv[2] != "clab-owned-r1" {
					t.Fatalf("diagnostic command targets an unowned node: %v", command.Argv)
				}
				return CommandResult{Code: 1, Stderr: "container is not running"}
			case "inspect":
				inspected = true
				if stopped {
					t.Error("inspected the removed container")
				}
				return CommandResult{Stdout: `[{"State":{"Status":"exited","ExitCode":1},"Config":{"Cmd":["ze","start","/etc/ze/ze.conf"]}}]`}
			case "logs":
				if len(command.Argv) != 5 || command.Argv[2] != "--tail" || command.Argv[3] != "200" {
					t.Fatalf("node logs have no bounded tail: %v", command.Argv)
				}
				logged = true
				if stopped {
					t.Error("read logs after removing the container")
				}
				return CommandResult{Stdout: "daemon starting\n", Stderr: "bind: cannot assign requested address\n"}
			case "ps":
				return CommandResult{}
			}
		}
		t.Fatalf("unexpected command: %v", command.Argv)
		return CommandResult{Code: 1}
	}
	err := fixture.checker.RunLive(context.Background(), fixture.root)
	if err == nil || !strings.Contains(err.Error(), "container is not running") {
		t.Fatalf("startup failure was lost: %v", err)
	}
	if !stopped {
		t.Fatal("startup failure did not tear down the lab")
	}
	directories, globErr := filepath.Glob(filepath.Join(fixture.root, "diagnostics-*"))
	if globErr != nil || len(directories) != 1 {
		t.Fatalf("private diagnostic directory was not retained: %v, %v", directories, globErr)
	}
	diagnostics := directories[0]
	if info, statErr := os.Stat(diagnostics); statErr != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("diagnostics are not private: %v, %v", info, statErr)
	}
	for name, expected := range map[string]string{
		"netlab-up.log":       "all containers created",
		"netlab.snapshot.yml": snapshot,
		filepath.Join("node_files", "r1", "ze.conf"): "rendered node configuration",
		"r1.inspect.json": `"ExitCode":1`,
		"r1.log":          "bind: cannot assign requested address",
	} {
		path := filepath.Join(diagnostics, name)
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Errorf("diagnostic %s was not retained: %v", name, readErr)
			continue
		}
		if !strings.Contains(string(body), expected) {
			t.Errorf("diagnostic %s lost producer output: %q", name, body)
		}
		if info, statErr := os.Stat(path); statErr != nil || info.Mode().Perm() != 0o600 {
			t.Errorf("diagnostic %s is not private: %v, %v", name, info, statErr)
		}
	}
}

// VALIDATES: a diagnostic failure cannot prevent teardown or erase the original
// node-start failure that made the diagnostic capture necessary.
func TestFailedLiveDiagnosticsStillTearsDown(t *testing.T) {
	fixture := newCheckerFixture(t, fixtureConfig)
	var stopped, checked bool
	fixture.checker.Execute = func(command Command) CommandResult {
		if command.Argv[0] == fixture.netlab {
			switch command.Argv[1] {
			case "up":
				writeFixtureFile(t, filepath.Join(fixture.root, "netlab.snapshot.yml"), "nodes:\n  r1:\n    hostname: clab-owned-r1\n", 0o600)
				return CommandResult{}
			case "initial":
				return CommandResult{Code: 1, Stderr: "node stopped"}
			case "down":
				stopped = true
				return CommandResult{}
			}
		}
		if command.Argv[0] == "docker" {
			if command.Argv[1] == "ps" {
				checked = true
				return CommandResult{}
			}
			return CommandResult{Code: 1, Stderr: "diagnostic transport refused"}
		}
		t.Fatalf("unexpected command: %v", command.Argv)
		return CommandResult{Code: 1}
	}
	err := fixture.checker.RunLive(context.Background(), fixture.root)
	if err == nil {
		t.Fatal("failed node reported success")
	}
	for _, phrase := range []string{"node stopped", "diagnostic transport refused"} {
		if !strings.Contains(err.Error(), phrase) {
			t.Errorf("failure lost %q: %v", phrase, err)
		}
	}
	if !stopped || !checked {
		t.Fatalf("diagnostic failure skipped cleanup or its assertion: stopped=%v checked=%v", stopped, checked)
	}
}
