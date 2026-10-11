// Package kernelbuilder builds Ze runtime and installer kernels without an interpreter.
package kernelbuilder

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

const (
	ownershipRepairTimeout = 2 * time.Minute
)

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
	args, err := containerRunArgs(req)
	if err != nil {
		return err
	}
	buildErr := runCommand(ctx, req, backendDocker, args...)
	repairCtx, cancelRepair := context.WithTimeout(context.WithoutCancel(ctx), ownershipRepairTimeout)
	defer cancelRepair()
	repairErr := repairOwnership(repairCtx, req, platform)
	// Pass or fail. A failed build leaves the largest tree behind, so the run
	// that most needs the space returned is the one that did not finish. The
	// context is the detached one, so a canceled build still discards.
	if store.vmDisk {
		reclaimBuildSpace(repairCtx, req, store.path)
	}
	if buildErr != nil {
		failed := containerFailure(ctx, req, buildErr)
		if repairErr != nil {
			return fmt.Errorf("%w (ownership repair also failed: %w)", failed, repairErr)
		}
		return failed
	}
	if repairErr != nil {
		return repairErr
	}
	return nil
}

// containerFailure names why `docker run` of the kernel container failed. A
// stop ends it with "signal: killed", which reads as a failed kernel compile,
// so the stop is named first. The stop kills the Docker CLI, not the
// container, so the container may still be compiling, whatever stopped the
// build: the line that finds it is printed on a Ctrl-C too
// (errBuildStoppedLeaving).
func containerFailure(ctx context.Context, req Request, buildErr error) error {
	if ctx.Err() != nil {
		return errBuildStoppedLeaving(ctx, "the kernel container was stopped while it built the kernel",
			"the build output above shows the last step it reached.",
			"Docker may still run the container, which holds the build volumes the next build mounts; "+
				"list it, then stop it with docker stop <id>:\n"+
				"docker ps --filter ancestor="+req.Image)
	}
	return fmt.Errorf("kernel container failed: %w", buildErr)
}

func repairOwnership(ctx context.Context, req Request, platform string) error {
	args := []string{"run", "--rm", dockerPlatformFlag, platform, "-v", hostOutputPath(req) + ":/out", req.Image, "chown", "-R", fmt.Sprintf("%d:%d", os.Getuid(), os.Getgid()), "/out"}
	if err := runCommand(ctx, req, backendDocker, args...); err != nil {
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
