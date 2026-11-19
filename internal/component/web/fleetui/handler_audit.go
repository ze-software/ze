// Design: docs/architecture/fleet-interface.md -- the audit administration page and its export
// Related: page_admin.templ -- draws the administration pages, the audit page among them
// Related: handler_config.go -- the version diff an entry that changed a configuration shows
//
// The audit page is read-only: no control on it edits or removes an entry.
// Its filter takes the fields actor, event, device and result, and the time
// terms after and before, which the time range menu writes. The export
// answers the same filtered entries as CSV or JSON, at most auditExportMax.

package fleetui

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The export of the audit trail.
const (
	// AuditExportPath is the address of the export. Its query holds the
	// filter as q and the format as format.
	AuditExportPath = fleetPath + "export/audit"
	// auditExportMax is the most entries one export holds: the most one
	// answer of the source holds.
	auditExportMax  = AuditEntriesMax
	paramFormat     = "format"
	auditFormatCSV  = "csv"
	auditFormatJSON = "json"
	auditTermAfter  = "after"
	auditTermBefore = "before"
	auditTimeLayout = "2006-01-02T15:04"
	auditDayLayout  = "2006-01-02"
	auditNoun       = "entries"
)

// auditWordActor labels who made a change, in the audit trail and on every
// page that records one: a configuration version, a template, a rollout, a
// task.
const auditWordActor = "Actor"

// auditWordResult labels how a change ended, in the audit trail, a rollout
// and a task.
const auditWordResult = "Result"

// auditFilters is the filter grammar of the audit table, without the time
// terms, which parseAuditQuery reads itself. Its fields are registered from
// the family's register file.
var auditFilters = NewFilterSchema[AuditEntry]("audit entries", auditBare, nil)

// auditBare reports whether an entry's summary, target or actor holds a bare
// word.
func auditBare(row *AuditEntry, needle string) bool {
	for _, text := range []string{row.Summary, row.Target, row.Actor, row.Event} {
		if strings.Contains(strings.ToLower(text), needle) {
			return true
		}
	}

	return false
}

// registerAuditFields registers the filter fields of the audit table.
func registerAuditFields() {
	auditFilters.MustRegisterField(TextField("actor", nil, func(row *AuditEntry) string { return row.Actor }))
	auditFilters.MustRegisterField(TextField("event", nil, func(row *AuditEntry) string { return row.Event }))
	auditFilters.MustRegisterField(TextField("device", nil, func(row *AuditEntry) string { return row.Target }))
	auditFilters.MustRegisterField(TextField("result", auditResultKeys[1:], func(row *AuditEntry) string { return row.Result.Key() }))
}

// auditQuery is a parsed audit filter: the field terms, and the time bounds
// of its after and before terms.
type auditQuery struct {
	filter    Filter[AuditEntry]
	after     time.Time
	before    time.Time
	hasAfter  bool
	hasBefore bool
}

// parseAuditQuery reads an audit filter. The after and before terms take a
// day or a minute, in the hub's time zone written as UTC; every other term
// goes to the audit grammar.
func parseAuditQuery(text string) (auditQuery, error) {
	terms := strings.Fields(text)
	if len(terms) > FilterTermsMax {
		_, err := auditFilters.Parse(text, nil)
		return auditQuery{}, err
	}

	query := auditQuery{}
	rest := make([]string, 0, len(terms))
	for _, term := range terms {
		name, value, _ := strings.Cut(term, ":")
		if name == auditTermAfter {
			at, err := parseAuditTime(term, value)
			if err != nil {
				return auditQuery{}, err
			}
			query.after, query.hasAfter = at, true
			continue
		}
		if name == auditTermBefore {
			at, err := parseAuditTime(term, value)
			if err != nil {
				return auditQuery{}, err
			}
			query.before, query.hasBefore = at, true
			continue
		}
		rest = append(rest, term)
	}

	filter, err := auditFilters.Parse(textbuf.New().Join(rest, " ").String(), nil)
	if err != nil {
		return auditQuery{}, err
	}
	query.filter = filter

	return query, nil
}

// parseAuditTime reads the value of a time term.
func parseAuditTime(term, value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, auditTimeLayout, auditDayLayout} {
		at, err := time.Parse(layout, value)
		if err == nil {
			return at, nil
		}
	}

	return time.Time{}, opererr.Wrap(ErrFilterRefused,
		textbuf.New().Str("The term ").Quoted(term).Str(" does not give a time.").String(),
		"A time term keeps the entries after or before a day or a minute, so a value that is not one cannot be applied.",
		"write after:2026-11-18 or before:2026-11-18T10:00")
}

// match reports whether an entry holds every term and falls inside the time
// bounds.
func (query *auditQuery) match(entry *AuditEntry) bool {
	if !query.filter.Match(entry) {
		return false
	}
	if query.hasAfter && !entry.At.After(query.after) {
		return false
	}
	if query.hasBefore && !entry.At.Before(query.before) {
		return false
	}

	return true
}

// apply answers the entries the query keeps, newest first.
func (query *auditQuery) apply(entries []AuditEntry) []AuditEntry {
	kept := make([]AuditEntry, 0, len(entries))
	for index := range entries {
		if query.match(&entries[index]) {
			kept = append(kept, entries[index])
		}
	}
	slices.SortStableFunc(kept, func(a, b AuditEntry) int { return b.At.Compare(a.At) })

	return kept
}

// auditContent builds the audit page: the filtered table, newest first, with
// the time range menu and the export links.
func auditContent(ctx context.Context, request *PageRequest) (templ.Component, error) {
	entries, err := request.Source.AuditEntries(ctx)
	if err != nil {
		return nil, err
	}

	view := request.Address.View
	query := &QueryRowView{
		ID: "fleet-audit-query", Action: request.Address.Path(), Text: view.Filter,
		Placeholder: "Filter entries: actor:alice event:config-push device:edge-lon-01 result:failed after:2026-11-18",
		Hidden:      treeViewHidden(request),
		Menus:       []MenuView{auditRangeMenu(request)},
	}

	matched := []AuditEntry{}
	parsed, refused := parseAuditQuery(view.Filter)
	if refused != nil {
		query.Refusal = refusalText(refused)
	} else {
		matched = parsed.apply(entries)
	}
	query.Matches = textbuf.New().Int(int64(len(matched))).Str(" of ").Int(int64(len(entries))).Str(" entries").String()

	page := Paginate(len(matched), view.Page, view.Size)
	shown := PageRows(matched, page)

	rows := make([]RowView, 0, len(shown))
	for index := range shown {
		rows = append(rows, auditRow(request, &shown[index]))
	}

	return AdminPage(AdminPageView{
		Exports: auditExports(view.Filter),
		ExportNote: textbuf.New().Str("An export holds the filtered entries, newest first, at most ").
			Int(auditExportMax).Str(".").String(),
		Table: TableSectionView{
			Title: "Audit trail", Query: query, Pager: auditPager(request, page),
			Table: TableView{
				Name: "Audit trail",
				Headers: []ColumnHeaderView{
					{Label: "Time", Kind: ColumnKindTime}, {Label: auditWordActor, Kind: ColumnKindText},
					{Label: "Event", Kind: ColumnKindText}, {Label: "Device or target", Kind: ColumnKindText},
					{Label: auditWordResult, Kind: ColumnKindState}, {Label: "Summary", Kind: ColumnKindText},
				},
				Rows:     rows,
				Controls: true,
				Empty:    &StateHeroView{Kind: HeroNoData, Sentence: "No audit entry matches the filter."},
			},
		},
		Form: manageForm(request),
	}), nil
}

// auditRow answers the row of one entry.
func auditRow(request *PageRequest, entry *AuditEntry) RowView {
	target := TextCell(entry.Target)
	if _, err := FindDevice(request.Snapshot.Views, entry.Target); err == nil {
		target = LinkCell(entry.Target, deviceTabHref(request, entry.Target, tabDeviceDashboard))
	}

	return RowView{
		ID: entry.ID, Name: entry.Event + " " + entry.ID,
		EyeHref: rowAddress(request, entry.ID),
		Opened:  request.Address.View.Row == entry.ID,
		Cells: []CellView{
			whenCell(request, entry.At), TextCell(entry.Actor), MonoCell(entry.Event),
			target, StatusCell(AuditResultStatus(entry.Result)), TextCell(entry.Summary),
		},
	}
}

// auditRangeMenu answers the time range menu: each item writes the after
// term into the filter, and the last removes both time terms.
func auditRangeMenu(request *PageRequest) MenuView {
	filter := RemoveFilterTerm(request.Address.View.Filter, auditTermBefore)
	now := request.Snapshot.Now.UTC()

	item := func(label, text string) ActionView {
		address := request.Address
		address.View.Filter = text
		address.View.Page = 0
		address.View.Row = ""

		return ActionView{Label: label, Href: address.String()}
	}

	return MenuView{Label: "Time", Items: []ActionView{
		item("Last hour", SetFilterTerm(filter, auditTermAfter, now.Add(-time.Hour).Format(auditTimeLayout))),
		item("Last day", SetFilterTerm(filter, auditTermAfter, now.Add(-24*time.Hour).Format(auditTimeLayout))),
		item("Last week", SetFilterTerm(filter, auditTermAfter, now.Add(-7*24*time.Hour).Format(auditTimeLayout))),
		item("Any time", RemoveFilterTerm(filter, auditTermAfter)),
	}}
}

// auditPager answers the pagination row of the audit table.
func auditPager(request *PageRequest, page PageWindow) *PagerView {
	at := func(number int) string {
		address := request.Address
		address.View.Page = number
		address.View.Row = ""

		return address.String()
	}

	return &PagerView{
		Noun: auditNoun, Page: page, Action: request.Address.Path(), Hidden: treeViewHidden(request),
		FirstHref: at(1), PrevHref: at(max(page.Number-1, 1)),
		NextHref: at(min(page.Number+1, page.Pages)), LastHref: at(page.Pages),
	}
}

// auditExports answers the two export links of the filtered entries.
func auditExports(filter string) []LinkView {
	link := func(format string) string {
		query := url.Values{}
		query.Set(paramFormat, format)
		if filter != "" {
			query.Set(paramFilter, filter)
		}

		return withQuery(AuditExportPath, query)
	}

	return []LinkView{{Label: "Export CSV", Href: link(auditFormatCSV)}, {Label: "Export JSON", Href: link(auditFormatJSON)}}
}

// auditPanel builds the side panel of an entry: the whole entry, the task it
// belongs to, and the diff when it changed a device's configuration, which
// is when the device's history holds a version recorded at the entry's time.
func auditPanel(ctx context.Context, request *PageRequest) (templ.Component, error) {
	entries, err := request.Source.AuditEntries(ctx)
	if err != nil {
		return nil, err
	}

	wanted := request.Address.View.Row
	index := -1
	for at := range entries {
		if entries[at].ID == wanted {
			index = at
		}
	}
	if index < 0 {
		return nil, opererr.Wrap(ErrObjectUnknown,
			textbuf.New().Str("The audit trail holds no entry ").Quoted(wanted).Str(".").String(),
			"The address names an entry the hub does not keep, so there is nothing to show.",
			"open the audit page and choose an entry from its table")
	}
	entry := &entries[index]

	rows := []ValueView{
		{Label: "Entry", Text: entry.ID, Mono: true, Copy: true},
		{Label: "Time", Text: whenText(request, entry.At)},
		{Label: auditWordActor, Text: entry.Actor},
		{Label: "Event", Text: entry.Event, Mono: true},
		{Label: "Device or target", Text: entry.Target},
		{Label: devicesWordDevices, Text: countText(entry.Devices)},
		{Label: auditWordResult, Status: AuditResultStatus(entry.Result)},
		{Label: "Summary", Text: entry.Summary},
	}
	if entry.TaskID != "" {
		task := Address{Kind: ObjectTasks, View: ViewParams{Row: entry.TaskID, TreeView: request.Address.View.TreeView}}
		rows = append(rows, ValueView{Label: "Task", Text: entry.TaskID, Href: task.String()})
	}

	view := PanelDiffView{Panel: PanelView{
		Title: entry.Event, CloseHref: rowAddress(request, ""),
		Cards: []PanelCardView{{Title: "Entry", Rows: rows}},
	}}

	diff, changed, err := auditDiff(ctx, request, entry)
	if err != nil {
		return nil, err
	}
	if changed {
		view.Diff = &diff
	}

	return PanelWithDiff(view), nil
}

// auditDiff answers the configuration diff of an entry, and false for an
// entry that changed no configuration: one about no single device of the
// estate, or one whose device's history holds no version recorded at the
// entry's time. The oldest version has no earlier one, and answers false.
func auditDiff(ctx context.Context, request *PageRequest, entry *AuditEntry) (DiffView, bool, error) {
	_, err := FindDevice(request.Snapshot.Views, entry.Target)
	if errors.Is(err, ErrObjectUnknown) {
		return DiffView{}, false, nil
	}
	if err != nil {
		return DiffView{}, false, err
	}

	versions, err := request.Source.ConfigVersions(ctx, entry.Target)
	if err != nil {
		return DiffView{}, false, err
	}

	for index := range versions {
		if !versions[index].At.Equal(entry.At) {
			continue
		}
		diff, _, diffErr := versionDiff(ctx, request, entry.Target, versions, versions[index].Version)
		if diffErr != nil {
			return DiffView{}, false, diffErr
		}
		if diff == nil {
			return DiffView{}, false, nil
		}
		return *diff, true, nil
	}

	return DiffView{}, false, nil
}

// auditExport answers the filtered audit entries as CSV or JSON. It is the
// handler of AuditExportPath.
type auditExport struct {
	source Source
}

// newAuditExportHandler answers the export handler over source.
func newAuditExportHandler(source Source) http.Handler {
	return &auditExport{source: source}
}

// auditRecord is one entry as the JSON export writes it.
type auditRecord struct {
	ID      string    `json:"id"`
	At      time.Time `json:"time"`
	Actor   string    `json:"actor"`
	Event   string    `json:"event"`
	Target  string    `json:"target"`
	Devices int       `json:"devices"`
	Result  string    `json:"result"`
	Summary string    `json:"summary"`
	TaskID  string    `json:"task,omitempty"`
}

// auditColumns is the header row of the CSV export.
var auditColumns = []string{"id", "time", "actor", "event", "target", "devices", "result", "summary", "task"}

func (export *auditExport) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	format := query.Get(paramFormat)
	if !slices.Contains([]string{auditFormatCSV, auditFormatJSON}, format) {
		answerRefusal(w, auditFormatRefused(format), http.StatusBadRequest)
		return
	}

	parsed, err := parseAuditQuery(query.Get(paramFilter))
	if err != nil {
		answerRefusal(w, err, http.StatusBadRequest)
		return
	}

	entries, err := export.source.AuditEntries(r.Context())
	if err != nil {
		answerRefusal(w, err, http.StatusServiceUnavailable)
		return
	}

	kept := parsed.apply(entries)
	kept = kept[:min(len(kept), auditExportMax)]

	var body bytes.Buffer
	contentType := "text/csv; charset=utf-8"
	if format == auditFormatJSON {
		contentType = "application/json"
		err = writeAuditJSON(&body, kept)
	} else {
		err = writeAuditCSV(&body, kept)
	}
	if err != nil {
		answerRefusal(w, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\"audit."+format+"\"")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	w.Write(body.Bytes()) //nolint:errcheck // the answer is committed; a failed write means the browser left
}

// writeAuditCSV writes the header row, then one row per entry.
func writeAuditCSV(body *bytes.Buffer, entries []AuditEntry) error {
	writer := csv.NewWriter(body)
	if err := writer.Write(auditColumns); err != nil {
		return err
	}
	for index := range entries {
		entry := &entries[index]
		record := []string{
			entry.ID, entry.At.UTC().Format(time.RFC3339), entry.Actor, entry.Event, entry.Target,
			countText(entry.Devices), entry.Result.Key(), entry.Summary, entry.TaskID,
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	writer.Flush()

	return writer.Error()
}

// writeAuditJSON writes the entries as one JSON array.
func writeAuditJSON(body *bytes.Buffer, entries []AuditEntry) error {
	records := make([]auditRecord, 0, len(entries))
	for index := range entries {
		entry := &entries[index]
		records = append(records, auditRecord{
			ID: entry.ID, At: entry.At.UTC(), Actor: entry.Actor, Event: entry.Event, Target: entry.Target,
			Devices: entry.Devices, Result: entry.Result.Key(), Summary: entry.Summary, TaskID: entry.TaskID,
		})
	}

	return json.NewEncoder(body).Encode(records)
}

// auditFormatRefused is the refusal for an export format that is not csv or
// json.
func auditFormatRefused(format string) error {
	return opererr.New(
		textbuf.New().Str("The audit export has no format ").Quoted(format).Str(".").String(),
		"The export writes the filtered entries as CSV or JSON only.",
		"ask "+AuditExportPath+"?format=csv or ?format=json")
}
