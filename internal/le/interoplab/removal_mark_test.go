// Related: docker.go -- RemoveContainer, which takes the unsettled mark;
// startsettle.go -- unsettledNames.

package interoplab

import (
	"testing"
)

// VALIDATES: a start the daemon refused after creating the container marks
// the name, and the first removal takes the mark even when its `rm -f`
// removed the container, so the set holds only removals still owed.
// METHOD: `docker run -d` is refused with the vhost-net device error (the
// daemon created the container first), every `rm -f` prints the name as a
// removal, then answers it absent. A second RemoveContainer of the name must
// issue exactly one `rm -f`.
// PREVENTS: a mark left behind by a removal that succeeded, which made a later
// removal of the same name wait the whole settle window for nothing.
func TestRemovalTakesTheMarkOfARefusedStartItRemoved(t *testing.T) {
	const name = "ze-peer-1"
	removals := 0
	present := true
	runner := &recordingRunner{run: func(command ProcessCommand) (ProcessResult, error) {
		switch command.Arguments[1] {
		case "run":
			return ProcessResult{ExitCode: 125, Stderr: "docker: Error response from daemon: error gathering device information while adding custom device \"/dev/vhost-net\": no such file or directory"}, nil
		case "rm":
			removals++
			if present {
				present = false
				return ProcessResult{Stdout: name + "\n"}, nil
			}
			return ProcessResult{}, nil
		}
		return ProcessResult{}, nil
	}}
	docker := newDocker(runner)

	if err := docker.RunDetached(t.Context(), DetachedContainer{Name: name, Image: "alpine:3"}); err == nil {
		t.Fatal("a refused start answered no error")
	}
	if err := docker.RemoveContainer(t.Context(), name); err != nil {
		t.Fatalf("first removal: %v", err)
	}
	removals = 0
	if err := docker.RemoveContainer(t.Context(), name); err != nil {
		t.Fatalf("second removal: %v", err)
	}
	if removals != 1 {
		t.Fatalf("second removal of a name the first removal removed issued %d rm -f, want 1", removals)
	}
}
