// Design: docs/architecture/testing/qemu-integration.md -- the slot a guest command runs in
// Related: actions.go -- runQEMUHere, the answer that admits through here
// Related: guestle.go -- guestJobParent, the stand-in a guest le names as its parent
// Related: run.go -- setupCommand, which names that stand-in for the guest command

package testqemu

import (
	"os"
	"strings"

	"github.com/ze-software/ze/internal/le/gaterun"
	"github.com/ze-software/ze/internal/le/job"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// runVerb is the verb `test qemu run` is typed as: the action declares it, and
// runArgv spells it back.
const runVerb = "run"

// runJobLabel is the registry label a `test qemu run` holds its slot under.
const runJobLabel = "qemu-run"

// runSelf answers the program a claimed slot re-executes. It is a variable so
// a test can name a child without depending on where the test binary runs.
var runSelf = os.Executable

// admitQEMURun admits one `test qemu run` through the host's job registry and
// answers its code.
//
// The host holds the slot, never the guest. The guest sees the host's registry
// through /workspace but not the host's processes, so a guest le that admitted
// itself there would be judged by a pid the host cannot see, and judge the
// host's holders by pids it cannot see. Its admission would also fingerprint
// the whole checkout over 9p, where every stat in git's index misses and
// `git diff HEAD` reads every tracked file: on 2026-10-27 that ran past
// job's git limit and killed `le test firewall` before its first test. So the
// host takes the slot here, and setupCommand names a stand-in for it as the
// guest command's job parent: every `le test <suite>` the command starts runs
// inside this slot.
//
// The run never attaches to another run's verdict. Its inputs include files
// git does not track (the kernel image, the cross-built binaries), so two runs
// with the same words over the same tree can still judge different code.
//
//   - KindInside: a parent already holds the slot (`le job run`, or this
//     command re-run as the child below), so run works in this process.
//   - KindClaimed: the slot is ours. The run re-executes this command as a
//     child, because the slot's log MUST grow while it is held and the harness
//     writes to this process's stdout and stderr. The child finds the entry
//     job.RunClaimed names and answers KindInside.
func admitQEMURun(root string, argv []string, run func() (any, int)) (any, int) {
	admission, err := job.NewIn(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, gaterun.CannotStart
	}
	admission.Out = os.Stderr
	admission.MayAttach = false

	ticket, err := admission.Admit(runJobLabel, argv)
	if err != nil {
		leaction.ReportError(err)
		return nil, gaterun.CannotStart
	}
	switch ticket.Kind {
	case job.KindInside:
		return run()
	case job.KindClaimed:
		return nil, runQEMUClaimed(admission, ticket, argv)
	case job.KindUnspecified, job.KindAttached, job.KindUnadmitted:
	}
	panic("BUG: a qemu run that may not attach was answered a ticket of kind " + ticket.Kind.String())
}

// runQEMUClaimed re-executes the run whose words follow argv[0] as a child in
// the slot the ticket holds. The ticket is released on every path.
func runQEMUClaimed(admission *job.Admission, ticket *job.Ticket, argv []string) int {
	self, err := runSelf()
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

// runArgv spells a parsed `test qemu run` line back as the words that typed
// it, from the parameters the action declares: the job key fingerprints these
// words, and a claimed slot re-executes them.
func runArgv(args leaction.Arguments) []string {
	argv := make([]string, 0, 4+2*len(runParameters))
	argv = append(argv, "le")
	argv = append(argv, strings.Fields(area)...)
	argv = append(argv, runVerb)
	for _, parameter := range runParameters {
		if !args.Has(parameter.Keyword) {
			continue
		}
		if parameter.Kind == leaction.KindSwitch {
			argv = append(argv, parameter.Keyword)
			continue
		}
		for _, value := range args.Values(parameter.Keyword) {
			argv = append(argv, parameter.Keyword, value)
		}
	}
	return argv
}
