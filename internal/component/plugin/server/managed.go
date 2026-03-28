// Design: docs/architecture/fleet-config.md — hub-side managed config handlers
// Related: config.go — ServerConfig holds hub transport config

package server

import (
	"encoding/base64"
	"errors"
	"sync"

	"codeberg.org/thomas-mangin/ze/pkg/fleet"
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
// version hashes for change detection. Tracks connected clients and rejects
// duplicate names. Safe for concurrent use.
type ManagedConfigService struct {
	readConfig ConfigReader
	mu         sync.Mutex
	connected  map[string]struct{} // connected client names
}

// NewManagedConfigService creates a service that reads client configs via reader.
func NewManagedConfigService(reader ConfigReader) *ManagedConfigService {
	return &ManagedConfigService{
		readConfig: reader,
		connected:  make(map[string]struct{}),
	}
}

// RegisterClient marks a client as connected. Returns ErrDuplicateClient
// if a client with the same name is already connected.
// Caller MUST call UnregisterClient when the client disconnects.
func (s *ManagedConfigService) RegisterClient(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.connected[name]; exists {
		return ErrDuplicateClient
	}
	s.connected[name] = struct{}{}
	return nil
}

// UnregisterClient removes a client from the connected set.
func (s *ManagedConfigService) UnregisterClient(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.connected, name)
}

// HandleConfigFetch processes a config-fetch request from a managed client.
// If the client's version matches the current config, returns status "current".
// Otherwise returns the full config as base64 with the new version hash.
func (s *ManagedConfigService) HandleConfigFetch(clientName string, req fleet.ConfigFetchRequest) (fleet.ConfigFetchResponse, error) {
	data, err := s.readConfig(clientName)
	if err != nil {
		return fleet.ConfigFetchResponse{}, err
	}

	version := fleet.VersionHash(data)

	if req.Version == version {
		return fleet.ConfigFetchResponse{Status: "current"}, nil
	}

	return fleet.ConfigFetchResponse{
		Version: version,
		Config:  base64.StdEncoding.EncodeToString(data),
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
