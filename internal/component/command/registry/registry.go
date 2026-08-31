// Package registry holds the process-wide registries for ze's command-line
// surface: offline local handlers, top-level root commands, and owner-backed
// root command handlers.
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
	"os"
	"sort"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var errRegisterLocalEmptyPath = errors.New("registry.RegisterLocal: empty path")

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

// LocalHandler runs a CLI command in-process (no daemon required).
type LocalHandler func(args []string) int

// LocalDataHandler answers a command with structured DATA instead of printing
// text, so the answer can go through the pipe layer like any other.
//
// A LocalHandler prints and returns an exit code, which is why 38 commands
// reached no pipe layer on any surface: by the time RunCommand had a result
// there was nothing left but an int, and `ze cli -c "show env list | json"`
// answered `unknown command` because the daemon serves no such method. A data
// handler returns the payload, and the caller renders it.
//
// The value MUST be structured data a JSON encoder can take: a map, a slice, or
// a struct. It MUST NOT be text a renderer already formatted, for the reason
// ai/rules/cli.md gives for every other handler: `| json`, `| yaml` and
// `| table` are three renderings of ONE payload.
//
// The two results are INDEPENDENT. The payload says whether there is an answer
// to render, the code says what the process exits with, and a command MAY have
// both: `validate config` answers the diagnostics of a config it rejects and
// exits 1. A handler with nothing to say MUST write its reason to stderr and
// return a nil payload.
type LocalDataHandler func(args []string) (any, int)

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
// pull in heavy packages (storage, the plugin server) are exposed through
// function values and primitives, not concrete types. Owners type-assert the
// storage value to the concrete storage.Storage interface (see StorageAs).
type RuntimeContext struct {
	// ResolveStorage opens the process storage backend lazily and returns it
	// as an opaque value the owner type-asserts to storage.Storage. Nil when
	// the process did not wire storage (some test harnesses).
	ResolveStorage func() any
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

// StorageAs type-asserts ResolveStorage's result to the requested type T,
// returning the zero value and false when storage is unavailable or of a
// different type. It is a convenience for owner handlers so they do not repeat
// the nil check and type assertion.
func StorageAs[T any](rctx *RuntimeContext) (T, bool) {
	var zero T
	if rctx == nil || rctx.ResolveStorage == nil {
		return zero, false
	}
	v, ok := rctx.ResolveStorage().(T)
	return v, ok
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
// Description is the SUMMARY: one sentence a list row, a completion candidate
// or a table cell can render whole. LongHelp is the explanation the per-command
// help page prints, and it MAY be written over several lines. The two are
// separate declarations for the same reason a YANG command node declares a
// description beside a ze:help: no renderer derives one from the other, so a
// summary is short because its author wrote it short
// (plan/spec-yang-short-and-long-command-help.md, AC-1). An empty LongHelp
// means nobody has written the explanation, never that the command has none.
type Meta struct {
	Description string
	LongHelp    string
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

// LocalCommandEntry pairs a registered local-command path with its metadata.
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
	mu               sync.RWMutex
	localHandlers    = make(map[string]LocalHandler)
	localMeta        = make(map[string]Meta)
	rootCommands     = make(map[string]Meta)
	rootHandlers     = make(map[string]RootHandler)
	offlineFallbacks = make(map[string]LocalHandler)
)

// RegisterLocal registers a handler for a CLI command path (for example,
// "show version" or "ping"). The path is the full space-separated command.
// Called at startup before dispatch.
func RegisterLocal(path string, handler LocalHandler) error {
	if path == "" {
		return errRegisterLocalEmptyPath
	}
	if handler == nil {
		return fmt.Errorf("registry.RegisterLocal: nil handler for %q", path)
	}
	mu.Lock()
	localHandlers[path] = handler
	mu.Unlock()
	return nil
}

// RegisterLocalMeta registers a handler AND its human-facing metadata.
// Metadata is surfaced by `ze help ai`.
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
		panic("BUG: registry.MustRegisterLocal: " + err.Error())
	}
}

// MustRegisterLocalMeta is the panicking variant, intended for init().
func MustRegisterLocalMeta(path string, handler LocalHandler, meta Meta) {
	if err := RegisterLocalMeta(path, handler, meta); err != nil {
		panic("BUG: registry.MustRegisterLocalMeta: " + err.Error())
	}
}

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

// Runtime storage resolver for local command handlers.
//
// Local handlers have the signature func(args []string) int and so do not
// receive a RuntimeContext, yet a few owner shortcuts (for example
// `show config history`) still need the process storage backend at dispatch
// time. cmd/ze/main.go installs the resolver once after global flag parsing;
// handlers read it lazily, so registration order does not matter. The value is
// exposed as any to keep this package leaf-like (it must not import storage);
// callers type-assert to storage.Storage.
var (
	runtimeStorageMu sync.RWMutex
	runtimeStorage   func() any
)

// SetRuntimeStorage installs the process storage resolver. Called once by
// cmd/ze/main.go.
func SetRuntimeStorage(fn func() any) {
	runtimeStorageMu.Lock()
	runtimeStorage = fn
	runtimeStorageMu.Unlock()
}

// RuntimeStorage resolves the process storage backend, or nil if none was
// installed. Each call opens a fresh backend that the caller must close.
func RuntimeStorage() any {
	runtimeStorageMu.RLock()
	fn := runtimeStorage
	runtimeStorageMu.RUnlock()
	if fn == nil {
		return nil
	}
	return fn()
}

// LookupLocal finds the longest prefix of words that matches a registered
// local handler. Returns the handler and the remaining words as args. Returns
// nil handler if no match.
//
// Caller joins words with spaces to form the match key; iteration tries
// longest first, so "show bgp decode" is preferred over "show bgp" or "show".
//
// THE MATCH IS REFUSED WHEN THE ARGV REACHES A DECLARED COMMAND FURTHER DOWN.
// Longest-prefix alone gives a handler registered at a SHORT path the whole
// subtree below it, including paths another owner declared as commands of their
// own. `show interface` is registered locally
// (internal/component/iface/cli/register.go) and declares seven children in
// ze-iface-interface-cmd.yang; every one of them landed on that handler, which
// reads its first argument as an interface NAME, so `ze show interface brief`
// looked for an interface called "brief". A handler still keeps every trailing
// word that names no declared command, which is how `ze show interface eth0`
// and `ze show debug profile name default` reach theirs.
//
// declared answers whether an absolute path is a registered ze:command; pass
// cli.IsDeclaredCommand. A nil declared makes every match unprovable, so none is
// served: this is a dispatch guard, and a guard with no data must fail closed
// rather than return the shadowing match it cannot judge (ai/rules/evidence.md).
//
// LookupOfflineFallback keeps plain longest-prefix on purpose. A fallback is
// consulted only after the daemon is unreachable, so covering a declared child
// is the point rather than a collision: `show host` serves `show host cpu` with
// no daemon running.
func LookupLocal(words []string, declared func(path string) bool) (LocalHandler, []string) {
	if declared == nil {
		return nil, nil
	}
	handler, matched := longestLocalPrefix(words)
	if handler == nil {
		return nil, nil
	}
	// Evaluated outside the registry lock: declared is a foreign callback that
	// reads the RPC registry, and holding one registry's lock across another's
	// is how a lock order gets invented by accident.
	for i := matched + 1; i <= len(words); i++ {
		if declared(textbuf.Join(words[:i], " ")) {
			return nil, nil
		}
	}
	return handler, append([]string(nil), words[matched:]...)
}

// longestLocalPrefix returns the handler registered at the longest prefix of
// words, and how many words that prefix consumed. matched is 0 when no prefix
// is registered, and the handler is then nil.
func longestLocalPrefix(words []string) (LocalHandler, int) {
	mu.RLock()
	defer mu.RUnlock()
	for i := len(words); i > 0; i-- {
		if handler, ok := localHandlers[textbuf.Join(words[:i], " ")]; ok {
			return handler, i
		}
	}
	return nil, 0
}

// localDataHandlers holds the commands that answer with data in this process.
var localDataHandlers = make(map[string]LocalDataHandler)

// RegisterLocalData registers a command that answers with structured data in
// this process, so its answer reaches the pipe layer.
//
// It ALSO registers a plain local handler, built from the same data handler, so
// `ze <verb>` prints exactly what it printed before and the two forms of one
// command cannot drift apart. That drift is real: `ze show interface` took the
// local path and `ze cli -c "show interface"` took the daemon's, and only the
// second honored a pipe.
func RegisterLocalData(path string, handler LocalDataHandler, meta Meta, render func(string, any) int) error {
	if path == "" {
		return errRegisterLocalEmptyPath
	}
	if handler == nil {
		return fmt.Errorf("registry.RegisterLocalData: nil handler for %q", path)
	}
	if render == nil {
		return fmt.Errorf("registry.RegisterLocalData: nil renderer for %q", path)
	}
	if err := RegisterLocalMeta(path, func(args []string) int {
		// A nonzero code with a payload is an ANSWER the command exits
		// nonzero on, not an error with nothing to say: `validate config`
		// renders the diagnostics of a config it rejects and exits 1. The
		// renderer's own failure wins, because then nothing was printed.
		payload, code := handler(args)
		if payload == nil {
			return code
		}
		if renderCode := render(path, payload); renderCode != 0 {
			return renderCode
		}
		return code
	}, meta); err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	localDataHandlers[path] = handler
	return nil
}

// MustRegisterLocalData is RegisterLocalData, and panics rather than letting a
// command register half.
func MustRegisterLocalData(path string, handler LocalDataHandler, meta Meta, render func(string, any) int) {
	err := RegisterLocalData(path, handler, meta, render)
	if err == nil {
		return
	}
	// The detail goes to stderr and the panic value is a literal. A registration
	// failure is a programming error at init, so the process must stop, and the
	// error already names the path that could not register.
	fmt.Fprintln(os.Stderr, "BUG: registry.MustRegisterLocalData:", err)
	panic("BUG: registry.MustRegisterLocalData")
}

// LookupLocalData answers the data handler for a command path, by longest
// registered prefix, with the words that follow it as its arguments.
func LookupLocalData(words []string) (LocalDataHandler, []string) {
	mu.RLock()
	defer mu.RUnlock()
	for i := len(words); i > 0; i-- {
		if handler, ok := localDataHandlers[textbuf.Join(words[:i], " ")]; ok {
			return handler, words[i:]
		}
	}
	return nil, nil
}

// ResetLocalDataForTest clears every registered data handler.
func ResetLocalDataForTest() {
	mu.Lock()
	defer mu.Unlock()
	localDataHandlers = make(map[string]LocalDataHandler)
}

// RegisterOfflineFallback registers an in-process handler for a read-only
// command path (for example "show crashes" or "show host") that is served ONLY
// when the daemon is unreachable. Unlike RegisterLocal, a fallback is never
// consulted while the daemon is up, so it does not shadow the daemon command:
// the CLI tries the daemon first and calls the fallback only after a
// connection-level failure. Intended for host-local read-only data (crash
// files, hardware inventory) an operator must still be able to read with no
// daemon running.
func RegisterOfflineFallback(path string, handler LocalHandler) error {
	if path == "" {
		return errRegisterLocalEmptyPath
	}
	if handler == nil {
		return fmt.Errorf("registry.RegisterOfflineFallback: nil handler for %q", path)
	}
	mu.Lock()
	offlineFallbacks[path] = handler
	mu.Unlock()
	return nil
}

// MustRegisterOfflineFallback is the panicking variant, intended for init().
// The path and handler are fixed at each call site, so a failure is a
// programming bug; the offending call site is evident from the panic stack.
func MustRegisterOfflineFallback(path string, handler LocalHandler) {
	if err := RegisterOfflineFallback(path, handler); err != nil {
		_ = err
		panic("BUG: registry.MustRegisterOfflineFallback: empty path or nil handler")
	}
}

// LookupOfflineFallback finds the longest prefix of words matching a registered
// offline fallback handler, returning the handler and remaining words as args.
// Returns a nil handler if no fallback is registered. Same longest-prefix
// semantics as LookupLocal, but a separate registry so fallbacks are only
// reachable through the daemon-unreachable path.
func LookupOfflineFallback(words []string) (LocalHandler, []string) {
	mu.RLock()
	defer mu.RUnlock()
	for i := len(words); i > 0; i-- {
		path := textbuf.Join(words[:i], " ")
		if handler, ok := offlineFallbacks[path]; ok {
			return handler, append([]string(nil), words[i:]...)
		}
	}
	return nil, nil
}

// ListLocal returns every registered local command sorted by path. Handlers
// are not returned; only path + metadata.
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

// ResetForTest clears every registry. Only intended for use from unit tests
// that want a clean slate between cases.
func ResetForTest() {
	mu.Lock()
	localHandlers = make(map[string]LocalHandler)
	localMeta = make(map[string]Meta)
	rootCommands = make(map[string]Meta)
	rootHandlers = make(map[string]RootHandler)
	offlineFallbacks = make(map[string]LocalHandler)
	mu.Unlock()
	runtimeStorageMu.Lock()
	runtimeStorage = nil
	runtimeStorageMu.Unlock()
}

// HasLocal reports whether a handler is registered for the exact path. Only
// intended for tests that need an existence check without pulling a handler.
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
