// Design: docs/architecture/site-facts.md -- "Size history", what the check proves
// Related: update.go -- the history resolution and the refusals the check shares with the update
// Related: measure.go -- the re-measurement of the newest recorded day
// Related: actions.go -- the check verb that runs this
//
// The Check Semantics of
// plan/post-release/engineering/spec-activity-codebase-growth.md: whether the
// committed size history describes HEAD's first-parent history. The check
// writes nothing. It proves the file's contract and line arithmetic on every
// day (ReadHistory), the rules, every recorded commit, the absence of a gap,
// and the newest recorded day in full by measuring it again. An older day's
// counts are proven by a whole-history run, whose output is deterministic, so
// an altered figure shows as a diff of the committed file.

package sitehistory

import (
	"context"
	"errors"
	"io/fs"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// checkTimeout bounds one check run: resolving HEAD, listing its history, and
// measuring at most two trees, the newest recorded day's and its baseline's.
// A throwaway prototype measured the whole history (327 days, about 159,000
// unique blobs) in 84 seconds on the development VM, and two trees are a small
// part of that work. Ten minutes leaves room for a much slower machine and a
// much larger tree, and still ends a run whose git process stopped answering.
const checkTimeout = 10 * time.Minute

// commandCheck is the check's own command line, which a refusal of a question
// that was never put names as the line to run again.
const commandCheck = "./le site history check"

// checked is what a check that passed answers: the day and commit the file
// describes the history through, and how far HEAD has moved past them. The lag
// is information and never fails the check: new commits must not turn a gate
// red.
type checked struct {
	// File is the history file judged, relative to the checkout.
	File string `json:"file"`
	// NewestDay is the file's newest recorded day.
	NewestDay Date `json:"newest-day"`
	// Commit is the commit the file records for NewestDay, the one the check
	// measured again.
	Commit string `json:"commit"`
	// Days counts the file's recorded days.
	Days int `json:"days"`
	// LagCommits counts the commits of HEAD's first-parent history after
	// Commit, on NewestDay or later.
	LagCommits int `json:"lag-commits"`
	// LagDays counts the days of HEAD's first-parent history after NewestDay.
	LagDays int `json:"lag-days"`
}

// Text renders the answer for a person. It ends in a newline. Every pipe
// operator bypasses it and reads the fields.
func (c checked) Text() string {
	var text textbuf.Buffer
	text.Str(c.File).Str(" describes HEAD's first-parent history through ").Str(c.NewestDay.String())
	text.Str(", at commit ").Str(c.Commit).Str(", over ")
	writeCount(&text, c.Days, "recorded day")
	text.Str("\nlag: ")
	writeCount(&text, c.LagCommits, "commit")
	text.Str(" after the recorded commit, ")
	writeCount(&text, c.LagDays, "history day")
	text.Str(" after ").Str(c.NewestDay.String()).Byte('\n')
	if c.LagCommits > 0 {
		text.Str("  ").Str(repairExtend).Str(" measures them; the check does not need it\n")
	}
	return text.String()
}

// writeCount writes count and noun, the noun in the plural unless count is one.
func writeCount(text *textbuf.Buffer, count int, noun string) {
	text.Int(int64(count)).Byte(' ').Str(noun)
	if count != 1 {
		text.Byte('s')
	}
}

// unjudged marks a refusal of a question that was never put: the history or
// one of its objects could not be read, so the file was neither proven nor
// refused. check answers 2 for it, and 1 for every other refusal.
type unjudged struct {
	// cause is the refusal, an operator error that names its own repair.
	cause error
}

// Error answers the cause's text.
func (u unjudged) Error() string {
	return u.cause.Error()
}

// Unwrap answers the cause, so the operator error it carries stays reachable.
func (u unjudged) Unwrap() error {
	return u.cause
}

// checkHistory judges the history file under root against HEAD's first-parent
// history and writes nothing. It refuses, naming the day, the section where
// there is one, both values where there are two, and the command that repairs
// it: a file ReadHistory refuses (absent, malformed, an unknown field, broken
// arithmetic), other rules or another section list, a recorded day that does
// not describe the history, a skipped day, and a newest day whose
// re-measurement differs. A refusal wrapped in unjudged is a question that was
// never put.
func checkHistory(ctx context.Context, root string) (checked, error) {
	line, err := resolveHistory(ctx, root, commandCheck)
	if err != nil {
		return checked{}, unjudged{cause: err}
	}
	file, err := ReadHistory(root)
	if err != nil {
		return checked{}, readRefusal(err)
	}
	if file.Rules() != RulesIdentifier() {
		return checked{}, otherRules(file.Rules(), "its days do not state what this tool counts today")
	}
	if !slices.Equal(file.sections, Sections()) {
		return checked{}, otherSections()
	}
	newestIndex, err := judgeRecorded(file.days, line)
	if err != nil {
		return checked{}, err
	}
	newest := file.days[len(file.days)-1]
	if err := remeasureNewest(ctx, root, file.days, line); err != nil {
		return checked{}, err
	}
	// judgeRecorded proved newest.Commit is on the history, so Index finds it.
	return checked{
		File:       HistoryFile,
		NewestDay:  newest.Date,
		Commit:     newest.Commit,
		Days:       len(file.days),
		LagCommits: len(line.commits) - 1 - slices.Index(line.commits, newest.Commit),
		LagDays:    len(line.days) - 1 - newestIndex,
	}, nil
}

// readRefusal answers ReadHistory's refusal as the check reads it. A file the
// disk would not hand over leaves the question unput; a file that is absent or
// breaks its contract does not describe the history.
func readRefusal(err error) error {
	if _, onDisk := errors.AsType[*fs.PathError](err); onDisk {
		return unjudged{cause: err}
	}
	return err
}

// otherSections is the refusal of a file whose section list is not the
// tool's, under the tool's own rules identifier.
func otherSections() error {
	return opererr.New(HistoryFile+" lists sections other than the section table of rules "+RulesIdentifier(),
		"the rules identifier names one section table, and the activity page labels its figures from the file's list, so the file was edited by hand",
		repairWhole)
}

// judgeRecorded proves every recorded day describes the history line and
// answers the index in line.days of the newest one. Each recorded commit is on
// the history with its record's day as its day (keptDayIndex), every history
// day from the first to the newest recorded one is recorded, and every recorded
// day older than the newest records its day's last commit. The newest may
// record an earlier commit of its day: commits made later that day are lag the
// default update measures. days MUST hold at least one day, as NewHistory
// guarantees.
func judgeRecorded(days []DayRecord, line resolvedHistory) (int, error) {
	last := len(days) - 1
	newest := days[last].Date
	for position, day := range days[:last] {
		index, err := recordedIndex(day, position, newest, line)
		if err != nil {
			return 0, err
		}
		if day.Commit != line.days[index].Commit {
			return 0, notLastCommit(day, line.days[index], newest)
		}
	}
	return recordedIndex(days[last], last, newest, line)
}

// recordedIndex answers the index in line.days of the day the record at
// position describes, and refuses a record keptDayIndex refuses or one that
// leaves a history day before it unrecorded.
func recordedIndex(day DayRecord, position int, newest Date, line resolvedHistory) (int, error) {
	index, err := keptDayIndex(day, line)
	if err != nil {
		return 0, err
	}
	// Recorded days ascend and each falls on its own history day, so index is
	// never below position. Above it, the history day at position is one no
	// record describes.
	if index > position {
		return 0, skippedDay(line.days[position], newest)
	}
	return index, nil
}

// remeasureNewest measures the newest recorded day again at the commit the
// file records for it, never at its day's current last commit: the commit that
// lands the file is itself a later commit of that day, and its tree holds the
// file, so following the day forward would refuse the commit that satisfied
// the check. The baseline is the recorded day before it at its recorded
// commit, which judgeRecorded proved is that day's last. A file of one day has
// no baseline: judgeRecorded proved its day is the history's first, which
// gains its whole tree against an empty one, as the update measured it.
func remeasureNewest(ctx context.Context, root string, days []DayRecord, line resolvedHistory) error {
	newest := days[len(days)-1]
	var baseline *Day
	if len(days) > 1 {
		previous := days[len(days)-2]
		baseline = &Day{Date: previous.Date, Commit: previous.Commit, Tree: line.trees[previous.Commit]}
	}
	target := Day{Date: newest.Date, Commit: newest.Commit, Tree: line.trees[newest.Commit]}
	measured, err := measureDays(ctx, root, commandCheck, baseline, []Day{target})
	if err != nil {
		return unjudged{cause: err}
	}
	return compareRecord(newest, measured[0])
}

// namedCount is one count of a section's values and the name the file and a
// refusal give it.
type namedCount struct {
	name  string
	count uint64
}

// sectionCounts answers every count of a section's values, in the file's
// order.
func sectionCounts(values SectionValues) [7]namedCount {
	return [7]namedCount{
		{"files", values.Files}, {"lines", values.Lines}, {"gained", values.Gained}, {"lost", values.Lost},
		{"go code", values.Go.Code}, {"go blank", values.Go.Blank}, {"go comment", values.Go.Comment},
	}
}

// compareRecord refuses the first count of the measured record that differs
// from the recorded one: the module count, then each section of the table in
// order, field by field. A section a record leaves out held nothing and
// changed nothing, so it compares as the zero values.
func compareRecord(recorded, measured DayRecord) error {
	day := "recorded day " + recorded.Date.String()
	if recorded.Modules != measured.Modules {
		return remeasuredDiffers(day, recorded.Commit, namedCount{"modules", recorded.Modules}, measured.Modules)
	}
	for _, section := range Sections() {
		before := sectionCounts(recorded.Sections[section.ID])
		after := sectionCounts(measured.Sections[section.ID])
		for field := range before {
			if before[field].count != after[field].count {
				return remeasuredDiffers(day+" section "+section.ID, recorded.Commit, before[field], after[field].count)
			}
		}
	}
	return nil
}

// remeasuredDiffers is the refusal of a newest recorded day, named by subject,
// whose recorded count differs from the count measured again at commit. Only
// a whole-history run repairs it: the figure was edited by hand or written by
// a defective run, and an older day, which the check does not measure, may be
// wrong the same way.
func remeasuredDiffers(subject, commit string, recorded namedCount, measured uint64) error {
	var problem textbuf.Buffer
	problem.Str(subject).Str(" records ").Str(recorded.name).Byte(' ').Uint(recorded.count)
	problem.Str(", but its commit ").Str(commit).Str(" measures ").Str(recorded.name).Byte(' ').Uint(measured)
	return opererr.New(problem.String(),
		"the newest recorded day is measured again at the commit the file records, so a figure that differs was edited by hand or written by a defective run, and older days, which the check does not measure, may be wrong the same way",
		repairWhole)
}
