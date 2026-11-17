// Design: docs/architecture/api/process-protocol.md -- runtime peer declarations.
package registry

import "sync"

var peerDeclarationsChanged struct {
	mu sync.RWMutex
	fn func() error
}

// SetPeerDeclarationsChanged installs the subsystem listener for runtime
// FilterInsertions, Claims and Dissemination changes after startup. The server
// calls it with config transaction exclusion, without holding its mutex or a
// process-manager lock. The listener MUST NOT start a config transaction. An
// error refuses the replacement whose declarations could not be reconciled.
// Nil removes the listener.
//
// It is a runtime seam, callable after Seal: the owning subsystem installs it
// when it starts, after the registry is sealed.
func SetPeerDeclarationsChanged(fn func() error) {
	peerDeclarationsChanged.mu.Lock()
	defer peerDeclarationsChanged.mu.Unlock()
	peerDeclarationsChanged.fn = fn
}

// NotifyPeerDeclarationsChanged invokes the installed listener outside the
// registry mutex. False means no subsystem owns peer placement in this daemon;
// it is not evidence that any peers were rebuilt.
func NotifyPeerDeclarationsChanged() (bool, error) {
	peerDeclarationsChanged.mu.RLock()
	fn := peerDeclarationsChanged.fn
	peerDeclarationsChanged.mu.RUnlock()
	if fn == nil {
		return false, nil
	}
	return true, fn()
}
