// Design: docs/architecture/core-design.md -- the qemu area, as one command
// Detail: hugepages.go -- the proof this table reaches
//
// actions.go is the table. The dispatch, the listing, the help line and the two
// refusals are letools/leaction, which every ported area shares.
//
// THE AREA IS THE GATE-NAME FAMILY, not the directory the script came from.
// `ze-qemu-vpp-hugepages-test` begins ze-qemu-. leaction removes `ze-<area>-`
// and nothing else to derive each verb. This gate therefore cannot be a row of
// letools/integration unless it is typed as its own whole name. It came out of
// scripts/evidence beside the ze-deployment- family. The two split here for the
// same reason that they are two Make target families.
//
// Thirteen more ze-qemu- Make targets exist, and none is an le gate today
// (mk/test-integration.mk). Each is a row here when it becomes one.

package qemu

import (
	"github.com/ze-software/ze/letools/leaction"
	"github.com/ze-software/ze/letools/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "qemu"

var actions = leaction.New(area,
	leaction.Action{
		Gate: "ze-qemu-vpp-hugepages-test",
		Why: "boot-time hugepage reservation, end to end: build an appliance carrying" +
			" image.hugepages, boot it, then assert `show host kernel` and `show host" +
			" memory` over the Ze CLI. Self-skips when qemu, sshpass, e2fsprogs or go" +
			" are absent; on Linux it needs membership of the kvm group" +
			" (make ze-dev-setup checks it as kvm-access)",
		Answer: runHugepagesHere,
	},
	leaction.Action{
		// No Make target names this one, and that is not an oversight. The
		// ze-qemu- targets run on the HOST: each cross-compiles three binaries
		// and boots a VM. This action is what runs INSIDE that VM, so its
		// caller is the guest command line rather than a gate
		// (mk/test-integration.mk, the --run string).
		Verb: "all-tests",
		Why: "the whole ze test suite, inside the QEMU Linux VM: every functional suite at a" +
			" VM-appropriate concurrency, the unit pass, and the integration-tagged tests." +
			" Refuses to start outside the guest, or with a suite list that has a hole in it",
		// The unit phase still delegates its work to another program. It calls
		// the Makefile's own -impl body and does not compile the package set
		// itself. The table declares this call so the census reads what this
		// action starts from the table rather than from the code
		// (letools/leaction, Forks).
		Forks:  []string{"make", "_ze-unit-test-cached-impl"},
		Answer: runAllTestsHere,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Gates answers the Make target of every action that has one, which is what the
// census claims.
func Gates() []string { return actions.Gates() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le qemu` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// runHugepagesHere proves the reservation over the checkout this command was
// run in.
func runHugepagesHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runHugepages(NewHugepages(root))
}

// runHugepages answers the proof over one run.
//
// A step that the run cannot perform is an error. It answers 1 with no verdict.
// A SKIP answers 0. This is the self-skip contract that the functional suite and
// CI both rely on. A machine without QEMU has not disproved anything.
func runHugepages(run *Hugepages) (HugepagesReport, int) {
	report, err := run.Run()
	if err != nil {
		leaction.ReportError(err)
		return report, 1
	}
	if report.Verdict == VerdictFail || report.Verdict == VerdictUnspecified {
		return report, 1
	}
	return report, 0
}

// runAllTestsHere runs every phase inside the guest this command was started in.
//
// It takes no checkout argument, and it does not consult lepath.Root(). The
// guest mounts the repository at one known path. If a run used a checkout from
// another location, it would use the host's tree with the guest's assumptions.
// The mount is the precondition, and the refusal names it.
func runAllTestsHere() (any, int) {
	report, code := NewAllTests().Execute()
	if len(report.Phases) == 0 {
		// The run never started. Its refusal is already on stderr. A report with
		// no phases does not answer "what did the VM prove". `| json` would put
		// an empty document beside a non-zero code.
		return nil, code
	}
	return report, code
}
