// Design: docs/architecture/core-design.md -- putting a tool on the machine
//
// install.go is the port of scripts/le/devtools/install.py. It contains one
// route per tool and vendors the Go dependencies at the end of an install run.
//
// USE ONE Installer PER RUN instead of package-level state. The replaced shell
// version kept a global apt-updated flag. That flag describes one run, but all
// runs shared it. A test that installed a tool left the flag set for the next
// test.

package devsetup

import (
	"os"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// DetectPackageManager returns the package manager that this host uses to
// install system packages.
//
// SELECT IT BY PLATFORM, not by the contents of PATH. macOS uses Homebrew, and
// Linux uses apt. An earlier version checked only PATH and selected brew first
// on every platform. Thus, a Linux box with Linuxbrew installed system packages
// through it instead of apt. This silently changed which package manager owned
// the machine.
//
// ManagerNone means that neither route exists. This is not a failure of this
// tool. The platform is not supported for installation, so the caller gives
// the manual list.
func (s *Setup) DetectPackageManager() PackageManager {
	switch s.goos() {
	case osDarwin:
		if s.Shell.Present(brewBin) {
			return ManagerBrew
		}
	case osLinux:
		if s.Shell.Present(aptBin) {
			return ManagerApt
		}
	}
	return ManagerNone
}

// The two system package managers, and the subcommand every install route
// takes.
const (
	brewBin           = "brew"
	aptBin            = "apt-get"
	installSubcommand = "install"
)

// Installer puts tools on the machine and records what it did during the
// current run.
//
// Manager is the system package manager. The `go install` and pipx routes do
// not use it and operate on either platform.
type Installer struct {
	Manager PackageManager

	setup      *Setup
	report     *Report
	aptUpdated bool
}

// NewInstaller builds the installer one run uses.
func (s *Setup) NewInstaller(manager PackageManager, report *Report) *Installer {
	return &Installer{Manager: manager, setup: s, report: report}
}

// Install puts tool on the machine by whichever route it declares, and answers
// whether that worked.
//
// Order matters: `go install` and pipx come first because they work the same on
// both platforms, so a tool declaring one gets the same version everywhere. The
// system package manager is the fallback.
func (i *Installer) Install(tool Tool) bool {
	if tool.GoInstall != "" {
		return i.goInstall(tool, tool.GoInstall)
	}
	if tool.PipxInstall != "" {
		return i.pipxInstall(tool, tool.PipxInstall)
	}

	name := tool.PackageFor(i.Manager)
	if name == "" {
		if tool.Note != "" {
			var tb textbuf.Buffer
			i.report.Note(tb.Str("  SKIP ").Str(tool.Name).Str(": ").Str(tool.Note).String())
		}
		return false
	}

	if i.Manager == ManagerBrew {
		return i.brewInstall(tool, name)
	}
	return i.aptInstall(tool, name)
}

// skip records that a route exists on paper and not on this machine yet.
func (i *Installer) skip(tool Tool, why string) bool {
	var tb textbuf.Buffer
	i.report.Note(tb.Str("  SKIP ").Str(tool.Name).Str(": ").Str(why).String())
	return false
}

// fail records a command that ran and did not work.
func (i *Installer) fail(tool Tool, result Result) bool {
	var tb textbuf.Buffer
	i.report.Note(tb.Str("  FAIL ").Str(tool.Name).Str(": ").Str(result.Complaint()).String())
	return false
}

// goInstall builds and installs a Go tool at its pinned version.
func (i *Installer) goInstall(tool Tool, target string) bool {
	if !i.setup.Shell.Present(toolGo) {
		return i.skip(tool, "go not available yet")
	}
	var tb textbuf.Buffer
	i.report.Note(tb.Str("  CGO_ENABLED=0 go install ").Str(target).String())

	result := i.setup.Shell.Run(Cmd{
		Argv: []string{toolGo, installSubcommand, target},
		Env:  append(os.Environ(), "CGO_ENABLED=0"),
	})
	if !result.OK() {
		return i.fail(tool, result)
	}
	return true
}

// pipxInstall installs a Python tool into its own environment.
func (i *Installer) pipxInstall(tool Tool, target string) bool {
	if !i.setup.Shell.Present(toolPipx) {
		return i.skip(tool, "pipx not available yet")
	}
	var tb textbuf.Buffer
	i.report.Note(tb.Str("  pipx install ").Str(target).String())

	result := i.setup.Shell.Run(Cmd{Argv: []string{toolPipx, installSubcommand, "--force", target}})
	if !result.OK() {
		return i.fail(tool, result)
	}
	return true
}

// brewInstall installs one Homebrew formula.
func (i *Installer) brewInstall(tool Tool, name string) bool {
	var tb textbuf.Buffer
	i.report.Note(tb.Str("  brew install ").Str(name).String())

	result := i.setup.Shell.Run(Cmd{Argv: []string{brewBin, installSubcommand, name}})
	if !result.OK() {
		return i.fail(tool, result)
	}
	return true
}

// aptInstall installs one Debian package, or names the command when root is out
// of reach.
//
// DEBIAN_FRONTEND=noninteractive rides in the argv rather than in the
// environment because sudo resets it (env_reset is the sudoers default), so an
// exported value does not reach apt-get. Without it a package carrying a
// debconf prompt stops the run dead.
func (i *Installer) aptInstall(tool Tool, name string) bool {
	// The recorded line must be usable on the box that printed it. Some root
	// containers do not contain sudo. If the line names sudo, the reader receives
	// a command that cannot run in those containers.
	mode := i.setup.Shell.Privilege()
	var tb textbuf.Buffer
	manual := tb.Str(mode.Prefix()).Str("apt-get install -y ").Str(name).String()

	if mode == PrivilegeNone {
		var line textbuf.Buffer
		i.report.Note(line.Str("  Run: ").Str(manual).String())
		return false
	}

	i.aptUpdate()

	ok, detail := i.setup.Shell.RunPrivileged(i.report,
		[]string{"env", "DEBIAN_FRONTEND=noninteractive", aptBin, installSubcommand, "-y", name}, nil, "")
	if !ok {
		var why textbuf.Buffer
		i.report.Note(why.Str("  FAIL ").Str(tool.Name).Str(": ").Str(detail).String())
		var line textbuf.Buffer
		i.report.Note(line.Str("  Run: ").Str(manual).String())
		return false
	}
	return true
}

// aptUpdate refreshes the package lists once per run before the first install.
//
// Container images can contain no package lists. An install with an empty list
// fails with "Unable to locate package <x>". This message suggests an incorrect
// package name instead of a missing index. An update for each tool would use
// approximately 20 seconds of network time, although the index does not change
// during one run.
//
// Make one attempt, whether it succeeds or fails. A stale index can still
// install most packages. Repeating the same failed update before each tool only
// adds more failed operations.
func (i *Installer) aptUpdate() {
	if i.aptUpdated {
		return
	}
	i.aptUpdated = true
	if ok, detail := i.setup.Shell.RunPrivileged(i.report, []string{aptBin, "update"}, nil, ""); !ok {
		var tb textbuf.Buffer
		i.report.Note(tb.Str("  WARN apt-get update: ").Str(detail).String())
	}
}

// VendorGoDeps synchronizes vendor/ with go.mod and returns whether the
// operation succeeded.
//
// THE CALLER USES THE RESULT. This corrects a defect that the script still
// contains. le.application.setup.action calls vendor_go_deps() and discards its
// result. Therefore, a failed `go mod tidy` or `go mod vendor` still ends the
// run with "Setup complete" and exit 0. Vendoring makes the tree build. If the
// operation fails but the run reports success, it has the defect described in
// the header of console.py
// (plan/journal/validated-value-discarded-by-its-caller.md, 2026-08-26).
func (s *Setup) VendorGoDeps(report *Report) bool {
	if !s.Shell.Present(toolGo) {
		report.Note("  SKIP vendoring: go not available")
		return false
	}
	report.Note("  go mod tidy && go mod vendor")

	for _, argv := range [][]string{{toolGo, "mod", "tidy"}, {toolGo, "mod", "vendor"}} {
		result := s.Shell.Run(Cmd{Argv: argv, Dir: s.Root})
		if !result.OK() {
			var tb textbuf.Buffer
			report.Note(tb.Str("  FAIL ").Join(argv, " ").Str(": ").Str(result.Complaint()).String())
			return false
		}
	}
	return true
}
