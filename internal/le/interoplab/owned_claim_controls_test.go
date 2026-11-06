// Design: docs/architecture/testing/interop.md -- exact process ownership of name claims.

package interoplab

import (
	"strings"
	"testing"
)

// TestNameClaimRequiresExactLaunch checks both name-claim boundaries with an
// otherwise identical process owner. Matching optional launch labels permit
// reclamation. A missing or different launch preserves the resource and refuses.
func TestNameClaimRequiresExactLaunch(t *testing.T) {
	for _, kind := range []resourceKind{resourceContainer, resourceNetwork} {
		t.Run(kind.String(), func(t *testing.T) {
			for _, row := range []struct {
				name           string
				ownerLaunch    string
				resourceLaunch string
				owned          bool
			}{
				{name: "both absent", owned: true},
				{name: "both inherited", ownerLaunch: "root-launch", resourceLaunch: "root-launch", owned: true},
				{name: "unexpected inherited launch", resourceLaunch: "foreign-launch"},
				{name: "missing inherited launch", ownerLaunch: "root-launch"},
				{name: "different inherited launch", ownerLaunch: "root-launch", resourceLaunch: "foreign-launch"},
			} {
				t.Run(row.name, func(t *testing.T) {
					const name = "launch-claim"
					owner := fixtureOwner()
					owner.launch = row.ownerLaunch
					labels := withKind(fixtureLabels(nil), kind)
					if row.resourceLaunch != "" {
						labels[labelLaunch] = row.resourceLaunch
					}
					runner := &recordingRunner{run: func(command ProcessCommand) (ProcessResult, error) {
						joined := strings.Join(command.Arguments, " ")
						if strings.Contains(joined, "--filter name=") {
							return ProcessResult{Stdout: fixtureContainerID + "\t" + name + "\n"}, nil
						}
						if strings.Contains(joined, " inspect ") {
							return ProcessResult{Stdout: labelJSON(labels)}, nil
						}
						return ProcessResult{}, nil
					}}
					docker := newDocker(runner)
					docker.owner = owner
					lab := newLab(docker, nil)
					var network string
					var peers []PeerConfig
					if kind == resourceContainer {
						peers = []PeerConfig{{Name: "peer", Container: name}}
					} else {
						network = name
					}
					err := lab.preClean(t.Context(), network, peers, "ZE_TEST_INTEROP_SUFFIX")
					if row.owned {
						if err != nil {
							t.Fatalf("exact owner's claim refused: %v", err)
						}
					} else {
						if err == nil {
							t.Fatal("a different launch's same-name resource was accepted")
						}
						if !strings.Contains(err.Error(), "ZE_TEST_INTEROP_SUFFIX") {
							t.Errorf("refusal lacks the name-conflict fix: %v", err)
						}
					}
					removal := "docker rm -f " + fixtureContainerID
					if kind == resourceNetwork {
						removal = "docker network rm " + fixtureContainerID
					}
					commands := recordedCommands(runner.commands)
					if removed := strings.Contains(commands, removal); removed != row.owned {
						t.Errorf("resource removed = %v, want %v:\n%s", removed, row.owned, commands)
					}
				})
			}
		})
	}
}
