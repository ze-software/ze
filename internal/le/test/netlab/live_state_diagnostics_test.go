// Design: docs/guide/netlab.md -- retain routing state before failed lab cleanup.
package testnetlab

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// VALIDATES: a failed adjacency proof preserves the node's protocol, RIB, FIB
// and address state before teardown, even after the proof context is canceled.
func TestFailedLiveProofRetainsRoutingStateBeforeCleanup(t *testing.T) {
	fixture := newCheckerFixture(t, fixtureConfig)
	snapshot := `nodes:
  r1:
    hostname: clab-owned-r1
    module: [bgp, ospf, isis, bfd, routing]
    bgp: {neighbors: [{ipv4: 10.0.0.2}]}
  r2:
    hostname: clab-owned-r2
    module: [bgp, ospf, isis, bfd, routing]
  r3:
    hostname: clab-owned-r3
    module: [bgp]
`
	queries := map[string]struct {
		file string
		body string
	}{
		"ze cli -c show bgp | json compact":                  {"bgp.json", `{"peers":[{"address":"10.0.0.2","state":"connecting"}]}`},
		"ze cli -c show ospf interface | json compact":       {"ospf-interface.json", `[{"interface":"eth1","state":"point-to-point"}]`},
		"ze cli -c show ospf neighbor | json compact":        {"ospf-neighbor.json", `[{"router-id":"10.0.0.2","state":"full"}]`},
		"ze cli -c show ospf database router | json compact": {"ospf-database-router.json", `[{"advertising-router":"10.0.0.2","links":["10.0.0.2/32"]}]`},
		"ze cli -c show ospf route | json compact":           {"ospf-route.json", `[{"prefix":"10.0.0.2/32","next-hop":"10.1.0.2"}]`},
		"ze cli -c show isis interface | json compact":       {"isis-interface.json", `[{"interface":"eth1","level":"level-2"}]`},
		"ze cli -c show isis neighbor | json compact":        {"isis-neighbor.json", `[{"system-id":"0000.0000.0002","state":"up"}]`},
		"ze cli -c show isis database detail | json compact": {"isis-database-detail.json", `[{"lsp-id":"0000.0000.0002.00-00","prefixes":["10.0.0.2/32"]}]`},
		"ze cli -c show isis route | json compact":           {"isis-route.json", `[{"prefix":"10.0.0.2/32","metric":10}]`},
		"ze cli -c show bfd sessions | json compact":         {"bfd.json", `[{"address":"10.1.0.2","state":"up"}]`},
		"ze cli -c show rib | json compact":                  {"rib.json", `[{"prefix":"10.0.0.2/32","protocol":"ospf"}]`},
		"ze cli -c show plugin list | json compact":          {"plugins.json", `[{"name":"ospf","state":"running"}]`},
		"ip -j address show":                                 {"addresses.json", `[{"ifname":"lo","addr_info":[{"local":"10.0.0.1","prefixlen":32}]}]`},
		"ip -j route show table main":                        {"routes.json", `[{"dst":"10.0.0.2/32","gateway":"10.1.0.2","dev":"eth1"}]`},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	proofFailed, stopped, checked := false, false, false
	captured := make(map[string]bool)
	fixture.checker.Execute = func(command Command) CommandResult {
		if command.Argv[0] == fixture.netlab {
			switch command.Argv[1] {
			case "up":
				writeFixtureFile(t, filepath.Join(fixture.root, "netlab.snapshot.yml"), snapshot, 0o600)
				return CommandResult{}
			case "initial":
				return CommandResult{}
			case "down":
				for _, node := range []string{"r1", "r2", "r3"} {
					for query := range queries {
						if !captured["clab-owned-"+node+" "+query] {
							t.Errorf("cleanup began without retaining %s state: %s", node, query)
						}
					}
				}
				stopped = true
				return CommandResult{}
			}
		}
		if command.Argv[0] != "docker" {
			t.Fatalf("unexpected command: %v", command.Argv)
		}
		switch command.Argv[1] {
		case "inspect", "logs":
			return CommandResult{Stdout: "retained container state"}
		case "ps":
			checked = true
			if !stopped {
				t.Error("container absence checked before teardown")
			}
			return CommandResult{}
		case "exec":
			query := strings.Join(command.Argv[3:], " ")
			if !proofFailed {
				if query != "ze cli -c show bgp | json compact" {
					t.Fatalf("unexpected adjacency proof: %v", command.Argv)
				}
				proofFailed = true
				cancel()
				return CommandResult{Stdout: queries[query].body}
			}
			if stopped {
				t.Errorf("state capture ran after teardown: %v", command.Argv)
			}
			if err := command.Context.Err(); err != nil {
				t.Errorf("diagnostic inherited canceled proof context: %v", err)
			}
			deadline, bounded := command.Context.Deadline()
			if !bounded {
				t.Error("state capture has no command timeout")
			}
			if time.Until(deadline) > 5*time.Second {
				t.Errorf("state capture command timeout exceeds five seconds: %v", deadline)
			}
			if !slices.Contains([]string{"clab-owned-r1", "clab-owned-r2", "clab-owned-r3"}, command.Argv[2]) {
				t.Fatalf("captured an unowned node: %v", command.Argv)
			}
			answer, exists := queries[query]
			if !exists {
				t.Fatalf("unexpected state capture: %v", command.Argv)
			}
			if query == "ze cli -c show rib | json compact" {
				for _, node := range []string{"clab-owned-r1", "clab-owned-r2", "clab-owned-r3"} {
					for _, essential := range []string{
						"ip -j address show",
						"ip -j route show table main",
						"ze cli -c show bgp | json compact",
						"ze cli -c show ospf neighbor | json compact",
						"ze cli -c show isis neighbor | json compact",
						"ze cli -c show bfd sessions | json compact",
					} {
						if !captured[node+" "+essential] {
							t.Errorf("detail capture began before essential state from %s: %s", node, essential)
						}
					}
				}
			}
			captured[command.Argv[2]+" "+query] = true
			return CommandResult{Stdout: answer.body}
		}
		t.Fatalf("unexpected command: %v", command.Argv)
		return CommandResult{Code: 1}
	}
	err := fixture.checker.RunLive(ctx, fixture.root)
	if err == nil {
		t.Fatal("failed adjacency proof reported success")
	}
	if !strings.Contains(err.Error(), "connecting") {
		t.Errorf("original adjacency state was lost: %v", err)
	}
	if !stopped {
		t.Error("failed proof skipped teardown")
	}
	if !checked {
		t.Error("failed proof skipped container absence checks")
	}
	directories, globErr := filepath.Glob(filepath.Join(fixture.root, "diagnostics-*"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(directories) != 1 {
		t.Fatalf("diagnostic directories: got %d, want 1", len(directories))
	}
	if info, statErr := os.Stat(directories[0]); statErr != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("routing diagnostics are not private: %v, %v", info, statErr)
	}
	for _, node := range []string{"r1", "r2", "r3"} {
		for _, answer := range queries {
			path := filepath.Join(directories[0], node+"."+answer.file)
			body, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Errorf("routing artifact %s was not retained: %v", path, readErr)
				continue
			}
			if string(body) != answer.body {
				t.Errorf("routing artifact %s lost full producer output: %q", path, body)
			}
			if info, statErr := os.Stat(path); statErr != nil || info.Mode().Perm() != 0o600 {
				t.Errorf("routing artifact %s is not private: %v, %v", path, info, statErr)
			}
		}
	}
}

// VALIDATES: the real executor's killed-process exit shape retains the command
// context's deadline cause rather than reporting only a synthetic exit code.
func TestLiveDiagnosticKilledCommandRetainsDeadlineCause(t *testing.T) {
	fixture := newCheckerFixture(t, fixtureConfig)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	fixture.checker.Execute = func(command Command) CommandResult {
		if !errors.Is(command.Context.Err(), context.DeadlineExceeded) {
			t.Fatalf("fixture did not reach an expired command context: %v", command.Context.Err())
		}
		// execute translates a killed *exec.ExitError to Code=-1 and Err=nil.
		return CommandResult{Code: -1, Stderr: "process killed"}
	}
	err := fixture.checker.keepLiveCommand(ctx, fixture.root, fixture.root,
		"r1.routes.json", []string{"docker", "exec", "clab-owned-r1", "ip", "-j", "route", "show", "table", "main"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("killed command lost its deadline cause: %v", err)
	}
	body, readErr := os.ReadFile(filepath.Join(fixture.root, "r1.routes.json.error.txt"))
	if readErr != nil {
		t.Fatalf("killed command has no failure artifact: %v", readErr)
	}
	for _, phrase := range []string{"deadline exceeded", "exited -1", "process killed", "ip -j route show table main"} {
		if !strings.Contains(string(body), phrase) {
			t.Errorf("killed command artifact lost %q: %q", phrase, body)
		}
	}
	if _, statErr := os.Stat(filepath.Join(fixture.root, "r1.routes.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("killed command produced a success artifact: %v", statErr)
	}
}

// VALIDATES: successful JSON state keeps stdout parseable and retains stderr
// separately, while node logs retain their established combined-stream form.
func TestLiveDiagnosticJSONKeepsStderrSeparateFromState(t *testing.T) {
	fixture := newCheckerFixture(t, fixtureConfig)
	const state = `[{"dst":"10.0.0.2/32","gateway":"10.1.0.2"}]`
	const warning = "diagnostic warning on stderr\n"
	fixture.checker.Execute = func(Command) CommandResult {
		return CommandResult{Stdout: state, Stderr: warning}
	}
	for _, file := range []string{"r1.routes.json", "r1.log"} {
		if err := fixture.checker.keepLiveCommand(context.Background(), fixture.root, fixture.root,
			file, []string{"docker", "exec", "clab-owned-r1", "ip", "-j", "route", "show", "table", "main"}); err != nil {
			t.Fatalf("successful diagnostic %s failed: %v", file, err)
		}
	}
	body, readErr := os.ReadFile(filepath.Join(fixture.root, "r1.routes.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(body) != state {
		t.Errorf("JSON artifact mixed process streams: %q", body)
	}
	if !json.Valid(body) {
		t.Errorf("JSON stdout became unparseable: %q", body)
	}
	stderrPath := filepath.Join(fixture.root, "r1.routes.json.stderr.txt")
	stderr, readErr := os.ReadFile(stderrPath)
	if readErr != nil {
		t.Errorf("JSON diagnostic lost its stderr sidecar: %v", readErr)
	} else if string(stderr) != warning {
		t.Errorf("JSON stderr sidecar lost producer output: %q", stderr)
	}
	if info, statErr := os.Stat(stderrPath); statErr != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("JSON stderr sidecar is not private: %v, %v", info, statErr)
	}
	log, readErr := os.ReadFile(filepath.Join(fixture.root, "r1.log"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(log) != state+warning {
		t.Errorf("node log lost combined process streams: %q", log)
	}
}

// VALIDATES: an unavailable state query leaves its command and failure in an
// error artifact, and capture continues without publishing empty healthy JSON.
func TestFailedLiveStateQueryLeavesExplicitFailureArtifact(t *testing.T) {
	fixture := newCheckerFixture(t, fixtureConfig)
	nodes := map[string]liveNode{"r1": {Hostname: "clab-owned-r1"}}
	fixture.checker.Execute = func(command Command) CommandResult {
		if command.Argv[1] != "exec" {
			return CommandResult{Stdout: "container diagnostic"}
		}
		query := strings.Join(command.Argv[3:], " ")
		switch query {
		case "ip -j address show":
			return CommandResult{Code: 1}
		case "ip -j route show table main":
			return CommandResult{Err: context.DeadlineExceeded, Stderr: "Docker transport stalled"}
		case "ze cli -c show bgp | json compact":
			return CommandResult{Code: 1, Stdout: "partial BGP response", Stderr: "daemon socket unavailable"}
		case "ze cli -c show rib | json compact":
			return CommandResult{Stdout: `[{"prefix":"10.0.0.2/32","protocol":"ospf"}]`}
		default:
			return CommandResult{Stdout: "[]"}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	directory, err := fixture.checker.keepLiveDiagnostics(ctx, fixture.root, "", nil, nodes)
	if err == nil {
		t.Error("failed state query reported a complete capture")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("capture lost the command timeout: %v", err)
	}
	for file, phrases := range map[string][]string{
		"r1.addresses.json": {"ip -j address show", "exited 1"},
		"r1.routes.json":    {"ip -j route show table main", "deadline exceeded", "Docker transport stalled"},
		"r1.bgp.json":       {"show bgp | json compact", "partial BGP response", "daemon socket unavailable"},
	} {
		path := filepath.Join(directory, file)
		if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Errorf("failed query published a successful state artifact %s: %v", file, statErr)
		}
		failurePath := path + ".error.txt"
		body, readErr := os.ReadFile(failurePath)
		if readErr != nil {
			t.Errorf("failed query has no explicit error artifact %s: %v", file, readErr)
			continue
		}
		for _, phrase := range phrases {
			if !strings.Contains(string(body), phrase) {
				t.Errorf("failed query artifact %s lost %q: %q", file, phrase, body)
			}
		}
		if info, statErr := os.Stat(failurePath); statErr != nil || info.Mode().Perm() != 0o600 {
			t.Errorf("failed query artifact %s is not private: %v, %v", file, info, statErr)
		}
	}
	body, readErr := os.ReadFile(filepath.Join(directory, "r1.rib.json"))
	if readErr != nil {
		t.Fatalf("failed query prevented later RIB capture: %v", readErr)
	}
	if string(body) != `[{"prefix":"10.0.0.2/32","protocol":"ospf"}]` {
		t.Errorf("later RIB capture lost actual output: %q", body)
	}
}
