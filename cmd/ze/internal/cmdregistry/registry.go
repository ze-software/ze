// Package cmdregistry is a thin compatibility shim over
// internal/component/command/registry, the importable leaf package that now
// owns ze's command-line registries.
//
// The registry moved out of cmd/ze/internal so that command owners under
// internal/component and internal/plugins can register their own commands and
// root handlers from init() without importing anything under cmd/ze. This shim
// re-exports the registry's API at the old import path so existing cmd/ze
// callers keep compiling during the ownership migration. New code -- and any
// migrated owner -- should import
// codeberg.org/thomas-mangin/ze/internal/component/command/registry directly.
//
// The shim holds no state of its own; every function delegates to the leaf
// package, so the shim and direct importers share one registry.
//
// Design: docs/architecture/core-design.md -- ze's registration pattern
package cmdregistry

import "codeberg.org/thomas-mangin/ze/internal/component/command/registry"

// Re-exported types. Aliases keep callers' `cmdregistry.Meta` etc. assignable
// to and from the leaf package's types.
type (
	// LocalHandler runs a CLI command in-process (no daemon required).
	LocalHandler = registry.LocalHandler
	// RootHandler runs an owner-backed root command in-process.
	RootHandler = registry.RootHandler
	// RuntimeContext carries process-entry dependencies for owner handlers.
	RuntimeContext = registry.RuntimeContext
	// Meta holds human-facing metadata for a registered command.
	Meta = registry.Meta
	// LocalCommandEntry pairs a registered local-command path with its metadata.
	LocalCommandEntry = registry.LocalCommandEntry
	// RootCommand pairs a registered root-command name with its metadata.
	RootCommand = registry.RootCommand
	// SectionEntry pairs a section title with its commands.
	SectionEntry = registry.SectionEntry
)

// Re-exported section constants.
const (
	SectionOperations    = registry.SectionOperations
	SectionConfiguration = registry.SectionConfiguration
	SectionSystem        = registry.SectionSystem
)

// SectionTitle returns the display title for a section constant.
func SectionTitle(section string) string { return registry.SectionTitle(section) }

// RegisterLocal registers a handler for a CLI command path.
func RegisterLocal(path string, handler LocalHandler) error {
	return registry.RegisterLocal(path, handler)
}

// RegisterLocalMeta registers a handler AND its human-facing metadata.
func RegisterLocalMeta(path string, handler LocalHandler, meta Meta) error {
	return registry.RegisterLocalMeta(path, handler, meta)
}

// MustRegisterLocal is the panicking variant, intended for init().
func MustRegisterLocal(path string, handler LocalHandler) {
	registry.MustRegisterLocal(path, handler)
}

// MustRegisterLocalMeta is the panicking variant, intended for init().
func MustRegisterLocalMeta(path string, handler LocalHandler, meta Meta) {
	registry.MustRegisterLocalMeta(path, handler, meta)
}

// RegisterRoot registers metadata for a `ze <name>` subcommand dispatched by
// cmd/ze/main.go.
func RegisterRoot(name string, meta Meta) { registry.RegisterRoot(name, meta) }

// RegisterRootHandler registers an owner-backed root command (handler + meta).
func RegisterRootHandler(name string, handler RootHandler, meta Meta) error {
	return registry.RegisterRootHandler(name, handler, meta)
}

// MustRegisterRootHandler is the panicking variant, intended for init().
func MustRegisterRootHandler(name string, handler RootHandler, meta Meta) {
	registry.MustRegisterRootHandler(name, handler, meta)
}

// LookupRoot returns the registered handler for a root command name, or nil.
func LookupRoot(name string) RootHandler { return registry.LookupRoot(name) }

// SetRuntimeStorage installs the process storage resolver for local handlers.
func SetRuntimeStorage(fn func() any) { registry.SetRuntimeStorage(fn) }

// HasRootHandler reports whether an owner-backed handler exists for name.
func HasRootHandler(name string) bool { return registry.HasRootHandler(name) }

// LookupLocal finds the longest prefix of words matching a local handler.
func LookupLocal(words []string) (LocalHandler, []string) { return registry.LookupLocal(words) }

// ListLocal returns every registered local command sorted by path.
func ListLocal() []LocalCommandEntry { return registry.ListLocal() }

// ResetForTest clears every registry. Intended for unit tests only.
func ResetForTest() { registry.ResetForTest() }

// HasLocal reports whether a handler is registered for the exact path.
func HasLocal(path string) bool { return registry.HasLocal(path) }

// ListRoot returns every registered root command sorted by name.
func ListRoot() []RootCommand { return registry.ListRoot() }

// ListRootBySection returns root commands grouped by section in display order.
func ListRootBySection() []SectionEntry { return registry.ListRootBySection() }
