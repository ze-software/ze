// Design: docs/architecture/hub-architecture.md — hub coordination

package hub

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	pluginserver "codeberg.org/thomas-mangin/ze/internal/plugin/server"
)

// ConfigState represents which config state to query.
type ConfigState int

const (
	// ConfigLive is the running configuration.
	ConfigLive ConfigState = iota
	// ConfigEdit is the candidate configuration being modified.
	ConfigEdit
)

// ConfigStore holds live and edit configuration states (VyOS-style).
type ConfigStore struct {
	live map[string]any
	edit map[string]any
	mu   sync.RWMutex
}

// NewConfigStore creates a new configuration store.
func NewConfigStore() *ConfigStore {
	return &ConfigStore{
		live: make(map[string]any),
		edit: make(map[string]any),
	}
}

// SetEdit sets the candidate (edit) configuration.
func (s *ConfigStore) SetEdit(cfg map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.edit = cfg
}

// SetLive sets the live configuration directly.
func (s *ConfigStore) SetLive(cfg map[string]any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.live = cfg
}

// Apply makes the edit configuration become the live configuration.
func (s *ConfigStore) Apply() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.live = deepCopy(s.edit)
}

// Query retrieves configuration at the given path.
// Path format: "bgp" or "bgp.peer.192.0.2.1.remote-as".
func (s *ConfigStore) Query(state ConfigState, path string) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var cfg map[string]any
	switch state {
	case ConfigLive:
		cfg = s.live
	case ConfigEdit:
		cfg = s.edit
	default:
		return nil, fmt.Errorf("unknown config state")
	}

	if cfg == nil {
		return nil, fmt.Errorf("path not found: %s", path)
	}

	return queryPath(cfg, path)
}

// GetLive returns the entire live configuration.
func (s *ConfigStore) GetLive() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.live
}

// GetEdit returns the entire edit configuration.
func (s *ConfigStore) GetEdit() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.edit
}

// queryPath navigates to a specific path in the config.
func queryPath(cfg map[string]any, path string) (any, error) {
	if path == "" {
		return cfg, nil
	}

	parts := strings.Split(path, ".")
	current := any(cfg)

	for i, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("path not found: %s (at %s)", path, strings.Join(parts[:i], "."))
		}

		val, exists := m[part]
		if !exists {
			return nil, fmt.Errorf("path not found: %s", path)
		}

		current = val
	}

	return current, nil
}

// deepCopy creates a deep copy of a config map.
func deepCopy(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}

	dst := make(map[string]any, len(src))
	for k, v := range src {
		switch val := v.(type) {
		case map[string]any:
			dst[k] = deepCopy(val)
		case []any:
			dst[k] = copySlice(val)
		default:
			dst[k] = v
		}
	}
	return dst
}

// copySlice creates a copy of a slice.
func copySlice(src []any) []any {
	if src == nil {
		return nil
	}
	dst := make([]any, len(src))
	for i, v := range src {
		switch val := v.(type) {
		case map[string]any:
			dst[i] = deepCopy(val)
		case []any:
			dst[i] = copySlice(val)
		default:
			dst[i] = v
		}
	}
	return dst
}

// SchemasByPriority returns all registered schemas sorted by priority (lower first).
func (o *Orchestrator) SchemasByPriority() []*pluginserver.Schema {
	modules := o.registry.ListModules()
	schemas := make([]*pluginserver.Schema, 0, len(modules))

	for _, name := range modules {
		schema, err := o.registry.GetByModule(name)
		if err == nil {
			schemas = append(schemas, schema)
		}
	}

	// Sort by priority (lower = processed first)
	sort.Slice(schemas, func(i, j int) bool {
		return schemas[i].Priority < schemas[j].Priority
	})

	return schemas
}
