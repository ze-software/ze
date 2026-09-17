// Design: docs/guide/config-editor.md — daemon-owned configuration publication
package registry

import (
	"errors"
	"sync"
)

var configCommit struct {
	sync.RWMutex
	fn func(string, []byte, []byte) error
}

// SetRuntimeConfigCommit installs the owning daemon's configuration publisher.
// The owner MUST clear it during shutdown before closing its store.
func SetRuntimeConfigCommit(fn func(path string, expected, content []byte) error) {
	configCommit.Lock()
	configCommit.fn = fn
	configCommit.Unlock()
}

// RuntimeConfigCommit publishes through the daemon's source-authority transaction.
// expected is the config the editor read; a competing edit MUST be refused.
func RuntimeConfigCommit(path string, expected, content []byte) error {
	configCommit.RLock()
	fn := configCommit.fn
	configCommit.RUnlock()
	if fn == nil {
		return errors.New("configuration publication unavailable: no owning daemon")
	}
	return fn(path, expected, content)
}
