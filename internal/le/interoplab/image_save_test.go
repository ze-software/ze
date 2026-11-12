package interoplab

import (
	"slices"
	"strings"
	"testing"
)

// VALIDATES: preview export saves the immutable ID, under the Docker owner.
// PREVENTS: exporting a shared tag which moved after inspection.
func TestSaveImageUsesImmutableReferenceAndBound(t *testing.T) {
	runner := &recordingRunner{}
	docker := newDocker(runner)
	reference := "sha256:" + strings.Repeat("a", 64)
	if err := docker.SaveImage(t.Context(), reference, "/private/image.tar"); err != nil {
		t.Fatal(err)
	}
	if len(runner.commands) != 1 {
		t.Fatalf("commands = %v", runner.commands)
	}
	want := []string{DockerExecutable, "image", "save", "--output", "/private/image.tar", reference}
	if !slices.Equal(runner.commands[0].Arguments, want) {
		t.Fatalf("argv = %v", runner.commands[0].Arguments)
	}
	if runner.commands[0].Timeout <= 0 {
		t.Fatal("image save has no bound")
	}
	for _, reference := range []string{"", "ze:latest", "-option"} {
		if err := docker.SaveImage(t.Context(), reference, "/private/image.tar"); err == nil {
			t.Fatalf("accepted %q", reference)
		}
	}
}
