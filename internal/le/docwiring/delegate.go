// Design: docs/architecture/core-design.md -- a selected gate, answered in this binary
// Overview: docwiring.go -- the router that selects these
//
// delegate.go is where a selected gate is run. Every row calls a linked Go
// owner. A target absent from the table is a programming error and is refused;
// there is no process fallback.

package docwiring

import (
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/commandlist"
	"github.com/ze-software/ze/internal/le/commandownership"
	"github.com/ze-software/ze/internal/le/digest"
	"github.com/ze-software/ze/internal/le/discoveryindex"
	"github.com/ze-software/ze/internal/le/docstocode"
	"github.com/ze-software/ze/internal/le/docvalid"
	"github.com/ze-software/ze/internal/le/functional"
	"github.com/ze-software/ze/internal/le/inventory"
	"github.com/ze-software/ze/internal/le/leroot"
	"github.com/ze-software/ze/internal/le/pluginimports"
	"github.com/ze-software/ze/internal/le/speccitation"
)

// call is one linked target invocation. The root is explicit for callbacks
// whose owner exposes a tree-taking API. Registered command adapters ignore it
// because their packages resolve the same checkout through lepath.
type call struct {
	answer func(root string) (any, int)
}

// registered adapts an existing native command entry point to the target table.
func registered(answer leroot.Answer, args []string) call {
	return call{answer: func(string) (any, int) { return answer(args) }}
}

// goTargets is the complete selected target table. Keeping every target here
// makes absence fail closed instead of turning into an undeclared process run.
var goTargets = map[string]call{
	"ze-command-contract-check":  registered(docvalid.Answer, []string{"command-contract-check"}),
	"ze-command-ownership-check": registered(commandownership.Answer, nil),
	"ze-doc-verify":              {answer: answerDocVerify},
	"ze-doc-index-check":         {answer: answerDocIndex},
	"ze-discovery-index-check":   registered(discoveryindex.Answer, []string{"check"}),
	"ze-digest-check":            registered(digest.Answer, nil),
	"ze-inventory-json":          registered(inventory.Answer, nil),
	"ze-command-list-json":       registered(commandlist.Answer, nil),
	"ze-plugin-imports-check":    registered(pluginimports.Answer, []string{"check"}),
	"ze-templ-output-check":      {answer: answerTemplOutput},
	// The selftest exercises every scan verdict on a known fixture. A scan after
	// a failed selftest would use a checker already shown to be broken.
	"ze-functional-docker-exec-check": registered(functional.Answer,
		[]string{"docker-exec-selftest", "docker-exec-check"}),
	"ze-spec-citation-check": {answer: answerSpecCitation},
}

// GoTargets answers the selected targets implemented in this binary. The
// answer comes from the run table, so selection and implementation cannot use
// separate inventories.
func GoTargets() []string {
	out := make([]string, 0, len(goTargets))
	for target := range goTargets {
		out = append(out, target)
	}
	return out
}

// runTarget calls one selected target. An absent callback is a structured
// failure, never permission to run another program or to report success.
func (g *gate) runTarget(target string) CheckResult {
	one, ok := goTargets[target]
	if !ok || one.answer == nil {
		var tb textbuf.Buffer
		return CheckResult{
			Failed:  true,
			Code:    2,
			Message: tb.Str("no native callback for target ").Str(target).String(),
		}
	}
	return g.runGoTarget(target, one)
}

// runGoTarget answers one selected gate by calling the package that owns it.
//
// The owner's non-zero code marks the target failed. The complete router keeps
// its historical binary verdict: any selected-target failure answers 1.
func (g *gate) runGoTarget(target string, one call) CheckResult {
	var tb textbuf.Buffer
	payload, code := one.answer(g.root)
	if code == 0 && payload == nil {
		return CheckResult{
			Failed:  true,
			Code:    2,
			Message: tb.Str(target).Str(" native callback returned no result").String(),
		}
	}
	if code == 0 {
		return CheckResult{
			Message: tb.Str(target).Str(" PASSED").String(),
			Output:  prose(payload),
		}
	}

	g.declareFailureGroup(target, nil,
		tb.Str("delegated check ").Str(target).Str(" failed").String(),
		tb.Reset().Str("make ").Str(target).String())

	return CheckResult{
		Failed:  true,
		Message: tb.Reset().Str(target).Str(" failed").String(),
		Output:  prose(payload),
	}
}

// answerDocIndex runs the source-anchor check against the tree named by this
// router rather than resolving another implicit checkout.
func answerDocIndex(root string) (any, int) {
	report, err := docstocode.CheckCodeIndex(root)
	if err != nil {
		return errorPage(err), 2
	}
	if len(report.Stale) > 0 || len(report.Claims) > 0 {
		return report, 1
	}
	return report, 0
}

// answerSpecCitation runs the citation owner against the explicit tree.
func answerSpecCitation(root string) (any, int) {
	report, err := speccitation.Scan(root)
	if err != nil {
		return errorPage(err), 2
	}
	if len(report.Dangling) > 0 {
		return report, 1
	}
	return report, 0
}

// nativeErrorPage keeps a native callback failure in the structured target
// result.
type nativeErrorPage struct {
	err error
}

func errorPage(err error) nativeErrorPage {
	return nativeErrorPage{err: err}
}

func (e nativeErrorPage) Text() string {
	var tb textbuf.Buffer
	return tb.Str("error: ").Err(e.err).Byte('\n').String()
}

// prose renders a payload as leroot renders a bare invocation. Thus, this gate
// relays the same page as the linked tool.
func prose(payload any) string {
	if text, ok := payload.(leroot.Prose); ok {
		return text.Text()
	}
	return ""
}
