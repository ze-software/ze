package interoplab

import (
	"maps"
	"strings"
	"testing"
)

// otherRunLabels are the owner labels of another live run on this client:
// every fact the fixture judge compares matches, only the run id differs.
func otherRunLabels() map[string]string {
	return fixtureLabels(func(labels map[string]string) { labels[labelRun] = "run-other" })
}

// VALIDATES: a name whose labels record this very run (run id and every other
// owner label equal to this client's) is this run's own leftover, and
// claimName removes it although its owner, this process, is alive; another
// live run's name, differing only by run id, is still refused.
// Method: one same-name container over the fake runner, the owner pid answered
// as running, the labels varied.
// PREVENTS: scenario N+1 of a lab sharing one network refusing scenario N's
// slow-to-vanish network with "stop pid <self>".
func TestClaimNameReclaimsThisRunsOwnLeftover(t *testing.T) {
	for _, row := range []struct {
		name    string
		labels  map[string]string
		removed bool
	}{
		{name: "this run's own leftover is removed", labels: fixtureLabels(nil), removed: true},
		{name: "another live run is refused", labels: otherRunLabels()},
		{name: "same run id under another uid is refused",
			labels: fixtureLabels(func(labels map[string]string) { labels[labelUID] = "1001" })},
	} {
		t.Run(row.name, func(t *testing.T) {
			runner := &recordingRunner{run: func(command ProcessCommand) (ProcessResult, error) {
				joined := strings.Join(command.Arguments, " ")
				switch {
				case strings.HasPrefix(joined, "docker network ls --no-trunc --filter name="):
					return ProcessResult{Stdout: fixtureContainerID + "\tlab-net\n"}, nil
				case strings.HasPrefix(joined, "docker network inspect"):
					return ProcessResult{Stdout: labelJSON(withKind(row.labels, resourceNetwork)) + "\n"}, nil
				}
				return ProcessResult{}, nil
			}}
			docker := newDocker(runner)
			docker.judge.lookup = lookupAnswering("100")
			err := docker.claimName(t.Context(), resourceNetwork, "lab-net", "")
			removed := strings.Contains(recordedCommands(runner.commands), "docker network rm "+fixtureContainerID)
			if removed != row.removed {
				t.Fatalf("removed = %v, want %v (err %v):\n%s", removed, row.removed, err, recordedCommands(runner.commands))
			}
			if row.removed && err != nil {
				t.Errorf("claimName refused this run's own leftover: %v", err)
			}
			if !row.removed && err == nil {
				t.Error("claimName accepted another run's live name")
			}
		})
	}
}

func withKind(labels map[string]string, kind resourceKind) map[string]string {
	copied := maps.Clone(labels)
	copied[labelKind] = kind.labelValue()
	return copied
}
