// Design: docs/architecture/api/process-protocol.md -- initial runtime grants.
// Related: register.go -- the reactor's post-startup callback owns this guard.
package plugin

import "sync"

// initialPeerConfig belongs to one reactor lifetime. Its zero value owes the
// initial rebuild; only success completes it. Safe for concurrent callbacks.
// Subsequent reloads reconcile their candidate through ApplyConfigDiff, rather
// than applying new declarations to the old running map during auto-load.
type initialPeerConfig struct {
	mu       sync.Mutex
	complete bool
}

func (initial *initialPeerConfig) resolve(rebuild func() error) error {
	initial.mu.Lock()
	defer initial.mu.Unlock()
	if initial.complete {
		return nil
	}
	if err := rebuild(); err != nil {
		return err
	}
	initial.complete = true
	return nil
}
