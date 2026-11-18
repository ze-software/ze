// Design: docs/architecture/api/commands.md -- account-bound detached artifact operations.
// Related: ../artifact/owner.go -- one shared lifetime and private artifact inventory.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config/artifact"
	storagecli "github.com/ze-software/ze/internal/component/config/storage/cli"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
)

// This registry binds the feature owner to the actual Server, including a
// daemon with no web listener. Its stop hook removes the binding before Close.
var artifactOwners = struct {
	sync.Mutex
	items map[*pluginserver.Server]*artifact.Owner
}{items: make(map[*pluginserver.Server]*artifact.Owner)}

func serverArtifacts(server *pluginserver.Server) (*artifact.Owner, error) {
	if err := server.CheckAdmission(); err != nil {
		return nil, err
	}
	artifactOwners.Lock()
	defer artifactOwners.Unlock()
	if owner, found := artifactOwners.items[server]; found {
		return owner, nil
	}
	owner := artifact.New()
	if err := attachServerArtifactsLocked(server, owner); err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	return owner, nil
}

// AttachArtifactOwner transfers a constructed, unused private owner's lifetime
// to one server. The caller MUST stop using its separate lifetime after success
// and MUST close a newly constructed owner after refusal. An owner already
// attached belongs to that server and MUST NOT be closed by a refused caller.
// The server MUST stop before its resources are discarded. An existing binding
// is never replaced. Safe for concurrent use.
func AttachArtifactOwner(server *pluginserver.Server, owner *artifact.Owner) error {
	if owner == nil {
		return errors.New("construct an artifact owner before attaching it")
	}
	if err := server.CheckAdmission(); err != nil {
		return err
	}
	artifactOwners.Lock()
	defer artifactOwners.Unlock()
	return attachServerArtifactsLocked(server, owner)
}

// attachServerArtifactsLocked publishes only after OnStop accepts ownership.
// The caller MUST hold artifactOwners and retain owner on refusal.
func attachServerArtifactsLocked(server *pluginserver.Server, owner *artifact.Owner) error {
	if _, found := artifactOwners.items[server]; found {
		return errors.New("the server already owns its private artifact workspace")
	}
	for _, existing := range artifactOwners.items {
		if existing == owner {
			return errors.New("this private artifact owner already belongs to another server; retain its original server's lifetime")
		}
	}
	if err := owner.SetDirectoryGuard(storagecli.CheckBackupDirectory); err != nil {
		return err
	}
	_, err := server.OnStop(func() error {
		closeErr := owner.Close()
		artifactOwners.Lock()
		delete(artifactOwners.items, server)
		artifactOwners.Unlock()
		return closeErr
	})
	if err != nil {
		return err
	}
	artifactOwners.items[server] = owner
	return nil
}

type artifactOperation func(context.Context, *artifact.Owner, string, command.ValidatedArgs) (plugin.ResponseData, error)

const (
	artifactFieldID    = "artifact"
	artifactFieldKey   = "key"
	artifactFieldBytes = "bytes"
)

func artifactHandler(operation artifactOperation) pluginserver.Handler {
	return func(ctx *pluginserver.CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
		owner, err := artifactCommandOwner(ctx, args)
		if err != nil {
			return plugin.OperatorErrorResponse(opererr.Wrap(err, "The artifact operation could not start.", "No artifact was changed.", "Reconnect to the running daemon and retry")), nil
		}
		data, err := operation(ctx.Context(), owner, ctx.Username, args)
		if err != nil {
			return artifactOperationFailure(err), nil
		}
		return &plugin.Response{Status: plugin.StatusDone, Data: data}, nil
	}
}

func artifactOperationFailure(err error) *plugin.Response {
	if refusal := artifactSingleRefusal(err); refusal != nil {
		if failure, native := refusal.Cause.(*opererr.Error); native { //nolint:errorlint // Only the direct native cause owns the complete refusal context.
			response := plugin.OperatorErrorResponse(failure)
			response.Data = plugin.Map{"http-status": refusal.Status}
			return response
		}
	}
	response := plugin.OperatorErrorResponse(opererr.Wrap(err, "The artifact operation could not complete.", "No live configuration was changed.", "Run ze show doctor, then retry the artifact operation"))
	response.Data = plugin.Map{"http-status": http.StatusInternalServerError}
	return response
}

// artifactSingleRefusal removes only singleton join nodes. A separate cleanup
// cause makes the whole operation fail; selecting its first refusal would lose
// that evidence and mistake a server failure for the refusal's HTTP category.
func artifactSingleRefusal(err error) *artifact.Refusal {
	for {
		if refusal, ok := err.(*artifact.Refusal); ok { //nolint:errorlint // Nested refusals may hide cleanup; only direct or singleton-joined refusals retain their category.
			return refusal
		}
		joined, ok := err.(interface{ Unwrap() []error })
		if !ok {
			return nil
		}
		causes := joined.Unwrap()
		if len(causes) != 1 {
			return nil
		}
		err = causes[0]
	}
}

// artifactResponseHandler preserves native warnings and failed verdict data.
// Admission and failures are shared with ordinary artifact data operations.
func artifactResponseHandler(operation func(*pluginserver.CommandContext, *artifact.Owner, command.ValidatedArgs) (*plugin.Response, error)) pluginserver.Handler {
	return func(ctx *pluginserver.CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
		owner, err := artifactCommandOwner(ctx, args)
		if err != nil {
			return artifactOperationFailure(err), nil
		}
		response, err := operation(ctx, owner, args)
		if err != nil {
			return artifactOperationFailure(err), nil
		}
		return response, nil
	}
}

// artifactCommandOwner validates the same closed input boundary before an
// operation acquires any private lifetime, including a full-restore handoff.
func artifactCommandOwner(ctx *pluginserver.CommandContext, args command.ValidatedArgs) (*artifact.Owner, error) {
	if len(args.PositionalTokens()) != 0 {
		return nil, opererr.New("The artifact command contains unexpected arguments.",
			"Use the command's declared keywords and values; no artifact operation was started.", "ze help")
	}
	return serverArtifacts(ctx.Server)
}

func artifactValue(args command.ValidatedArgs, name string) string {
	value, _ := args.Keyed(name)
	return value
}

func artifactInteger(args command.ValidatedArgs, name string) (int64, error) {
	value, found := args.Keyed(name)
	if !found {
		return -1, nil
	}
	return strconv.ParseInt(value, 10, 64)
}

func artifactTransfer(t artifact.Transfer) plugin.Map {
	return plugin.Map{"transfer": t.ID, "total": t.Total, "next": t.Next, "complete": t.Complete}
}

func artifactBegin(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs, text bool) (plugin.ResponseData, error) {
	total, err := artifactInteger(args, "total")
	if err != nil {
		return nil, err
	}
	name := artifactValue(args, "name")
	if text {
		name = "configuration.txt"
	}
	t, err := owner.Begin(ctx, user, name, total, text)
	return artifactTransfer(t), err
}
func artifactUploadBegin(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	return artifactBegin(ctx, owner, user, args, false)
}
func artifactTextBegin(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	return artifactBegin(ctx, owner, user, args, true)
}
func artifactUploadAppend(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	content, err := command.DecodeContent(artifactValue(args, "content"))
	if err != nil {
		return nil, err
	}
	offset, err := artifactInteger(args, "offset")
	if err != nil {
		return nil, err
	}
	t, err := owner.Append(ctx, user, artifactValue(args, "transfer"), offset, content)
	return artifactTransfer(t), err
}
func artifactUploadFinish(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	w, t, err := owner.Finish(ctx, user, artifactValue(args, "transfer"))
	if err != nil {
		return nil, err
	}
	if w.ID != "" {
		return plugin.Map{artifactFieldID: w.ID, "label": w.Label, "names": w.Names, artifactFieldBytes: w.Bytes}, nil
	}
	return artifactTransfer(t), nil
}
func artifactTransferAbort(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	id := artifactValue(args, "transfer")
	return plugin.Map{"transfer": id}, owner.Abort(ctx, user, id)
}
func artifactDownloadBegin(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	t, err := owner.Snapshot(ctx, user, artifactValue(args, "artifact"), "")
	return artifactTransfer(t), err
}
func artifactSourceBegin(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	t, err := owner.Snapshot(ctx, user, artifactValue(args, "artifact"), artifactValue(args, "name"))
	return artifactTransfer(t), err
}
func artifactTransferRead(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	offset, err := artifactInteger(args, "offset")
	if err != nil {
		return nil, err
	}
	t, content, err := owner.Read(ctx, user, artifactValue(args, "transfer"), offset)
	result := artifactTransfer(t)
	result["content"] = command.Blob(content)
	return result, err
}
func artifactList(ctx context.Context, owner *artifact.Owner, user string, _ command.ValidatedArgs) (plugin.ResponseData, error) {
	rows, err := owner.List(ctx, user)
	return plugin.Slice[artifact.Info](rows), err
}
func artifactInfo(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	w, err := owner.Inspect(ctx, user, artifactValue(args, "artifact"))
	return plugin.Map{artifactFieldID: w.ID, "label": w.Label, "names": w.Names, artifactFieldBytes: w.Bytes, "digest": w.Digest}, err
}
func artifactDelete(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	id := artifactValue(args, "artifact")
	return plugin.Map{artifactFieldID: id}, owner.Remove(ctx, user, id)
}
func artifactSource(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	name := artifactValue(args, "name")
	content, err := owner.Config(ctx, user, artifactValue(args, "artifact"), name)
	if err != nil {
		return nil, err
	}
	answer := sourceBytesAnswer(args, content, name, false)
	if answer.Code != 0 {
		return nil, opererr.New(answer.Diagnostic, "The selected source could not be safely inspected.", "Use the private artifact download to inspect its raw bytes")
	}
	encoded, err := json.Marshal(answer.Payload)
	return plugin.RawJSON(encoded), err
}
func artifactValidate(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	return artifactEdit(ctx, owner, user, args, "validate")
}
func artifactReview(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	return artifactEdit(ctx, owner, user, args, "review")
}
func artifactSave(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs) (plugin.ResponseData, error) {
	return artifactEdit(ctx, owner, user, args, "save")
}
func artifactEdit(ctx context.Context, owner *artifact.Owner, user string, args command.ValidatedArgs, action string) (plugin.ResponseData, error) {
	result, err := owner.Edit(ctx, user, artifactValue(args, "artifact"), artifactValue(args, "name"), artifactValue(args, "text"), artifactValue(args, "base"), artifactValue(args, "review"), action)
	if err != nil {
		return nil, err
	}
	diff := ""
	if result.Diff != "" {
		retained, err := owner.Retain(ctx, user, []byte(result.Diff))
		if err != nil {
			return nil, err
		}
		diff = retained.ID
	}
	return plugin.Map{"base": result.Base, "review": result.Review, "diff-transfer": diff, "message": result.Status, "saved": result.Saved}, nil
}
