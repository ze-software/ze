// Design: docs/architecture/site-facts.md -- the committed-facts pattern, its second instance
// Related: measure.go -- the measurement of each day this merges
// Related: file.go -- the file this reads and writes
// Related: actions.go -- the keyword grammar that names the request
//
// The Update Semantics of
// plan/post-release/engineering/spec-activity-codebase-growth.md: which days
// one `le site history update` line measures, what it refuses before it
// measures, and the merge it writes. Every refusal leaves the committed file
// as it was: the file is written once, last, after the merged history passed
// NewHistory.

package sitehistory

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/repo/objects"
)

// updateTimeout bounds one update run: resolving HEAD, listing its history and
// measuring every requested day. A throwaway prototype measured the whole
// history (327 days, about 159,000 unique blobs, 2.1 GB) in 84 seconds on the
// development VM. Thirty minutes leaves twenty times that for a slower machine
// and a longer history, and still ends a run whose git process stopped
// answering.
const updateTimeout = 30 * time.Minute

// The two repairs a refusal names: the whole-history run, which replaces the
// file, and the default run, which extends it from its newest recorded day.
const (
	repairWhole  = "./le site history update all"
	repairExtend = "./le site history update"
)

// updated is what an update answers: the file it wrote, the days it measured,
// and each section's lines on the newest recorded day.
type updated struct {
	// File is the path written.
	File string `json:"file"`
	// DaysMeasured counts the days measured, the newest recorded day included
	// when it was re-measured.
	DaysMeasured int `json:"days-measured"`
	// FirstDay is the first day measured.
	FirstDay Date `json:"first-day"`
	// LastDay is the last day measured.
	LastDay Date `json:"last-day"`
	// NewestDay is the newest recorded day of the file written.
	NewestDay Date `json:"newest-day"`
	// Sections holds each section's lines on NewestDay, in table order.
	Sections []sectionSize `json:"sections"`
	// TotalLines sums Sections.
	TotalLines uint64 `json:"total-lines"`
}

// sectionSize is one section's lines on the newest recorded day.
type sectionSize struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Lines uint64 `json:"lines"`
}

// Text renders the answer for a person. It ends in a newline. Every pipe
// operator bypasses it and reads the fields.
func (u updated) Text() string {
	var text textbuf.Buffer
	text.Str(u.File).Str(": measured ").Int(int64(u.DaysMeasured)).Str(" day")
	if u.DaysMeasured != 1 {
		text.Byte('s')
	}
	text.Str(", ").Str(u.FirstDay.String()).Str(" to ").Str(u.LastDay.String()).Byte('\n')
	text.Str("lines on ").Str(u.NewestDay.String()).Str(", the newest recorded day:\n")
	for _, section := range u.Sections {
		text.Str("  ").Str(section.Label).Str(": ").Uint(section.Lines).Byte('\n')
	}
	return text.Str("  total: ").Uint(u.TotalLines).Byte('\n').String()
}

// resolvedHistory is HEAD's first-parent history, pinned at one commit: its
// recorded days and, for every commit, the day it falls on and its tree.
type resolvedHistory struct {
	// days is every recorded day, oldest first, as SelectDays answers them.
	days []Day
	// commits holds every commit ID of the history, oldest first.
	commits []string
	// dayOf holds, for every commit of the history, the index in days of the
	// day it falls on.
	dayOf map[string]int
	// trees holds every commit's tree ID.
	trees map[string]string
}

// resolveHistory resolves HEAD once and answers its first-parent history.
// Every later read of the run descends from that one commit ID. command is the
// `./le site history` line that asked, which a refusal names as the line to
// run again once its cause is repaired.
func resolveHistory(ctx context.Context, root, command string) (resolvedHistory, error) {
	head, err := objects.Resolve(ctx, root, "HEAD")
	if err != nil {
		return resolvedHistory{}, opererr.Wrap(err, "cannot resolve HEAD under "+root,
			"the size history measures HEAD's first-parent history, so it needs HEAD to name a commit",
			"run "+command+" in a checkout whose HEAD names a commit")
	}
	commits, err := objects.FirstParents(ctx, root, head)
	if err != nil {
		return resolvedHistory{}, opererr.Wrap(err, "cannot read the first-parent history of "+head,
			"the size history records every day of that history, so a history it cannot read in full cannot be measured or judged",
			"git fetch --unshallow, then run the same "+command+" again")
	}
	days, err := SelectDays(commits)
	if err != nil {
		return resolvedHistory{}, err
	}

	// SelectDays keeps each day's last commit, and commits arrive in the same
	// order, so a commit belongs to the day whose commit it reaches first.
	// Each pass consumes one commit, so the loop is bounded by the history.
	line := resolvedHistory{days: days, commits: make([]string, 0, len(commits)),
		dayOf: make(map[string]int, len(commits)), trees: make(map[string]string, len(commits))}
	index := 0
	for _, commit := range commits {
		line.commits = append(line.commits, commit.ID)
		line.dayOf[commit.ID] = index
		line.trees[commit.ID] = commit.Tree
		if index < len(days) && commit.ID == days[index].Commit {
			index++
		}
	}
	return line, nil
}

// span answers the indices of the first and last recorded days from from to
// to inclusive, a nil bound reaching the first or the newest day, and whether
// the range holds any day.
func (h resolvedHistory) span(from, to *Date) (first, last int, found bool) {
	first, last = -1, -1
	for index, day := range h.days {
		if from != nil && day.Date.Compare(*from) < 0 {
			continue
		}
		if to != nil && day.Date.Compare(*to) > 0 {
			break
		}
		if first < 0 {
			first = index
		}
		last = index
	}
	return first, last, first >= 0
}

// updatePlan is what one update measures and what it keeps.
type updatePlan struct {
	// measured holds the history days to measure, ascending.
	measured []Day
	// baseline is the recorded day immediately before measured[0], at the
	// commit the file records for it, or nil when measured starts the history.
	baseline *Day
	// kept holds the file's days the measured days do not replace, ascending,
	// each record as the file holds it.
	kept []DayRecord
}

// updateHistory is the whole update: resolve, plan, measure, merge, write. It
// answers what it wrote, or a refusal that names its repair; a refusal writes
// nothing.
func updateHistory(ctx context.Context, root string, request updateRequest) (updated, error) {
	line, err := resolveHistory(ctx, root, repairExtend)
	if err != nil {
		return updated{}, err
	}
	plan, err := planUpdate(root, request, line)
	if err != nil {
		return updated{}, err
	}
	measured, err := measureDays(ctx, root, repairExtend, plan.baseline, plan.measured)
	if err != nil {
		return updated{}, err
	}
	history, err := NewHistory(RulesIdentifier(), Sections(), mergeDays(plan.kept, measured))
	if err != nil {
		return updated{}, brokenMerge(err, request, plan)
	}
	path, err := WriteHistory(root, history)
	if err != nil {
		return updated{}, opererr.Wrap(err, "cannot write "+HistoryFile,
			"the measured history reaches the activity page only through that file, and the old file was left as it is",
			"make website/data under "+root+" writable, then run the same ./le site history update again")
	}
	return answerOf(path, measured, history), nil
}

// planUpdate decides what request measures. A whole-history run, and a
// default run with no file yet, measure every history day and keep nothing.
// The default run with a file measures from its newest recorded day, which is
// re-measured at that day's current last commit, to the newest history day.
func planUpdate(root string, request updateRequest, line resolvedHistory) (updatePlan, error) {
	switch request := request.(type) {
	case wholeRequest:
		return updatePlan{measured: line.days}, nil
	case extendRequest:
		file, err := ReadHistory(root)
		if errors.Is(err, ErrNoHistory) {
			return updatePlan{measured: line.days}, nil
		}
		if err != nil {
			return updatePlan{}, err
		}
		newest := file.days[len(file.days)-1].Date
		return planRange(file, &newest, nil, request, line)
	case rangeRequest:
		file, err := ReadHistory(root)
		if errors.Is(err, ErrNoHistory) {
			return updatePlan{}, opererr.New("./le site history update cannot measure "+request.span()+": "+HistoryFile+" does not exist",
				"a range is merged into the recorded history, and nothing is recorded yet, so every day before the range would be missing",
				repairWhole)
		}
		if err != nil {
			return updatePlan{}, err
		}
		return planRange(file, request.from, request.to, request, line)
	}
	return updatePlan{}, errors.New("the update names no request; this is a defect of internal/le/site/history/actions.go parseUpdate")
}

// planRange plans the measurement of the history days from from to to into
// file, and refuses before anything is measured when the merge could not
// describe the history: other rules, no history day in the range, a kept day
// that left the history or is not its day's last commit, and a history day the
// merge would leave unrecorded.
func planRange(file History, from, to *Date, request updateRequest, line resolvedHistory) (updatePlan, error) {
	if file.Rules() != RulesIdentifier() {
		return updatePlan{}, otherRules(file.Rules(), request.span()+" measured now would not add up with the days the file keeps")
	}
	first, last, found := line.span(from, to)
	if !found {
		return updatePlan{}, noHistoryDay(request, line)
	}
	measured := line.days[first : last+1]

	// recorded marks each history day the merge would hold.
	recorded := make([]bool, len(line.days))
	for index := first; index <= last; index++ {
		recorded[index] = true
	}
	kept := make([]DayRecord, 0, len(file.days))
	keptIndex := make([]int, 0, len(file.days))
	for _, day := range file.days {
		if _, replaced := slices.BinarySearchFunc(measured, day.Date, compareDay); replaced {
			continue
		}
		index, err := keptDayIndex(day, line)
		if err != nil {
			return updatePlan{}, err
		}
		recorded[index] = true
		kept = append(kept, day)
		keptIndex = append(keptIndex, index)
	}

	// newest is the merge's newest day: the range's last day or a kept day
	// after it, whichever is later.
	newest := last
	if position := len(keptIndex) - 1; position >= 0 {
		newest = max(newest, keptIndex[position])
	}
	newestRecorded := file.days[len(file.days)-1]
	if missing := slices.Index(recorded[:newest], false); missing >= 0 {
		return updatePlan{}, unrecordedDay(line.days[missing], newestRecorded.Date, request)
	}
	for position, index := range keptIndex {
		if kept[position].Commit == line.days[index].Commit {
			continue
		}
		// The newest day of the merge may record an earlier commit of its day:
		// commits made later that day are lag the next default run measures.
		if index == newest {
			continue
		}
		return updatePlan{}, notLastCommit(kept[position], line.days[index], newestRecorded.Date)
	}

	plan := updatePlan{measured: measured, kept: kept}
	if first == 0 {
		return plan, nil
	}
	// The gap check above proved the day before the range recorded, and the
	// range does not hold it, so the file keeps it.
	position := slices.Index(keptIndex, first-1)
	if position < 0 {
		return updatePlan{}, errors.New("the day before the range is not kept; this is a defect of internal/le/site/history/update.go planRange")
	}
	baseline := kept[position]
	plan.baseline = &Day{Date: baseline.Date, Commit: baseline.Commit, Tree: line.trees[baseline.Commit]}
	return plan, nil
}

// compareDay orders a history day against a date, for a search over the
// ascending history days.
func compareDay(day Day, date Date) int {
	return day.Date.Compare(date)
}

// keptDayIndex answers the index of the history day a kept record describes,
// and refuses a record whose commit is not on HEAD's first-parent history or
// falls on another day there.
func keptDayIndex(day DayRecord, line resolvedHistory) (int, error) {
	index, onHistory := line.dayOf[day.Commit]
	if !onHistory {
		return 0, opererr.New("recorded day "+day.Date.String()+" names commit "+day.Commit+", which is not on HEAD's first-parent history",
			"the history was rewritten after that day was recorded, so "+HistoryFile+" describes commits HEAD no longer has",
			repairWhole)
	}
	if actual := line.days[index].Date; actual.Compare(day.Date) != 0 {
		return 0, opererr.New("recorded day "+day.Date.String()+" names commit "+day.Commit+", which falls on day "+actual.String()+" of HEAD's first-parent history",
			"a recorded day measures a commit of its own day, so this record was made under another day rule or before the history was rewritten",
			repairWhole)
	}
	return index, nil
}

// mergeDays answers kept and measured as one ascending list. The two hold no
// common day: the plan keeps only the file's days the range does not measure.
func mergeDays(kept, measured []DayRecord) []DayRecord {
	merged := make([]DayRecord, 0, len(kept)+len(measured))
	keptNext, measuredNext := 0, 0
	for keptNext < len(kept) && measuredNext < len(measured) {
		if kept[keptNext].Date.Compare(measured[measuredNext].Date) < 0 {
			merged = append(merged, kept[keptNext])
			keptNext++
			continue
		}
		merged = append(merged, measured[measuredNext])
		measuredNext++
	}
	merged = append(merged, kept[keptNext:]...)
	return append(merged, measured[measuredNext:]...)
}

// answerOf builds the answer from the days measured and the history written.
// Both hold at least one day: a plan never measures none, and NewHistory
// refuses an empty history.
func answerOf(path string, measured []DayRecord, history History) updated {
	newest := history.days[len(history.days)-1]
	answer := updated{
		File:         path,
		DaysMeasured: len(measured),
		FirstDay:     measured[0].Date,
		LastDay:      measured[len(measured)-1].Date,
		NewestDay:    newest.Date,
		Sections:     make([]sectionSize, 0, len(history.sections)),
	}
	for _, section := range history.sections {
		lines := newest.Sections[section.ID].Lines
		answer.Sections = append(answer.Sections, sectionSize{ID: section.ID, Label: section.Label, Lines: lines})
		answer.TotalLines += lines
	}
	return answer
}

// otherRules is the refusal of a file measured under other rules. consequence
// completes the explanation for the caller: what the change of rules breaks
// for the update or the check that met it.
func otherRules(recorded, consequence string) error {
	return opererr.New(HistoryFile+" was measured under rules "+recorded+", and this tool counts under rules "+RulesIdentifier(),
		"the section table or a counting rule changed since the file was written, so "+consequence,
		repairWhole)
}

// noHistoryDay is the refusal of a range that holds no day of the history.
func noHistoryDay(request updateRequest, line resolvedHistory) error {
	if _, extends := request.(extendRequest); extends {
		return opererr.New("the newest day "+HistoryFile+" records is after "+line.days[len(line.days)-1].Date.String()+", the newest day of HEAD's first-parent history",
			"the file records days HEAD's history does not hold, so it was measured on another history",
			repairWhole)
	}
	return opererr.New(request.span()+" hold no day of HEAD's first-parent history, whose days run from "+
		line.days[0].Date.String()+" to "+line.days[len(line.days)-1].Date.String(),
		"a range measures the history days it holds, so this one would measure nothing",
		repairExtend)
}

// unrecordedDay is the refusal of a merge that would leave the history day
// missing unrecorded. A day after the file's newest recorded day lies between
// the file and the range, which the default run fills; a day before it is a
// day the file itself skips (skippedDay).
func unrecordedDay(missing Day, newestRecorded Date, request updateRequest) error {
	if missing.Date.Compare(newestRecorded) > 0 {
		return opererr.New("measuring "+request.span()+" would leave "+missing.Date.String()+
			" unrecorded, between the newest recorded day "+newestRecorded.String()+" and the range",
			"the size history holds every day of the history from its first to its newest, and a gap would drop that day from the graph",
			repairExtend)
	}
	return skippedDay(missing, newestRecorded)
}

// skippedDay is the refusal of a file that does not record the history day
// missing, which lies before its newest recorded day. Only a whole-history run
// repairs it: the days around the gap were measured against each other, so the
// missing day cannot be slotted in between them.
func skippedDay(missing Day, newestRecorded Date) error {
	return opererr.New(HistoryFile+" does not record "+missing.Date.String()+", a day of HEAD's first-parent history before its newest recorded day "+newestRecorded.String(),
		"the size history holds every day of the history from its first to its newest, so a file that skips one does not describe this history",
		repairWhole)
}

// notLastCommit is the refusal of a kept day older than the merge's newest
// that does not record its day's last commit. When it is the file's newest
// recorded day, the commits after it are lag the default run measures;
// anywhere else the record does not describe the day's final tree.
func notLastCommit(day DayRecord, history Day, newestRecorded Date) error {
	problem := "recorded day " + day.Date.String() + " names commit " + day.Commit + ", but the last commit of that day is " + history.Commit
	if day.Date.Compare(newestRecorded) == 0 {
		return opererr.New(problem,
			"commits made that day after it was recorded are not measured yet, and a day older than the newest recorded day must record its day's last commit",
			repairExtend)
	}
	return opererr.New(problem,
		"a day older than the newest recorded day must record its day's last commit, so this record does not describe the day's final tree",
		repairWhole)
}

// brokenMerge is the refusal of a merged history that breaks the file
// contract. With days kept, a recorded day beside the range no longer agrees
// with the tree of its commit, which only a whole-history run repairs. With no
// day kept, the measurement broke its own arithmetic, which is a defect of
// this package.
func brokenMerge(err error, request updateRequest, plan updatePlan) error {
	if len(plan.kept) == 0 {
		return opererr.Wrap(err, "the measurement of "+request.span()+" breaks its own line arithmetic",
			"each day's gained and lost lines are counted from the same trees as its sizes, so only a defect in internal/le/site/history/measure.go produces this",
			"./le test unit pkg path ./internal/le/site/history/")
	}
	return opererr.Wrap(err, "the days measured for "+request.span()+" do not join the days "+HistoryFile+" keeps",
		"a recorded day beside the range no longer agrees with the tree of its commit, so the merged history breaks its line arithmetic",
		repairWhole)
}
