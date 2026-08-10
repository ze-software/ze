// Design: docs/architecture/appliance/build-artifacts.md -- installer kernel download/build
// Design: docs/architecture/appliance/kernel-profiles.md -- installer kernel profile registry
// Design: kernel-build-consolidation — single run.py driver, runtime verified path, --target

package appliance

import (
	"context"
	_ "embed" // for go:embed kernel.version
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// kernelVersionFile holds the single source of truth for the Linux kernel
// version Ze builds. The build-time reader is tools/kernel-builder/run.py (it
// self-locates this file); this //go:embed is the compile-time reader. Bump the
// kernel by editing internal/appliance/kernel.version only.
//
//go:embed kernel.version
var kernelVersionFile string

// defaultKernelVersion is the trimmed contents of kernel.version.
var defaultKernelVersion = strings.TrimSpace(kernelVersionFile)

const (
	kernelURLKey             = "ze.appliance.kernel.url"
	kernelBuilderDir         = "tools/kernel-builder"
	kernelInstallerConfigDir = "tools/installer-kernel"
	kernelInstallerOutputDir = "build/kernel"
	runtimeKernelConfigDir   = "gokrazy/kernel"
	runtimeKernelOutputDir   = "tmp/kernel/build"
	runtimeKernelPatchesDir  = "gokrazy/kernel/patches"
	runtimeKernelArtifact    = "vmlinuz"
	kernelBuildTimeout       = 120 * time.Minute
	builderDocker            = "docker"
	builderQEMU              = "qemu"
	firmwareCacheDir         = "firmware"
	firmwareBaseURL          = "https://git.kernel.org/pub/scm/linux/kernel/git/firmware/linux-firmware.git/plain"
	kernelTargetInstaller    = "installer"
	kernelTargetRuntime      = "runtime"
	defaultKernelTarget      = kernelTargetInstaller
	runtimeKernelProfile     = "runtime"
	runPyName                = "run.py"
)

var i915FirmwareBlobs = []string{
	"i915/adlp_dmc.bin",
}

var _ = env.MustRegister(env.EnvEntry{
	Key:         kernelURLKey,
	Type:        "string",
	Description: "Base URL for pre-built installer kernel downloads",
})

// kernelBuildFn invokes the shared driver. It is a package var so unit tests can
// substitute a fake build without docker or qemu.
var kernelBuildFn = defaultKernelBuild

func init() {
	cmdKernel = runKernel
}

// kernelTargetDesc captures the per-target build parameters. The installer emits
// a single Image; the runtime emits a tree (vmlinuz + lib/modules + DTBs).
type kernelTargetDesc struct {
	name           string
	configDir      string
	outputDir      string
	modules        string
	patchesDir     string
	artifact       string
	defaultProfile string
	floor          []string
	isTree         bool
	allowDownload  bool
}

// kernelTestOutputDirEnv lets a test relocate the build output dir so parallel
// `ze appliance kernel` runs don't race on the shared build/kernel
// (Go writes it via run.py and reads its config back for enforcement). Unset in
// production, where the hardcoded per-target dirs are used.
const kernelTestOutputDirEnv = "ZE_KERNEL_TEST_OUTPUT_DIR"

func kernelOutputDir(def string) string {
	if v := os.Getenv(kernelTestOutputDirEnv); v != "" {
		return v
	}
	return def
}

func kernelTargetFor(target string) (kernelTargetDesc, error) {
	switch target {
	case kernelTargetInstaller:
		return kernelTargetDesc{
			name:           kernelTargetInstaller,
			configDir:      kernelInstallerConfigDir,
			outputDir:      kernelOutputDir(kernelInstallerOutputDir),
			modules:        "no",
			patchesDir:     "",
			artifact:       kernelFileName,
			defaultProfile: defaultKernelProfile,
			floor:          universalKernelRequirements,
			isTree:         false,
			allowDownload:  true,
		}, nil
	case kernelTargetRuntime:
		return kernelTargetDesc{
			name:           kernelTargetRuntime,
			configDir:      runtimeKernelConfigDir,
			outputDir:      kernelOutputDir(runtimeKernelOutputDir),
			modules:        "yes",
			patchesDir:     runtimeKernelPatchesDir,
			artifact:       runtimeKernelArtifact,
			defaultProfile: runtimeKernelProfile,
			floor:          runtimeKernelRequirements,
			isTree:         true,
			allowDownload:  false,
		}, nil
	default:
		return kernelTargetDesc{}, fmt.Errorf("target %q must be %s or %s", target, kernelTargetInstaller, kernelTargetRuntime)
	}
}

// validateKernelVersionString mirrors tools/kernel-builder/build.py's
// validate_version: the embedded version is checked in the command path (not
// package init, which would panic every `ze` invocation) so a malformed
// kernel.version fails fast before any download or build.
func validateKernelVersionString(version string) error {
	if version == "" || strings.HasPrefix(version, ".") || strings.HasSuffix(version, ".") {
		return fmt.Errorf("kernel version %q must be a non-empty N.N.N string", version)
	}
	for _, ch := range version {
		if (ch < '0' || ch > '9') && ch != '.' {
			return fmt.Errorf("kernel version %q must contain only digits and dots", version)
		}
	}
	major, _, _ := strings.Cut(version, ".")
	if !majorAtLeast(major, 7) {
		return fmt.Errorf("kernel version %q: major must be >= 7 (L2TP_NETLINK removed, serial 8250 deps changed)", version)
	}
	return nil
}

func majorAtLeast(major string, min int) bool {
	if major == "" {
		return false
	}
	n := 0
	for _, ch := range major {
		if ch < '0' || ch > '9' {
			return false
		}
		n = n*10 + int(ch-'0')
	}
	return n >= min
}

func runKernel(args []string) int {
	fs := flag.NewFlagSet("appliance kernel", flag.ContinueOnError)
	archFlag := fs.String("arch", "", "Target architecture: amd64 or arm64 (default: from appliance config or host)")
	profileFlag := fs.String("profile", "", "Kernel profile token (default: from appliance config or target default)")
	builderFlag := fs.String("builder", "", "Build backend: docker or qemu (default: docker if available, else qemu)")
	targetFlag := fs.String("target", defaultKernelTarget, "Kernel target: installer (default) or runtime")
	versionFlag := fs.String("version", defaultKernelVersion, "Linux kernel version")
	printCacheDirFlag := fs.Bool("print-cache-dir", false, "Print the durable cache path for this kernel (key = target+arch+config) and exit, without downloading or building. Used by the make kernel path to route through the cache (Option C).")
	evictCacheFlag := fs.Bool("evict-cache", false, "Bound this kernel's cache namespace to the keep-N most recent entries and exit (the make path calls this after populating). Evicts, does not build.")

	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze appliance kernel",
			Summary: "Download or build a Ze kernel (installer or runtime target)",
			Usage:   []string{"ze appliance kernel [options] [<name>]"},
			Sections: []helpfmt.HelpSection{
				{Title: "Options", Entries: []helpfmt.HelpEntry{
					{Name: "--target <target>", Desc: "Kernel target: installer (default, monolithic PXE Image) or runtime (modules + vmlinuz tree for the gokrazy appliance)"},
					{Name: "--arch <arch>", Desc: "Target architecture: amd64 or arm64 (default: from appliance config or host)"},
					{Name: "--profile <profile>", Desc: "Kernel profile token (default: from appliance config or the target default: qemu for installer, runtime for runtime)"},
					{Name: "--builder <backend>", Desc: "Build backend: docker or qemu (default: docker if available, else qemu)"},
					{Name: "--version <ver>", Desc: func() string {
						var tb textbuf.Buffer
						return tb.Str("Linux kernel version (default: ").Str(defaultKernelVersion).Byte(')').String()
					}()},
				}},
			},
			Examples: []string{
				"ze appliance kernel prod",
				"ze appliance kernel --profile hardware prod",
				"ze appliance kernel --target runtime",
				"ze appliance kernel --builder docker --arch amd64",
			},
		}
		p.WriteErr()
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	target := *targetFlag
	td, err := kernelTargetFor(target)
	if err != nil {
		cliErrorf("%v", err)
		return exitError
	}

	arch := *archFlag
	profile := *profileFlag
	builder := *builderFlag
	if fs.NArg() > 0 {
		name := fs.Arg(0)
		dir := getBaseDir()
		cfg, err := LoadConfig(ConfigPath(dir, name))
		if err != nil {
			cliErrorf("load appliance %q config: %v", name, err)
			return exitError
		}
		if arch == "" {
			arch = cfg.Image.Arch
		}
		// The appliance KernelProfile is installer-oriented (qemu/hardware/...).
		// The runtime kernel is a single global profile (runtime), so only the
		// installer target inherits the per-appliance profile; a runtime build
		// with an appliance name must not pick up the installer "qemu" default.
		if target == kernelTargetInstaller && profile == "" && cfg.Image.KernelProfile != "" {
			profile = cfg.Image.KernelProfile
		}
	}
	if arch == "" {
		arch = runtime.GOARCH
	}
	if profile == "" {
		profile = td.defaultProfile
	}

	if arch != archAMD64 && arch != archARM64 {
		cliErrorf("arch %q must be amd64 or arm64", arch)
		return exitError
	}
	if err := validateKernelProfileName(profile); err != nil {
		cliErrorf("%v", err)
		return exitError
	}
	if builder != "" && builder != builderDocker && builder != builderQEMU {
		cliErrorf("builder %q must be docker or qemu", builder)
		return exitError
	}
	if err := validateKernelVersionString(*versionFlag); err != nil {
		cliErrorf("%v", err)
		return exitError
	}

	if *printCacheDirFlag {
		dir, err := kernelCachePathFor(*versionFlag, arch, profile, target)
		if err != nil {
			cliErrorf("%v", err)
			return exitError
		}
		fmt.Fprintln(os.Stdout, dir) //nolint:errcheck // CLI output
		return exitOK
	}

	if *evictCacheFlag {
		ns := kernelCacheDir
		if td.isTree {
			ns = runtimeKernelCacheDir
		}
		evictKeepN(filepath.Join(resolveCacheDir(), ns))
		return exitOK
	}

	path, err := resolveKernel(*versionFlag, arch, profile, builder, target)
	if err != nil {
		cliErrorf("%v", err)
		return exitError
	}

	fmt.Fprintf(os.Stdout, "kernel ready: %s (target=%s, profile=%s, version=%s)\n", path, target, profile, *versionFlag) //nolint:errcheck // CLI output
	return exitOK
}

func resolveKernel(version, arch, profile, builder, target string) (string, error) {
	td, err := kernelTargetFor(target)
	if err != nil {
		return "", err
	}
	resolved, err := resolveKernelProfile(td.configDir, profile)
	if err != nil {
		return "", err
	}
	variant := kernelCacheVariantFor(target, arch, resolved)
	if td.isTree {
		return resolveRuntimeKernel(version, arch, profile, builder, td, resolved, variant)
	}
	return resolveInstallerKernel(version, arch, profile, builder, td, resolved, variant)
}

// kernelCachePathFor returns the durable cache DIRECTORY for the given kernel, using the
// SAME key as resolveKernel (kernelCacheVariantFor), so the make kernel path (Option C)
// consumes exactly the entry `ze appliance kernel` would populate. No download, no build.
// For the runtime (tree) target it is the tree directory; for the installer (single Image)
// target it is the directory containing the cached Image.
func kernelCachePathFor(version, arch, profile, target string) (string, error) {
	td, err := kernelTargetFor(target)
	if err != nil {
		return "", err
	}
	resolved, err := resolveKernelProfile(td.configDir, profile)
	if err != nil {
		return "", err
	}
	variant := kernelCacheVariantFor(target, arch, resolved)
	if td.isTree {
		return kernelTreeCachePath(version, variant), nil
	}
	return filepath.Dir(kernelCachePath(version, variant)), nil
}

// installerKernelRequestFile is the build-system record tools/installer-kernel/Makefile
// writes beside the image it builds. It holds the arch-profile-builder that was
// asked for, and that Makefile skips the build while the record still matches.
const installerKernelRequestFile = ".request"

// invalidateInstallerKernelRequest deletes that record. Every path through
// resolveInstallerKernel replaces build/kernel/Image, and none of them runs the
// Makefile. A surviving record would therefore describe an image the Makefile
// did not produce. `make -C tools/installer-kernel PROFILE=qemu` would report
// nothing to do, over another profile's kernel.
//
// It returns the error, and the caller stops on it. A record this process
// cannot delete, whose image it is about to replace, is the state the deletion
// exists to prevent. To continue past it leaves the wrong kernel behind under a
// success exit code. An absent record is the wanted state, not a failure.
func invalidateInstallerKernelRequest(outputDir string) error {
	path := filepath.Join(outputDir, installerKernelRequestFile)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale build record %s: %w", path, err)
	}
	return nil
}

func resolveInstallerKernel(version, arch, profile, builder string, td kernelTargetDesc, resolved kernelProfileResolution, variant string) (string, error) {
	cached := kernelCachePath(version, variant)
	toolsDst := filepath.Join(td.outputDir, td.artifact)
	// Before any path below replaces the image, so no branch can skip it. The
	// cache hit is the common repeat path and it returns before any build.
	if err := invalidateInstallerKernelRequest(td.outputDir); err != nil {
		return "", err
	}

	if _, err := os.Stat(cached); err == nil {
		if cpErr := copyToToolsPath(cached, toolsDst); cpErr != nil {
			fmt.Fprintf(os.Stdout, "warning: copy to %s: %v\n", toolsDst, cpErr) //nolint:errcheck // CLI warning
		}
		return cached, nil
	}

	if td.allowDownload {
		if baseURL := env.Get(kernelURLKey); baseURL != "" {
			var tb textbuf.Buffer
			artifactURL := tb.Str(baseURL).Byte('/').Str(version).Byte('-').Str(arch).Byte('-').Str(profile).Byte('/').Str(td.artifact).String()
			checksumURL := tb.Reset().Str(artifactURL).Str(checksumSuffix).String()
			if err := downloadAndVerify(artifactURL, checksumURL, cached); err == nil {
				if cpErr := copyToToolsPath(cached, toolsDst); cpErr != nil {
					fmt.Fprintf(os.Stdout, "warning: copy to %s: %v\n", toolsDst, cpErr) //nolint:errcheck // CLI warning
				}
				return cached, nil
			} else {
				fmt.Fprintf(os.Stdout, "warning: download from %s failed: %v; falling back to local build\n", baseURL, err) //nolint:errcheck // CLI warning
			}
		}
	}

	if err := buildKernelArtifact(version, arch, profile, builder, td); err != nil {
		return "", err
	}

	builtKernel := filepath.Join(td.outputDir, td.artifact)
	if _, err := os.Stat(builtKernel); err != nil {
		return "", fmt.Errorf("kernel not produced at %s", builtKernel)
	}
	if err := enforceKernelRequirements(resolved, filepath.Join(td.outputDir, "config"), td.floor); err != nil {
		return "", err
	}
	if err := copyToToolsPath(builtKernel, cached); err != nil {
		return "", err
	}
	if err := copyToToolsPath(cached, toolsDst); err != nil {
		fmt.Fprintf(os.Stdout, "warning: copy to %s: %v\n", toolsDst, err) //nolint:errcheck // CLI warning
	}
	evictKeepN(filepath.Join(resolveCacheDir(), kernelCacheDir))
	return cached, nil
}

func resolveRuntimeKernel(version, arch, profile, builder string, td kernelTargetDesc, resolved kernelProfileResolution, variant string) (string, error) {
	cachedDir := kernelTreeCachePath(version, variant)
	cachedKernel := filepath.Join(cachedDir, td.artifact)

	if _, err := os.Stat(cachedKernel); err == nil {
		if cpErr := copyTree(cachedDir, td.outputDir); cpErr != nil {
			fmt.Fprintf(os.Stdout, "warning: copy tree to %s: %v\n", td.outputDir, cpErr) //nolint:errcheck // CLI warning
		}
		return cachedDir, nil
	}

	if err := buildKernelArtifact(version, arch, profile, builder, td); err != nil {
		return "", err
	}

	builtKernel := filepath.Join(td.outputDir, td.artifact)
	if _, err := os.Stat(builtKernel); err != nil {
		return "", fmt.Errorf("runtime kernel not produced at %s", builtKernel)
	}
	modulesDir := filepath.Join(td.outputDir, "lib", "modules")
	if _, err := os.Stat(modulesDir); err != nil {
		return "", fmt.Errorf("runtime kernel modules not produced at %s", modulesDir)
	}
	if err := enforceKernelRequirements(resolved, filepath.Join(td.outputDir, "config"), td.floor); err != nil {
		return "", err
	}
	if err := copyTree(td.outputDir, cachedDir); err != nil {
		return "", fmt.Errorf("cache runtime kernel tree: %w", err)
	}
	evictKeepN(filepath.Join(resolveCacheDir(), runtimeKernelCacheDir))
	return cachedDir, nil
}

// buildKernelArtifact resolves the firmware dir then invokes the shared driver.
func buildKernelArtifact(version, arch, profile, builder string, td kernelTargetDesc) error {
	fwDir, err := ensureFirmware(profile)
	if err != nil {
		return err
	}
	if fwDir != "" {
		absFW, absErr := filepath.Abs(fwDir)
		if absErr != nil {
			return fmt.Errorf("resolve firmware directory: %w", absErr)
		}
		fwDir = absFW
	}
	fmt.Fprintf(os.Stdout, "building %s kernel via %s...\n", td.name, runPyName) //nolint:errcheck // CLI output
	return kernelBuildFn(kernelBuildSpec{
		version:  version,
		arch:     arch,
		profile:  profile,
		builder:  builder,
		target:   td.name,
		srcDir:   td.configDir,
		outDir:   td.outputDir,
		modules:  td.modules,
		patches:  td.patchesDir,
		firmware: fwDir,
	})
}

// kernelBuildSpec is the request handed to the shared driver tools/kernel-builder/run.py.
type kernelBuildSpec struct {
	version  string
	arch     string
	profile  string
	builder  string
	target   string
	srcDir   string
	outDir   string
	modules  string
	patches  string
	firmware string
}

func defaultKernelBuild(spec kernelBuildSpec) error {
	if err := os.MkdirAll(spec.outDir, cacheDirPerm); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	scriptPath, err := filepath.Abs(filepath.Join(kernelBuilderDir, runPyName))
	if err != nil {
		return fmt.Errorf("resolve run.py path: %w", err)
	}

	args := []string{
		scriptPath,
		"--target", spec.target,
		"--arch", spec.arch,
		"--profile", spec.profile,
		"--src-dir", spec.srcDir,
		"--out-dir", spec.outDir,
		"--builder-dir", kernelBuilderDir,
		"--common-dir", kernelCommonDir,
		"--modules", spec.modules,
		"--version", spec.version,
	}
	if spec.builder != "" {
		args = append(args, "--builder", spec.builder)
	}
	if spec.patches != "" {
		args = append(args, "--patches-dir", spec.patches)
	}
	if spec.firmware != "" {
		args = append(args, "--firmware-dir", spec.firmware)
	}

	buildCtx, buildCancel := context.WithTimeout(context.Background(), kernelBuildTimeout)
	defer buildCancel()

	cmd := exec.CommandContext(buildCtx, "python3", args...) //nolint:gosec // controlled args, list form, no shell
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", runPyName, err)
	}
	return nil
}

func ensureFirmware(profile string) (string, error) {
	if profile != hardwareKMSProfile {
		return "", nil
	}
	fwDir := filepath.Join(resolveCacheDir(), firmwareCacheDir)
	for _, blob := range i915FirmwareBlobs {
		dest := filepath.Join(fwDir, blob)
		if _, err := os.Stat(dest); err == nil {
			continue
		}
		var tb textbuf.Buffer
		url := tb.Str(firmwareBaseURL).Byte('/').Str(blob).String()
		if err := downloadFirmwareBlob(url, dest); err != nil {
			return "", fmt.Errorf("download firmware %s: %w", blob, err)
		}
		fmt.Fprintf(os.Stdout, "  firmware: %s\n", blob) //nolint:errcheck // CLI output
	}
	return fwDir, nil
}

func downloadFirmwareBlob(rawURL, destPath string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), cacheDirPerm); err != nil {
		return fmt.Errorf("create firmware directory: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody) //nolint:gosec // URL from constant
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req) //nolint:bodyclose // closed below
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // HTTP response
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d from %s", resp.StatusCode, rawURL)
	}
	tmp, err := os.CreateTemp(filepath.Dir(destPath), ".fw-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() {
		os.Remove(tmpPath) //nolint:errcheck // cleanup
	}()
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close() //nolint:errcheck // cleanup
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, destPath)
}

func cliErrorf(format string, args ...any) {
	fmt.Fprintf(os.Stdout, "error: %s\n", fmt.Sprintf(format, args...)) //nolint:errcheck // CLI output
}
