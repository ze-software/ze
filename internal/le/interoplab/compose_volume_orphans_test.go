package interoplab

import (
	"slices"
	"strings"
	"testing"
)

// VALIDATES: the leftover listing names a marked volume of an earlier proof
// run, skips the current project's own volume and any marked volume outside
// the proof projects, and runs only the one listing, never a removal.
// PREVENTS: a store volume a killed run left that nothing names, and a listing
// that widens into removing volumes, which the lab cleanup never does.
func TestComposeProofVolumesNamesOnlyEarlierRuns(t *testing.T) {
	const earlier = ComposeProjectPrefix + "ab12cd34_ze-data"
	const current = ComposeProjectPrefix + "ffff0000"
	runner := &recordingRunner{run: func(command ProcessCommand) (ProcessResult, error) {
		return ProcessResult{Stdout: earlier + "\n" + current + "_ze-data\nsomeone-elses-data\n" +
			ComposeProjectPrefix + "x;rm\n"}, nil
	}}
	names, err := newDocker(runner).ComposeProofVolumes(t.Context(), current)
	if err != nil {
		t.Fatalf("ComposeProofVolumes: %v", err)
	}
	if !slices.Equal(names, []string{earlier}) {
		t.Errorf("leftovers = %v, want [%s]", names, earlier)
	}
	commands := strings.Split(recordedCommands(runner.commands), "\n")
	want := "docker volume ls --quiet --filter label=" + labelMarker + "=" + labelMarkerValue
	if !slices.Equal(commands, []string{want}) {
		t.Errorf("commands = %v, want only %q", commands, want)
	}
}

// VALIDATES: the override labels the project's named volume with this
// client's owner labels, so a leftover one is recognised as a proof's.
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
