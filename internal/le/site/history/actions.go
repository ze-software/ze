// Design: docs/architecture/core-design.md -- the size-history area, as one command
// Related: file.go -- the file update writes and check reads
// Related: days.go -- the Date a range bound is read into
//
// actions.go is the command surface: `le site history update` measures the
// repository's size per day into the committed file, and `le site history
// check` judges that file against the history it describes. The pair is the
// shape `le site facts update` and `le site facts check` already have.
//
// The dispatch, the listing, the help line and the parse refusals live in
// internal/le/le/action. What stays here is the table and the keyword grammar
// of "Update Semantics" in
// plan/post-release/engineering/spec-activity-codebase-growth.md.

package sitehistory

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/core/opererr"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// area is the name this command is typed as.
const area = "site history"

const (
	keywordFrom = "from"
	keywordTo   = "to"
	keywordAll  = "all"
	verbUpdate  = "update"
	verbCheck   = "check"
)

// actionTable is the whole command surface. It is built on each use rather
// than held in a variable, because the update body refuses through the table
// it belongs to, and a variable naming itself is an initialization cycle.
func actionTable() leaction.Area {
	return leaction.New(area,
		leaction.Action{Verb: verbUpdate, Summary: "measure repository size per day into the history", Why: "measure, from committed git objects only, the size of every section of the repository on each day of HEAD's first-parent history, at that day's last commit, and write website/data/repo-history.json, the file the activity page graphs; with no keyword it extends the file from its newest recorded day, from and to measure a range of days, and all measures the whole history and replaces the file", Writes: true,
			Parameters: []leaction.Parameter{
				{Keyword: keywordFrom, Kind: leaction.KindString, Help: "the first day to measure, as YYYY-MM-DD in UTC", Value: "day", Requirement: leaction.Optional},
				{Keyword: keywordTo, Kind: leaction.KindString, Help: "the last day to measure, as YYYY-MM-DD in UTC", Value: "day", Requirement: leaction.Optional},
				{Keyword: keywordAll, Kind: leaction.KindSwitch, Help: "measure the whole history and replace the file"},
			}, AnswerArgs: runUpdate},
		leaction.Action{Verb: verbCheck, Summary: "check the size history describes HEAD's history", Why: "judge website/data/repo-history.json against HEAD's first-parent history without writing: its contract, its rules identifier, its recorded commits, no gap between its days, its line arithmetic, and its newest day re-measured at the commit it records; commits after that one are reported as lag and never fail",
			Answer: runCheck},
	)
}

// Actions answers the command surface as data, so the listing, the Subs line
// help renders, and the test that checks them all read one table.
func Actions() leaction.List {
	return actionTable().Actions()
}

// Subs is the one-line hint help renders under the command.
func Subs() string {
	return actionTable().Subs()
}

// Answer is the `le site history` command.
func Answer(args []string) (any, int) {
	return actionTable().Answer(args)
}

// updateRequest is what one `site history update` line asks for: exactly one
// of extendRequest, rangeRequest and wholeRequest, which parseUpdate answers.
type updateRequest interface {
	// span names the days the request measures, for the answer and its
	// refusals.
	span() string
}

// extendRequest is the line with no keyword: from the newest recorded day,
// re-measured at that day's current last commit, to the newest day of the
// history, or the whole history when no file exists yet.
type extendRequest struct{}

// span names the days an extension measures.
func (extendRequest) span() string {
	return "the days from the newest recorded day to the newest day of the history"
}

// rangeRequest is a line naming from, to or both: those days inclusive.
type rangeRequest struct {
	// from is the first day to measure. Nil is an open end, the first day of
	// the history.
	from *Date
	// to is the last day to measure. Nil is an open end, the newest day of the
	// history.
	to *Date
}

// span names the days a range measures, an open end by what it reaches.
func (r rangeRequest) span() string {
	first := "the first day of the history"
	if r.from != nil {
		first = r.from.String()
	}
	last := "the newest day of the history"
	if r.to != nil {
		last = r.to.String()
	}
	return "the days from " + first + " to " + last
}

// wholeRequest is the line naming all: the whole history, replacing the file.
type wholeRequest struct{}

// span names the days a whole-history run measures.
func (wholeRequest) span() string {
	return "the whole history"
}

// parseUpdate reads the keyword grammar of `site history update`. It refuses
// all beside from or to, a day not written YYYY-MM-DD, and from after to.
// Every other refusal of "Update Semantics" needs the history and the file,
// so the update body owns it.
func parseUpdate(arguments leaction.Arguments) (updateRequest, error) {
	hasFrom := arguments.Has(keywordFrom)
	hasTo := arguments.Has(keywordTo)
	if arguments.Has(keywordAll) {
		if hasFrom {
			return nil, wholeWithBound(keywordFrom)
		}
		if hasTo {
			return nil, wholeWithBound(keywordTo)
		}
		return wholeRequest{}, nil
	}
	if !hasFrom && !hasTo {
		return extendRequest{}, nil
	}

	var request rangeRequest
	if hasFrom {
		day, err := parseBound(keywordFrom, arguments.One(keywordFrom))
		if err != nil {
			return nil, err
		}
		request.from = &day
	}
	if hasTo {
		day, err := parseBound(keywordTo, arguments.One(keywordTo))
		if err != nil {
			return nil, err
		}
		request.to = &day
	}
	// An open end cannot be inverted.
	if request.from == nil {
		return request, nil
	}
	if request.to == nil {
		return request, nil
	}
	if request.from.Compare(*request.to) > 0 {
		return nil, opererr.New("from "+request.from.String()+" is after to "+request.to.String(),
			"a range runs from its first day to its last, so this one holds no day",
			"./le site history update from "+request.to.String()+" to "+request.from.String())
	}
	return request, nil
}

// wholeWithBound is the refusal of all beside a range bound.
func wholeWithBound(keyword string) error {
	return opererr.New("all and "+keyword+" cannot be given together",
		"all measures the whole history and replaces the file, so a range bound contradicts it",
		"./le site history update all")
}

// parseBound reads the day one range keyword names.
func parseBound(keyword, raw string) (Date, error) {
	day, err := ParseDate(raw)
	if err != nil {
		return Date{}, opererr.Wrap(err, "the "+keyword+" day "+raw+" cannot be read",
			"a range bound names one UTC day of the history",
			"./le site history update "+keyword+" YYYY-MM-DD")
	}
	return day, nil
}

// runUpdate is `le site history update`. The grammar is read before the
// checkout is looked up, so a mistyped line is refused without reading it.
func runUpdate(arguments leaction.Arguments) (any, int) {
	request, err := parseUpdate(arguments)
	if err != nil {
		return nil, actionTable().Refuse(verbUpdate, err)
	}
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, "./le site history update cannot find the Ze checkout",
			"the size history is measured from the committed history of the checkout it is written into",
			"cd into the Ze checkout, or set ZE_REPO_ROOT to it, then run the same ./le site history update again"))
		return nil, 1
	}
	return update(root, request)
}

// update is runUpdate with the checkout named, which is what a test can call
// against a fixture rather than against the checkout it runs in. It answers
// what it wrote, or reports the refusal and answers 1 with the committed file
// left as it was (updateHistory, update.go).
func update(root string, request updateRequest) (any, int) {
	ctx, cancel := context.WithTimeout(context.Background(), updateTimeout)
	defer cancel()

	answer, err := updateHistory(ctx, root, request)
	if err != nil {
		leaction.ReportError(operatorError(repairExtend, err))
		return nil, 1
	}
	return answer, 0
}

// runCheck is `le site history check`.
//
// A file that does not describe the history answers 1 and a checkout that
// could not be judged answers 2, so a caller reads "the history is wrong"
// apart from "the question was never put", as `le site facts check` does.
func runCheck() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		leaction.ReportError(opererr.Wrap(err, commandCheck+" cannot find the Ze checkout",
			"the size history is judged against the committed history of the checkout that holds it",
			"cd into the Ze checkout, or set ZE_REPO_ROOT to it, then run "+commandCheck+" again"))
		return nil, 2
	}
	return check(root)
}

// check is runCheck with the checkout named, which is what a test can call
// against a fixture rather than against the checkout it runs in. It answers
// what the file describes and the lag behind HEAD, which never fails, or
// reports the refusal and answers 1 for a file that does not describe the
// history and 2 for a question that was never put (checkHistory, check.go).
func check(root string) (any, int) {
	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()

	answer, err := checkHistory(ctx, root)
	if err != nil {
		leaction.ReportError(operatorError(commandCheck, err))
		return nil, refusalCode(err)
	}
	return answer, 0
}

// refusalCode answers the exit code of a check refusal: 2 for a question that
// was never put (unjudged), 1 for a file that does not describe the history.
func refusalCode(err error) int {
	if _, notJudged := errors.AsType[unjudged](err); notJudged {
		return 2
	}
	return 1
}

// operatorError answers err as the operator error the writer is handed. Every
// refusal this package builds is one, and it reaches the writer as it was
// built, so the reader still meets its own repair on the last line rather than
// a generic one wrapped around it. An error no refusal built is a defect of
// this package, and is wrapped with the run that finds it. command is the line
// that met it.
func operatorError(command string, err error) *opererr.Error {
	if refusal, built := errors.AsType[*opererr.Error](err); built {
		return refusal
	}
	return opererr.Wrap(err, command+" stopped on an error that names no repair",
		"every refusal of the size history names the command that repairs it, so an error without one comes from a defect of internal/le/site/history",
		"./le test unit pkg path ./internal/le/site/history/")
}
