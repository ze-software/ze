// Design: docs/architecture/cli/plugin-modes.md — ze systemd: systemd service management

package systemd

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
	"unicode"

	"github.com/ze-software/ze/internal/core/textbuf"
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

func defaultRuntime() *serviceRuntime {
	return &serviceRuntime{
		stdout:   os.Stdout,
		stderr:   os.Stderr,
		ops:      realServiceOps{stdout: os.Stdout, stderr: os.Stderr},
		unitPath: defaultUnitPath,
	}
}

func RunInstall(args []string) int   { return defaultRuntime().cmdInstall(args) }
func RunUninstall(args []string) int { return defaultRuntime().cmdUninstall(args) }

func writef(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...) //nolint:errcheck // output
}

func writeln(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...) //nolint:errcheck // output
}

func (rt *serviceRuntime) requireSystemd() bool {
	if !rt.ops.isLinux() {
		writeln(rt.stderr, "error: ze systemd requires Linux with systemd")
		return false
	}
	if _, err := rt.ops.lookPath("systemctl"); err != nil {
		writeln(rt.stderr, "error: ze systemd requires systemd: systemctl not found")
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
		return fmt.Errorf("%s %s: %w", name, textbuf.Join(args, " "), err)
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
