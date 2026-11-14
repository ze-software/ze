// Design: docs/guide/operations.md -- upgrading a standalone installation.
// Related: register_native_install.go -- the same-binary lifecycle and the helpers this reuses.

package fixture

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/test/runner"
)

func init() {
	Register("ui/native-install-upgrade", nativeInstallUpgrade)
}

// nativeUpgradeRun is one private installation prefix: its installed binary,
// config directory and runtime directory.
type nativeUpgradeRun struct {
	prefix     string
	binary     string
	configDir  string
	runtimeDir string
}

// nativeInstallUpgrade upgrades a foreground installation of ze to
// ze-upgrade, a build of the same product that differs in bytes and in the
// release it reports, then bootstraps a second, fresh prefix with ze-upgrade.
// Only the shipped `ze install` command decides whether the upgrade copied the
// new binary and kept the store, so no unit seam can answer it.
func nativeInstallUpgrade(ctx context.Context, args []string) (upgradeErr error) {
	started := time.Now()
	fmt.Fprintln(os.Stderr, "native upgrade +0s begin")
	defer func() {
		code := 0
		if upgradeErr != nil {
			code = 1
		}
		fmt.Fprintln(os.Stderr, "native upgrade +"+time.Since(started).Round(time.Millisecond).String()+" end exit="+strconv.Itoa(code))
	}()
	ctx, cancel := context.WithTimeout(ctx, deploymentBudget(90*time.Second))
	defer cancel()
	if len(args) != 1 {
		return errors.New("native upgrade fixture requires the runner's SSH port")
	}
	port, err := strconv.ParseUint(args[0], 10, 16)
	if err != nil {
		return err
	}
	if port == 0 {
		return errors.New("native upgrade fixture requires a nonzero SSH port")
	}
	current, err := exec.LookPath("ze")
	if err != nil {
		return fmt.Errorf("ze is not on the child PATH: %w", err)
	}
	upgrade, err := exec.LookPath(runner.UpgradeBinary)
	if err != nil {
		return fmt.Errorf("%s is not on the child PATH: %w; run the ui suite through ./le test functional ui, which builds it beside ze", runner.UpgradeBinary, err)
	}
	currentDigest, err := nativeFileDigest(current)
	if err != nil {
		return err
	}
	upgradeDigest, err := nativeFileDigest(upgrade)
	if err != nil {
		return err
	}
	// Two equal binaries would make every assertion below pass on a reinstall.
	if bytes.Equal(currentDigest, upgradeDigest) {
		return fmt.Errorf("%s holds the same bytes as ze, so no upgrade can be observed", runner.UpgradeBinary)
	}

	first, err := nativeUpgradePrefix("native-upgrade-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(first.prefix) //nolint:errcheck // Only this fixture's private installation is removed.
	input := "native-admin\nnative-test-secret\n127.0.0.1\n" + args[0] + "\nnative-edge\n"
	installed := nativeInstallCommand(ctx, started, nil, input, "ze", "install", "--prefix", first.prefix, "--no-service")
	if installed.code != 0 {
		return fmt.Errorf("native install of ze failed: %s%s", installed.stdout, installed.stderr)
	}
	before := nativeInstallCommand(ctx, started, map[string]string{envConfigDir: first.configDir}, "", first.binary, "config", "cat", "native-edge.conf")
	if before.code != 0 {
		return fmt.Errorf("installed config unavailable: %s%s", before.stdout, before.stderr)
	}
	release, err := nativeInstalledRelease(ctx, started, first)
	if err != nil {
		return err
	}
	if nativeReportsUpgrade(release) {
		return fmt.Errorf("ze already reports %s's release before the upgrade: %q", runner.UpgradeBinary, release)
	}

	// Empty stdin: an upgrade that bootstrapped again would find no
	// credentials to read and fail, or replace the store.
	upgraded := nativeInstallCommand(ctx, started, nil, "", upgrade, "install", "--prefix", first.prefix, "--no-service")
	if upgraded.code != 0 {
		return fmt.Errorf("upgrade to %s failed: %s%s", runner.UpgradeBinary, upgraded.stdout, upgraded.stderr)
	}
	if err := nativeInstalledIs(ctx, started, first, upgradeDigest); err != nil {
		return fmt.Errorf("after the upgrade: %w", err)
	}
	after := nativeInstallCommand(ctx, started, map[string]string{envConfigDir: first.configDir}, "", first.binary, "config", "cat", "native-edge.conf")
	if after.code != 0 {
		return fmt.Errorf("upgraded config unavailable: %s%s", after.stdout, after.stderr)
	}
	if after.stdout != before.stdout {
		return fmt.Errorf("the upgrade changed the configuration:\nbefore:\n%s\nafter:\n%s", before.stdout, after.stdout)
	}
	// The daemon is now the upgraded binary, and the login is the one ze's
	// bootstrap stored.
	if err := nativeInstalledLogin(ctx, started, first.binary, nativeUpgradeRemote(first, args[0], "native-test-secret")); err != nil {
		return fmt.Errorf("upgraded daemon refused the retained login: %w", err)
	}

	second, err := nativeUpgradePrefix("native-fresh-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(second.prefix) //nolint:errcheck // Only this fixture's private installation is removed.
	fresh := "native-admin\nnative-fresh-secret\n127.0.0.1\n" + args[0] + "\nnative-edge\n"
	bootstrapped := nativeInstallCommand(ctx, started, nil, fresh, upgrade, "install", "--prefix", second.prefix, "--no-service")
	if bootstrapped.code != 0 {
		return fmt.Errorf("fresh install of %s failed: %s%s", runner.UpgradeBinary, bootstrapped.stdout, bootstrapped.stderr)
	}
	if err := nativeInstalledIs(ctx, started, second, upgradeDigest); err != nil {
		return fmt.Errorf("after the fresh install: %w", err)
	}
	if err := nativeInstalledLogin(ctx, started, second.binary, nativeUpgradeRemote(second, args[0], "native-fresh-secret")); err != nil {
		return fmt.Errorf("freshly bootstrapped %s refused its login: %w", runner.UpgradeBinary, err)
	}
	fmt.Fprintln(os.Stderr, "OK: upgrade to "+runner.UpgradeBinary+" installed its bytes and release "+runner.UpgradeRelease+", kept config and login, and a fresh "+runner.UpgradeBinary+" install accepts its own login")
	return nil
}

// nativeUpgradePrefix creates one private prefix under the test directory,
// with the runtime directory the daemon's socket goes in.
func nativeUpgradePrefix(pattern string) (nativeUpgradeRun, error) {
	dir, err := os.MkdirTemp(".", pattern)
	if err != nil {
		return nativeUpgradeRun{}, err
	}
	prefix, err := filepath.Abs(dir)
	if err != nil {
		return nativeUpgradeRun{}, err
	}
	run := nativeUpgradeRun{
		prefix:     prefix,
		binary:     filepath.Join(prefix, "bin", "ze"),
		configDir:  filepath.Join(prefix, "etc", "ze"),
		runtimeDir: filepath.Join(prefix, "run"),
	}
	if err := os.Mkdir(run.runtimeDir, 0o700); err != nil {
		return nativeUpgradeRun{}, err
	}
	return run, nil
}

// nativeUpgradeRemote is the environment a CLI child logs in to run's daemon
// with: the password the bootstrap was given, over the runner's SSH port.
func nativeUpgradeRemote(run nativeUpgradeRun, port, password string) map[string]string {
	return map[string]string{envConfigDir: run.configDir, envSSHHost: addrLoopback, envSSHPort: port, envSSHPassword: password, envXDGRuntimeDir: run.runtimeDir}
}

// nativeInstalledIs fails unless run's installed binary holds the bytes whose
// digest is want and reports ze-upgrade's release.
func nativeInstalledIs(ctx context.Context, started time.Time, run nativeUpgradeRun, want []byte) error {
	got, err := nativeFileDigest(run.binary)
	if err != nil {
		return err
	}
	if !bytes.Equal(got, want) {
		return fmt.Errorf("%s does not hold %s's bytes", run.binary, runner.UpgradeBinary)
	}
	release, err := nativeInstalledRelease(ctx, started, run)
	if err != nil {
		return err
	}
	if !nativeReportsUpgrade(release) {
		return fmt.Errorf("%s reports %q, not release %s", run.binary, release, runner.UpgradeRelease)
	}
	return nil
}

// nativeInstalledRelease answers what run's installed binary prints for
// `show version`, which answers without a daemon.
func nativeInstalledRelease(ctx context.Context, started time.Time, run nativeUpgradeRun) (string, error) {
	shown := nativeInstallCommand(ctx, started, map[string]string{envConfigDir: run.configDir}, "", run.binary, "show", "version")
	if shown.code != 0 {
		return "", fmt.Errorf("%s show version failed: %s%s", run.binary, shown.stdout, shown.stderr)
	}
	return strings.TrimSpace(shown.stdout), nil
}

// nativeReportsUpgrade reports whether a `show version` line names
// ze-upgrade's release ("ze <release> (built <date>)", version.Short).
func nativeReportsUpgrade(line string) bool {
	return strings.HasPrefix(line, "ze "+runner.UpgradeRelease+" ")
}

// nativeFileDigest answers the SHA-256 of the file at path, following a
// symlink, as `ze install` does when it resolves its own executable.
func nativeFileDigest(path string) ([]byte, error) {
	file, err := os.Open(path) //nolint:gosec // The fixture chooses the binaries it compares.
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck // Read-only file; the digest is complete once Copy returns.
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, fmt.Errorf("digest %s: %w", path, err)
	}
	return hash.Sum(nil), nil
}
