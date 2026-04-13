// Design: docs/architecture/core-design.md -- redistribute source registry

package redistribute

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
)

// ErrSourceConflict is returned when a source name is re-registered with a different protocol.
var ErrSourceConflict = errors.New("redistribute: source protocol conflict")

// RouteSource describes a redistribution source.
type RouteSource struct {
	Name        string // "ibgp", "ebgp", "static", "connected", ...
	Protocol    string // "bgp", "static", "connected", "ospf", "isis"
	Description string
}

var (
	mu      sync.RWMutex
	sources = map[string]*RouteSource{}
)

// RegisterSource adds a route source to the registry.
// Each protocol component calls this for the sources it provides:
//   - BGP registers "bgp", "ibgp", "ebgp"
//   - iface registers "connected"
//   - future components register their own
//
// Re-registration with identical name and protocol is a no-op.
// Re-registration with a different protocol returns an error.
func RegisterSource(src RouteSource) error {
	mu.Lock()
	defer mu.Unlock()
	if existing, ok := sources[src.Name]; ok {
		if existing.Protocol != src.Protocol {
			return fmt.Errorf("%w: %q registered as %q, got %q",
				ErrSourceConflict, src.Name, existing.Protocol, src.Protocol)
		}
		slog.Debug("redistribute source already registered", "name", src.Name)
		return nil
	}
	sources[src.Name] = &src
	return nil
}

// SourceNames returns all registered source names in sorted order.
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
