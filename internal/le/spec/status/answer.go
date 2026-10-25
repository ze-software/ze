// Design: docs/architecture/core-design.md -- the spec inventory as a command

package specstatus

import (
	"context"
	"errors"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

type closureAction uint8

const (
	closureUnspecified closureAction = iota
	closureList
	closureCheck
)

// The verbs and keywords of `le spec status`. The closure spellings are the
// ones the Stop hook and ze-close type, kept until the rename phase of
// plan/post-release/engineering/spec-le-cli-usability.md.
const (
	name           = "spec status"
	verbReport     = "report"
	verbClosure    = "closure"
	keywordList    = "list"
	keywordCheck   = "check"
	keywordSpec    = "spec"
	closureGrammar = "spec status closure takes list, or check spec <name>"
)

// table declares the whole command surface. It is built on each call because
// the closure body refuses through it, and a package-level table that its own
// body names is an initialization cycle.
func table() leaction.Area {
	return leaction.New(name,
		leaction.Action{
			Verb:   verbReport,
			Why:    "the spec inventory: release bucket, status and stale-skeleton flag for every spec",
			Answer: answerReport,
		},
		leaction.Action{
			Verb: verbClosure,
			Why:  "the closure detector: every spec whose closure is owed, or whether one spec may close",
			Parameters: []leaction.Parameter{
				{Keyword: keywordList, Kind: leaction.KindSwitch, Help: "list every spec whose closure is owed"},
				{Keyword: keywordCheck, Kind: leaction.KindSwitch, Help: "check the one spec that spec names"},
				{
					Keyword: keywordSpec, Kind: leaction.KindLabel, Value: "name",
					Help: "the spec check judges", Requirement: leaction.Optional,
				},
			},
			AnswerArgs: answerClosure,
		},
	)
}

// Answer is the command. Its bare form is the inventory, which hooks and
// ze-status call bare; every other line is the table's.
func Answer(args []string) (any, int) {
	if len(args) == 0 {
		return table().Answer([]string{verbReport})
	}
	return table().Answer(args)
}

// answerReport is the inventory. It REPORTS; it does not gate.
func answerReport() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		reportError(err)
		return nil, 3
	}
	inventory, err := Collect(context.Background(), root, time.Now(), warnToStderr)
	if err != nil {
		reportError(err)
		return nil, 3
	}
	// A tree full of blocked specs is not a failure, so the code is 0 whenever
	// the population was read.
	return inventory, 0
}

// answerClosure runs the closure action the parsed line selects. A line that
// selects none, or both, is refused with the action's usage and 2. A
// checkout or journal it cannot read answers 3, and `check` answers 1 for a
// spec whose closure is blocked: the check ran and found a problem. The
// closure actions preserve the detector's list and single-spec contracts.
func answerClosure(args leaction.Arguments) (any, int) {
	action, spec := closureActionOf(args)
	if action == closureUnspecified {
		return nil, table().Refuse(verbClosure, errors.New(closureGrammar))
	}
	root, err := lepath.Root()
	if err != nil {
		reportError(err)
		return nil, 3
	}
	switch action {
	case closureList:
		report, malformed, err := closureInventory(root)
		reportMalformedJournals(malformed)
		if err != nil {
			reportError(err)
			return nil, 3
		}
		return report, 0
	case closureCheck:
		report, malformed, err := CheckClosure(root, spec)
		reportMalformedJournals(malformed)
		if err != nil {
			reportError(err)
			return nil, 3
		}
		if report.Blocked() {
			return report, 1
		}
		return report, 0
	case closureUnspecified:
		panic("BUG: an unspecified closure action passed its refusal")
	default:
		panic("BUG: invalid spec closure action")
	}
}

// closureActionOf answers the closure action a parsed line selects: `list`
// alone, or `check` with `spec <name>`. Any other line selects none.
func closureActionOf(args leaction.Arguments) (closureAction, string) {
	list, check, spec := args.Has(keywordList), args.Has(keywordCheck), args.Has(keywordSpec)
	if list {
		if check || spec {
			return closureUnspecified, ""
		}
		return closureList, ""
	}
	if check {
		if spec {
			return closureCheck, args.One(keywordSpec)
		}
	}
	return closureUnspecified, ""
}

func reportMalformedJournals(paths []string) {
	for _, path := range paths {
		var tb textbuf.Buffer
		tb.Str("warning: malformed journal row in ").Str(path).
			Str(" (run `./le spec journal report`)\n").StdErr() //nolint:errcheck // CLI output
	}
}

// warnToStderr names a spec the inventory could not parse. It is the answer's
// side channel rather than part of the payload: the record for such a spec is
// still published, carrying the status `unparsed`.
func warnToStderr(line string) {
	var tb textbuf.Buffer
	tb.Str(line).Byte('\n').StdErr() //nolint:errcheck // CLI output
}

// reportError writes one failure line in the spelling every ported le tool uses.
func reportError(err error) {
	var tb textbuf.Buffer
	tb.Str("error: ").Err(err).Byte('\n').StdErr() //nolint:errcheck // CLI output
}
