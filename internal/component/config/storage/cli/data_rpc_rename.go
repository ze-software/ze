// Design: docs/architecture/storage-backends.md -- daemon-owned stored-config rename.
// Related: ../rename_config.go -- the transaction shared with the offline adapter.
// Related: register_data_rpc.go -- the owner RPC and mutation audit action.

package cli

import (
	"path/filepath"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/storage"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
)

const configRenameUsage = "request data config rename old <name> new <name>"

// handleDataConfigRename changes only an inactive stored identity. The running
// configuration and a configured managed-client fetch name retain external
// owners that this transaction cannot safely rebind, so they are refused.
func handleDataConfigRename(ctx *pluginserver.CommandContext, args command.ValidatedArgs) (*zePlugin.Response, error) {
	target := dataRPC.Load()
	if target == nil {
		return zePlugin.OperatorErrorResponse(opererr.New(
			"request data config rename: this daemon serves no store",
			"a stored configuration can be renamed only while its store owner is available",
			"ze config rename <old-name> <new-name>")), nil
	}
	oldName, found := args.Value("old")
	if !found {
		return configRenameArgumentRefusal("old"), nil
	}
	if oldName == "" {
		return configRenameArgumentRefusal("old"), nil
	}
	if !storage.PlainConfigName(oldName) {
		return configRenameArgumentRefusal("old"), nil
	}
	newName, found := args.Value("new")
	if !found {
		return configRenameArgumentRefusal("new"), nil
	}
	if newName == "" {
		return configRenameArgumentRefusal("new"), nil
	}
	if !storage.PlainConfigName(newName) {
		return configRenameArgumentRefusal("new"), nil
	}
	for _, name := range []string{oldName, newName} {
		if name == filepath.Base(target.ConfigPath) {
			return zePlugin.OperatorErrorResponse(opererr.New(
				"request data config rename: the running configuration identity cannot be renamed",
				"the daemon and its next startup still use "+name+"; renaming the stored keys would leave that source absent",
				"request shutdown")), nil
		}
		if target.ServesClient != nil {
			if client, managed := pluginserver.ClientNameFromConfigKey(name); managed {
				if target.ServesClient(client) {
					return zePlugin.OperatorErrorResponse(opererr.New(
						"request data config rename: the managed client configuration identity cannot be renamed",
						"managed client "+client+" is configured to fetch "+name+"; change its serving identity before renaming stored keys",
						"ze config edit")), nil
				}
			}
		}
	}
	if err := ctx.Context().Err(); err != nil {
		return zePlugin.OperatorErrorResponse(opererr.Wrap(err,
			"request data config rename: the request was canceled",
			"no configuration rename was started",
			configRenameUsage)), nil
	}
	if err := storage.RenameConfig(target.Store, oldName, newName); err != nil {
		return zePlugin.OperatorErrorResponse(opererr.Wrap(err,
			"request data config rename: "+oldName+" could not be renamed to "+newName,
			"the store keeps source data until a complete destination exists; any failed restoration names the complete copy retained, so inspect the reported names before retrying",
			"show config list")), nil
	}
	return &zePlugin.Response{
		Status: zePlugin.StatusDone,
		Data:   zePlugin.Map{"old": oldName, "new": newName},
	}, nil
}

func configRenameArgumentRefusal(keyword string) *zePlugin.Response {
	return zePlugin.OperatorErrorResponse(opererr.New(
		"request data config rename: "+keyword+" must name one config filename without a directory",
		"both old and new are required basenames; no stored keys have changed",
		configRenameUsage))
}
