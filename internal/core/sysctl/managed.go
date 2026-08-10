// Design: docs/architecture/core-design.md -- managed sysctl key registry
//
// Managed keys are sysctl keys owned by a higher-level config abstraction
// (e.g., system conntrack). When a key is managed, it must not appear in
// the sysctl {} config block directly. The sysctl plugin verifier checks
// this registry to reject dual-setting conflicts.

package sysctl

import "fmt"

// managedKey describes a sysctl key owned by a config abstraction.
type managedKey struct {
	SysctlKey    string // e.g., "net.netfilter.nf_conntrack_max"
	FriendlyName string // e.g., "system conntrack table-size"
}

var managedKeys = map[string]managedKey{}

// RegisterManagedKeys registers a set of sysctl keys as managed by a
// higher-level config abstraction. Called from init() in the owning package.
func RegisterManagedKeys(keys map[string]string) {
	mu.Lock()
	defer mu.Unlock()

	for sysctlKey, friendlyName := range keys {
		managedKeys[sysctlKey] = managedKey{
			SysctlKey:    sysctlKey,
			FriendlyName: friendlyName,
		}
	}
}

// CheckManaged returns an error if the key is managed by a config abstraction.
func CheckManaged(key string) error {
	mu.RLock()
	defer mu.RUnlock()

	if mk, ok := managedKeys[key]; ok {
		return fmt.Errorf("%s is managed by system conntrack %s; remove it from sysctl config", mk.SysctlKey, mk.FriendlyName)
	}
	return nil
}

// ResetManaged clears all managed key registrations. Only for use in tests.
func ResetManaged() {
	mu.Lock()
	defer mu.Unlock()
	managedKeys = map[string]managedKey{}
}
