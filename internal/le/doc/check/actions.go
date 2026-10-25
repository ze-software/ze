// Design: docs/architecture/core-design.md -- native documentation verifier actions
// Detail: links.go -- path, citation, suppression, and baseline checks.
// Detail: register.go -- root registration and answer shape.
//
// Package doccheck owns the four documentation actions that the verifier runs
// directly. The links action owns its scan. Verify and templ-output call the Go
// packages that already own those compositions. The dispatch, the listing, the
// help page and the refusals live in internal/le/le/action.

package doccheck

import (
	"github.com/ze-software/ze/internal/core/textbuf"
	docwiring "github.com/ze-software/ze/internal/le/doc/wiring"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

const area = "doc check"

// actions declares the whole command surface. No action takes a value: each
// judges the checkout, so the table refuses any word after the verb.
var actions = []leaction.Action{
	{
		Verb:   "links",
		Why:    "instruction links, tracked citations, suppression reasons, and Design references resolve",
		Answer: atRoot(runLinks),
	},
	{
		Verb:   "verify",
		Why:    "the complete ordered documentation gate passes",
		Answer: atRoot(docwiring.Verify),
	},
	{
		Verb:   "templ-output",
		Why:    "generated templ output matches its sources",
		Answer: atRoot(docwiring.TemplOutput),
	},
	{
		Verb:   "retired-commands",
		Why:    "no tracked line names a form the subject-first rename retired",
		Answer: atRoot(runRetiredCheck),
	},
}

// table is the area the declaration above builds.
var table = leaction.New(area, actions...)

// Actions returns the whole command surface from its canonical table.
func Actions() leaction.List { return table.Actions() }

// Subs is the one-line action hint shown by help.
func Subs() string { return table.Subs() }

// Answer is the `le doc check` command. Bare, it answers the action listing.
func Answer(args []string) (any, int) { return table.Answer(args) }

// atRoot binds one action to the checkout it judges. A checkout that cannot be
// found answers 2: the gate did not run, which a caller reads apart from a
// tree that failed it.
func atRoot(run func(string) (any, int)) func() (any, int) {
	return func() (any, int) {
		root, err := lepath.Root()
		if err != nil {
			leaction.ReportError(err)
			return nil, 2
		}
		return run(root)
	}
}

func runLinks(root string) (any, int) {
	report, err := checkLinks(root)
	if err != nil {
		return errorReport{Error: err.Error()}, 2
	}
	if len(report.Errors) > 0 {
		return report, 1
	}
	return report, 0
}

// errorReport is an operational failure that prevented a complete judgement.
type errorReport struct {
	Error string `json:"error"`
}

// Text renders the failure for a person.
func (r errorReport) Text() string {
	var out textbuf.Buffer
	return out.Str("error: ").Str(r.Error).Byte('\n').String()
}
