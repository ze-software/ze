// Design: docs/architecture/testing/interop.md -- actual consumed image provenance.
package interoplab

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

// TestLabPeerImagePinsConsumedIdentity measures the registered container's
// image, even when its declared shared tag has moved. Unknown peers and invalid
// daemon answers must refuse rather than substitute that tag.
func TestLabPeerImagePinsConsumedIdentity(t *testing.T) {
	const container = "owned-image-proof"
	id := "sha256:" + strings.Repeat("ab", 32)
	answer := ProcessResult{Stdout: id + "\n"}
	runner := &recordingRunner{run: func(command ProcessCommand) (ProcessResult, error) {
		want := []string{"docker", "inspect", "--format", "{{.Image}}", container}
		if !slices.Equal(command.Arguments, want) {
			t.Fatalf("image identity did not inspect the registered container: %v", command.Arguments)
		}
		return answer, nil
	}}
	lab := newLab(newDocker(runner), []PeerConfig{{Name: "peer", Container: container, Image: "shared-tag-now-points-elsewhere"}})
	images, ok := any(lab).(interface {
		PeerImage(context.Context, string) (string, error)
	})
	if !ok {
		t.Fatal("lab has no actual peer-image observation boundary")
	}
	got, err := images.PeerImage(t.Context(), "peer")
	if err != nil {
		t.Fatal(err)
	}
	if got != id {
		t.Fatalf("consumed identity=%q, want %q", got, id)
	}
	commands := len(runner.commands)
	if _, err := images.PeerImage(t.Context(), "unknown"); !errors.Is(err, ErrUnknownPeer) {
		t.Fatalf("unknown peer was not refused by the registry: %v", err)
	}
	if len(runner.commands) != commands {
		t.Fatal("unknown peer reached Docker")
	}
	for _, invalid := range []string{"", "gobgp-interop", "sha256:ab", "sha256:" + strings.Repeat("z", 64), id + "\n" + id} {
		answer = ProcessResult{Stdout: invalid}
		got, err := images.PeerImage(t.Context(), "peer")
		if err == nil {
			t.Fatalf("invalid image observation accepted: %q", invalid)
		}
		if got != "" {
			t.Fatalf("invalid observation substituted an identity: %q", got)
		}
	}
	answer = ProcessResult{ExitCode: 1, Stderr: "owned container is unavailable"}
	if got, err := images.PeerImage(t.Context(), "peer"); err == nil {
		t.Fatalf("failed inspect substituted %q", got)
	}
}
