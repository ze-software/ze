// Design: docs/architecture/web-interface.md -- private sources enter native selected or full restore.
// Related: ../storage/cli/data_rpc.go -- the shared selected-config publication.
// Related: ../storage/cli/data_rpc_full_restore.go -- the shared acceptance and restart boundary.
package cli

import (
	"io"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/artifact"
	"github.com/ze-software/ze/internal/component/config/storage"
	storagecli "github.com/ze-software/ze/internal/component/config/storage/cli"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
)

func artifactRestoreConfig(ctx *pluginserver.CommandContext, owner *artifact.Owner, args command.ValidatedArgs) (*plugin.Response, error) {
	id := artifactValue(args, "artifact")
	response := storagecli.RestoreConfigFromSource(ctx.Context(), artifactValue(args, "client"), func(deviceName string) (storage.RestoreSource, error) {
		var selected storage.RestoreSource
		err := owner.ViewSource(ctx.Context(), ctx.Username, id, func(path string) error {
			var err error
			selected, err = storage.ReadRestoreSource(path, artifactValue(args, "name"), deviceName)
			return err
		})
		return selected, err
	})
	if response.Status == plugin.StatusDone {
		if data, ok := response.Data.(plugin.Map); ok {
			data["artifact"] = id
		}
	}
	return response, nil
}

func artifactRestoreFull(ctx *pluginserver.CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
	owner, err := artifactCommandOwner(ctx, args)
	if err != nil {
		return plugin.OperatorErrorResponse(opererr.Wrap(err,
			"The artifact restore could not start.", "No recovery source was accepted.", "ze request config artifact restore full help")), nil
	}
	return storagecli.AcceptFullRestoreWriter(ctx, func(destination io.Writer) error {
		return owner.ExportTo(ctx.Context(), ctx.Username, artifactValue(args, "artifact"), destination)
	})
}
