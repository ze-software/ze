// Design: docs/architecture/core-design.md -- install and verify every tool
// that a Ze workflow needs
//
// setup.go is a Go port of scripts/le/application/setup.py. It replaced the
// `ze-dev-setup` Makefile target and its shell script. Both files were deleted.
//
// The two modes must agree. A probe-only run and an install run ask the same
// questions about the same machine. They must give the same verdict about
// missing items. The shell version permitted the modes to differ. Install mode
// reported "Setup complete" with exit 0 on a machine where check mode exited 1.
//
// Each branch printed a label and manually appended an item to a list. Here,
// every step returns one Outcome that contains both values. The final verdict
// comes from the collected outcomes and is not calculated separately.
//
// Each external dependency is a field of Setup. Tests for the script modified
// module globals. A Go test sets a field and runs the same code as the command.
// A zero-value Setup uses the real implementation for each dependency. The
// command gives this value to the dispatcher.

package devsetup

import (
	"os"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// Setup is one run: what it was asked for, where it looks, and every seam a
// test replaces.
type Setup struct {
	// Root is the checkout. The gopls probe runs in it and vendoring writes
	// into it.
	Root string
	// Check probes only: it changes nothing, and answers non-zero when a
	// required tool is missing.
	Check bool
	// Vendor runs `go mod tidy && go mod vendor` at the end of an install run.
	Vendor bool

	// Shell runs every external command. A nil Shell takes the real route.
	Shell *Shell

	// GOOS overrides the operating system the platform branches read. Empty
	// means this build's own.
	GOOS string
	// Home overrides where the harness plugin record is looked for. Empty
	// means $HOME.
	Home string
	// UsernsProc overrides the kernel knob the userns state is read from.
	UsernsProc string
	// KvmDev overrides the device the KVM state is read from.
	KvmDev string
	// Env answers an environment variable. Nil means the real environment.
	Env func(key string) string
	// User answers the invoking user's login name.
	User func() string
	// KvmGroupMember reports whether that user is in the kvm group.
	KvmGroupMember func() bool
	// Bindable reports whether one address can be bound right now.
	Bindable func(addr string) bool
	// Gopls answers the gopls probe.
	Gopls func() Result
	// Pyright answers the pyright probe.
	Pyright func() Result
}

// goos answers the operating system the platform branches read.
func (s *Setup) goos() string {
	if s.GOOS != "" {
		return s.GOOS
	}
	return hostGOOS
}

// env answers one environment variable.
func (s *Setup) env(key string) string {
	if s.Env != nil {
		return s.Env(key)
	}
	return os.Getenv(key)
}

// home answers the directory the harness plugin record lives under.
func (s *Setup) home() string {
	if s.Home != "" {
		return s.Home
	}
	if dir, err := os.UserHomeDir(); err == nil {
		return dir
	}
	return ""
}

// usernsProc answers the kernel knob the userns state is read from.
func (s *Setup) usernsProc() string {
	if s.UsernsProc != "" {
		return s.UsernsProc
	}
	return usernsProcDefault
}

// kvmDev answers the device the KVM state is read from.
func (s *Setup) kvmDev() string {
	if s.KvmDev != "" {
		return s.KvmDev
	}
	return kvmDevDefault
}

// prepare fills in the seams a zero Setup leaves nil.
func (s *Setup) prepare() {
	if s.Shell == nil {
		s.Shell = &Shell{}
	}
}

// Run probes the machine, installs what is missing, and answers the report and
// the exit code.
func (s *Setup) Run() (*Report, int) {
	s.prepare()
	report := &Report{}

	manager := s.DetectPackageManager()
	if manager == ManagerNone {
		report.Note("Unsupported platform: no brew (macOS) or apt (Linux) found.")
		s.noteManualList(report)
		return report, 1
	}

	var heading textbuf.Buffer
	report.Note(heading.Str("Ze dev setup (package manager: ").Str(string(manager)).Str(")").String())
	report.Note("")

	var installer *Installer
	if !s.Check {
		installer = s.NewInstaller(manager, report)
	}

	for _, tool := range AllTools() {
		report.Add(s.visitTool(report, tool, manager, installer))
	}

	// These checks test behavior, not only binaries. A language server on PATH
	// does not prove that the server works. Every call fails without a useful
	// result when the server does not work. Both modes run these checks. An
	// installation that leaves an unresponsive server is not a complete setup.
	report.Add(visitServer("gopls-answers", s.GoplsHealth()))
	report.Add(visitServer("pyright-answers", s.PyrightHealth()))

	// A server that runs and answers is still unreachable if the harness was
	// never told it exists. This is the check that would have caught Python
	// being unanswerable here while Go worked, with both binaries installed.
	missingPlugins := s.MissingLSPPlugins()
	for _, plugin := range LSPPlugins() {
		report.Add(s.visitLspPlugin(report, plugin, missingPlugins))
	}

	// Machine state that cannot be installed.
	if s.goos() == osLinux {
		report.Add(s.visitUserns(report))
		report.Add(s.visitKvm(report))
	}
	report.Add(s.visitLoopback(report))

	if s.Check {
		verdict := report.CheckVerdict()
		return report, verdict
	}

	vendored := true
	if s.Vendor {
		report.Note("")
		vendored = s.VendorGoDeps(report)
	}

	code := report.Summarize()
	// A vendor failure makes setup fail. The tree does not build unless vendor/
	// agrees with go.mod. Reporting success after this failure is the problem
	// that this module must prevent. The script discards this result
	// (le.application.setup.action). It then ends with "Setup complete" and exit
	// 0 (plan/journal/validated-value-discarded-by-its-caller.md, 2026-08-26).
	if !vendored {
		report.Note("Vendoring failed: the tree will not build until `go mod vendor` succeeds.")
		return report, 1
	}
	if code == 0 {
		report.Note("Verify with: make ze-smoke-verify")
	}
	return report, code
}

// detailRequired is the message that both modes use for a required item that
// they fail to find. Upper case makes the required status easy to identify.
const detailRequired = "REQUIRED"

// --- One step per thing the machine must have -----------------------------

// visitTool probes one tool, installs it when asked to, and says what happened.
func (s *Setup) visitTool(report *Report, tool Tool, manager PackageManager, installer *Installer) Outcome {
	if s.Probe(tool) {
		return Outcome{Name: tool.Name, State: StatePresent}
	}

	if !tool.InstallableBy(manager) {
		why := tool.Note
		if why == "" {
			why = "no package for this platform"
		}
		return Outcome{Name: tool.Name, State: StateSkipped, Detail: why}
	}

	if installer == nil {
		if tool.Required {
			return Outcome{Name: tool.Name, State: StateMissing, Detail: detailRequired}
		}
		return Outcome{Name: tool.Name, State: StateSkipped, Detail: "optional, and not installed"}
	}

	if !installer.Install(tool) {
		// apt has already recorded why, and the command to run.
		if manager == ManagerApt && tool.Apt != "" {
			return Outcome{Name: tool.Name, State: StatePending, Detail: "not installed"}
		}
		if tool.Required {
			return Outcome{Name: tool.Name, State: StateMissing, Detail: "required"}
		}
		return Outcome{Name: tool.Name, State: StateSkipped, Detail: "optional"}
	}

	if s.Probe(tool) {
		return Outcome{Name: tool.Name, State: StateInstalled}
	}

	// The installer succeeded, but the probe still cannot find the tool. Thus,
	// the tool is on the disk but is not usable. Previously, this state counted
	// as [installed]. The run ended with "Setup complete" and exit 0, but check
	// mode on the same machine exited 1. The two modes continued to disagree,
	// and install mode reported success for an absent tool.
	//
	// This condition occurs after every pipx installation on a fresh Debian
	// system. ~/.local/bin is on PATH only if it existed at login. A `go install`
	// operation can cause the same condition through ~/go/bin.
	var tb textbuf.Buffer
	report.Note(tb.Str("    add ").Str(whereItLanded(tool)).Str(", then re-run").String())
	return Outcome{Name: tool.Name, State: StatePending, Detail: "installed, not on PATH"}
}

// whereItLanded names the directory an install put the tool in, for a PATH fix.
func whereItLanded(tool Tool) string {
	if tool.PipxInstall != "" {
		return "~/.local/bin, which pipx uses; run `pipx ensurepath`"
	}
	if tool.GoInstall != "" {
		return "~/go/bin, which `go install` uses"
	}
	return "the package manager's bin directory"
}

// visitServer turns a language-server answer into an outcome.
//
// ABSENT is SKIPPED rather than MISSING: the tool row above installs the
// binary, so reporting it twice would make one missing server two failures.
// BROKEN is MISSING, because installing it again does not repair it and
// somebody has to look.
func visitServer(name string, answer ServerAnswer) Outcome {
	switch answer.Health {
	case HealthOK:
		return Outcome{Name: name, State: StatePresent, Detail: answer.Detail}
	case HealthNA:
		var tb textbuf.Buffer
		return Outcome{Name: name, State: StatePresent, Detail: tb.Str("n/a: ").Str(answer.Detail).String()}
	case HealthAbsent:
		return Outcome{Name: name, State: StateSkipped, Detail: answer.Detail}
	case HealthBroken:
		return Outcome{Name: name, State: StateMissing, Detail: answer.Detail}
	}
	return Outcome{Name: name, State: StateMissing, Detail: answer.Detail}
}

// visitLspPlugin says whether the harness can reach the language server for
// these file types.
//
// PENDING rather than MISSING when it is absent, because nothing this tool runs
// can fix it: `claude plugin ...` does not return from inside a session. A human
// runs one command, and PENDING is the state that means exactly that. It still
// fails the run, which is the point -- the silent version of this cost weeks of
// whole-file reads.
func (s *Setup) visitLspPlugin(report *Report, plugin LspPlugin, missing []LspPlugin) Outcome {
	var named textbuf.Buffer
	name := named.Str(plugin.Plugin).Str("-installed").String()

	if !containsPlugin(missing, plugin) {
		var tb textbuf.Buffer
		return Outcome{Name: name, State: StatePresent, Detail: tb.Join(plugin.Extensions, " ").String()}
	}

	var command textbuf.Buffer
	report.Note(command.Str("  Run: ").Str(plugin.InstallCommand()).String())
	var why textbuf.Buffer
	report.Note(why.Str("    ").Str(plugin.Why).String())

	var detail textbuf.Buffer
	detail.Str("the LSP tool refuses ").Join(plugin.Extensions, ", ")
	return Outcome{Name: name, State: StatePending, Detail: detail.String()}
}

// containsPlugin reports whether this plugin is one of the missing ones.
func containsPlugin(missing []LspPlugin, plugin LspPlugin) bool {
	for _, one := range missing {
		if one.Plugin == plugin.Plugin {
			return true
		}
	}
	return false
}

// visitUserns reports the unprivileged-userns restriction, and lifts it on an
// install run.
func (s *Setup) visitUserns(report *Report) Outcome {
	const name = "userns-unrestricted"

	state, err := s.UsernsState()
	if err != nil {
		// The knob is there and cannot be read, so nothing here knows whether
		// Chrome can start. That is a failure to answer, not an answer.
		var tb textbuf.Buffer
		return Outcome{Name: name, State: StateMissing, Detail: tb.Str("unreadable: ").Err(err).String()}
	}
	if state == UsernsOK {
		return Outcome{Name: name, State: StatePresent}
	}
	if state == UsernsNA {
		return Outcome{Name: name, State: StatePresent, Detail: "n/a: no apparmor userns knob"}
	}
	if s.Check {
		return Outcome{Name: name, State: StateMissing, Detail: detailRequired}
	}
	if s.ApplyUserns(report) {
		return Outcome{Name: name, State: StateInstalled}
	}
	report.Note("  Could not apply automatically; run manually:")
	noteUsernsFix(report)
	return Outcome{Name: name, State: StatePending, Detail: "restriction still in place"}
}

// visitKvm reports whether QEMU can use KVM, and joins the group on an install
// run.
func (s *Setup) visitKvm(report *Report) Outcome {
	const name = "kvm-access"

	switch s.KvmState() {
	case KvmOK:
		return Outcome{Name: name, State: StatePresent}
	case KvmNA:
		return Outcome{Name: name, State: StatePresent, Detail: "n/a: no /dev/kvm; QEMU uses tcg"}
	case KvmPendingLogin:
		var tb textbuf.Buffer
		detail := tb.Str("in the kvm group; log out and back in, or use: sg ").
			Str(KVMGroup).Str(" -c '<command>'").String()
		return Outcome{Name: name, State: StatePending, Detail: detail}
	case KvmNoGroup:
		// The device exists, but this user cannot open it. The steps below can
		// correct only this state.
	}

	if s.Check {
		return Outcome{Name: name, State: StateMissing, Detail: detailRequired}
	}
	if s.ApplyKvm(report) {
		return Outcome{Name: name, State: StatePending, Detail: "log out and back in to pick up the new group"}
	}
	report.Note("  Could not apply automatically; run manually:")
	s.noteKvmFix(report)
	return Outcome{Name: name, State: StatePending, Detail: "not in the kvm group"}
}

// visitLoopback reports the loopback addresses the suite binds, and adds the
// missing ones on an install run.
func (s *Setup) visitLoopback(report *Report) Outcome {
	const name = "loopback-addresses"

	missing := s.MissingLoopback()
	if len(missing) == 0 {
		return Outcome{Name: name, State: StatePresent}
	}

	var listed textbuf.Buffer
	list := listed.Join(missing, ", ").String()

	if s.Check {
		var tb textbuf.Buffer
		return Outcome{Name: name, State: StateMissing, Detail: tb.Str(list).Str(" (").Str(detailRequired).Str(")").String()}
	}
	if s.ApplyLoopback(report, missing) {
		return Outcome{Name: name, State: StateInstalled, Detail: list}
	}
	report.Note("  Could not apply automatically; run manually:")
	s.noteLoopbackFix(report, missing)
	return Outcome{Name: name, State: StatePending, Detail: list}
}

// noteManualList records every tool and the executables that prove it, for an
// unsupported host.
func (s *Setup) noteManualList(report *Report) {
	report.Note("")
	report.Note("Manual installation required. Tools needed:")
	report.Note("")

	for _, section := range [...]struct {
		heading  string
		required bool
		blank    bool
	}{{"Required:", true, false}, {"Optional:", false, true}} {
		if section.blank {
			report.Note("")
		}
		report.Note(section.heading)
		for _, tool := range AllTools() {
			if tool.Required != section.required {
				continue
			}
			var tb textbuf.Buffer
			tb.Str("  ").Str(tool.Name).Str(": ").Join(tool.Probe, ", ")
			if tool.Note != "" {
				tb.Str(" (").Str(tool.Note).Str(")")
			}
			report.Note(tb.String())
		}
	}
}
