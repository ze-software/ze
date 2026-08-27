// Design: docs/architecture/core-design.md -- the web-assets area, as one command
//
// actions.go is the table. The dispatch, the listing, the help line and the two
// refusals are internal/le/leaction, which every ported area shares.
//
// Three actions, and only one of them writes. `pages` is the script's --json
// mode: it answers the derived sets and touches nothing, because a test asks
// for them while it runs and must not leave the tree changed by reading it.

package webassets

import (
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from the gate name to derive its verb.
const area = "web-assets"

var actions = leaction.New(area,
	leaction.Action{
		Gate: "ze-web-assets-check",
		Why: "each page_assets.go agrees with the markup its package renders. The generator walks" +
			" the templ component graph from each page, so a component that gains hx-sse:connect" +
			" changes the set for every page reaching it. A stale file leaves a page missing an" +
			" extension it now needs, which is invisible everywhere but the browser: the page" +
			" renders and does nothing",
		Answer: runCheckHere,
	},
	leaction.Action{
		Verb:   "write",
		Why:    "rewrite each package's per-page asset sets from the markup it renders",
		Writes: true,
		Answer: runWriteHere,
	},
	leaction.Action{
		Verb:   "pages",
		Why:    "print the derived per-page asset sets and write nothing",
		Answer: runPagesHere,
	},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le web-assets` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

func runCheckHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runCheck(root)
}

func runWriteHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runWrite(root)
}

func runPagesHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return runPages(root)
}

// runCheck answers the check over one tree. A stale file is a verdict rather
// than an error, so it answers 1 with the report; markup the walk could not
// read is an error and answers 1 with none.
func runCheck(root string) (CheckReport, int) {
	report, err := Check(root)
	if err != nil {
		leaction.ReportError(err)
		return CheckReport{}, 1
	}
	if report.Stale != "" {
		return report, 1
	}
	return report, 0
}

// runWrite answers the write over one tree.
func runWrite(root string) (WriteReport, int) {
	report, err := Write(root)
	if err != nil {
		leaction.ReportError(err)
		return WriteReport{}, 1
	}
	return report, 0
}

// runPages answers the derived sets over one tree.
func runPages(root string) (PageSets, int) {
	sets, err := Pages(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return sets, 0
}
