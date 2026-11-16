// Design: docs/architecture/iface/logical-name-resolution.md -- exact backend resolution
// Related: backend.go -- backend selection and generation have one lock
// Related: resolve.go -- shared os-name and mac/match selection policy

package iface

import "fmt"

// ResolveForBackend resolves a logical name through the active backend only
// when its registered name equals required. It bypasses the ordinary resolver
// cache: those bindings carry no backend generation. Both selector reads use
// the captured owner, and a replacement or close during the read refuses the
// result without publishing a learned binding. Safe for concurrent use; this
// snapshot does not extend the backend's lifetime after the call returns.
func ResolveForBackend(required, name string) (Binding, error) {
	_, binding, err := ResolveBackend(required, name)
	return binding, err
}

// ResolveBackend returns the captured backend together with the binding it
// produced in the same generation. A caller needing backend-specific readback
// MUST use this owner rather than separately reading the active backend.
// Neither result extends that owner's lifetime after this call returns.
// Safe for concurrent use.
func ResolveBackend(required, name string) (Backend, Binding, error) {
	backendsMu.Lock()
	backend, err := requireActiveBackendLocked(required)
	generation := activeBackendGeneration
	backendsMu.Unlock()
	if err != nil {
		return nil, Binding{}, err
	}
	info, err := globalResolver.selectedDeviceFor(name, backend.GetLink, backend.ListInterfaces)
	if err != nil {
		return nil, Binding{}, err
	}
	if info == nil {
		return nil, Binding{}, fmt.Errorf("iface: backend %q returned no link readback for %q", required, name)
	}
	backendsMu.Lock()
	defer backendsMu.Unlock()
	if activeBackendGeneration != generation {
		return nil, Binding{}, fmt.Errorf("iface: backend %q changed while resolving %q; retry after interface reload completes", required, name)
	}
	globalResolver.recordBinding(name, info.OsName)
	return backend, bindingFromInfo(info.OsName, info), nil
}
