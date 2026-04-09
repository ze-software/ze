// Design: docs/architecture/core-design.md -- redistribute source registry

package redistribute

import (
	"sort"
	"sync"
)

// RouteSource describes a redistribution source/destination.
type RouteSource struct {
	Name        string // "ibgp", "ebgp"
	Protocol    string // "bgp"
	Description string
}

var (
	mu      sync.RWMutex
	sources = map[string]*RouteSource{}
)

// RegisterSource adds a route source to the registry.
// Called from init() functions by protocol packages.
func RegisterSource(src RouteSource) {
	mu.Lock()
	defer mu.Unlock()
	sources[src.Name] = &src
}

// SourceNames returns all registered source names in sorted order.
// Used by the redistribute-source validator for autocomplete.
func SourceNames() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(sources))
	for n := range sources {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// LookupSource returns the route source for the given name.
func LookupSource(name string) (*RouteSource, bool) {
	mu.RLock()
	defer mu.RUnlock()
	s, ok := sources[name]
	return s, ok
}
