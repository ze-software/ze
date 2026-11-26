// Design: docs/architecture/fleet-config.md -- managed device registry on the hub
// Related: managed_serve.go -- the authenticated session produces each fact
// Related: managed_doctor.go -- publication of the running server

package server

import (
	"errors"
	"fmt"
	"maps"
	"net/netip"
	"slices"
	"time"
	"unicode/utf8"

	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ManagedInventorySnapshot is an immutable hub-side record of one accepted
// inventory report together with its UTC receipt timestamp.
type ManagedInventorySnapshot struct {
	Report     fleet.InventoryReport `json:"report"`
	ReceivedAt time.Time             `json:"received-at"`
}

// Clone returns a deep copy of s, or nil when s is nil.
func (s *ManagedInventorySnapshot) Clone() *ManagedInventorySnapshot {
	// refuse-by-default: guards snapshotLocked -- absent inventory remains nil for unreported devices
	if s == nil {
		return nil
	}
	return &ManagedInventorySnapshot{
		Report:     s.Report.Clone(),
		ReceivedAt: s.ReceivedAt,
	}
}

// ManagedHealthSnapshot is an immutable hub-side record of one accepted health
// report together with its UTC receipt timestamp.
type ManagedHealthSnapshot struct {
	Report     fleet.HealthReport `json:"report"`
	ReceivedAt time.Time          `json:"received-at"`
}

// Clone returns a deep copy of s, or nil when s is nil.
func (s *ManagedHealthSnapshot) Clone() *ManagedHealthSnapshot {
	// refuse-by-default: guards snapshotLocked -- absent health remains nil for unreported devices
	if s == nil {
		return nil
	}
	return &ManagedHealthSnapshot{
		Report:     s.Report.Clone(),
		ReceivedAt: s.ReceivedAt,
	}
}

// ManagedDeviceRecord contains the facts that survive the end of a connection.
// Result and time fields stay absent until the corresponding event. Inventory
// and Health hold pointers to immutable snapshots so the struct remains
// comparable and each accepted report installs a distinct reference.
type ManagedDeviceRecord struct {
	Name            string                    `json:"name"`
	FirstConnected  time.Time                 `json:"first-connected,omitzero"`
	ConnectedSince  time.Time                 `json:"connected-since,omitzero"`
	LastSeen        time.Time                 `json:"last-seen,omitzero"`
	SourceAddress   netip.Addr                `json:"source-address,omitzero"`
	LastFetch       time.Time                 `json:"last-fetch,omitzero"`
	DeviceVersion   string                    `json:"device-version,omitempty"`
	LastFetchResult ManagedFetchResult        `json:"last-fetch-result,omitzero"`
	LastFetchError  string                    `json:"last-fetch-error,omitempty"`
	LastAck         time.Time                 `json:"last-ack,omitzero"`
	AckResult       ManagedAckResult          `json:"ack-result,omitzero"`
	AckVersion      string                    `json:"ack-version,omitempty"`
	AckError        string                    `json:"ack-error,omitempty"`
	RemovedAt       time.Time                 `json:"removed-at,omitzero"`
	Inventory       *ManagedInventorySnapshot `json:"inventory,omitempty"`
	Health          *ManagedHealthSnapshot    `json:"health,omitempty"`
}

// ManagedDevice is a caller-owned snapshot. The live fields die with the server.
type ManagedDevice struct {
	ManagedDeviceRecord
	Connected  bool              `json:"connected"`
	Declared   bool              `json:"declared"`
	Labels     map[string]string `json:"labels"`
	HubVersion string            `json:"hub-version"`
	Persisted  bool              `json:"persisted"`
	Unreadable bool              `json:"unreadable"`
	Diverged   bool              `json:"diverged"`
}

// managedDevice keeps connectivity in the session pointer. All fields are
// guarded by ManagedServer.mu. A listener names the devices it may admit.
type managedDevice struct {
	ManagedDeviceRecord
	conn        *rpc.MuxConn
	bound       *managedBound // The block that authenticated this session.
	secret      string        // Used to retire a session whose admission secret was rotated.
	sessionDone chan struct{}
	labels      map[string]string
	version     managedVersionCache
	declared    bool
	persisted   bool
	unreadable  bool
	// Last durable bytes distinguish our observer echo from an operator overwrite.
	durable            []byte
	readFailure        string
	forgetOnDisconnect bool
}

// ManagedRegistrySnapshot returns the device facts from the running managed
// server. Safe for concurrent use.
func ManagedRegistrySnapshot() ([]ManagedDevice, error) {
	managedRegistryReaders.RLock()
	defer managedRegistryReaders.RUnlock()
	s := managedDoctorSource.Load()
	if s == nil {
		return nil, ErrManagedNotServing
	}
	return s.registrySnapshot()
}

// ManagedDeviceByName reads one device's current session, configuration and held state.
func ManagedDeviceByName(name string) (ManagedDevice, error) {
	managedRegistryReaders.RLock()
	defer managedRegistryReaders.RUnlock()
	s := managedDoctorSource.Load()
	if s == nil {
		return ManagedDevice{}, ErrManagedNotServing
	}
	return s.registryDevice(name)
}

// ManagedDeviceNames returns the names known to the running hub for command
// completion. A daemon without a published registry has no names to complete.
func ManagedDeviceNames() []string {
	s := managedDoctorSource.Load()
	if s == nil {
		return nil
	}
	s.mu.Lock()
	names := make([]string, 0, len(s.devices))
	for name := range s.devices {
		if s.visibleDeviceLocked(name) != nil {
			names = append(names, name)
		}
	}
	s.mu.Unlock()
	slices.Sort(names)
	return names
}

// visibleDeviceLocked hides an undeclared device whose history has been
// deleted. Its private session entry survives until the close waiter wakes.
func (s *ManagedServer) visibleDeviceLocked(name string) *managedDevice {
	device := s.devices[name]
	if device != nil && device.forgetOnDisconnect && !device.declared {
		return nil
	}
	return device
}

// snapshotLocked copies the current session facts while the caller holds s.mu.
// Inventory and Health snapshots are deep-cloned so callers cannot mutate live
// or writer-retained state.
func (entry *managedDevice) snapshotLocked() ManagedDevice {
	record := entry.ManagedDeviceRecord
	record.Inventory = entry.Inventory.Clone()
	record.Health = entry.Health.Clone()
	return ManagedDevice{
		ManagedDeviceRecord: record,
		Connected:           entry.conn != nil, Declared: entry.declared,
		Labels:    maps.Clone(entry.labels),
		Persisted: entry.persisted, Unreadable: entry.unreadable,
	}
}

func (s *ManagedServer) admitOnListener(name string, addr netip.Addr, conn *rpc.MuxConn, bound *managedBound, secret string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	device := s.devices[name]
	if device == nil {
		return &pluginipc.AuthRefusal{Claimed: name, Cause: pluginipc.AuthRefusedNoSecret}
	}
	if !device.declared {
		return &pluginipc.AuthRefusal{Claimed: name, Cause: pluginipc.AuthRefusedNoSecret}
	}
	if bound != nil {
		expected := bound.clients[name]
		if expected == "" {
			return &pluginipc.AuthRefusal{Claimed: name, Cause: pluginipc.AuthRefusedNoSecret}
		}
		if expected != secret {
			return &pluginipc.AuthRefusal{Claimed: name, Cause: pluginipc.AuthRefusedToken}
		}
	}
	if device.conn != nil {
		return ErrDuplicateClient
	}
	device.conn, device.bound, device.secret = conn, bound, secret
	device.sessionDone = make(chan struct{})
	device.ConnectedSince = s.now().UTC()
	if device.FirstConnected.IsZero() {
		device.FirstConnected = device.ConnectedSince
	}
	device.SourceAddress = addr.Unmap()
	s.dirtyDeviceLocked(name)
	return nil
}

func (s *ManagedServer) disconnectDevice(name string, conn *rpc.MuxConn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	device := s.devices[name]
	if device == nil {
		return // The record has already been removed.
	}
	if device.conn != conn {
		return // A later admission already replaced this session.
	}
	device.conn, device.bound, device.secret = nil, nil, ""
	close(device.sessionDone)
	device.sessionDone = nil
	delete(s.pending, name)
	if device.forgetOnDisconnect {
		device.forgetOnDisconnect = false
		if !device.declared {
			delete(s.devices, name)
			return
		}
	}
	device.LastSeen = s.now().UTC()
	s.dirtyDeviceLocked(name)
}

func (s *ManagedServer) recordFetch(name, version string, resp fleet.ConfigFetchResponse, fetchErr error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	device := s.visibleDeviceLocked(name)
	if device == nil {
		return // The operator forgot this record before the fetch completed.
	}
	device.LastFetch = s.now().UTC()
	device.DeviceVersion = version
	device.LastFetchError = ""
	switch {
	case fetchErr != nil:
		device.LastFetchResult = ManagedFetchError
		device.LastFetchError = boundedManagedDetail(fetchErr.Error())
	case resp.Status == managedFetchStatusCurrent:
		device.LastFetchResult = ManagedFetchCurrent
	default:
		device.LastFetchResult = ManagedFetchServed
	}
	s.dirtyDeviceLocked(name)
}

func (s *ManagedServer) recordAck(name string, ack fleet.ConfigAck, result ManagedAckResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	device := s.visibleDeviceLocked(name)
	if device == nil {
		return // A late acknowledgement cannot restore a forgotten record.
	}
	device.LastAck = s.now().UTC()
	device.AckVersion = ack.Version
	device.AckResult = result
	device.AckError = ""
	if result != ManagedAckApplied {
		device.AckError = boundedManagedDetail(ack.Error)
	}
	s.dirtyDeviceLocked(name)
}

func (s *ManagedServer) recordInventoryReport(name string, mc *rpc.MuxConn, report fleet.InventoryReport) error {
	if mc == nil {
		return errors.New("inventory report requires an active authenticated session")
	}
	if err := report.Validate(); err != nil {
		return err
	}
	cloned := report.Clone()
	s.mu.Lock()
	defer s.mu.Unlock()
	device := s.visibleDeviceLocked(name)
	if device == nil {
		return errors.New("inventory report session is no longer active")
	}
	if device.conn != mc {
		return errors.New("inventory report session is superseded")
	}
	device.Inventory = &ManagedInventorySnapshot{
		Report:     cloned,
		ReceivedAt: s.currentTime().UTC(),
	}
	device.persisted = false
	s.dirtyDeviceLocked(name)
	return nil
}

func (s *ManagedServer) recordHealthReport(name string, mc *rpc.MuxConn, report fleet.HealthReport) error {
	if mc == nil {
		return errors.New("health report requires an active authenticated session")
	}
	if err := report.Validate(); err != nil {
		return err
	}
	cloned := report.Clone()
	s.mu.Lock()
	defer s.mu.Unlock()
	device := s.visibleDeviceLocked(name)
	if device == nil {
		return errors.New("health report session is no longer active")
	}
	if device.conn != mc {
		return errors.New("health report session is superseded")
	}
	device.Health = &ManagedHealthSnapshot{
		Report:     cloned,
		ReceivedAt: s.currentTime().UTC(),
	}
	device.persisted = false
	s.dirtyDeviceLocked(name)
	return nil
}

func boundedManagedDetail(detail string) string {
	if len(detail) <= managedDetailMax {
		return detail
	}
	end := managedDetailMax
	for end > 0 && !utf8.RuneStart(detail[end]) {
		end--
	}
	return detail[:end]
}

// ManagedFetchResult says what the hub answered the most recent config-fetch.
type ManagedFetchResult uint8

const (
	ManagedFetchUnspecified ManagedFetchResult = iota
	ManagedFetchServed
	ManagedFetchCurrent
	ManagedFetchError
)

var fetchResultWords = [...]string{"", "served", managedFetchStatusCurrent, "error"}

// ManagedAckResult says what the hub recorded for the most recent config-ack.
type ManagedAckResult uint8

const (
	ManagedAckUnspecified ManagedAckResult = iota
	ManagedAckApplied
	ManagedAckRefused
	ManagedAckHolding
)

var ackResultWords = [...]string{"", "applied", "refused", "holding"}

// String returns the operator-facing spelling, empty before the first fetch.
func (r ManagedFetchResult) String() string { return fetchResultWords[r] }

// String returns the operator-facing spelling, empty before the first ack.
func (r ManagedAckResult) String() string { return ackResultWords[r] }

// MarshalText encodes the wire spelling of a recorded fetch result.
func (r ManagedFetchResult) MarshalText() ([]byte, error) { return []byte(r.String()), nil }

// MarshalText encodes the wire spelling of a recorded ack result.
func (r ManagedAckResult) MarshalText() ([]byte, error) { return []byte(r.String()), nil }

// UnmarshalText refuses a fetch result the hub cannot interpret.
func (r *ManagedFetchResult) UnmarshalText(text []byte) error {
	index := slices.Index(fetchResultWords[1:], string(text))
	if index < 0 {
		return fmt.Errorf("unknown managed fetch result %q", text)
	}
	*r = ManagedFetchResult(index + 1)
	return nil
}

// UnmarshalText refuses an ack result the hub cannot interpret.
func (r *ManagedAckResult) UnmarshalText(text []byte) error {
	index := slices.Index(ackResultWords[1:], string(text))
	if index < 0 {
		return fmt.Errorf("unknown managed ack result %q", text)
	}
	*r = ManagedAckResult(index + 1)
	return nil
}
