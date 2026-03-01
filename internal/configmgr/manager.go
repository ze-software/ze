// Design: docs/plan/spec-arch-0-system-boundaries.md — ConfigProvider implementation
// Design: docs/plan/spec-arch-4-config-manager.md — ConfigProvider spec

package configmgr

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"sync"

	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// ConfigManager implements ze.ConfigProvider.
// It stores config as map[string]any trees keyed by root name,
// tracks YANG schema registrations, and notifies watchers on reload.
// Phase 5 will wire this to internal/config/ loaders and YANG validation.
type ConfigManager struct {
	mu       sync.RWMutex
	roots    map[string]map[string]any
	schemas  map[string]string
	modules  []string
	watchers map[string][]chan ze.ConfigChange
}

// NewConfigManager creates a new ConfigManager.
func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		roots:    make(map[string]map[string]any),
		schemas:  make(map[string]string),
		watchers: make(map[string][]chan ze.ConfigChange),
	}
}

// Load reads a JSON config file and stores its top-level keys as roots.
// Notifies watchers for any root that changed or appeared.
// Phase 5 will replace JSON with internal/config/ YANG-aware parsing.
func (m *ConfigManager) Load(path string) error {
	data, err := os.ReadFile(path) //nolint:gosec // Config path is user-provided
	if err != nil {
		return fmt.Errorf("load config %q: %w", path, err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse config %q: %w", path, err)
	}

	m.mu.Lock()

	// Extract top-level roots — each key is a root name.
	changedRoots := make(map[string]map[string]any)
	for key, val := range raw {
		subtree, ok := val.(map[string]any)
		if !ok {
			// Non-map top-level values stored as single-key map.
			subtree = map[string]any{key: val}
		}
		m.roots[key] = subtree
		changedRoots[key] = subtree
	}

	// Collect watchers to notify (under lock to snapshot channels).
	type notification struct {
		ch     chan ze.ConfigChange
		change ze.ConfigChange
	}
	var notifications []notification
	for root, tree := range changedRoots {
		for _, ch := range m.watchers[root] {
			notifications = append(notifications, notification{
				ch:     ch,
				change: ze.ConfigChange{Root: root, Tree: tree},
			})
		}
	}

	m.mu.Unlock()

	// Send notifications outside lock — non-blocking to avoid deadlock.
	for _, n := range notifications {
		select {
		case n.ch <- n.change:
		default: // non-blocking send — channel full, replace oldest
			select {
			case <-n.ch:
			default: // channel already drained by concurrent reader
			}
			n.ch <- n.change
		}
	}

	return nil
}

// Get returns the config subtree for a root name.
// Returns empty map if the root does not exist.
func (m *ConfigManager) Get(root string) (map[string]any, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tree, ok := m.roots[root]
	if !ok {
		return map[string]any{}, nil
	}

	// Return a shallow copy to prevent mutation.
	result := make(map[string]any, len(tree))
	maps.Copy(result, tree)
	return result, nil
}

// Validate checks the current config against the merged YANG schema.
// Phase 5 will wire this to internal/config/yang_schema.go validation.
// For now, returns no errors (standalone stub).
func (m *ConfigManager) Validate() []error {
	return nil
}

// Save writes the current config to a JSON file.
// Phase 5 will use internal/config/serialize.go for proper config format.
func (m *ConfigManager) Save(path string) error {
	m.mu.RLock()
	data, err := json.MarshalIndent(m.roots, "", "  ")
	m.mu.RUnlock()

	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0o600)
}

// Watch returns a channel that receives notifications when
// the config for a given root changes (e.g., on Load reload).
func (m *ConfigManager) Watch(root string) <-chan ze.ConfigChange {
	ch := make(chan ze.ConfigChange, 1)

	m.mu.Lock()
	m.watchers[root] = append(m.watchers[root], ch)
	m.mu.Unlock()

	return ch
}

// Schema returns the merged YANG schema info.
func (m *ConfigManager) Schema() ze.SchemaTree {
	m.mu.RLock()
	defer m.mu.RUnlock()

	modules := make([]string, len(m.modules))
	copy(modules, m.modules)
	return ze.SchemaTree{Modules: modules}
}

// RegisterSchema adds a plugin's YANG schema to the merged schema.
// Returns error if the name is already registered.
func (m *ConfigManager) RegisterSchema(name, yang string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.schemas[name]; exists {
		return fmt.Errorf("schema %q already registered", name)
	}

	m.schemas[name] = yang
	m.modules = append(m.modules, name)
	return nil
}
