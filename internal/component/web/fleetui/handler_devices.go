// Design: docs/architecture/fleet-interface.md -- the devices table of the estate and of a group node, and its side panel
// Related: page_devices.templ -- draws the tab around the table section
// Related: handler_devices_action.go -- the actions of a selection and the route that answers them
// Related: register_devices_pages.go -- registers the pages and the action route
// Related: register_devices.go -- the filter fields and the sort columns the table uses
// Related: table.go -- QueryDevices, which filters, sorts and pages the rows
//
// The Devices tab of the estate and of every group node is one table over the
// devices of its object: the query row, the selection, the table and the
// pager. Filtering, sorting and paging happen here, on the server, so a page
// never holds more rows than it shows, and every link the table draws is an
// address that reproduces the view it opens.

package fleetui

import (
	"context"
	"errors"
	"maps"
	"net/netip"
	"slices"
	"time"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The names the devices table gives its parts and the tabs it links to.
const (
	// devicesTab is the tab the table is the content of.
	devicesTab = "devices"
	// devicesAlarmsTab is the estate's alarms tab, which an alarm count opens
	// filtered on its device.
	devicesAlarmsTab = "alarms"
	// devicesConsoleTab and devicesConfigTab are the device tabs a row's menu
	// opens.
	devicesConsoleTab = "console"
	devicesConfigTab  = "config"
	// devicesAlarmField is the field of the alarms table that names a device.
	devicesAlarmField = "device"
	// devicesFormID is the id of the bulk form the row checkboxes belong to.
	devicesFormID = "fleet-devices-bulk"
	// devicesFilterID is the id of the filter field.
	devicesFilterID = "fleet-devices-filter"
	devicesTitle    = "Devices"
	// devicesPlaceholder shows the form of a filter in the empty field.
	devicesPlaceholder = "state:degraded,down site:lon"
	// devicesTimeLayout writes an exact time, in a tooltip and in the panel.
	devicesTimeLayout = "2006-01-02 15:04:05 UTC"
)

// The words of the table's headers, the panel's labels and the actions a
// row and the panel share.
const (
	devicesWordCPU         = "CPU"
	devicesWordMemory      = "Memory"
	devicesWordStorage     = "Storage"
	devicesWordUptime      = "Uptime"
	devicesWordDevice      = "Device"
	devicesWordDevices     = "Devices"
	devicesWordGroup       = "Group"
	devicesWordConnection  = "Connection"
	devicesWordState       = "State"
	devicesWordLabels      = "Labels"
	devicesWordVersion     = "Version"
	devicesWordConfig      = "Config"
	devicesWordLastSeen    = "Last seen"
	devicesWordAlarms      = "Alarms"
	devicesWordHealth      = "Health"
	devicesWordName        = "Name"
	devicesWordModel       = "Model"
	devicesWordOpenDevice  = "Open device"
	devicesWordOpenConsole = "Open console"
	devicesWordDiagnostic  = "Run diagnostic"
	devicesWordPushConfig  = "Push config"
	devicesWordLabel       = "Label"
	devicesWordAddress     = "Management address"
)

// The words of what a device reports about itself, in its panel, its
// dashboard, its System tab and the tree.
const (
	devicesWordSerial   = "Serial number"
	devicesWordSoftware = "Software version"
)

// devicesLabelJoin joins a label's key and value, as a tag and a
// label:key=value term write them.
const devicesLabelJoin = "="

// devicesLabelItemsMax bounds the items of the assisted Label menu: an estate
// with more label values than this is filtered by typing the term.
const devicesLabelItemsMax = 24

// devicesPanelAlarmsMax is how many open alarms the side panel lists.
const devicesPanelAlarmsMax = 3

// The sentences a refused part of the address adds to its refusal, saying
// what the table shows instead. The page keeps no rows between two requests,
// so a refused filter cannot keep the previous rows, and the table says so.
const (
	devicesUnfiltered = "The table below shows every device of this page, unfiltered: the page keeps no previous rows to show instead."
	devicesUnsorted   = "The table below is in its default order."
)

// devicesColumn is one column of the table: its header, and the sort column
// that orders it, which is empty for a column that cannot be sorted.
type devicesColumn struct {
	label   string
	kind    ColumnKind
	key     string
	numeric bool
}

// devicesColumns are the columns of the table after the selection, in the
// order deviceCells fills them. The sort keys are the ones register_devices.go
// registers.
var devicesColumns = []devicesColumn{
	{label: devicesWordDevice, kind: ColumnKindText, key: fieldName},
	{label: devicesWordState, kind: ColumnKindState, key: fieldState},
	{label: devicesWordLabels, kind: ColumnKindTag},
	{label: devicesWordVersion, kind: ColumnKindText, key: fieldVersion},
	{label: devicesWordConfig, kind: ColumnKindState, key: fieldConfig},
	{label: devicesWordUptime, kind: ColumnKindTime, key: fieldUptime},
	{label: devicesWordLastSeen, kind: ColumnKindTime, key: fieldLastSeen},
	{label: devicesWordCPU, kind: ColumnKindNumber, key: fieldCPU, numeric: true},
	{label: devicesWordMemory, kind: ColumnKindNumber, key: fieldMemory, numeric: true},
	{label: devicesWordAlarms, kind: ColumnKindNumber, key: fieldAlarms, numeric: true},
}

// devicesContent builds the Devices tab of the estate or of a group node.
func devicesContent(_ context.Context, request *PageRequest) (templ.Component, error) {
	view, err := devicesTabView(request)
	if err != nil {
		return nil, err
	}

	return DevicesTab(view), nil
}

// devicesScope answers the devices the table of the request's object holds:
// every device for the estate, the node's own for a group node. Only those
// two objects have a devices table.
func devicesScope(request *PageRequest) ([]DeviceView, error) {
	address := request.Address

	switch address.Kind {
	case ObjectEstate:
		return request.Snapshot.Views, nil
	case ObjectGroup:
		return GroupDevices(request.Snapshot.Views, address.Group)
	case ObjectDevice, ObjectTasks, ObjectAdmin:
		return nil, addressUnknown(address.Path())
	case ObjectUnspecified:
		panic(bugAddressKind)
	default:
		panic(bugAddressKind)
	}
}

// devicesTable is one answer of the table: the page of rows, the query that
// produced it, and the refusal of the part of the address the table dropped.
type devicesTable struct {
	answer DeviceAnswer
	// query is the query the rows answer. It differs from the address only
	// where the address was refused.
	query   DeviceQuery
	refusal string
}

// queryDevicesTable answers the page of the table over scope that view asks
// for. A filter the grammar refuses is dropped and a sort column the table
// lacks is the default one, each with its refusal, so the operator sees rows
// and the reason: the refusal names the bad term and the valid fields.
func queryDevicesTable(scope []DeviceView, view ViewParams) (devicesTable, error) {
	table := devicesTable{query: DeviceQuery{Filter: view.Filter, Sort: view.Sort, Page: view.Page, Size: view.Size}}
	refusals := make([]string, 0, 2)

	answer, err := QueryDevices(scope, table.query)
	if errors.Is(err, ErrFilterRefused) {
		refusals = append(refusals, devicesRefusalText(err, devicesUnfiltered))
		table.query.Filter = ""
		answer, err = QueryDevices(scope, table.query)
	}
	if errors.Is(err, ErrSortRefused) {
		refusals = append(refusals, devicesRefusalText(err, devicesUnsorted))
		table.query.Sort = SortSpec{}
		answer, err = QueryDevices(scope, table.query)
	}
	if err != nil {
		return devicesTable{}, err
	}

	table.answer = answer
	table.refusal = textbuf.New().Join(refusals, " ").Slice()

	return table, nil
}

// devicesRefusalText writes a refusal for the query row: what was refused,
// how to write it, and what the table shows instead.
func devicesRefusalText(err error, instead string) string {
	refusal := refusalOf(err)

	return textbuf.New().Str(refusal.Problem()).Str(" Fix: ").Str(refusal.Fix()).Str(". ").Str(instead).Slice()
}

// devicesTabView fills the Devices tab of the request's object.
func devicesTabView(request *PageRequest) (DevicesTabView, error) {
	scope, err := devicesScope(request)
	if err != nil {
		return DevicesTabView{}, err
	}

	table, err := queryDevicesTable(scope, request.Address.View)
	if err != nil {
		return DevicesTabView{}, err
	}

	base := devicesBaseView(request, &table)

	return DevicesTabView{Section: TableSectionView{
		Title: devicesTitle,
		Query: devicesQueryRow(request, scope, base, &table),
		Bulk:  devicesBulk(request, &table),
		Pager: devicesPager(request, base, &table),
		Table: devicesTableView(request, base, &table),
	}}, nil
}

// devicesBaseView answers the view every link of the table starts from: the
// address's view, with the filter and the sort the rows answer, so a link
// never carries a refused part along.
func devicesBaseView(request *PageRequest, table *devicesTable) ViewParams {
	view := request.Address.View
	view.Filter = table.query.Filter
	view.Sort = table.query.Sort
	view.Page = table.answer.Page.Number

	return view
}

// devicesLink answers the address of the request's tab under view.
func devicesLink(request *PageRequest, view ViewParams) string {
	target := request.Address
	target.View = view

	return target.String()
}

// devicesFilterLink answers the address of the first page of the table under
// filter, with no row open: the rows change, so the open row may be gone.
func devicesFilterLink(request *PageRequest, base ViewParams, filter string) string {
	view := base
	view.Filter = filter
	view.Page = 1
	view.Row = ""

	return devicesLink(request, view)
}

// devicesHidden answers the view parameters of view as the hidden fields of
// a form, without those the form itself sets or resets. The fields are in
// name order, so the markup is the same at every request.
func devicesHidden(view ViewParams, dropped ...string) []HiddenFieldView {
	query := view.Query()
	for _, name := range dropped {
		query.Del(name)
	}

	fields := make([]HiddenFieldView, 0, len(query))
	for _, name := range slices.Sorted(maps.Keys(query)) {
		fields = append(fields, HiddenFieldView{Name: name, Value: query.Get(name)})
	}

	return fields
}

// devicesCount writes a count of devices: "1 device", "3 devices".
func devicesCount(count int) string {
	text := textbuf.New().Int(int64(count)).Str(" device")
	if count != 1 {
		text.Byte('s')
	}

	return text.Slice()
}

// devicesQueryRow fills the query row: the filter as the address holds it,
// the assisted menus that write a term into it, the saved filters, and the
// count of the rows the filter keeps out of the object's devices.
func devicesQueryRow(request *PageRequest, scope []DeviceView, base ViewParams, table *devicesTable) *QueryRowView {
	matches := textbuf.New().Int(int64(table.answer.Page.Total)).Str(" of ").
		Int(int64(len(scope))).Byte(' ').Str(devicesSubject).Slice()

	saved := devicesSavedMenu(request, scope, base)

	return &QueryRowView{
		ID:          devicesFilterID,
		Action:      request.Address.Path(),
		Text:        request.Address.View.Filter,
		Placeholder: devicesPlaceholder,
		Matches:     matches,
		Refusal:     table.refusal,
		Hidden:      devicesHidden(base, paramFilter, paramPage, paramRow),
		Menus: []MenuView{
			devicesStateMenu(request, base),
			devicesLabelMenu(request, scope, base),
			devicesVersionMenu(request, scope, base),
		},
		Saved: &saved,
		Save: &ActionView{
			Label: "Save", Icon: glyphSave, Form: devicesFormID, Name: devicesFieldAction, Value: devicesActionFilterSave,
		},
	}
}

// devicesStateMenu is the assisted State menu: one item per display state,
// each writing the state term into the filter, and one removing it.
func devicesStateMenu(request *PageRequest, base ViewParams) MenuView {
	states := DisplayStates()
	items := make([]ActionView, 0, len(states)+1)
	for _, state := range states {
		filter := SetFilterTerm(base.Filter, fieldState, state.Key())
		items = append(items, ActionView{Label: state.String(), Href: devicesFilterLink(request, base, filter)})
	}
	items = append(items, ActionView{
		Label: "Any state", Href: devicesFilterLink(request, base, RemoveFilterTerm(base.Filter, fieldState)),
	})

	return MenuView{Label: devicesWordState, Icon: glyphFilter, Items: items}
}

// devicesLabelMenu is the assisted Label menu: one item per label value in
// use among the object's devices, each writing the key's term into the
// filter. A key that is also a field of the table is written in the long
// form, label:key=value, because the short form would name the field.
func devicesLabelMenu(request *PageRequest, scope []DeviceView, base ViewParams) MenuView {
	fields := DeviceFilters().Fields()
	items := make([]ActionView, 0, devicesLabelItemsMax+1)

	for _, usage := range LabelsInUse(scope) {
		for _, value := range usage.Values {
			if len(items) == devicesLabelItemsMax {
				items = append(items, ActionView{
					Label: "More labels", Disabled: true,
					Reason: "Only the first " + countText(devicesLabelItemsMax) + " labels are listed: type key:value in the filter for another.",
				})

				return MenuView{Label: devicesWordLabel, Icon: ColumnKindTag.Icon(), Items: items}
			}

			filter := SetFilterTerm(base.Filter, usage.Key, value.Value)
			if slices.Contains(fields, usage.Key) {
				filter = SetFilterTerm(base.Filter, filterLabelKey, usage.Key+devicesLabelJoin+value.Value)
			}

			label := textbuf.New().Str(usage.Key).Str(devicesLabelJoin).Str(value.Value).
				Str(" (").Int(int64(value.Count)).Byte(')').Slice()
			items = append(items, ActionView{Label: label, Href: devicesFilterLink(request, base, filter)})
		}
	}

	return MenuView{Label: devicesWordLabel, Icon: ColumnKindTag.Icon(), Items: items}
}

// devicesVersionMenu is the assisted Version menu: one item per software
// version the object's devices report, the newest first, and one removing
// the version term.
func devicesVersionMenu(request *PageRequest, scope []DeviceView, base ViewParams) MenuView {
	versions := Versions(scope)
	items := make([]ActionView, 0, len(versions)+1)
	for _, version := range versions {
		filter := SetFilterTerm(base.Filter, fieldVersion, version)
		items = append(items, ActionView{Label: version, Href: devicesFilterLink(request, base, filter)})
	}
	items = append(items, ActionView{
		Label: "Any version", Href: devicesFilterLink(request, base, RemoveFilterTerm(base.Filter, fieldVersion)),
	})

	return MenuView{Label: devicesWordVersion, Icon: ColumnKindText.Icon(), Items: items}
}

// devicesSavedFilters answers the saved filters the menu lists. Saving a
// filter per user is not backed by the prototype, so these are fixed
// samples; "Behind on software" names the newest version the object's
// devices report, and is absent when none reports one.
func devicesSavedFilters(scope []DeviceView) []SavedFilter {
	saved := []SavedFilter{
		{Name: "Needs attention", Filter: "state:degraded,down,offline,diverged"},
		{Name: "London edge", Filter: "site:lon role:edge"},
	}

	versions := Versions(scope)
	if len(versions) > 0 {
		saved = append(saved, SavedFilter{Name: "Behind on software", Filter: fieldVersion + ":!" + versions[0]})
	}

	return saved
}

// devicesSavedMenu is the Saved filters menu: each item replaces the whole
// filter with the saved one.
func devicesSavedMenu(request *PageRequest, scope []DeviceView, base ViewParams) MenuView {
	saved := devicesSavedFilters(scope)
	items := make([]ActionView, 0, len(saved))
	for _, filter := range saved {
		items = append(items, ActionView{Label: filter.Name, Href: devicesFilterLink(request, base, filter.Filter)})
	}

	return MenuView{Label: wordFilters, Icon: glyphSave, Items: items}
}

// devicesBulk fills the selection of the table: the form the checkboxes
// belong to, the actions of a selection, and the filter, posted along so
// that "every match" names the rows the table matched.
func devicesBulk(request *PageRequest, table *devicesTable) *BulkView {
	return &BulkView{
		Form:    devicesFormID,
		Action:  devicesActionAddress(request.Address),
		Filter:  table.query.Filter,
		Noun:    devicesSubject,
		Actions: devicesActionViews(devicesPlaceBar),
		More: &MenuView{
			Label:       "More",
			Items:       devicesActionViews(devicesPlaceMore),
			Destructive: devicesActionViews(devicesPlaceDestructive),
		},
		Total: table.answer.Page.Total,
	}
}

// devicesPager fills the pagination row.
func devicesPager(request *PageRequest, base ViewParams, table *devicesTable) *PagerView {
	page := table.answer.Page

	return &PagerView{
		Noun:      devicesSubject,
		FirstHref: devicesPageLink(request, base, 1),
		PrevHref:  devicesPageLink(request, base, max(page.Number-1, 1)),
		NextHref:  devicesPageLink(request, base, min(page.Number+1, page.Pages)),
		LastHref:  devicesPageLink(request, base, page.Pages),
		Action:    request.Address.Path(),
		Hidden:    devicesHidden(base, paramSize, paramPage),
		Page:      page,
	}
}

// devicesPageLink answers the address of page number of the table.
func devicesPageLink(request *PageRequest, base ViewParams, number int) string {
	view := base
	view.Page = number

	return devicesLink(request, view)
}

// devicesTableView fills the table: its headers, one row per device of the
// page, and the state it shows when no device matches.
func devicesTableView(request *PageRequest, base ViewParams, table *devicesTable) TableView {
	rows := make([]RowView, 0, len(table.answer.Rows))
	for index := range table.answer.Rows {
		rows = append(rows, devicesRow(request, base, &table.answer.Rows[index]))
	}

	return TableView{
		Name:     devicesTitle,
		Form:     devicesFormID,
		Headers:  devicesHeaders(request, base),
		Rows:     rows,
		Empty:    devicesEmpty(request, base),
		Controls: true,
	}
}

// devicesEmpty is the state of a table with no row: no device matches the
// filter, which a link clears, or the object holds no device.
func devicesEmpty(request *PageRequest, base ViewParams) *StateHeroView {
	if base.Filter == "" {
		return &StateHeroView{Kind: HeroNoData, Sentence: "There is no device here."}
	}

	return &StateHeroView{
		Kind:     HeroNoData,
		Sentence: "No device matches the filter.",
		Detail:   "Change the filter, or clear it to see every device.",
		Action:   &ActionView{Label: wordClearFilter, Icon: glyphClose, Href: devicesFilterLink(request, base, "")},
	}
}

// devicesHeaders fills the header cells. A sortable header links to the
// table sorted by its column, the other way round when it is the sort column
// already, and marks the current sort. The default sort is the first column
// registered, ascending.
func devicesHeaders(request *PageRequest, base ViewParams) []ColumnHeaderView {
	current := base.Sort.Column
	if current == "" {
		current = DeviceColumns().Keys()[0]
	}

	headers := make([]ColumnHeaderView, 0, len(devicesColumns))
	for _, column := range devicesColumns {
		header := ColumnHeaderView{Label: column.label, Kind: column.kind, Numeric: column.numeric}
		if column.key == "" {
			headers = append(headers, header)
			continue
		}

		sorted := column.key == current
		view := base
		view.Page = 1
		view.Sort = SortSpec{Column: column.key, Descending: sorted && !base.Sort.Descending}
		header.SortHref = devicesLink(request, view)

		if sorted {
			header.Sort = SortMarkAscending
			if base.Sort.Descending {
				header.Sort = SortMarkDescending
			}
		}
		headers = append(headers, header)
	}

	return headers
}

// devicesRow fills the row of one device. Its eye opens the device in the
// side panel by naming it in the address, and the address that shows it
// marks it opened.
func devicesRow(request *PageRequest, base ViewParams, view *DeviceView) RowView {
	name := view.Device.Name
	opened := base
	opened.Row = name

	return RowView{
		ID:      name,
		Name:    name,
		EyeHref: devicesLink(request, opened),
		Cells:   devicesCells(request, view),
		Menu:    devicesMenu(request, name),
		Opened:  request.Address.View.Row == name,
	}
}

// devicesIcon answers the icon of a device, marked with its state.
func devicesIcon(view *DeviceView) ObjectIconView {
	return ObjectIconView{Icon: iconDevice, Status: DisplayStatus(view.State), Marked: true}
}

// devicesCells fills the cells of one row, in the order of devicesColumns.
func devicesCells(request *PageRequest, view *DeviceView) []CellView {
	device := &view.Device

	return []CellView{
		ObjectCell(devicesIcon(view), device.Name, deviceAddress(request, device.Name).String()),
		StatusCell(DisplayStatus(view.State)),
		TagsCell(devicesRowTags(device)),
		MonoCell(device.Inventory.SoftwareVersion),
		StatusCell(ConfigStatus(device.Config.State)),
		TextCell(devicesUptime(view)),
		devicesSeenCell(request, view),
		devicesUsageCell(devicesWordCPU, device.Latest.CPUPercent, device.Latest.Reported),
		devicesUsageCell(devicesWordMemory, device.Latest.MemoryPercent, device.Latest.Reported),
		devicesAlarmsCell(request, view),
	}
}

// devicesRowTags answers the labels a row of the table shows: every label but
// the model's, which repeats the device's inventory (the model filter field
// reads the same fact) and is shown in the side panel. Leaving it out keeps
// the Labels column narrow enough for every other column to fit.
func devicesRowTags(device *Device) []TagView {
	tags := make([]TagView, 0, len(device.Labels))
	for _, label := range device.Labels {
		if label.Key == fieldModel {
			continue
		}
		tags = append(tags, TagView{Key: label.Key, Value: label.Value})
	}

	return tags
}

// devicesTags answers the labels of a device as tags.
func devicesTags(device *Device) []TagView {
	tags := make([]TagView, 0, len(device.Labels))
	for _, label := range device.Labels {
		tags = append(tags, TagView{Key: label.Key, Value: label.Value})
	}

	return tags
}

// devicesUptime writes how long the device's daemon has run, or the empty
// text, drawn as a dash, when the device is not connected or never said.
func devicesUptime(view *DeviceView) string {
	if !view.UptimeKnown {
		return ""
	}

	return devicesDuration(view.Uptime)
}

// devicesDuration writes a duration in its two largest units: "41d 3h",
// "3h 12m", "12m".
func devicesDuration(age time.Duration) string {
	days := int64(age / (24 * time.Hour))
	hours := int64(age % (24 * time.Hour) / time.Hour)
	minutes := int64(age % time.Hour / time.Minute)

	text := textbuf.New()
	if days > 0 {
		return text.Int(days).Str("d ").Int(hours).Byte('h').Slice()
	}
	if hours > 0 {
		return text.Int(hours).Str("h ").Int(minutes).Byte('m').Slice()
	}

	return text.Int(minutes).Byte('m').Slice()
}

// devicesExact writes an exact time.
func devicesExact(at time.Time) string {
	return at.UTC().Format(devicesTimeLayout)
}

// devicesSeenCell fills the Last seen cell: "now" for a connected device,
// how long ago for one that is not, and a dash for one never seen.
func devicesSeenCell(request *PageRequest, view *DeviceView) CellView {
	now := request.Snapshot.Now

	if view.Device.Connection.Connected {
		return TimeCell(relativeNow, devicesExact(now))
	}
	if view.SeenKnown {
		return TimeCell(RelativeTime(view.SeenAgo), devicesExact(now.Add(-view.SeenAgo)))
	}

	return TimeCell("", "")
}

// devicesUsageCell fills a usage cell from a percentage the device reported.
func devicesUsageCell(name string, percent float64, reported bool) CellView {
	whole, known := Percent(percent, widthPercentMax)

	return UsageCell(name, whole, known && reported)
}

// devicesAlarmsLink answers the address of the estate's alarms filtered on
// one device.
func devicesAlarmsLink(request *PageRequest, name string) string {
	target := Address{Kind: ObjectEstate, Tab: devicesAlarmsTab, View: linkView(request)}
	target.View.Filter = SetFilterTerm("", devicesAlarmField, name)

	return target.String()
}

// devicesAlarmsCell fills the Alarms cell: the count of open alarms, a link
// to them when there is one.
func devicesAlarmsCell(request *PageRequest, view *DeviceView) CellView {
	if view.AlarmsOpen == 0 {
		return NumberCell(countText(0))
	}

	return LinkCell(countText(view.AlarmsOpen), devicesAlarmsLink(request, view.Device.Name))
}

// devicesTabLink answers the address of one tab of a device.
func devicesTabLink(request *PageRequest, name, tab string) string {
	target := deviceAddress(request, name)
	target.Tab = tab

	return target.String()
}

// devicesMenu is the Actions menu of a row and of the side panel: each item
// opens the device page where the action is taken.
func devicesMenu(request *PageRequest, name string) *MenuView {
	return &MenuView{
		Label: actionsMenuWord + " for " + name,
		Items: []ActionView{
			{Label: devicesWordOpenDevice, Icon: iconOpen, Href: deviceAddress(request, name).String()},
			{Label: devicesWordOpenConsole, Icon: glyphTerminal, Href: devicesTabLink(request, name, devicesConsoleTab)},
			{Label: devicesWordDiagnostic, Icon: iconTasks, Href: devicesTabLink(request, name, tabDeviceTasks)},
			{Label: devicesWordPushConfig, Icon: iconAdmin, Href: devicesTabLink(request, name, devicesConfigTab)},
		},
	}
}

// devicesPanel builds the side panel of the row the address names. A row
// that names no device of the table answers the device's not-found error,
// which the panel shows as its state.
func devicesPanel(_ context.Context, request *PageRequest) (templ.Component, error) {
	scope, err := devicesScope(request)
	if err != nil {
		return nil, err
	}

	view, err := FindDevice(scope, request.Address.View.Row)
	if err != nil {
		return nil, err
	}

	return PanelContent(devicesPanelView(request, view)), nil
}

// devicesPanelView fills the side panel of one device.
func devicesPanelView(request *PageRequest, view *DeviceView) PanelView {
	name := view.Device.Name
	closed := request.Address
	closed.View.Row = ""

	return PanelView{
		Title:     name,
		CloseHref: closed.String(),
		Icon:      devicesIcon(view),
		Actions: []ActionView{
			{Label: devicesWordOpenDevice, Icon: iconOpen, Href: deviceAddress(request, name).String()},
			{Label: devicesWordOpenConsole, Icon: glyphTerminal, Href: devicesTabLink(request, name, devicesConsoleTab)},
		},
		// The panel is 22rem wide: two actions and the menu hold one row, so
		// running a diagnostic and the rest stay in the menu, which lists them.
		Menu: devicesMenu(request, name),
		Cards: []PanelCardView{
			{Title: "Identity", Rows: devicesIdentityRows(view)},
			{Title: devicesWordLabels, Rows: []ValueView{{Label: devicesWordLabels, Whole: true, Tags: devicesTags(&view.Device)}}},
			{Title: devicesWordConnection, Rows: devicesConnectionRows(request, view)},
			{Title: devicesWordHealth, Rows: devicesHealthRows(view)},
			{Title: "Now", Rows: devicesNowRows(view)},
			{
				Title: wordOpenAlarms,
				Rows:  devicesAlarmRows(request, name),
				Link:  &LinkView{Label: "All alarms", Href: devicesAlarmsLink(request, name)},
			},
		},
	}
}

// devicesAddress writes an address, or the empty text, drawn as a dash, for
// one not known.
func devicesAddress(addr netip.Addr) string {
	if !addr.IsValid() {
		return ""
	}

	return addr.String()
}

// devicesIdentityRows fills the Identity card: each identifier with its copy
// control.
func devicesIdentityRows(view *DeviceView) []ValueView {
	inventory := &view.Device.Inventory

	return []ValueView{
		{Label: devicesWordName, Text: view.Device.Name, Mono: true, Copy: true},
		{Label: devicesWordState, Status: DisplayStatus(view.State)},
		{Label: devicesWordModel, Text: inventory.Model, Copy: true},
		{Label: devicesWordSerial, Text: inventory.Serial, Mono: true, Copy: true},
		{Label: devicesWordSoftware, Text: inventory.SoftwareVersion, Mono: true, Copy: true},
		{Label: devicesWordAddress, Text: devicesAddress(inventory.ManagementAddr), Mono: true, Copy: true},
	}
}

// devicesConnectionRows fills the Connection card: since when the device is
// connected, or when it was last seen; where it connects from; and its last
// configuration fetch.
func devicesConnectionRows(request *PageRequest, view *DeviceView) []ValueView {
	connection := &view.Device.Connection
	config := &view.Device.Config

	rows := make([]ValueView, 0, 5)
	if connection.Connected {
		rows = append(rows, ValueView{Label: "Connected since", Text: devicesExact(connection.Since)})
	} else {
		seen := ""
		if view.SeenKnown {
			seen = devicesExact(request.Snapshot.Now.Add(-view.SeenAgo))
		}
		rows = append(rows, ValueView{Label: devicesWordLastSeen, Text: seen})
	}

	fetched := ""
	if config.Fetched {
		fetched = devicesExact(config.LastFetch)
	}

	return append(rows,
		ValueView{Label: "Source address", Text: devicesAddress(connection.SourceAddr), Mono: true, Copy: true},
		ValueView{Label: "Last configuration fetch", Text: fetched},
		ValueView{Label: "Fetch result", Text: config.LastFetchResult},
		ValueView{Label: wordConfigTitle, Status: ConfigStatus(config.State)},
	)
}

// devicesHealthRows fills the Health card: one row per component the device
// reported, with its state word.
func devicesHealthRows(view *DeviceView) []ValueView {
	if !view.Device.HealthReported {
		return []ValueView{{Label: devicesWordHealth, Text: "Not reported"}}
	}
	if len(view.Device.Health) == 0 {
		return []ValueView{{Label: devicesWordHealth, Text: "No component reported"}}
	}

	rows := make([]ValueView, 0, len(view.Device.Health))
	for _, component := range view.Device.Health {
		rows = append(rows, ValueView{Label: component.Name, Status: HealthStatus(component.State)})
	}

	return rows
}

// devicesNowRows fills the Now card: the usage bars of the last sample and
// the uptime.
func devicesNowRows(view *DeviceView) []ValueView {
	latest := &view.Device.Latest

	return []ValueView{
		{Label: devicesWordCPU, Usage: devicesUsageBar(devicesWordCPU, latest.CPUPercent, latest.Reported)},
		{Label: devicesWordMemory, Usage: devicesUsageBar(devicesWordMemory, latest.MemoryPercent, latest.Reported)},
		{Label: devicesWordStorage, Usage: devicesUsageBar(devicesWordStorage, latest.StoragePercent, latest.Reported)},
		{Label: devicesWordUptime, Text: devicesUptime(view)},
	}
}

// devicesUsageBar answers the usage bar of a percentage the device reported.
func devicesUsageBar(name string, percent float64, reported bool) *UsageBarView {
	whole, known := Percent(percent, widthPercentMax)

	return &UsageBarView{Name: name, Percent: whole, Known: known && reported}
}

// devicesAlarmRows fills the Open alarms card: the most severe open alarms of
// the device, the most severe first.
func devicesAlarmRows(request *PageRequest, name string) []ValueView {
	own := make([]Alarm, 0, devicesPanelAlarmsMax)
	for index := range request.Snapshot.Alarms {
		if request.Snapshot.Alarms[index].Device == name {
			own = append(own, request.Snapshot.Alarms[index])
		}
	}
	if len(own) == 0 {
		return []ValueView{{Label: devicesWordAlarms, Text: "No alarm open"}}
	}

	top := TopAlarms(own, devicesPanelAlarmsMax)
	rows := make([]ValueView, 0, len(top))
	for index := range top {
		label := top[index].Summary
		if label == "" {
			label = top[index].Kind
		}
		rows = append(rows, ValueView{Label: label, Status: AlarmStatus(top[index].Severity)})
	}

	return rows
}
