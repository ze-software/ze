// Package kernelbuilder builds Ze runtime and installer kernels without an interpreter.
package kernelbuilder

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultBuilderDir      = "tools/kernel-builder"
	defaultCommonDir       = "tools/kernel-builder/common"
	defaultImage           = "ze-kernel-builder"
	provenanceName         = "kernel.version"
	ownershipRepairTimeout = 2 * time.Minute

	archAMD64   = "amd64"
	archX8664   = "x86_64"
	archARM64   = "arm64"
	archAArch64 = "aarch64"

	backendDocker = "docker"
	backendQEMU   = "qemu"
	modulesYes    = "yes"
	modulesNo     = "no"

	dockerPlatformFlag = "--platform"
)

var dockerPlatforms = map[string]string{
	archAMD64: "linux/amd64",
	archX8664: "linux/amd64",
	archARM64: "linux/arm64",
}

// Request is one complete kernel build. Fragments MUST be in merge order.
type Request struct {
	Root        string
	Version     string
	Arch        string
	Profile     string
	Builder     string
	Target      string
	SourceDir   string
	OutputDir   string
	BuilderDir  string
	CommonDir   string
	Modules     string
	Jobs        string
	PatchesDir  string
	FirmwareDir string
	Fragments   []string
	Image       string
	Stdout      *os.File
	Stderr      *os.File
}

// Build selects a native backend, runs it, then records the exact build provenance.
func Build(ctx context.Context, req Request) error {
	if err := validateRequest(&req); err != nil {
		return err
	}
	backend, err := selectBuilder(req.Builder, req.Arch)
	if err != nil {
		return err
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
		return err
	}
	if err := writeProvenance(req.OutputDir, req, backend); err != nil {
		return err
	}
	fmt.Fprintf(req.Stdout, ">>> %s kernel ready (version=%s, target=%s, profile=%s)\n", req.Target, req.Version, req.Target, req.Profile) //nolint:errcheck // progress output
	return nil
}

func validateRequest(req *Request) error {
	if req.Root == "" {
		req.Root = "."
	}
	root, err := filepath.Abs(req.Root)
	if err != nil {
		return fmt.Errorf("resolve repository root: %w", err)
	}
	req.Root = root
	if req.BuilderDir == "" {
		req.BuilderDir = defaultBuilderDir
	}
	if req.CommonDir == "" {
		req.CommonDir = defaultCommonDir
	}
	if req.Image == "" {
		req.Image = defaultImage
	}
	if req.Target == "" {
		if req.Modules == modulesYes {
			req.Target = "runtime"
		} else {
			req.Target = "installer"
		}
	}
	if req.Stdout == nil {
		req.Stdout = os.Stdout
	}
	if req.Stderr == nil {
		req.Stderr = os.Stderr
	}
	if req.Arch == archAArch64 {
		req.Arch = archARM64
	}
	if err := validateVersion(req.Version); err != nil {
		return err
	}
	if _, ok := dockerPlatforms[req.Arch]; !ok {
		return fmt.Errorf("unsupported ARCH=%s (expected amd64, x86_64, or arm64)", req.Arch)
	}
	if err := validateProfile(req.Profile); err != nil {
		return err
	}
	if req.Modules != modulesYes && req.Modules != modulesNo {
		return fmt.Errorf("unsupported MODULES=%s (expected yes or no)", req.Modules)
	}
	if err := validateJobs(req.Jobs); err != nil {
		return err
	}
	for _, rel := range []string{req.SourceDir, req.BuilderDir, req.CommonDir} {
		if err := validateRepoPath(rel); err != nil {
			return err
		}
	}
	if err := validateOutputPath(&req.OutputDir); err != nil {
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

func validateOutputPath(value *string) error {
	if *value == "" {
		return errors.New("output path is empty")
	}
	clean := filepath.Clean(*value)
	volumeRoot := filepath.VolumeName(clean) + string(filepath.Separator)
	if clean == "." || clean == volumeRoot {
		return fmt.Errorf("unsafe output path: %s", *value)
	}
	if !filepath.IsAbs(clean) {
		if err := validateRepoPath(clean); err != nil {
			return err
		}
	}
	*value = clean
	return nil
}

func hostOutputPath(req Request) string {
	if filepath.IsAbs(req.OutputDir) {
		return req.OutputDir
	}
	return filepath.Join(req.Root, req.OutputDir)
}

func selectBuilder(requested, arch string) (string, error) {
	hasDocker := commandAvailable(backendDocker)
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
		return requested, nil
	case "":
		if hasDocker {
			return backendDocker, nil
		}
		if hasQEMU {
			return backendQEMU, nil
		}
		return "", errors.New("no builder available; install docker or qemu and go")
	default:
		return "", fmt.Errorf("unsupported builder %s (expected docker or qemu)", requested)
	}
}

var commandAvailable = func(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func runDocker(ctx context.Context, req Request) error {
	platform := dockerPlatforms[req.Arch]
	if err := os.MkdirAll(hostOutputPath(req), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	buildArgs := []string{"build", dockerPlatformFlag, platform, "-t", req.Image, "-f", filepath.Join(req.BuilderDir, "Dockerfile"), req.Root}
	if err := runCommand(ctx, req, backendDocker, buildArgs...); err != nil {
		return fmt.Errorf("build kernel builder image: %w", err)
	}
	args := []string{
		"run", "--rm", dockerPlatformFlag, platform,
		"-e", "KERNEL_VERSION=" + req.Version,
		"-v", filepath.Join(req.Root, req.SourceDir) + ":/src:ro",
		"-v", filepath.Join(req.Root, req.CommonDir) + ":/builder/common:ro",
		"-v", hostOutputPath(req) + ":/out",
		"-v", "ze-kernel-build:/tmp/kbuild",
		"-v", "ze-kernel-work:/build",
	}
	if req.FirmwareDir != "" {
		args = append(args, "-v", req.FirmwareDir+":/firmware:ro")
	}
	args = append(args, req.Image, "ze-kernel-builder", "--version", req.Version, "--arch", req.Arch, "--profile", req.Profile, "--modules", req.Modules, "--out-dir", "/out")
	if req.Jobs != "" {
		args = append(args, "--jobs", req.Jobs)
	}
	if req.PatchesDir != "" {
		rel, err := filepath.Rel(req.SourceDir, req.PatchesDir)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("patches directory %s must be under source directory %s", req.PatchesDir, req.SourceDir)
		}
		args = append(args, "--patches-dir", filepath.ToSlash(filepath.Join("/src", rel)))
	}
	if req.FirmwareDir != "" {
		args = append(args, "--firmware-dir", "/firmware")
	}
	for _, fragment := range req.Fragments {
		mapped, err := containerFragmentPath(req, fragment)
		if err != nil {
			return err
		}
		args = append(args, "--fragment", mapped)
	}
	buildErr := runCommand(ctx, req, backendDocker, args...)
	repairCtx, cancelRepair := context.WithTimeout(context.WithoutCancel(ctx), ownershipRepairTimeout)
	defer cancelRepair()
	repairErr := repairOwnership(repairCtx, req, platform)
	if buildErr != nil {
		if repairErr != nil {
			return fmt.Errorf("kernel container failed: %w (ownership repair also failed: %w)", buildErr, repairErr)
		}
		return fmt.Errorf("kernel container failed: %w", buildErr)
	}
	if repairErr != nil {
		return repairErr
	}
	return nil
}

func containerFragmentPath(req Request, fragment string) (string, error) {
	fragment = filepath.Clean(fragment)
	if rel, err := filepath.Rel(req.CommonDir, fragment); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(filepath.Join("/builder/common", rel)), nil
	}
	if rel, err := filepath.Rel(req.SourceDir, fragment); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(filepath.Join("/src", rel)), nil
	}
	return "", fmt.Errorf("fragment %s is outside source directory %s and common directory %s", fragment, req.SourceDir, req.CommonDir)
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

func writeProvenance(outputDir string, req Request, backend string) error {
	path := hostOutputPath(Request{Root: req.Root, OutputDir: outputDir})
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create provenance directory: %w", err)
	}
	data := fmt.Sprintf("version=%s\ntarget=%s\nprofile=%s\narch=%s\nmodules=%s\nbuilder=%s\n", req.Version, req.Target, req.Profile, req.Arch, req.Modules, backend)
	if err := os.WriteFile(filepath.Join(path, provenanceName), []byte(data), 0o644); err != nil {
		return fmt.Errorf("write kernel provenance: %w", err)
	}
	return nil
}

func workerGOARCH(arch string) string {
	if arch == archARM64 {
		return archARM64
	}
	return archAMD64
}
