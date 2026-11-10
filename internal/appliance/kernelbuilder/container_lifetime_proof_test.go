//go:build linux || darwin

package kernelbuilder

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestKernelContainerCancellationJoinsBeforeRepair runs Build against an actual
// Docker daemon and a small, locally cached shell image. A blocked worker writes
// a heartbeat through its mounted output directory. The ownership-repair worker
// independently inspects that heartbeat: it refuses if the original worker is
// still running. A separately created container must survive. This proves the
// daemon boundary rather than treating the Docker CLI's death as a container stop.
// KERNEL_CONTAINER_PROOF_IMAGE names an existing image with /bin/sh, sleep and
// chown; no image is pulled. This is lifecycle proof, not kernel or guest proof.
func TestKernelContainerCancellationJoinsBeforeRepair(t *testing.T) {
	base := os.Getenv("KERNEL_CONTAINER_PROOF_IMAGE")
	if base == "" {
		t.Skip("real Docker lifecycle proof requires KERNEL_CONTAINER_PROOF_IMAGE naming a locally cached shell image")
	}
	if strings.ContainsAny(base, "\r\n \t") {
		t.Fatal("proof image must be one Docker image reference")
	}
	probe := exec.CommandContext(t.Context(), "docker", "image", "inspect", base)
	if output, err := probe.CombinedOutput(); err != nil {
		t.Fatalf("required locally cached proof image unavailable: %v\n%s", err, output)
	}
	req := qemuBuildRequest(t, backendDocker)
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
	writeFixture(t, req.Root, filepath.Join(req.BuilderDir, "worker"), worker)
	dockerfile := "FROM " + base + "\nCOPY " + req.BuilderDir + "/worker /proof-worker\nRUN chmod 755 /proof-worker\nENTRYPOINT [\"/proof-worker\"]\n"
	writeFixture(t, req.Root, filepath.Join(req.BuilderDir, "Dockerfile"), dockerfile)
	foreign := "ze-kernel-foreign-" + suffix
	if output, err := exec.CommandContext(t.Context(), "docker", "run", "-d", "--name", foreign, "--entrypoint", "/bin/sh", base, "-c", "while :; do sleep 1; done").CombinedOutput(); err != nil {
		t.Fatalf("start foreign sentinel: %v\n%s", err, output)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 20*time.Second)
		defer cancel()
		if output, err := exec.CommandContext(cleanup, "docker", "rm", "-f", foreign).CombinedOutput(); err != nil {
			t.Errorf("remove fixture sentinel: %v\n%s", err, output)
		}
		// The fixture owns this unique image and every container using it.
		output, err := exec.CommandContext(cleanup, "docker", "ps", "-aq", "--filter", "ancestor="+req.Image).Output()
		if err != nil {
			t.Error(err)
		}
		for id := range strings.FieldsSeq(string(output)) {
			if output, err := exec.CommandContext(cleanup, "docker", "rm", "-f", id).CombinedOutput(); err != nil {
				t.Errorf("remove fixture worker: %v\n%s", err, output)
			}
		}
		if output, err := exec.CommandContext(cleanup, "docker", "image", "rm", req.Image).CombinedOutput(); err != nil {
			t.Errorf("remove fixture image: %v\n%s", err, output)
		}
	}()
	// Space policy is not under test; preserve actual Docker commands and daemon.
	oldFree := freeSpace
	freeSpace = func(string) (uint64, error) { return kernelBuildFreeBytesMin, nil }
	defer func() { freeSpace = oldFree }()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Build(ctx, req) }()
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
	output, err := exec.CommandContext(t.Context(), "docker", "ps", "-aq", "--filter", "ancestor="+req.Image).Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(output)) != "" {
		t.Errorf("owned container survived Build verdict: %s", output)
	}
	output, err = exec.CommandContext(t.Context(), "docker", "inspect", "--format", "{{.State.Running}}", foreign).Output()
	if err != nil || strings.TrimSpace(string(output)) != "true" {
		t.Errorf("Build touched foreign sentinel: %q, %v", output, err)
	}
}
