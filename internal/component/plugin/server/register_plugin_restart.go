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
	"slices"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/opererr"
)

func init() {
	RegisterRPCs(RPCRegistration{
		WireMethod:       "ze-plugin:system-plugin-restart",
		Handler:          handlePluginRestart,
		RequiresSelector: true,
		Complete:         completePluginRestart,
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
// The restart holds the config transaction lock, so it never replaces the
// process a reload or commit is talking to; one in flight refuses it. The
// lock is taken here and not in restartPlugin, which the rollback calls while
// it holds the lock.
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
	if !ctx.Server.txLock.tryAcquire() {
		return plugin.OperatorErrorResponse(opererr.New("plugin "+name+" was not restarted: a config transaction is in flight",
			"a restart replaces the process the reload or commit is talking to, which would fail it",
			process.RestartCommand(name)+", once the reload or commit in flight has answered")), nil
	}
	defer ctx.Server.txLock.release()

	wasRunning := false
	if proc := pm.GetProcess(name); proc != nil {
		wasRunning = proc.Running()
	}
	pm.Reenable(name)
	if err := ctx.Server.restartPlugin(name); err != nil {
		return plugin.OperatorErrorResponse(opererr.Wrap(err, "plugin "+name+" did not start again",
			"its restart failed, so it stays down and ze doctor names it",
			"ze show doctor")), nil
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
