// Package cmdregistry holds the process-wide registries for ze's
// command-line surface: offline local handlers and top-level root
// commands. It is a leaf package -- no dependencies on anything else
// under cmd/ze -- so every subcommand package can import it from
// `init()` without risking an import cycle with cmdutil (which imports
// cli, which cannot import back into cmdutil).
//
// Design: docs/architecture/core-design.md -- ze's registration pattern
package cmdregistry

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// LocalHandler runs a CLI command in-process (no daemon required).
type LocalHandler func(args []string) int

// Meta holds human-facing metadata for a registered command. Optional;
// empty fields render as blank in help output. Mode is a short tag
// used by the help printer ("offline", "daemon", "setup", "read-only").
// Subs is a one-line hint at commonly-used sub-paths.
type Meta struct {
	Description string
	Mode        string
	Subs        string
}

// LocalCommandEntry pairs a registered local-command path with its
// metadata.
type LocalCommandEntry struct {
	Path string
	Meta Meta
}

// RootCommand pairs a registered root-command name with its metadata.
type RootCommand struct {
	Name string
	Meta Meta
}

var (
	mu            sync.RWMutex
	localHandlers = make(map[string]LocalHandler)
	localMeta     = make(map[string]Meta)
	rootCommands  = make(map[string]Meta)
)

// RegisterLocal registers a handler for a CLI command path (for
// example, "show version" or "ping"). The path is the full
// space-separated command. Called at startup before dispatch.
func RegisterLocal(path string, handler LocalHandler) error {
	if path == "" {
		return fmt.Errorf("cmdregistry.RegisterLocal: empty path")
	}
	if handler == nil {
		return fmt.Errorf("cmdregistry.RegisterLocal: nil handler for %q", path)
	}
	mu.Lock()
	localHandlers[path] = handler
	mu.Unlock()
	return nil
}

// RegisterLocalMeta registers a handler AND its human-facing metadata.
// Metadata is surfaced by `ze help --ai`.
func RegisterLocalMeta(path string, handler LocalHandler, meta Meta) error {
	if err := RegisterLocal(path, handler); err != nil {
		return err
	}
	mu.Lock()
	localMeta[path] = meta
	mu.Unlock()
	return nil
}

// MustRegisterLocal is the panicking variant, intended for init().
func MustRegisterLocal(path string, handler LocalHandler) {
	if err := RegisterLocal(path, handler); err != nil {
		panic("BUG: cmdregistry.MustRegisterLocal: " + err.Error())
	}
}

// MustRegisterLocalMeta is the panicking variant, intended for init().
func MustRegisterLocalMeta(path string, handler LocalHandler, meta Meta) {
	if err := RegisterLocalMeta(path, handler, meta); err != nil {
		panic("BUG: cmdregistry.MustRegisterLocalMeta: " + err.Error())
	}
}

// RegisterRoot registers metadata for a top-level `ze <name>`
// subcommand. Dispatch itself lives in cmd/ze/main.go; this registry
// drives the help printer so root commands do not need a hand-
// maintained static list.
func RegisterRoot(name string, meta Meta) {
	mu.Lock()
	rootCommands[name] = meta
	mu.Unlock()
}

// LookupLocal finds the longest prefix of words that matches a
// registered local handler. Returns the handler and the remaining
// words as args. Returns nil handler if no match.
//
// Caller joins words with spaces to form the match key; iteration
// tries longest first, so "show bgp decode" is preferred over
// "show bgp" or "show".
func LookupLocal(words []string) (LocalHandler, []string) {
	mu.RLock()
	defer mu.RUnlock()
	for i := len(words); i > 0; i-- {
		path := strings.Join(words[:i], " ")
		if handler, ok := localHandlers[path]; ok {
			return handler, append([]string(nil), words[i:]...)
		}
	}
	return nil, nil
}

// ListLocal returns every registered local command sorted by path.
// Handlers are not returned; only path + metadata.
func ListLocal() []LocalCommandEntry {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]LocalCommandEntry, 0, len(localHandlers))
	for path := range localHandlers {
		out = append(out, LocalCommandEntry{Path: path, Meta: localMeta[path]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// ResetForTest clears every registry. Only intended for use from unit
// tests that want a clean slate between cases.
func ResetForTest() {
	mu.Lock()
	localHandlers = make(map[string]LocalHandler)
	localMeta = make(map[string]Meta)
	rootCommands = make(map[string]Meta)
	mu.Unlock()
}

// HasLocal reports whether a handler is registered for the exact path.
// Only intended for tests that need an existence check without pulling
// a handler.
func HasLocal(path string) bool {
	mu.RLock()
	_, ok := localHandlers[path]
	mu.RUnlock()
	return ok
}

// ListRoot returns every registered root command sorted by name.
func ListRoot() []RootCommand {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]RootCommand, 0, len(rootCommands))
	for name, meta := range rootCommands {
		out = append(out, RootCommand{Name: name, Meta: meta})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
