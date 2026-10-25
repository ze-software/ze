// Design: docs/architecture/core-design.md -- le's native development gates

package speccitation

import (
	"errors"

	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

const name = "spec citation"

// The verbs and keyword of `le spec citation`.
const (
	verbCheck   = "check"
	verbAnchors = "anchors"
	keywordSpec = "spec"
)

// areaDescription is what the gate judges. The area's long help and the check action's Why
// both read it, so the two cannot drift.
const areaDescription = "a spec citing a sibling spec absent on disk fails, unless the" +
	" target is grandfathered in plan/.citation-baseline; a path:line citation" +
	" whose backtick-quoted token drifted off that line warns"

// table declares the whole command surface. It is built on each call because
// the anchors body refuses through it, and a package-level table that its own
// body names is an initialization cycle.
func table() leaction.Area {
	return leaction.New(name,
		leaction.Action{
			Verb:    verbCheck,
			Summary: "check spec citations name real symbols",
			Why:     areaDescription,
			Answer:  answerCheck,
		},
		leaction.Action{
			Verb:    verbAnchors,
			Summary: "list design docs anchored to the spec's files",
			Why:     "the design documents whose source anchors name a file the spec changes",
			Parameters: []leaction.Parameter{{
				Keyword: keywordSpec, Kind: leaction.KindSpec, Value: "bucket/spec-name.md",
				Help: "the spec whose files are audited", Requirement: leaction.Required,
			}},
			AnswerArgs: answerAnchors,
		},
	)
}

// Answer is the `le spec citation` command. A bare line runs the citation
// gate, as the `check` action does: verify stages call it bare, and a bare
// listing would answer them 0 with nothing judged. Every other line is the
// table's.
func Answer(args []string) (any, int) {
	if len(args) == 0 {
		return table().Answer([]string{verbCheck})
	}
	return table().Answer(args)
}

// answerCheck runs the citation gate over the checkout.
func answerCheck() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		reportError(err)
		return nil, 1
	}
	report, err := Scan(root)
	if err != nil {
		reportError(err)
		return nil, 1
	}
	return report, verdict(report)
}

// answerAnchors runs the design-document owner audit for one spec.
func answerAnchors(args leaction.Arguments) (any, int) {
	if !args.Has(keywordSpec) {
		return nil, table().Refuse(verbAnchors, errors.New("spec citation anchors needs spec <bucket>/spec-<name>.md"))
	}
	root, err := lepath.Root()
	if err != nil {
		reportError(err)
		return nil, 1
	}
	report, err := AuditAnchors(root, args.One(keywordSpec))
	if err != nil {
		reportError(err)
		return nil, 2
	}
	if len(report.Owners) > 0 {
		return report, 1
	}
	return report, 0
}

func reportError(err error) {
	var tb textbuf.Buffer
	tb.Str("error: spec-citation: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // CLI output
}
