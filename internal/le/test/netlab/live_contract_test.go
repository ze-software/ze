package testnetlab

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// VALIDATES: the native proof consumes the JSON shapes and canonical lowercase
// tokens emitted by the live BGP, OSPF, IS-IS and BFD command producers.
func TestLiveProofAcceptsCanonicalProtocolJSON(t *testing.T) {
	for _, test := range []struct {
		module  string
		command string
		json    string
	}{
		{"bgp", "show bgp", `{"peers":[{"address":"10.0.0.2","state":"established"}]}`},
		{"ospf", "show ospf neighbor", `[{"interface":"eth1","router-id":"10.0.0.2","state":"full"}]`},
		{"isis", "show isis neighbor", `[{"interface":"eth1","system-id":"0000.0000.0002","state":"up"}]`},
		{"bfd", "show bfd sessions", `[{"peer":"10.1.0.2","state":"up"}]`},
	} {
		t.Run(test.module, func(t *testing.T) {
			fixture := newCheckerFixture(t, fixtureConfig)
			node := liveNode{Hostname: "clab-owned-r2", Module: []string{test.module}}
			node.Interfaces = []liveInterface{{OSPF: &liveIGP{BFD: true}, ISIS: &liveIGP{}}}
			// Decode the same external topology DTO that netlab supplies.
			node.BGP.Neighbors = append(node.BGP.Neighbors, struct {
				IPv4 string `yaml:"ipv4"`
			}{IPv4: "10.0.0.2"})
			fixture.checker.Execute = func(command Command) CommandResult {
				if command.Argv[len(command.Argv)-1] != test.command+" | json compact" {
					t.Fatalf("proof queried unexpected command: %v", command.Argv)
				}
				return CommandResult{Stdout: test.json}
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := fixture.checker.proveNode(ctx, fixture.root, "r2", node); err != nil {
				t.Fatalf("canonical healthy %s response rejected: %v", test.module, err)
			}
		})
	}
}

// VALIDATES: an operator's existing netlab deployment is refused before any
// startup or destructive cleanup command can run.
func TestLiveProofRefusesExistingDeployment(t *testing.T) {
	fixture := newCheckerFixture(t, fixtureConfig)
	marker := filepath.Join(fixture.root, "netlab.snapshot.yml")
	writeFixtureFile(t, marker, "name: operator-owned\n", 0o644)
	var commands int
	fixture.checker.Execute = func(Command) CommandResult {
		commands++
		return CommandResult{Code: 1}
	}
	if err := fixture.checker.RunLive(context.Background(), fixture.root); err == nil {
		t.Fatal("existing deployment accepted")
	}
	if commands != 0 {
		t.Fatalf("existing deployment received %d commands, including teardown", commands)
	}
	if body, err := os.ReadFile(marker); err != nil || string(body) != "name: operator-owned\n" {
		t.Fatalf("operator deployment changed: %q, %v", body, err)
	}
}

// VALIDATES: only one invocation can own a directory, including an invocation
// that has not yet written its deployment snapshot. Releasing the owner admits
// a fresh attempt; a refused contender never runs teardown.
func TestLiveDirectoryLeaseExcludesConcurrentInvocation(t *testing.T) {
	fixture := newCheckerFixture(t, fixtureConfig)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	fixture.checker.Execute = func(command Command) CommandResult {
		if command.Argv[1] == "up" {
			once.Do(func() { close(entered) })
			<-release
			return CommandResult{Err: errors.New("owned startup stopped"), Code: 1}
		}
		return CommandResult{}
	}
	done := make(chan error, 1)
	go func() { done <- fixture.checker.RunLive(context.Background(), fixture.root) }()
	<-entered
	contender := newChecker(fixture.root)
	contender.Env = fixture.checker.Env
	var commands int
	contender.Execute = func(Command) CommandResult { commands++; return CommandResult{Code: 1} }
	err := contender.RunLive(context.Background(), fixture.root)
	close(release)
	ownerErr := <-done
	if ownerErr == nil {
		t.Fatal("stopped owner reported success")
	}
	if err == nil {
		t.Fatal("concurrent owner accepted")
	}
	if commands != 0 {
		t.Fatalf("refused contender ran %d commands", commands)
	}
	commands = 0
	err = contender.RunLive(context.Background(), fixture.root)
	if err == nil {
		t.Fatal("failed fresh startup reported success")
	}
	if commands != 2 {
		t.Fatalf("lease was not released for fresh startup and teardown: %d commands, %v", commands, err)
	}
	if strings.Contains(err.Error(), "already owned") {
		t.Fatalf("owner exit left a held lease: %v", err)
	}
}

// VALIDATES: a fresh owner can complete every reference proof and teardown,
// release its lease, and admit another complete successful run in that directory.
func TestFreshLiveProofCanSucceedAgainAfterTeardown(t *testing.T) {
	fixture := newCheckerFixture(t, fixtureConfig)
	snapshot := `nodes:
  r1:
    hostname: clab-owned-r1
    module: [bgp, ospf, isis, bfd, routing]
    loopback: {ipv4: 10.0.0.1/32}
    interfaces: [{ospf: {bfd: true}, isis: {}}]
    bgp: {neighbors: [{ipv4: 10.0.0.2}, {ipv4: 10.1.0.6}]}
    routing: {static: [{ipv4: 10.42.0.0/24, nexthop: {ipv4: 10.1.0.2}}]}
  r2:
    hostname: clab-owned-r2
    module: [bgp, ospf, isis, bfd, routing]
    loopback: {ipv4: 10.0.0.2/32}
    interfaces: [{ospf: {bfd: true}, isis: {}}]
    bgp: {neighbors: [{ipv4: 10.0.0.1}]}
  r3:
    hostname: clab-owned-r3
    module: [bgp]
    loopback: {ipv4: 10.0.0.3/32}
    bgp: {neighbors: [{ipv4: 10.1.0.5}]}
`
	var starts, stops, pings, protocolQueries, routeQueries int
	fixture.checker.Execute = func(command Command) CommandResult {
		if command.Argv[0] == fixture.netlab {
			switch command.Argv[1] {
			case "up":
				starts++
				writeFixtureFile(t, filepath.Join(fixture.root, "netlab.snapshot.yml"), snapshot, 0o644)
			case "initial":
			case "down":
				stops++
				if err := os.Remove(filepath.Join(fixture.root, "netlab.snapshot.yml")); err != nil {
					t.Fatal(err)
				}
			default:
				t.Fatalf("unexpected lifecycle command: %v", command.Argv)
			}
			return CommandResult{}
		}
		if command.Argv[0] != "docker" {
			t.Fatalf("unexpected proof: %v", command.Argv)
		}
		if command.Argv[1] == "ps" {
			return CommandResult{}
		}
		switch command.Argv[3] {
		case "ping":
			pings++
			return CommandResult{Stdout: "1 packets transmitted, 1 received"}
		case "ip":
			routeQueries++
			return CommandResult{Stdout: `[{"dst":"10.42.0.0/24","gateway":"10.1.0.2"},{"dst":"10.43.0.0/24","type":"blackhole"}]`}
		case "ze":
			protocolQueries++
			switch command.Argv[len(command.Argv)-1] {
			case "show bgp | json compact":
				if command.Argv[2] == "clab-owned-r1" {
					return CommandResult{Stdout: `{"peers":[{"state":"established"},{"state":"established"}]}`}
				}
				return CommandResult{Stdout: `{"peers":[{"state":"established"}]}`}
			case "show ospf neighbor | json compact":
				return CommandResult{Stdout: `[{"state":"full"}]`}
			case "show isis neighbor | json compact", "show bfd sessions | json compact":
				return CommandResult{Stdout: `[{"state":"up"}]`}
			}
		}
		t.Fatalf("unexpected proof: %v", command.Argv)
		return CommandResult{Code: 1}
	}
	for range 2 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := fixture.checker.RunLive(ctx, fixture.root)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
	}
	if starts != 2 {
		t.Fatalf("starts: got %d, want 2", starts)
	}
	if stops != 2 {
		t.Fatalf("teardowns: got %d, want 2", stops)
	}
	if pings != 12 {
		t.Fatalf("directed sourced pings: got %d, want 12", pings)
	}
	if protocolQueries != 18 {
		t.Fatalf("protocol checks: got %d, want 18", protocolQueries)
	}
	if routeQueries != 2 {
		t.Fatalf("kernel static checks: got %d, want 2", routeQueries)
	}
}
