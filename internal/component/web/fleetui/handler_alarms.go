// Design: docs/architecture/fleet-interface.md -- the alarm tabs and the alarm side panel
// Related: register_activity_pages.go -- registers the pages, the fields, the columns and the alarm tabs used here
// Related: handler_tasks.go -- the table helpers the alarm tabs share with the task tabs
// Related: summary.go -- TopAlarms, the dashboard card's order, which this table's default order follows
//
// An Alarms tab lists the alarms open on the devices of its object, the
// estate's own included on the estate, in a table: severity, device, alarm,
// detail, since, duration. The default order is the most severe first, then
// the oldest. A device has no Alarms tab: its alarms are the estate's tab
// filtered with device:<name>. A row's side panel shows the alarm's history
// on its device over the last week and links to the device tab where the
// condition can be examined.

package fleetui

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// alarmsSubject names the rows of the alarm table in a refusal and a count.
const alarmsSubject = "alarms"

// The sortable columns of the alarm table, as a sort parameter names them.
// The severity comes first, so it is the default sort.
const (
	alarmColumnSeverity = "severity"
	alarmColumnDevice   = "device"
	alarmColumnKind     = "kind"
	alarmColumnSince    = "since"
)

// bugAlarmKind is what a refused registration of an alarm kind's tab names.
const bugAlarmKind = "the alarm kind"

// The words the alarm and task pages share: wordDetail labels the sentence
// that says what is wrong, in an alarm and in a health check; wordKind labels
// what an alarm or a task is about.
const (
	wordDetail = "Detail"
	wordKind   = "Kind"
)

// wordOpenAlarms titles the alarms still open, on the alarm page, a dashboard
// and a device's panel.
const wordOpenAlarms = "Open alarms"

// alarmHistorySpan is how far back the side panel of an alarm shows the
// alarm history of its device.
const alarmHistorySpan = 7 * 24 * time.Hour

// alarmRow is one open alarm with the view of the device it is about, which
// is nil for an alarm of the hub itself. It points into the request's
// snapshot, which outlives the table.
type alarmRow struct {
	Alarm  Alarm
	Device *DeviceView
}

// The filter grammar and the sortable columns of the alarm table. init()
// fills both, and nothing removes from either.
var (
	alarmFilters = NewFilterSchema[alarmRow](alarmsSubject, alarmContains, alarmLabel)
	alarmColumns = NewColumnSet[alarmRow](alarmsSubject)
)

// alarmTabs holds the device tab on which the condition of each alarm kind
// can be examined. Safe for concurrent use.
var alarmTabs struct {
	mu   sync.RWMutex
	tabs map[string]string
}

// mustRegisterAlarmTab registers the device tab of an alarm kind. Call it
// from init() in a register file. It panics on an empty kind or tab and on a
// kind already registered: each is a defect of the build's composition.
func mustRegisterAlarmTab(kind, tab string) {
	if kind == "" {
		registrationBug(bugAlarmKind, kind, alarmsSubject, bugUnnamed)
	}
	if tab == "" {
		registrationBug(bugAlarmKind, kind, alarmsSubject, "names no device tab")
	}

	alarmTabs.mu.Lock()
	defer alarmTabs.mu.Unlock()

	if _, held := alarmTabs.tabs[kind]; held {
		registrationBug(bugAlarmKind, kind, alarmsSubject, bugTwice)
	}
	if alarmTabs.tabs == nil {
		alarmTabs.tabs = make(map[string]string)
	}
	alarmTabs.tabs[kind] = tab
}

// alarmTab answers the device tab registered for an alarm kind, or false.
func alarmTab(kind string) (string, bool) {
	alarmTabs.mu.RLock()
	defer alarmTabs.mu.RUnlock()

	tab, held := alarmTabs.tabs[kind]

	return tab, held
}

// registerAlarmFields registers the fields an alarm filter may name. The
// labels of the alarm's device are fields too, through the schema's label
// reader.
func registerAlarmFields() {
	alarmFilters.MustRegisterField(TextField(activityFieldSeverity, alarmSeverityKeys[1:], func(row *alarmRow) string {
		return row.Alarm.Severity.Key()
	}))
	alarmFilters.MustRegisterField(TextField(activityFieldKind, nil, func(row *alarmRow) string {
		return row.Alarm.Kind
	}))
	alarmFilters.MustRegisterField(TextField(activityFieldDevice, nil, func(row *alarmRow) string {
		return row.Alarm.Device
	}))
	alarmFilters.MustRegisterField(TextField(fieldState, displayStateKeys[1:], func(row *alarmRow) string {
		if row.Device == nil {
			return ""
		}

		return row.Device.State.Key()
	}))
}

// registerAlarmColumns registers the columns the alarm table sorts by. The
// severity comes first and is the default: the most severe first, then the
// oldest, as the alarms card orders them.
func registerAlarmColumns() {
	alarmColumns.MustRegister(Column[alarmRow]{Key: alarmColumnSeverity, Compare: func(a, b *alarmRow) int {
		if order := cmp.Compare(a.Alarm.Severity, b.Alarm.Severity); order != 0 {
			return order
		}

		return a.Alarm.Since.Compare(b.Alarm.Since)
	}})
	alarmColumns.MustRegister(Column[alarmRow]{Key: alarmColumnDevice, Compare: func(a, b *alarmRow) int {
		return strings.Compare(a.Alarm.Device, b.Alarm.Device)
	}})
	alarmColumns.MustRegister(Column[alarmRow]{Key: alarmColumnKind, Compare: func(a, b *alarmRow) int {
		return strings.Compare(a.Alarm.Kind, b.Alarm.Kind)
	}})
	alarmColumns.MustRegister(Column[alarmRow]{Key: alarmColumnSince, Compare: func(a, b *alarmRow) int {
		return a.Alarm.Since.Compare(b.Alarm.Since)
	}})
}

// alarmContains reports whether an alarm holds a bare word of a filter,
// which arrives in lower case: in its device, its kind, its summary or its
// detail.
func alarmContains(row *alarmRow, needle string) bool {
	for _, text := range []string{row.Alarm.Device, row.Alarm.Kind, row.Alarm.Summary, row.Alarm.Detail} {
		if strings.Contains(strings.ToLower(text), needle) {
			return true
		}
	}

	return false
}

// alarmLabel answers a label of the alarm's device. An alarm of the hub
// itself carries none.
func alarmLabel(row *alarmRow, key string) (string, bool) {
	if row.Device == nil {
		return "", false
	}

	return row.Device.Label(key)
}

// scopedAlarms answers the alarms the page lists, joined with their devices:
// every open alarm on the estate, the hub's own included, and the alarms of
// the group's devices on a group node. It refuses a group no device is in.
func scopedAlarms(request *PageRequest) ([]alarmRow, []DeviceView, error) {
	devices, err := activityDevices(request)
	if err != nil {
		return nil, nil, err
	}

	alarms := request.Snapshot.Alarms
	if request.Address.Kind != ObjectEstate {
		alarms = ScopeAlarms(alarms, devices)
	}

	views := request.Snapshot.Views
	byName := make(map[string]*DeviceView, len(views))
	for index := range views {
		byName[views[index].Device.Name] = &views[index]
	}

	// An alarm of the hub itself names no device, and one whose device left
	// the estate names one the snapshot does not hold: either row carries no
	// device view, so it has no state and no labels to filter on.
	rows := make([]alarmRow, 0, len(alarms))
	for index := range alarms {
		rows = append(rows, alarmRow{Alarm: alarms[index], Device: byName[alarms[index].Device]})
	}

	return rows, devices, nil
}

// alarmsContent builds the content of an Alarms tab: the table stack over the
// open alarms of the object, filtered, sorted and paged by the address. A
// filter or a sort the table refuses is shown under the field, and the rows
// are then those before it.
func alarmsContent(_ context.Context, request *PageRequest) (templ.Component, error) {
	rows, devices, err := scopedAlarms(request)
	if err != nil {
		return nil, err
	}

	view := request.Address.View

	kept, refusal := rows, ""
	filter, err := alarmFilters.Parse(view.Filter, LabelKeys(devices))
	if err != nil {
		refusal = activityRefusalText(err)
	}
	if err == nil {
		kept = filter.Apply(rows)
	}

	sorted, err := alarmColumns.Sort(kept, view.Sort)
	if err != nil {
		refusal = activityRefusalText(err)
		sorted, err = alarmColumns.Sort(kept, SortSpec{})
		if err != nil {
			return nil, err
		}
	}

	page := Paginate(len(sorted), view.Page, view.Size)

	matches := activityMatches(len(sorted), len(rows), alarmsSubject)
	menus := []MenuView{
		activityFilterMenu(request, "Severity", activityFieldSeverity, alarmSeverityKeys[1:], alarmSeverityWords[1:]),
		activityFilterMenu(request, wordKind, activityFieldKind, alarmKindsInUse(rows), alarmKindsInUse(rows)),
	}

	return TableSection(TableSectionView{
		Title: wordOpenAlarms,
		Query: activityQuery(request, "fleet-alarms-filter",
			"Filter alarms, as in severity:critical kind:component-down device:edge-lon-04", matches, refusal, menus),
		Pager: activityPager(request.Address, page, alarmsSubject),
		Table: alarmTable(request, PageRows(sorted, page)),
	}), nil
}

// alarmKindsInUse answers the kinds of alarms, each once, in text order.
func alarmKindsInUse(rows []alarmRow) []string {
	kinds := make([]string, 0, len(rows))
	for index := range rows {
		kinds = append(kinds, rows[index].Alarm.Kind)
	}
	slices.Sort(kinds)

	return slices.Compact(kinds)
}

// alarmTable answers the table of one page of alarms.
func alarmTable(request *PageRequest, rows []alarmRow) TableView {
	table := TableView{
		Name:     wordOpenAlarms,
		Headers:  alarmHeaders(request),
		Rows:     make([]RowView, 0, len(rows)),
		Controls: true,
		Empty:    alarmsEmpty(request),
	}

	now := request.Snapshot.Now
	opened := request.Address.View.Row

	for index := range rows {
		row := &rows[index]
		alarm := &row.Alarm

		table.Rows = append(table.Rows, RowView{
			ID:      alarm.ID,
			Name:    alarm.Summary,
			EyeHref: activityRowHref(request.Address, alarm.ID),
			Opened:  alarm.ID == opened,
			Cells: []CellView{
				StatusCell(AlarmStatus(alarm.Severity)),
				alarmDeviceCell(request, row),
				TextCell(alarm.Summary),
				TextCell(alarm.Detail),
				TimeCell(RelativeTime(now.Sub(alarm.Since)), activityTime(alarm.Since)),
				TextCell(activityDuration(now.Sub(alarm.Since))),
			},
		})
	}

	return table
}

// alarmDeviceCell answers the device cell of an alarm: the device's icon with
// its state and its name, linking to it; the hub's name for an alarm of the
// hub itself; the bare name of a device the estate no longer holds.
func alarmDeviceCell(request *PageRequest, row *alarmRow) CellView {
	if row.Alarm.Device == "" {
		return TextCell(request.Snapshot.Estate.Name)
	}
	if row.Device == nil {
		return TextCell(row.Alarm.Device)
	}

	icon := ObjectIconView{Icon: iconDevice, Status: DisplayStatus(row.Device.State), Marked: true}

	return ObjectCell(icon, row.Alarm.Device, deviceAddress(request, row.Alarm.Device).String())
}

// alarmHeaders answers the header cells of the alarm table, each sortable
// one linking to the sort by it, the other way when it is the sort column.
func alarmHeaders(request *PageRequest) []ColumnHeaderView {
	return []ColumnHeaderView{
		alarmHeader(request, "Severity", ColumnKindState, alarmColumnSeverity),
		alarmHeader(request, devicesWordDevice, ColumnKindText, alarmColumnDevice),
		alarmHeader(request, "Alarm", ColumnKindText, alarmColumnKind),
		{Label: wordDetail, Kind: ColumnKindText},
		alarmHeader(request, "Since", ColumnKindTime, alarmColumnSince),
		{Label: "Duration", Kind: ColumnKindTime},
	}
}

// alarmHeader answers one sortable header cell.
func alarmHeader(request *PageRequest, label string, kind ColumnKind, column string) ColumnHeaderView {
	current := request.Address.View.Sort
	if current.Column == "" {
		current.Column = alarmColumnSeverity
	}

	header := ColumnHeaderView{Label: label, Kind: kind}
	target := request.Address
	target.View.Page = 1
	target.View.Sort = SortSpec{Column: column}

	if current.Column == column {
		header.Sort = SortMarkAscending
		if current.Descending {
			header.Sort = SortMarkDescending
		}
		target.View.Sort.Descending = !current.Descending
	}
	header.SortHref = target.String()

	return header
}

// alarmsEmpty is the state of an alarm table with no row: no match for a
// filter, with the way to clear it, or no alarm open at all.
func alarmsEmpty(request *PageRequest) *StateHeroView {
	if request.Address.View.Filter == "" {
		return &StateHeroView{
			Kind:     HeroNoData,
			Sentence: "No alarm is open here.",
			Detail:   "Every condition the hub watches on these devices is as it should be.",
		}
	}

	cleared := request.Address
	cleared.View.Filter = ""
	cleared.View.Page = 1
	cleared.View.Row = ""

	return &StateHeroView{
		Kind:     HeroNoData,
		Sentence: "No alarms match this filter.",
		Detail:   "The filter keeps none of the alarms open here.",
		Action:   &ActionView{Label: wordClearFilter, Icon: glyphClose, Href: cleared.String()},
	}
}

// alarmSidePanel builds the side panel of the alarm the address names: the
// alarm, its history on its device over the last week, and the link to the
// device tab where its condition can be examined.
func alarmSidePanel(ctx context.Context, request *PageRequest) (templ.Component, error) {
	rows, _, err := scopedAlarms(request)
	if err != nil {
		return nil, err
	}

	id := request.Address.View.Row

	var row *alarmRow
	for index := range rows {
		if rows[index].Alarm.ID == id {
			row = &rows[index]
		}
	}
	if row == nil {
		return nil, alarmUnknown(id)
	}

	closed := request.Address
	closed.View.Row = ""

	panel := PanelView{
		Title:     row.Alarm.Summary,
		CloseHref: closed.String(),
		Icon:      ObjectIconView{Icon: iconAlert, Status: AlarmStatus(row.Alarm.Severity), Marked: true},
		Cards:     []PanelCardView{alarmCard(request, row)},
	}

	if row.Alarm.Device == "" {
		return PanelContent(panel), nil
	}

	history, err := request.Source.AlarmHistory(ctx, row.Alarm.Device)
	if err != nil {
		return nil, err
	}
	panel.Cards = append(panel.Cards, alarmHistoryCard(request, row, history))

	return PanelContent(panel), nil
}

// alarmUnknown is the refusal of an alarm the page does not list: it cleared,
// or the address names an alarm of another object.
func alarmUnknown(id string) error {
	problem := textbuf.New().Str("No open alarm here is named ").Quoted(id).Str(".").String()

	return opererr.Wrap(ErrObjectUnknown, problem,
		"The alarm cleared since the address was made, or it belongs to another object, so there are no details to show.",
		"open the alarm again from the table")
}

// alarmCard answers the card of the alarm itself.
func alarmCard(request *PageRequest, row *alarmRow) PanelCardView {
	alarm := &row.Alarm
	now := request.Snapshot.Now

	device := ValueView{Label: devicesWordDevice, Text: request.Snapshot.Estate.Name}
	if alarm.Device != "" {
		device.Text = alarm.Device
	}
	if row.Device != nil {
		device = ValueView{
			Label: devicesWordDevice, Text: alarm.Device, Href: deviceAddress(request, alarm.Device).String(), Icon: iconDevice,
		}
	}

	return PanelCardView{Title: "Alarm", Rows: []ValueView{
		{Label: "Severity", Status: AlarmStatus(alarm.Severity)},
		device,
		{Label: wordKind, Text: alarm.Kind, Mono: true},
		{Label: "Alarm", Text: alarm.Summary},
		{Label: wordDetail, Text: alarm.Detail},
		{Label: "Since", Text: activityTime(alarm.Since)},
		{Label: "Open for", Text: activityDuration(now.Sub(alarm.Since))},
		{Label: "Identifier", Text: alarm.ID, Mono: true, Copy: true},
	}}
}

// alarmHistoryCard answers the alarms that opened or were still open on the
// alarm's device over the last week, newest first as the source answers
// them, and the link to the device tab where the condition is examined.
func alarmHistoryCard(request *PageRequest, row *alarmRow, history []AlarmEvent) PanelCardView {
	now := request.Snapshot.Now
	since := now.Add(-alarmHistorySpan)

	rows := make([]ValueView, 0, len(history))
	for index := range history {
		event := &history[index]
		if event.Cleared && event.ClearedAt.Before(since) {
			continue
		}

		text := textbuf.New().Str(event.Severity.String()).Str(": ").Str(event.Summary)
		if event.Cleared {
			text.Str(", cleared after ").Str(activityDuration(event.ClearedAt.Sub(event.Opened)))
		}
		if !event.Cleared {
			text.Str(", still open")
		}

		rows = append(rows, ValueView{Label: "Opened " + RelativeTime(now.Sub(event.Opened)), Text: text.String()})
	}
	if len(rows) == 0 {
		rows = append(rows, ValueView{Label: "Last week", Text: "No alarm opened on this device."})
	}

	return PanelCardView{
		Title: "History on " + row.Alarm.Device + " over the last week",
		Rows:  rows,
		Link:  alarmExamineLink(request, row),
	}
}

// alarmExamineLink answers the link to the device tab where the condition of
// the alarm can be examined. A kind no feature registered a tab for is
// examined on the device's dashboard, which shows every condition the hub
// holds on the device.
func alarmExamineLink(request *PageRequest, row *alarmRow) *LinkView {
	tab, registered := alarmTab(row.Alarm.Kind)
	if !registered {
		tab = TabDashboard
	}

	target := Address{Kind: ObjectDevice, Device: row.Alarm.Device, Tab: tab, View: linkView(request)}

	return &LinkView{Label: "Examine on the " + tabTitle(target) + " tab", Href: target.String()}
}
