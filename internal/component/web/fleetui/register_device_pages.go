// Design: docs/architecture/fleet-interface.md -- the device tabs: System, Interfaces, Routing, Logs and Console
// Related: page_device.templ -- the layout of the device tabs and their sections
// Related: handler_device_system.go, handler_device_interfaces.go, handler_device_routing.go -- the tabs' builders
// Related: handler_device_logs.go, handler_device_console.go -- the logs and the console
// Related: pages.go -- the page and fragment registries these pages join
//
// The device tabs that show live state only the device holds register here:
// System, Interfaces, Routing, Logs and Console. Each section of a tab is a
// fragment of its own, so a section the device cannot answer shows its state
// and the others still render. The Dashboard, Config and Tasks tabs belong
// to other page families.

package fleetui

import (
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/component/web"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The device tabs this file registers pages for. tabDeviceSystem is
// shell_states.go's, which links to it, and tabDeviceConfig the Config
// page's, which the console links to.
const (
	tabDeviceConsole    = "console"
	tabDeviceInterfaces = "interfaces"
	tabDeviceRouting    = "routing"
	tabDeviceLogs       = "logs"
)

// The names of the fragments of the device tabs: one per section.
const (
	fragmentDeviceIdentity   = "device-identity"
	fragmentDeviceHardware   = "device-hardware"
	fragmentDeviceResources  = "device-resources"
	fragmentDeviceHealth     = "device-health"
	fragmentDeviceLabels     = "device-labels"
	fragmentDeviceManagement = "device-management"
	fragmentDeviceInterfaces = "device-interfaces"
	fragmentDeviceRouting    = "device-routing"
	fragmentDeviceLogs       = "device-logs"
	fragmentDeviceTool       = "device-tool"
)

// The intervals the timed sections are fetched again at while their tab is
// open, as hx-trigger writes them.
const (
	everyResources  = "10s"
	everyInterfaces = "5s"
)

// consolePattern is the route a console command is posted to.
const consolePattern = "POST " + consolePath

func init() {
	MustRegisterPage(Page{Kind: ObjectDevice, Tab: tabDeviceSystem, Content: systemContent, Primary: consolePrimary})
	MustRegisterPage(Page{
		Kind: ObjectDevice, Tab: tabDeviceInterfaces, Content: interfacesContent, SidePanel: interfacePanel,
	})
	MustRegisterPage(Page{Kind: ObjectDevice, Tab: tabDeviceRouting, Content: routingContent, SidePanel: peerPanel})
	MustRegisterPage(Page{Kind: ObjectDevice, Tab: tabDeviceLogs, Content: logsContent})
	// Static, because the terminal's scrollback is state the browser owns,
	// which a live event must not redraw.
	MustRegisterPage(Page{Kind: ObjectDevice, Tab: tabDeviceConsole, Content: consoleContent, Static: true})

	for _, section := range systemSections {
		MustRegisterFragment(section.fragment, systemFragment(section))
	}
	MustRegisterFragment(fragmentDeviceInterfaces, interfacesFragment)
	MustRegisterFragment(fragmentDeviceTool, toolFragment)
	MustRegisterFragment(fragmentDeviceRouting, routingFragment)
	MustRegisterFragment(fragmentDeviceLogs, logsFragment)

	registerInterfaceTable()
	registerPeerTable()
	registerLogTable()

	mustRegisterRoute(fleetRoute{
		pattern: consolePattern,
		wrap:    web.WrapMutation,
		build:   func(source Source) http.Handler { return newConsoleHandler(source) },
	})
}

// deviceOf answers the device the request's address names, from the
// request's snapshot. A fragment address is not checked by the shell, so
// every builder of this family asks here first.
func deviceOf(request *PageRequest) (*DeviceView, error) {
	if request.Address.Kind != ObjectDevice {
		problem := textbuf.New().Str("The address ").Quoted(request.Address.Path()).
			Str(" names no device.").String()

		return nil, opererr.Wrap(ErrAddressUnknown, problem,
			"A device tab answers for one device, and this address names another kind of object.",
			"open /fleet/estate/devices/")
	}

	return FindDevice(request.Snapshot.Views, request.Address.Device)
}

// deviceTabAddress answers the address of a tab of the request's device,
// keeping only the tree view.
func deviceTabAddress(request *PageRequest, tab string) Address {
	return Address{Kind: ObjectDevice, Device: request.Address.Device, Tab: tab, View: linkView(request)}
}

// sectionLink answers the fragment address of one section of the page in
// hand.
func sectionLink(request *PageRequest, fragment string) string {
	return FragmentLink(fragment, request.Address, "")
}

// sectionState answers the card a section shows in place of its data when
// its question failed with err: the section's title over the state FailureOf
// names, in the source's own words.
func sectionState(request *PageRequest, title string, err error) templ.Component {
	hero := failureHeroView(request, err)

	return deviceFacts(DeviceFactsView{Title: title, State: &hero})
}

// sectionEmpty answers the card a section shows when the device answered
// nothing to show.
func sectionEmpty(title, sentence string) templ.Component {
	return deviceFacts(DeviceFactsView{Title: title, State: &StateHeroView{Kind: HeroNoData, Sentence: sentence}})
}

// exactTime answers a time as a page writes it in full, in UTC.
func exactTime(at time.Time) string {
	return at.UTC().Format(time.DateTime) + " UTC"
}

// sinceCell answers the cell of a time as an age against the hub's clock. A
// time the device did not give is a dash.
func sinceCell(now, at time.Time) CellView {
	if at.IsZero() {
		return TextCell("")
	}

	return TimeCell(RelativeTime(now.Sub(at)), exactTime(at))
}

// reportedNote answers the note a section fed by hub-held facts carries for
// a device that is offline: when the facts were reported. A connected
// device's facts are current, and carry none.
func reportedNote(request *PageRequest, view *DeviceView) string {
	connection := view.Device.Connection
	if connection.Connected {
		return ""
	}
	if connection.LastSeen.IsZero() {
		return "The device is offline and was never seen: these values are the last the hub holds."
	}

	return textbuf.New().Str("The device is offline. These values were reported before it was last seen, ").
		Str(RelativeTime(request.Snapshot.Now.Sub(connection.LastSeen))).Str(" (").
		Str(exactTime(connection.LastSeen)).Str(").").String()
}

// uintText writes a count.
func uintText(value uint64) string {
	return textbuf.StringUint(value)
}

// sortHeader answers the header cell of a sortable column of the table at
// address: its sort address turns the sort around when the table is already
// sorted by it, and goes back to the first page.
func sortHeader(address Address, label string, kind ColumnKind, key string, numeric bool) ColumnHeaderView {
	header := ColumnHeaderView{Label: label, Kind: kind, Numeric: numeric}

	sorted := address.View.Sort.Column == key
	if sorted {
		header.Sort = SortMarkAscending
		if address.View.Sort.Descending {
			header.Sort = SortMarkDescending
		}
	}

	target := address
	target.View.Sort = SortSpec{Column: key, Descending: sorted && !address.View.Sort.Descending}
	target.View.Page = 1
	target.View.Row = ""
	header.SortHref = target.String()

	return header
}

// carriedFields answers the view parameters of address a form carries along,
// in a fixed order, but those named in drop.
func carriedFields(address Address, drop ...string) []HiddenFieldView {
	query := address.View.Query()
	for _, name := range drop {
		query.Del(name)
	}

	fields := make([]HiddenFieldView, 0, len(query))
	for _, name := range slices.Sorted(maps.Keys(query)) {
		fields = append(fields, HiddenFieldView{Name: name, Value: query.Get(name)})
	}

	return fields
}

// filterMenu answers an assisted filter menu: one item per value, each
// opening the table with the term written into its filter.
func filterMenu(address Address, label, field string, values []string) MenuView {
	menu := MenuView{Label: label, Icon: "filter", Items: make([]ActionView, 0, len(values))}
	for _, value := range values {
		target := address
		target.View.Filter = SetFilterTerm(address.View.Filter, field, value)
		target.View.Page = 1
		target.View.Row = ""
		menu.Items = append(menu.Items, ActionView{Label: value, Href: target.String()})
	}

	return menu
}

// pagerView answers the pager of the table at address over page.
func pagerView(address Address, noun string, page PageWindow) *PagerView {
	at := func(number int) string {
		target := address
		target.View.Page = number
		target.View.Row = ""

		return target.String()
	}

	return &PagerView{
		Noun:      noun,
		FirstHref: at(1),
		PrevHref:  at(max(page.Number-1, 1)),
		NextHref:  at(min(page.Number+1, page.Pages)),
		LastHref:  at(page.Pages),
		Action:    address.Path(),
		Hidden:    carriedFields(address, paramSize, paramPage, paramRow),
		Page:      page,
	}
}

// openRowAddress answers the address of the table at address with the row named
// name opened in the side panel.
func openRowAddress(address Address, name string) string {
	target := address
	target.View.Row = name

	return target.String()
}

// closedAddress answers the address of the table at address with no row
// opened.
func closedAddress(address Address) string {
	target := address
	target.View.Row = ""

	return target.String()
}

// subTabsView answers the row of sub-tabs of the tab at address, with the
// one named current marked.
func subTabsView(address Address, label string, keys, titles []string, current string) *DeviceSubTabsView {
	row := &DeviceSubTabsView{Label: label, Tabs: make([]DeviceSubTabView, 0, len(keys))}
	for index, key := range keys {
		target := Address{Kind: address.Kind, Device: address.Device, Tab: address.Tab}
		target.View.TreeView = address.View.TreeView
		target.View.Sub = key
		row.Tabs = append(row.Tabs, DeviceSubTabView{Title: titles[index], Href: target.String(), Current: key == current})
	}

	return row
}

// subTabUnknown is the refusal for a sub-tab the tab does not have.
func subTabUnknown(tab, key string, keys []string) error {
	problem := textbuf.New().Str("The ").Str(tab).Str(" tab has no view named ").Quoted(key).Str(".").String()

	return opererr.Wrap(ErrAddressUnknown, problem,
		"The address names a view this device does not answer, so there is nothing to show.",
		textbuf.New().Str("choose one of: ").Join(keys, ", ").String())
}
