// Design: docs/plan/spec-arch-0-system-boundaries.md — ConfigProvider implementation
// Design: docs/plan/spec-arch-4-config-manager.md — ConfigProvider spec

package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"sync"

	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// Provider implements ze.ConfigProvider.
// It stores config as map[string]any trees keyed by root name,
// tracks YANG schema registrations, and notifies watchers on reload.
// Phase 5 will wire this to internal/config/ loaders and YANG validation.
type Provider struct {
	mu       sync.RWMutex
	roots    map[string]map[string]any
	schemas  map[string]string
	modules  []string
	watchers map[string][]chan ze.ConfigChange
}

// NewProvider creates a new Provider.
func NewProvider() *Provider {
	return &Provider{
		roots:    make(map[string]map[string]any),
		schemas:  make(map[string]string),
		watchers: make(map[string][]chan ze.ConfigChange),
	}
}

// Load reads a JSON config file and stores its top-level keys as roots.
// Notifies watchers for any root that changed or appeared.
// Phase 5 will replace JSON with internal/config/ YANG-aware parsing.
func (p *Provider) Load(path string) error {
	data, err := os.ReadFile(path) //nolint:gosec // Config path is user-provided
	if err != nil {
		return fmt.Errorf("load config %q: %w", path, err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse config %q: %w", path, err)
	}

	p.mu.Lock()

	// Extract top-level roots — each key is a root name.
	changedRoots := make(map[string]map[string]any)
	for key, val := range raw {
		subtree, ok := val.(map[string]any)
		if !ok {
			// Non-map top-level values stored as single-key map.
			subtree = map[string]any{key: val}
		}
		p.roots[key] = subtree
		changedRoots[key] = subtree
	}

	// Collect watchers to notify (under lock to snapshot channels).
	type notification struct {
		ch     chan ze.ConfigChange
		change ze.ConfigChange
	}
	var notifications []notification
	for root, tree := range changedRoots {
		for _, ch := range p.watchers[root] {
			notifications = append(notifications, notification{
				ch:     ch,
				change: ze.ConfigChange{Root: root, Tree: tree},
			})
		}
	}

	p.mu.Unlock()

	// Send notifications outside lock — non-blocking to avoid deadlock.
	// If channel is full, drain stale value then send latest.
	for _, n := range notifications {
		if !trySend(n.ch, n.change) {
			tryDrain(n.ch)
			n.ch <- n.change
		}
	}

	return nil
}

func trySend(ch chan ze.ConfigChange, change ze.ConfigChange) bool {
	select {
	case ch <- change:
		return true
	default:
		return false
	}
}

func tryDrain(ch chan ze.ConfigChange) {
	select {
	case <-ch:
		return
	default:
		return
	}
}

// Get returns the config subtree for a root name.
// Returns empty map if the root does not exist.
func (p *Provider) Get(root string) (map[string]any, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	tree, ok := p.roots[root]
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
func (p *Provider) Validate() []error {
	return nil
}

// Save writes the current config to a JSON file.
// Phase 5 will use internal/config/serialize.go for proper config format.
func (p *Provider) Save(path string) error {
	p.mu.RLock()
	data, err := json.MarshalIndent(p.roots, "", "  ")
	p.mu.RUnlock()

	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0o600)
}

// Watch returns a channel that receives notifications when
// the config for a given root changes (e.g., on Load reload).
func (p *Provider) Watch(root string) <-chan ze.ConfigChange {
	ch := make(chan ze.ConfigChange, 1)

	p.mu.Lock()
	p.watchers[root] = append(p.watchers[root], ch)
	p.mu.Unlock()

	return ch
}

// Schema returns the merged YANG schema info.
func (p *Provider) Schema() ze.SchemaTree {
	p.mu.RLock()
	defer p.mu.RUnlock()

	modules := make([]string, len(p.modules))
	copy(modules, p.modules)
	return ze.SchemaTree{Modules: modules}
}

// RegisterSchema adds a plugin's YANG schema to the merged schema.
// Returns error if the name is already registered.
func (p *Provider) RegisterSchema(name, yang string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.schemas[name]; exists {
		return fmt.Errorf("schema %q already registered", name)
	}

	p.schemas[name] = yang
	p.modules = append(p.modules, name)
	return nil
}
