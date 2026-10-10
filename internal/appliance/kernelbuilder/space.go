// Design: plan/journal/full-disk-false-red.md -- the class this guard exists to end
// Overview: driver.go -- runDocker, which calls both halves
// Related: internal/core/diskspace -- the one statfs answer

package kernelbuilder

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ze-software/ze/internal/core/diskspace"
)

// kernelBuildFreeBytesMin is the host free space a Docker kernel build needs
// before it may start.
//
// Measured on 2026-09-02, an arm64 runtime build that ran out: the
// ze-kernel-build volume held 13G when the build died PART WAY through
// drivers/, and /var/lib/docker held 41G in use with the builder image layers
// beside it. 40G is that measurement, rounded to a number an operator can check
// against their own df output.
//
// The figure is deliberately one number rather than a model of warm and cold
// caches. A refusal costs a message; passing into a full disk costs twenty
// minutes, the build, and the container runtime's image store.
const kernelBuildFreeBytesMin = 40 * diskspace.BytesPerGiB

// colimaVMDir and dockerDesktopDir are the two macOS layouts where the daemon
// runs inside a VM. dockerStateDir is the native Linux default, measured only
// when the daemon cannot name its own root.
const (
	colimaVMDir        = ".colima"
	dockerDesktopVMDir = "Library/Containers/com.docker.docker"
	dockerStateDir     = "/var/lib/docker"
)

// hostGOOS is runtime.GOOS, a variable so a test can take the native Linux
// branch on any machine.
var hostGOOS = runtime.GOOS

// containerStore is the HOST path the space guard measures, and why that path
// is the one receiving the build's writes. The refusal prints both, so an
// operator whose df disagrees can see which filesystem was read and why.
type containerStore struct {
	path string
	why  string
	// vmDisk is true when path is a VM's disk image directory, the one layout
	// whose freed space has to be handed back to the host (reclaimBuildSpace).
	vmDisk bool
}

// dockerStoreHostPath answers the HOST path whose filesystem receives a Docker
// build's writes. Its path is "" when this machine's layout is not one this
// guard knows, and why then says what was tried.
//
// The answer is NOT /var/lib/docker on a Mac. There the daemon runs inside a
// VM whose entire disk is one sparse file under the user's home, and that file
// is what runs out; /var/lib/docker names a path inside the guest, so statfs on
// it would answer about the wrong device. That is the same mistake `df` on the
// checkout makes about cache/, which is a symlink onto another filesystem
// (internal/le/scratch/cacheclean.go).
//
// Nor is it /var/lib/docker on every Linux host. The daemon's data-root is
// configurable, and a host that moved it off a small root disk keeps an empty
// /var/lib/docker behind: measured on 2026-10-10, an amd64 host whose daemon
// stored its data on a 295G disk with 118G free was refused because the 98G
// root disk had 20G. So on Linux the daemon is asked, and the default is
// measured only when it cannot answer, with the reason in why.
func dockerStoreHostPath(home, goos string, exists func(string) bool, daemonRoot func() (string, error)) containerStore {
	if home != "" {
		for _, dir := range []string{colimaVMDir, dockerDesktopVMDir} {
			candidate := filepath.Join(home, dir)
			if exists(candidate) {
				return containerStore{path: candidate, why: "the Docker daemon runs in a VM whose disk is stored there", vmDisk: true}
			}
		}
	}
	daemonNote := ""
	if goos == "linux" {
		root, err := daemonRoot()
		switch {
		case err != nil:
			daemonNote = ", and the daemon did not name its own (" + err.Error() + ")"
		case !exists(root):
			daemonNote = ", and the root the daemon named (" + root + ") does not exist on this host"
		default:
			return containerStore{path: root, why: "the Docker daemon names it as its root directory (docker info)"}
		}
	}
	if exists(dockerStateDir) {
		return containerStore{path: dockerStateDir, why: "it is the default Docker root directory" + daemonNote}
	}
	return containerStore{why: "no Docker VM disk under the home directory, and the default Docker root directory " + dockerStateDir + " does not exist" + daemonNote}
}

// dockerRootDir asks the daemon where it keeps its data, the directory
// `--data-root` or daemon.json set. It goes through runCommandOutput so a test
// fakes the daemon instead of reading the dev box's.
func dockerRootDir(ctx context.Context, req Request) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, hostProbeTimeout)
	defer cancel()
	out, err := runCommandOutput(ctx, req, backendDocker, "info", "--format", "{{.DockerRootDir}}")
	if ctx.Err() != nil {
		return "", fmt.Errorf("docker info did not answer within %s, so the daemon is hung or unreachable", hostProbeTimeout)
	}
	if err != nil {
		return "", fmt.Errorf("docker info: %w", err)
	}
	root := strings.TrimSpace(string(out))
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("docker info answered %q, which is not an absolute path", root)
	}
	return root, nil
}

// kernelBuildSpaceError refuses a build the filesystem cannot hold. It names
// both numbers, the path measured and why that path, then the command that
// checks the reading and the command that builds without Docker, each on its
// own line. It is pure, so the refusal is testable without a full disk.
func kernelBuildSpaceError(free uint64, store containerStore, req Request) error {
	if free >= kernelBuildFreeBytesMin {
		return nil
	}
	retry := []string{"ze", "appliance", "kernel", "--builder", "qemu"}
	for _, flag := range [][2]string{{"--target", req.Target}, {"--arch", req.Arch}, {"--profile", req.Profile}} {
		if flag[1] != "" {
			retry = append(retry, flag[0], flag[1])
		}
	}
	return fmt.Errorf(
		"kernel build needs %s free and %s has %s.\n"+
			"That path was measured because %s.\n"+
			"A Docker kernel build that runs out of space fails after many minutes and can damage the container image store.\n"+
			"Check the free space:\n  df -h %s\n"+
			"Free space on that filesystem, or build without Docker:\n  %s",
		diskspace.GiB(kernelBuildFreeBytesMin), store.path, diskspace.GiB(free),
		store.why, store.path, strings.Join(retry, " "))
}

// PinNoContainerStore makes this package answer "no known container store on
// this host" for the rest of the process, and returns the function that puts
// the real readings back.
//
// It exists for the compiled install fixtures
// (internal/test/fixture/install_fixture.go), which drive Build against a
// `docker` that is a symlink to a no-op on their own PATH. That build writes
// nothing and reads no image layer, so the 40G floor a REAL build needs decided
// their verdict from the free space of whichever machine ran them: green on a
// workstation whose container store had room, red on one that did not, with
// nothing about the product different between the two
// (plan/journal/gate-verdict-depends-on-the-machine.md, 2026-09-07).
//
// Every other caller keeps the guard, which is what an operator's build needs:
// the refusal costs a message and passing into a full disk costs twenty minutes
// and the container runtime's image store.
func PinNoContainerStore() (restore func()) {
	oldExists, oldFree, oldOutput := pathExists, freeSpace, runCommandOutput
	pathExists = func(string) bool { return false }
	freeSpace = func(string) (uint64, error) { return 0, nil }
	runCommandOutput = func(context.Context, Request, string, ...string) ([]byte, error) {
		return nil, errNoContainerStore
	}
	return func() { pathExists, freeSpace, runCommandOutput = oldExists, oldFree, oldOutput }
}

// checkKernelBuildSpace refuses a Docker kernel build that cannot finish.
//
// When the layout is one this guard does not know, it SAYS the check did not
// run and lets the build proceed. Silence would be the worse answer: a caller
// cannot tell "measured, and there is room" from "never measured", and a zero
// that reads as a valid answer is the defect ai/rules/principles.md names first.
// It returns the store it measured, so the reclaim after the build acts on the
// same answer instead of asking the daemon twice.
func checkKernelBuildSpace(ctx context.Context, req Request, exists func(string) bool) (containerStore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	store := dockerStoreHostPath(home, hostGOOS, exists, func() (string, error) { return dockerRootDir(ctx, req) })
	if store.path == "" {
		fmt.Fprintf(req.Stdout, ">>> free space not checked: %s\n", store.why) //nolint:errcheck // progress output
		return store, nil
	}
	free, err := freeSpace(store.path)
	if err != nil {
		fmt.Fprintf(req.Stdout, ">>> free space not checked: %v\n", err) //nolint:errcheck // progress output
		return store, nil
	}
	return store, kernelBuildSpaceError(free, store, req)
}

// reclaimBuildSpace returns the space the build finished with to the HOST,
// pass or fail.
//
// A VM-backed daemon writes into a sparse disk image that grows and never
// shrinks on its own, so deleting files inside the guest returns nothing until
// the guest DISCARDS the blocks. Without this the store is a one-way ratchet:
// every kernel build moves the host closer to full and no build ever moves it
// back. A native daemon needs none of it, because deleting a file there frees
// the host filesystem directly.
//
// Best effort by design. It runs after the build has already produced its
// verdict, so a failure here MUST NOT change that verdict; it is reported and
// dropped. vmDisk is the VM disk directory, or "" when the daemon is native.
func reclaimBuildSpace(ctx context.Context, req Request, vmDisk string) {
	if vmDisk == "" {
		return
	}
	if !commandAvailable("colima") {
		return
	}
	if err := runCommand(ctx, req, "colima", "ssh", "--", "sudo", "fstrim", dockerStateDir); err != nil {
		fmt.Fprintf(req.Stderr, ">>> could not return freed space to the host: %v\n", err) //nolint:errcheck // progress output
	}
}

// pathExists and freeSpace are the two readings this guard takes of the machine
// it runs on. Both are vars so a test pins the verdict instead of inheriting the
// dev box's container layout and free space. A suite whose answer moves with the
// machine is its own recorded failure class
// (plan/journal/gate-verdict-depends-on-the-machine.md).
var pathExists = func(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

var freeSpace = diskspace.Free

// errNoContainerStore is the daemon's answer while PinNoContainerStore holds.
var errNoContainerStore = errors.New("container store pinned absent")
