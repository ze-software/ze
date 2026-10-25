// Design: docs/contributing/running-commands.md -- installing le's completion
// Related: complete.go -- the candidates the callback prints
//
// A shell callback is an area a shell runs on a keystroke. The le root
// handler skips the cache-link repair, the patched-GOROOT selection and the
// before-dispatch hooks for it: each can print, wait or start work, and a
// keystroke owes nothing but its candidates on stdout.

package leroot

import "sync"

// shellCallbacks holds the areas declared as shell callbacks. Registration
// runs in init() and every read after it, but a test may read on any
// goroutine, so the set is guarded. Safe for concurrent use.
var shellCallbacks struct {
	sync.RWMutex
	byName map[string]bool
}

// RegisterShellCallback declares that area as a shell callback. Call it from
// the area's init(), beside its Register.
func RegisterShellCallback(area string) {
	shellCallbacks.Lock()
	defer shellCallbacks.Unlock()
	if shellCallbacks.byName == nil {
		shellCallbacks.byName = make(map[string]bool, 1)
	}
	shellCallbacks.byName[area] = true
}

// ShellCallback reports whether argv starts with an area declared as a shell
// callback, so the root handler MUST run nothing before dispatching it.
func ShellCallback(args []string) bool {
	if len(args) == 0 {
		return false
	}
	shellCallbacks.RLock()
	defer shellCallbacks.RUnlock()
	return shellCallbacks.byName[args[0]]
}
