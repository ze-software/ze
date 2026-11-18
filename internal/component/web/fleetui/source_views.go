// Design: docs/architecture/fleet-interface.md -- the questions only a device can answer
// Overview: source.go -- the data source seam
// Related: source_errors.go -- the failures a device view reports
//
// Every question of this file is answered by the device itself, through the
// command relay. So each can fail on its own: the device is offline, it
// grants no relay profile, its software does not know the command, or it does
// not answer in time. A source reports each with the matching error of
// source_errors.go, and a page shows the matching state for that section and
// keeps its other sections.

package fleetui

import (
	"context"
	"net/netip"
	"time"
)

// The bounds of the device view answers.
const (
	// HardwareItemsMax bounds the hardware items of one device.
	HardwareItemsMax = 64
	// InterfacesMax bounds the interfaces of one device.
	InterfacesMax = 1024
	// PeersMax bounds the BGP peers of one device.
	PeersMax = 4096
	// FamiliesMax bounds the address families of one device's BGP summary.
	FamiliesMax = 64
	// ProtocolsMax bounds the routing protocols of one device, BGP apart.
	ProtocolsMax = 16
	// AdjacenciesMax bounds the adjacencies of one protocol.
	AdjacenciesMax = 1024
	// LogLinesMax bounds the log lines one answer holds.
	LogLinesMax = 500
	// ConsoleOutputBytesMax bounds the output of one console command. A
	// longer output is cut and marked.
	ConsoleOutputBytesMax = 65536
)

// DeviceViewSource answers with live state only the device holds.
type DeviceViewSource interface {
	// DeviceSystem answers the device's hardware, its resources now and its
	// clock.
	// Asked by: the System tab. Answered by: fleet-4 (hardware) and fleet-9
	// (resources and clock, through the relay).
	DeviceSystem(ctx context.Context, device string) (DeviceSystem, error)

	// DeviceInterfaces answers the device's interfaces with their live state
	// and counters, at most InterfacesMax.
	// Asked by: the Interfaces tab. Answered by: fleet-9.
	DeviceInterfaces(ctx context.Context, device string) ([]Interface, error)

	// DevicePeers answers the device's BGP peers, at most PeersMax.
	// Asked by: the BGP peers sub-tab of the Routing tab.
	// Answered by: fleet-9.
	DevicePeers(ctx context.Context, device string) ([]Peer, error)

	// DeviceFamilies answers the device's BGP summary per address family, at
	// most FamiliesMax.
	// Asked by: the BGP summary sub-tab of the Routing tab.
	// Answered by: fleet-9.
	DeviceFamilies(ctx context.Context, device string) ([]FamilySummary, error)

	// DeviceAdjacencies answers, for each other routing protocol the device
	// runs, its adjacencies: at most ProtocolsMax protocols of at most
	// AdjacenciesMax rows. A protocol the device does not run is not in the
	// answer, so it has no sub-tab.
	// Asked by: the other sub-tabs of the Routing tab. Answered by: fleet-9.
	DeviceAdjacencies(ctx context.Context, device string) ([]ProtocolAdjacencies, error)

	// DeviceLogs answers the device's recent log lines, at most LogLinesMax,
	// newest first.
	// Asked by: the Logs tab. Answered by: fleet-9.
	DeviceLogs(ctx context.Context, device string) ([]LogLine, error)

	// Console runs one command typed by user on the device and answers its
	// output, at most ConsoleOutputBytesMax.
	// Asked by: the Console tab. Answered by: fleet-9.
	Console(ctx context.Context, device, user, command string) (ConsoleAnswer, error)
}

// DeviceSystem is what the System tab shows beyond the registry's facts.
type DeviceSystem struct {
	// Time is the device's clock when it answered.
	Time time.Time
	// Hardware holds the device's hardware, at most HardwareItemsMax.
	Hardware  []HardwareItem
	Resources Resources
}

// HardwareItem is one piece of a device's hardware.
type HardwareItem struct {
	// Kind says what the item is: processor, memory, network interface,
	// storage or temperature.
	Kind        string
	Description string
	// Detail is a health summary or a reading. It is empty when the item
	// has none.
	Detail string
}

// Resources is a device's resource use when it answered.
type Resources struct {
	CPUPercent        float64
	Load              float64
	MemoryUsedBytes   uint64
	MemoryTotalBytes  uint64
	StorageUsedBytes  uint64
	StorageTotalBytes uint64
}

// Interface is one interface of a device, with its live state and counters.
type Interface struct {
	Name string
	// Kind is the interface type, in the device's words.
	Kind      string
	AdminUp   bool
	LinkUp    bool
	Addresses []netip.Prefix
	SpeedBps  uint64
	MTU       int
	InBps     uint64
	OutBps    uint64
	Errors    uint64
	Drops     uint64
}

// Peer is one BGP peer of a device.
type Peer struct {
	Name     string
	Address  netip.Addr
	Group    string
	RemoteAS uint32
	// State is the session state, in the device's words.
	State       string
	Established bool
	// Since is when the peer entered State.
	Since            time.Time
	PrefixesReceived uint64
	PrefixesSent     uint64
	// LastError is the last error of the session. It is empty when the
	// device holds none.
	LastError string
}

// FamilySummary is a device's BGP state for one address family.
type FamilySummary struct {
	Family           string
	PeersEstablished int
	Routes           uint64
	BestRoutes       uint64
}

// ProtocolAdjacencies is the adjacencies of one routing protocol of a device.
type ProtocolAdjacencies struct {
	// Protocol names the protocol, in the device's words.
	Protocol string
	Rows     []Adjacency
}

// Adjacency is one adjacency or neighbor of a routing protocol.
type Adjacency struct {
	Neighbor  string
	Interface string
	// State is the adjacency state, in the device's words.
	State string
	Since time.Time
}

// LogLine is one line of a device's log.
type LogLine struct {
	At time.Time
	// Level is the level, in the device's words.
	Level   string
	Source  string
	Message string
}

// ConsoleAnswer is the output of one console command.
type ConsoleAnswer struct {
	Output string
	// Truncated reports whether the output was cut to fit.
	Truncated bool
}
