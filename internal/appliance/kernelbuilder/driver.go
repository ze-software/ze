// Design: docs/architecture/appliance/kernel-profiles.md -- native backend and container ownership.

// Package kernelbuilder builds Ze runtime and installer kernels without an interpreter.
package kernelbuilder

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	ownershipRepairTimeout = 2 * time.Minute
)

// containerLifecycleTimeout bounds each detached `docker create` and
// `docker rm --force` of a container this run owns. Both are daemon work on
// the image's layers, not a question to the host, so hostProbeTimeout (15s) is
// the wrong bound: creating the ~717 MB ze-kernel-builder container took 25.2s
// and 16.2s on a loaded Linux host while `docker info` answered in 0.1s, and
// every build failed with a container Docker went on to create. Two minutes is
// almost five times the slowest creation measured, and it still reports a hung
// daemon long before the 2-hour build limit. A variable so a test can play a
// creation that never answers without waiting for it.
var containerLifecycleTimeout = 2 * time.Minute

// hostGOARCH is the arch of the machine running the build. Every kernel is
// built on a host of its own arch, so a request for another arch is refused
// (validateRequest). It is a variable so a test can play either host.
var hostGOARCH = runtime.GOARCH

// CheckArch refuses a kernel arch this host cannot build. Owner decision
// 2026-10-09: every kernel is built on a host of its own arch, never under
// Docker --platform emulation or QEMU tcg, which run for hours and fail late.
// validateRequest runs it before any backend starts, and a caller that fetches
// inputs first (the installer firmware) runs it before the fetch.
func CheckArch(arch string) error {
	if arch == archAArch64 {
		arch = archARM64
	}
	if !ArchSupported(arch) {
		return fmt.Errorf("unsupported ARCH=%s (expected amd64, x86_64, or arm64); name this host's arch:\nze appliance kernel --arch %s", arch, hostGOARCH)
	}
	if want := workerGOARCH(arch); want != hostGOARCH {
		return fmt.Errorf("kernel for %s cannot be built here: the host is %s, and Ze builds every kernel on a host of its own arch, with no CPU emulation; build it on an %s host, or build this host's kernel:\nze appliance kernel --arch %s", want, hostGOARCH, want, hostGOARCH)
	}
	return nil
}

// ArchSupported answers whether Ze builds a kernel for arch on some host: the
// arches the Docker backend names a platform for, aarch64 read as arm64.
func ArchSupported(arch string) bool {
	if arch == archAArch64 {
		arch = archARM64
	}
	_, ok := dockerPlatforms[arch]
	return ok
}

// SupportedArches answers the kernel arches Ze builds, by their Go names and
// sorted, each built on a host of its own arch: what a refusal of an
// unsupported arch offers instead.
func SupportedArches() []string {
	var arches []string
	for arch := range dockerPlatforms {
		if goarch := workerGOARCH(arch); !slices.Contains(arches, goarch) {
			arches = append(arches, goarch)
		}
	}
	slices.Sort(arches)
	return arches
}

// HostArch answers the kernel arch of the machine running Ze: the one arch a
// kernel build here can produce. internal/appliance asks it rather than read
// runtime.GOARCH, so the build and the cache lookup never disagree.
func HostArch() string { return hostGOARCH }

// Build selects a native backend, runs it, then records the exact build provenance.
func Build(ctx context.Context, req Request) error {
	if err := validateRequest(&req); err != nil {
		return err
	}
	backend, err := SelectBuilder(ctx, req.Builder, req.Arch)
	if err != nil {
		// The accelerator probe runs a host command (sysctl on a Mac), which
		// a stop ends with exec's error, so the stop is named here as well.
		return stopReport(ctx, err)
	}
	fmt.Fprintf(req.Stdout, ">>> building %s kernel: version=%s arch=%s profile=%s builder=%s\n", req.Target, req.Version, req.Arch, req.Profile, backend) //nolint:errcheck // progress output
	switch backend {
	case backendDocker:
		err = runDocker(ctx, req)
	case backendQEMU:
		err = runQEMU(ctx, req)
	default:
		return fmt.Errorf("BUG: selected unsupported kernel builder %q", backend)
	}
	if err != nil {
		// A step that returned under a stop without naming it is named here
		// (stopReport), so no stop reaches the operator as a library's error
		// alone: the library's error stays, as the evidence of the step.
		return stopReport(ctx, err)
	}
	if err := writeProvenance(req.OutputDir, req, backend); err != nil {
		return err
	}
	fmt.Fprintf(req.Stdout, ">>> %s kernel ready (version=%s, target=%s, profile=%s)\n", req.Target, req.Version, req.Target, req.Profile) //nolint:errcheck // progress output
	return nil
}

func validateRequest(req *Request) error {
	if err := normalizeRequest(req); err != nil {
		return err
	}
	if err := validateVersion(req.Version); err != nil {
		return err
	}
	if err := CheckArch(req.Arch); err != nil {
		return err
	}
	if err := validateProfile(req.Profile); err != nil {
		return err
	}
	if req.Modules == "" {
		return fmt.Errorf("unsupported kernel target %q (expected %s or %s)", req.Target, targetInstaller, targetRuntime)
	}
	if err := validateJobs(req.Jobs); err != nil {
		return err
	}
	for _, rel := range []string{req.SourceDir, req.BuilderDir, req.CommonDir} {
		if err := validateRepoPath(rel); err != nil {
			return err
		}
	}
	if err := validateOutputPath(req.OutputDir); err != nil {
		return err
	}
	if req.PatchesDir != "" {
		if err := validateRepoPath(req.PatchesDir); err != nil {
			return err
		}
	}
	if len(req.Fragments) == 0 {
		return errors.New("kernel build has no resolved config fragments")
	}
	for _, fragment := range req.Fragments {
		if err := validateRepoPath(fragment); err != nil {
			return fmt.Errorf("fragment %q: %w", fragment, err)
		}
		if info, err := os.Stat(filepath.Join(req.Root, fragment)); err != nil || !info.Mode().IsRegular() {
			if err == nil {
				err = errors.New("not a regular file")
			}
			return fmt.Errorf("config fragment %s: %w", fragment, err)
		}
	}
	if req.FirmwareDir != "" && !filepath.IsAbs(req.FirmwareDir) {
		return fmt.Errorf("firmware directory must be absolute: %s", req.FirmwareDir)
	}
	return nil
}

func validateRepoPath(value string) error {
	if value == "" || filepath.IsAbs(value) {
		return fmt.Errorf("path must be repository-relative: %s", value)
	}
	clean := filepath.Clean(value)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path escapes repository: %s", value)
	}
	for _, ch := range filepath.ToSlash(value) {
		if !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789/._-", ch) {
			return fmt.Errorf("path contains unsupported character %q: %s", ch, value)
		}
	}
	return nil
}

// validateOutputPath refuses an output path the build must not write to.
// normalizeRequest has already cleaned it.
func validateOutputPath(value string) error {
	if value == "" {
		return errors.New("output path is empty")
	}
	volumeRoot := filepath.VolumeName(value) + string(filepath.Separator)
	if value == "." || value == volumeRoot {
		return fmt.Errorf("unsafe output path: %s", value)
	}
	if !filepath.IsAbs(value) {
		if err := validateRepoPath(value); err != nil {
			return err
		}
	}
	return nil
}

// SelectBuilder answers the backend a kernel build for arch uses: requested
// when it is named, else Docker, else QEMU with Go. QEMU counts only when it can
// run under a hardware accelerator on this host (hardwareAccelerator), so a host
// whose QEMU would exit at once is refused here, before any download or build.
// `ze doctor` asks the same question before a build (internal/appliance).
func SelectBuilder(ctx context.Context, requested, arch string) (string, error) {
	hasDocker := commandAvailable(backendDocker)
	if requested == "" && PreferredBackend("") == backendDocker {
		return backendDocker, nil
	}
	qemu := qemuBinary(arch)
	hasQEMU := commandAvailable(qemu) && commandAvailable("go")
	switch requested {
	case backendDocker:
		if !hasDocker {
			return "", errors.New("docker builder requested but docker not found")
		}
		return requested, nil
	case backendQEMU:
		if !hasQEMU {
			return "", fmt.Errorf("qemu builder requested but %s or go not found", qemu)
		}
		if _, err := acceleratorCheck(ctx, qemu); err != nil {
			return "", err
		}
		return requested, nil
	case "":
		if !hasQEMU {
			return "", errors.New("no builder available; install docker or qemu and go")
		}
		if _, err := acceleratorCheck(ctx, qemu); err != nil {
			return "", fmt.Errorf("no builder available: docker is not installed, and %w", err)
		}
		return backendQEMU, nil
	default:
		return "", fmt.Errorf("unsupported builder %s (expected docker or qemu)", requested)
	}
}

// PreferredBackend answers the backend a build with requested names would run
// on, with no probe: the one named, else Docker when it is installed, else
// QEMU. SelectBuilder makes the same choice and then refuses a backend this host
// cannot run. The cached-kernel key carries the answer (internal/appliance),
// because the two backends compile with different toolchains: a kernel the one
// built is never served for the other.
func PreferredBackend(requested string) string {
	if requested != "" {
		return requested
	}
	if commandAvailable(backendDocker) {
		return backendDocker
	}
	return backendQEMU
}

// acceleratorCheck is hardwareAccelerator, a variable so a test can select a
// QEMU backend on a host with no QEMU.
var acceleratorCheck = hardwareAccelerator

var commandAvailable = func(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func runDocker(ctx context.Context, req Request) error {
	platform := dockerPlatforms[req.Arch]
	// Before the image build, which itself writes gigabytes. A kernel build
	// that starts on a nearly full host does not fail cleanly: it fills the
	// sparse disk backing the container runtime, and the error that surfaces
	// names a content blob rather than the disk (plan/journal/full-disk-false-red.md).
	store, err := checkKernelBuildSpace(ctx, req, pathExists)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(hostOutputPath(req), 0o750); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := runCommand(ctx, req, backendDocker, containerBuildArgs(req)...); err != nil {
		// runCommand ends in "signal: killed" whenever ctx ends, which reads
		// as a failed image build, so the stop is named first.
		if ctx.Err() != nil {
			return errBuildStopped(ctx, "Docker was stopped while it built the kernel builder image",
				"the docker output above shows the last step it reached; run the same command by hand to see where it stops:\n"+
					backendDocker+" "+shellJoin(containerBuildArgs(req)))
		}
		return fmt.Errorf("build kernel builder image: %w", err)
	}
	name := "ze-kernel-" + rand.Text()
	args, err := containerCreateArgs(req, name)
	if err != nil {
		return err
	}
	buildErr, teardownErr := runContainer(ctx, req, name, args)
	failed := containerFailure(ctx, buildErr)
	if teardownErr != nil {
		// A CLI exit does not prove the daemon worker stopped. Neither chown
		// nor discard may race a worker whose removal was not confirmed.
		return errors.Join(failed, teardownErr)
	}
	repairCtx, cancelRepair := context.WithTimeout(context.WithoutCancel(ctx), ownershipRepairTimeout)
	defer cancelRepair()
	repairErr := repairOwnership(repairCtx, req, platform)
	if repairErr != nil {
		return errors.Join(failed, repairErr)
	}
	// Pass or fail, but only after both owned containers have been removed.
	// The detached context lets a canceled build return freed space too.
	if store.vmDisk {
		reclaimBuildSpace(repairCtx, req, store.path)
	}
	return failed
}

// containerFailure names a stopped build before exec's CLI error. Successful
// teardown is established separately, never inferred from the CLI's exit.
func containerFailure(ctx context.Context, buildErr error) error {
	if buildErr == nil {
		return nil
	}
	if ctx.Err() != nil {
		return errBuildStopped(ctx, "the kernel container was stopped while it built the kernel",
			"the build output above shows the last step it reached.")
	}
	return fmt.Errorf("kernel container failed: %w", buildErr)
}

// runContainer MUST remove the exact container it creates before its caller
// repairs ownership or reclaims space. Callers MUST refuse those operations if
// teardownErr is non-nil. It is synchronous and has no concurrent users.
// Creation is detached and bounded by containerLifecycleTimeout: cancellation
// cannot lose the daemon's ID response between creating the container and
// recording what this run owns. A failed, overrun or malformed creation
// response is Unknown, not absence; no start or output repair follows it.
// Only a full daemon-issued ID authorizes removal.
func runContainer(ctx context.Context, req Request, name string, args []string) (runErr, teardownErr error) {
	if err := ctx.Err(); err != nil {
		return err, nil
	}
	createCtx, cancelCreate := context.WithTimeout(context.WithoutCancel(ctx), containerLifecycleTimeout)
	output, err := runCommandOutput(createCtx, req, backendDocker, args...)
	err = namedLifecycleOverrun(createCtx, err)
	cancelCreate()
	if err != nil {
		return err, fmt.Errorf("Docker did not confirm creating container %s (%w); Ze did not start it, and output ownership repair and disk reclaim were refused. Inspect only this run's container before removing it:\ndocker container inspect %s", name, err, name)
	}
	id := strings.TrimSpace(string(output))
	if len(id) != 64 {
		return errors.New("Docker did not return a full container ID"),
			fmt.Errorf("Docker returned %q for container %s; Ze did not start it, and output ownership repair and disk reclaim were refused. Inspect only this run's container before removing it:\ndocker container inspect %s", id, name, name)
	}
	for _, ch := range id {
		if !strings.ContainsRune("0123456789abcdef", ch) {
			return errors.New("Docker returned an invalid container ID"),
				fmt.Errorf("Docker returned %q for container %s; Ze did not start it, and output ownership repair and disk reclaim were refused. Inspect only this run's container before removing it:\ndocker container inspect %s", id, name, name)
		}
	}
	runErr = ctx.Err()
	if runErr == nil {
		runErr = runCommand(ctx, req, backendDocker, "start", "--attach", id)
	}
	removeCtx, cancelRemove := context.WithTimeout(context.WithoutCancel(ctx), containerLifecycleTimeout)
	defer cancelRemove()
	if err := namedLifecycleOverrun(removeCtx, runCommand(removeCtx, req, backendDocker, "rm", "--force", id)); err != nil {
		return runErr, fmt.Errorf("Docker did not confirm removing owned container %s (%w); it may still hold the build volumes, so output ownership repair and disk reclaim were refused. Remove only this container before another build:\ndocker rm --force %s", id, err, id)
	}
	// Docker's successful removal joins its worker and removes the identity,
	// including when start was canceled while the daemon was handling it.
	return runErr, nil
}

// namedLifecycleOverrun says when a container create or removal failed because
// containerLifecycleTimeout ran out: the CLI killed at the deadline reports
// only "signal: killed", which reads as a Docker failure, not a slow daemon.
// Any other error, and a nil one, is returned unchanged.
func namedLifecycleOverrun(callCtx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if !errors.Is(callCtx.Err(), context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("no answer within the %s limit: %w", LimitWords(containerLifecycleTimeout), err)
}

func repairOwnership(ctx context.Context, req Request, platform string) error {
	name := "ze-kernel-ownership-" + rand.Text()
	var owner textbuf.Buffer
	owner.Int(int64(os.Getuid())).Byte(':').Int(int64(os.Getgid()))
	args := []string{"create", "--name", name, dockerPlatformFlag, platform, "-v", hostOutputPath(req) + ":/out", req.Image, "chown", "-R", owner.String(), "/out"}
	runErr, teardownErr := runContainer(ctx, req, name, args)
	if err := errors.Join(runErr, teardownErr); err != nil {
		return fmt.Errorf("repair output ownership: %w", err)
	}
	return nil
}

var runCommand = func(ctx context.Context, req Request, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // argv is constructed from validated tokens
	cmd.Dir = req.Root
	cmd.Stdout = req.Stdout
	cmd.Stderr = req.Stderr
	return cmd.Run()
}

// runCommandOutput runs a command for its answer rather than its progress: it
// returns stdout, and folds stderr into the error so a refusal says why.
var runCommandOutput = func(ctx context.Context, req Request, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // argv is constructed from validated tokens
	cmd.Dir = req.Root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("%w: %s", err, msg)
		}
		return nil, err
	}
	return out, nil
}
