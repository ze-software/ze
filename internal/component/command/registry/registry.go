// Package registry holds the process-wide registries for ze's top-level root
// commands and owner-backed root command handlers, and the Meta every command
// registration carries. The local handlers and offline fallbacks live in
// package command, because their handler type takes command.ValidatedArgs and
// this package cannot import command.
//
// It is a leaf package -- it imports only the standard library -- so any
// command owner (an internal/component/* package, an internal/plugins/*
// plugin, or a remaining cmd/ze/* package) can import it from init() to
// register a command without risking an import cycle. In particular it must
// never import a concrete command owner, storage, the plugin server, the CLI
// package, or the hub package. Dependencies that owner handlers need at
// dispatch time but that cannot be captured during init() (storage, the
// plugin list, process flags) are passed through RuntimeContext, whose heavy
// types are exposed as function values and primitives precisely so this
// package can stay leaf-like.
//
// Two kinds of root command exist:
//
//   - RegisterRoot registers metadata only. cmd/ze/main.go owns the dispatch
//     for these (process-global commands such as start, help, version).
//   - RegisterRootHandler registers metadata AND a dispatch handler. The
//     registry owns the dispatch, so the owning package can live anywhere
//     without cmd/ze importing it directly. This is how command ownership
//     moves out of the central static switch.
//
// Design: docs/architecture/core-design.md -- ze's registration pattern
package registry

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"
)

// Errors returned by RegisterRootHandler. Exported so callers and tests can
// match them with errors.Is.
var (
	// ErrRootHandlerEmptyName is returned when a root command name is empty.
	ErrRootHandlerEmptyName = errors.New("registry.RegisterRootHandler: empty command name")
	// ErrRootHandlerNilHandler is returned when a nil handler is registered.
	ErrRootHandlerNilHandler = errors.New("registry.RegisterRootHandler: nil handler")
	// ErrRootHandlerDuplicate is returned when a root command name already has
	// a registered handler. Duplicate ownership is a programming bug.
	ErrRootHandlerDuplicate = errors.New("registry.RegisterRootHandler: duplicate root command")
)

// RootHandler runs an owner-backed root command in-process. It receives the
// process RuntimeContext built by cmd/ze/main.go after global flag parsing,
// plus the args that follow the root command name. Handlers that need no
// process dependencies can ignore the context.
type RootHandler func(rctx *RuntimeContext, args []string) int

// RuntimeContext carries the process-entry dependencies that owner root
// handlers need but cannot capture during init(). cmd/ze/main.go assembles it
// after global flag parsing and passes it to the matched root handler.
//
// To keep this registry a leaf package, dependency types that would otherwise
// pull in heavy packages (the plugin server) are exposed through function
// values and primitives, not concrete types. Storage is not carried here: a
// handler that needs the store resolves it from its own arguments through
// internal/core/resolve.
type RuntimeContext struct {
	// Out and ErrOut override process stdout and stderr for an in-process
	// dispatcher. Nil selects the process writers.
	Out    io.Writer
	ErrOut io.Writer
	// Plugins is the --plugin list parsed from global flags, in order.
	Plugins []string
	// ConfigOverride is the -f file override, empty when unset.
	ConfigOverride string
	// PrintVersion prints the process version (extended or short form).
	PrintVersion func(extended bool)
	// WebPort, WebOnly, InsecureWeb, MCPAddr, MCPToken are the web/MCP process
	// flags captured during global flag parsing.
	WebPort     string
	WebOnly     bool
	InsecureWeb bool
	MCPAddr     string
	MCPToken    string
	// ChaosSeed and ChaosRate are the chaos-testing parameters from global flags.
	ChaosSeed int64
	ChaosRate float64
}

const (
	SectionOperations    = "operations"
	SectionConfiguration = "configuration"
	SectionSystem        = "system"
	SectionTest          = "test"
)

var sectionOrder = []string{SectionOperations, SectionConfiguration, SectionSystem, SectionTest}

// SectionTitle returns the display title for a section constant.
func SectionTitle(section string) string {
	return sectionTitles[section]
}

var sectionTitles = map[string]string{
	SectionOperations:    "Operations (interact with the running daemon)",
	SectionConfiguration: "Configuration (change how the box behaves)",
	SectionSystem:        "System (manage the process and environment)",
	SectionTest:          "Test (functional test runners, mock servers, tools)",
}

// Meta holds human-facing metadata for a registered command. Optional; empty
// fields render as blank in help output. Mode is a short tag used by the help
// printer ("offline", "daemon", "setup", "read-only"). Section groups the
// command in help output ("operations", "configuration", "system"). Subs is a
// one-line hint at commonly-used sub-paths. SubsFunc, when non-nil, is called
// instead of reading Subs directly; use it when sub-paths are registered by
// other packages whose init() order is not guaranteed.
//
// ShortHelp is the SUMMARY: one sentence a list row, a completion candidate
// or a table cell can render whole. Description is the explanation the per-command
// help page prints, and it MAY be written over several lines. The two are
// separate declarations for the same reason a YANG command node declares a
// ze:help beside a description: no renderer derives one from the other, so a
// summary is short because its author wrote it short
// (plan/spec-yang-short-and-long-command-help.md, AC-1). An empty Description
// means nobody has written the explanation, never that the command has none.
type Meta struct {
	ShortHelp   string
	Description string
	Mode        string
	Section     string
	Subs        string
	SubsFunc    func() string
}

// ResolveSubs returns the Subs string, calling SubsFunc if set.
func (m Meta) ResolveSubs() string {
	if m.SubsFunc != nil {
		return m.SubsFunc()
	}
	return m.Subs
}

// RootCommand pairs a registered root-command name with its metadata.
type RootCommand struct {
	Name string
	Meta Meta
}

var (
	mu           sync.RWMutex
	rootCommands = make(map[string]Meta)
	rootHandlers = make(map[string]RootHandler)
)

// RegisterRoot registers metadata for a top-level `ze <name>` subcommand whose
// dispatch lives in cmd/ze/main.go. Use this only for process-global commands
// (no narrower owner). Owner-backed commands should use RegisterRootHandler so
// the registry owns dispatch.
func RegisterRoot(name string, meta Meta) {
	mu.Lock()
	rootCommands[name] = meta
	mu.Unlock()
}

// RegisterRootHandler registers an owner-backed root command: its dispatch
// handler AND its help metadata. A root registered here is dispatched by the
// registry (see LookupRoot), so the owning package can live in any
// internal/component, internal/plugins, or cmd/ze package without cmd/ze
// importing it directly.
//
// Returns an error if name is empty, handler is nil, or a handler is already
// registered for name. Duplicate ownership is a programming bug and is
// rejected deterministically.
func RegisterRootHandler(name string, handler RootHandler, meta Meta) error {
	if name == "" {
		return ErrRootHandlerEmptyName
	}
	if handler == nil {
		return fmt.Errorf("%w for %q", ErrRootHandlerNilHandler, name)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, exists := rootHandlers[name]; exists {
		return fmt.Errorf("%w %q", ErrRootHandlerDuplicate, name)
	}
	rootHandlers[name] = handler
	rootCommands[name] = meta
	return nil
}

// MustRegisterRootHandler is the panicking variant, intended for init().
func MustRegisterRootHandler(name string, handler RootHandler, meta Meta) {
	if err := RegisterRootHandler(name, handler, meta); err != nil {
		panic("BUG: registry.MustRegisterRootHandler: " + err.Error())
	}
}

// LookupRoot returns the registered handler for a root command name, or nil if
// no owner registered one. cmd/ze/main.go calls this to dispatch owner-backed
// roots before its legacy static switch.
func LookupRoot(name string) RootHandler {
	mu.RLock()
	defer mu.RUnlock()
	return rootHandlers[name]
}

// ResetForTest clears the root registries. Only intended for use from unit
// tests that want a clean slate between cases. The local handlers are cleared
// by command.ResetLocalForTest.
func ResetForTest() {
	mu.Lock()
	rootCommands = make(map[string]Meta)
	rootHandlers = make(map[string]RootHandler)
	mu.Unlock()
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

// SectionEntry pairs a section title with its commands.
type SectionEntry struct {
	Section  string
	Commands []RootCommand
}

// ListRootBySection returns root commands grouped by section in display order
// (operations, configuration, system). Commands within each section are sorted
// by name.
func ListRootBySection() []SectionEntry {
	mu.RLock()
	defer mu.RUnlock()

	bySection := make(map[string][]RootCommand, len(sectionOrder))
	for name, meta := range rootCommands {
		s := meta.Section
		if s == "" {
			s = SectionSystem
		}
		bySection[s] = append(bySection[s], RootCommand{Name: name, Meta: meta})
	}

	out := make([]SectionEntry, 0, len(sectionOrder))
	for _, s := range sectionOrder {
		cmds := bySection[s]
		if len(cmds) == 0 {
			continue
		}
		sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })
		out = append(out, SectionEntry{Section: s, Commands: cmds})
	}
	return out
}
