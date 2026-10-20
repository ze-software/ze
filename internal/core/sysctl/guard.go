// Design: docs/architecture/core-design.md -- sysctl write guards
// Related: conflicts.go -- the other registry a consumer of a sysctl key extends

package sysctl

import (
	"errors"
	"maps"
	"slices"
	"sync"
)

// WriteGuard judges one runtime sysctl write before it reaches the kernel. It
// returns nil to let the write through, or an error naming what is wrong, why,
// and the fix, which the write's caller shows the operator.
//
// A guard is how a component that depends on a kernel setting refuses a
// runtime write that would break it, without the sysctl component naming that
// component: the vrrp plugin refuses proxy ARP on a device a VRRP group uses.
// A guard MUST be safe for concurrent use and MUST NOT write a sysctl itself.
type WriteGuard func(key, value string) error

var (
	guardMu sync.RWMutex
	guards  = map[string]WriteGuard{}
)

// errGuardNameEmpty and errGuardNil refuse a registration that could not be
// named in a refusal or that would panic on the first write.
var (
	errGuardNameEmpty = errors.New("sysctl: a write guard needs a name")
	errGuardNil       = errors.New("sysctl: a write guard needs a function")
	errGuardDuplicate = errors.New("sysctl: a write guard of that name is already registered")
)

// RegisterWriteGuard adds a guard under a name, normally the registering
// component's. It returns the function that removes it again, which a test
// calls in its cleanup. A name registered twice is refused, because the second
// registration would silently replace the first guard. Safe for concurrent use.
func RegisterWriteGuard(name string, guard WriteGuard) (func(), error) {
	if name == "" {
		return nil, errGuardNameEmpty
	}
	if guard == nil {
		return nil, errGuardNil
	}
	guardMu.Lock()
	defer guardMu.Unlock()
	if _, exists := guards[name]; exists {
		return nil, errGuardDuplicate
	}
	guards[name] = guard
	return func() {
		guardMu.Lock()
		defer guardMu.Unlock()
		delete(guards, name)
	}, nil
}

// CheckWrite runs every registered guard over one write, in name order so the
// refusal an operator sees is the same on every run, and returns the first
// refusal. Safe for concurrent use.
func CheckWrite(key, value string) error {
	guardMu.RLock()
	names := slices.Sorted(maps.Keys(guards))
	ordered := make([]WriteGuard, 0, len(names))
	for _, name := range names {
		ordered = append(ordered, guards[name])
	}
	guardMu.RUnlock()

	for _, guard := range ordered {
		if err := guard(key, value); err != nil {
			return err
		}
	}
	return nil
}
