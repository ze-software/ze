package testnetlab

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// VALIDATES: no empty response, missing state, partial row set, or unhealthy
// configured adjacency can satisfy the live protocol check.
func TestProtocolStateRequiresEveryDeclaredAdjacency(t *testing.T) {
	for _, rows := range [][]stateRow{nil, {{}}, {{State: "Established"}}, {{State: "Established"}, {State: "Idle"}}} {
		if statesReady(rows, "Established", 2) {
			t.Fatalf("unhealthy or incomplete state accepted: %+v", rows)
		}
	}
	if !statesReady([]stateRow{{State: "Established"}, {State: "Established"}}, "Established", 2) {
		t.Fatal("complete healthy state rejected")
	}
}

// VALIDATES: both reference static routes must reach the kernel, with the
// declared next hop and the discard action, before a live run can succeed.
func TestStaticProofRequiresNextHopAndDiscard(t *testing.T) {
	routes := []routeRow{{Destination: "10.42.0.0/24", Gateway: "10.1.0.2"}}
	if staticReady(routes, "10.1.0.2") {
		t.Fatal("missing discard route accepted")
	}
	routes = append(routes, routeRow{Destination: "10.43.0.0/24", Type: "blackhole"})
	if !staticReady(routes, "10.1.0.2") {
		t.Fatal("complete static routes rejected")
	}
	if staticReady(routes, "192.0.2.1") {
		t.Fatal("wrong next hop accepted")
	}
}

// VALIDATES: netlab can create some nodes and then fail; teardown is still
// attempted, and a teardown failure is retained beside the startup failure.
func TestPartialLiveStartAlwaysTearsDown(t *testing.T) {
	fixture := newCheckerFixture(t, fixtureConfig)
	var calls [][]string
	fixture.checker.Execute = func(command Command) CommandResult {
		calls = append(calls, command.Argv)
		if command.Argv[1] == "up" {
			return CommandResult{Err: errors.New("partial startup"), Code: 1}
		}
		return CommandResult{Err: errors.New("cleanup failed"), Code: 1}
	}
	err := fixture.checker.RunLive(context.Background(), fixture.root)
	if err == nil {
		t.Fatal("partial startup reported success")
	}
	for _, phrase := range []string{"partial startup", "cleanup failed"} {
		if !strings.Contains(err.Error(), phrase) {
			t.Errorf("failure omits %q: %v", phrase, err)
		}
	}
	want := [][]string{{fixture.netlab, "up", "--no-config", "topology.yml"}, {fixture.netlab, "down", "--cleanup"}}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("lifecycle: got %v, want %v", calls, want)
	}
}
