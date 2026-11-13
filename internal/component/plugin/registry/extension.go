// Design: docs/architecture/plugin/plugin-system.md -- the registration seal shared by owner registries.

package registry

import "fmt"

// WithRegistration applies publish under the registry's startup seal. An owner
// registry MUST use it before publishing a declaration, rather than keeping a
// second seal. publish MUST NOT call another registry function: mu is held.
// Safe for concurrent use. A nil publish function is an invalid declaration.
func WithRegistration(kind, name string, publish func() error) error {
	mu.Lock()
	defer mu.Unlock()
	if err := refuseSealedLocked(kind, name); err != nil {
		return err
	}
	if publish == nil {
		return fmt.Errorf("registry: %s %q has no publication function", kind, name)
	}
	return publish()
}
