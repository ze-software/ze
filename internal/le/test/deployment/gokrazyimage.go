// Design: docs/architecture/testing/qemu-integration.md -- the image an appliance proof boots
// Related: gokrazykernel.go -- the validation every candidate package goes through
// Related: gokrazylab.go -- the network the built image is booted onto
// Related: gokrazyl2tp.go -- the proof that boots the image this builds
//
// gokrazyimage.go produces the appliance image that the proof boots. It resolves
// a kernel that can carry the proof, prepares a gokrazy instance with this
// proof's own configuration, and builds the image.
//
// Every build step is a compiled Go path. The host driver, `le build gokrazy`,
// kernel builder, and image database injector are shared with the product
// appliance commands.

package testdeployment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/appliance"
	"github.com/ze-software/ze/internal/core/textbuf"
	buildcompile "github.com/ze-software/ze/internal/le/build/compile"
	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
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

// imageBuildTimeout bounds compilation and disk assembly. It stops a hung
// build, not a slow build.
const imageBuildTimeout = 30 * time.Minute

// These variables let an operator point this proof at their own inputs. The
// code reads them from the OS environment rather than register them as Ze
// settings because they select local evidence artifacts.
const (
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
	// The build resolves ze's runtime kernel itself. This proof checks the cache
	// entry first because it needs PPPoL2TP, and a cold cache is refused with the
	// command that fills it rather than started as a 30-minute build here.
	if err := checkRuntimeKernelCache(tree, arch, progress); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), imageBuildTimeout)
	defer cancel()

	host, err := hostTool(tree)
	if err != nil {
		return "", err
	}
	database, err := prepareProofDatabase(ctx, host, work, template, progress)
	if err != nil {
		return "", err
	}
	if err := buildGokrazyImage(ctx, tree, parent, arch, image, progress); err != nil {
		return "", err
	}
	if err := appliance.InjectDatabase(image, database); err != nil {
		return "", err
	}
	if !isRegularFile(image) {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str("gokrazy image not found after build: ").Str(image).String())
	}
	return image, nil
}

func hostTool(tree string) (string, error) {
	report, code := buildcompile.BuildHost(tree)
	if code != 0 || report.Output == "" {
		return "", errors.New("build native host appliance driver")
	}
	return report.Output, nil
}

func prepareProofDatabase(
	ctx context.Context,
	host, work, template string,
	progress io.Writer,
) (string, error) {
	configDir := filepath.Join(work, "init")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		return "", err
	}
	database := filepath.Join(configDir, "database.zefs")
	initCommand := exec.CommandContext(ctx, host,
		"init", "--force", "--yes", "--seed", "--web-cert", "0.0.0.0:8080")
	initCommand.Env = append(os.Environ(), "ze.config.dir="+configDir)
	initCommand.Stdin = strings.NewReader("admin\nsecret\n0.0.0.0\n22\nze\n")
	initCommand.Stdout, initCommand.Stderr = progress, progress
	if err := initCommand.Run(); err != nil {
		return "", fmt.Errorf("initialize appliance database: %w", err)
	}
	writeTemplate := exec.CommandContext(ctx, host, //nolint:gosec // host is the repository-built appliance driver; no safer execution API exists
		"data", "--path", database, "write", "file/template/ze.conf", template)
	writeTemplate.Stdout, writeTemplate.Stderr = progress, progress
	if err := writeTemplate.Run(); err != nil {
		return "", fmt.Errorf("write appliance configuration template: %w", err)
	}
	return database, nil
}

func buildGokrazyImage(
	ctx context.Context,
	tree, parent, arch, image string,
	progress io.Writer,
) error {
	toolchain, err := gotoolchain.New(tree)
	if err != nil {
		return err
	}
	// gok runs in a child le, `le build gokrazy`, rather than in this process:
	// the child's environment targets the appliance architecture, for which it
	// resolves ze's runtime kernel, and gok's packer calls os.Exit on a failed
	// build, which would end this proof without its report.
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate le for build gokrazy: %w", err)
	}

	run := exec.CommandContext(ctx, self, "build", "gokrazy", //nolint:gosec // self is this le binary, and the argv is this package's own
		"--parent_dir", parent, "-i", "ze", "overwrite",
		"--full", image, "--target_storage_bytes", "2147483648")
	run.Dir = tree
	run.Env = toolchain.Environment(gotoolchain.EnvOptions{
		GOOS: "linux", GOARCH: arch,
	})
	run.Env = append(run.Env, gokrazyBuildUser()...)
	run.Stdout, run.Stderr = progress, progress
	if err := run.Run(); err != nil {
		return fmt.Errorf("build gokrazy image: %w", err)
	}
	return nil
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

// checkRuntimeKernelCache refuses a runtime kernel cache entry that cannot
// carry this proof: absent, built for another arch, without PPPoL2TP, or not the
// pinned version. The image build reads the same entry, so a refusal here names
// the build's own kernel before the build starts.
func checkRuntimeKernelCache(tree, arch string, progress io.Writer) error {
	pinned, err := pinnedKernelVersion(tree)
	if err != nil {
		return err
	}
	cache, err := RuntimeKernelCacheDir(tree, arch, progress)
	if err != nil {
		return err
	}
	problems, err := kernelPackageProblems(cache, arch, pinned)
	if err != nil {
		return err
	}
	if len(problems) > 0 {
		return coldCacheError(cache, arch, problems)
	}
	return nil
}

// RuntimeKernelCacheDir asks the host ze where the runtime kernel for this architecture
// is cached.
//
// The host binary is built first because it owns this answer. The cache layout
// is keyed by architecture and by the pinned version. A second statement of that
// layout here would create a second thing to keep in step.
func RuntimeKernelCacheDir(tree, arch string, progress io.Writer) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), imageBuildTimeout)
	defer cancel()

	host, err := hostTool(tree)
	if err != nil {
		return "", err
	}
	writeProgress(progress, "reading the native runtime-kernel cache identity...")
	out, ok := hostToolText(ctx, tree, host,
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
		return "", errors.New("could not resolve the runtime kernel cache dir: " +
			"ze-host appliance kernel --print-cache-dir produced no output; try: " +
			"./ze appliance kernel --target runtime --arch " + arch)
	}
	return last, nil
}

// coldCacheError answers the refusal for a cache that cannot provide the runtime
// kernel. It includes the remediation that actually applies.
//
// An operator must first remove an existing but unusable entry. The message
// identifies the cache's owner because this proof usually runs under sudo, and
// sudo resets HOME.
func coldCacheError(cache, arch string, problems []string) error {
	remedy := "build it once with: ./ze appliance kernel --target runtime --arch "
	if isDir(cache) {
		remedy = "the cache entry exists but is unusable; remove it, then rebuild it with: ./ze appliance kernel --target runtime --arch "
	}

	var tb textbuf.Buffer
	tb.Str("this proof needs ze's runtime kernel with PPPoL2TP (without it the appliance").
		Str(" would crash-loop on ze's fail-closed module probe), but the").
		Str(" durable cache at ").Str(cache).Str(" cannot provide it:\n  ").
		Str(strings.Join(problems, "\n  ")).Byte('\n').
		Str(remedy).Str(arch).Str(" (about 30 minutes, on a host of that arch, with Docker or QEMU), then re-run this proof.\n").
		Str("note: this proof usually runs under sudo, and sudo commonly resets HOME, so the").
		Str(" cache read here is root's; a kernel built as your own user lives in YOUR cache")
	return errors.New(tb.String())
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
