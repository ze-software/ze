// Design: docs/architecture/fleet-interface.md -- the Interfaces tab of a device
// Related: register_device_pages.go -- registers the page, its section fragment and the tool fragment
// Related: page_device.templ -- deviceTab and deviceSection, which draw it
// Related: component_table.templ -- the table stack the interfaces are listed in
//
// The Interfaces tab lists the device's interfaces with their live state and
// counters, asked through the relay and fetched again every five seconds. The
// table filters, sorts and pages through its address. A row's side panel
// shows its full counters, and a row's related tools answer, in the
// prototype, a toast saying nothing ran.

package fleetui

import (
	"cmp"
	"context"
	"net/netip"
	"strings"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// titleInterfaces is the title of the Interfaces tab's one section.
const titleInterfaces = "Interfaces"

// interfacesSubject names the rows of the interfaces table in a refusal.
const interfacesSubject = "interfaces"

// The names of the interface fields a filter term and a sort name, beyond
// the devices table's fieldName and fieldState.
const (
	fieldType   = "type"
	fieldSpeed  = "speed"
	fieldMTU    = "mtu"
	fieldIn     = "in"
	fieldOut    = "out"
	fieldErrors = "errors"
	fieldDrops  = "drops"
)

// The words of an interface's state, as its cells and its filter write them.
const (
	interfaceUp        = "up"
	interfaceDown      = "down"
	interfaceAdminDown = "admin-down"
)

// interfaceStateKeys are the values the state field of an interface filter
// takes.
var interfaceStateKeys = []string{interfaceUp, interfaceDown, interfaceAdminDown}

// The related tools of an interface, as the tool fragment's item names them.
const (
	toolCounters = "counters"
	toolPing     = "ping"
)

// interfaceToolTitles names each related tool of an interface.
var interfaceToolTitles = map[string]string{
	toolCounters: "Counters detail",
	toolPing:     "Ping from the interface",
}

// The filter grammar and the sortable columns of the interfaces table.
// registerInterfaceTable fills both from init().
var (
	interfaceFilters = NewFilterSchema[Interface](interfacesSubject, interfaceContains, nil)
	interfaceColumns = NewColumnSet[Interface](interfacesSubject)
)

// registerInterfaceTable registers the fields and the sortable columns of the
// interfaces table. The first column, the name, is the default sort.
func registerInterfaceTable() {
	interfaceFilters.MustRegisterField(TextField(fieldState, interfaceStateKeys, interfaceState))
	interfaceFilters.MustRegisterField(TextField(fieldType, nil, func(row *Interface) string { return row.Kind }))

	for _, column := range []Column[Interface]{
		{Key: fieldName, Compare: func(a, b *Interface) int { return cmp.Compare(a.Name, b.Name) }},
		{Key: fieldType, Compare: func(a, b *Interface) int { return cmp.Compare(a.Kind, b.Kind) }},
		{Key: fieldState, Compare: func(a, b *Interface) int { return cmp.Compare(interfaceState(a), interfaceState(b)) }},
		{Key: fieldSpeed, Compare: func(a, b *Interface) int { return cmp.Compare(a.SpeedBps, b.SpeedBps) }},
		{Key: fieldMTU, Compare: func(a, b *Interface) int { return cmp.Compare(a.MTU, b.MTU) }},
		{Key: fieldIn, Compare: func(a, b *Interface) int { return cmp.Compare(a.InBps, b.InBps) }},
		{Key: fieldOut, Compare: func(a, b *Interface) int { return cmp.Compare(a.OutBps, b.OutBps) }},
		{Key: fieldErrors, Compare: func(a, b *Interface) int { return cmp.Compare(a.Errors, b.Errors) }},
		{Key: fieldDrops, Compare: func(a, b *Interface) int { return cmp.Compare(a.Drops, b.Drops) }},
	} {
		interfaceColumns.MustRegister(column)
	}
}

// interfaceState answers an interface's state in a word: admin-down when an
// operator shut it, otherwise its link state.
func interfaceState(row *Interface) string {
	if !row.AdminUp {
		return interfaceAdminDown
	}
	if row.LinkUp {
		return interfaceUp
	}

	return interfaceDown
}

// interfaceContains reports whether an interface's name, type or one of its
// addresses holds needle, which is lower case.
func interfaceContains(row *Interface, needle string) bool {
	if strings.Contains(strings.ToLower(row.Name), needle) || strings.Contains(strings.ToLower(row.Kind), needle) {
		return true
	}
	for _, address := range row.Addresses {
		if strings.Contains(address.String(), needle) {
			return true
		}
	}

	return false
}

// interfacesContent builds the Interfaces tab: its one section, and the form
// its rows' related tools submit.
func interfacesContent(ctx context.Context, request *PageRequest) (templ.Component, error) {
	view, err := deviceOf(request)
	if err != nil {
		return nil, err
	}

	return deviceTab(DeviceTabView{
		Label:    "Interfaces of " + view.Device.Name,
		ToolForm: toolFormLink(request),
		Sections: []DeviceSectionView{{
			Title: titleInterfaces,
			URL:   sectionLink(request, fragmentDeviceInterfaces),
			Every: everyInterfaces,
			Wide:  true,
			Body:  interfacesBody(ctx, request, view),
		}},
	}), nil
}

// interfacesFragment answers the section of the Interfaces tab.
func interfacesFragment(ctx context.Context, request *PageRequest, _ string) (templ.Component, error) {
	view, err := deviceOf(request)
	if err != nil {
		return nil, err
	}

	return interfacesBody(ctx, request, view), nil
}

// interfacesBody builds the interfaces table, or the state of the section
// when the device cannot answer.
func interfacesBody(ctx context.Context, request *PageRequest, view *DeviceView) templ.Component {
	interfaces, err := request.Source.DeviceInterfaces(ctx, view.Device.Name)
	if err != nil {
		return sectionState(request, titleInterfaces, err)
	}
	interfaces = interfaces[:min(len(interfaces), InterfacesMax)]

	address := request.Address
	query := &QueryRowView{
		ID:          "fleet-interfaces-query",
		Action:      address.Path(),
		Text:        address.View.Filter,
		Placeholder: "Filter: a name, state:down, type:ethernet",
		Hidden:      carriedFields(address, paramFilter, paramPage, paramRow),
		Menus:       []MenuView{filterMenu(address, "State", fieldState, interfaceStateKeys)},
	}

	matched := interfaces
	filter, err := interfaceFilters.Parse(address.View.Filter, nil)
	if err == nil {
		matched = filter.Apply(interfaces)
	} else {
		query.Refusal = refusalOf(err).Problem()
	}

	sorted, err := interfaceColumns.Sort(matched, address.View.Sort)
	if err != nil {
		sorted = matched
		query.Refusal = refusalOf(err).Problem()
	}

	page := Paginate(len(sorted), address.View.Page, address.View.Size)
	query.Matches = matchesText(len(sorted), len(interfaces), interfacesSubject)

	shown := PageRows(sorted, page)
	rows := make([]RowView, 0, len(shown))
	for index := range shown {
		rows = append(rows, interfaceRow(address, &shown[index]))
	}

	empty := &StateHeroView{Kind: HeroNoData, Sentence: "No interface matches the filter."}
	if len(interfaces) == 0 {
		empty = &StateHeroView{Kind: HeroNoData, Sentence: view.Device.Name + " reported no interface."}
	}

	return TableSection(TableSectionView{
		Title: titleInterfaces,
		Query: query,
		Pager: pagerView(address, interfacesSubject, page),
		Table: TableView{
			Name:     "Interfaces of " + view.Device.Name,
			Headers:  interfaceHeaders(address),
			Rows:     rows,
			Empty:    empty,
			Controls: true,
		},
	})
}

// matchesText writes how many rows a filter kept: "4 of 12 interfaces".
func matchesText(kept, total int, noun string) string {
	return textbuf.New().Int(int64(kept)).Str(" of ").Int(int64(total)).Byte(' ').Str(noun).String()
}

// interfaceHeaders answers the header cells of the interfaces table.
func interfaceHeaders(address Address) []ColumnHeaderView {
	return []ColumnHeaderView{
		sortHeader(address, devicesWordName, ColumnKindText, fieldName, false),
		sortHeader(address, "Type", ColumnKindText, fieldType, false),
		{Label: "Admin", Kind: ColumnKindState},
		sortHeader(address, "Link", ColumnKindState, fieldState, false),
		{Label: "Addresses", Kind: ColumnKindAddress},
		sortHeader(address, "Speed", ColumnKindNumber, fieldSpeed, true),
		sortHeader(address, "MTU", ColumnKindNumber, fieldMTU, true),
		sortHeader(address, "In", ColumnKindNumber, fieldIn, true),
		sortHeader(address, "Out", ColumnKindNumber, fieldOut, true),
		sortHeader(address, "Errors", ColumnKindNumber, fieldErrors, true),
		sortHeader(address, "Drops", ColumnKindNumber, fieldDrops, true),
	}
}

// adminStatus answers how an interface's administrative state is drawn.
func adminStatus(row *Interface) Status {
	if row.AdminUp {
		return Status{Tone: ToneSuccess, Word: interfaceUp}
	}

	return Status{Tone: ToneMuted, Word: interfaceDown}
}

// linkStatus answers how an interface's link state is drawn: a link that is
// down while the interface is enabled is an error, one an operator shut is
// not.
func linkStatus(row *Interface) Status {
	switch {
	case !row.AdminUp:
		return Status{Tone: ToneMuted, Word: interfaceAdminDown}
	case row.LinkUp:
		return Status{Tone: ToneSuccess, Word: interfaceUp}
	default:
		return Status{Tone: ToneError, Word: interfaceDown}
	}
}

// addressesText writes an interface's addresses, comma separated.
func addressesText(addresses []netip.Prefix) string {
	text := textbuf.New()
	for index, address := range addresses {
		if index > 0 {
			text.Str(", ")
		}
		text.Prefix(address)
	}

	return text.String()
}

// speedText writes an interface's speed, or nothing when the device gave
// none.
func speedText(row *Interface) string {
	if row.SpeedBps == 0 {
		return ""
	}

	return rateText(float64(row.SpeedBps))
}

// mtuText writes an interface's MTU, or nothing when the device gave none.
func mtuText(row *Interface) string {
	if row.MTU <= 0 {
		return ""
	}

	return textbuf.StringInt(int64(row.MTU))
}

// interfaceRow answers one row of the interfaces table.
func interfaceRow(address Address, row *Interface) RowView {
	return RowView{
		ID:      row.Name,
		Name:    row.Name,
		EyeHref: openRowAddress(address, row.Name),
		Opened:  address.View.Row == row.Name,
		Menu:    &MenuView{Label: "Tools for " + row.Name, Icon: "dots", Items: interfaceTools(row.Name)},
		Cells: []CellView{
			MonoCell(row.Name),
			TextCell(row.Kind),
			StatusCell(adminStatus(row)),
			StatusCell(linkStatus(row)),
			MonoCell(addressesText(row.Addresses)),
			NumberCell(speedText(row)),
			NumberCell(mtuText(row)),
			NumberCell(rateText(float64(row.InBps))),
			NumberCell(rateText(float64(row.OutBps))),
			NumberCell(uintText(row.Errors)),
			NumberCell(uintText(row.Drops)),
		},
	}
}

// interfaceTools answers the related tools of the interface named name: each
// submits the tool form with the tool and the interface as its item.
func interfaceTools(name string) []ActionView {
	return []ActionView{
		{Label: interfaceToolTitles[toolCounters], Icon: "chart", Form: deviceToolFormID, Name: paramItem, Value: toolCounters + "/" + name},
		{Label: interfaceToolTitles[toolPing], Icon: glyphTerminal, Form: deviceToolFormID, Name: paramItem, Value: toolPing + "/" + name},
	}
}

// toolFormLink answers the address the tool form asks: the tool fragment of
// the page in hand, with no view parameter, so the form's item is the only
// one.
func toolFormLink(request *PageRequest) string {
	page := Address{Kind: ObjectDevice, Device: request.Address.Device, Tab: request.Address.Tab}

	return FragmentLink(fragmentDeviceTool, page, "")
}

// toolFragment answers a related tool run from a row: in the prototype, the
// toast saying nothing ran, naming the tool and its target.
func toolFragment(_ context.Context, request *PageRequest, item string) (templ.Component, error) {
	view, err := deviceOf(request)
	if err != nil {
		return nil, err
	}

	tool, target, _ := strings.Cut(item, "/")
	title, known := interfaceToolTitles[tool]
	if !known || target == "" {
		problem := textbuf.New().Str("The interface tools have no tool named ").Quoted(tool).Str(".").String()
		refusal := opererr.New(problem, "The row asked for a tool this page does not offer, so nothing ran.",
			"choose a tool from the row's menu")

		return Toast(ToastView{Message: refusal.Problem(), Detail: refusal.Why(), Level: ToastWarning}), nil
	}

	detail := textbuf.New().Str(title).Str(" of ").Str(target).Str(" on ").Str(view.Device.Name).
		Str(" runs on the device through the command relay, which the prototype does not have.").String()

	return Toast(ToastView{Message: "The prototype changed nothing", Detail: detail, Level: ToastInformational}), nil
}

// interfacePanel builds the side panel of the interface the address names:
// its state and its full counters. The hub's history holds no series per
// interface, so the panel draws no traffic chart.
func interfacePanel(ctx context.Context, request *PageRequest) (templ.Component, error) {
	view, err := deviceOf(request)
	if err != nil {
		return nil, err
	}

	interfaces, err := request.Source.DeviceInterfaces(ctx, view.Device.Name)
	if err != nil {
		return nil, err
	}

	name := request.Address.View.Row
	for index := range interfaces {
		row := &interfaces[index]
		if row.Name != name {
			continue
		}

		return PanelContent(PanelView{
			Title:     row.Name,
			CloseHref: closedAddress(request.Address),
			Icon:      ObjectIconView{Icon: iconDevice, Status: linkStatus(row), Marked: true},
			Actions:   interfaceTools(row.Name),
			Cards: []PanelCardView{
				{Title: "State", Rows: []ValueView{
					{Label: "Type", Text: row.Kind},
					{Label: "Administrative state", Status: adminStatus(row)},
					{Label: "Link state", Status: linkStatus(row)},
					{Label: "Addresses", Text: addressesText(row.Addresses), Mono: true, Copy: true},
					{Label: "Speed", Text: speedText(row)},
					{Label: "MTU", Text: mtuText(row)},
				}},
				{Title: "Counters", Rows: []ValueView{
					{Label: "Traffic in", Text: rateText(float64(row.InBps))},
					{Label: "Traffic out", Text: rateText(float64(row.OutBps))},
					{Label: "Errors", Text: uintText(row.Errors)},
					{Label: "Drops", Text: uintText(row.Drops)},
				}},
			},
		}), nil
	}

	problem := textbuf.New().Str(view.Device.Name).Str(" has no interface named ").Quoted(name).Str(".").String()

	return nil, opererr.Wrap(ErrObjectUnknown, problem,
		"The address names an interface the device did not report, so there is nothing to show.",
		"open "+closedAddress(request.Address))
}
