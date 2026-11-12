// Design: docs/architecture/doctor-and-health-checks.md -- plugin registration and delivery checks
// Related: doctor_snapshot.go -- the snapshot that carries these records to the checks
// Related: startup.go -- onReady registers the plugin's commands through registerProcessCommands
// Related: engine_event.go -- invokeHandler records a recovered engine handler panic
// Related: enricher.go -- registerProxyEnrichers and makeProxyCall record enricher failures
// Related: config_tx_bridge.go -- subscribeCommitted and dispatchCommitted record commits
// Related: ../process/doctor_records.go -- the per-process records
//
// The server-side records of the registration and delivery problem classes:
// who owns a command name a plugin was refused, the recovered engine handler
// panics, the enricher failures per plugin and the daemon's last committed
// transaction. A per-plugin record lives on the plugin's process; what has no
// process to live on (an engine handler, a package-level enricher registry,
// the daemon's own commit) lives here, bounded.

package server

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/events"
)

// serverRecordMax bounds each server-side doctor record list.
const serverRecordMax = 16

// CommandOwnerBuiltin is the owner a refusal names when the name is a
// builtin command.
const CommandOwnerBuiltin = "builtin"

// commandOwnedError is a deprecated alias refused because another holder
// owns the name. Error keeps the registry's refusal text.
type commandOwnedError struct {
	message string
	owner   string
}

func (e *commandOwnedError) Error() string { return e.message }

// registerProcessCommands registers the commands and deprecated aliases a
// plugin declared, and records each refusal on the plugin's process, naming
// the owner of a name already held, for the doctor (AC-13).
func registerProcessCommands(r *CommandRegistry, proc *process.Process, reg *plugin.PluginRegistration) {
	if reg == nil {
		return
	}
	if len(reg.Commands) == 0 {
		return
	}
	defs := make([]CommandDef, len(reg.Commands))
	for i, name := range reg.Commands {
		defs[i] = CommandDef{
			Name:        name,
			ShortHelp:   reg.CommandShortHelp[name],
			Description: reg.CommandDescription[name],
			Hidden:      reg.CommandHidden[name],
			Completable: reg.CommandCompletable[name],
		}
	}
	for _, result := range r.Register(proc, defs) {
		if result.OK {
			logger().Debug("command registered", "plugin", proc.Name(), "command", result.Name)
			continue
		}
		logger().Warn("command registration rejected", "plugin", proc.Name(), "command", result.Name,
			"owner", result.Owner, "error", result.Error,
			"code", diagnostic.CodeDoctorPluginCommandNotRegistered)
		proc.RecordCommandRefusal(process.CommandRefusal{Command: result.Name, Owner: result.Owner, Reason: result.Error})
	}
	for canonicalName, oldNames := range reg.CommandDeprecatedNames {
		for _, oldName := range oldNames {
			err := r.registerDeprecated(proc, oldName, canonicalName)
			if err == nil {
				logger().Debug("deprecated alias registered", "plugin", proc.Name(), "old", oldName, "new", canonicalName)
				continue
			}
			owner := ""
			if owned, ok := errors.AsType[*commandOwnedError](err); ok {
				owner = owned.owner
			}
			logger().Warn("deprecated alias rejected", "plugin", proc.Name(), "old", oldName, "new", canonicalName,
				"owner", owner, "error", err, "code", diagnostic.CodeDoctorPluginCommandNotRegistered)
			proc.RecordCommandRefusal(process.CommandRefusal{Command: oldName, Alias: true, Owner: owner, Reason: err.Error()})
		}
	}
}

// EngineHandlerPanic is one engine event handler panic that was recovered.
type EngineHandlerPanic struct {
	Namespace string
	EventType string
	Panic     string
	Time      time.Time
}

// HandlerPanicWindow is how far back the recovered engine handler panics the
// doctor reports reach. An engine handler has no process a restart replaces,
// so a panic stops being reported once it is older than the window, the way a
// delivery problem leaves process.DeliveryWindow.
const HandlerPanicWindow = 5 * time.Minute

// handlerPanicRecord keeps the serverRecordMax newest recovered panics: the
// newest overwrite the oldest, and the time of the newest overwritten one says
// whether the window lost any. Safe for concurrent use.
type handlerPanicRecord struct {
	mu     sync.Mutex
	panics [serverRecordMax]EngineHandlerPanic
	next   int
	count  int
	// overwrittenNewest is the time of the newest panic the ring overwrote,
	// zero while it overwrote none.
	overwrittenNewest time.Time
}

// recordHandlerPanic records one recovered engine handler panic.
func (e *engineEventSubscribers) recordHandlerPanic(ns events.NamespaceID, et events.EventTypeID, value any) {
	panicked := EngineHandlerPanic{
		Namespace: ns.String(),
		EventType: et.String(),
		Panic:     fmt.Sprint(value),
		Time:      time.Now(),
	}
	r := &e.panics
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.count == serverRecordMax {
		if victim := r.panics[r.next].Time; victim.After(r.overwrittenNewest) {
			r.overwrittenNewest = victim
		}
	}
	r.panics[r.next] = panicked
	r.next = (r.next + 1) % serverRecordMax
	if r.count < serverRecordMax {
		r.count++
	}
}

// handlerPanics answers the recovered engine handler panics inside the
// HandlerPanicWindow that ends now.
func (e *engineEventSubscribers) handlerPanics() []EngineHandlerPanic {
	panics, _ := e.handlerPanicsAt(time.Now())
	return panics
}

// handlerPanicsAt answers a copy of the recovered engine handler panics inside
// the HandlerPanicWindow that ends at now, oldest first, and whether the ring
// overwrote a panic that would still be inside it.
func (e *engineEventSubscribers) handlerPanicsAt(now time.Time) ([]EngineHandlerPanic, bool) {
	since := now.Add(-HandlerPanicWindow)
	r := &e.panics
	r.mu.Lock()
	defer r.mu.Unlock()
	truncated := false
	if !r.overwrittenNewest.IsZero() {
		truncated = !r.overwrittenNewest.Before(since)
	}
	panics := make([]EngineHandlerPanic, 0, r.count)
	start := (r.next - r.count + serverRecordMax) % serverRecordMax
	for i := range r.count {
		panicked := r.panics[(start+i)%serverRecordMax]
		if panicked.Time.Before(since) {
			continue
		}
		panics = append(panics, panicked)
	}
	return panics, truncated
}

// EnricherFailure is one `show` enricher a plugin declared that could not be
// registered (Registration true) or whose last call failed.
type EnricherFailure struct {
	Command      string
	Key          string
	Registration bool
	Error        string
	Time         time.Time
}

// enricherFailureList is one plugin's enricher failures: the first
// serverRecordMax kept whole, then the identity alone (command, key, kind) of
// each failing enricher past that bound, so the doctor row counts them. The
// identities are bounded by the enrichers the plugin declared (maxEnrichers,
// validateEnricherDecls), each failing in one kind per lifetime.
type enricherFailureList struct {
	kept    []EnricherFailure
	dropped []EnricherFailure
}

// enricherFailureRecord holds the enricher failures per plugin name. A call
// that succeeds clears its own call failure, kept or counted; the plugin's
// process cleanup clears them all (unregisterProxyEnrichers), and so does the
// next process's registration (registerProxyEnrichers).
var (
	enricherFailureMu  sync.Mutex
	enricherFailureMap = map[string]*enricherFailureList{}
)

// recordEnricherFailure records a failure, replacing an earlier one of the
// same command, key and kind. Past serverRecordMax kept per plugin, the
// failing enricher is counted once, however often it fails.
func recordEnricherFailure(pluginName string, failure EnricherFailure) {
	failure.Time = time.Now()
	enricherFailureMu.Lock()
	defer enricherFailureMu.Unlock()
	list := enricherFailureMap[pluginName]
	if list == nil {
		list = &enricherFailureList{}
		enricherFailureMap[pluginName] = list
	}
	for i := range list.kept {
		if sameEnricher(&list.kept[i], &failure) {
			list.kept[i] = failure
			return
		}
	}
	if len(list.kept) < serverRecordMax {
		list.kept = append(list.kept, failure)
		return
	}
	identity := EnricherFailure{Command: failure.Command, Key: failure.Key, Registration: failure.Registration}
	if slices.ContainsFunc(list.dropped, func(f EnricherFailure) bool { return sameEnricher(&f, &identity) }) {
		return
	}
	list.dropped = append(list.dropped, identity)
}

// sameEnricher reports whether two failures are of one enricher and kind.
func sameEnricher(a, b *EnricherFailure) bool {
	if a.Command != b.Command {
		return false
	}
	if a.Key != b.Key {
		return false
	}
	return a.Registration == b.Registration
}

// clearEnricherCallFailure removes the call failure of one enricher, kept or
// counted, once a call succeeded.
func clearEnricherCallFailure(pluginName, command, key string) {
	enricherFailureMu.Lock()
	defer enricherFailureMu.Unlock()
	list := enricherFailureMap[pluginName]
	if list == nil {
		return
	}
	call := EnricherFailure{Command: command, Key: key}
	isCall := func(f EnricherFailure) bool { return sameEnricher(&f, &call) }
	list.kept = slices.DeleteFunc(list.kept, isCall)
	list.dropped = slices.DeleteFunc(list.dropped, isCall)
	if len(list.kept) == 0 && len(list.dropped) == 0 {
		delete(enricherFailureMap, pluginName)
	}
}

// forgetEnricherFailures drops every failure of a plugin whose process ended.
func forgetEnricherFailures(pluginName string) {
	enricherFailureMu.Lock()
	defer enricherFailureMu.Unlock()
	delete(enricherFailureMap, pluginName)
}

// enricherFailures answers a copy of one plugin's kept enricher failures.
func enricherFailures(pluginName string) []EnricherFailure {
	enricherFailureMu.Lock()
	defer enricherFailureMu.Unlock()
	list := enricherFailureMap[pluginName]
	// refuse-by-default: guards pluginDoctorState -- a plugin with no entry has no enricher failure: recordEnricherFailure creates the entry on the first one, and the clears delete it once empty
	if list == nil {
		return nil
	}
	return slices.Clone(list.kept)
}

// enricherFailuresDropped answers how many of one plugin's enrichers fail
// past the serverRecordMax kept whole.
func enricherFailuresDropped(pluginName string) int {
	enricherFailureMu.Lock()
	defer enricherFailureMu.Unlock()
	list := enricherFailureMap[pluginName]
	// refuse-by-default: guards pluginDoctorState -- a plugin with no entry has no enricher failure, so none past the bound
	if list == nil {
		return 0
	}
	return len(list.dropped)
}

// committedRecord is the daemon's last committed config transaction. Safe
// for concurrent use.
type committedRecord struct {
	mu sync.Mutex
	tx string
}

// recordCommitted records the transaction the daemon committed, before the
// config-committed callbacks go out.
func (s *Server) recordCommitted(tx string) {
	s.committed.mu.Lock()
	defer s.committed.mu.Unlock()
	s.committed.tx = tx
}

// lastCommittedTransaction answers the daemon's last committed transaction,
// empty when none committed since the server started.
func (s *Server) lastCommittedTransaction() string {
	s.committed.mu.Lock()
	defer s.committed.mu.Unlock()
	return s.committed.tx
}
