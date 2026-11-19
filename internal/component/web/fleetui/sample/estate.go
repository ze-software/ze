// Design: docs/architecture/fleet-interface.md -- the devices of the sample estate
// Overview: source.go -- the sample data source
// Related: history.go -- the samples a device's last sample is taken from
// Related: config.go -- the configuration state of each device
// Related: views.go -- the peers and interfaces the last sample counts
//
// The estate is 44 devices over four sites, and three with no site label.
// Every device is one row of deviceRows, and what makes a device differ from
// a healthy, connected, in-sync one is a row of one of the exception tables
// below: who is offline, which component has a fault, who is new. The counts
// the fleet-ui-1 spec states follow from these tables and from configStages
// in config.go, and TestSampleEstateInvariants holds them.

package sample

import (
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/component/web/fleetui"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The label keys of the sample estate. Every sample device takes its labels
// from this set, so a key is spelled once and a tree level or a filter that
// names one matches every device carrying it.
const (
	labelSite     = "site"
	labelRole     = "role"
	labelModel    = "model"
	labelCustomer = "customer"
)

// The customers of the customer-premises devices. They are invented.
const (
	customerAcme     = "acme-logistics"
	customerGlobex   = "globex-retail"
	customerInitech  = "initech-labs"
	customerUmbrella = "umbrella-foods"
	// customerNone marks a device that carries no customer label.
	customerNone = ""
)

// The names of the devices. Each is spelled once, here, so a table that names
// a device cannot name one the estate does not hold.
const (
	deviceEdgeLon01 = "edge-lon-01"
	deviceEdgeLon02 = "edge-lon-02"
	deviceEdgeLon03 = "edge-lon-03"
	deviceEdgeLon04 = "edge-lon-04"
	deviceEdgeAms01 = "edge-ams-01"
	deviceEdgeAms02 = "edge-ams-02"
	deviceEdgeAms03 = "edge-ams-03"
	deviceEdgeFra01 = "edge-fra-01"
	deviceEdgeFra02 = "edge-fra-02"
	deviceEdgeFra03 = "edge-fra-03"
	deviceEdgePar01 = "edge-par-01"
	deviceEdgePar02 = "edge-par-02"

	deviceCoreLon01 = "core-lon-01"
	deviceCoreLon02 = "core-lon-02"
	deviceCoreAms01 = "core-ams-01"
	deviceCoreAms02 = "core-ams-02"
	deviceCoreFra01 = "core-fra-01"
	deviceCoreFra02 = "core-fra-02"
	deviceCorePar01 = "core-par-01"
	deviceCorePar02 = "core-par-02"

	deviceRsLon01 = "rs-lon-01"
	deviceRsAms01 = "rs-ams-01"
	deviceRsFra01 = "rs-fra-01"
	deviceRsPar01 = "rs-par-01"

	deviceCpeLon01 = "cpe-lon-01"
	deviceCpeLon02 = "cpe-lon-02"
	deviceCpeLon03 = "cpe-lon-03"
	deviceCpeLon04 = "cpe-lon-04"
	deviceCpeLon05 = "cpe-lon-05"
	deviceCpeAms01 = "cpe-ams-01"
	deviceCpeAms02 = "cpe-ams-02"
	deviceCpeAms03 = "cpe-ams-03"
	deviceCpeAms04 = "cpe-ams-04"
	deviceCpeFra01 = "cpe-fra-01"
	deviceCpeFra02 = "cpe-fra-02"
	deviceCpeFra03 = "cpe-fra-03"
	deviceCpeFra04 = "cpe-fra-04"
	deviceCpePar03 = "cpe-par-03"
	deviceCpePar05 = "cpe-par-05"
	deviceCpePar07 = "cpe-par-07"
	deviceCpePar09 = "cpe-par-09"
	deviceCpe0113  = "cpe-0113"
	deviceCpe0114  = "cpe-0114"
	deviceCpe0127  = "cpe-0127"
)

// site is where a device stands. siteUnlabeled is a device with no site
// label, which is a real case of the estate and not an invalid value.
type site uint8

const (
	siteUnlabeled site = iota
	siteLon
	siteAms
	siteFra
	sitePar
	siteCount
)

// siteFacts is what a site gives its devices: the label value and the range
// their addresses are taken from. Every range is a documentation range.
type siteFacts struct {
	label string
	base  netip.Addr
}

var siteTable = [siteCount]siteFacts{
	siteUnlabeled: {label: "", base: netip.MustParseAddr("2001:db8:ffff::")},
	siteLon:       {label: "lon", base: netip.MustParseAddr("192.0.2.0")},
	siteAms:       {label: "ams", base: netip.MustParseAddr("198.51.100.0")},
	siteFra:       {label: "fra", base: netip.MustParseAddr("203.0.113.0")},
	sitePar:       {label: "par", base: netip.MustParseAddr("2001:db8:4::")},
}

// role is what a device does. The zero value is not a role.
type role uint8

const (
	roleEdge role = iota + 1
	roleCore
	roleRouteServer
	roleCPE
	roleCount
)

// The sizes the role table is written in.
const (
	gibibyte uint64 = 1 << 30
	mebibyte uint64 = 1 << 20
	megabit         = 1e6
	gigabit         = 1e9
)

// roleFacts is what a role gives its devices: the label value, the hardware,
// the configuration group and template, and the level its samples move
// around.
type roleFacts struct {
	label string
	model string
	// group is the configuration group, and template the template the group
	// is assigned. An empty template means the device holds its own
	// configuration.
	group        string
	template     string
	kernel       string
	localAS      uint32
	cores        int
	memoryBytes  uint64
	storageBytes uint64
	// cpuBase, memoryBase and storageBase are percentages.
	cpuBase     float64
	memoryBase  float64
	storageBase float64
	// trafficPeakBps is the traffic received at the daily peak.
	trafficPeakBps float64
	// historyBytes is what a week of samples of one device takes.
	historyBytes uint64
}

// The models, kernels, groups and templates. The models are invented.
const (
	kernelRouter = "6.12.9-ze"
	kernelEdge   = "6.12.9-ze-arm64"

	groupEdge = "edge-routers"
	groupCore = "core-routers"
	groupRS   = "route-servers"
	groupCPE  = "cpe"

	templateEdge = "edge-standard"
	templateCPE  = "cpe-standard"
	templateNone = ""

	asBackbone uint32 = 64496
	asCustomer uint32 = 64511
)

var roleTable = [roleCount]roleFacts{
	roleEdge: {
		label: "edge", model: "ZX-4800", group: groupEdge, template: templateEdge,
		kernel: kernelRouter, localAS: asBackbone, cores: 16,
		memoryBytes: 64 * gibibyte, storageBytes: 480 * gibibyte,
		cpuBase: 24, memoryBase: 46, storageBase: 31,
		trafficPeakBps: 6.4 * gigabit, historyBytes: 3 * mebibyte,
	},
	roleCore: {
		label: "core", model: "ZX-9600", group: groupCore, template: templateNone,
		kernel: kernelRouter, localAS: asBackbone, cores: 32,
		memoryBytes: 128 * gibibyte, storageBytes: 960 * gibibyte,
		cpuBase: 31, memoryBase: 58, storageBase: 27,
		trafficPeakBps: 18 * gigabit, historyBytes: 4 * mebibyte,
	},
	roleRouteServer: {
		label: "route-server", model: "ZV-200 virtual", group: groupRS, template: templateNone,
		kernel: kernelRouter, localAS: asBackbone, cores: 8,
		memoryBytes: 32 * gibibyte, storageBytes: 120 * gibibyte,
		cpuBase: 12, memoryBase: 63, storageBase: 18,
		trafficPeakBps: 40 * megabit, historyBytes: 2 * mebibyte,
	},
	roleCPE: {
		label: "customer-premises", model: "ZC-110", group: groupCPE, template: templateCPE,
		kernel: kernelEdge, localAS: asCustomer, cores: 4,
		memoryBytes: 4 * gibibyte, storageBytes: 32 * gibibyte,
		cpuBase: 9, memoryBase: 38, storageBase: 44,
		trafficPeakBps: 180 * megabit, historyBytes: mebibyte,
	},
}

// release is a software version in use. The zero value is not a release, and
// a higher value is a newer release.
type release uint8

const (
	releaseOldest release = iota + 1
	releasePrevious
	releaseNewest
	releaseCount
)

// releaseFacts is the version and the build of one release.
type releaseFacts struct {
	version string
	build   string
}

var releaseTable = [releaseCount]releaseFacts{
	releaseOldest:   {version: "1.3.9", build: "2026-07-14 4c1e9b0"},
	releasePrevious: {version: "1.4.1", build: "2026-09-30 a71d3f2"},
	releaseNewest:   {version: "1.4.2", build: "2026-11-02 9f3c1a8"},
}

// deviceRow is one device of the sample estate.
type deviceRow struct {
	name     string
	site     site
	role     role
	customer string
	release  release
	// host is the last byte of the device's addresses. It is unique in the
	// estate.
	host uint8
}

// deviceRows is the sample estate: 12 edge routers, 8 core routers, 4 route
// servers and 20 customer-premises devices.
var deviceRows = []deviceRow{
	{deviceEdgeLon01, siteLon, roleEdge, customerNone, releaseNewest, 11},
	{deviceEdgeLon02, siteLon, roleEdge, customerNone, releaseNewest, 12},
	{deviceEdgeLon03, siteLon, roleEdge, customerNone, releaseNewest, 13},
	{deviceEdgeLon04, siteLon, roleEdge, customerNone, releaseNewest, 14},
	{deviceEdgeAms01, siteAms, roleEdge, customerNone, releaseNewest, 15},
	{deviceEdgeAms02, siteAms, roleEdge, customerNone, releaseNewest, 16},
	{deviceEdgeAms03, siteAms, roleEdge, customerNone, releaseNewest, 17},
	{deviceEdgeFra01, siteFra, roleEdge, customerNone, releaseNewest, 18},
	{deviceEdgeFra02, siteFra, roleEdge, customerNone, releaseNewest, 19},
	{deviceEdgeFra03, siteFra, roleEdge, customerNone, releaseNewest, 20},
	{deviceEdgePar01, sitePar, roleEdge, customerNone, releaseNewest, 21},
	{deviceEdgePar02, sitePar, roleEdge, customerNone, releaseNewest, 22},

	{deviceCoreLon01, siteLon, roleCore, customerNone, releasePrevious, 23},
	{deviceCoreLon02, siteLon, roleCore, customerNone, releasePrevious, 24},
	{deviceCoreAms01, siteAms, roleCore, customerNone, releasePrevious, 25},
	{deviceCoreAms02, siteAms, roleCore, customerNone, releasePrevious, 26},
	{deviceCoreFra01, siteFra, roleCore, customerNone, releasePrevious, 27},
	{deviceCoreFra02, siteFra, roleCore, customerNone, releasePrevious, 28},
	{deviceCorePar01, sitePar, roleCore, customerNone, releasePrevious, 29},
	{deviceCorePar02, sitePar, roleCore, customerNone, releasePrevious, 30},

	{deviceRsLon01, siteLon, roleRouteServer, customerNone, releasePrevious, 31},
	{deviceRsAms01, siteAms, roleRouteServer, customerNone, releasePrevious, 32},
	{deviceRsFra01, siteFra, roleRouteServer, customerNone, releasePrevious, 33},
	{deviceRsPar01, sitePar, roleRouteServer, customerNone, releasePrevious, 34},

	{deviceCpeLon01, siteLon, roleCPE, customerAcme, releaseNewest, 35},
	{deviceCpeLon02, siteLon, roleCPE, customerAcme, releaseNewest, 36},
	{deviceCpeLon03, siteLon, roleCPE, customerGlobex, releaseNewest, 37},
	{deviceCpeLon04, siteLon, roleCPE, customerInitech, releaseNewest, 38},
	{deviceCpeLon05, siteLon, roleCPE, customerInitech, releaseNewest, 39},
	{deviceCpeAms01, siteAms, roleCPE, customerAcme, releaseNewest, 40},
	{deviceCpeAms02, siteAms, roleCPE, customerGlobex, releaseNewest, 41},
	{deviceCpeAms03, siteAms, roleCPE, customerGlobex, releaseNewest, 42},
	{deviceCpeAms04, siteAms, roleCPE, customerUmbrella, releaseNewest, 43},
	{deviceCpeFra01, siteFra, roleCPE, customerAcme, releaseNewest, 44},
	{deviceCpeFra02, siteFra, roleCPE, customerInitech, releaseNewest, 45},
	{deviceCpeFra03, siteFra, roleCPE, customerUmbrella, releaseNewest, 46},
	{deviceCpeFra04, siteFra, roleCPE, customerUmbrella, releaseNewest, 47},
	{deviceCpePar03, sitePar, roleCPE, customerAcme, releaseNewest, 48},
	{deviceCpePar05, sitePar, roleCPE, customerGlobex, releaseNewest, 49},
	{deviceCpePar07, sitePar, roleCPE, customerInitech, releaseNewest, 50},
	{deviceCpePar09, sitePar, roleCPE, customerUmbrella, releaseNewest, 51},
	{deviceCpe0113, siteUnlabeled, roleCPE, customerGlobex, releaseNewest, 52},
	{deviceCpe0114, siteUnlabeled, roleCPE, customerUmbrella, releaseOldest, 53},
	{deviceCpe0127, siteUnlabeled, roleCPE, customerAcme, releaseNewest, 54},
}

// offlineFor names the devices that hold no managed connection, with how long
// ago each was last seen. One has been away for six days.
var offlineFor = map[string]time.Duration{
	deviceCorePar02: 22 * time.Minute,
	deviceCpePar07:  3 * time.Hour,
	deviceCpe0127:   6*24*time.Hour + 2*time.Hour,
}

// The health components every sample device reports.
const (
	componentBGP     = "bgp"
	componentFIB     = "fib"
	componentIface   = "iface"
	componentPlugins = "plugins"
	componentSystem  = "system"
)

var healthComponents = []string{componentBGP, componentFIB, componentIface, componentPlugins, componentSystem}

// faultRow is one component of one device that does not report healthy.
type faultRow struct {
	device    string
	component string
	state     fleetui.HealthState
	detail    string
	// since is how long ago the fault began.
	since time.Duration
	// peersDown is how many of the device's BGP peers, the first of its list,
	// the fault takes down, and peerError what each of them reports.
	peersDown int
	peerError string
	// linkDown names the interface the fault takes down. It is empty when
	// the fault takes none.
	linkDown string
}

// The interfaces the tables name more than once.
const (
	ifaceLoopback = "lo"
	ifaceEth0     = "eth0"
	ifaceEth1     = "eth1"
	ifaceEth2     = "eth2"
	ifaceEth3     = "eth3"
	ifaceBond0    = "bond0"
	ifaceBridge0  = "br0"
	ifaceNone     = ""
)

// faultRows holds the faults of the estate: one device down and three
// degraded.
var faultRows = []faultRow{
	{
		device: deviceEdgeAms02, component: componentBGP, state: fleetui.HealthStateDown,
		detail:    "No BGP session is established: the BGP process stopped when it ran out of file descriptors.",
		since:     2*time.Hour + 10*time.Minute,
		peersDown: 6, peerError: "connection refused: the local BGP process is not running",
		linkDown: ifaceNone,
	},
	{
		device: deviceEdgeLon04, component: componentIface, state: fleetui.HealthStateDegraded,
		detail:    "eth1 lost its link, and the session to transit-a that it carried is down. Traffic moved to eth0.",
		since:     41 * time.Minute,
		peersDown: 1, peerError: "hold timer expired",
		linkDown: ifaceEth1,
	},
	{
		device: deviceCoreFra02, component: componentFIB, state: fleetui.HealthStateDegraded,
		detail:   "Route installation is slow: 1840 routes wait for the kernel.",
		since:    3*time.Hour + 20*time.Minute,
		linkDown: ifaceNone,
	},
	{
		device: deviceCpeAms03, component: componentPlugins, state: fleetui.HealthStateDegraded,
		detail:   "The plugin bgp-rib restarted 3 times in the last hour.",
		since:    55 * time.Minute,
		linkDown: ifaceNone,
	},
}

// The newcomer is the device that connected for the first time a few minutes
// ago. It has sent its inventory and no health report yet, it has fetched no
// configuration, and so it grants no relay profile.
const (
	newcomerName         = deviceCpe0113
	newcomerConnectedFor = 12 * time.Minute
)

// ageSpan is a period of the past, as two ages: from is the older end.
type ageSpan struct {
	from time.Duration
	to   time.Duration
}

// pastOutages names the devices that were offline for a while during the
// week and came back. Their history has a gap there.
var pastOutages = map[string]ageSpan{
	deviceEdgePar02: {from: 4*24*time.Hour + 2*time.Hour, to: 3*24*time.Hour + 21*time.Hour},
	deviceCpeAms02:  {from: 30 * time.Hour, to: 29 * time.Hour},
}

// relayUngranted names the devices whose configuration grants no profile to
// relayed commands, so every view that asks the device is refused.
var relayUngranted = map[string]struct{}{
	deviceCpeFra03: {},
	newcomerName:   {},
}

// relayProfile is the profile every other device grants.
const relayProfile = "operator"

// The shifts that keep the two addresses of a device apart, and the time a
// daemon runs before its managed connection is up.
const (
	sourceHostShift  uint8 = 100
	daemonStartDelay       = 4 * time.Minute
	uptimeBeforeLoss       = 5 * 24 * time.Hour
)

// rowIndex finds a row of deviceRows by the device's name.
var rowIndex = indexRows()

func indexRows() map[string]int {
	index := make(map[string]int, len(deviceRows))
	for at := range deviceRows {
		index[deviceRows[at].name] = at
	}

	return index
}

// findRow answers the row of the device named name, or the refusal the fleet
// package words for an unknown device.
func findRow(name string) (*deviceRow, error) {
	at, held := rowIndex[name]
	if !held {
		return nil, fleetui.DeviceUnknownError(name)
	}

	return &deviceRows[at], nil
}

// faultOf answers the fault of the device, or false when it has none.
func faultOf(name string) (*faultRow, bool) {
	for at := range faultRows {
		if faultRows[at].device == name {
			return &faultRows[at], true
		}
	}

	return nil, false
}

// hostAddr answers the address of host in the range that starts at base.
func hostAddr(base netip.Addr, host uint8) netip.Addr {
	raw := base.As16()
	raw[len(raw)-1] = host

	return netip.AddrFrom16(raw).Unmap()
}

// lastSeen answers when the device was last seen and true when it holds no
// managed connection at the snapshot: because the estate says so, or because
// the script took it offline.
func lastSeen(row *deviceRow, snap *snapshot) (time.Time, bool) {
	if away, offline := offlineFor[row.name]; offline {
		return snap.ago(away), true
	}
	if row.name != scriptDevice {
		return time.Time{}, false
	}
	if !snap.script.offline {
		return time.Time{}, false
	}

	return snap.script.offlineAt, true
}

// connectedFor answers how long before the start of the source the device's
// present connection was made. It is drawn from the device's own row, so it
// is the same at every start.
func connectedFor(row *deviceRow) time.Duration {
	if row.name == newcomerName {
		return newcomerConnectedFor
	}
	if outage, held := pastOutages[row.name]; held {
		return outage.to
	}

	days := time.Duration(2 + 3*(row.host%9))
	hours := time.Duration(row.host % 7)

	return days*24*time.Hour + hours*time.Hour
}

// connectionAt answers the connection of a device at the snapshot.
func connectionAt(row *deviceRow, snap *snapshot) fleetui.Connection {
	_, ungranted := relayUngranted[row.name]

	connection := fleetui.Connection{
		EverConnected: true,
		SourceAddr:    hostAddr(siteTable[row.site].base, row.host+sourceHostShift),
		RelayGranted:  !ungranted,
	}
	if connection.RelayGranted {
		connection.RelayProfile = relayProfile
	}

	if seen, offline := lastSeen(row, snap); offline {
		connection.LastSeen = seen
		return connection
	}

	connection.Connected = true
	connection.Since = snap.ago(connectedFor(row))
	if row.name == scriptDevice {
		if snap.script.returned {
			connection.Since = snap.script.backAt
		}
	}

	return connection
}

// healthAt answers the health report of a device: every component healthy,
// except the one its fault names.
func healthAt(row *deviceRow) []fleetui.HealthComponent {
	fault, faulty := faultOf(row.name)

	report := make([]fleetui.HealthComponent, 0, len(healthComponents))
	for _, name := range healthComponents {
		component := fleetui.HealthComponent{Name: name, State: fleetui.HealthStateHealthy}
		if faulty {
			if fault.component == name {
				component.State = fault.state
				component.Detail = fault.detail
			}
		}
		report = append(report, component)
	}

	return report
}

// serialOf answers the serial number of a device. It is plainly invented.
func serialOf(row *deviceRow) string {
	return textbuf.New().Str("SAMPLE-").Uint(uint64(row.role)).Byte('-').Uint(10000 + uint64(row.host)).Slice()
}

// inventoryAt answers the inventory of a device at the snapshot.
func inventoryAt(row *deviceRow, snap *snapshot) fleetui.Inventory {
	facts := &roleTable[row.role]
	version := &releaseTable[row.release]

	started := snap.ago(connectedFor(row) + daemonStartDelay)
	if away, offline := offlineFor[row.name]; offline {
		started = snap.ago(away + uptimeBeforeLoss)
	}

	return fleetui.Inventory{
		Reported:        true,
		Model:           facts.model,
		Serial:          serialOf(row),
		SoftwareVersion: version.version,
		Build:           version.build,
		Kernel:          facts.kernel,
		ManagementAddr:  hostAddr(siteTable[row.site].base, row.host),
		StartedAt:       started,
	}
}

// labelsOf answers the labels of a device. A device with no site, or with no
// customer, does not carry that label.
func labelsOf(row *deviceRow) []fleetui.Label {
	facts := &roleTable[row.role]

	labels := make([]fleetui.Label, 0, 4)
	if row.site != siteUnlabeled {
		labels = append(labels, fleetui.Label{Key: labelSite, Value: siteTable[row.site].label})
	}
	labels = append(labels,
		fleetui.Label{Key: labelRole, Value: facts.label},
		fleetui.Label{Key: labelModel, Value: facts.model},
	)
	if row.customer != customerNone {
		labels = append(labels, fleetui.Label{Key: labelCustomer, Value: row.customer})
	}

	return labels
}

// deviceAt answers the facts of one device at the snapshot.
func deviceAt(row *deviceRow, snap *snapshot) fleetui.Device {
	device := fleetui.Device{
		Name:       row.name,
		Labels:     labelsOf(row),
		Group:      roleTable[row.role].group,
		SecretSet:  true,
		Connection: connectionAt(row, snap),
		Config:     configFactsAt(row, snap),
		Inventory:  inventoryAt(row, snap),
		Latest:     latestAt(row, snap),
	}

	if row.name != newcomerName {
		device.HealthReported = true
		device.Health = healthAt(row)
	}

	return device
}

// devicesAt answers every device of the estate at the snapshot, in the order
// of deviceRows.
func devicesAt(snap *snapshot) []fleetui.Device {
	devices := make([]fleetui.Device, 0, len(deviceRows))
	for at := range deviceRows {
		devices = append(devices, deviceAt(&deviceRows[at], snap))
	}

	return devices
}

// declaredTreeViews answers the tree views the sample hub declares beside the
// built-in ones.
func declaredTreeViews() []fleetui.TreeView {
	return []fleetui.TreeView{
		{Name: "customer", Title: "By customer", Levels: []string{labelCustomer}},
		{Name: "site-model", Title: "By site, then model", Levels: []string{labelSite, labelModel}},
	}
}

// savedFilters answers the saved filters of the sample hub.
func savedFilters() []fleetui.SavedFilter {
	return []fleetui.SavedFilter{
		{Name: "Edge routers", Filter: "role:edge"},
		{Name: "Not healthy", Filter: "state:down,degraded,offline,diverged"},
		{Name: "Paris", Filter: "site:par"},
	}
}
