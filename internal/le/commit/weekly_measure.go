// Design: docs/architecture/core-design.md -- the weekly report of tooling events
// Related: weekly_source.go -- weeklyReadFragments, the reading of the range's fragments both fragment sections share
// Related: newsfragment.go -- newsHeadings and newsReadsNone, which each fact cell reads
// Related: register.go -- registers this section with le weekly report
//
// weekly_measure.go answers the fragment-measure section of `le weekly report`:
// the table plan/records/weekly-fragment-measure.md takes once per update,
// filled with what the range's news fragments carry. The judgement stays with
// the update's writer, so the section's note names it: a claim the producer
// refuted, a fact research found, an item no fragment described.

package commit

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/le/weekly"
)

// weeklyMeasureSection is the section this file answers, kebab-case as weekly
// requires.
const weeklyMeasureSection = "fragment-measure"

// weeklyMeasureCarried is the cell of a fact a fragment writes text for, from
// the vocabulary plan/records/weekly-fragment-measure.md declares. A fact the
// fragment writes only none for takes newsNone, the same word.
const weeklyMeasureCarried = "fragment"

// weeklyMeasureColumn is one fact column of the measure table and the
// fragment headings it reads.
type weeklyMeasureColumn struct {
	name     string
	headings []string
}

// weeklyMeasureColumns are the table's fact columns, in the order
// plan/records/weekly-fragment-measure.md sets. Before covers the Before and
// After headings together, as that file states. Each heading of newsHeadings
// is read by exactly one column, which
// TestWeeklyMeasureColumnsReadEveryHeadingOnce checks.
var weeklyMeasureColumns = []weeklyMeasureColumn{
	{name: newsHeadingBefore, headings: []string{newsHeadingBefore, newsHeadingAfter}},
	{name: newsHeadingCorrects, headings: []string{newsHeadingCorrects}},
	{name: newsHeadingExposure, headings: []string{newsHeadingExposure}},
	{name: newsHeadingUpstream, headings: []string{newsHeadingUpstream}},
	{name: newsHeadingAskedBy, headings: []string{newsHeadingAskedBy}},
}

// The two columns around the fact columns: what the row is, and which
// fragment it was read from.
const (
	weeklyMeasureItemColumn     = "Item"
	weeklyMeasureFragmentColumn = "Fragment"
)

// weeklyMeasureNote tells the writer what the table holds and what is left to
// do before appending it.
const weeklyMeasureNote = "The table plan/records/weekly-fragment-measure.md takes for this range: one row per " +
	"news fragment added in it, read at the second revision. A fact reads fragment when the fragment writes text " +
	"under its heading, and none when it writes only none; a fragment missing a heading does not parse, and fails " +
	"this section. Item holds the fragment's topic, because a fragment has no title. Before appending: name the " +
	"published item in Item; turn fragment into failed where verification at the producer refuted the claim; turn " +
	"none into research where you found the fact, or n/a where the item has no such fact; drop the row of a " +
	"fragment the update does not publish, and repeat it for each further item it carries; add one row per " +
	"published item that had no fragment, with none under Fragment."

// answerWeeklyFragmentMeasure answers the measure table of the fragments the
// range adds. A fragment that does not parse fails the section through
// weeklyReadFragments, so the table never omits one in silence; a range that
// adds none answers a zero count and an empty table.
func answerWeeklyFragmentMeasure(ctx context.Context, events weekly.EventRange) (weekly.SectionBody, error) {
	fragments, err := weeklyReadFragments(ctx, events)
	if err != nil {
		return weekly.SectionBody{}, err
	}
	block := make([]string, 0, len(fragments)+4)
	block = append(block, "## "+events.FromCommit+".."+events.ToCommit, "",
		weeklyMeasureHeader(), weeklyMeasureRule())
	for _, fragment := range fragments {
		row, err := weeklyMeasureRow(fragment)
		if err != nil {
			return weekly.SectionBody{}, err
		}
		block = append(block, row)
	}
	facts := []weekly.Fact{{Name: "fragments measured", Value: strconv.Itoa(len(fragments))}}
	return weekly.SectionBody{Note: weeklyMeasureNote, Facts: facts, Block: block}, nil
}

// weeklyMeasureHeader answers the table's header line.
func weeklyMeasureHeader() string {
	names := make([]string, 0, len(weeklyMeasureColumns)+2)
	names = append(names, weeklyMeasureItemColumn)
	for _, column := range weeklyMeasureColumns {
		names = append(names, column.name)
	}
	names = append(names, weeklyMeasureFragmentColumn)
	return "| " + strings.Join(names, " | ") + " |"
}

// weeklyMeasureRule answers the line under the header: one run of dashes per
// column, two wider than its name, as the record file writes it.
func weeklyMeasureRule() string {
	var rule strings.Builder
	rule.WriteString("|")
	widths := make([]int, 0, len(weeklyMeasureColumns)+2)
	widths = append(widths, len(weeklyMeasureItemColumn))
	for _, column := range weeklyMeasureColumns {
		widths = append(widths, len(column.name))
	}
	widths = append(widths, len(weeklyMeasureFragmentColumn))
	for _, width := range widths {
		rule.WriteString(strings.Repeat("-", width+2))
		rule.WriteString("|")
	}
	return rule.String()
}

// weeklyMeasureRow answers one fragment's row: its topic, one cell per fact
// column, and its file stem.
func weeklyMeasureRow(fragment newsFragment) (string, error) {
	cells := make([]string, 0, len(weeklyMeasureColumns)+2)
	cells = append(cells, weeklyMeasureEscape(fragment.topic))
	for _, column := range weeklyMeasureColumns {
		cell, err := weeklyMeasureCell(fragment, column)
		if err != nil {
			return "", err
		}
		cells = append(cells, cell)
	}
	stem := strings.TrimSuffix(strings.TrimPrefix(fragment.path, newsFragmentDirectory), ".md")
	cells = append(cells, weeklyMeasureEscape(stem))
	return "| " + strings.Join(cells, " | ") + " |", nil
}

// weeklyMeasureCell answers weeklyMeasureCarried when the fragment writes text
// other than none under any heading the column reads, and newsNone when every
// one reads only none.
func weeklyMeasureCell(fragment newsFragment, column weeklyMeasureColumn) (string, error) {
	for _, heading := range column.headings {
		text, known := fragment.Section(heading)
		// parseNewsFragment refuses a fragment missing a heading, so a parsed
		// one without it is a defect that would otherwise print as none.
		if !known {
			return "", errors.New("BUG: parsed news fragment " + fragment.path + " carries no " + heading +
				" heading, which parseNewsFragment requires")
		}
		if !newsReadsNone(text) {
			return weeklyMeasureCarried, nil
		}
	}
	return newsNone, nil
}

// weeklyMeasureEscape keeps a cell's text from closing the cell: a pipe in a
// topic or a file name would otherwise shift every cell after it.
func weeklyMeasureEscape(text string) string {
	return strings.ReplaceAll(text, "|", `\|`)
}
