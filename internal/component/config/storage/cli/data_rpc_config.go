// Design: docs/architecture/hub-architecture.md -- request data RPCs beside request reload.
// Related: data_rpc.go -- the store target and restoreDaemonConfig, the path a config restore takes.
// Related: register_data_rpc.go -- registers the handler.

package cli

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"

	"github.com/ze-software/ze/internal/component/cli/contract"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/confirm"
	"github.com/ze-software/ze/internal/component/config/storage"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
)

// pathUpdateDataConfig is the command `update data config` is typed as, which
// its refusals name first and the local `ze data import` sends, and
// usageUpdateDataConfig the whole command, the fix of a refusal that says what
// to type. answerVersion is the answer key of the version stamp the import
// committed.
const (
	pathUpdateDataConfig  = "update data config"
	usageUpdateDataConfig = "update data config name <name> content <base64>"
	answerVersion         = "version"
)

// fixEditStagedChange is the fix of an import refused because a change is
// already staged: the editor, where that change is committed or discarded.
const fixEditStagedChange = "ze config edit"

// handleDataConfigUpdate answers `update data config name <name> content
// <base64>`. It imports one config as a new version through the store handle
// the daemon owns. For the daemon's own config the content is staged as the
// candidate and promoted by the reload, the path `request data restore ...
// config` takes, inside the confirmed-commit window's worker: a config the
// reload refuses never becomes active, and an open window refuses the import
// with its own refusal. For any other name the content is committed as a new
// active version, the previous one kept as rollback, and nothing reloads; a
// managed client's config (client-<name>.conf) is refused unless this hub
// serves that client, so the version always reaches someone. The
// validator has already judged content against its leaf, and the handler
// decodes it again rather than trust that. Every refusal runs before the store
// is touched, and none quotes the content. The answer names the config, the
// version stamp and the bytes imported.
func handleDataConfigUpdate(ctx *pluginserver.CommandContext, validated command.ValidatedArgs) (*zePlugin.Response, error) {
	target := dataRPC.Load()
	if target == nil {
		return zePlugin.OperatorErrorResponse(opererr.New(
			pathUpdateDataConfig+": this daemon serves no store",
			"the daemon publishes its store to this command only while the store is open, so it is starting, stopping, or runs with no store",
			"ze data import <file>")), nil
	}
	name, found := validated.Value(keywordName)
	if !found {
		return zePlugin.OperatorErrorResponse(opererr.New(
			pathUpdateDataConfig+": no config name was given",
			"the command imports exactly one config, named by the value typed after name",
			usageUpdateDataConfig)), nil
	}
	encoded, found := validated.Value(keywordContent)
	if !found {
		return zePlugin.OperatorErrorResponse(opererr.New(
			pathUpdateDataConfig+": no content was given",
			"the command imports the bytes typed after content, base64-encoded",
			usageUpdateDataConfig)), nil
	}
	if !storage.PlainConfigName(name) {
		return zePlugin.OperatorErrorResponse(opererr.New(
			pathUpdateDataConfig+": "+strconv.Quote(name)+" is not a plain config name",
			"a config is named by its file name alone, with no directory, and never . or .., so no config changed",
			usageUpdateDataConfig)), nil
	}
	content, err := command.DecodeContent(encoded)
	if err != nil {
		// The shared decoder's error never repeats the supplied bytes.
		return zePlugin.OperatorErrorResponse(opererr.Wrap(err,
			pathUpdateDataConfig+": content is not base64",
			"content carries the config's bytes in base64 (RFC 4648 Section 4, with = padding), the encoding ze data import sends a file in, so no config changed",
			usageUpdateDataConfig)), nil
	}
	if len(content) == 0 {
		return zePlugin.OperatorErrorResponse(opererr.New(
			pathUpdateDataConfig+": the content is empty",
			"a config is imported with at least one byte, so "+name+" is unchanged",
			usageUpdateDataConfig)), nil
	}
	if name == filepath.Base(target.ConfigPath) {
		return importDaemonConfig(ctx.Context(), target, name, content), nil
	}
	if client, isClient := pluginserver.ClientNameFromConfigKey(name); isClient {
		problem := pathUpdateDataConfig + ": " + name + " was not imported"
		if refusal := unservedClientRefusal(target, problem, name, client); refusal != nil {
			return zePlugin.OperatorErrorResponse(refusal), nil
		}
	}
	return importOtherConfig(target, name, content), nil
}

// fixAddHubClient is the fix of a write to a client config this daemon would
// never serve: the editor, where the client entry under plugin hub server is
// added.
const fixAddHubClient = "ze config edit"

// UnservedClientConfigRefusal refuses a writer of the stored config name when
// name is client-<client>.conf, the config a hub serves to one managed client,
// and the running daemon serves no such client. problem is the caller's first
// line: the command the operator typed and what it did not do. The reason and
// the fix are the ones the import answers (handleDataConfigUpdate), from the
// same producer.
//
// It answers nil for a name that is not a client config, and for a client a
// serving block declares, whether or not that client is connected: a client
// that is away fetches when it connects.
//
// The served clients are read from the installed target at each call, never
// captured, because the hub publishes them only once its managed server has
// started (InstallDataRPC). With no target installed nobody can say which
// clients are served, so the writer is refused rather than admitted.
// Safe for concurrent use.
func UnservedClientConfigRefusal(problem, name string) *opererr.Error {
	client, isClient := pluginserver.ClientNameFromConfigKey(name)
	if !isClient {
		return nil
	}
	target := dataRPC.Load()
	if target == nil {
		return opererr.New(problem,
			name+" is the config a hub serves to the managed client "+client+", and the daemon publishes the clients it serves only while its store is open, so it is starting or stopping and cannot tell whether a client would ever fetch it",
			"ze doctor")
	}
	return unservedClientRefusal(target, problem, name, client)
}

// unservedClientRefusal refuses a write of client-<client>.conf, the config a
// hub serves to one managed client, when this daemon serves no such client:
// the version would be committed and no client would ever fetch it, which is
// why `request data restore ... client` refuses the same case
// (restoreConfigFromSource). problem is the first line the caller owns. It
// answers nil when the hub serves the client: the write observer then pushes
// config-changed to it.
func unservedClientRefusal(target *DataRPCTarget, problem, name, client string) *opererr.Error {
	if target.ServesClient == nil {
		return opererr.New(problem,
			name+" is the config a hub serves to the managed client "+client+", and this daemon serves no managed client, so no client would ever fetch it",
			fixAddHubClient)
	}
	if !target.ServesClient(client) {
		return opererr.New(problem,
			name+" is the config this hub serves to the managed client "+client+", and the hub has no client entry named "+client+" under plugin hub server, so no client would ever fetch it",
			fixAddHubClient)
	}
	return nil
}

// importDaemonConfig imports content as the daemon's own config, through the
// reload, inside the confirmed-commit window's worker.
func importDaemonConfig(ctx context.Context, target *DataRPCTarget, name string, content []byte) *zePlugin.Response {
	var window *confirm.Window
	if target.Window != nil {
		window = target.Window()
	}
	var stamp string
	err := confirm.WriteOutside(window, func() error {
		var stageErr error
		stamp, stageErr = restoreDaemonConfig(ctx, target, content)
		return stageErr
	})
	if err != nil {
		return zePlugin.OperatorErrorResponse(daemonConfigImportRefusal(name, err))
	}
	return configImported(name, stamp, len(content))
}

// daemonConfigImportRefusal explains why the daemon's own config was not
// imported: an open confirmed-commit window (its own refusal, naming its
// owner), a change somebody else staged, or the reload's refusal of the
// config. In each case nothing became active.
//
// An operator error the store gave (the freeze of a managed device, a key the
// freeze could not read) is answered as the store wrote it: it is already the
// whole operator error, and wrapping it would end the answer on another fix
// than its own.
func daemonConfigImportRefusal(name string, err error) *opererr.Error {
	if refusal, given := storage.FreezeRefusal(err); given {
		return refusal
	}
	problem := pathUpdateDataConfig + ": " + name + " was not imported"
	if _, open := errors.AsType[*confirm.OtherUserError](err); open {
		return opererr.Wrap(err, problem,
			"a confirmed-commit window is open, and its revert would wipe the import, so nothing was staged",
			contract.CommitCommand(contract.CommitAccept))
	}
	if errors.Is(err, errConfigChangeStaged) {
		return opererr.Wrap(err, problem,
			"an import would replace the change somebody else staged, so nothing was staged",
			fixEditStagedChange)
	}
	return opererr.Wrap(err, problem,
		"the active config, its pointers and the running daemon are unchanged",
		"ze config validate <file>")
}

// importOtherConfig commits content as a new active version of a config that
// is not the daemon's own, keeping the previous one as rollback. Nothing
// reloads: the daemon does not run that config.
func importOtherConfig(target *DataRPCTarget, name string, content []byte) *zePlugin.Response {
	stamp, err := storage.RestoreConfig(target.Store, name, content)
	if errors.Is(err, storage.ErrCandidateExists) {
		return zePlugin.OperatorErrorResponse(opererr.Wrap(err,
			pathUpdateDataConfig+": "+name+" was not imported",
			"a change to "+name+" is already staged; commit or discard it first, because an import would replace it",
			fixEditStagedChange))
	}
	if err != nil {
		return zePlugin.OperatorErrorResponse(opererr.Wrap(err,
			pathUpdateDataConfig+": the store refused the new version of "+name,
			name+" is unchanged",
			"ze doctor"))
	}
	return configImported(name, stamp, len(content))
}

// configImported is the answer of an import: the config, the version stamp
// committed and the bytes imported.
func configImported(name, stamp string, octets int) *zePlugin.Response {
	return &zePlugin.Response{
		Status: zePlugin.StatusDone,
		Data:   zePlugin.Map{keywordName: name, answerVersion: stamp, keywordBytes: octets},
	}
}
