// Design: docs/architecture/fleet-interface.md -- the cards of the device dashboard
// Related: handler_dashboard.go -- the card registry, the scope and the offline state
// Related: handler_dashboard_cards.go -- the cards the device dashboard shares with the estate's
// Related: page_dashboard.templ -- the markup of the cards no shared component draws
//
// Every card here but Quick info and the charts is live: it shows what the
// device reports now, so the dashboard of an offline device leaves it out.

package fleetui

import (
	"context"
	"math"
	"time"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The filter values of the peers and interfaces tables a donut segment of
// the device dashboard opens.
const (
	dashPeerEstablished = "established"
	dashStateDown       = "down"
	dashStateAdminDown  = "admin-down"
	dashInterfaceUp     = "up"
)

// dashPeerNotEstablished is the state filter value of the peers that are
// neither established nor administratively down.
const dashPeerNotEstablished = filterNegation + dashPeerEstablished + filterListSeparator + dashStateAdminDown

// dashboardNoSample is the line of a card over the last sample of a device
// that has reported none.
const dashboardNoSample = "This device has not reported a sample yet."

// dashboardQuickInfoCard is the Quick info card of a device.
func dashboardQuickInfoCard(_ context.Context, _ *PageRequest, scope *dashboardScope, frame CardView) (templ.Component, error) {
	return QuickInfo(quickInfoView(scope, frame)), nil
}

// quickInfoView answers the Quick info card: the device's identity, state,
// software, configuration and reachability, each identifier with a copy
// control. A value the device has not reported is left empty, which the
// card draws as a dash.
func quickInfoView(scope *dashboardScope, frame CardView) QuickInfoView {
	device := scope.device
	facts := &device.Device

	labels := make([]TagView, 0, len(facts.Labels))
	for _, label := range facts.Labels {
		labels = append(labels, TagView{Key: label.Key, Value: label.Value})
	}

	uptime := ""
	if device.UptimeKnown {
		uptime = durationWords(device.Uptime)
	}

	var seen string
	switch {
	case facts.Connection.Connected:
		seen = "now"
	case device.SeenKnown:
		seen = RelativeTime(device.SeenAgo)
	default:
		seen = "never"
	}

	address := ""
	if facts.Inventory.ManagementAddr.IsValid() {
		address = facts.Inventory.ManagementAddr.String()
	}

	frame.TitleHref = scope.tab(tabDeviceSystem, "").String()

	return QuickInfoView{Card: frame, Items: []ValueView{
		{Label: devicesWordName, Text: facts.Name, Mono: true, Copy: true},
		{Label: "State", Status: DisplayStatus(device.State)},
		{Label: devicesWordLabels, Tags: labels},
		{Label: devicesWordModel, Text: facts.Inventory.Model},
		{Label: "Serial number", Text: facts.Inventory.Serial, Mono: true, Copy: true},
		{Label: "Software version", Text: deviceVersion(device), Mono: true},
		{Label: "Uptime", Text: uptime},
		{Label: wordConfigTitle, Status: ConfigStatus(facts.Config.State)},
		{Label: "Configuration version", Text: facts.Config.HubVersion, Mono: true, Copy: true},
		{Label: devicesWordLastSeen, Text: seen},
		{Label: devicesWordAddress, Text: address, Mono: true, Copy: true},
	}}
}

// durationWords writes how long something has lasted in its largest whole
// unit: "41 days", "3 hours", "12 minutes", "less than a minute".
func durationWords(length time.Duration) string {
	switch {
	case length >= 24*time.Hour:
		return unitWords(int64(length/(24*time.Hour)), "day")
	case length >= time.Hour:
		return unitWords(int64(length/time.Hour), "hour")
	case length >= time.Minute:
		return unitWords(int64(length/time.Minute), "minute")
	default:
		return "less than a minute"
	}
}

// unitWords writes a count of one unit, plural beyond one.
func unitWords(count int64, unit string) string {
	text := textbuf.New().Int(count).Byte(' ').Str(unit)
	if count != 1 {
		text.Byte('s')
	}

	return text.Slice()
}

// dashboardDeviceConfigCard is the Configuration card of a device. A device
// that diverged carries the divergence callout, which asks the hub how many
// changes were made on the device.
func dashboardDeviceConfigCard(ctx context.Context, request *PageRequest, scope *dashboardScope, frame CardView) (templ.Component, error) {
	view := deviceConfigView(request, scope, frame)

	facts := &scope.device.Device
	if facts.Config.State == ConfigStateDiverged {
		divergence, err := request.Source.Divergence(ctx, facts.Name)
		if err != nil {
			return nil, err
		}
		view.Divergence = divergenceCallout(request, scope, &divergence)
	}

	return dashboardConfig(view), nil
}

// deviceConfigView answers the Configuration card without its callout.
func deviceConfigView(request *PageRequest, scope *dashboardScope, frame CardView) DashboardConfigView {
	config := &scope.device.Device.Config

	fetched := "never"
	if config.Fetched {
		fetched = RelativeTime(request.Snapshot.Now.Sub(config.LastFetch))
	}

	frame.TitleHref = scope.tab(dashTabConfig, "").String()

	return DashboardConfigView{Card: frame, Rows: []ValueView{
		{Label: "State", Status: ConfigStatus(config.State)},
		{Label: "Hub version", Text: config.HubVersion, Mono: true},
		{Label: "Device version", Text: config.DeviceVersion, Mono: true},
		{Label: "Template", Text: config.Template},
		{Label: "Last fetch", Text: fetched},
		{Label: "Fetch result", Text: config.LastFetchResult},
	}}
}

// divergenceCallout answers the banner of a device whose configuration was
// edited on the device.
func divergenceCallout(request *PageRequest, scope *dashboardScope, divergence *Divergence) *CalloutView {
	text := textbuf.New().
		Str("The device holds ").Str(changesText(divergence.Changes)).
		Str(" made on the device ").Str(RelativeTime(request.Snapshot.Now.Sub(divergence.EditedAt))).
		Str(", so its configuration is not the hub's.").Slice()

	return &CalloutView{
		Title: "Configuration diverged",
		Text:  text,
		Link:  &LinkView{Label: "Compare the two", Href: scope.tab(dashTabConfig, "").String()},
	}
}

// changesText answers "1 change" or "3 changes".
func changesText(count int) string {
	return unitWords(int64(count), "change")
}

// dashboardPeersCard is the Peers card: the device's BGP peers by state in
// its last sample.
func dashboardPeersCard(_ context.Context, _ *PageRequest, scope *dashboardScope, frame CardView) (templ.Component, error) {
	latest := &scope.device.Device.Latest
	frame.TitleHref = scope.tab(dashTabRouting, "").String()
	if !latest.Reported {
		return dashboardNote(DashboardNoteView{Card: frame, Text: dashboardNoSample}), nil
	}

	routing := func(state string) string {
		return scope.tab(dashTabRouting, SetFilterTerm("", fieldState, state)).String()
	}

	return DonutCard(DonutCardView{Card: frame, Donuts: []DonutView{{Icon: iconDevice, Segments: []DonutSegmentView{
		{Word: "Established", Href: routing(dashPeerEstablished), Count: latest.PeersEstablished, Tone: ToneSuccess},
		// A peer that is not established is in whatever state the device
		// names, so the link asks for none of the other two segments' words.
		{Word: "Not established", Href: routing(dashPeerNotEstablished), Count: latest.PeersDown, Tone: ToneError},
		{Word: "Administratively down", Href: routing(dashStateAdminDown), Count: latest.PeersAdminDown, Tone: ToneMuted},
	}}}}), nil
}

// dashboardInterfacesCard is the Interfaces card: the device's interfaces by
// state in its last sample.
func dashboardInterfacesCard(_ context.Context, _ *PageRequest, scope *dashboardScope, frame CardView) (templ.Component, error) {
	latest := &scope.device.Device.Latest
	frame.TitleHref = scope.tab(dashTabInterfaces, "").String()
	if !latest.Reported {
		return dashboardNote(DashboardNoteView{Card: frame, Text: dashboardNoSample}), nil
	}

	interfaces := func(state string) string {
		return scope.tab(dashTabInterfaces, SetFilterTerm("", fieldState, state)).String()
	}

	return DonutCard(DonutCardView{Card: frame, Donuts: []DonutView{{Icon: iconDevice, Segments: []DonutSegmentView{
		{Word: "Up", Href: interfaces(dashInterfaceUp), Count: latest.InterfacesUp, Tone: ToneSuccess},
		{Word: enumWordDown, Href: interfaces(dashStateDown), Count: latest.InterfacesDown, Tone: ToneError},
		{Word: "Administratively down", Href: interfaces(dashStateAdminDown), Count: latest.InterfacesAdminDown, Tone: ToneMuted},
	}}}}), nil
}

// dashboardMemoryCard is the Memory card: the use of the device's memory in
// its last sample, with used and total.
func dashboardMemoryCard(_ context.Context, _ *PageRequest, scope *dashboardScope, frame CardView) (templ.Component, error) {
	latest := &scope.device.Device.Latest

	return usageCard(scope, frame, latest.Reported, latest.MemoryPercent, latest.MemoryUsedBytes, latest.MemoryTotalBytes), nil
}

// dashboardStorageCard is the Storage card: the use of the device's storage
// in its last sample, with used and total.
func dashboardStorageCard(_ context.Context, _ *PageRequest, scope *dashboardScope, frame CardView) (templ.Component, error) {
	latest := &scope.device.Device.Latest

	return usageCard(scope, frame, latest.Reported, latest.StoragePercent, latest.StorageUsedBytes, latest.StorageTotalBytes), nil
}

// usageCard answers a card of one usage bar with its used and total figures,
// or the line of a device that has reported no sample.
func usageCard(scope *dashboardScope, frame CardView, reported bool, percent float64, used, total uint64) templ.Component {
	frame.TitleHref = scope.tab(tabDeviceSystem, "").String()
	if !reported {
		return dashboardNote(DashboardNoteView{Card: frame, Text: dashboardNoSample})
	}

	usedValue, usedUnit := bytesFigure(used)
	totalValue, totalUnit := bytesFigure(total)

	return UsageGroup(UsageGroupView{Card: frame, Columns: []UsageColumnView{{
		Bar: UsageBarView{Name: "Used", Percent: int(math.Round(percent)), Known: true, Tall: true},
		Foot: []FigureView{
			{Caption: "Used", Value: usedValue, Unit: usedUnit},
			{Caption: "Total", Value: totalValue, Unit: totalUnit},
		},
	}}})
}
