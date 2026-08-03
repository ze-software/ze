// Design: docs/architecture/api/commands.md -- show verb RPC registration
// Overview: doc.go -- show verb package registration
// Related: show.go -- the other process-global show handlers (health, warnings, uptime)

package show

import (
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// `show reload-status` stays in the CENTRAL show package on purpose. The reload
// generation counter is process-global daemon state with no removable owner --
// the same class as `show warnings` and `show health` -- so it does not belong
// to any plugin's subtree (ai/rules/plugins.md). In particular
// it is NOT `show config reload-status`: that subtree is owned by
// internal/plugins/config-cli, and hanging a centrally-handled command off a
// plugin's schema inverts the removal test.
func init() {
	pluginserver.RegisterRPCs(
		pluginserver.RPCRegistration{
			WireMethod: "ze-show:reload-status",
			Handler:    handleShowReloadStatus,
		},
	)
}

// handleShowReloadStatus returns the reload generation counter, the outcome of
// the most recent reload, and when it finished.
//
// The counter advances on every PROCESSED reload, applied or rejected, which is
// what makes it usable as a fence. A reload that rejects a change (l2tp
// refusing a listener rebind) or that changes nothing leaves no other
// observable trace, so an observer that wants to assert "the reload ran and
// correctly left this alone" has nothing else to wait on. It reads the
// generation, triggers the reload, polls until the generation advances, then
// asserts. See internal/component/plugin/server/reload_generation.go.
func handleShowReloadStatus(ctx *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	if ctx == nil || ctx.Server == nil {
		return &plugin.Response{
			Status: plugin.StatusError,
			Error:  "daemon not running",
		}, nil
	}

	generation, outcome, at := ctx.Server.ReloadStatus()
	data := plugin.Map{
		"generation":   generation,
		"last-outcome": outcome,
	}
	// Zero time before the first reload: report the field as empty rather than
	// as a year-1 timestamp, so an observer can tell "never reloaded" from a
	// real completion time.
	if at.IsZero() {
		data["last-reload-at"] = ""
	} else {
		data["last-reload-at"] = at.UTC().Format(time.RFC3339)
	}

	return &plugin.Response{Status: plugin.StatusDone, Data: data}, nil
}
