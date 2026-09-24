// Design: docs/architecture/core-design.md -- the doc index area, as one command
//
// actions.go is the command surface of the two generated indexes. The area
// generates two files, ai/DOCS-TO-CODE.md and ai/CODE-TO-DOCS.md, and like
// every other generated file it answers two verbs: `check` judges both files
// and the anchors, and `write` regenerates both files (D-5 of
// plan/spec-le-subject-first-command-tree.md).
//
// The dispatch, the listing, the help line and the two refusals live in
// internal/le/leaction. What stays here is the TABLE.

package docindex

import (
	"errors"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const area = "doc index"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{
		Verb: "check",
		Why: "ai/DOCS-TO-CODE.md and ai/CODE-TO-DOCS.md match the tree, and every " +
			"`<!-- source: -->` anchor in docs/ resolves to a real file and symbol",
		Answer: check,
	},
	leaction.Action{Verb: "write", Why: "regenerate ai/DOCS-TO-CODE.md and ai/CODE-TO-DOCS.md",
		Writes: true,
		Answer: write},
)

// IndexReport is the whole answer of one `le doc index` run: the design-doc
// index and the source-anchor index, judged or written together.
type IndexReport struct {
	Design Report     `json:"docs-to-code"`
	Code   CodeReport `json:"code-to-docs"`
}

// Text renders both verdicts for a person, the design-doc index first. It ends
// in a newline, because each half does.
func (r IndexReport) Text() string {
	return r.Design.Text() + r.Code.Text()
}

// check judges both files and the anchors, and answers one verdict for all
// three.
//
// A stale anchor or an unproven claim answers 1: a pointer nobody can follow
// is a defect in a page, and no generator repairs it. A file that differs from
// its rendering answers StaleExit (3), so a caller can tell drift that `write`
// repairs from a defect it does not. A tree without ai/ answers 1, the
// generator failure code. An unreadable tree answers 2.
func check() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	design, err := Check(tree)
	if err != nil {
		return nil, generatorFailure(err)
	}
	code, err := checkCodeIndexFile(tree)
	if err != nil {
		return nil, generatorFailure(err)
	}

	report := IndexReport{Design: design, Code: code}
	if len(code.Stale) > 0 || len(code.Claims) > 0 {
		return report, 1
	}
	if design.Stale || code.FileStale {
		return report, StaleExit
	}
	return report, 0
}

// write regenerates both files from the tree.
func write() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	design, err := Update(tree)
	if err != nil {
		return nil, generatorFailure(err)
	}
	code, err := UpdateCodeIndex(tree)
	if err != nil {
		return nil, generatorFailure(err)
	}
	return IndexReport{Design: design, Code: code}, 0
}

// generatorFailure reports err and answers its exit code: 1 when the tree
// holds no ai/ directory, 2 when the tree could not be read or written.
func generatorFailure(err error) int {
	leaction.ReportError(err)
	if errors.Is(err, ErrNoAIDir) {
		return 1
	}
	return 2
}

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le doc index` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }
