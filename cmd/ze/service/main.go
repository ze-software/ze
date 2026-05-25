// Design: (none — offline CLI command for systemd service management)

package service

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/suggest"
)

const (
	exitOK    = 0
	exitError = 1
)

type serviceOps interface {
	isLinux() bool
	isRoot() bool
	executable() (string, error)
	evalSymlinks(string) (string, error)
	lookPath(string) (string, error)
	stat(string) (fs.FileInfo, error)
	writeFile(string, []byte, fs.FileMode) error
	remove(string) error
	run(string, ...string) error
	output(string, ...string) ([]byte, error)
	chown(string, string, string) error
	activeConfigs(string) ([][]byte, error)
}

type serviceRuntime struct {
	stdout   io.Writer
	stderr   io.Writer
	ops      serviceOps
	unitPath string
}

type realServiceOps struct {
	stdout io.Writer
	stderr io.Writer
}

type serviceCommand struct {
	name string
	desc string
}

var serviceCommands = []serviceCommand{
	{name: "install", desc: "Install and enable ze as a systemd service"},
	{name: "uninstall", desc: "Stop, disable, and remove the systemd service"},
	{name: "status", desc: "Show systemctl status for ze.service"},
	{name: "help", desc: "Show this help"},
}

// Run executes the service subcommand with the given arguments.
func Run(args []string) int {
	rt := &serviceRuntime{
		stdout:   os.Stdout,
		stderr:   os.Stderr,
		ops:      realServiceOps{stdout: os.Stdout, stderr: os.Stderr},
		unitPath: defaultUnitPath,
	}
	return rt.run(args)
}

func (rt *serviceRuntime) run(args []string) int {
	if len(args) < 1 {
		rt.usage()
		return exitError
	}

	subcmd := args[0]
	subArgs := args[1:]
	if subcmd == "help" || subcmd == "-h" || subcmd == "--help" {
		rt.usage()
		return exitOK
	}

	switch subcmd {
	case "install":
		return rt.cmdInstall(subArgs)
	case "uninstall":
		return rt.cmdUninstall(subArgs)
	case "status":
		return rt.cmdStatus(subArgs)
	default:
		writef(rt.stderr, "unknown service subcommand: %s\n", subcmd)
		if s := suggest.Command(subcmd, serviceCommandNames()); s != "" {
			writef(rt.stderr, "hint: did you mean '%s'?\n", s)
		}
		rt.usage()
		return exitError
	}
}

func (rt *serviceRuntime) usage() {
	entries := make([]helpfmt.HelpEntry, 0, len(serviceCommands))
	for _, cmd := range serviceCommands {
		entries = append(entries, helpfmt.HelpEntry{Name: cmd.name, Desc: cmd.desc})
	}
	p := helpfmt.Page{
		Command: "ze service",
		Summary: "Manage ze as a systemd service on standard Linux hosts",
		Usage:   []string{"ze service <command> [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Commands", Entries: entries},
		},
		Examples: []string{
			"sudo ze service install --start",
			"ze service install --dry-run --config /etc/ze",
			"ze service status",
			"sudo ze service uninstall",
		},
	}
	p.WriteTo(rt.stderr, false)
}

func serviceCommandNames() []string {
	names := make([]string, 0, len(serviceCommands))
	for _, cmd := range serviceCommands {
		names = append(names, cmd.name)
	}
	return names
}

func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func writeln(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}

func (rt *serviceRuntime) requireSystemd() bool {
	if !rt.ops.isLinux() {
		writeln(rt.stderr, "error: ze service requires Linux with systemd")
		return false
	}
	if _, err := rt.ops.lookPath("systemctl"); err != nil {
		writeln(rt.stderr, "error: ze service requires systemd: systemctl not found")
		return false
	}
	return true
}

func (rt *serviceRuntime) requireRoot() bool {
	if rt.ops.isRoot() {
		return true
	}
	writeln(rt.stderr, "error: must be run as root")
	return false
}

func (rt *serviceRuntime) resolveBinaryPath() (string, error) {
	exe, err := rt.ops.executable()
	if err != nil {
		return "", fmt.Errorf("cannot find own binary: %w", err)
	}
	resolved, err := rt.ops.evalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("resolving binary path: %w", err)
	}
	if !filepath.IsAbs(resolved) {
		return "", errors.New("resolved binary path is not absolute")
	}
	if containsUnitPathUnsafeChar(resolved) {
		return "", errors.New("resolved binary path contains invalid characters (whitespace or control)")
	}
	return resolved, nil
}

func containsUnitPathUnsafeChar(path string) bool {
	for _, r := range path {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func (r realServiceOps) isLinux() bool { return runtime.GOOS == "linux" }

func (r realServiceOps) isRoot() bool { return os.Getuid() == 0 }

func (r realServiceOps) executable() (string, error) { return os.Executable() }

func (r realServiceOps) evalSymlinks(path string) (string, error) { return filepath.EvalSymlinks(path) }

func (r realServiceOps) lookPath(name string) (string, error) { return exec.LookPath(name) }

func (r realServiceOps) stat(path string) (fs.FileInfo, error) { return os.Stat(path) }

func (r realServiceOps) writeFile(path string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(path, data, perm) // #nosec G306 - systemd unit files are world-readable by convention
}

func (r realServiceOps) remove(path string) error { return os.Remove(path) }

func (r realServiceOps) run(name string, args ...string) error {
	cmd := exec.CommandContext(context.Background(), name, args...) // #nosec G204 - command names and args are fixed by service handlers
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func (r realServiceOps) output(name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(context.Background(), name, args...) // #nosec G204 - command names and args are fixed by service handlers
	return cmd.Output()
}

func (r realServiceOps) chown(path, user, group string) error {
	return r.run("chown", user+":"+group, path)
}

func newFlagSet(name string, stderr io.Writer, usage func()) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = usage
	return fs
}
