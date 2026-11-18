// Design: docs/architecture/fleet-interface.md -- the registry questions and the device facts
// Overview: source.go -- the data source seam
// Related: state.go -- the display state derived from these facts
// Related: derive.go -- the device view the pages work on

package fleetui

import (
	"context"
	"net/netip"
	"time"
)

// The bounds of the registry answers.
const (
	// DevicesMax bounds the devices of one estate.
	DevicesMax = 4096
	// LabelsMax bounds the labels of one device.
	LabelsMax = 32
	// HealthComponentsMax bounds the health components of one device.
	HealthComponentsMax = 32
	// TreeViewsMax bounds the tree views a hub declares.
	TreeViewsMax = 32
	// TreeLevelsMax bounds the levels of one tree view, and so the depth of
	// the tree and of a group address.
	TreeLevelsMax = 8
	// SavedFiltersMax bounds the saved filters of one user.
	SavedFiltersMax = 64
)

// RegistrySource answers from the hub's device registry. Every page asks it.
type RegistrySource interface {
	// Now answers the hub's clock. Every age and every chart range on a page
	// is measured against it, so no page reads the wall clock.
	// Asked by: every page. Answered by: fleet-1.
	Now(ctx context.Context) (time.Time, error)

	// Estate answers what names the estate.
	// Asked by: the shell header and the estate pages. Answered by: fleet-1.
	Estate(ctx context.Context) (Estate, error)

	// Devices answers every device of the estate with its facts, at most
	// DevicesMax.
	// Asked by: the tree, the devices tables, the estate and group
	// dashboards, the header search, the labels administration page.
	// Answered by: fleet-1 (registry, labels, connection, configuration
	// state), fleet-4 (inventory, health), fleet-8 (latest sample).
	Devices(ctx context.Context) ([]Device, error)

	// Device answers one device by name. An unknown name is
	// DeviceUnknownError.
	// Asked by: every device tab and the device side panel.
	// Answered by: fleet-1, fleet-4, fleet-8.
	Device(ctx context.Context, name string) (Device, error)

	// TreeViews answers the tree views the hub's configuration declares, at
	// most TreeViewsMax. The built-in views are not among them: see
	// BuiltinTreeViews.
	// Asked by: the tree view menu and the tree views administration page.
	// Answered by: fleet-ui-2 (hub configuration).
	TreeViews(ctx context.Context) ([]TreeView, error)

	// SavedFilters answers the filters user saved, at most SavedFiltersMax.
	// Asked by: the saved filter menu of every table and the saved filters
	// administration page. Answered by: fleet-ui-2 (per-user state).
	SavedFilters(ctx context.Context, user string) ([]SavedFilter, error)
}

// Estate is what names the set of devices one hub manages. There is one per
// hub.
type Estate struct {
	// Name is the name of the hub, which is the name of the estate.
	Name string
}

// Device is one managed Ze instance, as the hub holds it. It holds facts
// only: the display state is derived by DeviceDisplayState.
//
// A Device is larger than a function should take by value: pass a pointer.
type Device struct {
	Name string
	// Labels are the device's labels, at most LabelsMax, in the order the
	// source gives them. A key appears once.
	Labels []Label
	// Group is the configuration group of the device, the unit that assigns
	// a configuration template. It is empty when the device is in none.
	Group string
	// SecretSet reports whether the hub holds a secret for the device. The
	// secret itself never leaves the hub.
	SecretSet  bool
	Connection Connection
	Config     ConfigFacts
	// HealthReported reports whether a health report has arrived. Health
	// means nothing while it is false.
	HealthReported bool
	// Health holds the components of the last health report, at most
	// HealthComponentsMax.
	Health    []HealthComponent
	Inventory Inventory
	Latest    LatestSample
}

// Label is one key and value attached to a device, such as site=lon.
type Label struct {
	Key   string
	Value string
}

// Connection is the state of a device's managed connection.
type Connection struct {
	// Connected reports whether the device holds a managed connection now.
	Connected bool
	// Since is when the present connection was made. It means nothing while
	// Connected is false.
	Since time.Time
	// EverConnected reports whether the hub has ever seen the device.
	EverConnected bool
	// LastSeen is when the last connection ended. It means nothing while
	// Connected is true or EverConnected is false.
	LastSeen time.Time
	// SourceAddr is the address the device connects from. The invalid Addr
	// means the hub holds none.
	SourceAddr netip.Addr
	// RelayGranted reports whether the device's configuration grants a
	// profile to relayed commands.
	RelayGranted bool
	// RelayProfile is the profile the device grants. It means nothing while
	// RelayGranted is false.
	RelayProfile string
}

// ConfigFacts is the state of a device's configuration, as the hub holds it.
type ConfigFacts struct {
	State ConfigState
	// HubVersion is the version the hub holds for the device.
	HubVersion string
	// DeviceVersion is the version the device runs. It is empty when the
	// device never fetched one.
	DeviceVersion string
	// Template is the name of the template the configuration comes from. It
	// is empty when the configuration is the device's own.
	Template string
	// Fetched reports whether the device has fetched a configuration.
	Fetched bool
	// LastFetch is when the device last fetched. It means nothing while
	// Fetched is false.
	LastFetch time.Time
	// LastFetchResult is the result of that fetch, in the device's words.
	LastFetchResult string
}

// HealthComponent is one component of a device's health report.
type HealthComponent struct {
	Name   string
	State  HealthState
	Detail string
}

// Inventory is what a device is: its hardware and software identity.
type Inventory struct {
	// Reported reports whether the device has sent its inventory. The other
	// fields mean nothing while it is false.
	Reported        bool
	Model           string
	Serial          string
	SoftwareVersion string
	Build           string
	Kernel          string
	// ManagementAddr is the address of the device's own interface. The
	// invalid Addr means none is configured.
	ManagementAddr netip.Addr
	// StartedAt is when the device's daemon started.
	StartedAt time.Time
}

// LatestSample is the last sample a device reported.
type LatestSample struct {
	// Reported reports whether a sample has arrived. The other fields mean
	// nothing while it is false.
	Reported bool
	// At is when the sample was taken.
	At                  time.Time
	CPUPercent          float64
	MemoryPercent       float64
	StoragePercent      float64
	Load                float64
	MemoryUsedBytes     uint64
	MemoryTotalBytes    uint64
	StorageUsedBytes    uint64
	StorageTotalBytes   uint64
	TrafficInBps        uint64
	TrafficOutBps       uint64
	PeersEstablished    int
	PeersDown           int
	PeersAdminDown      int
	InterfacesUp        int
	InterfacesDown      int
	InterfacesAdminDown int
}

// TreeView is one way of grouping the estate into a tree: an ordered list of
// level keys, at most TreeLevelsMax. A key is a registered tree level or a
// label key.
type TreeView struct {
	// Name is the view's name in an address.
	Name string
	// Title is what the view menu shows.
	Title  string
	Levels []string
}

// SavedFilter is one filter a user saved under a name.
type SavedFilter struct {
	Name   string
	Filter string
	// Default reports whether the user made it the default of the estate's
	// devices tab.
	Default bool
}
