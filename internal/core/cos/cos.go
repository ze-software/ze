// Design: docs/architecture/traffic/cos-plugin.md -- shared CoS profile registry

package cos

import (
	"maps"
	"strings"
	"sync"
)

// Profile holds ingress and egress 802.1p QoS maps for a named
// class-of-service profile. Keys and values are 0-7 (3-bit PCP).
type Profile struct {
	IngressMap map[uint32]uint32 // received PCP -> internal priority
	EgressMap  map[uint32]uint32 // internal priority -> transmitted PCP
}

// ResolverFunc resolves class-of-service references for an interface unit.
// parentCoS is the interface-level setting, unitCoS is the per-unit setting.
// hasInlineMaps is true when the unit already has inline ingress/egress maps.
// Returns resolved maps (nil/nil when no profile applies) or an error on
// conflict or missing profile.
type ResolverFunc func(parentCoS, unitCoS string, hasInlineMaps bool) (ingress, egress map[uint32]uint32, err error)

var (
	mu       sync.RWMutex
	profiles = map[string]Profile{}
	resolver ResolverFunc
)

// Register stores a named profile, replacing any previous entry.
func Register(name string, p Profile) {
	mu.Lock()
	profiles[name] = p
	mu.Unlock()
}

// Lookup returns the profile for the given name and whether it exists.
func Lookup(name string) (Profile, bool) {
	mu.RLock()
	p, ok := profiles[name]
	mu.RUnlock()
	return p, ok
}

// All returns a copy of all registered profiles keyed by name.
func All() map[string]Profile {
	mu.RLock()
	defer mu.RUnlock()
	m := make(map[string]Profile, len(profiles))
	maps.Copy(m, profiles)
	return m
}

// Clear removes all registered profiles.
func Clear() {
	mu.Lock()
	profiles = map[string]Profile{}
	mu.Unlock()
}

// RegisterResolver sets the callback used by Resolve. The cos plugin
// registers this at init time; removing the plugin leaves no resolver,
// so Resolve becomes a no-op.
func RegisterResolver(fn ResolverFunc) {
	mu.Lock()
	resolver = fn
	mu.Unlock()
}

// ClearResolver removes the registered resolver.
func ClearResolver() {
	mu.Lock()
	resolver = nil
	mu.Unlock()
}

// Resolve delegates to the registered resolver. Returns nil, nil, nil
// when no resolver is registered (cos plugin absent).
func Resolve(parentCoS, unitCoS string, hasInlineMaps bool) (ingress, egress map[uint32]uint32, err error) {
	mu.RLock()
	fn := resolver
	mu.RUnlock()
	if fn == nil {
		return nil, nil, nil
	}
	return fn(parentCoS, unitCoS, hasInlineMaps)
}

const filterPrefix = "cos:"

// ParseFilterID extracts the profile name from a RADIUS Filter-Id
// value with a "cos:" prefix. Returns ("", false) for non-CoS values.
func ParseFilterID(filterID string) (string, bool) {
	if !strings.HasPrefix(filterID, filterPrefix) {
		return "", false
	}
	name := filterID[len(filterPrefix):]
	if name == "" {
		return "", false
	}
	return name, true
}
