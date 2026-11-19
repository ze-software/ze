// Design: docs/architecture/fleet-interface.md -- the Logs tab of a device
// Related: register_device_pages.go -- registers the page and its section fragment
// Related: page_device.templ -- deviceTab, deviceSubTabs and deviceSection, which draw it
// Related: source_views.go -- DeviceLogs, the lines the device answers
//
// The Logs tab shows the device's recent log lines in three sub-tabs: Recent,
// Warnings and Errors. Each is a table of time, level, source and message,
// newest first, bounded to the last LogLinesMax lines, which the query row
// filters by text, level and source. Recent carries the follow control; the
// prototype draws it live and does not stream.

package fleetui

import (
	"context"
	"slices"
	"strings"

	"github.com/a-h/templ"
)

// titleLogs is the title of the Logs tab's one section.
const titleLogs = "Logs"

// logsSubject names the rows of the log table in a refusal.
const logsSubject = "log lines"

// The sub-tabs of the Logs tab, in the order the row shows them.
const (
	subRecent   = "recent"
	subWarnings = "warnings"
	subErrors   = "errors"
)

var (
	logSubKeys   = []string{subRecent, subWarnings, subErrors}
	logSubTitles = []string{"Recent", "Warnings", "Errors"}
)

// The names of the log fields a filter term names.
const (
	fieldLevel  = "level"
	fieldSource = "source"
)

// The levels, in the words devices use for them, that the Warnings and the
// Errors sub-tabs keep.
var (
	warningLevels = []string{"warn", "warning"}
	errorLevels   = []string{"error", "err", "crit", "critical", "alert", "emerg", "fatal"}
)

// logFilters is the filter grammar of the log table. registerLogTable fills
// it from init().
var logFilters = NewFilterSchema[LogLine](logsSubject, logContains, nil)

// registerLogTable registers the fields a log filter may name.
func registerLogTable() {
	logFilters.MustRegisterField(TextField(fieldLevel, nil, func(row *LogLine) string { return row.Level }))
	logFilters.MustRegisterField(TextField(fieldSource, nil, func(row *LogLine) string { return row.Source }))
}

// logContains reports whether a line's message or source holds needle, which
// is lower case.
func logContains(row *LogLine, needle string) bool {
	return strings.Contains(strings.ToLower(row.Message), needle) ||
		strings.Contains(strings.ToLower(row.Source), needle)
}

// levelIn reports whether a line's level is one of levels.
func levelIn(row *LogLine, levels []string) bool {
	return slices.Contains(levels, strings.ToLower(row.Level))
}

// levelStatus answers how a line's level is drawn.
func levelStatus(row *LogLine) Status {
	switch {
	case levelIn(row, errorLevels):
		return Status{Tone: ToneError, Word: row.Level}
	case levelIn(row, warningLevels):
		return Status{Tone: ToneWarning, Word: row.Level}
	default:
		return Status{Tone: ToneNeutral, Word: row.Level}
	}
}

// logSub answers the sub-tab the address names: Recent when it names none.
func logSub(request *PageRequest) (string, error) {
	sub := request.Address.View.Sub
	if sub == "" {
		return subRecent, nil
	}
	if !slices.Contains(logSubKeys, sub) {
		return "", subTabUnknown("Logs", sub, logSubKeys)
	}

	return sub, nil
}

// logsContent builds the Logs tab: its sub-tabs, the follow control on
// Recent, and the section of the sub-tab the address names.
func logsContent(ctx context.Context, request *PageRequest) (templ.Component, error) {
	view, err := deviceOf(request)
	if err != nil {
		return nil, err
	}

	sub, subErr := logSub(request)
	current := sub
	if subErr != nil {
		current = request.Address.View.Sub
	}

	return deviceTab(DeviceTabView{
		Label:   "Logs of " + view.Device.Name,
		SubTabs: subTabsView(request.Address, "Log views", logSubKeys, logSubTitles, current),
		Follow:  sub == subRecent,
		Sections: []DeviceSectionView{{
			Title: titleLogs,
			URL:   sectionLink(request, fragmentDeviceLogs),
			Wide:  true,
			Body:  logsBody(ctx, request, view),
		}},
	}), nil
}

// logsFragment answers the section of the Logs tab.
func logsFragment(ctx context.Context, request *PageRequest, _ string) (templ.Component, error) {
	view, err := deviceOf(request)
	if err != nil {
		return nil, err
	}

	return logsBody(ctx, request, view), nil
}

// logsBody builds the log table of the sub-tab the address names, or the
// state of the section when the device cannot answer.
func logsBody(ctx context.Context, request *PageRequest, view *DeviceView) templ.Component {
	sub, err := logSub(request)
	if err != nil {
		return sectionState(request, titleLogs, err)
	}

	lines, err := request.Source.DeviceLogs(ctx, view.Device.Name)
	if err != nil {
		return sectionState(request, titleLogs, err)
	}

	// Newest first, whatever order the device answered in, then the last
	// LogLinesMax lines only.
	lines = slices.Clone(lines)
	slices.SortStableFunc(lines, func(a, b LogLine) int { return b.At.Compare(a.At) })
	lines = lines[:min(len(lines), LogLinesMax)]

	switch sub {
	case subRecent:
		// Recent keeps every line.
	case subWarnings:
		lines = slices.DeleteFunc(lines, func(row LogLine) bool { return !levelIn(&row, warningLevels) })
	case subErrors:
		lines = slices.DeleteFunc(lines, func(row LogLine) bool { return !levelIn(&row, errorLevels) })
	}

	address := request.Address
	query := &QueryRowView{
		ID:          "fleet-logs-query",
		Action:      address.Path(),
		Text:        address.View.Filter,
		Placeholder: "Filter: a word, level:error, source:bgp",
		Hidden:      carriedFields(address, paramFilter, paramPage, paramRow),
		Menus: []MenuView{
			filterMenu(address, "Level", fieldLevel, distinct(lines, func(row *LogLine) string { return row.Level })),
			filterMenu(address, "Source", fieldSource, distinct(lines, func(row *LogLine) string { return row.Source })),
		},
	}

	matched := lines
	filter, err := logFilters.Parse(address.View.Filter, nil)
	if err == nil {
		matched = filter.Apply(lines)
	} else {
		query.Refusal = refusalOf(err).Problem()
	}

	page := Paginate(len(matched), address.View.Page, address.View.Size)
	query.Matches = matchesText(len(matched), len(lines), logsSubject)

	now := request.Snapshot.Now
	shown := PageRows(matched, page)
	rows := make([]RowView, 0, len(shown))
	for index := range shown {
		row := &shown[index]
		rows = append(rows, RowView{
			Name: row.Message,
			Cells: []CellView{
				sinceCell(now, row.At),
				StatusCell(levelStatus(row)),
				MonoCell(row.Source),
				TextCell(row.Message),
			},
		})
	}

	empty := &StateHeroView{Kind: HeroNoData, Sentence: "No log line matches the filter."}
	if len(lines) == 0 {
		empty = &StateHeroView{Kind: HeroNoData, Sentence: view.Device.Name + " logged no line of this kind."}
	}

	return TableSection(TableSectionView{
		Title: titleLogs,
		Query: query,
		Pager: pagerView(address, logsSubject, page),
		Table: TableView{
			Name: "Log of " + view.Device.Name,
			Headers: []ColumnHeaderView{
				{Label: "Time", Kind: ColumnKindTime},
				{Label: "Level", Kind: ColumnKindState},
				{Label: "Source", Kind: ColumnKindText},
				{Label: "Message", Kind: ColumnKindText},
			},
			Rows:  rows,
			Empty: empty,
		},
	})
}

// distinct answers the values text reads from rows, each once, in the order
// they first appear, leaving out the empty value.
func distinct[T any](rows []T, text func(row *T) string) []string {
	values := []string{}
	for index := range rows {
		value := text(&rows[index])
		if value != "" && !slices.Contains(values, value) {
			values = append(values, value)
		}
	}

	return values
}
