// Design: docs/contributing/spec-workflow.md -- Thomas's absence, and what starts without his approval
// Related: records.go -- the atomic replace and the record reader
// Related: pending.go -- lists the away state and each start admitted under it
//
// away.go is `le spec design away until <YYYY-MM-DD> words "<text>"` and `le
// spec design return`: from Thomas's terminal only, the first writes
// tmp/spec-design/away.md, the second removes it. The record lapses on its own
// after its until date (plan/release/validation/spec-design-needs-owner-approval.md,
// AC-27, C-4).
package specdesign

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	"github.com/ze-software/ze/internal/le/le/human"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// awayFile is the away record, one per checkout.
const awayFile = "away.md"

// The away record's fields, in order.
const (
	fieldFrom  = "From"
	fieldUntil = "Until"
)

// awayFields are the fields every away record carries.
var awayFields = []string{fieldFrom, fieldUntil, fieldOwnerWords}

// untilKeyword is the away action's date keyword.
const untilKeyword = "until"

// Away is Thomas's recorded absence: from the day he recorded it to the last
// day of it, both inclusive, and what he said.
type Away struct {
	From  string `json:"from"`
	Until string `json:"until"`
	Words string `json:"owner-words"`
}

// awayPath is the away record of the checkout at root.
func awayPath(root string) string {
	return filepath.Join(root, filepath.FromSlash(recordsDir), awayFile)
}

// ActiveAway answers Thomas's absence as it stands on the day now falls on:
// found false when no record exists or its until date is past, so a record
// lapses on its own. A record that cannot be read or parsed is an error
// naming the file, never "not away" and never "away".
func ActiveAway(root string, now time.Time) (away Away, found bool, err error) {
	file := awayPath(root)
	lines, found, err := readRecord(file)
	if err != nil || !found {
		return Away{}, false, err
	}
	repair := "; fix: have Thomas record it again (./le spec design away until <YYYY-MM-DD> words \"<what he said>\") or remove it (./le spec design return)"
	if missing := lines.missingField(awayFields); missing != "" {
		return Away{}, false, &recordError{file: file, problem: "carries no " + missing + " line" + repair}
	}
	until, err := time.ParseInLocation(time.DateOnly, lines.single[fieldUntil], now.Location())
	if err != nil {
		return Away{}, false, &recordError{file: file, problem: "carries an Until that is not YYYY-MM-DD" + repair}
	}
	if until.Before(day(now)) {
		return Away{}, false, nil
	}
	return Away{From: lines.single[fieldFrom], Until: lines.single[fieldUntil], Words: lines.single[fieldOwnerWords]}, true, nil
}

// day is the midnight that starts the day t falls on, in t's location.
func day(t time.Time) time.Time {
	year, month, date := t.Date()
	return time.Date(year, month, date, 0, 0, 0, 0, t.Location())
}

// awayAnswer is the answer of a recorded absence: the dates and the rule it
// admits under.
type awayAnswer struct {
	Away
	Record string `json:"record"`
}

// Text renders the admission rule for Thomas.
func (a awayAnswer) Text() string {
	var tb textbuf.Buffer
	return tb.Str("Thomas away ").Str(a.From).Str(" to ").Str(a.Until).
		Str(": a judged ready fix spec with no departs row starts without approval; ").
		Str("./le spec design pending lists each such start for your return\n").String()
}

// answerAway is `le spec design away until <YYYY-MM-DD> words "<text>"`. The
// terminal gate runs before anything is read or written.
func answerAway(args leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		return nil, reportError(err)
	}
	until := args.One(untilKeyword)
	words := args.One(wordsParameter.Keyword)
	operands := []string{untilKeyword, until, wordsParameter.Keyword, words}
	if err := human.RequireAuthorization(root, "", area+" away", operands); err != nil {
		return nil, refuseAction(err.Error())
	}
	return recordAway(root, until, words, time.Now())
}

// recordAway refuses a missing or past until date and malformed words, then
// replaces the away record with an absence from now's day to until.
func recordAway(root, until, words string, now time.Time) (any, int) {
	if until == "" {
		return nil, refuseAction("refused: away needs the last day of the absence",
			"  why: an absence with no end would admit fix specs without Thomas until somebody remembers to end it",
			"  fix: run ./le spec design away until <YYYY-MM-DD> words \"<what he said>\"")
	}
	last, err := time.ParseInLocation(time.DateOnly, until, now.Location())
	if err != nil {
		return nil, refuseAction("refused: until " + until + " is not a date; fix: give the last day as YYYY-MM-DD, today or later")
	}
	if last.Before(day(now)) {
		return nil, refuseAction("refused: until " + until + " is already over; fix: give the last day of the absence, today or later")
	}
	if problem := wordsProblem(words); problem != "" {
		return nil, refuseAction("refused: " + problem)
	}
	away := Away{From: now.Format(time.DateOnly), Until: until, Words: words}
	var tb textbuf.Buffer
	tb.Str(fieldFrom).Str(": ").Str(away.From).Byte('\n').
		Str(fieldUntil).Str(": ").Str(away.Until).Byte('\n').
		Str(fieldOwnerWords).Str(": ").Str(away.Words).Byte('\n')
	if err := replaceRecord(root, recordsDir, awayFile, tb.String()); err != nil {
		return nil, refuseAction("refused: " + err.Error())
	}
	return awayAnswer{Away: away, Record: recordsDir + "/" + awayFile}, 0
}

// returnAnswer is the answer of `le spec design return`.
type returnAnswer struct {
	// Removed is false when no away record existed.
	Removed bool   `json:"removed"`
	Record  string `json:"record"`
}

// Text renders the return for Thomas.
func (a returnAnswer) Text() string {
	if !a.Removed {
		return "Thomas was not recorded away; every ready spec already needs his approval\n"
	}
	return "Thomas is back: every ready spec again needs his approval before its claim\n"
}

// answerReturn is `le spec design return`. The terminal gate runs before the
// record is touched.
func answerReturn() (any, int) {
	root, err := lepath.Root()
	if err != nil {
		return nil, reportError(err)
	}
	if err := human.RequireAuthorization(root, "", area+" return", nil); err != nil {
		return nil, refuseAction(err.Error())
	}
	answer := returnAnswer{Removed: true, Record: recordsDir + "/" + awayFile}
	err = os.Remove(awayPath(root))
	if errors.Is(err, os.ErrNotExist) {
		answer.Removed = false
		return answer, 0
	}
	if err != nil {
		return nil, refuseAction("refused: remove "+answer.Record+": "+err.Error(), "  fix: repair the file's permissions, then run ./le spec design return again")
	}
	return answer, 0
}
