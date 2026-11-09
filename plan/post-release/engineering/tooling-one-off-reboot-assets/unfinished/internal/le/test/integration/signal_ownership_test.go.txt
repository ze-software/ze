// Design: docs/architecture/testing/qemu-integration.md -- canceled Docker runs.

package testintegration

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// signalOwnershipRunner records the real cleanup producer's commands. It
// obtains complete labels from the fixture client's own create, then changes
// only the run identity to model an unrelated dead run on the same client.
type signalOwnershipRunner struct {
	labels   map[string]string
	commands []string
}

func (r *signalOwnershipRunner) Run(_ context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
	joined := strings.Join(command.Arguments, " ")
	r.commands = append(r.commands, joined)
	if strings.HasPrefix(joined, "docker network create ") {
		r.labels = make(map[string]string)
		for index, token := range command.Arguments {
			if token == "--label" {
				key, value, found := strings.Cut(command.Arguments[index+1], "=")
				if !found {
					return interoplab.ProcessResult{}, errors.New("fixture create label has no equals sign")
				}
				r.labels[key] = value
			}
		}
		r.labels["ze.lab.run"] += "-other-run"
		// Foreign creation has a different launch as well as a different process run.
		r.labels["ze.lab.launch"] = r.labels["ze.lab.run"] + "-foreign-launch"
		return interoplab.ProcessResult{}, nil
	}
	if strings.HasPrefix(joined, "docker network ls ") {
		return interoplab.ProcessResult{Stdout: "a000000000000000000000000000000000000000000000000000000000000002\tother-run-network\n"}, nil
	}
	if strings.HasPrefix(joined, "docker network inspect ") {
		labels, err := json.Marshal(r.labels)
		if err != nil {
			return interoplab.ProcessResult{}, err
		}
		return interoplab.ProcessResult{Stdout: string(labels)}, nil
	}
	return interoplab.ProcessResult{}, nil
}

// TestSignalSweepPreservesOtherRunsResources exercises the actual signal
// cleanup boundary with the real orphan cleaner and a fake Docker daemon.
// An empty process group carries no provenance for the unrelated dead owner;
// the automatic inspection may report its resource but must not remove it.
func TestSignalSweepPreservesOtherRunsResources(t *testing.T) {
	runner := &signalOwnershipRunner{}
	docker := interoplab.NewFixtureDocker(runner)
	if _, err := docker.CreateNetwork(t.Context(), interoplab.NetworkSpec{
		Name: "label-source", Candidates: []interoplab.Subnet{{IPv4: netip.MustParsePrefix("172.29.0.0/24")}},
	}); err != nil {
		t.Fatalf("obtain fixture owner's labels: %v", err)
	}
	text := sweepAfterSignal(t.Context(), dockerPeersVerb, docker, gaterun.ProcessGroup{}, groupExitTimeout)
	if !strings.Contains(text, "other-run-network") {
		t.Errorf("automatic cleanup never reported the retained foreign resource: %q", text)
	}
	for _, command := range runner.commands {
		if strings.HasPrefix(command, "docker network rm ") {
			t.Errorf("automatic signal cleanup removed another run's resource: %s", command)
		}
	}
}
