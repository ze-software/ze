// Design: docs/architecture/testing/qemu-integration.md -- the image an appliance proof boots
// Related: gokrazykernel.go -- the validation every candidate package goes through
// Related: gokrazylab.go -- the network the built image is booted onto
// Related: gokrazyl2tp.go -- the proof that boots the image this builds
//
// gokrazyimage.go produces the appliance image that the proof boots. It resolves
// a kernel that can carry the proof, prepares a gokrazy instance with this
// proof's own configuration, and builds the image.
//
// It reaches the build through `make ze-gokrazy-build` rather than through a Go
// call. The Python original used this method, and this port preserves it. The
// Makefile target owns the build composition, which includes gok, the instance
// preparer, and the kernel package. A copy here would be a second statement of
// that composition. When the Makefile goes (the spec's end state B), this call
// becomes a function call to the package the target already reaches.

package deployment

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ZePackage is the Go package gokrazy is asked to put in the image. It is the
// key this proof's environment settings are attached under.
const ZePackage = "github.com/ze-software/ze/cmd/ze"

// ProofDaemonEnv is the environment that the appliance's ze receives in
// addition to whatever the checked-in instance already sets.
//
// IPv6CP is off because the pool this proof configures hands out v4 only. The
// two timeouts are shorter so a stalled negotiation is REPORTED inside the
// proof's own bound rather than after that bound.
var ProofDaemonEnv = []string{
	"ze.l2tp.ncp.enable-ipv6cp=false",
	"ze.l2tp.ncp.ip-timeout=15s",
	"ze.l2tp.auth.timeout=15s",
}

// These are the bounds on the two builds this file starts. An image build
// compiles the daemon and assembles a disk. A kernel materialized from a cold
// cache requires a container build. Both bounds are generous enough for these
// slow builds. They stop a hung build, not a slow build.
const (
	imageBuildTimeout  = 30 * time.Minute
	kernelBuildTimeout = 60 * time.Minute
)

// These variables let an operator point this proof at their own inputs. The code
// reads them from the OS environment rather than register them as ze settings.
// Each one is a MAKE variable that this proof passes through. KERNEL_PKG is what
// `make ze-gokrazy-build` takes, and the other two name artifacts of a previous
// run.
const (
	KernelPkgEnv        = "KERNEL_PKG"
	GokrazyImageEnv     = "ZE_GOKRAZY_IMAGE"
	GokrazySkipBuildEnv = "ZE_GOKRAZY_SKIP_BUILD"
)

// gokrazyImage builds the appliance image and answers where it landed.
//
// Three routes lead to an image, and only the first builds one. An operator can
// name an image or ask for the last build to be reused. Both routes support a
// developer who iterates on the proof itself. Both carry the warning that the
// image must already contain this proof's template, its environment AND an
// L2TP-capable kernel.
func gokrazyImage(tree, work, template, arch string, progress io.Writer) (string, error) {
	image := proofImagePath(tree, work)

	if os.Getenv(GokrazySkipBuildEnv) == "1" {
		if !isRegularFile(image) {
			var tb textbuf.Buffer
			return "", errors.New(tb.Str("gokrazy image not found: ").Str(image).String())
		}
		writeProgress(progress, "using existing gokrazy image; it must already carry the L2TP"+
			" proof template, the proof runtime environment, and an L2TP-capable kernel")
		return image, nil
	}

	parent, err := prepareInstance(tree, work)
	if err != nil {
		return "", err
	}
	kernel, err := resolveKernelPackage(tree, work, arch, progress)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), imageBuildTimeout)
	defer cancel()

	var tb textbuf.Buffer
	argv := []string{
		"ze-gokrazy-build",
		tb.Str("GOKRAZY_DIR=").Str(parent).String(),
		"GOKRAZY_INSTANCE=ze",
		tb.Reset().Str("GOKRAZY_ARCH=").Str(arch).String(),
		tb.Reset().Str("GOKRAZY_IMG=").Str(image).String(),
		"USER=admin",
		"PASS=secret",
		tb.Reset().Str("GOKRAZY_TEMPLATE=").Str(template).String(),
		tb.Reset().Str("KERNEL_PKG=").Str(kernel).String(),
	}

	build := makeCommand(ctx, tree, argv...)
	build.Stdout = progress
	build.Stderr = progress
	build.Env = append(os.Environ(), gokrazyBuildUser()...)
	if err := build.Run(); err != nil {
		return "", errors.New("make ze-gokrazy-build for L2TP appliance evidence failed")
	}
	if !isRegularFile(image) {
		return "", errors.New(tb.Reset().Str("gokrazy image not found after build: ").Str(image).String())
	}
	return image, nil
}

// makeCommand answers one `make` invocation over the tree.
//
// The Makefile owns the composition of the image, host, and kernel builds.
// This proof ports a DRIVER. It does not redesign what the DRIVER drives.
func makeCommand(ctx context.Context, tree string, argv ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "make", argv...) //nolint:gosec // the argv is this package's own, never an operator's
	cmd.Dir = tree
	return cmd
}

// hostToolText runs one host binary over the tree. It answers the standard
// output and whether the binary succeeded.
//
// The function returns standard output alone because the caller reads an ANSWER
// from it. If the tool writes a warning to the other stream, the caller does not
// read that warning as part of the answer.
func hostToolText(ctx context.Context, tree, name string, argv ...string) (string, bool) {
	cmd := exec.CommandContext(ctx, name, argv...) //nolint:gosec // the argv is this package's own, never an operator's
	cmd.Dir = tree
	out, err := cmd.Output()
	return string(out), err == nil
}

// gokrazyBuildUser answers the one environment entry the build needs and a
// developer's shell CAN lack. gokrazy names the image's account after USER.
// A build agent often has no USER entry.
func gokrazyBuildUser() []string {
	if os.Getenv("USER") != "" {
		return nil
	}
	return []string{"USER=admin"}
}

// proofImagePath answers where the image is, or where it will be built.
func proofImagePath(tree, work string) string {
	if named := os.Getenv(GokrazyImageEnv); named != "" {
		if filepath.IsAbs(named) {
			return named
		}
		return filepath.Join(tree, named)
	}
	if os.Getenv(GokrazySkipBuildEnv) == "1" {
		return filepath.Join(tree, "tmp", "gokrazy", "ze.img")
	}
	return filepath.Join(work, "ze.img")
}

// prepareInstance writes a gokrazy parent directory that carries ONLY this
// proof's change to the checked-in instance, and it answers that directory.
//
// The build directory is SYMLINKED rather than copied. The build's own preparer
// copies it into the instance it assembles and resolves the symlink on the way.
// A copy here would duplicate the work and create a second place that CAN go
// stale.
func prepareInstance(tree, work string) (string, error) {
	parent := filepath.Join(work, "gokrazy-parent")
	instance := filepath.Join(parent, "ze")
	if err := os.MkdirAll(instance, 0o750); err != nil {
		return "", err
	}

	source := filepath.Join(tree, "gokrazy", "ze")
	link := filepath.Join(instance, "builddir")
	if err := os.RemoveAll(link); err != nil {
		return "", err
	}
	if err := os.Symlink(filepath.Join(source, "builddir"), link); err != nil {
		return "", err
	}

	patched, err := instanceConfig(filepath.Join(source, "config.json"))
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(instance, "config.json"), patched, 0o600); err != nil {
		return "", err
	}
	return parent, nil
}

// instanceConfig answers the checked-in gokrazy configuration with this proof's
// environment added to ze's package entry.
//
// The function leaves an entry whose KEY is already present unchanged. The
// checked-in instance is allowed to set any of these entries. A duplicate would
// let the two values disagree, and the last value would silently win.
func instanceConfig(path string) ([]byte, error) {
	body, err := os.ReadFile(path) //nolint:gosec // a path inside the checkout this command was run in
	if err != nil {
		return nil, err
	}

	var config map[string]any
	if err := json.Unmarshal(body, &config); err != nil {
		return nil, err
	}

	packages, _ := config["PackageConfig"].(map[string]any)
	if packages == nil {
		packages = map[string]any{}
		config["PackageConfig"] = packages
	}
	entry, _ := packages[ZePackage].(map[string]any)
	if entry == nil {
		entry = map[string]any{}
		packages[ZePackage] = entry
	}

	existing, _ := entry["Environment"].([]any)
	for _, item := range ProofDaemonEnv {
		key, _, _ := strings.Cut(item, "=")
		if environmentHasKey(existing, key) {
			continue
		}
		existing = append(existing, item)
	}
	entry["Environment"] = existing

	rendered, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return nil, err
	}
	return append(rendered, '\n'), nil
}

// environmentHasKey reports whether the list already sets key.
func environmentHasKey(entries []any, key string) bool {
	for _, entry := range entries {
		text, ok := entry.(string)
		if !ok {
			continue
		}
		if name, _, _ := strings.Cut(text, "="); name == key {
			return true
		}
	}
	return false
}

// resolveKernelPackage answers a per-run copy of a kernel package that can
// carry this proof.
//
// The function tries three routes in order. It validates an operator's own
// package for architecture and PPPoL2TP but NOT for version because the operator
// chose to name it. It reuses a package that a previous build already staged if
// the package is valid for the pinned version. Otherwise, it materializes the
// runtime kernel from the durable cache. If the cache cannot provide the kernel,
// the function reports the command that fixes it rather than produce an image
// that can only crash-loop.
func resolveKernelPackage(tree, work, arch string, progress io.Writer) (string, error) {
	if named := os.Getenv(KernelPkgEnv); named != "" {
		pkg := named
		if !filepath.IsAbs(pkg) {
			pkg = filepath.Join(tree, pkg)
		}
		if err := assertKernelPackage(pkg, arch, "from KERNEL_PKG", ""); err != nil {
			return "", err
		}
		return copyKernelPackage(pkg, work, arch)
	}

	pinned, err := PinnedKernelVersion(tree)
	if err != nil {
		return "", err
	}
	staged := filepath.Join(tree, "tmp", "kernel", "pkg")
	problems, err := kernelPackageProblems(staged, arch, pinned)
	if err != nil {
		return "", err
	}
	if len(problems) == 0 {
		return copyKernelPackage(staged, work, arch)
	}

	cache, err := kernelCacheDir(tree, arch, progress)
	if err != nil {
		return "", err
	}
	cacheProblems, err := kernelPackageProblems(cache, arch, pinned)
	if err != nil {
		return "", err
	}
	if len(cacheProblems) > 0 {
		return "", coldCacheError(cache, arch, cacheProblems)
	}

	writeProgress(progress, "materializing the runtime kernel package from the durable cache"+
		" (make ze-kernel-build; instant on this validated cache)...")

	ctx, cancel := context.WithTimeout(context.Background(), kernelBuildTimeout)
	defer cancel()

	var tb textbuf.Buffer
	build := makeCommand(ctx, tree, "ze-kernel-build", tb.Str("KERNEL_ARCH=").Str(arch).String())
	// The command output is streamed rather than captured. If the cache is evicted
	// between the check above and this call, the target becomes a container build
	// of about half an hour. Its progress must be visible while it happens.
	build.Stdout = progress
	build.Stderr = progress
	if err := build.Run(); err != nil {
		return "", errors.New("make ze-kernel-build failed; see the output above")
	}
	if err := assertKernelPackage(staged, arch, "assembled by make ze-kernel-build", pinned); err != nil {
		return "", err
	}
	return copyKernelPackage(staged, work, arch)
}

// kernelCacheDir asks the host ze where the runtime kernel for this architecture
// is cached.
//
// The host binary is built first because it owns this answer. The cache layout
// is keyed by architecture and by the pinned version. A second statement of that
// layout here would create a second thing to keep in step.
func kernelCacheDir(tree, arch string, progress io.Writer) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), imageBuildTimeout)
	defer cancel()

	host := makeCommand(ctx, tree, "ze-host-build")
	host.Stdout = progress
	host.Stderr = progress
	if err := host.Run(); err != nil {
		return "", errors.New("make ze-host-build failed")
	}

	out, ok := hostToolText(ctx, tree, filepath.Join(tree, "ze-host"),
		"appliance", "kernel", "--target", "runtime", "--arch", arch, "--print-cache-dir")
	if !ok {
		return "", errors.New("resolve runtime kernel cache dir failed")
	}

	var last string
	for line := range strings.SplitSeq(out, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			last = trimmed
		}
	}
	if last == "" {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str("could not resolve the runtime kernel cache dir: ze-host").
			Str(" appliance kernel --print-cache-dir produced no output; try:").
			Str(" make ze-kernel-build KERNEL_ARCH=").Str(arch).String())
	}
	return last, nil
}

// coldCacheError answers the refusal for a cache that cannot provide the runtime
// kernel. It includes the remediation that actually applies.
//
// An operator must first remove an EXISTING but unusable entry. The build
// target's hit branch tests existence alone and would re-materialize the bad
// entry unchanged. The message identifies the cache's owner because this proof
// usually runs under sudo, and sudo resets HOME.
func coldCacheError(cache, arch string, problems []string) error {
	remedy := "build it once with: make ze-kernel-build KERNEL_ARCH="
	if isDir(cache) {
		remedy = "the cache entry exists but is unusable; remove it, then rebuild it with: make ze-kernel-build KERNEL_ARCH="
	}

	var tb textbuf.Buffer
	tb.Str("this proof needs the runtime kernel (PPPoL2TP built in; the pinned kernel has none").
		Str(" and the appliance would crash-loop on ze's fail-closed module probe), but the").
		Str(" durable cache at ").Str(cache).Str(" cannot provide it:\n  ").
		Str(strings.Join(problems, "\n  ")).Byte('\n').
		Str(remedy).Str(arch).Str(" (about 30 minutes, needs docker), then re-run this proof.\n").
		Str("note: this proof usually runs under sudo, and sudo commonly resets HOME, so the").
		Str(" cache read here is root's; a kernel built as your own user lives in YOUR cache")
	return errors.New(tb.String())
}

// assertKernelPackage answers an error naming every reason pkg cannot carry the
// proof, or nil.
func assertKernelPackage(pkg, arch, context, wantVersion string) error {
	problems, err := kernelPackageProblems(pkg, arch, wantVersion)
	if err != nil {
		return err
	}
	if len(problems) == 0 {
		return nil
	}
	return kernelPackageError(context, arch, problems)
}

// copyKernelPackage copies a package into this run's own directory and
// validates the COPY.
//
// A concurrent kernel build rewrites the shared staged path. The rewrite starts
// when the build removes the path. This build reads the package minutes after
// the validation. The per-run copy prevents a race between those actions.
func copyKernelPackage(src, work, arch string) (string, error) {
	dst := filepath.Join(work, "kernel-pkg")
	if err := os.RemoveAll(dst); err != nil {
		return "", err
	}
	if err := copyTree(src, dst); err != nil {
		return "", err
	}

	var tb textbuf.Buffer
	if err := assertKernelPackage(dst, arch, tb.Str("per-run copy of ").Str(src).String(), ""); err != nil {
		return "", err
	}
	return dst, nil
}

// copyTree copies a directory, keeping symlinks as symlinks.
//
// A kernel package's module tree carries a `build` link into an absent source
// tree. The copy keeps that link as a symlink. Following the link would fail or
// copy a kernel source tree.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		info, err := entry.Info()
		if err != nil {
			return err
		}
		switch {
		case entry.IsDir():
			return os.MkdirAll(target, 0o750)
		case info.Mode()&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target) //nolint:gosec // G122: the source is a kernel package this run validated, and the destination is under this run's own directory
		default:
			return copyFile(path, target, info.Mode())
		}
	})
}

// copyFile copies one file, keeping its mode.
func copyFile(src, dst string, mode os.FileMode) error {
	from, err := os.Open(src) //nolint:gosec // a path inside the package this command was pointed at
	if err != nil {
		return err
	}
	defer from.Close() //nolint:errcheck // a read-only file

	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	to, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode) //nolint:gosec // a path under this run's own directory
	if err != nil {
		return err
	}
	if _, err := io.Copy(to, from); err != nil {
		to.Close() //nolint:errcheck // the copy already failed
		return err
	}
	return to.Close()
}

// isRegularFile reports whether path is a file rather than a directory or
// nothing at all.
func isRegularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// isDir reports whether path is a directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
