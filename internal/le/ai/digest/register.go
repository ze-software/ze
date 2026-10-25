// Design: docs/architecture/core-design.md -- le's composition, one import per tool
//
// One package, one register.go, one init(). Adding a tool to le is this file
// plus a blank import in internal/le/register.go, and nothing else.

package aidigest

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// name is the word this command is typed as.
const name = "ai digest"

// checkVerb names the gate, which is also what the bare command line runs.
const checkVerb = "check"

// table declares the whole command surface. The one action takes no value: the
// tree is the checkout and the rendering is a pipe operator.
var table = leaction.New(name, leaction.Action{
	Verb:   checkVerb,
	Why:    "every file:line anchor in ai/digests/*.md resolves to a real file and an in-range line",
	Answer: answerCheck,
})

func init() {
	leroot.Register(name, leroot.GroupGate, Answer, registry.Meta{
		ShortHelp: "every file:line anchor in ai/digests/*.md resolves to a real file and an in-range line",
		Mode:      "offline",
		// SectionTest is where ze files a tool rather than a product command;
		// internal/perf/cli registers le perf under it for the same reason.
		Section:  registry.SectionTest,
		SubsFunc: table.Subs,
	})
	leroot.RegisterActions(name, table.Actions)

	// The answer carries counts beside two row sets, so the any-shape operators
	// render the whole document rather than being refused.
	leroot.RegisterShape(name, command.ShapeMap)

	// The census counts this gate as ported from here, in the same init() that
	// registers the command. A claim whose command never registered is red, so
	// the count cannot fall for a tool nothing can reach.

}

// Answer is the `le ai digest` command. A bare line runs the gate, as the
// `check` action does: hooks and verify stages call it bare, and a bare listing
// would answer them 0 with nothing judged. Every other line is the table's.
func Answer(args []string) (any, int) {
	if len(args) == 0 {
		return table.Answer([]string{checkVerb})
	}
	return table.Answer(args)
}

// answerCheck runs the gate over the checkout.
func answerCheck() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: digest: %v\n", err) //nolint:errcheck // CLI output
		return nil, 2
	}

	report, err := Check(tree)
	if err != nil {
		// 2 rather than 1: the gate cannot read the judged population. The
		// caller can distinguish this from a digest with a rotted anchor.
		fmt.Fprintf(os.Stderr, "error: digest: %v\n", err) //nolint:errcheck // CLI output
		return nil, 2
	}

	if len(report.Errors) > 0 {
		os.Stderr.WriteString(report.Diagnosis()) //nolint:errcheck // CLI output
		return report, 1
	}
	return report, 0
}
