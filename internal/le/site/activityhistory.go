// Design: docs/architecture/site-facts.md -- one published copy of each value: the size history's marker and fill
// Related: activity.go -- renderActivityPage reads the history and fills the page with it
// Related: activitygrowth.go -- the size view that carries the markers, and the script that refills them
// Related: history/file.go -- ReadHistory, the one reader of website/data/repo-history.json
//
// The activity page states repository size only from the committed size
// history. Every size figure is an element marked data-ze-history, written by
// historyValueHTML. The build writes the newest recorded day's value inside
// each one, so a reader with no JavaScript reads the figures of the last build;
// the page script then writes the same figures from the published copy of the
// file, so a page built yesterday shows today's file.

package site

import (
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"

	sitehistory "github.com/ze-software/ze/internal/le/site/history"
)

// historyDataFile is where the build publishes the size history inside the
// artifact. The build stages every published file of the website sources at
// its own path, so the artifact path is the file's path under website/.
var historyDataFile = strings.TrimPrefix(sitehistory.HistoryFile, "website/")

// The subjects a history marker names besides a section of the file, and the
// separator of a subject that sums several sections.
const (
	// historySubjectDay names the newest recorded day itself: its date and its
	// module count.
	historySubjectDay = "day"
	// historySubjectTotal sums every section the file lists.
	historySubjectTotal = "total"
	// historySubjectJoin separates the sections a summed subject names, as in
	// "go-production+go-tests".
	historySubjectJoin = "+"
)

// The fields a history marker names. A section or a summed subject carries the
// first four, a Go section the split as well, and the day its date and module
// count. The page script answers the same names from the published file.
const (
	historyFieldFiles   = "files"
	historyFieldLines   = "lines"
	historyFieldGained  = "gained"
	historyFieldLost    = "lost"
	historyFieldCode    = "code"
	historyFieldBlank   = "blank"
	historyFieldComment = "comment"
	historyFieldDate    = "date"
	historyFieldModules = "modules"
)

// historyValueElement is the one shape the fill writes into: the element
// historyValueHTML writes, empty.
var historyValueElement = regexp.MustCompile(`<span data-ze-history="([^"<>:]+):([^"<>]+)"></span>`)

// historyMarkerAttribute opens a history marker's attribute in a page. The
// stray check searches for it once every well-formed marker is set aside.
const historyMarkerAttribute = `data-ze-history="`

// historyValueHTML is an element the fill and the page script write subject's
// field into. A subject is a section identifier of the file, several joined by
// historySubjectJoin, historySubjectTotal or historySubjectDay.
func historyValueHTML(subject, field string) string {
	return `<span ` + historyMarkerAttribute + html.EscapeString(subject+":"+field) + `"></span>`
}

// historyFigures answers the values of the newest recorded day of one size
// history, the day every marker states.
type historyFigures struct {
	// sections is the file's section list, in its order.
	sections []sitehistory.Section
	// newest is the newest recorded day.
	newest sitehistory.DayRecord
}

// newHistoryFigures takes the newest recorded day of history. A history
// always holds a day, because NewHistory refuses one with none; the check here
// keeps a zero History from publishing a page of zeros.
func newHistoryFigures(history sitehistory.History) (historyFigures, error) {
	days := history.Days()
	if len(days) == 0 {
		return historyFigures{}, errors.New("the size history holds no recorded day, so the activity page has no size to publish; " +
			"run ./le site history update all")
	}
	return historyFigures{sections: history.Sections(), newest: days[len(days)-1]}, nil
}

// subjectSections answers the sections subject sums, refusing a section the
// file does not list.
func (f historyFigures) subjectSections(subject string) ([]sitehistory.Section, error) {
	if subject == historySubjectTotal {
		return f.sections, nil
	}
	var parts []sitehistory.Section
	for id := range strings.SplitSeq(subject, historySubjectJoin) {
		section, known := f.section(id)
		if !known {
			return nil, fmt.Errorf("the size history lists no section %q", id)
		}
		parts = append(parts, section)
	}
	return parts, nil
}

// goSubject answers the summed subject of every section the file splits into
// code, blank and comment lines: all Go, third-party included. The split is
// the file's own mark of a Go section, so no section is named here.
func (f historyFigures) goSubject() (string, error) {
	var ids []string
	for _, section := range f.sections {
		if section.GoSplit {
			ids = append(ids, section.ID)
		}
	}
	if len(ids) == 0 {
		return "", errors.New("the size history lists no section with a Go split, so the activity page cannot state its Go lines; " +
			"run ./le site history update all")
	}
	return strings.Join(ids, historySubjectJoin), nil
}

// section answers the listed section whose identifier is id.
func (f historyFigures) section(id string) (sitehistory.Section, bool) {
	for _, section := range f.sections {
		if section.ID == id {
			return section, true
		}
	}
	return sitehistory.Section{}, false
}

// label answers the name a reader sees for subject: the label of each section
// it sums, joined, so no label is written twice.
func (f historyFigures) label(subject string) (string, error) {
	parts, err := f.subjectSections(subject)
	if err != nil {
		return "", historyMarkerRefusal(subject, "label", err)
	}
	labels := make([]string, len(parts))
	for index, section := range parts {
		labels[index] = section.Label
	}
	return strings.Join(labels, " and "), nil
}

// value answers the text a marker naming subject and field holds: the newest
// recorded day's figure, grouped the way the page prints every number.
func (f historyFigures) value(subject, field string) (string, error) {
	if subject == historySubjectDay {
		return f.dayValue(field)
	}
	parts, err := f.subjectSections(subject)
	if err != nil {
		return "", historyMarkerRefusal(subject, field, err)
	}
	var sum uint64
	for _, section := range parts {
		count, err := historyCount(f.newest.Sections[section.ID], section, field)
		if err != nil {
			return "", historyMarkerRefusal(subject, field, err)
		}
		sum += count
	}
	return historyNumber(sum), nil
}

// dayValue answers a field of the newest recorded day itself.
func (f historyFigures) dayValue(field string) (string, error) {
	switch field {
	case historyFieldDate:
		return f.newest.Date.String(), nil
	case historyFieldModules:
		return historyNumber(f.newest.Modules), nil
	}
	return "", historyMarkerRefusal(historySubjectDay, field,
		fmt.Errorf("%q is not a field of the day, which carries date and modules", field))
}

// historyCount answers one field of one section's values. A section absent
// from the day held nothing that day, so its zero values are its answer.
func historyCount(values sitehistory.SectionValues, section sitehistory.Section, field string) (uint64, error) {
	switch field {
	case historyFieldFiles:
		return values.Files, nil
	case historyFieldLines:
		return values.Lines, nil
	case historyFieldGained:
		return values.Gained, nil
	case historyFieldLost:
		return values.Lost, nil
	case historyFieldCode, historyFieldBlank, historyFieldComment:
		if !section.GoSplit {
			return 0, fmt.Errorf("section %s carries no Go split, so it has no %s lines", section.ID, field)
		}
		return historyGoCount(values.Go, field), nil
	}
	return 0, fmt.Errorf("%q is not a field of a section, which carries files, lines, gained and lost, "+
		"and code, blank and comment on a Go section", field)
}

// historyGoCount answers one count of a Go split. Its caller has already
// matched field to one of the three.
func historyGoCount(split sitehistory.GoLines, field string) uint64 {
	if field == historyFieldCode {
		return split.Code
	}
	if field == historyFieldBlank {
		return split.Blank
	}
	return split.Comment
}

// historyNumber prints a count the way the page prints every other number.
func historyNumber(count uint64) string {
	return groupThousands(int(count)) //nolint:gosec // the history reader bounds every count at 2^48
}

// historyMarkerRefusal names the marker the build cannot fill and why.
func historyMarkerRefusal(subject, field string, cause error) error {
	return fmt.Errorf("the activity page marks a figure %s%s:%s\", which the size history cannot answer: %w; "+
		"write the marker with historyValueHTML, naming a section the file lists, %s or %s, and a field that subject carries "+
		"(internal/le/site/activityhistory.go)", historyMarkerAttribute, subject, field, cause, historySubjectTotal, historySubjectDay)
}

// fillHTML writes the newest recorded day's value inside every history marker
// of a page, refusing the page when a marker is misshapen or names a subject or
// field the file cannot answer. It runs once over a page: a filled marker is
// no longer in the shape it fills.
func (f historyFigures) fillHTML(page string) (string, error) {
	if err := historyStrayMarker(page); err != nil {
		return "", err
	}
	var failure error
	page = historyValueElement.ReplaceAllStringFunc(page, func(element string) string {
		match := historyValueElement.FindStringSubmatch(element)
		text, err := f.value(html.UnescapeString(match[1]), html.UnescapeString(match[2]))
		if err != nil {
			failure = errors.Join(failure, err)
			return element
		}
		closing := strings.LastIndexByte(element, '<')
		return element[:closing] + html.EscapeString(text) + element[closing:]
	})
	if failure != nil {
		return "", failure
	}
	return page, nil
}

// historyStrayMarker refuses a history marker the fill cannot reach: an
// element carrying the attribute but not written in historyValueHTML's exact
// shape (another attribute, another tag, text already inside it). The fill
// would leave it as it stands, with no refusal.
func historyStrayMarker(page string) error {
	rest := historyValueElement.ReplaceAllString(page, "")
	at := strings.Index(rest, historyMarkerAttribute)
	if at < 0 {
		return nil
	}
	end := min(len(rest), at+120)
	return errors.New("a data-ze-history element is not in the shape the build fills, so it would publish without the size history's value: " +
		rest[max(0, at-40):end] + "; write it with historyValueHTML (internal/le/site/activityhistory.go)")
}
