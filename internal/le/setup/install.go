// Design: docs/architecture/core-design.md -- putting a tool on the machine
//
// install.go contains one route per tool and vendors the Go dependencies at
// the end of an install run.
//
// USE ONE Installer PER RUN instead of package-level state. The replaced shell
// version kept a global apt-updated flag. That flag describes one run, but all
// runs shared it. A test that installed a tool left the flag set for the next
// test.

package setup

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// detectPackageManager returns the package manager that this host uses to
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
func (s *Setup) detectPackageManager() PackageManager {
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
	installSubcommand = installVerb
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
// Go-installed tools take precedence over the system package manager.
func (i *Installer) Install(tool Tool) bool {
	if tool.GoInstall != "" {
		return i.goInstall(tool, tool.GoInstall)
	}

	name := tool.packageFor(i.Manager)
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
	i.report.Note(tb.Str("  FAIL ").Str(tool.Name).Str(": ").Str(result.complaint()).String())
	return false
}

// goInstall builds and installs a Go tool. The pinned gopls package is built
// from the root vendor tree; versioned standalone tools retain their explicit
// module target.
func (i *Installer) goInstall(tool Tool, target string) bool {
	if !i.setup.Shell.Present(toolGo) {
		return i.skip(tool, "go not available yet")
	}
	argv := []string{toolGo, installSubcommand, target}
	dir := i.setup.Root
	if target == goplsTarget {
		argv = []string{toolGo, installSubcommand, "-mod=vendor", target}
	}
	var tb textbuf.Buffer
	i.report.Note(tb.Str("  CGO_ENABLED=0 ").Join(argv, " ").String())
	result := i.setup.Shell.Run(Cmd{
		Argv: argv,
		Dir:  dir,
		Env:  append(os.Environ(), "CGO_ENABLED=0"),
	})
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

	ok, detail := i.setup.Shell.runPrivileged(i.report,
		[]string{envBin, "DEBIAN_FRONTEND=noninteractive", aptBin, installSubcommand, "-y", name}, nil, "")
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
	if ok, detail := i.setup.Shell.runPrivileged(i.report, []string{aptBin, updateSubcommand}, nil, ""); !ok {
		var tb textbuf.Buffer
		i.report.Note(tb.Str("  WARN apt-get update: ").Str(detail).String())
	}
}

var vendorPatchPaths = [...]string{
	"internal/le/vendorpatch/patches/bubbles-textinput-cursor.patch",
	"internal/le/vendorpatch/patches/netlink-xfrm-fixes.patch",
}

// vendorGoDeps synchronizes vendor/ with go.mod and populates the offline
// gokrazy module cache. A failed command makes setup fail.
func (s *Setup) vendorGoDeps(report *Report) bool {
	if !s.Shell.Present(toolGo) {
		report.Note("  SKIP vendoring: go not available")
		return false
	}
	report.Note("  go mod tidy && go mod vendor")

	for _, argv := range [][]string{{toolGo, goModSubcommand, "tidy"}, {toolGo, goModSubcommand, "vendor"}} {
		result := s.Shell.Run(Cmd{Argv: argv, Dir: s.Root})
		if !result.OK() {
			var tb textbuf.Buffer
			report.Note(tb.Str("  FAIL ").Join(argv, " ").Str(": ").Str(result.complaint()).String())
			return false
		}
	}
	if !s.applyVendorPatches(report) {
		return false
	}
	return s.downloadApplianceDeps(report)
}

func (s *Setup) applyVendorPatches(report *Report) bool {
	for _, relative := range vendorPatchPaths {
		patch := filepath.Join(s.Root, filepath.FromSlash(relative))
		if _, err := os.Stat(patch); os.IsNotExist(err) {
			continue
		} else if err != nil {
			report.Note("  FAIL read vendor patch " + relative + ": " + err.Error())
			return false
		}
		if !s.Shell.Present(toolGit) {
			report.Note("  FAIL apply vendor patch " + relative + ": git not available")
			return false
		}
		reverse := s.Shell.Run(Cmd{Argv: []string{toolGit, gitApply, "--reverse", "--check", relative}, Dir: s.Root})
		if reverse.OK() {
			report.Note("  vendor patch already applied: " + relative)
			continue
		}
		check := s.Shell.Run(Cmd{Argv: []string{toolGit, gitApply, "--check", relative}, Dir: s.Root})
		if !check.OK() {
			report.Note("  FAIL check vendor patch " + relative + ": " + check.complaint())
			return false
		}
		apply := s.Shell.Run(Cmd{Argv: []string{toolGit, gitApply, relative}, Dir: s.Root})
		if !apply.OK() {
			report.Note("  FAIL apply vendor patch " + relative + ": " + apply.complaint())
			return false
		}
		report.Note("  applied vendor patch: " + relative)
	}
	return true
}

func (s *Setup) downloadApplianceDeps(report *Report) bool {
	root := filepath.Join(s.Root, "gokrazy", "ze", "builddir")
	var modules []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && entry.Name() == "go.mod" {
			modules = append(modules, filepath.Dir(path))
		}
		return nil
	})
	if os.IsNotExist(err) {
		return true
	}
	if err != nil {
		report.Note("  FAIL discover appliance modules: " + err.Error())
		return false
	}
	cache := filepath.Join(s.Root, "gokrazy", "modcache")
	environment := applianceDownloadEnvironment(cache)
	for _, module := range modules {
		argv := []string{toolGo, goModSubcommand, "download", "all"}
		report.Note("  " + module + ": go mod download all")
		result := s.Shell.Run(Cmd{Argv: argv, Dir: module, Env: environment})
		if !result.OK() {
			report.Note("  FAIL " + module + ": " + result.complaint())
			return false
		}
	}
	return true
}

func applianceDownloadEnvironment(cache string) []string {
	environment := make([]string, 0, len(os.Environ())+2)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "GOMODCACHE=") || strings.HasPrefix(entry, "GOFLAGS=") {
			continue
		}
		environment = append(environment, entry)
	}
	return append(environment, "GOMODCACHE="+cache, "GOFLAGS=-modcacherw")
}
