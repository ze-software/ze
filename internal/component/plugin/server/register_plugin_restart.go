// Design: docs/architecture/doctor-and-health-checks.md -- plugin state a runtime check reads
// Related: reload_tx.go -- restartPlugin, the restart this command runs
// Related: ../process/stopped.go -- RestartCommand, the fix the doctor rows name
//
// `request plugin restart <name>` stops a configured plugin if it runs and
// starts it again. A plugin can stop for good on several paths (its failure
// policy, the respawn limit, a failed restart, a failed handshake), and before
// this command the only way back was restarting ze. A running plugin can hold
// state only a new process clears (a missed config-committed, a failed event
// application, a post-startup callback it never received), so the command
// takes a running plugin too. It is the fix doctor names for those rows and
// for doctor-plugin-stopped.

package server

import (
	"errors"
	"slices"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/opererr"
)

// The registration carries no RequiresSelector. That guard is the peer
// selector's: it waits for a value bound to the leaf named `selector`
// (selectorLeaf), which a non-peer command never declares, because
// ai/rules/cli.md types a named member with `name`. The mandatory `name` leaf
// in ze-system-cmd.yang is this command's guard: command.ValidateArgs binds
// the lone value typed after `restart`, or the `name <value>` pair, and
// refuses a line with neither before the handler runs, as it does for
// `system command help <name>`.
func init() {
	RegisterRPCs(RPCRegistration{
		WireMethod: "ze-plugin:system-plugin-restart",
		Handler:    handlePluginRestart,
		Complete:   completePluginRestart,
	})
}

// The completion help of a configured plugin, by whether it runs.
const (
	completionRunning    = "configured plugin, running: the restart stops it first"
	completionNotRunning = "configured plugin, not running"
)

// completePluginRestart offers every configured plugin, exactly the names
// handlePluginRestart accepts, saying which run. A name nothing configured is
// never offered.
func completePluginRestart(s *Server) []Completion {
	// refuse-by-default: guards handleArgComplete -- no plugin server holds no configured plugin, so no name completes
	if s == nil {
		return nil
	}
	pm := s.procManager.Load()
	// refuse-by-default: guards handleArgComplete -- before startup hands plugins to a manager none is configured yet
	if pm == nil {
		return nil
	}
	names := pm.ConfiguredNames()
	candidates := make([]Completion, 0, len(names))
	for _, name := range names {
		help := completionNotRunning
		if proc := pm.GetProcess(name); proc != nil && proc.Running() {
			help = completionRunning
		}
		candidates = append(candidates, Completion{Value: name, ShortHelp: help})
	}
	return candidates
}

// pluginRestartLeaf is the YANG leaf the plugin name is typed under
// (ze-system-cmd.yang, request plugin restart).
const pluginRestartLeaf = "name"

// The answer keys: the plugin started again, and whether it was running.
const (
	fieldRestartedPlugin   = "plugin"
	fieldRestartWasRunning = "was-running"
)

// handlePluginRestart restarts the named configured plugin. A running plugin
// is stopped first by the same restart a config rollback runs for a broken
// one (restartPlugin, ProcessManager.Respawn stops and joins the process it
// replaces), which cuts the commands it was serving; its failure policy is not
// applied, because the name already holds the replacement
// (applyFailurePolicy). The operator's order lifts a respawn-limit disable,
// because the limit guards against a crash loop nobody asked for, and this
// restart was asked for.
//
// The restart holds the config transaction lock, so it never runs beside a
// reload or commit, which may be talking to the plugin's process; any
// transaction in flight refuses it. The lock is taken here and not in
// restartPlugin, which the rollback calls while it holds the lock.
//
// Two more refusals keep the restart from undoing work it cannot see. A
// plugin asking for its own restart is refused before anything stops: the
// restart waits for the runtime cleanup of the process it replaces, and that
// cleanup runs only once the process's in-flight calls have returned, this
// request among them (the reload refuses to stop its caller the same way,
// reload.go). A plugin whose removal a refused reload left pending is refused
// under the lock, because the removal attempt and its recovery own that
// process until they settle.
func handlePluginRestart(ctx *CommandContext, validated command.ValidatedArgs) (*plugin.Response, error) {
	names := ctx.ArgsOrSelector(validated.Tokens(), pluginRestartLeaf)
	if len(names) != 1 {
		return plugin.OperatorErrorResponse(opererr.New("request plugin restart needs exactly one plugin name",
			"the command restarts one plugin, so which one must be named",
			"ze request plugin restart <name>, with a name from ze show doctor")), nil
	}
	name := names[0]
	if ctx.Server == nil {
		return plugin.OperatorErrorResponse(opererr.New("plugin "+name+" cannot be restarted: no plugin server runs here",
			"the restart is run by the daemon's plugin server",
			"ze start")), nil
	}
	pm := ctx.Server.procManager.Load()
	if pm == nil {
		return plugin.OperatorErrorResponse(opererr.New("plugin "+name+" cannot be restarted: plugin startup has not begun",
			"the daemon has not handed its plugins to a process manager yet",
			"ze show doctor")), nil
	}
	if refusal := pluginRestartRefusal(pm, name); refusal != nil {
		return plugin.OperatorErrorResponse(refusal), nil
	}
	if ctx.Process != nil && ctx.Process.Name() == name {
		return plugin.OperatorErrorResponse(opererr.New("plugin "+name+" cannot restart itself",
			"the restart waits until the plugin's calls into ze have ended, and this request is one of them, "+
				"so it would never end; type the command at the CLI instead",
			process.RestartCommand(name))), nil
	}
	if !ctx.Server.txLock.tryAcquire() {
		return plugin.OperatorErrorResponse(opererr.New("plugin "+name+" was not restarted: a config transaction is in flight",
			"a restart replaces the process the reload or commit is talking to, which would fail it",
			process.RestartCommand(name)+", once the reload or commit in flight has answered")), nil
	}
	defer ctx.Server.txLock.release()

	if ctx.Server.removalPendingFor(name) {
		return plugin.OperatorErrorResponse(opererr.New("plugin "+name+" was not restarted: its removal is still pending",
			"a reload removed it and the removal has not settled (the plugin has not acknowledged it, or putting it back failed), "+
				"and a restart would replace the process that removal is acting on; a reload retries the removal's recovery",
			"ze request reload")), nil
	}

	wasRunning := false
	if proc := pm.GetProcess(name); proc != nil {
		wasRunning = proc.Running()
	}
	pm.Reenable(name)
	if err := ctx.Server.restartPlugin(name); err != nil {
		return plugin.OperatorErrorResponse(pluginRestartFailure(name, err)), nil
	}
	message := "plugin " + name + " started again"
	if wasRunning {
		message = "plugin " + name + " was running: stopped and started again"
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{fieldMessage: message, fieldRestartedPlugin: name, fieldRestartWasRunning: wasRunning},
	}, nil
}

// pluginRestartFailure is the operator error of a restart that failed. Its
// cause is in err either way; a stopped process whose cleanup had not
// finished gets its own explanation, because the restart can succeed only
// once that cleanup has finished, and ze knows only that it had not: it may
// wait for a call the process made into ze, or still be running.
func pluginRestartFailure(name string, err error) *opererr.Error {
	if errors.Is(err, errReplacedCleanupLate) {
		return opererr.Wrap(err, "plugin "+name+" did not start again",
			"a new process cannot take the plugin's name until the process it replaces has finished its cleanup, "+
				"so the plugin stays down and ze show doctor names it; run the command below again once that cleanup "+
				"has finished, which the doctor row stops saying it owes; if it never finishes, the last resort is "+
				"to restart the ze daemon, which ends every call a plugin made into it: systemctl restart ze under systemd "+
				"(ze signal restart writes the graceful-restart marker and shuts ze down, so ze comes back only under a supervisor)",
			process.RestartCommand(name))
	}
	return opererr.Wrap(err, "plugin "+name+" did not start again",
		"its restart failed, so it stays down and ze doctor names it",
		"ze show doctor")
}

// removalPendingFor reports whether a removal attempt still owns a process of
// name: a reload removed the plugin and the removal has not settled. The
// caller holds the transaction lock, so no removal attempt is being created
// (notifyPluginRemoval runs inside a reload); one that outlived its reload
// stays in s.removals until its recovery or a retry ends it.
func (s *Server) removalPendingFor(name string) bool {
	s.removalMu.Lock()
	defer s.removalMu.Unlock()
	for proc := range s.removals {
		if proc.Name() == name {
			return true
		}
	}
	return false
}

// pluginRestartRefusal answers why name cannot be restarted, or nil. Only a
// configured plugin can be: an unknown name is refused rather than guessed.
func pluginRestartRefusal(pm *process.ProcessManager, name string) *opererr.Error {
	if !slices.Contains(pm.ConfiguredNames(), name) {
		return opererr.New("plugin "+name+" is not a configured plugin",
			"only a plugin the configuration or its auto-load started can be restarted",
			"ze show doctor, which names every configured plugin")
	}
	return nil
}
