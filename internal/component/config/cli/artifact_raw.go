// Design: docs/architecture/web-interface.md -- native raw key and salvage commands.
package cli

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/artifact"
	"github.com/ze-software/ze/internal/component/config/storage"
	storagecli "github.com/ze-software/ze/internal/component/config/storage/cli"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/zefs"
)

func artifactBytesBegin(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	total, err := artifactInteger(args, "total")
	if err != nil {
		return nil, err
	}
	transfer, err := owner.BeginBytes(ctx, user, artifactValue(args, "name"), total)
	return artifactTransfer(transfer), err
}

func artifactKeyList(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	var rows []map[string]any
	err := owner.ViewStore(ctx, user, artifactValue(args, "artifact"), func(source storage.Storage) error {
		keys, err := source.ListKeys(artifactValue(args, "prefix"))
		if err != nil {
			return err
		}
		rows = make([]map[string]any, 0, len(keys))
		for _, key := range keys {
			rows = append(rows, map[string]any{artifactFieldKey: key})
		}
		return nil
	})
	return plugin.Map{"keys": rows}, err
}

func artifactKeyRead(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	transfer, err := owner.SnapshotKey(ctx, user, artifactValue(args, "artifact"), artifactValue(args, "key"))
	return artifactTransfer(transfer), err
}

func artifactKeyWrite(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	count, err := owner.WriteKeyFromSource(ctx, user, artifactValue(args, "artifact"), artifactValue(args, "key"), artifactValue(args, "transfer"))
	return plugin.Map{artifactFieldKey: artifactValue(args, "key"), artifactFieldBytes: count}, err
}

func artifactKeyDelete(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	key := artifactValue(args, "key")
	err := owner.ChangeStore(ctx, user, artifactValue(args, "artifact"), func(target storage.Storage) error { return target.RemoveKey(key) })
	return plugin.Map{artifactFieldKey: key, "removed": err == nil}, err
}

func artifactCheck(ctx *pluginserver.CommandContext, owner *artifact.Owner, args command.ValidatedArgs) (*plugin.Response, error) {
	id, selected := args.Keyed("artifact")
	transfer, selectedTransfer := args.Keyed("transfer")
	if selected == selectedTransfer {
		return nil, opererr.New("Select one private artifact or one complete source to check.",
			"Both selections or neither selection would make the source ambiguous; nothing was checked.", "ze show config artifact check help")
	}
	label := "artifact " + id
	if selectedTransfer {
		id, label = transfer, "source "+transfer
	}
	var answer command.LocalAnswer
	err := owner.ViewSource(ctx.Context(), ctx.Username, id, func(path string) error {
		answer = storagecli.CheckSourceAnswer(path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if payload, ok := answer.Payload.(map[string]any); ok {
		payload["path"] = label
	}
	encoded, err := json.Marshal(answer.Payload)
	if err != nil {
		return nil, err
	}
	response := plugin.NewResponse(plugin.StatusDone, plugin.RawJSON(encoded))
	if answer.Code != 0 {
		response = plugin.OperatorErrorResponse(opererr.New("The private source failed its native integrity check.",
			answer.Diagnostic, "ze show config artifact check help"))
		response.Data = plugin.RawJSON(encoded)
	}
	return response, nil
}

func artifactRepair(ctx *pluginserver.CommandContext, owner *artifact.Owner, args command.ValidatedArgs) (*plugin.Response, error) {
	var report *zefs.RepairReport
	var findings []storage.HistoryFinding
	info, err := owner.ProduceArtifact(ctx.Context(), ctx.Username, artifactValue(args, "transfer"), func(source, output string) error {
		var err error
		report, findings, err = storagecli.RepairSourceToArtifact(source, output)
		return err
	})
	if err != nil {
		return nil, err
	}
	if report == nil {
		return nil, errors.New("the native salvage returned no report")
	}
	return storagecli.ArtifactRepairAnswer(info.ID, report, findings), nil
}
