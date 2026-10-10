// Design: docs/architecture/appliance/build-artifacts.md -- installer kernel download/build
// Design: docs/architecture/appliance/kernel-profiles.md -- installer kernel profile registry
// Kernel builds run entirely in Go; Docker and QEMU are native backends.

package appliance

import (
	"context"
	_ "embed" // for go:embed kernel.version
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/appliance/kernelbuilder"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// kernelVersionFile holds the single source of truth for the Linux kernel
// version Ze builds. Bump the kernel by editing
// internal/appliance/kernel.version only.
//
//go:embed kernel.version
var kernelVersionFile string

// defaultKernelVersion is the trimmed contents of kernel.version.
var defaultKernelVersion = strings.TrimSpace(kernelVersionFile)

const (
	kernelURLKey             = "ze.appliance.kernel.url"
	kernelBuilderDir         = kernelbuilder.DefaultBuilderDir
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
)

var i915FirmwareBlobs = []string{
	"i915/adlp_dmc.bin",
}

var _ = env.MustRegister(env.EnvEntry{
	Key:         kernelURLKey,
	Type:        envTypeString,
	Description: "Base URL for pre-built installer kernel downloads",
})

// kernelBuildFn invokes the native builder. It is a package var so unit tests can
// substitute a fake build without Docker or QEMU.
var kernelBuildFn = defaultKernelBuild

var nativeKernelBuildFn = kernelbuilder.Build

func init() {
	cmdKernel = runKernel
}

// kernelTargetDesc captures the per-target build parameters. The installer emits
// a single Image; the runtime emits a tree (vmlinuz + lib/modules + DTBs).
type kernelTargetDesc struct {
	name           string
	configDir      string
	outputDir      string
	patchesDir     string
	artifact       string
	defaultProfile string
	floor          []string
	isTree         bool
	allowDownload  bool
}

// kernelTestOutputDirEnv lets a test relocate the build output dir so parallel
// `ze appliance kernel` runs don't race on the shared build output directory.
// Unset in production, where the hardcoded per-target dirs are used.
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

// validateKernelVersionString mirrors kernelbuilder.ValidateVersion. The
// embedded version is checked in the command path (not package init, which
// would panic every `ze` invocation) so a malformed version fails before work.
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
	printCacheDirFlag := fs.Bool("print-cache-dir", false, "Print the durable cache path for this kernel (key = target+arch+config) and exit, without downloading or building.")
	evictCacheFlag := fs.Bool("evict-cache", false, "Bound this kernel's cache namespace to the keep-N most recent entries of each architecture, keeping the current entry, and exit. Evicts, does not build.")

	fs.Usage = func() {
		p := helpfmt.Page{
			Command:   "ze appliance kernel",
			ShortHelp: "Download or build a Ze kernel (installer or runtime target)",
			Usage:     []string{"ze appliance kernel [options] [<name>]"},
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
		arch = kernelbuilder.HostArch()
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
		dir, err := kernelCachePathFor(*versionFlag, arch, profile, builder, target)
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
		current, err := kernelCachePathFor(*versionFlag, arch, profile, builder, target)
		if err != nil {
			cliErrorf("%v", err)
			return exitError
		}
		evictKeepN(filepath.Join(resolveCacheDir(), ns), current)
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
	variant, err := kernelCacheVariantFor(target, arch, builder, resolved)
	if err != nil {
		return "", err
	}
	if td.isTree {
		return resolveRuntimeKernel(version, arch, profile, builder, td, resolved, variant)
	}
	return resolveInstallerKernel(version, arch, profile, builder, td, resolved, variant)
}

// kernelCachePathFor returns the durable cache directory for the same identity
// resolveKernel uses. It observes the cache namespace without downloading or
// building. Runtime targets return the tree directory; installer targets return
// the directory containing Image. builder is the --builder value, empty for the
// backend a build here would choose (kernelbuilder.PreferredBackend).
func kernelCachePathFor(version, arch, profile, builder, target string) (string, error) {
	td, err := kernelTargetFor(target)
	if err != nil {
		return "", err
	}
	resolved, err := resolveKernelProfile(td.configDir, profile)
	if err != nil {
		return "", err
	}
	variant, err := kernelCacheVariantFor(target, arch, builder, resolved)
	if err != nil {
		return "", err
	}
	if td.isTree {
		return kernelTreeCachePath(version, variant), nil
	}
	return filepath.Dir(kernelCachePath(version, variant)), nil
}

// installerKernelVariantFile records the installer arch, profile, version, and
// backend. installerKernelBuildMatches in cmd_iso.go reads it before accepting
// build/kernel/Image as an ISO input.
const installerKernelVariantFile = ".variant"

func invalidateInstallerKernelVariant(outputDir string) error {
	path := filepath.Join(outputDir, installerKernelVariantFile)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale build record %s: %w", path, err)
	}
	return nil
}

func resolveInstallerKernel(version, arch, profile, builder string, td kernelTargetDesc, resolved kernelProfileResolution, variant string) (string, error) {
	cached := kernelCachePath(version, variant)
	toolsDst := filepath.Join(td.outputDir, td.artifact)
	// Before any path below replaces the image, so no branch can leave an old
	// arch/profile record beside new bytes.
	if err := invalidateInstallerKernelVariant(td.outputDir); err != nil {
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
			downloadErr := downloadAndVerify(artifactURL, checksumURL, cached)
			if downloadErr == nil {
				if cpErr := copyToToolsPath(cached, toolsDst); cpErr != nil {
					fmt.Fprintf(os.Stdout, "warning: copy to %s: %v\n", toolsDst, cpErr) //nolint:errcheck // CLI warning
				}
				return cached, nil
			}
			fmt.Fprintf(os.Stdout, "warning: download from %s failed: %v; falling back to local build\n", baseURL, downloadErr) //nolint:errcheck // CLI warning
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
	evictKeepN(filepath.Join(resolveCacheDir(), kernelCacheDir), cached)
	return cached, nil
}

func resolveRuntimeKernel(version, arch, profile, builder string, td kernelTargetDesc, resolved kernelProfileResolution, variant string) (string, error) {
	// The version floor is checked before the cache, so a kernel.version bump
	// below the reserve_mem floor fails the build rather than being served from
	// a tree built when the floor still held.
	if err := enforceReserveMemKernelFloor(version); err != nil {
		return "", err
	}
	cachedDir := kernelTreeCachePath(version, variant)
	cachedKernel := filepath.Join(cachedDir, td.artifact)

	if _, err := os.Stat(cachedKernel); err == nil {
		if cpErr := copyTree(cachedDir, td.outputDir); cpErr != nil {
			fmt.Fprintf(os.Stdout, "warning: copy tree to %s: %v\n", td.outputDir, cpErr) //nolint:errcheck // CLI warning
		}
		return cachedDir, nil
	}

	// Ze builds every kernel on a host of its own arch (AC-13), so a cold cache
	// for another arch cannot be filled here. Refused before the build starts,
	// with the two moves that work: a kernel built on a host of that arch fills
	// that host's cache, never this one, so the remedy is where the image is
	// built, or which arch it is built for.
	if arch != kernelbuilder.HostArch() {
		return "", &foreignArchKernelError{version: version, arch: arch, cachedDir: cachedDir}
	}
	if err := buildKernelArtifact(version, arch, profile, builder, td); err != nil {
		// A cold cache with no network fails here, at the source download, so the
		// error names the entry that was missing and the command that fills it.
		return "", fmt.Errorf("runtime kernel %s for %s is not in the cache at %s, and building it failed: %w\n"+
			"build it once, with network access, on this host:\nze appliance kernel --target runtime --arch %s",
			version, arch, cachedDir, err, arch)
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
	evictKeepN(filepath.Join(resolveCacheDir(), runtimeKernelCacheDir), cachedDir)
	return cachedDir, nil
}

// foreignArchKernelError refuses a runtime kernel this host cannot build: the
// cache holds no entry for arch, and Ze builds every kernel on a host of its own
// arch (AC-13). Error answers the fix for a bare kernel request; forInstance
// answers it for an image build, naming the instance file and the build command.
type foreignArchKernelError struct {
	version, arch, cachedDir string
}

func (e *foreignArchKernelError) refusal(fix string) string {
	var tb textbuf.Buffer
	return tb.Str("runtime kernel ").Str(e.version).Str(" for ").Str(e.arch).
		Str(" is not in the cache at ").Str(e.cachedDir).
		Str(", and this ").Str(kernelbuilder.HostArch()).
		Str(" host cannot build it: Ze builds every kernel on a host of its own arch, with no CPU emulation; ").
		Str(fix).String()
}

func (e *foreignArchKernelError) Error() string {
	return e.refusal("run this on an " + e.arch + " host, which builds it into that host's cache:\nze appliance kernel --target runtime --arch " + e.arch)
}

// forInstance answers the refusal for the image build of appliance name in
// dir. A kernel built on another host fills that host's cache, never this one,
// so the moves that work are building the image there, or building an image
// of this host's arch here; both end in the same command.
func (e *foreignArchKernelError) forInstance(dir, name string) error {
	host := kernelbuilder.HostArch()
	return errors.New(e.refusal("build the image on an " + e.arch + " host, or set \"image\": {\"arch\": \"" + host +
		"\"} in " + ConfigPath(dir, name) + " to build an " + host + " image here; then run:\nze appliance --dir " + shellArg(dir) + " build " + shellArg(name)))
}

// forGokrazy answers the refusal for `./le build gokrazy` run with args, whose
// GOARCH asked for e.arch. The kernel cannot be built here, so the moves that
// work are building on an e.arch host, or building this host's arch here.
func (e *foreignArchKernelError) forGokrazy(args []string) error {
	host := kernelbuilder.HostArch()
	quoted := make([]string, len(args))
	for index, arg := range args {
		quoted[index] = shellArg(arg)
	}
	return errors.New(e.refusal("GOARCH=" + e.arch + " asks for an " + e.arch + " image: build it on an " + e.arch +
		" host, or build an " + host + " image here:\nGOARCH=" + host + " ./le build gokrazy " + strings.Join(quoted, " ")))
}

// shellArg answers value as one shell word: unchanged when it holds only
// characters no shell treats specially, else single-quoted.
func shellArg(value string) string {
	if value != "" && strings.Trim(value, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789/._-=:,+@") == "" {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// runtimeKernelReachable answers the refusal an image build of arch would meet
// at the runtime kernel, before anything long starts: nil when this host can
// build arch, or when the cache already holds its kernel.
func runtimeKernelReachable(arch string) error {
	if arch == kernelbuilder.HostArch() {
		return nil
	}
	td, err := kernelTargetFor(kernelTargetRuntime)
	if err != nil {
		return err
	}
	cachedDir, err := kernelCachePathFor(defaultKernelVersion, arch, runtimeKernelProfile, "", kernelTargetRuntime)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(cachedDir, td.artifact)); err == nil {
		return nil
	}
	return &foreignArchKernelError{version: defaultKernelVersion, arch: arch, cachedDir: cachedDir}
}

// buildKernelArtifact resolves the firmware directory then invokes the native driver.
func buildKernelArtifact(version, arch, profile, builder string, td kernelTargetDesc) error {
	// The arch is refused before the firmware download, which a build this
	// host cannot run would only waste.
	if err := kernelbuilder.CheckArch(arch); err != nil {
		return err
	}
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
	fmt.Fprintf(os.Stdout, "building %s kernel via Go...\n", td.name) //nolint:errcheck // CLI output
	return kernelBuildFn(kernelBuildSpec{
		version:  version,
		arch:     arch,
		profile:  profile,
		builder:  builder,
		target:   td.name,
		srcDir:   td.configDir,
		outDir:   td.outputDir,
		patches:  td.patchesDir,
		firmware: fwDir,
	})
}

// kernelBuildSpec is the request handed to the native kernel builder.
type kernelBuildSpec struct {
	version  string
	arch     string
	profile  string
	builder  string
	target   string
	srcDir   string
	outDir   string
	patches  string
	firmware string
}

func defaultKernelBuild(spec kernelBuildSpec) error {
	if err := os.MkdirAll(spec.outDir, cacheDirPerm); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	resolved, err := resolveKernelProfile(spec.srcDir, spec.profile)
	if err != nil {
		return err
	}
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	buildCtx, cancel := context.WithTimeout(signalCtx, kernelBuildTimeout)
	defer cancel()
	// The builder directory, the common directory and the modules switch are
	// left to the builder (kernelbuilder.normalizeRequest): they decide the
	// kernel's bytes, so they are declared where the cache key hashes them.
	return nativeKernelBuildFn(buildCtx, kernelbuilder.Request{
		Version:     spec.version,
		Arch:        spec.arch,
		Profile:     spec.profile,
		Builder:     spec.builder,
		Target:      spec.target,
		SourceDir:   spec.srcDir,
		OutputDir:   spec.outDir,
		PatchesDir:  spec.patches,
		FirmwareDir: spec.firmware,
		Fragments:   resolved.Fragments,
		Stdout:      os.Stdout,
		Stderr:      os.Stdout,
	})
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
