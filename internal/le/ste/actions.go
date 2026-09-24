// Design: docs/architecture/core-design.md -- the ste area
// Overview: ste.go -- the checker behind each action
//
// The action table owns dispatch, listing, help, and write metadata, and it
// owns the scoped `file <path>` form of check as well. Every word of this
// area's grammar is declared in the table below, so the manifest publishes it
// and one parser reads it.
package ste

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// area is the word this tool is typed as.
const area = "ste"

// actions is the whole command surface.
var actions = leaction.New(area,
	leaction.Action{Verb: "check", Why: "no ASD-STE100 habit grew against HEAD in a changed file. HEAD is the baseline, " +
		"so legacy prose stays until someone rewrites it, no baseline file exists to re-bless, " +
		"and the one way to green is to fix the prose",
		Parameters: []leaction.Parameter{{
			Keyword: fileKeyword, Value: "path",
			// Optional: a check that names no file reads the changed set from
			// git, which is what the gate runs. Repeat: the commit helper names
			// every file of the commit, one keyword for each.
			Requirement: leaction.Optional, Repeat: true,
		}},
		AnswerArgs: checkNamed},
	leaction.Action{Verb: "review", Why: "every ASD-STE100 finding in the tree, with file:line and the fix",
		Answer: reviewAnswer},
	leaction.Action{Verb: "review-changed", Why: "the same findings, over the files this working tree changed",
		Answer: changedAnswer},
)

// Actions answers the command surface as data.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// fileKeyword types the scoped-check value. CLI grammar requires a selector
// kind before a free-form path (ai/rules/cli.md).
const fileKeyword = "file"

// Answer is the `le ste` command. Every action dispatches through the table,
// which reads the keywords, refuses an option in a value slot, and renders the
// grammar for a help word.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// checkNamed runs the ratchet over the files the invocation named, or over the
// changed set when it named none. Values answers nothing for an absent keyword,
// which is the population a bare check already read.
func checkNamed(args leaction.Arguments) (any, int) {
	return checkAnswer(args.Values(fileKeyword))
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
	report := newCheckReport(growth, examined)
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
//
// The documents are reviewed in parallel and merged in list order, so the page
// and the payload are the same bytes a serial walk writes. Review is a pure
// function of one document, which is what makes the split safe. The whole tree
// is about 11,000 documents and one CPU-minute of pattern matching, so a serial
// walk made every `le ste review` wait a minute.
func reviewFiles(root string, files []string) (any, int) {
	results := reviewEach(root, files)

	var findings []Finding
	reviewed, skipped := 0, 0
	for i := range results {
		result := &results[i]
		if result.err != nil {
			leaction.ReportError(result.err)
			return nil, 1
		}
		switch {
		case !result.present:
		case result.skipReason != "":
			skipped++
		default:
			reviewed++
			findings = append(findings, result.findings...)
		}
	}
	return newReviewReport(findings, reviewed, skipped), 0
}

// documentReview is what reviewing one listed path answered.
type documentReview struct {
	findings   []Finding
	skipReason string
	err        error
	// present is false for a path that vanished since it was listed, and for a
	// path no surface reads.
	present bool
}

// reviewEach reviews every path and answers one result for each, in the order
// of files.
//
// A fixed pool of workers, one for each CPU the runtime may use, reads indices
// from a channel that is closed once every index is queued. Each worker writes
// only its own slots of results, so the slice needs no lock, and wg.Wait orders
// every write before the caller reads.
func reviewEach(root string, files []string) []documentReview {
	results := make([]documentReview, len(files))
	indices := make(chan int)
	var wg sync.WaitGroup
	for range min(runtime.GOMAXPROCS(0), max(len(files), 1)) {
		wg.Go(func() {
			for i := range indices {
				results[i] = reviewDocument(root, files[i])
			}
		})
	}
	for i := range files {
		indices <- i
	}
	close(indices)
	wg.Wait()
	return results
}

// reviewDocument reads and reviews one path.
func reviewDocument(root, rel string) documentReview {
	body, err := readDocument(root, rel)
	if err != nil {
		return documentReview{err: err}
	}
	if body == nil {
		return documentReview{}
	}
	surface, ok := surfaceOf(rel)
	if !ok {
		return documentReview{}
	}
	found, skipReason := Review(rel, string(body), surface)
	return documentReview{findings: found, skipReason: skipReason, present: true}
}
