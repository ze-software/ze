// Design: docs/architecture/api/commands.md — where a command is served
// Related: local_data.go — the data handlers whose `ze <verb>` form registers here
//
// local.go holds the commands that run in THIS process and print their own
// answer: the `ze <verb>` local handlers and the offline fallbacks the CLI
// serves when the daemon is unreachable.

package command

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// LocalHandler runs a CLI command in-process (no daemon required) and answers
// the process exit code.
//
// It takes the value ValidateArgs returned, never a token slice: only a
// successful judgment of the arguments against the leaves the command's YANG
// declares builds one, so no route can run a local handler on tokens nobody
// judged. Every route that runs one calls InvokeLocal, which judges first.
type LocalHandler func(args ValidatedArgs) int

// LocalCommandEntry pairs a registered local-command path with its metadata.
type LocalCommandEntry struct {
	Path string
	Meta registry.Meta
}

// local holds the local handlers, their metadata and the offline fallbacks. It
// lives here rather than in the registry package because its handler type
// names ValidatedArgs, which only this package can build, and the registry
// cannot import this package. Safe for concurrent use.
var local = struct {
	sync.RWMutex
	handlers  map[string]LocalHandler
	meta      map[string]registry.Meta
	fallbacks map[string]LocalHandler
}{
	handlers:  make(map[string]LocalHandler),
	meta:      make(map[string]registry.Meta),
	fallbacks: make(map[string]LocalHandler),
}

// errRegisterLocalEmptyPath is the refusal of a local handler or offline
// fallback registered with no command path.
var errRegisterLocalEmptyPath = errors.New("command.RegisterLocal: empty path")

// RegisterLocal registers a handler for a CLI command path (for example,
// "show version" or "ping"). The path is the full space-separated command.
// Called at startup before dispatch.
func RegisterLocal(path string, handler LocalHandler) error {
	if path == "" {
		return errRegisterLocalEmptyPath
	}
	if handler == nil {
		return fmt.Errorf("command.RegisterLocal: nil handler for %q", path)
	}
	local.Lock()
	local.handlers[path] = handler
	local.Unlock()
	return nil
}

// RegisterLocalMeta registers a handler AND its human-facing metadata.
// Metadata is surfaced by `ze help ai`.
func RegisterLocalMeta(path string, handler LocalHandler, meta registry.Meta) error {
	if err := RegisterLocal(path, handler); err != nil {
		return err
	}
	local.Lock()
	local.meta[path] = meta
	local.Unlock()
	return nil
}

// MustRegisterLocal is the panicking variant, intended for init().
func MustRegisterLocal(path string, handler LocalHandler) {
	if err := RegisterLocal(path, handler); err != nil {
		panic("BUG: command.MustRegisterLocal: " + err.Error())
	}
}

// MustRegisterLocalMeta is the panicking variant, intended for init().
func MustRegisterLocalMeta(path string, handler LocalHandler, meta registry.Meta) {
	if err := RegisterLocalMeta(path, handler, meta); err != nil {
		panic("BUG: command.MustRegisterLocalMeta: " + err.Error())
	}
}

// LookupLocal finds the longest prefix of words that matches a registered
// local handler. Returns the handler and the remaining words as args. Returns
// nil handler if no match. The caller MUST run the handler through
// InvokeLocal, with the registered path, so the remaining words are judged.
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
// For the same reason an error from declared refuses the match, and is returned
// so the caller reports why no handler was served.
//
// LookupOfflineFallback keeps plain longest-prefix on purpose. A fallback is
// consulted only after the daemon is unreachable, so covering a declared child
// is the point rather than a collision: `show host` serves `show host cpu` with
// no daemon running.
func LookupLocal(words []string, declared func(path string) (bool, error)) (LocalHandler, []string, error) {
	if declared == nil {
		return nil, nil, nil
	}
	handler, matched := longestLocalPrefix(words)
	if handler == nil {
		return nil, nil, nil
	}
	// Evaluated outside the registry lock: declared is a foreign callback that
	// reads the RPC registry, and holding one registry's lock across another's
	// is how a lock order gets invented by accident.
	for i := matched + 1; i <= len(words); i++ {
		isDeclared, err := declared(textbuf.Join(words[:i], " "))
		if err != nil {
			return nil, nil, fmt.Errorf("local command %q: %w", textbuf.Join(words[:matched], " "), err)
		}
		if isDeclared {
			return nil, nil, nil
		}
	}
	return handler, append([]string(nil), words[matched:]...), nil
}

// longestLocalPrefix returns the handler registered at the longest prefix of
// words, and how many words that prefix consumed. matched is 0 when no prefix
// is registered, and the handler is then nil.
func longestLocalPrefix(words []string) (LocalHandler, int) {
	local.RLock()
	defer local.RUnlock()
	for i := len(words); i > 0; i-- {
		if handler, ok := local.handlers[textbuf.Join(words[:i], " ")]; ok {
			return handler, i
		}
	}
	return nil, 0
}

// InvokeLocal judges args against the leaves the model declares for the
// registered path and runs handler on the value that judgment returned. A
// refusal is written to stderr and exits 1 without reaching the handler. Every
// route that runs a handler LookupLocal or LookupOfflineFallback answered MUST
// call this with the path it matched and every token it will pass: the
// `ze <verb>` routes (R6) and the daemon-down fallback (R7).
func InvokeLocal(path string, handler LocalHandler, args []string) int {
	validated, err := ValidateModelArgs(path, args, nil)
	if err != nil {
		writeLocalRefusal(err)
		return 1
	}
	return handler(validated)
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
		return fmt.Errorf("command.RegisterOfflineFallback: nil handler for %q", path)
	}
	local.Lock()
	local.fallbacks[path] = handler
	local.Unlock()
	return nil
}

// MustRegisterOfflineFallback is the panicking variant, intended for init().
// The path and handler are fixed at each call site, so a failure is a
// programming bug; the offending call site is evident from the panic stack.
func MustRegisterOfflineFallback(path string, handler LocalHandler) {
	if err := RegisterOfflineFallback(path, handler); err != nil {
		panic("BUG: command.MustRegisterOfflineFallback: " + err.Error())
	}
}

// LookupOfflineFallback finds the longest prefix of words matching a registered
// offline fallback handler, returning the handler and remaining words as args.
// Returns a nil handler if no fallback is registered. Same longest-prefix
// semantics as LookupLocal, but a separate registry so fallbacks are only
// reachable through the daemon-unreachable path. The caller MUST run the
// handler through InvokeLocal.
func LookupOfflineFallback(words []string) (LocalHandler, []string) {
	local.RLock()
	defer local.RUnlock()
	for i := len(words); i > 0; i-- {
		path := textbuf.Join(words[:i], " ")
		if handler, ok := local.fallbacks[path]; ok {
			return handler, append([]string(nil), words[i:]...)
		}
	}
	return nil, nil
}

// ListLocal returns every registered local command sorted by path. Handlers
// are not returned; only path + metadata.
func ListLocal() []LocalCommandEntry {
	local.RLock()
	defer local.RUnlock()
	out := make([]LocalCommandEntry, 0, len(local.handlers))
	for path := range local.handlers {
		out = append(out, LocalCommandEntry{Path: path, Meta: local.meta[path]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// HasLocal reports whether a handler is registered for the exact path.
func HasLocal(path string) bool {
	local.RLock()
	_, ok := local.handlers[path]
	local.RUnlock()
	return ok
}

// ResetLocalForTest clears the local handlers, their metadata and the offline
// fallbacks. Only intended for unit tests that want a clean slate.
func ResetLocalForTest() {
	local.Lock()
	defer local.Unlock()
	local.handlers = make(map[string]LocalHandler)
	local.meta = make(map[string]registry.Meta)
	local.fallbacks = make(map[string]LocalHandler)
}
