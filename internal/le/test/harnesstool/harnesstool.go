// Design: docs/architecture/testing/ci-format.md -- every harness command is `le test <name>`
// Related: ../../../test/cli/ci_runner.go -- the suite runner a suite command calls
// Related: ../../job/job.go -- the admission a runner command takes on the host
//
// Package harnesstool adapts one handler of the test harness to an le command
// under the `test` namespace (plan/spec-le-subject-first-command-tree.md, D-8).
//
// It registers nothing itself and holds no register.go. Each harness command
// has its own package at internal/le/test/<name>, whose register.go calls
// leroot.Register, leroot.RegisterShape and leroot.RegisterForwarding with the
// answer and help built here, so a new harness command adds one package and
// edits no list.
//
// Two kinds of harness command exist (AC-45). A runner (a suite, or bgp,
// editor, exabgp, vpp, web) runs the functional runner, which starts ze daemons
// by the dozen, so on the host it admits its run through internal/le/job
// (RunnerAnswer, SuiteAnswer). A helper tool (peer, rpki, a mock) runs inside a
// test, and the suite that started it already holds the slot, so it never
// admits (Answer).
package harnesstool

import (
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/job"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/test/cli"
	"github.com/ze-software/ze/internal/test/runner"
)

// Answer adapts a helper tool's handler to an le answer. It never admits: a
// helper runs inside a test whose suite holds the slot.
//
// Every word after `test <name>` reaches run unchanged, and run's exit code is
// le's exit code.
// The answer carries no payload, because the harness writes its own output.
//
// The command MUST be registered as forwarding (leroot.RegisterForwarding), so
// a trailing help word reaches run and the harness prints its own help.
func Answer(run func([]string) int) leroot.Answer {
	return func(args []string) (any, int) {
		return nil, run(args)
	}
}

// Meta answers the help metadata of a harness command.
func Meta(shortHelp string) registry.Meta {
	return registry.Meta{
		ShortHelp: shortHelp,
		Mode:      "offline",
		Section:   registry.SectionTest,
	}
}

// SuiteAnswer adapts one functional suite to an le answer that runs the .ci
// files under test/<suite.TestSubdir>.
//
// suite.DefaultParallel is the DEFAULT for -p, not a ceiling. Zero means "no
// opinion, use the host-derived default" (runner.DefaultSuiteConcurrency), NOT
// "all at once": unset and all-at-once used to be the same value, which is how
// `ospf --all` came to launch 97 ze daemons simultaneously and kill CI's runner
// agent. An operator who genuinely wants that behavior still asks for it with
// `-p 0`. The default is resolved when the command runs, so registration reads
// no host state.
//
// A suite is a runner, so the answer admits its run (RunnerAnswer).
func SuiteAnswer(suite cli.CIRunnerConfig) leroot.Answer {
	return RunnerAnswer("test "+suite.Name, func(args []string) int {
		return RunSuite(suite, args)
	})
}

// RunSuite runs one functional suite with the caller's words and answers the
// exit code. It resolves a zero DefaultParallel to the host-derived default
// (SuiteAnswer says why zero is not "all at once"). It admits nothing: the
// answer that calls it MUST come from RunnerAnswer.
func RunSuite(suite cli.CIRunnerConfig, args []string) int {
	resolved := suite
	if resolved.DefaultParallel == 0 {
		resolved.DefaultParallel = runner.DefaultSuiteConcurrency()
	}
	return cli.RunCISubcommand(resolved, args)
}

// AreaMember is one member of a harness area: the word that picks it, one
// line of help, and the handler that receives the words after that word.
type AreaMember struct {
	Word string
	Help string
	Run  func([]string) int
}

// areaSuiteDirs holds the test directory of every suite an area runs
// (SuiteMember), in registration order. Written only from package init, so
// read-only once main starts.
var areaSuiteDirs []string

// SuiteMember answers the area member that runs one functional suite, and
// records the suite's test directory as reached (AreaSuiteDirs). MUST be
// called from package init, like the leroot registration it feeds.
func SuiteMember(word string, suite cli.CIRunnerConfig) AreaMember {
	areaSuiteDirs = append(areaSuiteDirs, suite.TestSubdir)
	var tb textbuf.Buffer
	return AreaMember{
		Word: word,
		Help: tb.Str("Run ").Str(suite.Description).Str(" functional tests").String(),
		Run:  func(args []string) int { return RunSuite(suite, args) },
	}
}

// AreaSuiteDirs answers a copy of the test directories that area members run,
// so a guard that looks for a `test <dir>` command also finds a suite an area
// reaches under another name (`test/isis-wire` through `le test wire isis`).
func AreaSuiteDirs() []string {
	return slices.Clone(areaSuiteDirs)
}

// Area answers a handler for a harness area such as `le test wire`, whose
// first word picks one member and whose remaining words reach that member
// unchanged. No word, or a help word, lists the members and answers 0. A word
// that names no member is refused with exit 1, because running a default
// member would hide the typo.
//
// An area groups members that share a subject but not a runner: each keeps
// its own handler, so a new member adds one AreaMember and nothing else.
func Area(area string, members []AreaMember) func([]string) int {
	return func(args []string) int {
		if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
			os.Stdout.WriteString(areaList(area, members)) //nolint:errcheck // CLI output
			return 0
		}
		for _, member := range members {
			if member.Word == args[0] {
				return member.Run(args[1:])
			}
		}
		var tb textbuf.Buffer
		tb.Str("le ").Str(area).Str(": unknown member ").Quoted(args[0]).Str("; run le ").Str(area).Str(" to list them\n")
		os.Stderr.WriteString(tb.String()) //nolint:errcheck // CLI output
		return 1
	}
}

// areaList answers the member listing Area prints: one line per member, in
// declaration order.
func areaList(area string, members []AreaMember) string {
	var tb textbuf.Buffer
	tb.Str("Usage: le ").Str(area).Str(" <member> [options]\n\nMembers:\n")
	for _, member := range members {
		tb.Str("  ").PadRight(member.Word, 8).Str(member.Help).Byte('\n')
	}
	return tb.String()
}

// SuiteMeta answers the help metadata of a suite command.
func SuiteMeta(suite cli.CIRunnerConfig) registry.Meta {
	var tb textbuf.Buffer
	return Meta(tb.Str("Run ").Str(suite.Description).Str(" functional tests").String())
}

// RunnerAnswer adapts a runner's handler to an le answer that admits the run
// through the job registry on the host, and marks name admitted
// (leroot.RegisterAdmitted). The mark and the admitting answer are one fact,
// declared by this one call, so the pretool hook cannot read a runner as light
// while its answer takes a slot.
//
// MUST be called from the init of the package that registers name, as the
// answer passed to leroot.Register for that same name.
func RunnerAnswer(name string, run func([]string) int) leroot.Answer {
	leroot.RegisterAdmitted(name)
	words := strings.Fields(name)
	label := strings.Join(words, "-")
	return func(args []string) (any, int) {
		return nil, admitRun(label, words, args, run)
	}
}

// checkoutRoot and selfExecutable are the two host facts admitRun reads. They
// are variables so a test can name a checkout and a child without depending
// on where the test binary runs.
var (
	checkoutRoot   = lepath.Root
	selfExecutable = os.Executable
)

// admitRun runs one runner command under admission and answers its code.
//
// With no checkout (a container, or a QEMU guest's mount point with no
// registry) the run is unadmitted: no registry exists to share, so the
// command runs in this process. On the host the answer depends on the ticket:
//
//   - KindInside: a parent job already holds a slot (le test functional, or
//     this command re-run as the child below), so the run is in-process.
//   - KindAttached: the same work over the same tree ran elsewhere, and its
//     output and verdict have been replayed.
//   - KindClaimed: the slot is ours. The run re-executes this command as a
//     child in the working directory, because the job's log MUST grow while
//     the slot is held, and the harness writes to the process's stdout. The
//     child finds the entry job.RunClaimed names and answers KindInside.
func admitRun(label string, words, args []string, run func([]string) int) int {
	root, err := checkoutRoot()
	if errors.Is(err, lepath.ErrNoCheckout) {
		return run(args)
	}
	if err != nil {
		leaction.ReportError(err)
		return gaterun.CannotStart
	}
	admission, err := job.NewIn(root)
	if err != nil {
		leaction.ReportError(err)
		return gaterun.CannotStart
	}

	argv := make([]string, 0, 1+len(words)+len(args))
	argv = append(argv, "le")
	argv = append(argv, words...)
	argv = append(argv, args...)
	ticket, err := admission.Admit(label, argv)
	if err != nil {
		leaction.ReportError(err)
		return gaterun.CannotStart
	}

	switch ticket.Kind {
	case job.KindInside:
		return run(args)
	case job.KindAttached:
		return ticket.Code
	case job.KindClaimed:
		return runClaimed(admission, ticket, argv)
	case job.KindUnspecified, job.KindUnadmitted:
	}
	panic("BUG: job admission answered a ticket of kind " + ticket.Kind.String())
}

// runClaimed re-executes the command whose words follow argv[0] as a child in
// the slot the ticket holds. The ticket is released on every path.
func runClaimed(admission *job.Admission, ticket *job.Ticket, argv []string) int {
	self, err := selfExecutable()
	if err != nil {
		ticket.Release(gaterun.CannotStart)
		leaction.ReportError(err)
		return gaterun.CannotStart
	}
	dir, err := os.Getwd()
	if err != nil {
		ticket.Release(gaterun.CannotStart)
		leaction.ReportError(err)
		return gaterun.CannotStart
	}
	child := make([]string, 0, len(argv))
	child = append(child, self)
	child = append(child, argv[1:]...)
	return admission.RunClaimed(ticket, child, dir, nil)
}
