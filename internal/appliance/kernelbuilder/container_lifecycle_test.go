package kernelbuilder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/diskspace"
)

// pinDockerCreateOutput records create through the same seam as start/remove,
// while answering with a distinct full ID derived from each unique name.
// Non-create commands MUST retain the previously installed probe responder.
func pinDockerCreateOutput(t *testing.T) {
	t.Helper()
	old := runCommandOutput
	t.Cleanup(func() { runCommandOutput = old })
	runCommandOutput = func(ctx context.Context, req Request, name string, args ...string) ([]byte, error) {
		if len(args) == 0 {
			return old(ctx, req, name, args...)
		}
		if args[0] != "create" {
			return old(ctx, req, name, args...)
		}
		if len(args) < 3 {
			t.Fatalf("unexpected Docker creation argv: %v", args)
		}
		if !slices.Equal(args[:2], []string{"create", "--name"}) {
			t.Fatalf("unexpected Docker creation argv: %v", args)
		}
		if err := runCommand(ctx, req, name, args...); err != nil {
			return nil, err
		}
		return []byte(fixtureContainerID(args[2]) + "\n"), nil
	}
}

func fixtureContainerID(name string) string {
	digest := sha256.Sum256([]byte(name))
	return hex.EncodeToString(digest[:])
}

// slowestContainerCreation is the slowest `docker create` of the
// ze-kernel-builder image measured on a loaded Linux host (25.2s), the run that
// a 15s host-probe bound killed while Docker went on to create the container.
const slowestContainerCreation = 25200 * time.Millisecond

// TestRunContainerCancellationDuringCreateRetainsIdentity cancels at the
// acquisition boundary and proves removal uses the returned full ID, without
// starting the worker or inheriting the caller's canceled context.
func TestRunContainerCancellationDuringCreateRetainsIdentity(t *testing.T) {
	oldRun, oldOutput := runCommand, runCommandOutput
	t.Cleanup(func() { runCommand, runCommandOutput = oldRun, oldOutput })
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	const name = "ze-kernel-acquisition-fixture"
	id := fixtureContainerID(name)
	runCommandOutput = func(createCtx context.Context, _ Request, _ string, args ...string) ([]byte, error) {
		cancel()
		if err := createCtx.Err(); err != nil {
			t.Fatalf("caller cancellation reached acquisition: %v", err)
		}
		if deadline, ok := createCtx.Deadline(); !ok || time.Until(deadline) > containerLifecycleTimeout {
			t.Fatal("container creation has no lifecycle bound")
		}
		if deadline, _ := createCtx.Deadline(); time.Until(deadline) < slowestContainerCreation {
			t.Fatalf("container creation bound is shorter than the %s a loaded host took to create the builder container", slowestContainerCreation)
		}
		if !slices.Equal(args, []string{"create", "--name", name, "fixture"}) {
			t.Fatalf("creation argv = %v", args)
		}
		return []byte(id), nil
	}
	removed := false
	runCommand = func(removeCtx context.Context, _ Request, _ string, args ...string) error {
		if !slices.Equal(args, []string{"rm", "--force", id}) {
			t.Fatalf("canceled acquisition started or removed another container: %v", args)
		}
		if err := removeCtx.Err(); err != nil {
			t.Fatalf("caller cancellation reached removal: %v", err)
		}
		if deadline, ok := removeCtx.Deadline(); !ok || time.Until(deadline) > containerLifecycleTimeout {
			t.Fatal("container removal has no lifecycle bound")
		}
		if deadline, _ := removeCtx.Deadline(); time.Until(deadline) < slowestContainerCreation {
			t.Fatalf("container removal bound is shorter than the %s a loaded host took to create the builder container", slowestContainerCreation)
		}
		removed = true
		return nil
	}
	runErr, teardownErr := runContainer(ctx, Request{}, name, []string{"create", "--name", name, "fixture"})
	if !errors.Is(runErr, context.Canceled) {
		t.Errorf("acquisition cancellation = %v", runErr)
	}
	if teardownErr != nil {
		t.Fatalf("owned container removal: %v", teardownErr)
	}
	if !removed {
		t.Fatal("cancellation lost the acquired container")
	}
}

// TestRunDockerRefusesRepairWhenContainerRemovalUnknown makes the daemon refuse
// exact removal after a canceled worker, and proves neither chown nor disk
// reclaim follows an unconfirmed worker lifetime.
func TestRunDockerRefusesRepairWhenContainerRemovalUnknown(t *testing.T) {
	req := qemuBuildRequest(t, backendDocker)
	req.Image = "fixture"
	oldExists, oldFree, oldAvailable, oldRun := pathExists, freeSpace, commandAvailable, runCommand
	t.Cleanup(func() {
		pathExists, freeSpace, commandAvailable, runCommand = oldExists, oldFree, oldAvailable, oldRun
	})
	pathExists = func(path string) bool { return strings.HasSuffix(path, colimaVMDir) }
	freeSpace = func(string) (uint64, error) { return 400 * diskspace.BytesPerGiB, nil }
	commandAvailable = func(name string) bool { return name == "colima" }
	pinDockerCreateOutput(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	removalFailure := errors.New("daemon removal response unavailable")
	var calls [][]string
	runCommand = func(callCtx context.Context, _ Request, name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		if args[0] == "start" {
			cancel()
			return ctx.Err()
		}
		if args[0] == "rm" {
			if err := callCtx.Err(); err != nil {
				t.Fatalf("removal inherited cancellation: %v", err)
			}
			return removalFailure
		}
		return nil
	}
	err := runDocker(ctx, req)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("lost originating cancellation: %v", err)
	}
	if !errors.Is(err, removalFailure) {
		t.Fatalf("lost teardown refusal: %v", err)
	}
	if len(calls) != 4 {
		t.Fatalf("repair or reclaim followed unknown removal: %v", calls)
	}
	id := fixtureContainerID(calls[1][3])
	if !slices.Equal(calls[3], []string{"docker", "rm", "--force", id}) {
		t.Fatalf("removal did not name the owned full ID: %v", calls[3])
	}
	if !strings.HasSuffix(err.Error(), "docker rm --force "+id) {
		t.Fatalf("refusal lacks exact operator repair: %v", err)
	}
}

// TestRunDockerRefusesUnknownCreation keeps failed or malformed daemon identity
// responses distinct from absence: there must be no start, chown or removal
// guessed from a name, image or shortened ID.
func TestRunDockerRefusesUnknownCreation(t *testing.T) {
	creationFailure := errors.New("daemon creation response unavailable")
	for _, tc := range []struct {
		name   string
		output string
		err    error
		hang   bool
	}{
		{name: "missing ID"},
		{name: "short ID", output: "123456"},
		{name: "invalid full ID", output: strings.Repeat("z", 64)},
		{name: "creation failed", err: creationFailure},
		{name: "creation overran its bound", hang: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := qemuBuildRequest(t, backendDocker)
			req.Image = "fixture"
			pinHostWithoutAContainerStore(t)
			oldRun, oldOutput, oldBound := runCommand, runCommandOutput, containerLifecycleTimeout
			t.Cleanup(func() { runCommand, runCommandOutput, containerLifecycleTimeout = oldRun, oldOutput, oldBound })
			if tc.hang {
				containerLifecycleTimeout = 200 * time.Millisecond
			}
			calls := 0
			runCommand = func(_ context.Context, _ Request, _ string, args ...string) error {
				calls++
				if args[0] != "build" {
					t.Fatalf("unknown creation reached worker, repair or removal: %v", args)
				}
				return nil
			}
			var name string
			runCommandOutput = func(ctx context.Context, req Request, binary string, args ...string) ([]byte, error) {
				if args[0] != "create" {
					return oldOutput(ctx, req, binary, args...)
				}
				name = args[2]
				if tc.hang {
					// Plays a Docker CLI that outlives the bound: killed at
					// the deadline, as exec reports it.
					<-ctx.Done()
					return nil, errors.New("signal: killed")
				}
				return []byte(tc.output), tc.err
			}
			err := runDocker(t.Context(), req)
			if err == nil {
				t.Fatal("unknown creation reported success")
			}
			if !strings.Contains(err.Error(), "ownership repair and disk reclaim were refused") {
				t.Errorf("unknown lifetime was not refused: %v", err)
			}
			if !strings.HasSuffix(err.Error(), "docker container inspect "+name) {
				t.Errorf("unknown creation lacks exact inspection command: %v", err)
			}
			if tc.err != nil {
				if !errors.Is(err, tc.err) {
					t.Errorf("lost creation cause: %v", err)
				}
			}
			if tc.hang {
				if !strings.Contains(err.Error(), "no answer within the 200ms limit: signal: killed") {
					t.Errorf("an overrun creation is not named as one: %v", err)
				}
			}
			if calls != 1 {
				t.Fatalf("unknown creation reached more than image build: %d calls", calls)
			}
		})
	}
}

// TestRunContainerPreservesWorkerOutcome keeps normal completion and worker
// failure independent of the mandatory successful exact-ID teardown.
func TestRunContainerPreservesWorkerOutcome(t *testing.T) {
	failure := errors.New("worker exited unsuccessfully")
	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "completed"},
		{name: "worker failed", err: failure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oldRun, oldOutput := runCommand, runCommandOutput
			t.Cleanup(func() { runCommand, runCommandOutput = oldRun, oldOutput })
			const name = "ze-kernel-outcome-fixture"
			id := fixtureContainerID(name)
			runCommandOutput = func(context.Context, Request, string, ...string) ([]byte, error) {
				return []byte(id), nil
			}
			var calls [][]string
			runCommand = func(_ context.Context, _ Request, _ string, args ...string) error {
				calls = append(calls, slices.Clone(args))
				if args[0] == "start" {
					return tc.err
				}
				return nil
			}
			runErr, teardownErr := runContainer(t.Context(), Request{}, name, []string{"create", "--name", name, "fixture"})
			if !errors.Is(runErr, tc.err) {
				t.Errorf("worker outcome = %v, want %v", runErr, tc.err)
			}
			if teardownErr != nil {
				t.Fatalf("normal teardown = %v", teardownErr)
			}
			if len(calls) != 2 {
				t.Fatalf("container lifecycle = %v", calls)
			}
			if !slices.Equal(calls[0], []string{"start", "--attach", id}) {
				t.Errorf("worker attachment = %v", calls[0])
			}
			if !slices.Equal(calls[1], []string{"rm", "--force", id}) {
				t.Errorf("worker removal = %v", calls[1])
			}
		})
	}
}

// TestDockerCreateMockPreservesDaemonProbe drives dockerRootDir through the
// creation mock and its cleanup, retaining the previously installed responder.
func TestDockerCreateMockPreservesDaemonProbe(t *testing.T) {
	old := runCommandOutput
	t.Cleanup(func() { runCommandOutput = old })
	root := t.TempDir()
	probes := 0
	runCommandOutput = func(_ context.Context, _ Request, name string, args ...string) ([]byte, error) {
		if name != backendDocker {
			t.Fatalf("unexpected probe binary: %s", name)
		}
		if !slices.Equal(args, []string{"info", "--format", "{{.DockerRootDir}}"}) {
			t.Fatalf("unexpected daemon probe argv: %v", args)
		}
		probes++
		return []byte(root), nil
	}
	t.Run("creation mock installed", func(t *testing.T) {
		pinDockerCreateOutput(t)
		got, err := dockerRootDir(t.Context(), Request{})
		if err != nil {
			t.Fatalf("creation mock intercepted daemon probe: %v", err)
		}
		if got != root {
			t.Errorf("daemon root = %s, want %s", got, root)
		}
	})
	got, err := dockerRootDir(t.Context(), Request{})
	if err != nil {
		t.Fatalf("creation mock cleanup lost the earlier responder: %v", err)
	}
	if got != root {
		t.Errorf("restored daemon root = %s, want %s", got, root)
	}
	if probes != 2 {
		t.Errorf("earlier daemon responder received %d probes, want 2", probes)
	}
}
