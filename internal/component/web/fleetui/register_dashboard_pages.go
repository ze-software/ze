// Design: docs/architecture/fleet-interface.md -- the dashboards of the estate, of a group node and of a device
// Related: handler_dashboard.go -- the card registry and the dashboard builder
// Related: handler_dashboard_cards.go, handler_dashboard_device.go, handler_chart.go -- the cards
//
// The dashboard is the default tab of every object. Each dashboard is the
// ordered list of the cards registered for its kind of object, with their
// widths: a later feature adds a card here without editing the dashboard.

package fleetui

// The names of the cards, the item of their fragment address.
const (
	cardDevices       = "devices"
	cardConfiguration = "configuration"
	cardSoftware      = "software"
	cardEstate        = "estate"
	cardGroup         = "group"
	cardAlarms        = "alarms"
	cardAttention     = "attention"
	cardRollouts      = "rollouts"
	cardChanges       = "changes"
	cardBusiest       = "busiest"
	cardQuickInfo     = "quick-info"
	cardPeers         = "peers"
	cardInterfaces    = "interfaces"
	cardMemory        = "memory"
	cardStorage       = "storage"
	cardChartPrefix   = "chart-"
)

// The titles of the cards that more than one dashboard shows.
const (
	titleDevices       = "Devices"
	titleConfiguration = "Configuration"
	titleSoftware      = "Software"
	titleAlarms        = "Alarms"
	titleAttention     = "Needs attention"
	titleBusiest       = "Busiest devices"
	titleConnected     = "Devices connected"
)

func init() {
	for _, kind := range []ObjectKind{ObjectEstate, ObjectGroup, ObjectDevice} {
		MustRegisterPage(Page{Kind: kind, Tab: TabDashboard, Content: dashboardContent, Primary: estatePrimary})
	}

	MustRegisterFragment(fragmentDashboardCard, dashboardCardFragment)
	MustRegisterFragment(fragmentDashboardChart, dashboardChartFragment)

	registerEstateCards()
	registerGroupCards()
	registerDeviceCards()
}

// registerEstateCards registers the cards of the estate dashboard, row by
// row on its twelve columns.
func registerEstateCards() {
	for _, card := range []dashboardCard{
		{Name: cardDevices, Title: titleDevices, Span: 3, Build: dashboardDevicesCard},
		{Name: cardConfiguration, Title: titleConfiguration, Span: 3, Build: dashboardConfigCard},
		{Name: cardSoftware, Title: titleSoftware, Span: 3, Build: dashboardSoftwareCard},
		{Name: cardEstate, Title: "Estate", Span: 3, Build: dashboardEstateCard},
		{Name: cardAlarms, Title: titleAlarms, Span: 8, Build: dashboardAlarmsCard},
		{Name: cardAttention, Title: titleAttention, Span: 4, Build: dashboardAttentionCard},
		{Name: cardRollouts, Title: "Rollouts", Span: 4, Build: dashboardRolloutsCard},
		{Name: cardChanges, Title: "Recent changes", Span: 4, Build: dashboardChangesCard},
		{Name: cardBusiest, Title: titleBusiest, Span: 4, Build: dashboardBusiestCard},
		{Name: cardChartPrefix + chartDevicesConnected, Title: titleConnected, Span: 6, Build: dashboardChartCard(chartDevicesConnected)},
		{Name: cardChartPrefix + chartTraffic, Title: "Traffic, estate total", Span: 6, Build: dashboardChartCard(chartTraffic)},
	} {
		mustRegisterDashboardCard(ObjectEstate, card)
	}
}

// registerGroupCards registers the cards of a group node's dashboard: the
// estate's, scoped to the node's devices, with the Group card in place of
// the Estate card, and without Rollouts and Recent changes.
func registerGroupCards() {
	for _, card := range []dashboardCard{
		{Name: cardDevices, Title: titleDevices, Span: 3, Build: dashboardDevicesCard},
		{Name: cardConfiguration, Title: titleConfiguration, Span: 3, Build: dashboardConfigCard},
		{Name: cardSoftware, Title: titleSoftware, Span: 3, Build: dashboardSoftwareCard},
		{Name: cardGroup, Title: "Group", Span: 3, Build: dashboardGroupCard},
		{Name: cardAlarms, Title: titleAlarms, Span: 8, Build: dashboardAlarmsCard},
		{Name: cardAttention, Title: titleAttention, Span: 4, Build: dashboardAttentionCard},
		{Name: cardBusiest, Title: titleBusiest, Span: 4, Build: dashboardBusiestCard},
		{Name: cardChartPrefix + chartDevicesConnected, Title: titleConnected, Span: 6, Build: dashboardChartCard(chartDevicesConnected)},
		{Name: cardChartPrefix + chartTraffic, Title: "Traffic, group total", Span: 6, Build: dashboardChartCard(chartTraffic)},
	} {
		mustRegisterDashboardCard(ObjectGroup, card)
	}
}

// registerDeviceCards registers the cards of a device's dashboard, row by
// row on its eight columns. The live cards are left out while the device is
// offline.
func registerDeviceCards() {
	for _, card := range []dashboardCard{
		{Name: cardQuickInfo, Title: "Quick info", Span: 8, Build: dashboardQuickInfoCard},
		{Name: cardAlarms, Title: titleAlarms, Span: 5, Live: true, Build: dashboardAlarmsCard},
		{Name: cardConfiguration, Title: titleConfiguration, Span: 3, Live: true, Build: dashboardDeviceConfigCard},
		{Name: cardPeers, Title: "Peers", Span: 2, Live: true, Build: dashboardPeersCard},
		{Name: cardInterfaces, Title: "Interfaces", Span: 2, Live: true, Build: dashboardInterfacesCard},
		{Name: cardMemory, Title: "Memory", Span: 2, Live: true, Build: dashboardMemoryCard},
		{Name: cardStorage, Title: "Storage", Span: 2, Live: true, Build: dashboardStorageCard},
		{Name: cardChartPrefix + chartCPU, Title: "CPU", Span: 2, Build: dashboardChartCard(chartCPU)},
		{Name: cardChartPrefix + chartMemory, Title: "Memory use", Span: 2, Build: dashboardChartCard(chartMemory)},
		{Name: cardChartPrefix + chartTraffic, Title: "Traffic", Span: 2, Build: dashboardChartCard(chartTraffic)},
		{Name: cardChartPrefix + chartLoad, Title: "Load", Span: 2, Build: dashboardChartCard(chartLoad)},
	} {
		mustRegisterDashboardCard(ObjectDevice, card)
	}
}
