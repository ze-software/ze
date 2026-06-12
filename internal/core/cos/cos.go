// Design: plan/spec-cos-plugin.md -- shared CoS profile registry

package cos

import "sync"

// Profile holds ingress and egress 802.1p QoS maps for a named
// class-of-service profile. Keys and values are 0-7 (3-bit PCP).
type Profile struct {
	IngressMap map[uint32]uint32 // received PCP -> internal priority
	EgressMap  map[uint32]uint32 // internal priority -> transmitted PCP
}

var (
	mu       sync.RWMutex
	profiles = map[string]Profile{}
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

// Clear removes all registered profiles.
func Clear() {
	mu.Lock()
	profiles = map[string]Profile{}
	mu.Unlock()
}
