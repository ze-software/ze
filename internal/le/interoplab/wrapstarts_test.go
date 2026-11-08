package interoplab

import (
	"context"
	"net/netip"
	"slices"
	"strings"
	"testing"
)

// VALIDATES: an image first wrapped for a Prestart alone, in one scenario, is
// still probed for sh and ip when a later scenario of the same run makes it a
// link member, and its start is inspected only once (AC-5).
// PREVENTS: the start cache standing in for the probe, so a member image with
// no ip reaches `docker run` and dies under errexit instead of being refused
// before the scenario starts, naming its Dockerfile.
func TestWrapStartsProbesAnImageFirstSeenWithoutLinks(t *testing.T) {
	runner := &recordingRunner{run: func(command ProcessCommand) (ProcessResult, error) {
		if slices.Contains(command.Arguments, "inspect") {
			return ProcessResult{Stdout: `{"Entrypoint":["/usr/bin/daemon"],"Cmd":null}`}, nil
		}
		return ProcessResult{}, nil
	}}
	suite := Suite{Docker: newDocker(runner)}
	starts := imageStarts{start: make(map[string]ImageStart), probed: make(map[string]bool)}

	prestartOnly := []PeerConfig{{Name: "a", Image: "img", Host: 2, Prestart: "true"}}
	if _, err := suite.wrapStarts(context.Background(), nil, prestartOnly, prestartOnly, starts); err != nil {
		t.Fatalf("first scenario: %v", err)
	}

	members := []PeerConfig{{Name: "a", Image: "img", Host: 2}, {Name: "b", Image: "img", Host: 3}}
	links := []Link{{Name: "ab", VLAN: 10, IPv4: netip.MustParsePrefix("10.10.0.0/24"), Members: []string{"a", "b"}}}
	if _, err := suite.wrapStarts(context.Background(), links, members, members, starts); err != nil {
		t.Fatalf("second scenario: %v", err)
	}

	probes, inspects := 0, 0
	for _, command := range runner.commands {
		joined := strings.Join(command.Arguments, " ")
		if strings.Contains(joined, "command -v ip") {
			probes++
		}
		if slices.Contains(command.Arguments, "inspect") {
			inspects++
		}
	}
	if probes != 1 {
		t.Fatalf("link tool probes = %d, want 1 (the image became a link member in the second scenario); commands: %v", probes, runner.commands)
	}
	if inspects != 1 {
		t.Fatalf("image inspects = %d, want 1 (the start is cached across scenarios); commands: %v", inspects, runner.commands)
	}
}
