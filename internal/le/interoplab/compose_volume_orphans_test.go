package interoplab

import (
	"slices"
	"strings"
	"testing"
)

const (
	proofVolumeProject = ComposeProjectPrefix + "ab12cd34"
	proofVolumeName    = proofVolumeProject + "_ze-data"
	proofVolumeCurrent = ComposeProjectPrefix + "ffff0000"
	proofVolumeListing = "docker volume ls --quiet --filter label=" + labelMarker + "=" + labelMarkerValue
)

// proofVolumeDaemon answers the marker listing with another project's volume,
// the current project's own and two marked volumes outside the proof projects;
// the volume's inspect with labels; and the project's container listing with
// container, empty for none.
func proofVolumeDaemon(labels map[string]string, container string) func(ProcessCommand) (ProcessResult, error) {
	return func(command ProcessCommand) (ProcessResult, error) {
		joined := strings.Join(command.Arguments, " ")
		switch {
		case joined == proofVolumeListing:
			return ProcessResult{Stdout: proofVolumeName + "\n" + proofVolumeCurrent + "_ze-data\nsomeone-elses-data\n" +
				ComposeProjectPrefix + "x;rm\n"}, nil
		case strings.HasPrefix(joined, "docker volume inspect"):
			return ProcessResult{Stdout: labelJSON(labels) + "\n"}, nil
		case strings.HasPrefix(joined, "docker ps --all --quiet --filter label="+ComposeProjectLabel+"="+proofVolumeProject):
			return ProcessResult{Stdout: container}, nil
		}
		return ProcessResult{}, nil
	}
}

// proofVolumeLabels is a volume the fixture owner (pid 4242) created for
// proofVolumeProject, with mutate applied.
func proofVolumeLabels(mutate func(map[string]string)) map[string]string {
	return createdAs(fixtureLabels(func(labels map[string]string) {
		labels[ComposeProjectLabel] = proofVolumeProject
		if mutate != nil {
			mutate(labels)
		}
	}), resourceVolume)
}

// assertProofVolumeCommands fails unless the first command is the marker
// listing, only the other proof project's volume was inspected, and nothing
// was removed or pruned.
func assertProofVolumeCommands(t *testing.T, runner *recordingRunner) {
	t.Helper()
	commands := strings.Split(recordedCommands(runner.commands), "\n")
	if len(commands) == 0 || commands[0] != proofVolumeListing {
		t.Errorf("commands = %v, want the marker listing first", commands)
	}
	for _, command := range commands {
		if strings.HasPrefix(command, "docker volume inspect") && !strings.HasSuffix(command, " "+proofVolumeName) {
			t.Errorf("a volume outside the other proof project was inspected: %q", command)
		}
	}
	for _, command := range runner.commands {
		if slices.Contains(command.Arguments, "rm") || slices.Contains(command.Arguments, "prune") {
			t.Errorf("judging volumes ran %q", strings.Join(command.Arguments, " "))
		}
	}
}

// VALIDATES: a volume whose owner the cleanup's judge (ownerJudge.judge)
// proves gone, in a project with no container, is left behind; the current
// project's volume and marked volumes outside the proof projects are never
// named; and nothing is removed. A dead owner whose project still holds a
// container is not left behind.
// PREVENTS: a killed run's store with a credential hash and host key that
// nothing names, and a listing that widens into removing volumes.
func TestComposeProofVolumesNamesOnlyEarlierRuns(t *testing.T) {
	runner := &recordingRunner{run: proofVolumeDaemon(proofVolumeLabels(nil), "")}
	volumes, err := newDocker(runner).ComposeProofVolumes(t.Context(), proofVolumeCurrent)
	if err != nil {
		t.Fatalf("ComposeProofVolumes: %v", err)
	}
	if len(volumes) != 1 || volumes[0].Name != proofVolumeName || volumes[0].State != ComposeVolumeLeftBehind {
		t.Fatalf("volumes = %#v, want only %s, left behind", volumes, proofVolumeName)
	}
	assertProofVolumeCommands(t, runner)

	runner = &recordingRunner{run: proofVolumeDaemon(proofVolumeLabels(nil), "c0ffee\n")}
	volumes, err = newDocker(runner).ComposeProofVolumes(t.Context(), proofVolumeCurrent)
	if err != nil {
		t.Fatalf("ComposeProofVolumes: %v", err)
	}
	if len(volumes) != 1 || volumes[0].State != ComposeVolumeMaybeLive {
		t.Errorf("a project holding a container answered %#v, want maybe live", volumes)
	}
	assertProofVolumeCommands(t, runner)
}

// VALIDATES: a live owner's volume is never left behind, including in the gap
// between that run's down and its second up, when its project has no
// container and its volume is unmounted.
// PREVENTS: a notice whose removal command deletes a concurrent run's store.
func TestComposeProofVolumesLiveOwnerInTheGapIsNotLeftBehind(t *testing.T) {
	runner := &recordingRunner{run: proofVolumeDaemon(proofVolumeLabels(nil), "")}
	docker := newDocker(runner)
	docker.judge.lookup = lookupAnswering("100")
	volumes, err := docker.ComposeProofVolumes(t.Context(), proofVolumeCurrent)
	if err != nil {
		t.Fatalf("ComposeProofVolumes: %v", err)
	}
	if len(volumes) != 1 || volumes[0].State != ComposeVolumeMaybeLive {
		t.Errorf("a live owner with no container answered %#v, want maybe live", volumes)
	}
	assertProofVolumeCommands(t, runner)
}

// VALIDATES: a volume whose owner labels cannot be judged is never left
// behind.
// PREVENTS: a malformed label read as proof that the owner is gone.
func TestComposeProofVolumesUnjudgeableOwnerIsNotLeftBehind(t *testing.T) {
	labels := proofVolumeLabels(func(labels map[string]string) { delete(labels, labelPID) })
	runner := &recordingRunner{run: proofVolumeDaemon(labels, "")}
	volumes, err := newDocker(runner).ComposeProofVolumes(t.Context(), proofVolumeCurrent)
	if err != nil {
		t.Fatalf("ComposeProofVolumes: %v", err)
	}
	if len(volumes) != 1 || volumes[0].State != ComposeVolumeMaybeLive {
		t.Errorf("an unjudgeable owner answered %#v, want maybe live", volumes)
	}
	assertProofVolumeCommands(t, runner)
}

// VALIDATES: the override labels the project's named volume with this
// client's owner labels, so a later run can judge its owner.
func TestComposeOverrideLabelsTheVolume(t *testing.T) {
	text, err := newDocker(&recordingRunner{run: func(ProcessCommand) (ProcessResult, error) { return ProcessResult{}, nil }}).ComposeOverrideFile(ComposeOverride{
		Service: "ze", Image: "img", Volume: "ze-data",
	})
	if err != nil {
		t.Fatalf("ComposeOverrideFile: %v", err)
	}
	if !strings.Contains(text, "volumes:\n  ze-data:\n    labels:\n") || !strings.Contains(text, `"ze.lab.kind": "volume"`) {
		t.Errorf("override does not label the volume:\n%s", text)
	}
	if ownedVolumeName("ze-compose-x;rm") || ownedVolumeName("ze-compose-") || !ownedVolumeName("ze-compose-ab_ze-data") {
		t.Error("ownedVolumeName admits the wrong names")
	}
}
