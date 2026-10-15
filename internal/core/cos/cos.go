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

// ResolverFunc resolves class-of-service references for an interface unit
// against the profile set it is handed. parentCoS is the interface-level
// setting, unitCoS is the per-unit setting. hasInlineMaps is true when the
// unit already has inline ingress/egress maps. Returns resolved maps (nil/nil
// when no profile applies) or an error on conflict or missing profile. It
// MUST NOT read the installed profiles: a verify resolves a candidate whose
// profiles are not installed yet.
type ResolverFunc func(profiles map[string]Profile, parentCoS, unitCoS string, hasInlineMaps bool) (ingress, egress map[uint32]uint32, err error)

// ProfileParserFunc parses the class-of-service config section JSON
// ({"class-of-service": {...}}) into its profiles. It MUST NOT change any
// state: an interface verify calls it over a candidate that may never commit.
type ProfileParserFunc func(data string) (map[string]Profile, error)

var (
	mu       sync.RWMutex
	profiles = map[string]Profile{}
	resolver ResolverFunc
	parser   ProfileParserFunc
)

// Replace installs set as the profiles of the running config, replacing every
// previous entry in one step, so a reader never sees half of one set and half
// of the other. The caller MUST NOT change set or its maps afterwards: the
// registry keeps it as given. A nil set installs no profile.
func Replace(set map[string]Profile) {
	installed := make(map[string]Profile, len(set))
	maps.Copy(installed, set)
	mu.Lock()
	profiles = installed
	mu.Unlock()
}

// Lookup returns the installed profile for the given name and whether it exists.
func Lookup(name string) (Profile, bool) {
	mu.RLock()
	p, ok := profiles[name]
	mu.RUnlock()
	return p, ok
}

// All returns a copy of all installed profiles keyed by name.
func All() map[string]Profile {
	mu.RLock()
	defer mu.RUnlock()
	m := make(map[string]Profile, len(profiles))
	maps.Copy(m, profiles)
	return m
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

// RegisterProfileParser sets the callback used by ParseProfiles. The cos
// plugin registers it at init time, beside its resolver, so the interface
// component parses candidate profiles without importing the plugin.
func RegisterProfileParser(fn ProfileParserFunc) {
	mu.Lock()
	parser = fn
	mu.Unlock()
}

// ClearProfileParserForTest removes the registered profile parser. Only a test
// calls it, to undo the parser it registered; the cos plugin registers once at
// init and never removes it.
func ClearProfileParserForTest() {
	mu.Lock()
	parser = nil
	mu.Unlock()
}

// ParseProfiles delegates to the registered profile parser. With no parser
// registered (cos plugin absent) it returns no profile and no error: the
// class-of-service schema is the plugin's, so no section can carry one.
func ParseProfiles(data string) (map[string]Profile, error) {
	mu.RLock()
	fn := parser
	mu.RUnlock()
	if fn == nil {
		return map[string]Profile{}, nil
	}
	return fn(data)
}

// Resolve delegates to the registered resolver, over the given profile set.
// Returns nil, nil, nil when no resolver is registered (cos plugin absent).
func Resolve(set map[string]Profile, parentCoS, unitCoS string, hasInlineMaps bool) (ingress, egress map[uint32]uint32, err error) {
	mu.RLock()
	fn := resolver
	mu.RUnlock()
	if fn == nil {
		return nil, nil, nil
	}
	return fn(set, parentCoS, unitCoS, hasInlineMaps)
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
