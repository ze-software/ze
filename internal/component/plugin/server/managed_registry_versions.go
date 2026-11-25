// Design: docs/architecture/fleet-config.md -- the current hub version and held state of a managed device
// Related: managed_registry.go -- the in-memory session and configuration facts
// Related: managed_doctor.go -- withdrawManagedDoctor joins operator reads before Stop releases the store

package server

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"sync"

	"github.com/ze-software/ze/pkg/fleet"
)

// A registry reader borrows the running server's store. Withdrawal takes the
// write lock so Stop cannot return while an operator is still reading it.
var managedRegistryReaders sync.RWMutex

// A record can be forgotten while a command is reading its configuration.
var errRegistryDeviceForgotten = errors.New("managed device record was forgotten")

// The generation changes on every write, including writes during a config read.
// The cache is independent of the version a concurrent fetch served its client.
type managedVersionCache struct {
	hash       string
	generation uint64
	known      bool
}

// MarkManagedConfigChanged invalidates the one client's cached configuration
// hash. It performs no I/O and is safe to call from a storage write observer.
func (s *ManagedServer) MarkManagedConfigChanged(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if device := s.devices[name]; device != nil {
		device.version.generation++
		device.version.known = false
	}
}

// registrySnapshot joins the session facts with what the store currently
// serves and the presence of a held copy. An unreadable copy still exists.
func (s *ManagedServer) registrySnapshot() ([]ManagedDevice, error) {
	// Keep each name tied to the record it identified when this read began.
	type entryVersion struct {
		name    string
		device  *managedDevice
		version string
	}
	s.mu.Lock()
	entries := make([]entryVersion, 0, len(s.devices))
	for name, device := range s.devices {
		entries = append(entries, entryVersion{name: name, device: device})
	}
	s.mu.Unlock()
	diverged := make(map[string]bool)
	if records := s.svc.records; records != nil {
		names, err := HeldClients(records.store)
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			diverged[name] = true
		}
	}
	for i, entry := range entries {
		// A malformed history key can be listed and forgotten, but cannot name
		// a client configuration. Only a valid name has a version to look up.
		if !fleet.ValidDeviceKeyName(entry.name) {
			continue
		}
		version, err := s.currentConfigVersion(entry.name, entry.device)
		if errors.Is(err, errRegistryDeviceForgotten) {
			continue
		}
		if err != nil {
			return nil, err
		}
		entries[i].version = version
	}
	// Copy all session facts together after I/O. A new device under a
	// forgotten name belongs to the next snapshot, with its own version.
	devices := make([]ManagedDevice, 0, len(entries))
	s.mu.Lock()
	for _, entry := range entries {
		if s.devices[entry.name] != entry.device {
			continue
		}
		device := entry.device.snapshotLocked()
		device.HubVersion = entry.version
		device.Diverged = diverged[entry.name]
		devices = append(devices, device)
	}
	s.mu.Unlock()
	sort.Slice(devices, func(i, j int) bool { return devices[i].Name < devices[j].Name })
	return devices, nil
}

// currentConfigVersion uses the cache for every client whose config has not
// changed. The read and hash run outside mu. A concurrent write invalidates
// the read before it can cache an obsolete hash. Retry once to pick up a write
// during the read. If writes continue, return the value sampled during the
// second read and leave the cache stale for the next command.
func (s *ManagedServer) currentConfigVersion(name string, device *managedDevice) (string, error) {
	var hash string
	for range 2 {
		s.mu.Lock()
		if s.devices[name] != device {
			s.mu.Unlock()
			return "", errRegistryDeviceForgotten
		}
		cached := device.version
		s.mu.Unlock()
		if cached.known {
			return cached.hash, nil
		}
		data, err := s.svc.readConfig(name)
		hash = ""
		switch {
		case err == nil:
			hash = fleet.VersionHash(data)
		case errors.Is(err, fs.ErrNotExist), errors.Is(err, ErrClientConfigNotFound):
			// A declaration can precede its configuration file. Cache this absence.
			err = nil
		}
		s.mu.Lock()
		if s.devices[name] != device {
			s.mu.Unlock()
			return "", errRegistryDeviceForgotten
		}
		if err != nil {
			s.mu.Unlock()
			return "", fmt.Errorf("read the hub configuration for %s: %w", name, err)
		}
		current := device.version.generation == cached.generation
		if current {
			device.version.hash = hash
			device.version.known = true
		}
		s.mu.Unlock()
		if current {
			break
		}
	}
	return hash, nil
}
