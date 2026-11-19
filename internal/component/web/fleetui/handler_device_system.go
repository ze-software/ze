// Design: docs/architecture/fleet-interface.md -- the System tab of a device
// Related: register_device_pages.go -- registers the page and one fragment per section
// Related: page_device.templ -- deviceTab, deviceSection and deviceFacts, which draw it
// Related: source_views.go -- DeviceSystem, the relayed part of the tab
//
// The System tab says what the device is. Identity, Health, Labels and group
// and Management come from the facts the hub holds, so they still show for
// an offline device, marked with when they were reported. Hardware,
// Resources and the device's clock are asked of the device through the
// relay, and each shows its own state when the device cannot answer.

package fleetui

import (
	"context"
	"net/netip"
	"slices"
	"time"

	"github.com/a-h/templ"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The titles of the System tab's sections.
const (
	titleIdentity   = "Identity"
	titleHardware   = "Hardware"
	titleResources  = "Resources"
	titleHealth     = "Health"
	titleLabels     = "Labels and group"
	titleManagement = "Management"
)

// httpsPort is the port of the device's own web interface.
const httpsPort = 443

// systemSection is one section of the System tab: its fragment, its title,
// the interval it is fetched again at, and how its content is built. build
// always answers a component: a section that cannot show its data shows the
// state that says why.
type systemSection struct {
	fragment string
	title    string
	every    string
	wide     bool
	build    func(ctx context.Context, request *PageRequest, view *DeviceView) templ.Component
}

// systemSections are the sections of the System tab, in the order it shows
// them.
var systemSections = []systemSection{
	{fragment: fragmentDeviceIdentity, title: titleIdentity, build: identitySection},
	{fragment: fragmentDeviceHardware, title: titleHardware, build: hardwareSection},
	{fragment: fragmentDeviceResources, title: titleResources, every: everyResources, wide: true, build: resourcesSection},
	{fragment: fragmentDeviceHealth, title: titleHealth, build: healthSection},
	{fragment: fragmentDeviceLabels, title: titleLabels, build: labelsSection},
	{fragment: fragmentDeviceManagement, title: titleManagement, build: managementSection},
}

// systemContent builds the System tab: every section, each with its own
// state.
func systemContent(ctx context.Context, request *PageRequest) (templ.Component, error) {
	view, err := deviceOf(request)
	if err != nil {
		return nil, err
	}

	sections := make([]DeviceSectionView, 0, len(systemSections))
	for _, section := range systemSections {
		sections = append(sections, DeviceSectionView{
			Title: section.title,
			URL:   sectionLink(request, section.fragment),
			Every: section.every,
			Wide:  section.wide,
			Body:  section.build(ctx, request, view),
		})
	}

	return deviceTab(DeviceTabView{Label: "System of " + view.Device.Name, Sections: sections}), nil
}

// systemFragment answers the fragment builder of one section.
func systemFragment(section systemSection) FragmentBuilder {
	return func(ctx context.Context, request *PageRequest, _ string) (templ.Component, error) {
		view, err := deviceOf(request)
		if err != nil {
			return nil, err
		}

		return section.build(ctx, request, view), nil
	}
}

// consolePrimary is the System tab's primary action: the device's console.
func consolePrimary(request *PageRequest) (ActionView, bool) {
	target := deviceTabAddress(request, tabDeviceConsole)

	return ActionView{Label: devicesWordOpenConsole, Icon: glyphTerminal, Href: target.String()}, true
}

// identitySection shows what the device is, from its reported inventory, and
// its clock against the hub's, asked through the relay.
func identitySection(ctx context.Context, request *PageRequest, view *DeviceView) templ.Component {
	device := &view.Device
	inventory := &device.Inventory
	if !inventory.Reported {
		return sectionEmpty(titleIdentity, device.Name+" has not reported its inventory yet.")
	}

	uptime := ""
	if view.UptimeKnown {
		uptime = uptimeText(view.Uptime)
	}

	rows := slices.Concat([]ValueView{
		{Label: devicesWordName, Text: device.Name, Mono: true, Copy: true},
		{Label: devicesWordModel, Text: inventory.Model},
		{Label: devicesWordSerial, Text: inventory.Serial, Mono: true, Copy: true},
		{Label: devicesWordSoftware, Text: inventory.SoftwareVersion, Mono: true},
		{Label: "Build", Text: inventory.Build, Mono: true, Copy: true},
		{Label: "Kernel", Text: inventory.Kernel, Mono: true},
		{Label: devicesWordUptime, Text: uptime},
	}, clockRows(ctx, request, device.Name))

	return deviceFacts(DeviceFactsView{Title: titleIdentity, Reported: reportedNote(request, view), Rows: rows})
}

// clockRows answers the device's clock and its offset from the hub's, or the
// device's reason for not answering in place of the clock.
func clockRows(ctx context.Context, request *PageRequest, name string) []ValueView {
	system, err := request.Source.DeviceSystem(ctx, name)
	if err != nil {
		return []ValueView{{Label: "System time", Text: activityRefusalText(err)}}
	}

	return []ValueView{
		{Label: "System time", Text: exactTime(system.Time), Mono: true},
		{Label: "Offset from the hub", Text: offsetText(system.Time.Sub(request.Snapshot.Now))},
	}
}

// systemOffsetNone is the word written for a clock that agrees with the hub's.
const systemOffsetNone = "none"

// offsetText writes how far the device's clock is from the hub's.
func offsetText(offset time.Duration) string {
	offset = offset.Round(time.Millisecond)
	if offset == 0 {
		return systemOffsetNone
	}
	if offset > 0 {
		return offset.String() + " ahead of the hub"
	}

	return (-offset).String() + " behind the hub"
}

// uptimeText writes a long duration in its two largest units, such as
// "41 days 3 hours" or "12 minutes".
func uptimeText(duration time.Duration) string {
	days := int64(duration / (24 * time.Hour))
	hours := int64(duration % (24 * time.Hour) / time.Hour)
	minutes := int64(duration % time.Hour / time.Minute)

	text := textbuf.New()
	switch {
	case days > 0:
		unitCount(text, days, "day").Byte(' ')
		unitCount(text, hours, "hour")
	case hours > 0:
		unitCount(text, hours, "hour").Byte(' ')
		unitCount(text, minutes, "minute")
	default:
		unitCount(text, minutes, "minute")
	}

	return text.String()
}

// unitCount writes a count of one unit, plural but for one.
func unitCount(text *textbuf.Buffer, count int64, unit string) *textbuf.Buffer {
	text.Int(count).Byte(' ').Str(unit)
	if count != 1 {
		text.Byte('s')
	}

	return text
}

// hardwareSection shows the device's hardware, asked through the relay.
func hardwareSection(ctx context.Context, request *PageRequest, view *DeviceView) templ.Component {
	system, err := request.Source.DeviceSystem(ctx, view.Device.Name)
	if err != nil {
		return sectionState(request, titleHardware, err)
	}
	if len(system.Hardware) == 0 {
		return sectionEmpty(titleHardware, view.Device.Name+" reported no hardware.")
	}

	shown := system.Hardware[:min(len(system.Hardware), HardwareItemsMax)]
	rows := make([]ValueView, 0, len(shown))
	for _, item := range shown {
		text := textbuf.New().Str(item.Description)
		if item.Detail != "" {
			text.Str(", ").Str(item.Detail)
		}
		rows = append(rows, ValueView{Label: item.Kind, Text: text.String()})
	}

	return deviceFacts(DeviceFactsView{Title: titleHardware, Rows: rows})
}

// resourcesSection shows the device's resource use now as usage bars, asked
// through the relay. The section is fetched again every ten seconds.
func resourcesSection(ctx context.Context, request *PageRequest, view *DeviceView) templ.Component {
	system, err := request.Source.DeviceSystem(ctx, view.Device.Name)
	if err != nil {
		return sectionState(request, titleResources, err)
	}

	resources := system.Resources
	cpu, cpuKnown := Percent(resources.CPUPercent, widthPercentMax)
	memory, memoryKnown := Percent(float64(resources.MemoryUsedBytes), float64(resources.MemoryTotalBytes))
	storage, storageKnown := Percent(float64(resources.StorageUsedBytes), float64(resources.StorageTotalBytes))
	load := textbuf.New().Float(resources.Load, 2).String()

	return UsageGroup(UsageGroupView{
		Card: CardView{Title: titleResources},
		Columns: []UsageColumnView{
			{
				Title: devicesWordCPU, Subtitle: "Load " + load,
				Bar:  UsageBarView{Name: devicesWordCPU, Percent: cpu, Known: cpuKnown, Tall: true},
				Foot: []FigureView{{Caption: "LOAD", Value: load}},
			},
			usageColumn("Memory", memory, memoryKnown, resources.MemoryUsedBytes, resources.MemoryTotalBytes),
			usageColumn("Storage", storage, storageKnown, resources.StorageUsedBytes, resources.StorageTotalBytes),
		},
	})
}

// usageColumn answers the column of one resource measured in bytes.
func usageColumn(title string, percent int, known bool, used, total uint64) UsageColumnView {
	return UsageColumnView{
		Title:    title,
		Subtitle: bytesText(used) + " of " + bytesText(total),
		Bar:      UsageBarView{Name: title, Percent: percent, Known: known, Tall: true},
		Foot: []FigureView{
			{Caption: "USED", Value: bytesText(used)},
			{Caption: "AVAILABLE", Value: bytesText(total - min(used, total))},
			{Caption: "TOTAL", Value: bytesText(total)},
		},
	}
}

// healthSection shows the device's health report: one row per component
// with its state word and detail.
func healthSection(_ context.Context, request *PageRequest, view *DeviceView) templ.Component {
	device := &view.Device
	if !device.HealthReported {
		return sectionEmpty(titleHealth, device.Name+" has sent no health report yet.")
	}

	rows := make([]RowView, 0, len(device.Health))
	for _, component := range device.Health {
		rows = append(rows, RowView{
			ID:   component.Name,
			Name: component.Name,
			Cells: []CellView{
				MonoCell(component.Name), StatusCell(HealthStatus(component.State)), TextCell(component.Detail),
			},
		})
	}

	table := &TableView{
		Name: "Health of " + device.Name,
		Headers: []ColumnHeaderView{
			{Label: "Component", Kind: ColumnKindText},
			{Label: columnWordState, Kind: ColumnKindState},
			{Label: wordDetail, Kind: ColumnKindText},
		},
		Rows: rows,
	}

	return deviceFacts(DeviceFactsView{Title: titleHealth, Reported: reportedNote(request, view), Table: table})
}

// labelsSection shows the device's labels as tags and its configuration
// group.
func labelsSection(_ context.Context, _ *PageRequest, view *DeviceView) templ.Component {
	device := &view.Device

	tags := make([]TagView, 0, len(device.Labels))
	for _, label := range device.Labels {
		tags = append(tags, TagView{Key: label.Key, Value: label.Value})
	}

	return deviceFacts(DeviceFactsView{Title: titleLabels, Rows: []ValueView{
		{Label: devicesWordLabels, Tags: tags},
		{Label: "Configuration group", Text: device.Group, Mono: true},
	}})
}

// managementSection shows how the hub reaches the device: its management
// address with a link to its own interface, its connection, and the profile
// it grants to relayed commands.
func managementSection(_ context.Context, request *PageRequest, view *DeviceView) templ.Component {
	device := &view.Device
	connection := &device.Connection

	address := ""
	var external *LinkView
	if managed := device.Inventory.ManagementAddr; managed.IsValid() {
		address = managed.String()
		external = &LinkView{
			Label: "Open the device's own interface",
			Href:  "https://" + netip.AddrPortFrom(managed, httpsPort).String() + "/",
		}
	}

	source := ""
	if connection.SourceAddr.IsValid() {
		source = connection.SourceAddr.String()
	}

	profile := "None: the device refuses relayed commands"
	if connection.RelayGranted {
		profile = connection.RelayProfile
	}

	return deviceFacts(DeviceFactsView{
		Title: titleManagement,
		Rows: []ValueView{
			{Label: devicesWordAddress, Text: address, Mono: true, Copy: true},
			{Label: "Connected since", Text: connectedText(request, connection)},
			{Label: "Connection from", Text: source, Mono: true, Copy: true},
			{Label: "Relay profile granted", Text: profile, Mono: connection.RelayGranted},
		},
		External: external,
	})
}

// connectedText writes since when the device is connected, or when it was
// last seen.
func connectedText(request *PageRequest, connection *Connection) string {
	now := request.Snapshot.Now
	if connection.Connected {
		return RelativeTime(now.Sub(connection.Since)) + " (" + exactTime(connection.Since) + ")"
	}
	if connection.LastSeen.IsZero() {
		return "Never connected"
	}

	return "Offline, last seen " + RelativeTime(now.Sub(connection.LastSeen)) + " (" + exactTime(connection.LastSeen) + ")"
}
