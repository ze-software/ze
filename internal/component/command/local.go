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
	"os"
	"sort"
	"sync"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/opererr"
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

// DaemonUnanswered is why the daemon did not answer the command an offline
// fallback is run for, and the command that gets the daemon's answer. The
// route that asked the daemon builds it from the failure it met; a fallback
// words its own answer with it and never guesses the cause.
//
// Build it with NewDaemonUnanswered. The zero value names no cause and is not
// a valid one: InvokeOfflineFallback refuses it.
type DaemonUnanswered struct {
	why string
	fix string
}

// NewDaemonUnanswered builds the cause an offline fallback receives. why is a
// clause with no capital and no full stop that says why no daemon answered,
// naming the address when it is known ("no ze listens at 127.0.0.1:2222
// (connection refused)"). fix is the one command that gets the daemon's
// answer, alone, as opererr.New takes it. Both are text the route writes
// around the failure it met, so an empty one is a defect of that route and
// panics with BUG.
func NewDaemonUnanswered(why, fix string) DaemonUnanswered {
	if why == "" {
		panic("BUG: command.NewDaemonUnanswered called with an empty why")
	}
	if fix == "" {
		panic("BUG: command.NewDaemonUnanswered called with an empty fix for: " + why)
	}
	return DaemonUnanswered{why: why, fix: fix}
}

// Why answers why no daemon answered, as a clause with no capital and no full
// stop.
func (u DaemonUnanswered) Why() string { return u.why }

// Fix answers the command that gets the daemon's answer, without a label.
func (u DaemonUnanswered) Fix() string { return u.fix }

// OfflineHandler answers a command in this process after the daemon did not
// answer it, and answers the process exit code. args is the judged value a
// LocalHandler takes. unanswered is why the daemon did not answer: a fallback
// that holds the answer in this process (hardware inventory, crash files)
// prints it and exits 0, and InvokeOfflineFallback then tells the operator the
// answer is this process's and why. A fallback whose answer only the daemon
// holds refuses, and its refusal MUST name unanswered.Why and end on
// unanswered.Fix, never a cause of its own.
type OfflineHandler func(args ValidatedArgs, unanswered DaemonUnanswered) int

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
	fallbacks map[string]OfflineHandler
}{
	handlers:  make(map[string]LocalHandler),
	meta:      make(map[string]registry.Meta),
	fallbacks: make(map[string]OfflineHandler),
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
//
// A meta.DaemonPath equal to path is refused: the field names ANOTHER path
// that answers on the daemon, and naming itself would hide a missing YANG
// node behind a declaration that looks satisfied.
func RegisterLocalMeta(path string, handler LocalHandler, meta registry.Meta) error {
	if meta.DaemonPath == path {
		return fmt.Errorf("command.RegisterLocalMeta: %q declares itself as its daemon path; leave DaemonPath empty when the local path is the YANG path", path)
	}
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
// Lookup matches whole decoded words against registered path segments,
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
		path := textbuf.Join(words[:i], " ")
		if handler, ok := local.handlers[path]; ok {
			if !invocationMatchesPathPrefix(words[:i], path) {
				continue
			}
			return handler, i
		}
	}
	return nil, 0
}

// InvokeLocal judges args against the leaves the model declares for the
// registered path and runs handler on the value that judgment returned. A
// refusal is written to stderr and exits 1 without reaching the handler. Every
// route that runs a handler LookupLocal answered MUST call this with the path
// it matched and every token it will pass: the `ze <verb>` routes (R6). The
// daemon-down fallback (R7) calls InvokeOfflineFallback, which judges the same
// way.
func InvokeLocal(path string, handler LocalHandler, args []string) int {
	validated, judged := judgeLocalArgs(path, args)
	if !judged {
		return 1
	}
	return handler(validated)
}

// judgeLocalArgs judges args against the leaves the model declares for path.
// judged is false when the model refused them: the refusal is then already on
// stderr, and the caller exits 1 without running a handler.
func judgeLocalArgs(path string, args []string) (validated ValidatedArgs, judged bool) {
	invocation := "ze " + path
	validated, err := validateModelArgsAs(invocation, path, args, nil)
	if err != nil {
		writeLocalRefusal(invocation, err)
		return ValidatedArgs{}, false
	}
	return validated, true
}

// InvokeOfflineFallback runs an offline fallback (route R7): it judges args as
// InvokeLocal does, then runs handler with why the daemon did not answer.
// Every route that runs a handler LookupOfflineFallback answered MUST call
// this with the path it matched, every token it will pass, and the cause it
// learned from its own failed ask of the daemon.
//
// A fallback that exits 0 answered from this process. The operator is then
// told so on stderr, with why the daemon did not answer and the command that
// gets the daemon's answer, so a local answer is never read as the daemon's.
// A fallback that exits non-zero printed its own refusal, which names the
// cause itself (OfflineHandler).
func InvokeOfflineFallback(path string, handler OfflineHandler, args []string, unanswered DaemonUnanswered) int {
	if unanswered.why == "" {
		panic("BUG: command.InvokeOfflineFallback called with no cause for: " + path)
	}
	validated, judged := judgeLocalArgs(path, args)
	if !judged {
		return 1
	}
	code := handler(validated, unanswered)
	if code == 0 {
		os.Stderr.WriteString(offlineAnswerNote(path, unanswered).Error() + "\n") //nolint:errcheck // CLI diagnostic
	}
	return code
}

// offlineAnswerNote is the text InvokeOfflineFallback writes after a fallback
// answered from this process: whose answer it is, why the daemon did not give
// one, and the command that gets the daemon's.
func offlineAnswerNote(path string, unanswered DaemonUnanswered) *opererr.Error {
	var problem, why textbuf.Buffer
	problem.Str("note: this ze process answered ").Str(path).Str(" itself, because the daemon did not")
	why.Str("the daemon was asked first and gave no answer: ").Str(unanswered.why)
	return opererr.New(problem.String(), why.String(), unanswered.fix)
}

// RegisterOfflineFallback registers an in-process handler for a read-only
// command path (for example "show crashes" or "show host") that is served ONLY
// when the daemon did not answer. Unlike RegisterLocal, a fallback is never
// consulted while the daemon answers, so it does not shadow the daemon command:
// the CLI asks the daemon first and calls the fallback only after that ask
// failed, handing it why (OfflineHandler, DaemonUnanswered). Intended for
// host-local read-only data (crash files, hardware inventory) an operator must
// still be able to read with no daemon running. A path below such a fallback
// whose answer only the daemon holds registers a fallback of its own, which
// refuses with the cause it was handed.
func RegisterOfflineFallback(path string, handler OfflineHandler) error {
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
func MustRegisterOfflineFallback(path string, handler OfflineHandler) {
	if err := RegisterOfflineFallback(path, handler); err != nil {
		panic("BUG: command.MustRegisterOfflineFallback: " + err.Error())
	}
}

// LookupOfflineFallback finds the longest prefix of words matching a registered
// offline fallback handler, returning the handler and remaining words as args.
// Returns a nil handler if no fallback is registered. Same longest-prefix
// semantics as LookupLocal, but a separate registry so fallbacks are only
// reachable through the daemon-unreachable path. The caller MUST run the
// handler through InvokeOfflineFallback.
func LookupOfflineFallback(words []string) (OfflineHandler, []string) {
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

// ModelPath answers the YANG path whose leaves judge the arguments of path:
// the registration's DaemonPath for a local command at an offline root
// (registry.Meta.DaemonPath), path itself for every other command. Every
// route that validates a local command reads it, so `ze doctor` is judged by
// the leaves of `show doctor`, the node that answers it on the daemon.
func ModelPath(path string) string {
	local.RLock()
	meta, ok := local.meta[path]
	local.RUnlock()
	if ok && meta.DaemonPath != "" {
		return meta.DaemonPath
	}
	return path
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
	local.fallbacks = make(map[string]OfflineHandler)
}
