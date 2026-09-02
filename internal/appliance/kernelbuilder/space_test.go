package kernelbuilder

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diskspace"
)

// VALIDATES: the guard refuses below the floor and passes above it, and its
// message carries both numbers an operator compares against their own df.
// PREVENTS: a twenty-minute build that ends with the host volume at 100% and
// the container runtime's image store corrupted, which is what happened on
// 2026-09-02 (plan/journal/full-disk-false-red.md).
func TestKernelBuildSpaceErrorNamesBothNumbers(t *testing.T) {
	err := kernelBuildSpaceError(6*diskspace.BytesPerGiB, "/host/store")
	if err == nil {
		t.Fatal("6G free passed a guard that needs 40G")
	}
	for _, want := range []string{"6.0G", "40.0G", "/host/store"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q does not name %q", err, want)
		}
	}

	if err := kernelBuildSpaceError(kernelBuildFreeBytesMin, "/host/store"); err != nil {
		t.Errorf("exactly the floor was refused: %v", err)
	}
	if err := kernelBuildSpaceError(400*diskspace.BytesPerGiB, "/host/store"); err != nil {
		t.Errorf("400G free was refused: %v", err)
	}
}

// VALIDATES: the guard measures the HOST filesystem that receives the writes,
// which on a Mac is the VM's sparse disk under the user's home and NOT
// /var/lib/docker.
// PREVENTS: statfs answering about the wrong device, the same mistake `df` on
// the checkout makes about cache/ (internal/le/scratch/cacheclean.go).
func TestDockerStoreHostPathPrefersTheVMDisk(t *testing.T) {
	present := func(paths ...string) func(string) bool {
		set := make(map[string]bool, len(paths))
		for _, p := range paths {
			set[p] = true
		}
		return func(p string) bool { return set[p] }
	}

	if got := dockerStoreHostPath("/home/t", present("/home/t/.colima", "/var/lib/docker")); got != "/home/t/.colima" {
		t.Errorf("with a colima VM present, got %q, want the VM disk", got)
	}
	if got := dockerStoreHostPath("/home/t", present("/var/lib/docker")); got != "/var/lib/docker" {
		t.Errorf("with a native daemon, got %q, want /var/lib/docker", got)
	}
	if got := dockerStoreHostPath("/home/t", present()); got != "" {
		t.Errorf("an unknown layout answered %q; it must answer the empty string so the caller can say the guard did not run", got)
	}
}

// VALIDATES: an unknown layout does not silently pass as "enough space".
// PREVENTS: a zero value reading as a valid answer, which is the failure
// ai/rules/principles.md names first.
func TestCheckKernelBuildSpaceSaysWhenItCouldNotMeasure(t *testing.T) {
	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer stdout.Close() //nolint:errcheck // test scratch file

	req := Request{Stdout: stdout, Stderr: stdout}
	if err := checkKernelBuildSpace(req, func(string) bool { return false }); err != nil {
		t.Fatalf("an unmeasurable layout blocked the build: %v", err)
	}

	if _, err := stdout.Seek(0, 0); err != nil {
		t.Fatalf("seek: %v", err)
	}
	said, err := os.ReadFile(stdout.Name())
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(said), "free space not checked") {
		t.Errorf("the guard skipped silently; output was %q", said)
	}
}

// VALIDATES: the reclaim runs for a VM-backed runtime and is skipped where the
// daemon writes the host filesystem directly.
// PREVENTS: a sparse VM disk that only ever grows. Deleting files inside the
// guest returns nothing to the host until the guest discards the blocks.
func TestReclaimRunsOnlyForAVMBackedRuntime(t *testing.T) {
	old := runCommand
	t.Cleanup(func() { runCommand = old })
	var calls [][]string
	runCommand = func(_ context.Context, _ Request, name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		return nil
	}

	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer stdout.Close() //nolint:errcheck // test scratch file
	req := Request{Stdout: stdout, Stderr: stdout}

	reclaimBuildSpace(context.Background(), req, "/home/t/.colima")
	if len(calls) != 1 {
		t.Fatalf("VM-backed runtime ran %d commands, want 1: %v", len(calls), calls)
	}
	if calls[0][0] != "colima" || !strings.Contains(strings.Join(calls[0], " "), "fstrim") {
		t.Errorf("reclaim ran %v, want a colima fstrim", calls[0])
	}

	calls = nil
	reclaimBuildSpace(context.Background(), req, "/var/lib/docker")
	if len(calls) != 0 {
		t.Errorf("a native daemon needs no discard, but ran %v", calls)
	}
}

// pinHostWithoutAContainerStore makes the space guard and the reclaim step
// no-ops, so a test that counts docker calls counts runDocker's own and not the
// dev machine's container layout.
func pinHostWithoutAContainerStore(t *testing.T) {
	t.Helper()
	oldExists, oldFree := pathExists, freeSpace
	t.Cleanup(func() { pathExists, freeSpace = oldExists, oldFree })
	pathExists = func(string) bool { return false }
	freeSpace = func(string) (uint64, error) { return 0, nil }
}

// VALIDATES: runDocker returns the freed space to the host as its last act, on
// a VM-backed runtime.
// PREVENTS: the reclaim existing but never being called, which is how the
// bound this class asked for in 2026-08-18 came to exist and never run
// (plan/journal/full-disk-false-red.md).
func TestRunDockerReturnsSpaceAfterTheBuild(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "configs/kernel.config", "CONFIG_A=y\n")
	writeFixture(t, root, "configs/kernel.require", "CONFIG_A\n")
	writeFixture(t, root, "tools/kernel-builder/Dockerfile", "FROM scratch\n")

	oldExists, oldFree, oldAvailable, oldRun := pathExists, freeSpace, commandAvailable, runCommand
	t.Cleanup(func() {
		pathExists, freeSpace, commandAvailable, runCommand = oldExists, oldFree, oldAvailable, oldRun
	})
	pathExists = func(path string) bool { return strings.HasSuffix(path, colimaVMDir) }
	freeSpace = func(string) (uint64, error) { return 400 * diskspace.BytesPerGiB, nil }
	commandAvailable = func(name string) bool { return name == "colima" }

	var calls [][]string
	runCommand = func(_ context.Context, _ Request, name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		return nil
	}

	req := Request{Root: root, Version: "7.1.1", Arch: "amd64", Profile: "qemu", Builder: "docker", Target: "installer", SourceDir: "configs", OutputDir: "out", BuilderDir: "tools/kernel-builder", CommonDir: "common", Modules: "no", Fragments: []string{"configs/kernel.config"}, Image: "fixture", Stdout: os.Stdout, Stderr: os.Stderr}
	if err := validateRequest(&req); err != nil {
		t.Fatal(err)
	}
	if err := runDocker(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	if len(calls) == 0 {
		t.Fatal("runDocker ran no commands")
	}
	last := strings.Join(calls[len(calls)-1], " ")
	if last != "colima ssh -- sudo fstrim /var/lib/docker" {
		t.Errorf("last call = %q, want the host reclaim", last)
	}
}
