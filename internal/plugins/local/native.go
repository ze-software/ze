// Design: docs/architecture/cli/plugin-modes.md -- native installation lifecycle.

package local

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ze-software/ze/cmd/ze/install"
	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/paths"
)

type nativeRuntime struct {
	source    string
	stdin     io.Reader
	stdout    io.Writer
	stderr    io.Writer
	bootstrap func(string) int
	service   install.NativeService
	operation string
	configDir string
}

type nativeTarget struct {
	binary    string
	configDir string
}

func nativeInstallOptions() []helpfmt.HelpEntry   { return nativeOptions("install") }
func nativeUninstallOptions() []helpfmt.HelpEntry { return nativeOptions("uninstall") }

func nativeOptions(path string) []helpfmt.HelpEntry {
	flags := registry.CommandFlags(path)
	entries := make([]helpfmt.HelpEntry, 0, len(flags))
	for _, option := range flags {
		entries = append(entries, helpfmt.HelpEntry{Name: option.Name, Desc: option.Description})
	}
	return entries
}

// RunNativeInstall installs the daemon with the owners registered at startup.
func RunNativeInstall(args []string) int {
	rt, err := nativeDefaults()
	if err != nil {
		fmt.Fprintln(os.Stderr, opererr.Wrap(err, "Native installation is unavailable", "Use a Linux host and a normal Ze executable.", "ze help install"))
		return exitError
	}
	return rt.install(args)
}

// RunNativeUninstall removes the selected runtime and preserves state by default.
func RunNativeUninstall(args []string) int {
	rt, err := nativeDefaults()
	if err != nil {
		fmt.Fprintln(os.Stderr, opererr.Wrap(err, "Native removal is unavailable", "Use a Linux host and the selected Ze executable.", "ze help uninstall"))
		return exitError
	}
	return rt.uninstall(args)
}

func nativeDefaults() (*nativeRuntime, error) {
	if runtime.GOOS != "linux" {
		return nil, errors.New("native installation requires Linux; use ze install local to copy a binary on another host")
	}
	self, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locate this binary: %w; run the selected Ze executable directly", err)
	}
	source, err := filepath.EvalSymlinks(self)
	if err != nil {
		return nil, fmt.Errorf("resolve this binary: %w; run the selected Ze executable directly", err)
	}
	bootstrap, _ := install.Bootstrap()
	service, _ := install.Service()
	return &nativeRuntime{source: source, stdin: os.Stdin, stdout: os.Stdout, stderr: os.Stderr, bootstrap: bootstrap, service: service}, nil
}

func (rt *nativeRuntime) install(args []string) (code int) {
	rt.operation = "install"
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(rt.stderr)
	prefix := fs.String("prefix", "/usr/local", "Installation prefix")
	noService := fs.Bool("no-service", false, "Install for foreground operation without a service")
	dryRun := fs.Bool("dry-run", false, "Show the selected paths without changing them")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitError
	}
	if fs.NArg() != 0 {
		return rt.refuse(errors.New("unexpected installation argument; run ze install --help"))
	}
	target, err := selectNativeTarget(*prefix)
	if err != nil {
		return rt.refuse(err)
	}
	rt.configDir = target.configDir
	if *dryRun {
		fmt.Fprintln(rt.stdout, "install binary:", target.binary)
		fmt.Fprintln(rt.stdout, "preserve or initialize state:", target.configDir)
		return exitOK
	}
	existing, err := target.preflight(rt.source)
	if err != nil {
		return rt.refuse(err)
	}
	if !existing {
		if rt.bootstrap == nil {
			return rt.refuse(errors.New("this binary has no initializer; use a normal distro build"))
		}
	}
	service, useService, err := rt.selectedService(*noService)
	if err != nil {
		return rt.refuse(err)
	}
	if useService {
		if err := service.Preflight(target.binary, target.configDir); err != nil {
			return rt.refuse(err)
		}
	}
	replacement, err := captureNativeReplacement(target)
	if err != nil {
		return rt.refuse(err)
	}
	defer replacement.close() //nolint:errcheck // Read-only inode retained until success or recovery.
	var stopped install.StoppedService
	if useService {
		stopped, err = service.Stop(target.binary, target.configDir)
		if err != nil {
			return rt.refuse(err)
		}
	}
	defer func() {
		if code == exitOK {
			return
		}
		if err := rt.recoverInstall(replacement, service, stopped); err != nil {
			rt.refuse(fmt.Errorf("installation recovery failed: %w; stop ze.service and reinstall the last known working executable", err))
		}
	}()
	if err := os.MkdirAll(filepath.Dir(target.binary), 0o755); err != nil {
		return rt.refuse(err)
	}
	// The service account must traverse a new prefix's config parent after
	// ownership of the private config leaf transfers to it. MkdirAll leaves
	// existing parent permissions unchanged.
	if err := os.MkdirAll(filepath.Dir(target.configDir), 0o755); err != nil {
		return rt.refuse(err)
	}
	if err := os.MkdirAll(target.configDir, 0o700); err != nil {
		return rt.refuse(err)
	}
	if !existing {
		if code := rt.bootstrap(target.configDir); code != exitOK {
			return code
		}
	}
	guard, err := storage.AcquireMaintenance(target.configDir)
	if err != nil {
		return rt.refuse(err)
	}
	copyErr := copyFile(rt.source, target.binary)
	if copyErr == nil {
		replacement.changed = true
	}
	if err := errors.Join(copyErr, guard.Close()); err != nil {
		return rt.refuse(err)
	}
	if useService {
		if code := stopped.Install(); code != exitOK {
			return code
		}
		fmt.Fprintln(rt.stdout, "installed:", target.binary)
		fmt.Fprintln(rt.stdout, "service status: systemctl status ze.service (a previously inactive service remains inactive)")
		fmt.Fprintln(rt.stdout, "connect: sudo -u ze env ZE_CONFIG_DIR="+target.configDir+" XDG_RUNTIME_DIR=/run/ze "+target.binary+" cli")
		return exitOK
	}
	fmt.Fprintln(rt.stdout, "installed:", target.binary)
	fmt.Fprintln(rt.stdout, "no service was installed; start in the foreground:", target.binary+" start")
	return exitOK
}

func (rt *nativeRuntime) uninstall(args []string) int {
	rt.operation = "uninstall"
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	fs.SetOutput(rt.stderr)
	prefix := fs.String("prefix", "/usr/local", "Installation prefix")
	noService := fs.Bool("no-service", false, "Remove a foreground installation without a service")
	yes := fs.Bool("yes", false, "Confirm removal without a prompt")
	purge := fs.Bool("purge", false, "Also remove the selected config directory and private state")
	dryRun := fs.Bool("dry-run", false, "Show the selected paths without changing them")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitError
	}
	if fs.NArg() != 0 {
		return rt.refuse(errors.New("unexpected removal argument; run ze uninstall --help"))
	}
	target, err := selectNativeTarget(*prefix)
	if err != nil {
		return rt.refuse(err)
	}
	rt.configDir = target.configDir
	if *dryRun {
		fmt.Fprintln(rt.stdout, "remove binary:", target.binary)
		fmt.Fprintln(rt.stdout, "state:", target.configDir, "purge:", *purge)
		return exitOK
	}
	existing, err := target.preflight("")
	if err != nil {
		return rt.refuse(err)
	}
	service, useService, err := rt.selectedService(*noService)
	if err != nil {
		return rt.refuse(err)
	}
	if useService {
		if err := service.Preflight(target.binary, target.configDir); err != nil {
			return rt.refuse(err)
		}
	}
	if !*yes {
		fmt.Fprintln(rt.stderr, "remove binary:", target.binary, "(state purge:", *purge, ")")
		fmt.Fprintln(rt.stderr, "continue? [y/N]")
		scanner := bufio.NewScanner(rt.stdin)
		if !scanner.Scan() {
			return rt.refuse(errors.New("removal was not confirmed; rerun ze uninstall to select the installation"))
		}
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if answer != "y" {
			if answer != "yes" {
				return rt.refuse(errors.New("removal was not confirmed; rerun ze uninstall when ready"))
			}
		}
	}
	var stopped install.StoppedService
	if useService {
		stopped, err = service.Stop(target.binary, target.configDir)
		if err != nil {
			return rt.refuse(err)
		}
	}
	var guard io.Closer
	if existing {
		guard, err = storage.AcquireMaintenance(target.configDir)
		if err != nil {
			if stopped != nil {
				err = errors.Join(err, stopped.Restore())
			}
			return rt.refuse(err)
		}
		defer guard.Close() //nolint:errcheck // The lock lifetime ends after removal; no store write needs flushing.
	}
	if useService {
		if code := service.Uninstall(target.binary, target.configDir); code != exitOK {
			return code
		}
	}
	if err := os.Remove(target.binary); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return rt.refuse(err)
		}
	}
	if *purge {
		if err := os.RemoveAll(target.configDir); err != nil {
			return rt.refuse(err)
		}
		fmt.Fprintln(rt.stdout, "removed state:", target.configDir)
	} else {
		fmt.Fprintln(rt.stdout, "preserved state:", target.configDir)
	}
	fmt.Fprintln(rt.stdout, "removed:", target.binary)
	return exitOK
}

func (rt *nativeRuntime) selectedService(disabled bool) (install.NativeService, bool, error) {
	if disabled {
		return nil, false, nil
	}
	if rt.service == nil {
		return nil, false, nil
	}
	available, err := rt.service.Available()
	if err != nil {
		return nil, false, err
	}
	if !available {
		return nil, false, nil
	}
	return rt.service, true, nil
}

func (rt *nativeRuntime) refuse(err error) int {
	if operator, ok := errors.AsType[*opererr.Error](err); ok {
		fmt.Fprintln(rt.stderr, operator)
		return exitError
	}
	fix := "ze help " + rt.operation
	if rt.configDir != "" {
		fix = "env ZE_CONFIG_DIR=" + nativeShellWord(rt.configDir) + " ze doctor"
	}
	fmt.Fprintln(rt.stderr, opererr.Wrap(err, "Ze "+rt.operation+" did not finish", "Correct the reported path or service problem before retrying the operation.", fix))
	return exitError
}

func nativeShellWord(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

// selectNativeTarget admits only absolute paths with no interpreted components.
func selectNativeTarget(prefix string) (nativeTarget, error) {
	if !filepath.IsAbs(prefix) {
		return nativeTarget{}, errors.New("installation prefix must be absolute; use --prefix /usr/local")
	}
	if strings.ContainsAny(prefix, " \t\r\n\x00'\"\\$%;&|<>()`*?[]") {
		return nativeTarget{}, errors.New("installation prefix contains an unsafe character; choose an absolute path without whitespace or shell characters")
	}
	for _, part := range strings.Split(prefix, string(filepath.Separator)) {
		if part == ".." {
			return nativeTarget{}, errors.New("installation prefix contains ..; use its absolute path without parent components")
		}
	}
	prefix = filepath.Clean(prefix)
	if prefix == "/usr" {
		return nativeTarget{}, errors.New("/usr belongs to the distribution package manager; use --prefix /usr/local for a standalone installation")
	}
	if prefix == "/" {
		return nativeTarget{}, errors.New("the system binary directories belong to the distribution; use --prefix /usr/local")
	}
	binary := filepath.Join(prefix, "bin", "ze")
	return nativeTarget{binary: binary, configDir: paths.ConfigDirFromBinary(binary)}, nil
}

func (target nativeTarget) preflight(source string) (bool, error) {
	for _, path := range []string{target.binary, target.configDir} {
		if err := nativePath(path); err != nil {
			return false, err
		}
	}
	if info, err := os.Lstat(target.binary); err == nil {
		if !info.Mode().IsRegular() {
			return false, fmt.Errorf("selected binary %s is not a regular file; select a real installation path", target.binary)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if info, err := os.Lstat(target.configDir); err == nil {
		if !info.IsDir() {
			return false, fmt.Errorf("selected configuration path %s is not a directory; select a real installation path", target.configDir)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if err := packageOwnership(target.binary, source); err != nil {
		return false, err
	}
	for _, name := range []string{storage.BlobName, "database.import-intent"} {
		path := filepath.Join(target.configDir, name)
		if _, err := os.Lstat(path); err == nil {
			return false, fmt.Errorf("existing import artifact %s requires recovery; complete ze init --from before installing", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	path := filepath.Join(target.configDir, storage.TreeName)
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() {
		return false, fmt.Errorf("store %s is not a real directory; repair its path before installing", path)
	}
	if info.Mode().Perm() != 0o700 {
		return false, fmt.Errorf("store %s is not private; restore its mode to 0700 before installing", path)
	}
	return true, nil
}

// nativePath refuses a symlink anywhere in a selected path before mutation.
func nativePath(path string) error {
	current := string(filepath.Separator)
	parts := strings.Split(strings.TrimPrefix(path, current), current)
	for i, part := range parts {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		} // The remaining suffix is absent too.
		if err != nil {
			return fmt.Errorf("inspect %s: %w; repair access before installing", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("selected path %s is a symbolic link; select a real installation path", current)
		}
		if i < len(parts)-1 {
			if !info.IsDir() {
				return fmt.Errorf("selected ancestor %s is not a directory; select a real installation path", current)
			}
		}
	}
	return nil
}

// packageOwnership is shared by the native and explicit local operations.
func packageOwnership(binaries ...string) error {
	for _, binary := range binaries {
		if binary == "" {
			continue
		}
		dir := filepath.Dir(binary)
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			dir = resolved
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if filepath.Base(dir) != "bin" {
			continue
		}
		marker := filepath.Join(filepath.Dir(dir), "share", "ze", "install-method")
		if _, err := os.Lstat(marker); err == nil {
			return fmt.Errorf("package-managed installation marker %s exists; use its package manager to upgrade or remove Ze", marker)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect package ownership %s: %w; repair access before installing", marker, err)
		}
	}
	return nil
}
