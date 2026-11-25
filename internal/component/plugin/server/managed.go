// Design: docs/architecture/fleet-config.md — hub-side managed config handlers
// Related: config.go — ServerConfig holds hub transport config
// Related: managed_held.go — the held records HandleConfigFetch names and clears

package server

import (
	"encoding/base64"
	"errors"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/pkg/fleet"
)

// ErrClientConfigNotFound is returned when no config exists for a client name.
var ErrClientConfigNotFound = errors.New("client config not found")

// ErrDuplicateClient is returned when a client with the same name is already connected.
var ErrDuplicateClient = errors.New("duplicate client name")

// ConfigReader reads a client's config by name from the hub's blob store.
// Returns the raw config bytes, or ErrClientConfigNotFound if the client
// has no config entry.
type ConfigReader func(name string) ([]byte, error)

// ManagedConfigService handles hub-side config-fetch and config-changed operations
// for managed clients. It reads client configs via a ConfigReader and computes
// version hashes for change detection. Safe for concurrent use.
type ManagedConfigService struct {
	readConfig ConfigReader
	// records holds the copy each holding client sent (heldRecords). nil
	// keeps none: no answer names a held version and nothing is cleared.
	records *heldRecords
}

// newManagedConfigService creates a service that reads client configs via reader.
func newManagedConfigService(reader ConfigReader) *ManagedConfigService {
	return &ManagedConfigService{readConfig: reader}
}

// HandleConfigFetch processes a config-fetch request from a managed client.
// If the client's version matches the current config, returns status "current".
// Otherwise returns the full config as base64 with the new version hash, and
// held-version names the copy the hub holds of a configuration the client
// holds, so a client that holds sends no second copy of the same version.
// discard-version carries an operator's revert decision to the client: the
// device hash D the client is to discard, offered only while the client
// fetches with D and the hub serves it the configuration the operator saw
// (heldRecords.offer). An unreadable record offers no revert either.
//
// A "current" answer removes the client's held record: it proves the hub and
// the client hold the same configuration, so the copy shows nothing to
// resolve. A record that cannot be removed or read is logged and the fetch is
// still answered, because an error would end the client's fetch and make it
// reconnect. An unreadable record is answered as no held-version, which is a
// guard and not an absence: the client then sends a copy, and its write
// replaces the record that did not decode.
func (s *ManagedConfigService) HandleConfigFetch(clientName string, req fleet.ConfigFetchRequest) (fleet.ConfigFetchResponse, error) {
	data, err := s.readConfig(clientName)
	if err != nil {
		return fleet.ConfigFetchResponse{}, err
	}

	version := fleet.VersionHash(data)

	if req.Version == version {
		if clearErr := s.records.clear(clientName); clearErr != nil {
			managedLogger().Warn("the hub could not remove the held copy of a managed client that is now in sync",
				"code", diagnostic.CodeDoctorStoreIntegrity, "name", clientName, "error", clearErr)
		}
		return fleet.ConfigFetchResponse{Status: "current"}, nil
	}

	held, discard, heldErr := s.records.offer(clientName, req.Version, version)
	if heldErr != nil {
		managedLogger().Warn("the hub could not read the held copy of a managed client's configuration, so it names none and offers no revert",
			"code", diagnostic.CodeDoctorStoreIntegrity, "name", clientName, "error", heldErr)
	}
	return fleet.ConfigFetchResponse{
		Version:        version,
		Config:         base64.StdEncoding.EncodeToString(data),
		HeldVersion:    held,
		DiscardVersion: discard,
	}, nil
}

// BuildConfigChanged creates a config-changed notification for a client.
// Reads the client's current config and computes its version hash.
func (s *ManagedConfigService) BuildConfigChanged(clientName string) (fleet.ConfigChanged, error) {
	data, err := s.readConfig(clientName)
	if err != nil {
		return fleet.ConfigChanged{}, err
	}

	return fleet.ConfigChanged{
		Version: fleet.VersionHash(data),
	}, nil
}
