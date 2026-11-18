// Design: docs/architecture/storage-backends.md -- indexed rollback through the store owner.
// Related: ../rollback_config.go -- inactive configuration publication.
// Related: data_rpc.go -- restoreDaemonConfig stages the active source for reload.

package cli

import (
	"path/filepath"
	"strconv"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
)

const configRollbackUsage = "request data config rollback name <name> revision <N>"

// handleDataConfigRollback shares the exact stored history selector with the
// local adapter. The running source is staged and reloaded under its confirmed
// window worker; other stored names use the guarded version transaction.
func handleDataConfigRollback(ctx *pluginserver.CommandContext, args command.ValidatedArgs) (*zePlugin.Response, error) {
	target := dataRPC.Load()
	if target == nil {
		return configRollbackRefusal("this daemon serves no store", nil), nil
	}
	name, found := args.Value("name")
	if !found {
		return configRollbackRefusal("a stored config name is required", nil), nil
	}
	if name == "" {
		return configRollbackRefusal("the stored config name is empty", nil), nil
	}
	if !storage.PlainConfigName(name) {
		return configRollbackRefusal("the stored config name must be a filename without a directory", nil), nil
	}
	name, err := storage.StoredConfigName(target.Store, name)
	if err != nil {
		return configRollbackRefusal("the stored config name is refused", err), nil
	}
	value, found := args.Value("revision")
	if !found {
		return configRollbackRefusal("a one-based revision number is required", nil), nil
	}
	revision, err := strconv.Atoi(value)
	if err != nil {
		return configRollbackRefusal("the revision number is invalid", err), nil
	}
	if revision < 1 {
		return configRollbackRefusal("the revision number must be at least 1", nil), nil
	}
	if err := ctx.Context().Err(); err != nil {
		return configRollbackRefusal("the rollback request was canceled", err), nil
	}
	if name == filepath.Base(target.ConfigPath) {
		return rollbackDaemonConfig(ctx, target, name, revision), nil
	}
	if client, managed := pluginserver.ClientNameFromConfigKey(name); managed {
		if target.ServesClient == nil {
			return configRollbackRefusal("managed client "+client+" is not served by this daemon", nil), nil
		}
		if !target.ServesClient(client) {
			return configRollbackRefusal("managed client "+client+" is not configured on this hub", nil), nil
		}
	}
	var selected storage.VersionInfo
	var stamp string
	err = confirm.WriteOutsideRecorded(target.Store, name, func() error {
		var rollbackErr error
		selected, stamp, rollbackErr = storage.RollbackConfig(target.Store, name, revision)
		return rollbackErr
	})
	if err != nil {
		return configRollbackRefusal("the stored rollback could not finish", err), nil
	}
	return configRolledBack(name, revision, selected, stamp), nil
}

func rollbackDaemonConfig(ctx *pluginserver.CommandContext, target *DataRPCTarget, name string, revision int) *zePlugin.Response {
	var window *confirm.Window
	if target.Window != nil {
		window = target.Window()
	}
	var selected storage.VersionInfo
	var stamp string
	err := confirm.WriteOutside(window, func() error {
		// The live worker excludes window opens during this stored-record
		// check, giving the same ownership guarantee as an offline writer.
		return confirm.WriteOutsideRecorded(target.Store, target.ConfigPath, func() error {
			version, data, readErr := storage.ReadRollbackRevision(target.Store, target.ConfigPath, revision)
			if readErr != nil {
				return readErr
			}
			selected = version
			var reloadErr error
			stamp, reloadErr = restoreDaemonConfig(ctx.Context(), target, data)
			return reloadErr
		})
	})
	if err != nil {
		return configRollbackRefusal("the running configuration rollback could not finish", err)
	}
	return configRolledBack(name, revision, selected, stamp)
}

func configRolledBack(name string, revision int, selected storage.VersionInfo, stamp string) *zePlugin.Response {
	return &zePlugin.Response{
		Status: zePlugin.StatusDone,
		Data: zePlugin.Map{
			"name": name, "revision": revision, "source-version": selected.Stamp,
			"source-time": selected.Date, "version": stamp,
		},
	}
}

func configRollbackRefusal(problem string, cause error) *zePlugin.Response {
	return zePlugin.OperatorErrorResponse(opererr.Wrap(cause,
		"request data config rollback: "+problem,
		"no different revision is substituted; inspect the reported history or pending transaction before retrying",
		configRollbackUsage))
}
