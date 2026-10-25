// Design: docs/contributing/running-commands.md -- le's hidden commands
// Related: manifest.go -- the root page that leaves a hidden command out
// Related: tree.go -- the tree completion and the menu read, which leaves it out
// Related: helpai.go -- `le help ai commands`, the one listing that names it
//
// A hidden command is one Ze starts itself and a person never types: a plugin a
// test daemon spawns from its config (`le mock engine-steps`). It dispatches
// like any other command, and only the listings a person reads leave it out.
// `le help ai commands` still lists it, marked hidden, so every word le
// accepts is in le's help.

package leroot

import "sync"

// hidden holds the name of every hidden command. Registration runs in init()
// and every read happens after it, but help runs on any goroutine, so the set
// is guarded. Safe for concurrent use.
var hidden struct {
	sync.RWMutex
	names map[string]bool
}

// RegisterHidden marks a command as one help pages and completion leave out.
// MUST be called from the init() that registers the command (Register).
func RegisterHidden(name string) {
	if name == "" {
		panic("BUG: leroot.RegisterHidden: an empty command name")
	}
	hidden.Lock()
	defer hidden.Unlock()
	if hidden.names == nil {
		hidden.names = make(map[string]bool, 4)
	}
	hidden.names[name] = true
}

// Hidden reports whether a command was registered hidden (RegisterHidden).
func Hidden(name string) bool {
	hidden.RLock()
	defer hidden.RUnlock()
	return hidden.names[name]
}
