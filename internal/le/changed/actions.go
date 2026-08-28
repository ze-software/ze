// Design: docs/architecture/testing/verify-freshness-scope.md -- what a scoped run covers
// Overview: changed.go -- the group selection
// Detail: scope.go and selector.go -- the scoped change-set selection
//
// actions.go is the table. The dispatch, listing, help line and refusals are
// internal/le/leaction, which every ported area shares.
//
// `scope` is the native change-set selector. Its values follow closed keywords
// so every free-form token has a declared meaning.

package changed

import (
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/lepath"
)

var actions = leaction.New(area,
	leaction.Action{
		Verb: "groups",
		Why: "the test groups holding a changed .go file, one name per line." +
			" Empty means nothing changed, and a checkout that cannot be read is an error",
		Answer: groupNamesHere,
	},
	leaction.Action{
		Verb:   "group-packages",
		Why:    "the same selection as Go package patterns, which is what a test run takes as arguments",
		Answer: groupPackagesHere,
	},
	leaction.Action{
		Verb: "packages",
		Why: "the packages a scoped verify stage must cover, from the one change-set selector." +
			" Widens to ./... on every route it cannot answer",
		Answer: scopePackagesHere,
	},
	leaction.Action{
		Verb: "scope",
		Why:  "the native scoped-verify selector, including feature tags and reverse-import depth",
		Parameters: []leaction.Parameter{
			{Keyword: "print", Value: "mode"},
			{Keyword: "depth", Value: "n"},
			{Keyword: "paths-from", Value: "path"},
			{Keyword: "drop-log", Value: "path"},
		},
		AnswerArgs: scopeHere,
	},
)

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List { return actions.Actions() }

// Subs is the one-line hint help renders under the command.
func Subs() string { return actions.Subs() }

// Answer is the `le changed` command.
func Answer(args []string) (any, int) { return actions.Answer(args) }

// groupNamesHere answers the group names for the checkout this ran in.
func groupNamesHere() (any, int) {
	selection, code := selectHere()
	if code != 0 {
		return nil, code
	}
	return groupNames{Selection: selection}, 0
}

// groupPackagesHere answers the same selection as package patterns.
func groupPackagesHere() (any, int) {
	selection, code := selectHere()
	if code != 0 {
		return nil, code
	}
	return groupPackages{Selection: selection}, 0
}

// selectHere runs the selection over the checkout this command was run in.
//
// A failure answers 2 instead of 1. This code distinguishes a selection failure
// from an empty selection. The distinction is the purpose of the port. The
// shell answered 0 and nothing for both cases.
func selectHere() (Selection, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return Selection{}, 2
	}
	selection, err := Selector{Root: root}.Select()
	if err != nil {
		leaction.ReportError(err)
		return Selection{}, 2
	}
	return selection, 0
}

// scopePackagesHere answers the scoped-verify package set for this checkout.
//
// It answers 0 after it widens because widening is a valid answer. The recipe
// must run over `./...` instead of stopping.
func scopePackagesHere() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return newScope(root).Resolve(nil)
}

// scopeHere runs the selector directly. It deliberately does not call
// newScope: the selector action always computes a fresh change set, while the
// packages action may consume a verify run's precomputed package file.
func scopeHere(arguments leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}

	legacy := make([]string, 0, len(arguments))
	for _, keyword := range []string{"print", "depth", "paths-from", "drop-log"} {
		if value, present := arguments[keyword]; present {
			legacy = append(legacy, "--"+keyword+"="+value)
		}
	}
	return (Scope{Root: root}).Resolve(legacy)
}
