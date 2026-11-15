// Design: docs/architecture/plugin/plugin-system.md -- isolated registration values and test restoration.
// Related: registry.go -- Lookup and Snapshot retain the original registration.

package registry

import (
	"fmt"
	"net"
)

// WrapRunnerForTest wraps the connection passed to the named plugin's real
// engine without changing its declaration, setup or runtime state.
// Only for use in tests: a transport fixture can hide an optional connection
// interface while retaining the real engine's protocol and callbacks.
//
// The caller MUST install the wrapper before starting its runner, MUST stop
// and join every runner using it before restoring, and MUST restore exactly
// once. The returned restoration MUST run after those owners stop. Nested
// wrappers MUST restore in reverse order. Restoration refuses to overwrite a
// later declaration change. Safe for concurrent registry readers.
func WrapRunnerForTest(name string, wrap func(net.Conn) net.Conn) (func() error, error) {
	if wrap == nil {
		return nil, fmt.Errorf("registry: test runner for %q has no connection wrapper", name)
	}
	mu.Lock()
	defer mu.Unlock()
	original := plugins[name]
	if original == nil {
		return nil, fmt.Errorf("registry: test runner %q is not registered", name)
	}
	replacement := cloneRegistration(original)
	replacement.RunEngine = func(conn net.Conn) int {
		return original.RunEngine(wrap(conn))
	}
	plugins[name] = replacement
	rebuildFamilyIndexLocked()
	restored := false
	return func() error {
		mu.Lock()
		defer mu.Unlock()
		if restored {
			return fmt.Errorf("registry: test runner %q was already restored", name)
		}
		if plugins[name] != replacement {
			return fmt.Errorf("registry: test runner %q no longer owns its registration", name)
		}
		plugins[name] = original
		rebuildFamilyIndexLocked()
		restored = true
		return nil
	}, nil
}
