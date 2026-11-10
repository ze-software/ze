//go:build linux || darwin

package kernelbuilder_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestKernelContainerCancellationJoinsBeforeRepair runs Build against an actual
// Docker daemon and a small, locally cached shell image. A blocked worker writes
// a heartbeat through its mounted output directory. The ownership-repair worker
// independently inspects that heartbeat: it refuses if the original worker is
// still running. A separately created container must survive. This proves the
// daemon boundary rather than treating the Docker CLI's death as a container stop.
// KERNEL_CONTAINER_PROOF_IMAGE names an existing image with /bin/sh, sleep and
// chown; no image is pulled. This is lifecycle proof, not kernel or guest proof.
// Every docker command the test itself issues goes through interoplab.Docker,
// so the sentinel it starts carries the owner labels `./le test integration
// clean` judges; Build's own containers are the code under test.
func TestKernelContainerCancellationJoinsBeforeRepair(t *testing.T) {
	base := os.Getenv("KERNEL_CONTAINER_PROOF_IMAGE")
	if base == "" {
		t.Skip("real Docker lifecycle proof requires KERNEL_CONTAINER_PROOF_IMAGE naming a locally cached shell image")
	}
	if strings.ContainsAny(base, "\r\n \t") {
		t.Fatal("proof image must be one Docker image reference")
	}
	// The build tool client asks the daemon this process's environment names,
	// the one Build's own docker commands reach.
	docker := interoplab.NewBuildToolDocker()
	if present, err := docker.ImagePresent(t.Context(), base); err != nil || !present {
		t.Fatalf("required locally cached proof image unavailable: present=%v, %v", present, err)
	}
	req := kernelbuilder.DockerProofRequest(t)
	identity, err := os.CreateTemp(req.Root, "identity-")
	if err != nil {
		t.Fatal(err)
	}
	if err := identity.Close(); err != nil {
		t.Fatal(err)
	}
	suffix := filepath.Base(identity.Name())
	req.Image = "ze-kernel-lifetime-" + suffix
	if err := os.MkdirAll(filepath.Join(req.Root, req.BuilderDir), 0o700); err != nil {
		t.Fatal(err)
	}
	worker := `#!/bin/sh
set -eu
if [ "$1" = chown ]; then
  before=$(cat /out/heartbeat)
  sleep 0.3
  after=$(cat /out/heartbeat)
  if [ "$before" != "$after" ]; then
    echo 'kernel worker still running during output ownership repair' >&2
    exit 73
  fi
  touch /out/repaired
  exit 0
fi
trap '' TERM
count=0
while :; do
  count=$((count+1))
  echo "$count" > /out/heartbeat
  sleep 0.05
done
`
	kernelbuilder.WriteProofFixture(t, req.Root, filepath.Join(req.BuilderDir, "worker"), worker)
	dockerfile := "FROM " + base + "\nCOPY " + req.BuilderDir + "/worker /proof-worker\nRUN chmod 755 /proof-worker\nENTRYPOINT [\"/proof-worker\"]\n"
	kernelbuilder.WriteProofFixture(t, req.Root, filepath.Join(req.BuilderDir, "Dockerfile"), dockerfile)
	foreign := "ze-kernel-foreign-" + suffix
	sentinel := interoplab.DetachedContainer{
		Name: foreign, Image: base, Entrypoint: "/bin/sh",
		Command: []string{"-c", "while :; do sleep 1; done"},
	}
	if err := docker.RunDetached(t.Context(), sentinel); err != nil {
		t.Fatalf("start foreign sentinel: %v", err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 20*time.Second)
		defer cancel()
		if err := docker.RemoveContainer(cleanup, foreign); err != nil {
			t.Errorf("remove fixture sentinel: %v", err)
		}
		// The fixture owns this unique image and every container using it.
		ids, err := docker.ContainersFromImage(cleanup, req.Image)
		if err != nil {
			t.Error(err)
		}
		for _, id := range ids {
			if err := docker.RemoveContainer(cleanup, id); err != nil {
				t.Errorf("remove fixture worker: %v", err)
			}
		}
		if err := docker.RemoveImageTagForTest(cleanup, req.Image); err != nil {
			t.Errorf("remove fixture image: %v", err)
		}
	}()
	// Space policy is not under test; preserve actual Docker commands and daemon.
	kernelbuilder.PinProofFreeSpace(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- kernelbuilder.Build(ctx, req) }()
	heartbeat := filepath.Join(req.Root, req.OutputDir, "heartbeat")
	until := time.NewTimer(60 * time.Second)
	defer until.Stop()
	poll := time.NewTicker(20 * time.Millisecond)
	defer poll.Stop()
started:
	for {
		select {
		case err := <-done:
			t.Fatalf("build did not reach the actual container worker: %v", err)
		case <-until.C:
			t.Fatal("actual Docker container did not reach heartbeat boundary")
		case <-poll.C:
			if body, err := os.ReadFile(heartbeat); err == nil && strings.TrimSpace(string(body)) != "" {
				break started
			}
		}
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("Build lost its originating cancellation: %v", err)
		}
		if err == nil {
			t.Error("canceled kernel build reported success")
		}
	case <-time.After(20 * time.Second):
		t.Fatal("Build did not finish owned-container teardown")
	}
	if _, err := os.Stat(filepath.Join(req.Root, req.OutputDir, "repaired")); err != nil {
		t.Errorf("ownership repair did not observe a joined worker: %v", err)
	}
	survivors, err := docker.ContainersFromImage(t.Context(), req.Image)
	if err != nil {
		t.Fatal(err)
	}
	if len(survivors) != 0 {
		t.Errorf("owned container survived Build verdict: %v", survivors)
	}
	if running, err := docker.ContainerRunning(t.Context(), foreign); err != nil || !running {
		t.Errorf("Build touched foreign sentinel: running=%v, %v", running, err)
	}
}
