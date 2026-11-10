// Design: docs/architecture/api/process-protocol.md -- Filter Insertion
// Related: filter_declaration.go -- RuntimeFilterInsertions, the set whose change this announces

package registry

import "sync"

// insertionsChanged is the listener the subsystem that places filter
// insertions installs (SetFilterInsertionsChanged).
var insertionsChanged struct {
	mu sync.RWMutex
	fn func()
}

// SetFilterInsertionsChanged installs the listener told when the running
// set of declared filter insertions changed after startup: a third-party
// plugin respawned with a different Stage 1 declaration, or a plugin that
// declared insertions stopped. The subsystem that places insertions (the BGP
// config loader) rebuilds its peers from the running config in it, so the
// lists follow the declarations without a reload. Called once from that
// subsystem's startup; nil removes it. The plugin server calls it holding
// its config transaction lock, so the rebuild never interleaves with a
// reload: the listener MUST NOT start a config transaction, which that lock
// would refuse.
//
// It is a runtime seam, callable after Seal: the BGP engine installs it when
// it starts, after the registry is sealed.
func SetFilterInsertionsChanged(fn func()) {
	insertionsChanged.mu.Lock()
	defer insertionsChanged.mu.Unlock()
	insertionsChanged.fn = fn
}

// NotifyFilterInsertionsChanged tells the installed listener that the
// declared insertions changed. It answers false when no listener is
// installed: no subsystem places insertions in this daemon, so there is
// nothing to rebuild, and the caller MUST NOT read false as a rebuild.
func NotifyFilterInsertionsChanged() bool {
	insertionsChanged.mu.RLock()
	fn := insertionsChanged.fn
	insertionsChanged.mu.RUnlock()
	if fn == nil {
		return false
	}
	fn()
	return true
}
