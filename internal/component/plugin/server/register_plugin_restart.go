// Design: docs/architecture/doctor-and-health-checks.md -- plugin state a runtime check reads
// Related: reload_tx.go -- restartPlugin, the restart this command runs
// Related: ../process/stopped.go -- RestartCommand, the fix every doctor-plugin-stopped row names
//
// `request plugin restart <name>` is the command that starts a stopped plugin
// again. A plugin can stop for good on several paths (its failure policy, the
// respawn limit, a failed restart, a failed handshake), and before this
// command the only way back was restarting ze. It is the fix doctor names for
// doctor-plugin-stopped.

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

// completePluginRestart offers the configured plugins that are not running:
// exactly the names handlePluginRestart accepts. A running plugin is left
// out because the command refuses it, and a name nothing configured is never
// offered at all.
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
	var candidates []Completion
	for _, name := range pm.ConfiguredNames() {
		if pluginRestartRefusal(pm, name) != nil {
			continue
		}
		candidates = append(candidates, Completion{Value: name, ShortHelp: "configured plugin, not running"})
	}
	return candidates
}

// pluginRestartLeaf is the YANG leaf the plugin name is typed under
// (ze-system-cmd.yang, request plugin restart).
const pluginRestartLeaf = "name"

// fieldRestartedPlugin is the answer key that names the plugin started again.
const fieldRestartedPlugin = "plugin"

// handlePluginRestart starts the named configured plugin again. The operator's
// order lifts a respawn-limit disable, because the limit guards against a
// crash loop nobody asked for, and this restart was asked for. A running
// plugin is refused, because restarting it would stop a working plugin.
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

	pm.Reenable(name)
	if err := ctx.Server.restartPlugin(name); err != nil {
		return plugin.OperatorErrorResponse(opererr.Wrap(err, "plugin "+name+" did not start again",
			"its restart failed, so it stays down and ze doctor names it",
			"ze show doctor")), nil
	}
	return &plugin.Response{
		Status: plugin.StatusDone,
		Data:   plugin.Map{fieldMessage: "plugin " + name + " started again", fieldRestartedPlugin: name},
	}, nil
}

// pluginRestartRefusal answers why name cannot be restarted, or nil. Only a
// configured plugin that is not running can be: an unknown name is refused
// rather than guessed, and a running one is left alone.
func pluginRestartRefusal(pm *process.ProcessManager, name string) *opererr.Error {
	if !slices.Contains(pm.ConfiguredNames(), name) {
		return opererr.New("plugin "+name+" is not a configured plugin",
			"only a plugin the configuration or its auto-load started can be restarted",
			"ze show doctor, which names every configured plugin that is down")
	}
	if proc := pm.GetProcess(name); proc != nil {
		if proc.Running() {
			return opererr.New("plugin "+name+" is running",
				"restarting it would stop a working plugin",
				"ze show doctor, which names every configured plugin that is down")
		}
	}
	return nil
}
