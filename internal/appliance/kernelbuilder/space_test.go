package kernelbuilder

import (
	"context"
	"errors"
	"os"
	"slices"
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
	store := containerStore{path: "/host/store", why: "the fixture says so"}
	req := Request{Target: "installer", Arch: "amd64", Profile: "qemu"}
	err := kernelBuildSpaceError(6*diskspace.BytesPerGiB, store, req)
	if err == nil {
		t.Fatal("6G free passed a guard that needs 40G")
	}
	for _, want := range []string{"6.0G", "40.0G", "/host/store", "the fixture says so"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q does not name %q", err, want)
		}
	}
	// ai/rules/user-facing-errors.md: the command that fixes it on a line of
	// its own, so an operator can copy it.
	lines := strings.Split(err.Error(), "\n")
	for _, want := range []string{"  df -h /host/store", "  ze appliance kernel --builder qemu --target installer --arch amd64 --profile qemu"} {
		if !slices.Contains(lines, want) {
			t.Errorf("message %q has no line %q", err, want)
		}
	}

	if err := kernelBuildSpaceError(kernelBuildFreeBytesMin, store, req); err != nil {
		t.Errorf("exactly the floor was refused: %v", err)
	}
	if err := kernelBuildSpaceError(400*diskspace.BytesPerGiB, store, req); err != nil {
		t.Errorf("400G free was refused: %v", err)
	}
}

// present answers true for exactly the paths it was given.
func present(paths ...string) func(string) bool {
	set := make(map[string]bool, len(paths))
	for _, p := range paths {
		set[p] = true
	}
	return func(p string) bool { return set[p] }
}

// VALIDATES: the guard measures the HOST filesystem that receives the writes,
// which on a Mac is the VM's sparse disk under the user's home and NOT
// /var/lib/docker.
// PREVENTS: statfs answering about the wrong device, the same mistake `df` on
// the checkout makes about cache/ (internal/le/scratch/cacheclean.go).
func TestDockerStoreHostPathPrefersTheVMDisk(t *testing.T) {
	noDaemon := func() (string, error) { return "", errors.New("daemon not running") }

	if got := dockerStoreHostPath("/home/t", "linux", present("/home/t/.colima", "/var/lib/docker"), noDaemon); got.path != "/home/t/.colima" || !got.vmDisk {
		t.Errorf("with a colima VM present, got %+v, want the VM disk", got)
	}
	got := dockerStoreHostPath("/home/t", "linux", present("/var/lib/docker"), noDaemon)
	if got.path != "/var/lib/docker" || got.vmDisk {
		t.Errorf("with a native daemon that cannot answer, got %+v, want /var/lib/docker", got)
	}
	if !strings.Contains(got.why, "daemon not running") {
		t.Errorf("the fallback does not say why the default was measured: %q", got.why)
	}
	if got := dockerStoreHostPath("/home/t", "darwin", present(), noDaemon); got.path != "" || got.why == "" {
		t.Errorf("an unknown layout answered %+v; it must answer an empty path and a reason so the caller can say the guard did not run", got)
	}
}

// VALIDATES: on native Linux the guard measures the root directory the daemon
// names, and measures /var/lib/docker only when the daemon cannot answer or
// names a path this host does not have.
// PREVENTS: the 2026-10-10 refusal on an amd64 host whose daemon stored its
// data on a 295G disk with 118G free, refused because an empty
// /var/lib/docker sat on the 98G root disk with 20G free.
func TestDockerStoreHostPathAsksTheLinuxDaemon(t *testing.T) {
	const moved = "/home/thomas/.cache/docker"
	answers := func() (string, error) { return moved, nil }

	if got := dockerStoreHostPath("/home/thomas", "linux", present(dockerStateDir, moved), answers); got.path != moved || got.vmDisk {
		t.Errorf("the daemon named %s, but the guard measured %+v", moved, got)
	}
	got := dockerStoreHostPath("/home/thomas", "linux", present(dockerStateDir), answers)
	if got.path != dockerStateDir || !strings.Contains(got.why, moved) {
		t.Errorf("the daemon named a root this host lacks; got %+v, want the default and a reason naming %s", got, moved)
	}
}

// VALIDATES: the real probe, `docker info --format {{.DockerRootDir}}` through
// runCommandOutput, decides which filesystem checkKernelBuildSpace reads.
// PREVENTS: the same 2026-10-10 refusal, end to end through the guard.
func TestCheckKernelBuildSpaceMeasuresTheDaemonRoot(t *testing.T) {
	const moved = "/home/thomas/.cache/docker"
	oldGOOS, oldExists, oldFree, oldOutput := hostGOOS, pathExists, freeSpace, runCommandOutput
	t.Cleanup(func() { hostGOOS, pathExists, freeSpace, runCommandOutput = oldGOOS, oldExists, oldFree, oldOutput })
	hostGOOS = "linux"
	pathExists = present(dockerStateDir, moved)
	freeSpace = func(path string) (uint64, error) {
		if path == moved {
			return 118 * diskspace.BytesPerGiB, nil
		}
		return 20 * diskspace.BytesPerGiB, nil
	}
	var asked []string
	daemonUp := true
	runCommandOutput = func(_ context.Context, _ Request, name string, args ...string) ([]byte, error) {
		asked = append(asked, strings.Join(append([]string{name}, args...), " "))
		if !daemonUp {
			return nil, errors.New("cannot connect to the Docker daemon")
		}
		return []byte(moved + "\n"), nil
	}

	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatalf("temp: %v", err)
	}
	defer stdout.Close() //nolint:errcheck // test scratch file
	req := Request{Stdout: stdout, Stderr: stdout, Arch: "amd64", Target: "runtime"}

	store, err := checkKernelBuildSpace(context.Background(), req, pathExists)
	if err != nil {
		t.Fatalf("the daemon's root has 118G free, but the build was refused: %v", err)
	}
	if store.path != moved {
		t.Errorf("measured %q, want the daemon's root %q", store.path, moved)
	}
	if len(asked) != 1 || asked[0] != "docker info --format {{.DockerRootDir}}" {
		t.Errorf("the guard ran %q, want one docker info probe", asked)
	}

	daemonUp = false
	_, err = checkKernelBuildSpace(context.Background(), req, pathExists)
	if err == nil {
		t.Fatal("with no daemon answer the guard measured the 20G default and still passed")
	}
	for _, want := range []string{dockerStateDir, "cannot connect to the Docker daemon", "--builder qemu"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("refusal %q does not name %q", err, want)
		}
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

	oldGOOS, oldOutput := hostGOOS, runCommandOutput
	t.Cleanup(func() { hostGOOS, runCommandOutput = oldGOOS, oldOutput })
	hostGOOS = "linux"
	runCommandOutput = func(context.Context, Request, string, ...string) ([]byte, error) {
		return nil, errors.New("daemon not running")
	}

	req := Request{Stdout: stdout, Stderr: stdout}
	if _, err := checkKernelBuildSpace(context.Background(), req, func(string) bool { return false }); err != nil {
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
	// reclaimBuildSpace returns before runCommand when colima is absent, so a
	// host without it counted zero calls and read as the native-daemon case.
	// Pin the lookup, the way the sibling tests pin pathExists and freeSpace.
	oldAvailable := commandAvailable
	t.Cleanup(func() { commandAvailable = oldAvailable })
	commandAvailable = func(name string) bool { return name == "colima" }
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
	reclaimBuildSpace(context.Background(), req, "")
	if len(calls) != 0 {
		t.Errorf("a native daemon needs no discard, but ran %v", calls)
	}
}

// pinHostWithoutAContainerStore makes the space guard and the reclaim step
// no-ops, so a test that counts docker calls counts runDocker's own and not the
// dev machine's container layout.
func pinHostWithoutAContainerStore(t *testing.T) {
	t.Helper()
	restore := PinNoContainerStore()
	t.Cleanup(restore)
}

// VALIDATES: runDocker returns the freed space to the host as its last act, on
// a VM-backed runtime.
// PREVENTS: the reclaim existing but never being called, which is how the
// bound this class asked for in 2026-08-18 came to exist and never run
// (plan/journal/full-disk-false-red.md).
func TestRunDockerReturnsSpaceAfterTheBuild(t *testing.T) {
	hostIs(t, "amd64")
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
