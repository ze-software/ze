// Design: docs/architecture/appliance/build-artifacts.md -- the runtime-kernel cache key

package kernelbuilder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/core/tmplink"
)

// BuildSources names the files of this package that decide a kernel's bytes:
// this file, which holds what the host hands the build (the container argv, the
// QEMU guest's toolchain and command line, the request defaults, the provenance
// record), and worker.go, which is the build itself. The cached-kernel key
// (internal/appliance, kernelBuilderSources) hashes these, every file under
// tools/kernel-builder, and every Ze package they import, and nothing else.
//
// Every other file here is host-side: it probes, measures, or words a refusal,
// and an edit there keeps every cached kernel. Code that changes what the build
// produces goes in a file named here, never in a host-side file.
// TestKernelCacheKeyCoversEveryBuildInput (internal/appliance) fails when a file
// named here reaches a declaration that lives outside them.
var BuildSources = []string{"recipe.go", "worker.go"}

const (
	ProvenanceName = "kernel.version"

	// The provenance record's keys for its source: what writeProvenance writes
	// and what the image assembly (internal/appliance/instance) reads back for
	// the GPLv2 notice. Declared once, here, beside the writer.
	ProvenanceKeyVersion      = "version"
	ProvenanceKeySourceURL    = "source-url"
	ProvenanceKeySourceSHA256 = "source-sha256"

	archAMD64   = "amd64"
	archX8664   = "x86_64"
	archARM64   = "arm64"
	archAArch64 = "aarch64"

	modulesYes = "yes"
	modulesNo  = "no"

	// The two backends. Each compiles with its own toolchain (the Debian gcc of
	// the Docker image, the Alpine gcc of the QEMU guest), so the backend is
	// part of the cached-kernel key and of the provenance record.
	backendDocker = "docker"
	backendQEMU   = "qemu"

	// DefaultBuilderDir is the worker's Go package and the Docker build
	// context; DefaultCommonDir holds the config fragments every profile
	// shares. internal/appliance hashes the first whole into the cache key and
	// resolves profiles from the second, so both read these declarations.
	DefaultBuilderDir = "tools/kernel-builder"
	DefaultCommonDir  = "tools/kernel-builder/common"
	defaultImage      = "ze-kernel-builder"

	targetRuntime   = "runtime"
	targetInstaller = "installer"

	dockerPlatformFlag = "--platform"

	// buildVerb is the subcommand of both docker and go that builds.
	buildVerb = "build"

	// The QEMU guest's toolchain: the Alpine release whose packages build the
	// kernel, and the packages installed into it. python3 is a kernel build
	// dependency, as in tools/kernel-builder/Dockerfile: the arm64 runtime build
	// reaches drivers/gpu/drm/msm, whose Makefile runs gen_header.py, and without
	// python3 the guest build dies at GENHDR with "python3: not found".
	alpineVersion = "3.21"
	alpineMinor   = "3"
	buildPackages = "build-base bc bison flex elfutils-dev openssl-dev linux-headers perl python3 wget xz diffutils findutils cpio patch kmod zstd"

	// The 9p mount options every guest share uses.
	guestMount = " -t 9p -o trans=virtio,version=9p2000.L,msize=1048576"
)

var dockerPlatforms = map[string]string{
	archAMD64: "linux/amd64",
	archX8664: "linux/amd64",
	archARM64: "linux/arm64",
}

// The mount points the kernel build container sees. A path handed to the
// container is one of these joined with a path relative to its host directory.
const (
	containerSrcDir    = "/src"
	containerCommonDir = "/builder/common"
)

// Request is one complete kernel build. Fragments MUST be in merge order. It
// is declared here because its fields are what the build receives: a field
// added or retyped changes the build, so it changes the cache key.
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

// normalizeRequest fills the defaults a request may leave out and rewrites it
// into the form every backend reads. validateRequest runs it, then refuses what
// is still wrong. It is the only code that writes a Request field: a rewrite
// decides what the build receives, so it lives in a file the cache key hashes
// (TestHostSideNeverWritesTheRequest, internal/appliance).
func normalizeRequest(req *Request) error {
	if req.Root == "" {
		req.Root = "."
	}
	root, err := filepath.Abs(req.Root)
	if err != nil {
		return fmt.Errorf("resolve repository root: %w", err)
	}
	req.Root = root
	if req.BuilderDir == "" {
		req.BuilderDir = DefaultBuilderDir
	}
	if req.CommonDir == "" {
		req.CommonDir = DefaultCommonDir
	}
	if req.Image == "" {
		req.Image = defaultImage
	}
	if req.Target == "" {
		if req.Modules == modulesYes {
			req.Target = targetRuntime
		} else {
			req.Target = targetInstaller
		}
	}
	// The target decides whether the kernel carries modules: a caller names
	// the target, never the modules switch, so the value that changes the
	// kernel's bytes is declared here, where the key hashes it.
	req.Modules = targetModules[req.Target]
	if req.Stdout == nil {
		req.Stdout = os.Stdout
	}
	if req.Stderr == nil {
		req.Stderr = os.Stderr
	}
	if req.Arch == archAArch64 {
		req.Arch = archARM64
	}
	if req.OutputDir != "" {
		req.OutputDir = filepath.Clean(req.OutputDir)
	}
	return nil
}

// targetModules answers, for each kernel target, whether the kernel is built
// with loadable modules: the runtime tree carries lib/modules, the installer
// Image is monolithic. A target missing here has no answer, and
// validateRequest refuses it.
var targetModules = map[string]string{
	targetRuntime:   modulesYes,
	targetInstaller: modulesNo,
}

// containerBuildArgs answers the `docker build` argv that builds the kernel
// builder image: the toolchain every Docker kernel is compiled with. runDocker
// passes it unchanged (TestRunDockerArgvAndOwnershipRepair).
func containerBuildArgs(req Request) []string {
	return []string{buildVerb, dockerPlatformFlag, dockerPlatforms[req.Arch], "-t", req.Image, "-f", filepath.Join(req.BuilderDir, "Dockerfile"), req.Root}
}

// guestWorkerBuild answers how the host compiles the worker the QEMU guest
// runs: the repository-relative binary path, the go command's argv, and the
// environment added to the host's. buildGuestWorker passes all three
// unchanged (TestBuildGuestWorkerTakesTheRecipe).
func guestWorkerBuild(req Request) (rel string, args, env []string) {
	goarch := workerGOARCH(req.Arch)
	rel = filepath.Join("tmp", backendQEMU, "bin", goarch, "ze-kernel-builder")
	args = []string{buildVerb, "-o", filepath.Join(req.Root, rel), "./" + DefaultBuilderDir}
	env = []string{"GOOS=linux", "GOARCH=" + goarch, "CGO_ENABLED=0"}
	return rel, args, env
}

// containerRunArgs answers the `docker run` argv that runs the worker in the
// kernel builder image. runDocker passes it unchanged
// (TestRunDockerArgvAndOwnershipRepair).
func containerRunArgs(req Request) ([]string, error) {
	args := []string{
		"run", "--rm", dockerPlatformFlag, dockerPlatforms[req.Arch],
		"-e", "KERNEL_VERSION=" + req.Version,
		"-v", filepath.Join(req.Root, req.SourceDir) + ":/src:ro",
		"-v", filepath.Join(req.Root, req.CommonDir) + ":/builder/common:ro",
		"-v", hostOutputPath(req) + ":/out",
		"-v", "ze-kernel-build:" + defaultBuildDir,
		"-v", "ze-kernel-work:" + defaultWorkDir,
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
			return nil, fmt.Errorf("patches directory %s must be under source directory %s", req.PatchesDir, req.SourceDir)
		}
		args = append(args, "--patches-dir", filepath.ToSlash(filepath.Join(containerSrcDir, rel)))
	}
	if req.FirmwareDir != "" {
		args = append(args, "--firmware-dir", "/firmware")
	}
	for _, fragment := range req.Fragments {
		mapped, err := containerFragmentPath(req, fragment)
		if err != nil {
			return nil, err
		}
		args = append(args, "--fragment", mapped)
	}
	return args, nil
}

func containerFragmentPath(req Request, fragment string) (string, error) {
	fragment = filepath.Clean(fragment)
	if rel, err := filepath.Rel(req.CommonDir, fragment); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(filepath.Join(containerCommonDir, rel)), nil
	}
	if rel, err := filepath.Rel(req.SourceDir, fragment); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(filepath.Join(containerSrcDir, rel)), nil
	}
	return "", fmt.Errorf("fragment %s is outside source directory %s and common directory %s", fragment, req.SourceDir, req.CommonDir)
}

func hostOutputPath(req Request) string {
	if filepath.IsAbs(req.OutputDir) {
		return req.OutputDir
	}
	return filepath.Join(req.Root, req.OutputDir)
}

// guestSetup answers the shell line the guest runs before the worker, and the
// guest path the worker writes its output to. shares are the checkout tmp
// directories that live outside the tree (internal/core/tmplink).
func guestSetup(req Request, shares []tmplink.Share) (string, string) {
	var tb textbuf.Buffer
	tb.Str("set -e && printf 'https://dl-cdn.alpinelinux.org/alpine/v").Str(alpineVersion).
		Str("/main\\nhttps://dl-cdn.alpinelinux.org/alpine/v").Str(alpineVersion).
		Str("/community\\n' > /etc/apk/repositories && apk update && apk add --no-cache ").Str(buildPackages).
		Str(" ccache && mkdir -p /workspace && mount").Str(guestMount).Str(" workspace /workspace").
		Str(" && mkdir -p /ccache && mount").Str(guestMount).Str(" ccache /ccache").
		Str(" && mkdir -p /build && mount").Str(guestMount).Str(" builddir /build")
	guestOutput := "/workspace/" + filepath.ToSlash(req.OutputDir)
	if filepath.IsAbs(req.OutputDir) {
		tb.Str(" && mkdir -p /output && mount").Str(guestMount).Str(" output /output")
		guestOutput = "/output"
	}
	if req.FirmwareDir != "" {
		tb.Str(" && mkdir -p /firmware && mount").Str(guestMount).Str(",ro firmware /firmware")
	}
	// The worker binary and the runtime output both sit under /workspace/tmp,
	// so a relocated tmp child is mounted at its own absolute path before the
	// worker follows the symlink the 9p mount hands it.
	for _, share := range shares {
		tb.Str(" && mkdir -p ").Str(shellQuote(share.Guest)).Str(" && mount").Str(guestMount).Byte(' ').Str(share.Tag).Byte(' ').Str(shellQuote(share.Guest))
	}
	return tb.String(), guestOutput
}

// guestBuildCommand answers the one shell command the QEMU guest runs over SSH:
// the setup line, then the worker with the request's arguments, under ccache.
// runQEMUBuild passes it unchanged (TestRunQEMUBuildSendsTheRecipeCommand,
// internal/appliance).
func guestBuildCommand(req Request, workerRel string, shares []tmplink.Share) string {
	setup, guestOutput := guestSetup(req, shares)
	workerArgs := []string{"/workspace/" + workerRel, "--version", req.Version, "--arch", req.Arch, "--profile", req.Profile, "--src-dir", "/workspace/" + filepath.ToSlash(req.SourceDir), "--out-dir", guestOutput, "--modules", req.Modules}
	for _, fragment := range req.Fragments {
		workerArgs = append(workerArgs, "--fragment", "/workspace/"+filepath.ToSlash(fragment))
	}
	if req.PatchesDir != "" {
		workerArgs = append(workerArgs, "--patches-dir", "/workspace/"+filepath.ToSlash(req.PatchesDir))
	}
	if req.FirmwareDir != "" {
		workerArgs = append(workerArgs, "--firmware-dir", "/firmware")
	}
	if req.Jobs != "" {
		workerArgs = append(workerArgs, "--jobs", req.Jobs)
	}
	return "sh -c " + shellQuote(setup+" && CCACHE_DIR=/ccache CCACHE_MAXSIZE=5G PATH=/usr/lib/ccache/bin:$PATH "+shellJoin(workerArgs))
}

func shellQuote(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
func shellJoin(args []string) string {
	quoted := make([]string, len(args))
	for index, arg := range args {
		quoted[index] = shellQuote(arg)
	}
	return strings.Join(quoted, " ")
}

// writeProvenance records what was built, and the source it was built from: the
// tarball URL downloadKernelSource fetches and the tracked SHA-256
// verifyKernelSource checked. Both derive from req.Version through the same
// declarations the worker uses, so the record cannot name a source the build
// did not verify. A version with no tracked digest gets no record, because the
// worker refuses to build it and an empty digest would read as verified.
func writeProvenance(outputDir string, req Request, backend string) error {
	digest, tracked := SourceDigest(req.Version)
	if !tracked {
		return fmt.Errorf("write kernel provenance: linux %s has no tracked SHA-256 (kernelSourceSHA256), so its source is not one the build verified", req.Version)
	}
	path := hostOutputPath(Request{Root: req.Root, OutputDir: outputDir})
	if err := os.MkdirAll(path, 0o750); err != nil {
		return fmt.Errorf("create provenance directory: %w", err)
	}
	var tb textbuf.Buffer
	data := tb.Str(ProvenanceKeyVersion).Byte('=').Str(req.Version).
		Str("\ntarget=").Str(req.Target).
		Str("\nprofile=").Str(req.Profile).
		Str("\narch=").Str(req.Arch).
		Str("\nmodules=").Str(req.Modules).
		Str("\nbuilder=").Str(backend).
		Byte('\n').Str(ProvenanceKeySourceURL).Byte('=').Str(kernelTarballURL(req.Version)).
		Byte('\n').Str(ProvenanceKeySourceSHA256).Byte('=').Str(digest).
		Str("\n").Bytes()
	if err := os.WriteFile(filepath.Join(path, ProvenanceName), data, 0o600); err != nil {
		return fmt.Errorf("write kernel provenance: %w", err)
	}
	return nil
}

// workerGOARCH answers the GOARCH the worker binary runs as for a kernel arch.
func workerGOARCH(arch string) string {
	if arch == archARM64 {
		return archARM64
	}
	return archAMD64
}
