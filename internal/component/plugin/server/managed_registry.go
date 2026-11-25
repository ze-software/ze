// Design: docs/architecture/fleet-config.md -- managed device registry on the hub
// Related: managed_serve.go -- the authenticated session produces each fact
// Related: managed_doctor.go -- publication of the running server

package server

import (
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

// ManagedDeviceRecord contains the facts that survive the end of a connection.
// Result and time fields stay absent until the corresponding event.
type ManagedDeviceRecord struct {
	Name            string             `json:"name"`
	FirstConnected  time.Time          `json:"first-connected,omitzero"`
	ConnectedSince  time.Time          `json:"connected-since,omitzero"`
	LastSeen        time.Time          `json:"last-seen,omitzero"`
	SourceAddress   netip.Addr         `json:"source-address,omitzero"`
	LastFetch       time.Time          `json:"last-fetch,omitzero"`
	DeviceVersion   string             `json:"device-version,omitempty"`
	LastFetchResult ManagedFetchResult `json:"last-fetch-result,omitzero"`
	LastFetchError  string             `json:"last-fetch-error,omitempty"`
	LastAck         time.Time          `json:"last-ack,omitzero"`
	AckResult       ManagedAckResult   `json:"ack-result,omitzero"`
	AckVersion      string             `json:"ack-version,omitempty"`
	AckError        string             `json:"ack-error,omitempty"`
	RemovedAt       time.Time          `json:"removed-at,omitzero"`
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
	hubVersion  string
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

// snapshotLocked copies the current session facts while the caller holds s.mu.
func (entry *managedDevice) snapshotLocked() ManagedDevice {
	return ManagedDevice{
		ManagedDeviceRecord: entry.ManagedDeviceRecord,
		Connected:           entry.conn != nil, Declared: entry.declared,
		Labels: maps.Clone(entry.labels), HubVersion: entry.hubVersion,
		Persisted: entry.persisted, Unreadable: entry.unreadable,
	}
}

func (s *ManagedServer) admitDevice(name string, addr netip.Addr, conn *rpc.MuxConn) error {
	return s.admitOnListener(name, addr, conn, nil, "")
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
	device := s.devices[name]
	device.LastFetch = s.now().UTC()
	device.DeviceVersion = version
	device.LastFetchError = ""
	switch {
	case fetchErr != nil:
		device.LastFetchResult = ManagedFetchError
		device.LastFetchError = boundedManagedDetail(fetchErr.Error())
		device.hubVersion = ""
	case resp.Status == "current":
		device.LastFetchResult = ManagedFetchCurrent
		device.hubVersion = version
	default:
		device.LastFetchResult = ManagedFetchServed
		device.hubVersion = resp.Version
	}
	s.dirtyDeviceLocked(name)
}

func (s *ManagedServer) recordAck(name string, ack fleet.ConfigAck, result ManagedAckResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	device := s.devices[name]
	device.LastAck = s.now().UTC()
	device.AckVersion = ack.Version
	device.AckResult = result
	device.AckError = ""
	if result != ManagedAckApplied {
		device.AckError = boundedManagedDetail(ack.Error)
	}
	s.dirtyDeviceLocked(name)
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

var fetchResultWords = [...]string{"", "served", "current", "error"}

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
