//go:build linux || darwin

package appliance_test

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/appliance"
	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// proofWorker is the container entrypoint the fixture builder image runs.
// As the build worker ($1 is ze-kernel-builder) it ignores TERM and writes a
// heartbeat through the output mount forever, so only a daemon-side removal
// stops it. As the ownership-repair worker ($1 is chown) it refuses when the
// heartbeat still moves: the build worker was not joined before the repair.
const proofWorker = `#!/bin/sh
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

// TestKernelCommandInterruptRemovesOwnedContainers proves the operator's stop
// of `ze appliance kernel` on the Docker builder, through the command's own
// interrupt handling: SIGINT during the build ends the command non-zero, with
// the stop named and no "kernel ready" verdict, after every container the
// build created is removed, while a container the build did not create keeps
// running.
//
// Method: runKernel runs in this process over a minimal installer registry in a
// temporary root, through defaultKernelBuild (which installs the interrupt
// context) and kernelbuilder.Build against the actual Docker daemon. The only
// substitution is the builder image's tag, made unique so the shared
// ze-kernel-builder image is never retagged, and the builder Dockerfile, which
// packages proofWorker on KERNEL_CONTAINER_PROOF_IMAGE: an existing image with
// /bin/sh, sleep and chown. No image is pulled; without the variable the test
// skips. Once the worker's heartbeat appears, the test sends SIGINT to its own
// process. Its own signal.Notify keeps an unhandled interrupt from killing the
// test binary, so a route that lost its handler fails on the timeout instead.
// Every docker command the test itself issues goes through interoplab.Docker,
// so the foreign sentinel carries the owner labels `./le test integration
// clean` judges; the build's containers are the code under test.
//
// Sibling: kernelbuilder TestKernelContainerCancellationJoinsBeforeRepair
// proves the same teardown from a canceled context at Build, below the command.
//
// MUTATION: in kernelbuilder.runContainer, return (runErr, nil) when ctx.Err()
// is set, before the `docker rm --force`: the CLI dies, the worker keeps
// beating, the repair refuses, and this test reports the repair did not
// observe a joined worker and the owned container survived.
// MUTATION: in defaultKernelBuild, build from context.Background() instead of
// signal.NotifyContext: the interrupt stops nothing, and this test reports the
// command did not stop.
func TestKernelCommandInterruptRemovesOwnedContainers(t *testing.T) {
	base := os.Getenv("KERNEL_CONTAINER_PROOF_IMAGE")
	if base == "" {
		t.Skip("real Docker interrupt proof requires KERNEL_CONTAINER_PROOF_IMAGE naming a locally cached shell image")
	}
	if strings.ContainsAny(base, "\r\n \t") {
		t.Fatal("proof image must be one Docker image reference")
	}
	// The build tool client asks the daemon this process's environment names,
	// the one the build's own docker commands reach.
	docker := interoplab.NewBuildToolDocker()
	if present, err := docker.ImagePresent(t.Context(), base); err != nil || !present {
		t.Fatalf("required locally cached proof image unavailable: present=%v, %v", present, err)
	}

	root := t.TempDir()
	t.Chdir(root)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("ZE_APPLIANCE_KERNEL_URL", "")
	env.ResetCache()
	appliance.WriteInstallerKernelRegistryForTest(t)
	identity, err := os.CreateTemp(root, "identity-")
	if err != nil {
		t.Fatal(err)
	}
	if err := identity.Close(); err != nil {
		t.Fatal(err)
	}
	suffix := strings.ToLower(filepath.Base(identity.Name()))
	image := "ze-kernel-cancel-" + suffix
	foreign := "ze-kernel-foreign-" + suffix
	writeCancelFixture(t, root, base)

	outputs := make(chan string, 1)
	appliance.UseNativeKernelBuildForTest(t, func(ctx context.Context, req kernelbuilder.Request) error {
		req.Image = image
		output := req.OutputDir
		if !filepath.IsAbs(output) {
			output = filepath.Join(req.Root, output)
		}
		outputs <- output
		return kernelbuilder.Build(ctx, req)
	})
	// Space policy is not under test; preserve actual Docker commands and daemon.
	restoreStore := kernelbuilder.PinNoContainerStore()
	t.Cleanup(restoreStore)

	sentinel := interoplab.DetachedContainer{
		Name: foreign, Image: base, Entrypoint: "/bin/sh",
		Command: []string{"-c", "while :; do sleep 1; done"},
	}
	if err := docker.RunDetached(t.Context(), sentinel); err != nil {
		t.Fatalf("start foreign sentinel: %v", err)
	}

	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)

	stdout, stderr, restoreOutput := captureCommandOutput(t)
	done := make(chan int, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		done <- appliance.RunKernelForTest([]string{"--builder", "docker", "--arch", kernelbuilder.HostArch()})
	}()
	defer removeCancelFixture(t, docker, foreign, image, finished, restoreOutput)

	waitHeartbeat(t, outputs, done)
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatalf("send interrupt: %v", err)
	}
	var code int
	select {
	case code = <-done:
	case <-time.After(20 * time.Second):
		restoreOutput()
		t.Fatal("interrupt did not stop `ze appliance kernel` within 20s")
	}
	restoreOutput()

	answer, report := readFile(t, stdout), readFile(t, stderr)
	if code != appliance.ExitErrorForTest {
		t.Errorf("interrupted `ze appliance kernel` exit = %d, want %d", code, appliance.ExitErrorForTest)
	}
	if strings.Contains(answer, "kernel ready") {
		t.Errorf("interrupted build reported success:\n%s", answer)
	}
	if !strings.Contains(report, "the kernel build was canceled") {
		t.Errorf("interrupted build did not name the stop:\n%s", report)
	}
	output := <-outputs
	if _, err := os.Stat(filepath.Join(output, "repaired")); err != nil {
		t.Errorf("ownership repair did not observe a joined worker: %v\n%s", err, report)
	}
	survivors, err := docker.ContainersFromImage(t.Context(), image)
	if err != nil {
		t.Fatal(err)
	}
	if len(survivors) != 0 {
		t.Errorf("owned container survived the command's verdict: %v", survivors)
	}
	if running, err := docker.ContainerRunning(t.Context(), foreign); err != nil || !running {
		t.Errorf("interrupted build touched the foreign sentinel: running=%v, %v", running, err)
	}
}

// writeCancelFixture writes the builder Dockerfile and its worker where the
// build reads them, and the common directory the worker mounts: a bind mount
// of a missing host path would be created by the daemon, owned by root.
func writeCancelFixture(t *testing.T, root, base string) {
	t.Helper()
	builder := filepath.Join(root, kernelbuilder.DefaultBuilderDir)
	if err := os.MkdirAll(filepath.Join(root, kernelbuilder.DefaultCommonDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(builder, "worker"), []byte(proofWorker), 0o755); err != nil { //nolint:gosec // G306: the container must execute the worker.
		t.Fatal(err)
	}
	dockerfile := "FROM " + base + "\nCOPY " + kernelbuilder.DefaultBuilderDir + "/worker /proof-worker\nRUN chmod 755 /proof-worker\nENTRYPOINT [\"/proof-worker\"]\n"
	if err := os.WriteFile(filepath.Join(builder, "Dockerfile"), []byte(dockerfile), 0o644); err != nil { //nolint:gosec // G306: a build input, not a secret.
		t.Fatal(err)
	}
}

// waitHeartbeat returns once the build worker has written its heartbeat, so
// the interrupt lands while the owned container runs.
func waitHeartbeat(t *testing.T, outputs chan string, done chan int) {
	t.Helper()
	var output string
	select {
	case output = <-outputs:
		outputs <- output
	case code := <-done:
		t.Fatalf("`ze appliance kernel` ended with %d before the native build started", code)
	case <-time.After(60 * time.Second):
		t.Fatal("`ze appliance kernel` did not reach the native build")
	}
	heartbeat := filepath.Join(output, "heartbeat")
	until := time.NewTimer(60 * time.Second)
	defer until.Stop()
	poll := time.NewTicker(20 * time.Millisecond)
	defer poll.Stop()
	for {
		select {
		case code := <-done:
			t.Fatalf("`ze appliance kernel` ended with %d before the container worker ran", code)
		case <-until.C:
			t.Fatal("the build's container did not reach the heartbeat boundary")
		case <-poll.C:
			if body, err := os.ReadFile(heartbeat); err == nil && strings.TrimSpace(string(body)) != "" {
				return
			}
		}
	}
}

// removeCancelFixture restores the test's output, then removes the sentinel,
// every container of the fixture's own image, and the image. A command that is
// still running (a stop that did not stop it) is unblocked by that removal and
// waited for before the image goes, so no build outlives the test.
func removeCancelFixture(t *testing.T, docker *interoplab.Docker, foreign, image string, finished chan struct{}, restoreOutput func()) {
	t.Helper()
	restoreOutput()
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 30*time.Second)
	defer cancel()
	if err := docker.RemoveContainer(cleanup, foreign); err != nil {
		t.Errorf("remove fixture sentinel: %v", err)
	}
	ids, err := docker.ContainersFromImage(cleanup, image)
	if err != nil {
		t.Error(err)
	}
	for _, id := range ids {
		if err := docker.RemoveContainer(cleanup, id); err != nil {
			t.Errorf("remove fixture worker: %v", err)
		}
	}
	select {
	case <-finished:
	case <-cleanup.Done():
		t.Error("`ze appliance kernel` still running after its containers were removed")
	}
	if err := docker.RemoveImageTagForTest(cleanup, image); err != nil {
		t.Errorf("remove fixture image: %v", err)
	}
}

// captureCommandOutput points os.Stdout and os.Stderr at files until restore
// is called, so the command's answer and its refusal can be read apart.
func captureCommandOutput(t *testing.T) (stdout, stderr string, restore func()) {
	t.Helper()
	dir := t.TempDir()
	stdout, stderr = filepath.Join(dir, "stdout"), filepath.Join(dir, "stderr")
	out, err := os.Create(stdout)
	if err != nil {
		t.Fatal(err)
	}
	errOut, err := os.Create(stderr)
	if err != nil {
		t.Fatal(err)
	}
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = out, errOut
	restored := false
	restore = func() {
		if restored {
			return
		}
		restored = true
		os.Stdout, os.Stderr = oldOut, oldErr
		if err := errors.Join(out.Close(), errOut.Close()); err != nil {
			t.Errorf("close captured output: %v", err)
		}
	}
	t.Cleanup(restore)
	return stdout, stderr, restore
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path) //nolint:gosec // G304: the test's own capture file.
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
