// Design: docs/architecture/cli/plugin-modes.md — ze systemd install: unit file + account setup

package systemd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/paths"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

var daemonUserPattern = regexp.MustCompile(`(?s)\bdaemon\s*\{.*?\buser\b`)

func (rt *serviceRuntime) cmdInstall(args []string) int {
	var configDirFlag string
	var start bool
	var force bool
	var dryRun bool

	fs := newFlagSet("systemd install", rt.stderr, func() { installUsageTo(rt.stderr) })
	fs.StringVar(&configDirFlag, "config", "", "Override config directory in the unit file")
	fs.BoolVar(&start, "start", false, "Start ze.service after install")
	fs.BoolVar(&force, "force", false, "Overwrite an existing ze.service unit file")
	fs.BoolVar(&dryRun, "dry-run", false, "Print unit file to stdout without making changes")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitError
	}
	if fs.NArg() != 0 {
		writeln(rt.stderr, "error: ze systemd install takes no positional arguments")
		fs.Usage()
		return exitError
	}

	binaryPath, err := rt.resolveBinaryPath()
	if err != nil {
		writef(rt.stderr, "error: %v\n", err)
		return exitError
	}
	configDir, err := resolveConfigDir(binaryPath, configDirFlag)
	if err != nil {
		writef(rt.stderr, "error: %v\n", err)
		return exitError
	}

	unit := buildUnitFile(unitSpec{BinaryPath: binaryPath, ConfigDir: configDir})
	if dryRun {
		_, err := io.WriteString(rt.stdout, unit)
		if err != nil {
			writef(rt.stderr, "error: writing dry-run output: %v\n", err)
			return exitError
		}
		return exitOK
	}

	if !rt.requireSystemd() || !rt.requireRoot() {
		return exitError
	}
	if !force {
		if _, err := rt.ops.stat(rt.unitPath); err == nil {
			writeln(rt.stderr, "error: service already installed (use --force to overwrite)")
			return exitError
		} else if !errors.Is(err, os.ErrNotExist) {
			writef(rt.stderr, "error: checking %s: %v\n", rt.unitPath, err)
			return exitError
		}
	}
	if err := rt.verifyConfigReady(configDir); err != nil {
		writef(rt.stderr, "error: %v\n", err)
		return exitError
	}
	rt.warnDaemonUserConfig(configDir)
	if err := rt.ensureServiceAccount(); err != nil {
		writef(rt.stderr, "error: %v\n", err)
		return exitError
	}
	if err := rt.chownConfig(configDir); err != nil {
		writef(rt.stderr, "error: %v\n", err)
		return exitError
	}
	if err := rt.ops.writeFile(rt.unitPath, []byte(unit), 0o644); err != nil {
		writef(rt.stderr, "error: writing %s: %v\n", rt.unitPath, err)
		return exitError
	}
	if err := rt.ops.run("systemctl", "daemon-reload"); err != nil {
		writef(rt.stderr, "error: %v\n", err)
		return exitError
	}
	if err := rt.ops.run("systemctl", "enable", serviceName); err != nil {
		writef(rt.stderr, "error: %v\n", err)
		return exitError
	}
	if start {
		if err := rt.ops.run("systemctl", "start", serviceName); err != nil {
			writef(rt.stderr, "error: %v\n", err)
			return exitError
		}
	}

	writeln(rt.stderr, "service installed and enabled")
	if start {
		writeln(rt.stderr, "service started")
	}
	printSocketHint(rt.stderr)
	return exitOK
}

func resolveConfigDir(binaryPath, override string) (string, error) {
	if override != "" {
		if !filepath.IsAbs(override) {
			return "", errors.New("--config must be an absolute path")
		}
		clean := filepath.Clean(override)
		if containsUnitPathUnsafeChar(clean) {
			return "", errors.New("--config path contains invalid characters (whitespace or control)")
		}
		return clean, nil
	}
	configDir := paths.ConfigDirFromBinary(binaryPath)
	if configDir == "" {
		return "", errors.New("cannot resolve config directory from binary path; pass --config")
	}
	if containsUnitPathUnsafeChar(configDir) {
		return "", errors.New("config path contains invalid characters (whitespace or control)")
	}
	return configDir, nil
}

func (rt *serviceRuntime) verifyConfigReady(configDir string) error {
	var tb textbuf.Buffer
	if _, err := rt.ops.stat(configDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New(tb.Str("ze init has not been run: config directory not found: ").Str(configDir).String())
		}
		return fmt.Errorf("checking config directory %s: %w", configDir, err)
	}
	dbPath := filepath.Join(configDir, "database.zefs")
	if _, err := rt.ops.stat(dbPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New(tb.Reset().Str("ze init has not been run: ").Str(dbPath).Str(" not found").String())
		}
		return fmt.Errorf("checking %s: %w", dbPath, err)
	}
	return nil
}

func (rt *serviceRuntime) ensureServiceAccount() error {
	if out, err := rt.ops.output("getent", "group", serviceGroup); err == nil {
		writef(rt.stderr, "group %s exists%s\n", serviceGroup, groupSuffix(out))
	} else if err := rt.createGroup(); err != nil {
		return err
	}

	if out, err := rt.ops.output("getent", "passwd", serviceUser); err == nil {
		writef(rt.stderr, "user %s exists%s\n", serviceUser, userSuffix(out))
		return nil
	}
	return rt.createUser()
}

func (rt *serviceRuntime) createGroup() error {
	if _, err := rt.ops.lookPath("groupadd"); err == nil {
		return rt.ops.run("groupadd", "--system", serviceGroup)
	}
	if _, err := rt.ops.lookPath("addgroup"); err == nil {
		return rt.ops.run("addgroup", "-S", serviceGroup)
	}
	return errors.New("cannot create ze group: groupadd/addgroup not found")
}

func (rt *serviceRuntime) createUser() error {
	shell, err := rt.nologinShell()
	if err != nil {
		return err
	}
	if _, err := rt.ops.lookPath("useradd"); err == nil {
		return rt.ops.run("useradd", "--system", "--no-create-home", "--gid", serviceGroup, "--home-dir", "/nonexistent", "--shell", shell, serviceUser)
	}
	if _, err := rt.ops.lookPath("adduser"); err == nil {
		return rt.ops.run("adduser", "-S", "-D", "-H", "-G", serviceGroup, "-s", shell, serviceUser)
	}
	return errors.New("cannot create ze user: useradd/adduser not found")
}

func (rt *serviceRuntime) deleteUser() error {
	if _, err := rt.ops.lookPath("userdel"); err == nil {
		return rt.ops.run("userdel", serviceUser)
	}
	if _, err := rt.ops.lookPath("deluser"); err == nil {
		return rt.ops.run("deluser", serviceUser)
	}
	return errors.New("cannot remove ze user: userdel/deluser not found")
}

func (rt *serviceRuntime) deleteGroup() error {
	if _, err := rt.ops.lookPath("groupdel"); err == nil {
		return rt.ops.run("groupdel", serviceGroup)
	}
	if _, err := rt.ops.lookPath("delgroup"); err == nil {
		return rt.ops.run("delgroup", serviceGroup)
	}
	return errors.New("cannot remove ze group: groupdel/delgroup not found")
}

func (rt *serviceRuntime) nologinShell() (string, error) {
	for _, path := range []string{"/usr/sbin/nologin", "/sbin/nologin"} {
		if _, err := rt.ops.stat(path); err == nil {
			return path, nil
		}
	}
	return "", errors.New("cannot create ze user: nologin shell not found")
}

func (rt *serviceRuntime) chownConfig(configDir string) error {
	if err := rt.ops.chown(configDir, serviceUser, serviceGroup); err != nil {
		return fmt.Errorf("chown %s: %w", configDir, err)
	}
	dbPath := filepath.Join(configDir, "database.zefs")
	if err := rt.ops.chown(dbPath, serviceUser, serviceGroup); err != nil {
		return fmt.Errorf("chown %s: %w", dbPath, err)
	}
	return nil
}

func (rt *serviceRuntime) warnDaemonUserConfig(configDir string) {
	configs, err := rt.ops.activeConfigs(configDir)
	if err != nil {
		writef(rt.stderr, "warning: cannot inspect config for daemon user setting: %v\n", err)
		return
	}
	if slices.ContainsFunc(configs, daemonUserPattern.Match) {
		writeln(rt.stderr, "warning: config contains daemon { user }; remove daemon user when running under systemd User=ze")
	}
}

func (r realServiceOps) activeConfigs(configDir string) ([][]byte, error) {
	store, err := zefs.Open(filepath.Join(configDir, "database.zefs"))
	if err != nil {
		return nil, err
	}
	defer store.Close() //nolint:errcheck // read-only inspection

	entries, err := store.ReadDir(zefs.KeyFileActive.Dir())
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	configs := make([][]byte, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		var tb textbuf.Buffer
		data, err := store.ReadFile(tb.Str(zefs.KeyFileActive.Dir()).Byte('/').Str(entry.Name()).String())
		if err != nil {
			return nil, err
		}
		configs = append(configs, data)
	}
	return configs, nil
}

func groupSuffix(out []byte) string {
	parts := strings.Split(strings.TrimSpace(string(out)), ":")
	if len(parts) > 2 && parts[2] != "" {
		var tb textbuf.Buffer
		return tb.Str(" (gid ").Str(parts[2]).Byte(')').String()
	}
	return ""
}

func userSuffix(out []byte) string {
	parts := strings.Split(strings.TrimSpace(string(out)), ":")
	if len(parts) > 3 && parts[2] != "" && parts[3] != "" {
		var tb textbuf.Buffer
		return tb.Str(" (uid ").Str(parts[2]).Str(", gid ").Str(parts[3]).Byte(')').String()
	}
	return ""
}

func printSocketHint(w io.Writer) {
	writeln(w, "")
	writeln(w, "socket access: ze runs with XDG_RUNTIME_DIR=/run/ze, so the daemon socket is /run/ze/ze.socket")
	writeln(w, "for operator CLI access, set daemon socket \"/run/ze/ze.socket\" in config or export XDG_RUNTIME_DIR=/run/ze")
}

func installUsageTo(w io.Writer) {
	p := helpfmt.Page{
		Command: "ze systemd install",
		Summary: "Install ze as a systemd service",
		Usage:   []string{"ze systemd install [--config <dir>] [--start] [--force]", "ze systemd install --dry-run [--config <dir>]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Options", Entries: []helpfmt.HelpEntry{
				{Name: "--config <dir>", Desc: "Override config directory in the unit file"},
				{Name: "--start", Desc: "Start ze.service after install"},
				{Name: "--force", Desc: "Overwrite an existing unit file"},
				{Name: "--dry-run", Desc: "Print unit file to stdout without writing files or calling systemctl"},
			}},
			{Title: "Prerequisite", Entries: []helpfmt.HelpEntry{
				{Name: "ze init", Desc: "Must be run before install so database.zefs exists"},
			}},
		},
		Examples: []string{
			"sudo ze systemd install",
			"sudo ze systemd install --start",
			"sudo ze systemd install --config /opt/ze/etc/ze",
			"ze systemd install --dry-run",
		},
	}
	p.WriteTo(w, false)
}
