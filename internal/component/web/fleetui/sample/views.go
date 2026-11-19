// Design: docs/architecture/fleet-interface.md -- what a sample device answers through the relay
// Overview: source.go -- the sample data source
// Related: estate.go -- the rows, the roles and the faults the views read
// Related: history.go -- the last sample, which counts these peers and interfaces
//
// What a device shows is drawn from its role: the interfaces, the BGP peers,
// the other adjacencies, the hardware and the log are one table per role. A
// device's fault changes the rows it names, so the health report, the alarm,
// the peers table and the interfaces table of a device tell the same story.
//
// A view asks the device, so it can fail on its own: an offline device and a
// device that grants no relay profile refuse every view, and the device on
// the oldest release does not answer the BGP summary.

package sample

import (
	"net/netip"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/component/web/fleetui"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// relayRefusal is what a device that grants no relay profile answers, word
// for word. The configuration line it names is illustrative: the leaf that
// grants the relay is not designed yet, and `set fleet relay profile <name>`
// stands for "the line that grants the relay" until fleet-9 fixes it.
const relayRefusal = "relayed commands are refused: this device grants them no profile. " +
	"The line that grants the relay is: set fleet relay profile operator"

// The device tabs a refusal sends the operator to.
const (
	tabTasks  = "tasks"
	tabConfig = "config"
	tabSystem = "system"
)

// devicePathPrefix starts the address of a device page.
const devicePathPrefix = "/fleet/device/"

// devicePage answers the address of one tab of a device.
func devicePage(name, tab string) string {
	return textbuf.New().Str("open ").Str(devicePathPrefix).Str(name).Byte('/').Str(tab).Byte('/').Slice()
}

// reach answers the row of a device the relay can ask at the snapshot. It
// refuses an unknown device, an offline one, and one that grants no relay
// profile.
func reach(snap *snapshot, name string) (*deviceRow, error) {
	row, err := findRow(name)
	if err != nil {
		return nil, err
	}

	if seen, offline := lastSeen(row, snap); offline {
		problem := textbuf.New().Str(row.name).Str(" has been offline since ").
			Str(seen.UTC().Format(time.DateTime)).Str(" UTC.").Slice()

		return nil, opererr.Wrap(fleetui.ErrDeviceOffline, problem,
			"The device holds no managed connection, so the hub cannot ask it for its live state.",
			devicePage(row.name, tabTasks))
	}

	if _, ungranted := relayUngranted[row.name]; ungranted {
		problem := textbuf.New().Str(row.name).Str(" refused the command: ").Str(relayRefusal).Str(".").Slice()

		return nil, opererr.Wrap(fleetui.ErrRelayRefused, problem,
			"The device decides which profile a relayed command runs under, and its configuration names none.",
			// The fix is the line that grants a profile, which the failure
			// state shows to copy; operator is the sample's illustrative
			// profile, and the device's Config tab is where it is set.
			"set fleet relay profile operator")
	}

	return row, nil
}

// viewUnsupported is the refusal of a view the device's release does not
// answer.
func viewUnsupported(row *deviceRow) error {
	problem := textbuf.New().Str(row.name).Str(" runs version ").Str(releaseTable[row.release].version).
		Str(", which does not answer this view.").Slice()

	return opererr.Wrap(fleetui.ErrViewUnsupported, problem,
		"The command behind this view was added in a later release, so the device has nothing to answer.",
		devicePage(row.name, tabSystem))
}

// faultAt answers the fault of the device when it had begun at offset from
// the start, or false.
func faultAt(row *deviceRow, offset time.Duration) (*faultRow, bool) {
	fault, faulty := faultOf(row.name)
	if !faulty {
		return nil, false
	}
	if offset <= -fault.since {
		return nil, false
	}

	return fault, true
}

// The words a sample device uses for a session, an adjacency and a log level.
const (
	stateEstablished = "established"
	stateActive      = "active"
	stateFull        = "full"
	stateUp          = "up"
	stateDown        = "down"

	levelInfo  = "info"
	levelWarn  = "warn"
	levelError = "error"
)

// The peer groups and address families of the role tables.
const (
	peerGroupTransit  = "transit"
	peerGroupIX       = "ix"
	peerGroupCore     = "core"
	peerGroupEdge     = "edge"
	peerGroupMembers  = "members"
	peerGroupUpstream = "upstream"

	familyIPv4 = "ipv4/unicast"
	familyIPv6 = "ipv6/unicast"
)

// peerRow is one BGP peer of a role. Every address is in a documentation
// range and every AS number is a documentation or private one.
type peerRow struct {
	name     string
	group    string
	address  netip.Addr
	remoteAS uint32
	received uint64
	sent     uint64
}

func peerAddr(text string) netip.Addr {
	return netip.MustParseAddr(text)
}

var rolePeers = [roleCount][]peerRow{
	roleEdge: {
		{"transit-a", peerGroupTransit, peerAddr("198.51.100.1"), 64500, 942113, 412},
		{"transit-b", peerGroupTransit, peerAddr("203.0.113.1"), 64501, 938870, 412},
		{"ix-rs1", peerGroupIX, peerAddr("2001:db8:1::1"), 64510, 184220, 412},
		{"ix-rs2", peerGroupIX, peerAddr("2001:db8:1::2"), 64510, 184101, 412},
		{"core-a", peerGroupCore, peerAddr("192.0.2.23"), asBackbone, 310, 951204},
		{"core-b", peerGroupCore, peerAddr("192.0.2.24"), asBackbone, 310, 951204},
	},
	roleCore: {
		{"edge-a", peerGroupEdge, peerAddr("192.0.2.11"), asBackbone, 951204, 310},
		{"edge-b", peerGroupEdge, peerAddr("192.0.2.12"), asBackbone, 951198, 310},
		{"edge-c", peerGroupEdge, peerAddr("192.0.2.13"), asBackbone, 951201, 310},
		{"core-peer", peerGroupCore, peerAddr("192.0.2.29"), asBackbone, 1241, 1241},
		{"rs-a", peerGroupCore, peerAddr("192.0.2.31"), asBackbone, 184220, 310},
		{"rs-b", peerGroupCore, peerAddr("192.0.2.32"), asBackbone, 184101, 310},
	},
	roleRouteServer: {
		{"member-01", peerGroupMembers, peerAddr("2001:db8:2::1"), 64502, 41020, 143200},
		{"member-02", peerGroupMembers, peerAddr("2001:db8:2::2"), 64503, 38114, 146106},
		{"member-03", peerGroupMembers, peerAddr("2001:db8:2::3"), 64504, 22907, 161313},
		{"member-04", peerGroupMembers, peerAddr("2001:db8:2::4"), 64505, 19340, 164880},
		{"member-05", peerGroupMembers, peerAddr("2001:db8:2::5"), 64506, 17551, 166669},
		{"member-06", peerGroupMembers, peerAddr("2001:db8:2::6"), 64507, 12006, 172214},
		{"member-07", peerGroupMembers, peerAddr("2001:db8:2::7"), 64508, 9871, 174349},
		{"member-08", peerGroupMembers, peerAddr("2001:db8:2::8"), 64509, 4410, 179810},
	},
	roleCPE: {
		{"provider", peerGroupUpstream, peerAddr("198.51.100.254"), asBackbone, 2, 1},
	},
}

// peerCounts is how many peers of a device are established and how many are
// not.
type peerCounts struct {
	established int
	down        int
}

// peerCountsAt answers the peer counts of a device at offset from the start.
func peerCountsAt(row *deviceRow, offset time.Duration) peerCounts {
	counts := peerCounts{established: len(rolePeers[row.role])}

	if fault, faulty := faultAt(row, offset); faulty {
		counts.down = min(fault.peersDown, counts.established)
		counts.established -= counts.down
	}

	return counts
}

// peersEstablishedAt answers how many peers of a device were established at
// offset from the start.
func peersEstablishedAt(row *deviceRow, offset time.Duration) int {
	return peerCountsAt(row, offset).established
}

// The time a session and an adjacency take to come up after the managed
// connection.
const (
	sessionDelay   = 90 * time.Second
	adjacencyDelay = 40 * time.Second
)

// peersAt answers the BGP peers of a device at the snapshot. The first peers
// of the role's list are the ones a fault takes down.
func peersAt(row *deviceRow, snap *snapshot) []fleetui.Peer {
	rows := rolePeers[row.role]
	up := connectionAt(row, snap).Since.Add(sessionDelay)

	fault, faulty := faultAt(row, snap.now.Sub(snap.start))

	peers := make([]fleetui.Peer, 0, len(rows))
	for at := range rows {
		peer := fleetui.Peer{
			Name:             rows[at].name,
			Address:          rows[at].address,
			Group:            rows[at].group,
			RemoteAS:         rows[at].remoteAS,
			State:            stateEstablished,
			Established:      true,
			Since:            up,
			PrefixesReceived: rows[at].received,
			PrefixesSent:     rows[at].sent,
		}
		if faulty {
			if at < fault.peersDown {
				peer.State = stateActive
				peer.Established = false
				peer.Since = snap.ago(fault.since)
				peer.PrefixesReceived = 0
				peer.PrefixesSent = 0
				peer.LastError = fault.peerError
			}
		}
		peers = append(peers, peer)
	}

	return peers
}

// familyRow is the BGP routes of one address family of a role.
type familyRow struct {
	family string
	routes uint64
	best   uint64
}

var roleFamilies = [roleCount][]familyRow{
	roleEdge:        {{familyIPv4, 1881603, 951204}, {familyIPv6, 421330, 212870}},
	roleCore:        {{familyIPv4, 2854120, 951204}, {familyIPv6, 638442, 212870}},
	roleRouteServer: {{familyIPv4, 121880, 98114}, {familyIPv6, 43339, 36106}},
	roleCPE:         {{familyIPv4, 2, 2}},
}

// familiesAt answers the BGP summary of a device. A device with no
// established peer holds no route.
func familiesAt(row *deviceRow) []fleetui.FamilySummary {
	rows := roleFamilies[row.role]
	established := peersEstablishedAt(row, 0)

	families := make([]fleetui.FamilySummary, 0, len(rows))
	for at := range rows {
		summary := fleetui.FamilySummary{Family: rows[at].family, PeersEstablished: established}
		if established > 0 {
			summary.Routes = rows[at].routes
			summary.BestRoutes = rows[at].best
		}
		families = append(families, summary)
	}

	return families
}

// interfaceRow is one interface of a role.
type interfaceRow struct {
	name     string
	kind     string
	speedBps uint64
	mtu      int
	// share is the part of the device's traffic the interface carries.
	share float64
	// index is the interface's byte in its address.
	index     uint8
	adminDown bool
}

// The interface kinds and speeds of the role tables.
const (
	kindLoopback = "loopback"
	kindEthernet = "ethernet"
	kindBond     = "bond"
	kindBridge   = "bridge"

	speed1G  uint64 = 1e9
	speed10G uint64 = 1e10
	speed25G uint64 = 25e9
	speed50G uint64 = 5e10

	mtuLoopback = 65536
	mtuEthernet = 1500
	mtuJumbo    = 9000
)

var roleInterfaces = [roleCount][]interfaceRow{
	roleEdge: {
		{name: ifaceLoopback, kind: kindLoopback, mtu: mtuLoopback},
		{name: ifaceEth0, kind: kindEthernet, speedBps: speed10G, mtu: mtuJumbo, share: 0.55, index: 1},
		{name: ifaceEth1, kind: kindEthernet, speedBps: speed10G, mtu: mtuJumbo, share: 0.35, index: 2},
		{name: ifaceEth2, kind: kindEthernet, speedBps: speed1G, mtu: mtuEthernet, share: 0.10, index: 3},
		{name: ifaceEth3, kind: kindEthernet, speedBps: speed1G, mtu: mtuEthernet, index: 4, adminDown: true},
	},
	roleCore: {
		{name: ifaceLoopback, kind: kindLoopback, mtu: mtuLoopback},
		{name: ifaceBond0, kind: kindBond, speedBps: speed50G, mtu: mtuJumbo, share: 0.60, index: 1},
		{name: ifaceEth0, kind: kindEthernet, speedBps: speed25G, mtu: mtuJumbo, share: 0.20, index: 2},
		{name: ifaceEth1, kind: kindEthernet, speedBps: speed25G, mtu: mtuJumbo, share: 0.20, index: 3},
	},
	roleRouteServer: {
		{name: ifaceLoopback, kind: kindLoopback, mtu: mtuLoopback},
		{name: ifaceEth0, kind: kindEthernet, speedBps: speed10G, mtu: mtuEthernet, share: 1, index: 1},
	},
	roleCPE: {
		{name: ifaceLoopback, kind: kindLoopback, mtu: mtuLoopback},
		{name: ifaceEth0, kind: kindEthernet, speedBps: speed1G, mtu: mtuEthernet, share: 1, index: 1},
		{name: ifaceBridge0, kind: kindBridge, speedBps: speed1G, mtu: mtuEthernet, share: 0.9, index: 2},
	},
}

// linkCounts is how many interfaces of a device are up, down and
// administratively down.
type linkCounts struct {
	up        int
	down      int
	adminDown int
}

// linkUp reports whether an interface that is administratively up has its
// link, given the fault of its device at that time.
func linkUp(iface *interfaceRow, fault *faultRow, faulty bool) bool {
	if !faulty {
		return true
	}

	return fault.linkDown != iface.name
}

// interfaceCounts answers the interface counts of a device at offset from the
// start.
func interfaceCounts(row *deviceRow, offset time.Duration) linkCounts {
	fault, faulty := faultAt(row, offset)

	var counts linkCounts
	rows := roleInterfaces[row.role]
	for at := range rows {
		switch {
		case rows[at].adminDown:
			counts.adminDown++
		case linkUp(&rows[at], fault, faulty):
			counts.up++
		default:
			counts.down++
		}
	}

	return counts
}

// loopbackBase is the range the loopback addresses are taken from.
var loopbackBase = netip.MustParseAddr("192.0.2.0")

// The lengths of the prefixes of a loopback and of a link, and the errors a
// link that lost its carrier has counted.
const (
	loopbackBits   = 32
	linkBits       = 64
	lostLinkErrors = 18422
)

// interfaceAddresses answers the addresses of one interface of a device. An
// interface that is administratively down holds none.
func interfaceAddresses(row *deviceRow, iface *interfaceRow) []netip.Prefix {
	if iface.adminDown {
		return []netip.Prefix{}
	}
	if iface.kind == kindLoopback {
		return []netip.Prefix{netip.PrefixFrom(hostAddr(loopbackBase, row.host), loopbackBits)}
	}

	link := netip.AddrFrom16([16]byte{0x20, 0x01, 0x0d, 0xb8, 0, uint8(row.site), 0, iface.index, 15: row.host})

	return []netip.Prefix{netip.PrefixFrom(link, linkBits)}
}

// interfacesAt answers the interfaces of a device at the snapshot. Their
// rates are shares of the device's last sample.
func interfacesAt(row *deviceRow, snap *snapshot) []fleetui.Interface {
	latest := latestAt(row, snap)
	fault, faulty := faultAt(row, snap.now.Sub(snap.start))

	rows := roleInterfaces[row.role]
	interfaces := make([]fleetui.Interface, 0, len(rows))
	for at := range rows {
		iface := &rows[at]
		wear := 1 + bias(row.name, iface.name)

		answer := fleetui.Interface{
			Name:      iface.name,
			Kind:      iface.kind,
			AdminUp:   !iface.adminDown,
			Addresses: interfaceAddresses(row, iface),
			SpeedBps:  iface.speedBps,
			MTU:       iface.mtu,
		}
		if answer.AdminUp {
			answer.LinkUp = linkUp(iface, fault, faulty)
		}
		if answer.LinkUp {
			answer.InBps = uint64(float64(latest.TrafficInBps) * iface.share)
			answer.OutBps = uint64(float64(latest.TrafficOutBps) * iface.share)
			answer.Errors = uint64(3 * wear)
			answer.Drops = uint64(40 * wear * iface.share)
		} else if answer.AdminUp {
			answer.Errors = lostLinkErrors
		}

		interfaces = append(interfaces, answer)
	}

	return interfaces
}

// adjacencyRow is one adjacency of a role, and adjacencyGroup the adjacencies
// of one protocol.
type adjacencyRow struct {
	neighbor string
	iface    string
	state    string
}

type adjacencyGroup struct {
	protocol string
	rows     []adjacencyRow
}

// The other routing protocols a sample device runs.
const (
	protocolOSPF = "ospf"
	protocolBFD  = "bfd"
)

// roleAdjacencies holds the adjacencies of the routing protocols a role runs
// beside BGP. A route server and a customer-premises device run none.
var roleAdjacencies = [roleCount][]adjacencyGroup{
	roleEdge: {
		{protocol: protocolOSPF, rows: []adjacencyRow{
			{"192.0.2.23", ifaceEth0, stateFull},
			{"192.0.2.24", ifaceEth1, stateFull},
		}},
	},
	roleCore: {
		{protocol: protocolOSPF, rows: []adjacencyRow{
			{"192.0.2.11", ifaceBond0, stateFull},
			{"192.0.2.12", ifaceEth0, stateFull},
			{"192.0.2.29", ifaceEth1, stateFull},
		}},
		{protocol: protocolBFD, rows: []adjacencyRow{
			{"192.0.2.13", ifaceBond0, stateUp},
			{"192.0.2.30", ifaceEth1, stateUp},
		}},
	},
}

// adjacenciesAt answers the adjacencies of a device at the snapshot. An
// adjacency over an interface a fault took down is down.
func adjacenciesAt(row *deviceRow, snap *snapshot) []fleetui.ProtocolAdjacencies {
	up := connectionAt(row, snap).Since.Add(adjacencyDelay)
	fault, faulty := faultAt(row, snap.now.Sub(snap.start))

	groups := roleAdjacencies[row.role]
	protocols := make([]fleetui.ProtocolAdjacencies, 0, len(groups))
	for at := range groups {
		rows := make([]fleetui.Adjacency, 0, len(groups[at].rows))
		for _, adjacency := range groups[at].rows {
			answer := fleetui.Adjacency{
				Neighbor:  adjacency.neighbor,
				Interface: adjacency.iface,
				State:     adjacency.state,
				Since:     up,
			}
			if faulty {
				if fault.linkDown == adjacency.iface {
					answer.State = stateDown
					answer.Since = snap.ago(fault.since)
				}
			}
			rows = append(rows, answer)
		}
		protocols = append(protocols, fleetui.ProtocolAdjacencies{Protocol: groups[at].protocol, Rows: rows})
	}

	return protocols
}

// The kinds of hardware a sample device holds.
const (
	hardwareProcessor   = "processor"
	hardwareMemory      = "memory"
	hardwareNetwork     = "network interface"
	hardwareStorage     = "storage"
	hardwareTemperature = "temperature"
)

// roleHardware holds the hardware of a role. It is invented.
var roleHardware = [roleCount][]fleetui.HardwareItem{
	roleEdge: {
		{Kind: hardwareProcessor, Description: "16 cores, 2.4 GHz (sample)"},
		{Kind: hardwareMemory, Description: "64 GiB ECC", Detail: "no error reported"},
		{Kind: hardwareNetwork, Description: "2 x 10 Gbit/s, 2 x 1 Gbit/s"},
		{Kind: hardwareStorage, Description: "480 GB solid state", Detail: "healthy, 3% worn"},
		{Kind: hardwareTemperature, Description: "Inlet", Detail: "31 C"},
	},
	roleCore: {
		{Kind: hardwareProcessor, Description: "32 cores, 2.9 GHz (sample)"},
		{Kind: hardwareMemory, Description: "128 GiB ECC", Detail: "no error reported"},
		{Kind: hardwareNetwork, Description: "2 x 25 Gbit/s bonded, 2 x 25 Gbit/s"},
		{Kind: hardwareStorage, Description: "2 x 480 GB solid state, mirrored", Detail: "healthy, 5% worn"},
		{Kind: hardwareTemperature, Description: "Inlet", Detail: "29 C"},
	},
	roleRouteServer: {
		{Kind: hardwareProcessor, Description: "8 virtual cores"},
		{Kind: hardwareMemory, Description: "32 GiB"},
		{Kind: hardwareNetwork, Description: "1 x 10 Gbit/s virtual"},
		{Kind: hardwareStorage, Description: "120 GB virtual disk", Detail: "healthy"},
	},
	roleCPE: {
		{Kind: hardwareProcessor, Description: "4 cores, 1.6 GHz (sample)"},
		{Kind: hardwareMemory, Description: "4 GiB"},
		{Kind: hardwareNetwork, Description: "2 x 1 Gbit/s"},
		{Kind: hardwareStorage, Description: "32 GB flash", Detail: "healthy, 11% worn"},
		{Kind: hardwareTemperature, Description: "Board", Detail: "44 C"},
	},
}

// systemAt answers the hardware, the resources and the clock of a device at
// the snapshot. The resources are the device's last sample.
func systemAt(row *deviceRow, snap *snapshot) fleetui.DeviceSystem {
	latest := latestAt(row, snap)

	return fleetui.DeviceSystem{
		Time:     snap.now,
		Hardware: slices.Clone(roleHardware[row.role]),
		Resources: fleetui.Resources{
			CPUPercent:        latest.CPUPercent,
			Load:              latest.Load,
			MemoryUsedBytes:   latest.MemoryUsedBytes,
			MemoryTotalBytes:  latest.MemoryTotalBytes,
			StorageUsedBytes:  latest.StorageUsedBytes,
			StorageTotalBytes: latest.StorageTotalBytes,
		},
	}
}

// logRow is one log line. device is empty for a line every device logs.
type logRow struct {
	device  string
	age     time.Duration
	level   string
	source  string
	message string
}

// The parts of a sample device that write log lines.
const (
	logEveryDevice = ""
	logConfig      = "config"
	logManaged     = "managed"
	logPlugin      = "plugin"
)

// logRows is the log of the sample devices, the newest first within each
// device. A device's own lines are the ones its fault wrote.
var logRows = []logRow{
	{logEveryDevice, 3 * time.Minute, levelInfo, logManaged, "health report sent to the hub"},
	{logEveryDevice, 18 * time.Minute, levelInfo, componentSystem, "time synchronized, offset 0.4 ms"},
	{logEveryDevice, 49 * time.Minute, levelInfo, componentBGP, "route refresh answered for ipv4/unicast"},
	{logEveryDevice, 2 * time.Hour, levelInfo, logManaged, "sample sent to the hub"},
	{logEveryDevice, 5 * time.Hour, levelWarn, componentSystem, "log rotation took 2.1 s, longer than 1 s"},
	{logEveryDevice, 9 * time.Hour, levelInfo, logConfig, "configuration verified, nothing to apply"},
	{logEveryDevice, 26 * time.Hour, levelInfo, componentBGP, "graceful restart timers refreshed"},
	{logEveryDevice, 50 * time.Hour, levelInfo, logConfig, "configuration applied from the hub"},

	{deviceEdgeAms02, 2*time.Hour + 9*time.Minute, levelError, componentBGP, "no peer is established"},
	{deviceEdgeAms02, 2*time.Hour + 10*time.Minute, levelError, logPlugin, "bgp process exited: too many open files"},
	{deviceEdgeAms02, 2*time.Hour + 12*time.Minute, levelWarn, componentSystem, "open file descriptors at 98% of the limit"},

	{deviceEdgeLon04, 40 * time.Minute, levelError, componentBGP, "peer transit-a: hold timer expired"},
	{deviceEdgeLon04, 41 * time.Minute, levelWarn, componentIface, "eth1: link down"},
	{deviceEdgeLon04, 44 * time.Minute, levelWarn, componentIface, "eth1: 18422 receive errors in the last minute"},

	{deviceCoreFra02, 30 * time.Minute, levelWarn, componentSystem, "route installation queue holds 1840 routes"},
	{deviceCoreFra02, 3*time.Hour + 20*time.Minute, levelWarn, componentSystem, "kernel route installation slower than 500 routes per second"},

	{deviceCpeAms03, 12 * time.Minute, levelWarn, logPlugin, "bgp-rib restarted, third time this hour"},
	{deviceCpeAms03, 31 * time.Minute, levelError, logPlugin, "bgp-rib exited with status 2"},
	{deviceCpeAms03, 55 * time.Minute, levelError, logPlugin, "bgp-rib exited with status 2"},
}

// logsAt answers the log lines of a device at the snapshot, the newest first.
func logsAt(row *deviceRow, snap *snapshot) []fleetui.LogLine {
	lines := make([]fleetui.LogLine, 0, len(logRows))
	for at := range logRows {
		line := &logRows[at]
		if line.device != logEveryDevice {
			if line.device != row.name {
				continue
			}
		}

		lines = append(lines, fleetui.LogLine{
			At: snap.ago(line.age), Level: line.level, Source: line.source, Message: line.message,
		})
	}

	slices.SortStableFunc(lines, func(a, b fleetui.LogLine) int { return b.At.Compare(a.At) })

	return lines
}
