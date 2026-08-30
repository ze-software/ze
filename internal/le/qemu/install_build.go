// Design: docs/architecture/testing/qemu-integration.md -- installer artifacts
// Overview: install.go -- the shared installer runner
// Related: install_iso.go -- ISO assembly consumes these artifacts
// Related: install_boot.go -- the lifecycle that serves and boots the artifacts
package qemu

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/dockerhost"
)

const installBuildTimeout = 20 * time.Minute

// The command words this package drives two other programs with. The two
// spellings of "build" are two words, not one: goCommandBuild is a Go
// toolchain subcommand and zeApplianceVerbBuild is a verb of the `ze
// appliance` area, so a rename of either must not move the other.
const (
	goCommandBuild       = "build"
	zeApplianceArea      = "appliance"
	zeApplianceVerbBuild = "build"
)

func (installer *Installer) buildHostZe(ctx context.Context, work string) (string, error) {
	return installer.buildHostZeEnv(ctx, work, nil)
}

func (installer *Installer) buildHostZeEnv(ctx context.Context, work string, extraEnv []string) (string, error) {
	ze := filepath.Join(work, "ze-host")
	hostEnv := installEnvWithoutTarget(installer.ops.Environ())
	hostEnv = installEnvSet(hostEnv, extraEnv...)
	result, err := installer.run(ctx, commandSpec{
		Name: "go", Args: []string{goCommandBuild, "-tags", "ze_core,ze_setup", "-o", ze, "./cmd/ze"},
		Dir: installer.Tree, Env: hostEnv,
	})
	if err != nil {
		return "", err
	}
	if result.Code != 0 {
		var tb textbuf.Buffer
		return "", errors.New(tb.Str("host ze build failed:\n").Str(result.Stdout).Str(result.Stderr).String())
	}
	return ze, nil
}

func (installer *Installer) buildInitrd(ctx context.Context, work string, extraEnv ...string) (string, error) {
	ze, err := installer.buildHostZeEnv(ctx, work, extraEnv)
	if err != nil {
		return "", err
	}
	var tb textbuf.Buffer
	arch := tb.Str("GOARCH=").Str(installer.Options.Arch).String()
	targetEnv := installEnvSet(installer.ops.Environ(), "GOOS=linux", arch, "CGO_ENABLED=0")
	targetEnv = installEnvSet(targetEnv, extraEnv...)
	result, err := installer.run(ctx, commandSpec{
		Name: ze, Args: []string{zeApplianceArea, "initrd"}, Dir: installer.Tree, Env: targetEnv,
	})
	if err != nil {
		return "", err
	}
	if result.Code != 0 {
		tb.Reset()
		return "", errors.New(tb.Str("ze appliance initrd failed:\n").Str(result.Stdout).Str(result.Stderr).String())
	}
	for line := range strings.SplitSeq(result.Stdout, "\n") {
		if !strings.HasPrefix(line, "initrd ready:") {
			continue
		}
		path := strings.TrimSpace(strings.TrimPrefix(line, "initrd ready:"))
		if info, statErr := installer.ops.FS.Stat(path); statErr == nil && info.Mode().IsRegular() {
			return path, nil
		}
	}
	fallback := filepath.Join(installer.Tree, "tools", "installer-initrd", "build", "initrd.img.gz")
	if info, statErr := installer.ops.FS.Stat(fallback); statErr == nil && info.Mode().IsRegular() {
		return fallback, nil
	}
	tb.Reset()
	return "", errors.New(tb.Str("ze appliance initrd produced no output:\n").Str(result.Stdout).String())
}

type installImage struct {
	Path string
	ZeFS string
}

func (installer *Installer) buildImage(ctx context.Context, work string) (installImage, error) {
	if installer.Options.Image != "" {
		image := installer.Options.Image
		if !filepath.IsAbs(image) {
			image = filepath.Join(installer.Tree, image)
		}
		zefs := installer.Options.ZeFS
		if zefs != "" && !filepath.IsAbs(zefs) {
			zefs = filepath.Join(installer.Tree, zefs)
		}
		return installImage{Path: image, ZeFS: zefs}, nil
	}
	ze, err := installer.buildHostZe(ctx, work)
	if err != nil {
		return installImage{}, err
	}
	applianceDir := filepath.Join(work, "appliances")
	environ := installer.applianceEnv(applianceDir)
	result, err := installer.run(ctx, commandSpec{
		Name: ze, Args: []string{zeApplianceArea, "init", InstallApplianceName}, Dir: installer.Tree,
		Env: environ, Stdin: strings.NewReader(""),
	})
	if err != nil {
		return installImage{}, err
	}
	var tb textbuf.Buffer
	if result.Code != 0 {
		return installImage{}, errors.New(tb.Str("ze appliance init failed:\n").
			Str(result.Stdout).Str(result.Stderr).String())
	}
	appDir := filepath.Join(applianceDir, InstallApplianceName)
	if err := installer.setApplianceArch(filepath.Join(appDir, "appliance.json")); err != nil {
		return installImage{}, err
	}
	result, err = installer.run(ctx, commandSpec{
		Name: ze, Args: []string{zeApplianceArea, zeApplianceVerbBuild, InstallApplianceName}, Dir: installer.Tree, Env: environ,
	})
	if err != nil {
		return installImage{}, err
	}
	if result.Code != 0 {
		tb.Reset()
		return installImage{}, errors.New(tb.Str("ze appliance build failed:\n").
			Str(result.Stdout).Str(result.Stderr).String())
	}
	images, err := installMatches(applianceDir, "ze-", ".img")
	if err != nil {
		return installImage{}, err
	}
	if len(images) == 0 {
		return installImage{}, errors.New("appliance build produced no image")
	}
	result, err = installer.run(ctx, commandSpec{
		Name: ze, Args: []string{zeApplianceArea, "assemble", "--keep", InstallApplianceName}, Dir: installer.Tree, Env: environ,
	})
	if err != nil {
		return installImage{}, err
	}
	if result.Code != 0 {
		tb.Reset()
		return installImage{}, errors.New(tb.Str("ze appliance assemble failed:\n").
			Str(result.Stdout).Str(result.Stderr).String())
	}
	zefs, err := installMatches(applianceDir, "", ".zefs")
	if err != nil {
		return installImage{}, err
	}
	retained := ""
	if len(zefs) != 0 {
		retained = zefs[0]
	}
	return installImage{Path: images[len(images)-1], ZeFS: retained}, nil
}

func (installer *Installer) applianceEnv(applianceDir string) []string {
	var tb textbuf.Buffer
	applianceSetting := tb.Str("ZE_APPLIANCE_DIR=").Str(applianceDir).String()
	tb.Reset()
	passwordSetting := tb.Str("ze.appliance.ssh.password=").Str(installer.Options.SSHPassword).String()
	// `ze appliance build` runs Docker to build the kernel and the initrd, and it
	// inherits this environment, so the daemon socket is named here when nothing
	// else can name it (`internal/le/dockerhost`).
	home, _ := installer.ops.Home()
	inherited := dockerhost.Environment(installer.ops.GOOS, home, installer.ops.Environ(), installer.ops.Socket)
	environ := installEnvSet(inherited, applianceSetting, passwordSetting)
	dirs := installer.brewKegDirs("e2fsprogs", "sbin")
	if len(dirs) == 0 {
		return environ
	}
	pathValue := ""
	for _, entry := range environ {
		if value, ok := strings.CutPrefix(entry, "PATH="); ok {
			pathValue = value
			break
		}
	}
	parts := append([]string(nil), dirs...)
	if pathValue != "" {
		parts = append(parts, pathValue)
	}
	tb.Reset()
	pathSetting := tb.Str("PATH=").Join(parts, string(os.PathListSeparator)).String()
	return installEnvSet(environ, pathSetting)
}

func (installer *Installer) setApplianceArch(name string) error {
	data, err := installer.ops.FS.ReadFile(name)
	if err != nil {
		return fmt.Errorf("read appliance config %s: %w", name, err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("decode appliance config %s: %w", name, err)
	}
	image, ok := document["image"].(map[string]any)
	if !ok {
		image = make(map[string]any)
		document["image"] = image
	}
	image["arch"] = installer.Options.Arch
	if installer.Options.ImageSize != 0 {
		image["size-bytes"] = installer.Options.ImageSize
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("encode appliance config %s: %w", name, err)
	}
	if err := installer.ops.FS.WriteFile(name, encoded, 0o600); err != nil {
		return fmt.Errorf("write appliance config %s: %w", name, err)
	}
	return nil
}

func installMatches(root, prefix, suffix string) ([]string, error) {
	matches := make([]string, 0, 4)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, suffix) {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk appliance artifacts under %s: %w", root, err)
	}
	sort.Strings(matches)
	return matches, nil
}

func (installer *Installer) writeChecksum(image, served string) (string, error) {
	target := filepath.Join(served, InstallImageName)
	if err := copyInstallFile(image, target); err != nil {
		return "", err
	}
	digest, err := installSHA256(target)
	if err != nil {
		return "", err
	}
	sidecar := filepath.Join(served, InstallImageName+".sha256")
	var tb textbuf.Buffer
	body := tb.Str(digest).Str("  ").Str(InstallImageName).Byte('\n').Bytes()
	if err := installer.ops.FS.WriteFile(sidecar, body, 0o600); err != nil {
		return "", err
	}
	return target, nil
}

func installSHA256(name string) (digest string, resultErr error) {
	// #nosec G304 -- name is a resolved installer artifact path from the closed build plan.
	file, err := os.Open(name)
	if err != nil {
		return "", fmt.Errorf("open %s for checksum: %w", name, err)
	}
	defer func() {
		resultErr = joinInstallCleanup(resultErr, file.Close, "close installer checksum input")
	}()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash %s: %w", name, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func copyInstallFile(source, target string) (resultErr error) {
	// #nosec G304 -- callers provide resolved installer artifact paths; no request data reaches this copy.
	input, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open %s: %w", source, err)
	}
	defer func() {
		resultErr = joinInstallCleanup(resultErr, input.Close, "close installer copy input")
	}()
	// #nosec G304 -- target is constructed beneath the installer-owned work directory.
	output, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("create %s: %w", target, err)
	}
	if _, err := io.Copy(output, input); err != nil {
		return errors.Join(
			fmt.Errorf("copy %s to %s: %w", source, target, err),
			output.Close(),
		)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("close %s: %w", target, err)
	}
	return nil
}

func (installer *Installer) brewDebugfs() string {
	for _, dir := range installer.brewKegDirs("e2fsprogs", "sbin") {
		candidate := filepath.Join(dir, "debugfs")
		if info, err := installer.ops.FS.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate
		}
	}
	return ""
}

func (installer *Installer) brewKegDirs(formula, sub string) []string {
	prefixes := (&Run{ops: installer.ops.runOps}).brewPrefixes()
	dirs := make([]string, 0, len(prefixes)*2)
	for _, prefix := range prefixes {
		opt := filepath.Join(prefix, "opt", formula, sub)
		if info, err := installer.ops.FS.Stat(opt); err == nil && info.IsDir() {
			dirs = append(dirs, opt)
		}
		cellar, _ := filepath.Glob(filepath.Join(prefix, "Cellar", formula, "*", sub))
		sort.Slice(cellar, func(i, j int) bool { return installVersionLess(cellar[j], cellar[i]) })
		for _, dir := range cellar {
			if info, err := installer.ops.FS.Stat(dir); err == nil && info.IsDir() {
				dirs = append(dirs, dir)
			}
		}
	}
	return dirs
}

func (r *Run) brewPrefixes() []string {
	prefixes := make([]string, 0, 4)
	if exported := r.ops.Getenv("HOMEBREW_PREFIX"); exported != "" {
		prefixes = append(prefixes, exported)
	}
	if brew, err := r.ops.Look("brew"); err == nil {
		prefixes = append(prefixes, filepath.Dir(filepath.Dir(brew)))
	}
	if r.ops.GOOS == goosDarwin {
		prefixes = append(prefixes, "/opt/homebrew", "/usr/local")
	}
	seen := make(map[string]bool, len(prefixes))
	result := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		if seen[prefix] {
			continue
		}
		seen[prefix] = true
		if info, err := r.ops.FS.Stat(prefix); err == nil && info.IsDir() {
			result = append(result, prefix)
		}
	}
	return result
}

var installVersionParts = regexp.MustCompile(`[._-]`)

func installVersionLess(left, right string) bool {
	leftParts := installVersionParts.Split(filepath.Base(filepath.Dir(left)), -1)
	rightParts := installVersionParts.Split(filepath.Base(filepath.Dir(right)), -1)
	count := min(len(leftParts), len(rightParts))
	for index := range count {
		leftN, leftErr := strconv.Atoi(leftParts[index])
		rightN, rightErr := strconv.Atoi(rightParts[index])
		if leftErr == nil && rightErr == nil {
			if leftN != rightN {
				return leftN < rightN
			}
			continue
		}
		if leftErr == nil {
			return false
		}
		if rightErr == nil {
			return true
		}
		if leftParts[index] != rightParts[index] {
			return leftParts[index] < rightParts[index]
		}
	}
	return len(leftParts) < len(rightParts)
}
