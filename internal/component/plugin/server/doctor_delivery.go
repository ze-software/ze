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

// handlerPanicRecord keeps the first serverRecordMax recovered panics and
// counts the rest. Safe for concurrent use.
type handlerPanicRecord struct {
	mu      sync.Mutex
	panics  []EngineHandlerPanic
	dropped int
}

// recordHandlerPanic records one recovered engine handler panic.
func (e *engineEventSubscribers) recordHandlerPanic(ns events.NamespaceID, et events.EventTypeID, value any) {
	r := &e.panics
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.panics) >= serverRecordMax {
		r.dropped++
		return
	}
	r.panics = append(r.panics, EngineHandlerPanic{
		Namespace: ns.String(),
		EventType: et.String(),
		Panic:     fmt.Sprint(value),
		Time:      time.Now(),
	})
}

// handlerPanics answers a copy of the recovered engine handler panics.
func (e *engineEventSubscribers) handlerPanics() []EngineHandlerPanic {
	r := &e.panics
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.panics)
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

// enricherFailureRecord holds the enricher failures per plugin name. A call
// that succeeds clears its own call failure; the plugin's process cleanup
// clears them all (unregisterProxyEnrichers).
var (
	enricherFailureMu  sync.Mutex
	enricherFailureMap = map[string][]EnricherFailure{}
)

// recordEnricherFailure records a failure, replacing an earlier one of the
// same command, key and kind, bounded per plugin at serverRecordMax.
func recordEnricherFailure(pluginName string, failure EnricherFailure) {
	failure.Time = time.Now()
	enricherFailureMu.Lock()
	defer enricherFailureMu.Unlock()
	list := enricherFailureMap[pluginName]
	for i := range list {
		if sameEnricher(&list[i], &failure) {
			list[i] = failure
			return
		}
	}
	if len(list) >= serverRecordMax {
		return
	}
	enricherFailureMap[pluginName] = append(list, failure)
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

// clearEnricherCallFailure removes the call failure of one enricher, once a
// call succeeded.
func clearEnricherCallFailure(pluginName, command, key string) {
	enricherFailureMu.Lock()
	defer enricherFailureMu.Unlock()
	list := enricherFailureMap[pluginName]
	call := EnricherFailure{Command: command, Key: key}
	list = slices.DeleteFunc(list, func(f EnricherFailure) bool { return sameEnricher(&f, &call) })
	if len(list) == 0 {
		delete(enricherFailureMap, pluginName)
		return
	}
	enricherFailureMap[pluginName] = list
}

// forgetEnricherFailures drops every failure of a plugin whose process ended.
func forgetEnricherFailures(pluginName string) {
	enricherFailureMu.Lock()
	defer enricherFailureMu.Unlock()
	delete(enricherFailureMap, pluginName)
}

// enricherFailures answers a copy of one plugin's enricher failures.
func enricherFailures(pluginName string) []EnricherFailure {
	enricherFailureMu.Lock()
	defer enricherFailureMu.Unlock()
	return slices.Clone(enricherFailureMap[pluginName])
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
