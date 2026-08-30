// Design: docs/architecture/core-design.md -- the wiki-index area, as one command
//
// actions.go is the command surface. `le site wiki update` derives the index
// from a wiki checkout and writes the committed file; `le site wiki check`
// reports what the committed file and the checkout disagree about.
//
// The pair is the shape `le site facts` already has, and for the same reason: a
// generated file nobody gates goes stale in silence.
//
// Related: index.go -- the derivation and the file format.
package sitewiki

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

// area is the name this command is typed as.
const area = "site wiki"

// The keywords this area accepts. A value is accepted only after its keyword,
// so a path can never occupy a verb position.
const (
	keywordWiki    = "wiki"
	keywordBaseURL = "base-url"
)

var actions = leaction.New(area,
	leaction.Action{Verb: "update",
		Why:        "derive the wiki page index from a wiki checkout and write website/data/wiki.json, the file the site build reads instead of opening the wiki",
		Writes:     true,
		Parameters: []leaction.Parameter{{Keyword: keywordWiki, Value: "directory"}, {Keyword: keywordBaseURL, Value: "url"}},
		AnswerArgs: runUpdate},
	leaction.Action{Verb: "check",
		Why:        "report whether the committed wiki index still states what the wiki checkout holds",
		Parameters: []leaction.Parameter{{Keyword: keywordWiki, Value: "directory"}},
		AnswerArgs: runCheck},
)

// Actions answers the command surface as data, so the listing, the help line
// and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le site wiki` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// written is what the update answers: the file it wrote and what went into it.
type written struct {
	File     string     `json:"file"`
	BaseURL  string     `json:"base-url"`
	Groups   int        `json:"groups"`
	Pages    int        `json:"pages"`
	Unlisted []Unlisted `json:"unlisted,omitempty"`
}

// report is what the check answers.
type report struct {
	File  string `json:"file"`
	Wiki  string `json:"wiki"`
	Stale bool   `json:"stale"`
	// Fix names the action that makes a stale file current, so the answer
	// carries its own next step.
	Fix string `json:"fix,omitempty"`
}

// runUpdate is `le site wiki update`.
func runUpdate(arguments leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	wikiRoot, err := wikiCheckout(root, arguments[keywordWiki])
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	index, err := Derive(wikiRoot, arguments[keywordBaseURL])
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	path, err := Write(root, index)
	if err != nil {
		leaction.ReportError(err)
		return nil, 1
	}
	return written{
		File: path, BaseURL: index.BaseURL, Groups: len(index.Groups),
		Pages: index.PageCount(), Unlisted: index.Unlisted,
	}, 0
}

// runCheck is `le site wiki check`.
//
// A stale file answers 1 and a checkout that could not be judged answers 2, so
// a caller reads "the index is out of date" apart from "the question was never
// put". A machine without the wiki beside it gets the second answer: the site
// build needs no wiki, and this check does.
func runCheck(arguments leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	wikiRoot, err := wikiCheckout(root, arguments[keywordWiki])
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	index, err := Derive(wikiRoot, committedBaseURL(root))
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	derived, err := Marshal(index)
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	path := filepath.Join(root, filepath.FromSlash(IndexFile))
	committed, err := os.ReadFile(path) //nolint:gosec // the checkout this action was pointed at
	if err != nil && !os.IsNotExist(err) {
		leaction.ReportError(err)
		return nil, 2
	}
	answer := report{File: path, Wiki: wikiRoot, Stale: !bytes.Equal(committed, derived)}
	if !answer.Stale {
		return answer, 0
	}
	answer.Fix = "le site wiki update"
	return answer, 1
}

// committedBaseURL answers the base URL the committed index states, so a check
// judges the page list rather than reporting the default as a difference.
func committedBaseURL(root string) string {
	index, err := Read(root)
	if err != nil {
		return ""
	}
	return index.BaseURL
}

// wikiCheckout answers the wiki checkout this run reads.
//
// The default is the sibling directory, which is where every machine that has
// the wiki keeps it. An absent checkout is an error rather than an empty index:
// writing an index of no pages over the committed one would publish the wiki as
// if it were empty.
func wikiCheckout(root, named string) (string, error) {
	path := named
	if path == "" {
		path = filepath.Join(filepath.Dir(root), "wiki")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(filepath.Join(absolute, sidebarFile))
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("no wiki checkout at %s: it must hold %s", absolute, sidebarFile)
	}
	return absolute, nil
}
