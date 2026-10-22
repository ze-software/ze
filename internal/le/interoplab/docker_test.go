package interoplab

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// VALIDATES: AC-8 (b): a Docker whose endpoint selection refused (macOS, no
// DOCKER_HOST, no `ze` profile socket) answers that refusal from the kernel
// check, an image build and a one-shot container, and runs no docker command.
// PREVENTS: a lab reaching whichever daemon the client's context names, or
// building an image, after the selection said no daemon is the labs'.
func TestDockerAnswersTheEndpointRefusalBeforeAnyCommand(t *testing.T) {
	refusal := errors.New("the Docker socket of the `ze` Colima profile is absent")
	runner := &recordingRunner{run: func(ProcessCommand) (ProcessResult, error) { return ProcessResult{}, nil }}
	docker := newDocker(runner)
	docker.endpointRefusal = refusal
	ctx := context.Background()
	ze := filepath.Join(t.TempDir(), "ze")
	if err := os.WriteFile(ze, []byte("ze"), 0o700); err != nil {
		t.Fatalf("stage ze: %v", err)
	}
	if err := DockerKernel(ze)(ctx, docker); !errors.Is(err, refusal) {
		t.Errorf("kernel check: %v, want the endpoint refusal", err)
	}
	if _, err := docker.Build(ctx, ImageBuild{Tag: "ze-test:latest", Context: "/ctx", Dockerfile: "/ctx/Dockerfile"}); !errors.Is(err, refusal) {
		t.Errorf("build: %v, want the endpoint refusal", err)
	}
	if _, err := docker.RunOneShot(ctx, OneShotContainer{Image: "alpine:3.24.3", Timeout: time.Second}); !errors.Is(err, refusal) {
		t.Errorf("one-shot: %v, want the endpoint refusal", err)
	}
	if len(runner.commands) != 0 {
		t.Errorf("docker commands ran: %+v", runner.commands)
	}
}
