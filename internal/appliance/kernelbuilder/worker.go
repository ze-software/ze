package kernelbuilder

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	defaultWorkDir      = "/build"
	defaultBuildDir     = "/tmp/kbuild"
	hardwareKMSProfile  = "hardware-kms"
	i915Firmware        = "i915/adlp_dmc.bin"
	kernelSourceBaseURL = "https://cdn.kernel.org/pub/linux/kernel"
)

// kernelSourceSHA256 is the SHA-256 of each kernel.org tarball Ze builds, keyed
// by version, copied from https://cdn.kernel.org/pub/linux/kernel/v7.x/sha256sums.asc
// when the version was pinned. internal/appliance/kernel.version names the
// version Ze ships, and TestKernelVersionHasDigestPin (internal/appliance) fails
// when that version has no entry here. A version with no entry is not built:
// the tarball is the kernel source Ze compiles and distributes, so it is the one
// that was reviewed, never whatever the network served.
var kernelSourceSHA256 = map[string]string{
	"7.2.9": "b4c5dfbe51a364a6c7f03869200f88c8e1f77403539005f14b7fc6bc91b8d8ba",
}

// SourceDigest returns the tracked SHA-256 of linux-<version>.tar.xz, and false
// when the version has no tracked digest.
func SourceDigest(version string) (string, bool) {
	digest, ok := kernelSourceSHA256[version]
	return digest, ok
}

// WorkerRequest describes the in-container or in-VM kernel compilation step.
type WorkerRequest struct {
	Version string
	// SourceSHA256 is the digest the source tarball MUST have. Empty means the
	// tracked digest for Version (kernelSourceSHA256).
	SourceSHA256 string
	Arch         string
	Profile      string
	Modules      string
	Jobs         string
	SourceDir    string
	OutputDir    string
	WorkDir      string
	BuildDir     string
	PatchesDir   string
	FirmwareDir  string
	Fragments    []string
	Stdout       io.Writer
	Stderr       io.Writer
}

// RunWorker downloads, configures, compiles, and publishes one Linux kernel.
func RunWorker(ctx context.Context, req WorkerRequest) error {
	if req.Stdout == nil {
		req.Stdout = os.Stdout
	}
	if req.Stderr == nil {
		req.Stderr = os.Stderr
	}
	if req.WorkDir == "" {
		req.WorkDir = envOr("WORK_DIR", defaultWorkDir)
	}
	if req.BuildDir == "" {
		req.BuildDir = envOr("BUILD_DIR", defaultBuildDir)
	}
	if req.SourceDir == "" {
		req.SourceDir = "/src"
	}
	if req.OutputDir == "" {
		req.OutputDir = "/out"
	}
	if err := validateVersion(req.Version); err != nil {
		return err
	}
	if req.SourceSHA256 == "" {
		digest, tracked := kernelSourceSHA256[req.Version]
		if !tracked {
			return fmt.Errorf("linux %s has no tracked SHA-256 (kernelSourceSHA256, internal/appliance/kernelbuilder/worker.go); add the digest kernel.org publishes in sha256sums.asc before building it", req.Version)
		}
		req.SourceSHA256 = digest
	}
	kernelArch, imagePath, makeTarget, err := workerArch(req.Arch)
	if err != nil {
		return err
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
	if req.Jobs == "" {
		req.Jobs = strconv.Itoa(runtime.NumCPU())
	}
	if len(req.Fragments) == 0 {
		return errors.New("no resolved config fragments")
	}
	for _, fragment := range req.Fragments {
		if err := requireRegularFile(fragment, "config fragment"); err != nil {
			return err
		}
	}
	if req.PatchesDir != "" {
		if err := requireRegularFile(filepath.Join(req.PatchesDir, "series"), "patch series"); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(req.WorkDir, 0o750); err != nil {
		return fmt.Errorf("create work directory: %w", err)
	}
	tarball, err := downloadKernelSource(ctx, req)
	if err != nil {
		return err
	}
	buildTree, err := restoreOrExtractTree(ctx, req, tarball)
	if err != nil {
		return err
	}
	if err := applyPatches(ctx, req, buildTree); err != nil {
		return err
	}
	fmt.Fprintf(req.Stdout, ">>> configuring (defconfig + resolved %s profile) for %s\n", req.Profile, req.Arch) //nolint:errcheck // progress output
	if err := runWorkerCommand(ctx, req, buildTree, "make", "ARCH="+kernelArch, "defconfig"); err != nil {
		return err
	}
	merge := filepath.Join(buildTree, "scripts", "kconfig", "merge_config.sh")
	args := make([]string, 0, 2+len(req.Fragments))
	args = append(args, "-m", ".config")
	args = append(args, req.Fragments...)
	if err := runWorkerCommand(ctx, req, buildTree, merge, args...); err != nil {
		return fmt.Errorf("merge kernel config: %w", err)
	}
	if err := embedFirmware(req, buildTree); err != nil {
		return err
	}
	if err := runWorkerCommand(ctx, req, buildTree, "make", "ARCH="+kernelArch, "olddefconfig"); err != nil {
		return err
	}
	if err := enforceRequiredSymbols(req, buildTree); err != nil {
		return err
	}
	fmt.Fprintf(req.Stdout, ">>> building %s with -j%s (modules=%s)\n", makeTarget, req.Jobs, req.Modules) //nolint:errcheck // progress output
	makeArgs := []string{"ARCH=" + kernelArch, "-j" + req.Jobs, makeTarget}
	if req.Modules == modulesYes {
		makeArgs = append(makeArgs, "modules")
	}
	if err := runWorkerCommand(ctx, req, buildTree, "make", makeArgs...); err != nil {
		return err
	}
	if err := os.MkdirAll(req.OutputDir, 0o750); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := copyFile(filepath.Join(buildTree, ".config"), filepath.Join(req.OutputDir, "config")); err != nil {
		return err
	}
	if req.Modules == modulesYes {
		return copyRuntimeOutputs(ctx, req, buildTree, kernelArch, imagePath)
	}
	return copyInstallerOutputs(ctx, req, buildTree, imagePath)
}

// validateVersion enforces the single kernel version grammar and major floor.
func validateVersion(value string) error {
	if value == "" || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") {
		return fmt.Errorf("unsupported KERNEL_VERSION=%s (expected digits and dots)", value)
	}
	for _, ch := range value {
		if (ch < '0' || ch > '9') && ch != '.' {
			return fmt.Errorf("unsupported KERNEL_VERSION=%s (expected digits and dots)", value)
		}
	}
	major, _, _ := strings.Cut(value, ".")
	n, err := strconv.Atoi(major)
	if err != nil || n < 7 {
		return errors.New("kernel >= 7.0 required (L2TP_NETLINK removed, serial 8250 deps changed)")
	}
	return nil
}

// validateProfile validates a profile/include token before using it in a path.
func validateProfile(value string) error {
	if value == "" || ((value[0] < 'a' || value[0] > 'z') && (value[0] < '0' || value[0] > '9')) {
		return fmt.Errorf("unsupported PROFILE=%s (expected ^[a-z0-9][a-z0-9-]*$)", value)
	}
	for _, ch := range value {
		if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '-' {
			return fmt.Errorf("unsupported PROFILE=%s (expected ^[a-z0-9][a-z0-9-]*$)", value)
		}
	}
	return nil
}

// validateJobs rejects values that could become make options.
func validateJobs(value string) error {
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return fmt.Errorf("unsupported JOBS=%s (expected digits)", value)
		}
	}
	return nil
}

func workerArch(arch string) (string, string, string, error) {
	switch arch {
	case archARM64:
		return archARM64, "arch/arm64/boot/Image", "Image", nil
	case archAMD64, archX8664:
		return archX8664, "arch/x86/boot/bzImage", "bzImage", nil
	default:
		return "", "", "", fmt.Errorf("unsupported ARCH=%s (expected arm64, amd64, or x86_64)", arch)
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func kernelTarballName(version string) string { return "linux-" + version + ".tar.xz" }
func kernelTarballURL(version string) string {
	major, _, _ := strings.Cut(version, ".")
	return kernelSourceBaseURL + "/v" + major + ".x/" + kernelTarballName(version)
}

func downloadKernelSource(ctx context.Context, req WorkerRequest) (string, error) {
	name := kernelTarballName(req.Version)
	path := filepath.Join(req.WorkDir, name)
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
		fmt.Fprintf(req.Stdout, ">>> using pre-downloaded %s\n", name) //nolint:errcheck // progress output
		if err := verifyKernelSource(path, req.SourceSHA256); err != nil {
			return "", err
		}
		return path, nil
	}
	fmt.Fprintf(req.Stdout, ">>> downloading linux %s\n", req.Version) //nolint:errcheck // progress output
	part := path + ".part"
	_ = os.Remove(part)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, kernelTarballURL(req.Version), http.NoBody)
	if err != nil {
		return "", err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("download kernel source: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return "", fmt.Errorf("download kernel source: HTTP %s", response.Status)
	}
	out, err := os.OpenFile(part, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //nolint:gosec // G304: part is the download scratch name this function built from WorkDir
	if err != nil {
		return "", fmt.Errorf("create kernel source part: %w", err)
	}
	_, copyErr := io.Copy(out, response.Body)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(part)
		return "", errors.Join(copyErr, closeErr)
	}
	if err := verifyKernelSource(part, req.SourceSHA256); err != nil {
		_ = os.Remove(part)
		return "", err
	}
	if err := os.Rename(part, path); err != nil {
		_ = os.Remove(part)
		return "", fmt.Errorf("publish kernel source: %w", err)
	}
	return path, nil
}

// verifyKernelSource refuses a tarball whose SHA-256 is not want, naming both
// digests, before anything is extracted from it.
func verifyKernelSource(path, want string) error {
	file, err := os.Open(path) //nolint:gosec // G304: path is the tarball name downloadKernelSource built from WorkDir
	if err != nil {
		return fmt.Errorf("open kernel source: %w", err)
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil {
		return fmt.Errorf("hash kernel source: %w", errors.Join(copyErr, closeErr))
	}
	got := hex.EncodeToString(hash.Sum(nil))
	if got != want {
		return fmt.Errorf("kernel source %s has SHA-256 %s, want %s: refusing to build from it", filepath.Base(path), got, want)
	}
	return nil
}

func restoreOrExtractTree(ctx context.Context, req WorkerRequest, tarball string) (string, error) {
	if err := os.MkdirAll(req.BuildDir, 0o750); err != nil {
		return "", fmt.Errorf("create build directory: %w", err)
	}
	buildTree := filepath.Join(req.BuildDir, fmt.Sprintf("linux-%s-%s", req.Version, req.Modules))
	cacheTar := filepath.Join(req.WorkDir, fmt.Sprintf("linux-%s-%s.built.tar", req.Version, req.Modules))
	if req.PatchesDir != "" {
		if err := os.RemoveAll(buildTree); err != nil {
			return "", err
		}
	}
	if regularFile(filepath.Join(buildTree, "scripts", "Kbuild.include")) {
		fmt.Fprintf(req.Stdout, ">>> reusing existing source tree %s\n", buildTree) //nolint:errcheck // progress output
		return buildTree, nil
	}
	if req.PatchesDir == "" && regularFile(cacheTar) {
		fmt.Fprintf(req.Stdout, ">>> restoring cached build tree from %s\n", filepath.Base(cacheTar)) //nolint:errcheck // progress output
		if err := extractTar(ctx, req, cacheTar, req.BuildDir, false); err != nil {
			return "", err
		}
		if regularFile(filepath.Join(buildTree, "scripts", "Kbuild.include")) {
			return buildTree, nil
		}
	}
	if err := os.RemoveAll(buildTree); err != nil {
		return "", err
	}
	plainTree := filepath.Join(req.BuildDir, "linux-"+req.Version)
	if err := os.RemoveAll(plainTree); err != nil {
		return "", err
	}
	fmt.Fprintf(req.Stdout, ">>> extracting to %s\n", buildTree) //nolint:errcheck // progress output
	if err := extractTar(ctx, req, tarball, req.BuildDir, true); err != nil {
		return "", err
	}
	if err := os.Rename(plainTree, buildTree); err != nil {
		return "", fmt.Errorf("rename extracted kernel tree: %w", err)
	}
	return buildTree, nil
}

func extractTar(ctx context.Context, req WorkerRequest, archive, dest string, xz bool) error {
	listArgs := []string{"-tf", archive}
	extractArgs := []string{"-xf", archive, "-C", dest}
	if xz {
		listArgs[0] = "-tJf"
		extractArgs[0] = "-xJf"
	}
	cmd := exec.CommandContext(ctx, "tar", listArgs...) //nolint:gosec // archive is a controlled cache path
	listing, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("list kernel archive: %w", err)
	}
	for name := range strings.SplitSeq(string(listing), "\n") {
		clean := filepath.Clean(name)
		if filepath.IsAbs(name) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("kernel tarball contains unsafe path: %s", name)
		}
	}
	if err := runWorkerCommand(ctx, req, "", "tar", extractArgs...); err != nil {
		return fmt.Errorf("extract kernel archive: %w", err)
	}
	return nil
}

func applyPatches(ctx context.Context, req WorkerRequest, buildTree string) error {
	if req.PatchesDir == "" {
		return nil
	}
	series := filepath.Join(req.PatchesDir, "series")
	data, err := os.ReadFile(series) //nolint:gosec // G304: PatchesDir is a request field the caller validated, and "series" is fixed
	if err != nil {
		return fmt.Errorf("read patch series: %w", err)
	}
	fmt.Fprintf(req.Stdout, ">>> applying patches from %s\n", req.PatchesDir) //nolint:errcheck // progress output
	for raw := range strings.SplitSeq(string(data), "\n") {
		name := strings.TrimSpace(raw)
		if name == "" || strings.HasPrefix(name, "#") {
			continue
		}
		if strings.Contains(name, "/") || strings.Contains(name, "..") {
			return fmt.Errorf("invalid patch name in series: %s", name)
		}
		path := filepath.Join(req.PatchesDir, name)
		file, err := os.Open(path) //nolint:gosec // G304: the series entry is rejected above when it holds a separator or ".."
		if err != nil {
			return fmt.Errorf("open patch %s: %w", path, err)
		}
		// --fuzz=0: a hunk whose context no longer matches the pinned source is
		// refused rather than placed by guesswork, so a kernel.version bump that
		// the series does not apply to cleanly fails here, at the build.
		cmd := exec.CommandContext(ctx, "patch", "-p1", "--fuzz=0", "--batch") //nolint:gosec // fixed executable and argv
		cmd.Dir, cmd.Stdin, cmd.Stdout, cmd.Stderr = buildTree, file, req.Stdout, req.Stderr
		runErr := cmd.Run()
		closeErr := file.Close()
		if runErr != nil || closeErr != nil {
			return fmt.Errorf("apply patch %s: %w", name, errors.Join(runErr, closeErr))
		}
	}
	return nil
}

func embedFirmware(req WorkerRequest, buildTree string) error {
	if req.Profile != hardwareKMSProfile {
		return nil
	}
	if req.FirmwareDir == "" {
		return fmt.Errorf("profile %s requires --firmware-dir with %s", hardwareKMSProfile, i915Firmware)
	}
	blob := filepath.Join(req.FirmwareDir, filepath.FromSlash(i915Firmware))
	info, err := os.Stat(blob)
	if err != nil || !info.Mode().IsRegular() {
		if err == nil {
			err = errors.New("not a regular file")
		}
		return fmt.Errorf("missing firmware %s: %w", blob, err)
	}
	fmt.Fprintf(req.Stdout, ">>> embedding firmware from %s\n  %s: %d bytes\n", req.FirmwareDir, i915Firmware, info.Size()) //nolint:errcheck // progress output
	config, err := os.OpenFile(filepath.Join(buildTree, ".config"), os.O_APPEND|os.O_WRONLY, 0)                             //nolint:gosec // G304: the kernel config in the tree this worker extracted
	if err != nil {
		return fmt.Errorf("open kernel config for firmware: %w", err)
	}
	_, writeErr := fmt.Fprintf(config, "CONFIG_EXTRA_FIRMWARE=%q\nCONFIG_EXTRA_FIRMWARE_DIR=%q\n", i915Firmware, req.FirmwareDir)
	return errors.Join(writeErr, config.Close())
}

func enforceRequiredSymbols(req WorkerRequest, buildTree string) error {
	required := make([]string, 0)
	seen := make(map[string]bool)
	for _, fragment := range req.Fragments {
		manifest := strings.TrimSuffix(fragment, filepath.Ext(fragment)) + ".require"
		file, err := os.Open(manifest) //nolint:gosec // G304: the manifest sits beside a fragment path the request named
		if err != nil {
			return fmt.Errorf("missing require manifest %s: %w", manifest, err)
		}
		scanner := bufio.NewScanner(file)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if trimmed, found := strings.CutSuffix(line, "=y"); found {
				line = trimmed
			} else if strings.Contains(line, "=") {
				_ = file.Close()
				return fmt.Errorf("%s:%d require entries must be CONFIG_SYMBOL or CONFIG_SYMBOL=y", manifest, lineNo)
			}
			if !strings.HasPrefix(line, "CONFIG_") || strings.ContainsAny(line, "/\\") {
				_ = file.Close()
				return fmt.Errorf("%s:%d invalid required symbol %q", manifest, lineNo, line)
			}
			if !seen[line] {
				seen[line] = true
				required = append(required, line)
			}
		}
		err = errors.Join(scanner.Err(), file.Close())
		if err != nil {
			return fmt.Errorf("read require manifest %s: %w", manifest, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(buildTree, ".config")) //nolint:gosec // G304: the kernel config in the tree this worker extracted
	if err != nil {
		return fmt.Errorf("read resolved kernel config: %w", err)
	}
	enabled := make(map[string]bool)
	for line := range strings.SplitSeq(string(data), "\n") {
		if key, value, ok := strings.Cut(line, "="); ok && strings.HasPrefix(key, "CONFIG_") && value == "y" {
			enabled[key] = true
		}
	}
	for _, symbol := range required {
		if !enabled[symbol] {
			return fmt.Errorf("kernel profile %s: %s did not resolve to =y in %s", req.Profile, symbol, filepath.Join(buildTree, ".config"))
		}
	}
	return nil
}

func copyRuntimeOutputs(ctx context.Context, req WorkerRequest, buildTree, kernelArch, imagePath string) error {
	if err := copyFile(filepath.Join(buildTree, filepath.FromSlash(imagePath)), filepath.Join(req.OutputDir, "vmlinuz")); err != nil {
		return err
	}
	if err := runWorkerCommand(ctx, req, buildTree, "make", "ARCH="+kernelArch, "INSTALL_MOD_PATH="+req.OutputDir, "modules_install"); err != nil {
		return err
	}
	modules := filepath.Join(req.OutputDir, "lib", "modules")
	if info, err := os.Stat(modules); err == nil && info.IsDir() {
		err = filepath.WalkDir(modules, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() && (entry.Name() == "build" || entry.Name() == "source") {
				return os.Remove(path) //nolint:gosec // G122: walks the module tree this worker just wrote in its own output directory; no other writer races it
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("remove module build links: %w", err)
		}
	}
	dts := filepath.Join(buildTree, "arch", archARM64, "boot", "dts")
	if kernelArch == archARM64 {
		if info, err := os.Stat(dts); err == nil && info.IsDir() {
			err = filepath.WalkDir(dts, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".dtb") {
					return copyFile(path, filepath.Join(req.OutputDir, entry.Name()))
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("copy device trees: %w", err)
			}
		}
	}
	for _, source := range []string{filepath.Join(buildTree, "overlays"), filepath.Join(dts, "overlays")} {
		if info, err := os.Stat(source); err == nil && info.IsDir() {
			dest := filepath.Join(req.OutputDir, "overlays")
			_ = os.RemoveAll(dest)
			if err := copyTree(source, dest); err != nil {
				return err
			}
			break
		}
	}
	fmt.Fprintf(req.Stdout, ">>> done: %s\n", filepath.Join(req.OutputDir, "vmlinuz")) //nolint:errcheck // progress output
	return nil
}

func copyInstallerOutputs(ctx context.Context, req WorkerRequest, buildTree, imagePath string) error {
	dest := filepath.Join(req.OutputDir, "Image")
	if err := copyFile(filepath.Join(buildTree, filepath.FromSlash(imagePath)), dest); err != nil {
		return err
	}
	if req.PatchesDir == "" {
		cacheTar := filepath.Join(req.WorkDir, fmt.Sprintf("linux-%s-%s.built.tar", req.Version, req.Modules))
		fmt.Fprintf(req.Stdout, ">>> caching build tree to %s\n", cacheTar) //nolint:errcheck // progress output
		if err := runWorkerCommand(ctx, req, filepath.Dir(buildTree), "tar", "-cf", cacheTar+".part", filepath.Base(buildTree)); err != nil {
			return err
		}
		if err := os.Rename(cacheTar+".part", cacheTar); err != nil {
			return fmt.Errorf("publish cached build tree: %w", err)
		}
	}
	info, err := os.Stat(dest)
	if err != nil {
		return err
	}
	fmt.Fprintf(req.Stdout, ">>> done: %s (%d MiB, profile=%s)\n", dest, info.Size()/(1024*1024), req.Profile) //nolint:errcheck // progress output
	return nil
}

var runWorkerCommand = func(ctx context.Context, req WorkerRequest, dir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // executable and argv are controlled builder commands
	cmd.Dir, cmd.Stdout, cmd.Stderr = dir, req.Stdout, req.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func requireRegularFile(path, label string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("missing %s: %s: %w", label, path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("missing %s: %s is not a regular file", label, path)
	}
	return nil
}
func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
func copyFile(source, dest string) (err error) {
	in, err := os.Open(source) //nolint:gosec // G304: both ends are paths inside the build tree and the output directory
	if err != nil {
		return fmt.Errorf("open %s: %w", source, err)
	}
	defer func() {
		err = errors.Join(err, in.Close())
	}()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm()) //nolint:gosec // G304+G302: dest mirrors source inside the output directory, and the copy keeps the source mode
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	return errors.Join(copyErr, out.Close())
}
func copyTree(source, dest string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if entry.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			_ = os.Remove(target)
			return os.Symlink(link, target) //nolint:gosec // G122: recreating a symlink is the point of this branch, and both trees are the worker's own
		}
		return copyFile(path, target)
	})
}
