// Design: docs/architecture/core-design.md -- selected native checks.
// Overview: docwiring.go -- the router that selects these.
//
// Every row calls a linked Go owner. An action absent from the table is a
// programming error and is refused; there is no process fallback.

package docwiring

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	commandlist "github.com/ze-software/ze/internal/le/command/list"
	commandownership "github.com/ze-software/ze/internal/le/command/ownership"
	"github.com/ze-software/ze/internal/le/digest"
	"github.com/ze-software/ze/internal/le/discoveryindex"
	"github.com/ze-software/ze/internal/le/docstocode"
	"github.com/ze-software/ze/internal/le/docvalid"
	"github.com/ze-software/ze/internal/le/inventory"
	"github.com/ze-software/ze/internal/le/leroot"
	pluginimports "github.com/ze-software/ze/internal/le/plugin/imports"
	speccitation "github.com/ze-software/ze/internal/le/spec/citation"
)

// call is one linked action invocation. The root is explicit for callbacks
// whose owner exposes a tree-taking API.
type call struct {
	answer func(root string) (any, int)
}

// registered adapts an existing native command entry point.
func registered(answer leroot.Answer, args []string) call {
	return call{answer: func(string) (any, int) { return answer(args) }}
}

// goActions is the complete selected action table.
var goActions = map[string]call{
	actionDocvalidCommandContract: registered(docvalid.Answer, []string{"command-contract"}),
	"command ownership":           registered(commandownership.Answer, nil),
	actionDocCheckVerify:          {answer: answerDocVerify},
	actionDocsToCodeIndexCheck:    {answer: answerDocIndex},
	actionDiscoveryIndexCheck:     registered(discoveryindex.Answer, []string{"check"}),
	actionDigest:                  registered(digest.Answer, nil),
	actionInventory:               registered(inventory.Answer, nil),
	actionCommandList:             registered(commandlist.Answer, nil),
	actionPluginImportsCheck:      registered(pluginimports.Answer, []string{"check"}),
	"doc check/templ-output":      {answer: answerTemplOutput},
	"spec citation/anchors":       {answer: answerSpecCitation},
}

// runAction calls one selected action. An absent callback is a structured
// failure, never permission to run another program or report success.
func (g *checker) runAction(action string) CheckResult {
	one, ok := goActions[action]
	if !ok || one.answer == nil {
		var tb textbuf.Buffer
		return CheckResult{
			Failed: true, Code: 2,
			Message: tb.Str("no native callback for action ").Str(action).String(),
		}
	}
	return g.runGoAction(action, one)
}

func (g *checker) runGoAction(action string, one call) CheckResult {
	var tb textbuf.Buffer
	payload, code := one.answer(g.root)
	if code == 0 && payload == nil {
		return CheckResult{
			Failed: true, Code: 2,
			Message: tb.Str(action).Str(" native callback returned no result").String(),
		}
	}
	if code == 0 {
		return CheckResult{Message: tb.Str(action).Str(" PASSED").String(), Output: prose(payload)}
	}
	rerun := tb.Reset().Str("./le ").Str(strings.ReplaceAll(action, "/", " ")).String()
	g.declareFailureGroup(action, delegatedFailurePaths(payload),
		tb.Reset().Str("delegated check ").Str(action).Str(" failed").String(), rerun)
	return CheckResult{
		Failed:  true,
		Message: tb.Reset().Str(action).Str(" failed").String(),
		Output:  prose(payload),
	}
}

// failurePathLister is implemented by a delegated action's report that can name
// the files its failure is about.
//
// It exists because a group with no paths is UNATTRIBUTABLE, and an
// unattributable structural red is charged to every commit in the checkout
// rather than to the one that caused it (internal/le/commit/verification.go,
// structuralGateReds). Every delegated action used to report nil here, so one
// session's red blocked every other session's commit, however unrelated.
//
// Opting in is per-report: an action whose report cannot name files keeps the
// old behavior, which is the honest answer for a check that judges the tree as
// a whole.
type failurePathLister interface {
	FailingPaths() []string
}

// delegatedFailurePaths asks a delegated action's report which files its failure
// is about, and answers nil when the report cannot say.
func delegatedFailurePaths(payload any) []string {
	lister, ok := payload.(failurePathLister)
	if !ok {
		return nil
	}
	return lister.FailingPaths()
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
