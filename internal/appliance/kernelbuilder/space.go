// Design: plan/journal/full-disk-false-red.md -- the class this guard exists to end
// Overview: driver.go -- runDocker, which calls both halves
// Related: internal/core/diskspace -- the one statfs answer

package kernelbuilder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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
// runs inside a VM. dockerStateDir is the native Linux one.
const (
	colimaVMDir        = ".colima"
	dockerDesktopVMDir = "Library/Containers/com.docker.docker"
	dockerStateDir     = "/var/lib/docker"
)

// dockerStoreHostPath answers the HOST path whose filesystem receives a Docker
// build's writes, or "" when this machine's layout is not one this guard knows.
//
// The answer is NOT /var/lib/docker on a Mac. There the daemon runs inside a
// VM whose entire disk is one sparse file under the user's home, and that file
// is what runs out; /var/lib/docker names a path inside the guest, so statfs on
// it would answer about the wrong device. That is the same mistake `df` on the
// checkout makes about cache/, which is a symlink onto another filesystem
// (internal/le/scratch/cacheclean.go).
func dockerStoreHostPath(home string, exists func(string) bool) string {
	if home != "" {
		for _, dir := range []string{colimaVMDir, dockerDesktopVMDir} {
			candidate := filepath.Join(home, dir)
			if exists(candidate) {
				return candidate
			}
		}
	}
	if exists(dockerStateDir) {
		return dockerStateDir
	}
	return ""
}

// kernelBuildSpaceError refuses a build the filesystem cannot hold, and names
// both numbers so the operator can act without measuring anything themselves.
// It is pure, so the refusal is testable without a full disk.
func kernelBuildSpaceError(free uint64, store string) error {
	if free >= kernelBuildFreeBytesMin {
		return nil
	}
	return fmt.Errorf(
		"kernel build needs %s free and %s has %s: free space, or build with --builder qemu",
		diskspace.GiB(kernelBuildFreeBytesMin), store, diskspace.GiB(free))
}

// checkKernelBuildSpace refuses a Docker kernel build that cannot finish.
//
// When the layout is one this guard does not know, it SAYS the check did not
// run and lets the build proceed. Silence would be the worse answer: a caller
// cannot tell "measured, and there is room" from "never measured", and a zero
// that reads as a valid answer is the defect ai/rules/principles.md names first.
func checkKernelBuildSpace(req Request, exists func(string) bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	store := dockerStoreHostPath(home, exists)
	if store == "" {
		fmt.Fprintf(req.Stdout, ">>> free space not checked: no known container store on this host\n") //nolint:errcheck // progress output
		return nil
	}
	free, err := freeSpace(store)
	if err != nil {
		fmt.Fprintf(req.Stdout, ">>> free space not checked: %v\n", err) //nolint:errcheck // progress output
		return nil
	}
	return kernelBuildSpaceError(free, store)
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
// dropped.
func reclaimBuildSpace(ctx context.Context, req Request, store string) {
	if store == "" || store == dockerStateDir {
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
