// Design: docs/architecture/core-design.md -- the ste area, as one command
// Overview: ste.go -- the checker behind each action
//
// actions.go ports the Python area. `le check-docs ze-ste-check` selected one
// GateSet gate. `le ste check` selects one action from the table below. Each
// action retains the Gate's Make target, `--list` reason, and WRITES value. None
// writes because this tool only reads and reports prose.
//
// The dispatch, the listing, the help line and the two refusals live in
// letools/leaction. What stays here is the TABLE, and the ONE argument form
// leaction does not cover.
package ste

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/letools/leaction"
	"github.com/ze-software/ze/letools/lepath"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// area is the word this tool is typed as, and the prefix leaction removes from
// each gate name to derive the verb.
const area = "ste"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{
		Gate: "ze-ste-check",
		Why: "no ASD-STE100 habit grew against HEAD in a changed file. HEAD is the baseline, " +
			"so legacy prose stays until someone rewrites it, no baseline file exists to re-bless, " +
			"and the one way to green is to fix the prose",
		Answer: func() (any, int) { return checkAnswer(nil) },
	},
	leaction.Action{
		Gate:   "ze-ste-review",
		Why:    "every ASD-STE100 finding in the tree, with file:line and the fix",
		Answer: reviewAnswer,
	},
	leaction.Action{
		Gate:   "ze-ste-review-changed",
		Why:    "the same findings, over the files this working tree changed",
		Answer: changedAnswer,
	},
)

// Gates answers the Make targets this area claims, which is what the census
// counts and what register.go declares.
func Gates() []string { return actions.Gates() }

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// fileKeyword types the scoped-gate value. CLI grammar requires a selector kind
// before a free-form value. A path is such a value (ai/rules/cli.md).
const fileKeyword = "file"

// Answer is the `le ste` command.
//
// Only `check` accepts values. It gates named files from ONE commit, as the
// commit helper requires. Several sessions share this checkout, so working-tree
// attribution is incorrect. All other actions go to leaction, which refuses
// values after verbs that accept none.
func Answer(args []string) (any, int) {
	if len(args) > 1 && args[0] == "check" {
		named, err := namedFiles(args[1:])
		if err != nil {
			leaction.ReportError(err)
			return nil, 1
		}
		return checkAnswer(named)
	}
	return actions.Answer(args)
}

// namedFiles reads `file <path>` repeated, and refuses anything else.
//
// Argument count is the bound because each keyword consumes one later word. A
// keyword without a value is a refusal, not a default file set for the gate.
func namedFiles(args []string) ([]string, error) {
	var named []string
	for index := 0; index < len(args); index++ {
		if args[index] != fileKeyword {
			return nil, unknownKeyword(args[index])
		}
		if index+1 >= len(args) {
			return nil, missingValue()
		}
		named = append(named, args[index+1])
		index++
	}
	return named, nil
}

// unknownKeyword refuses a word this action does not take. A path in an untyped
// positional slot is what the CLI rule bans, so the keyword is named rather
// than guessed at.
func unknownKeyword(got string) error {
	var tb textbuf.Buffer
	return errors.New(tb.Str("unknown keyword ").Quoted(got).
		Str("; say file <path>, once for each file of the commit").String())
}

// missingValue refuses a `file` keyword with nothing after it.
func missingValue() error {
	return errors.New("the keyword \"file\" needs a path after it")
}

// readDocument answers the bytes of one document, or nil when the path has
// vanished since it was listed.
func readDocument(root, rel string) ([]byte, error) {
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))) // #nosec G304 -- a path this package listed inside the checkout
	if err == nil {
		return body, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	var tb textbuf.Buffer
	return nil, errors.Join(
		errors.New(tb.Str("ste: cannot read ").Str(rel).
			Str(", so no habit in it can be counted").String()), err)
}

// checkAnswer runs the ratchet. No habit can grow in a file changed by this
// working tree.
func checkAnswer(named []string) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}

	growth, examined, err := Ratchet(root, named)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	report := NewGateReport(growth, examined)
	return report, report.Code()
}

// reviewAnswer reads every writing surface in the tree.
func reviewAnswer() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	files, err := DefaultFiles(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return reviewFiles(root, files)
}

// changedAnswer reads the writing surfaces this working tree changed.
func changedAnswer() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	files, err := ChangedFiles(root)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return reviewFiles(root, files)
}

// reviewFiles reviews a named population and answers the report.
//
// A discovered file can disappear between listing and reading because spec
// closure deletes files in this shared checkout. This is not a tool failure, so
// the review skips an absent file. A PRESENT but unreadable file is an error.
// Otherwise, unreadable documents would lower the finding count and appear to
// pass.
func reviewFiles(root string, files []string) (any, int) {
	var findings []Finding
	reviewed, skipped := 0, 0

	for _, rel := range files {
		body, err := readDocument(root, rel)
		if err != nil {
			leaction.ReportError(err)
			return nil, 1
		}
		if body == nil {
			continue
		}
		surface, ok := surfaceOf(rel)
		if !ok {
			continue
		}
		found, skipReason := Review(rel, string(body), surface)
		if skipReason != "" {
			skipped++
			continue
		}
		reviewed++
		findings = append(findings, found...)
	}

	return NewReviewReport(findings, reviewed, skipped), 0
}
