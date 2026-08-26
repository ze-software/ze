// Design: docs/architecture/core-design.md -- a delegated gate, answered in this binary
// Overview: docwiring.go -- the router that selects these
//
// delegate.go is where a SELECTED gate is run.
//
// Eight of the twelve selected targets are Go packages already linked into this
// binary, so the router calls them as functions. Starting `make
// ze-digest-check` would start make and then the Python le to reach work already
// in this process. That uses three processes for one function call and makes the
// census misreport the work as ported.
//
// The other four targets have no Go implementation, so they still start make.
// forks.go publishes each command line instead of hiding it.
// letools/parity counts the gate as claimed-but-not-converted while any target
// remains a script.

package docwiring

import (
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/letools/commandlist"
	"github.com/ze-software/ze/letools/commandownership"
	"github.com/ze-software/ze/letools/digest"
	"github.com/ze-software/ze/letools/discoveryindex"
	"github.com/ze-software/ze/letools/docvalid"
	"github.com/ze-software/ze/letools/functional"
	"github.com/ze-software/ze/letools/inventory"
	"github.com/ze-software/ze/letools/leroot"
	"github.com/ze-software/ze/letools/pluginimports"
)

// call contains one tool invocation: its command entry point and the arguments
// that select the action.
//
// The entry point is Answer because that is what `le <tool>` runs. A deeper
// function call CAN pass while the operator command fails.
type call struct {
	answer leroot.Answer
	args   []string
}

// goTargets maps a delegated Make target to the calls that do its work in this
// binary. A target absent here has no Go implementation and starts make.
//
// The tool resolves the checkout itself, with lepath.Root, which is the same
// walk this command's own Answer made before it built the router. The two agree
// for every caller that reached here through the command.
var goTargets = map[string]call{
	"ze-command-contract-check":  {docvalid.Answer, []string{"command-contract-check"}},
	"ze-command-ownership-check": {commandownership.Answer, nil},
	"ze-discovery-index-check":   {discoveryindex.Answer, []string{"check"}},
	"ze-digest-check":            {digest.Answer, nil},
	"ze-inventory-json":          {inventory.Answer, nil},
	"ze-command-list-json":       {commandlist.Answer, nil},
	"ze-plugin-imports-check":    {pluginimports.Answer, []string{"check"}},
	// The Make target names the selftest before the scan, and so does this list.
	// The selftest exercises every scan verdict on a known fixture. A scan after
	// a failed selftest would use a checker already shown to be broken.
	// leaction.StopAtFirstFailure states that ordering rule once.
	"ze-functional-docker-exec-check": {
		functional.Answer, []string{"docker-exec-selftest", "docker-exec-check"},
	},
}

// GoTargets answers the delegated targets implemented in this binary. The
// answer comes from the table above, so a selection-order test cannot compare
// against another copy.
func GoTargets() []string {
	out := make([]string, 0, len(goTargets))
	for target := range goTargets {
		out = append(out, target)
	}
	return out
}

// runTarget runs one selected target, in this process when a Go package holds
// its work and as a Make invocation when none does.
func (g *gate) runTarget(target string) CheckResult {
	if one, ok := goTargets[target]; ok {
		return g.runGoTarget(target, one)
	}
	return g.runMakeTarget(target)
}

// runGoTarget answers one delegated gate by calling the tool that holds it.
//
// The tool's exit code does NOT leave this function. This gate reports whether
// the complete run is clean, as Run documents. Otherwise, a stale-index code 3
// would become the router's result for a gate that failed.
func (g *gate) runGoTarget(target string, one call) CheckResult {
	var tb textbuf.Buffer
	payload, code := one.answer(one.args)
	if code == 0 {
		return CheckResult{Message: tb.Str(target).Str(" PASSED").String()}
	}

	// The tool names its own files when it knows them, and this gate does not
	// read a report it did not produce. The group is declared so the red is
	// CHARGED rather than leaving the failure index. The rerun is the Make
	// target, because that is the word every rule, doc and journal row spells
	// for this gate.
	g.declareFailureGroup(target, nil,
		tb.Str("delegated check ").Str(target).Str(" failed").String(),
		tb.Reset().Str("make ").Str(target).String())

	return CheckResult{
		Failed:  true,
		Message: tb.Reset().Str(target).Str(" failed").String(),
		Output:  prose(payload),
	}
}

// prose renders a payload as leroot renders a bare invocation. Thus, this gate
// relays the same page as the tool. A payload without its own rendering carries
// facts in `| json`, so there is no prose to relay.
func prose(payload any) string {
	if text, ok := payload.(leroot.Prose); ok {
		return text.Text()
	}
	return ""
}
