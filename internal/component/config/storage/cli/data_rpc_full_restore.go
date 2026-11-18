// Design: docs/architecture/hub-architecture.md -- accepted recovery after daemon teardown.
// Related: data_rpc.go -- the target and the path-based full restore adapter.
package cli

import (
	"errors"
	"io"

	"github.com/ze-software/ze/internal/component/config/storage"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// AcceptFullRestoreWriter shares full recovery with a caller-owned artifact
// producer. write MUST finish its complete stable export before returning; its
// bytes become an independent lifecycle-owned snapshot before acceptance.
func AcceptFullRestoreWriter(ctx *pluginserver.CommandContext, write func(io.Writer) error) (*zePlugin.Response, error) {
	return acceptFullRestore(ctx, func(admission *storage.RecoveryAdmission) (storage.RecoveryReceipt, error) {
		return admission.StageWriter(ctx.Context(), write)
	})
}

func acceptFullRestore(ctx *pluginserver.CommandContext, stage func(*storage.RecoveryAdmission) (storage.RecoveryReceipt, error)) (*zePlugin.Response, error) {
	target := dataRPC.Load()
	if target == nil {
		return fullRestoreLifecycleUnavailable(), nil
	}
	admission := storage.RecoveryFor(target.Store)
	if admission == nil {
		return fullRestoreLifecycleUnavailable(), nil
	}
	if ctx == nil {
		return fullRestoreLifecycleUnavailable(), nil
	}
	complete, err := ctx.Server.CaptureRestartCompletion(ctx.Context())
	if err != nil {
		return fullRestoreFailure(err), nil
	}
	receipt, err := stage(admission)
	if err != nil {
		return fullRestoreFailure(err), nil
	}
	response := &zePlugin.Response{Status: zePlugin.StatusDone, Data: zePlugin.Map{
		"state": "accepted", "bytes": receipt.Bytes, "digest": receipt.Digest,
		"restart-exit-status": rpc.DaemonRestartExitStatus,
		"message":             "full-store recovery was accepted and will run after a clean daemon shutdown",
	}}
	response.OnTransportComplete(complete)
	return response, nil
}

func fullRestoreFailure(err error) *zePlugin.Response {
	if refusal, ok := errors.AsType[*opererr.Error](err); ok {
		return zePlugin.OperatorErrorResponse(refusal)
	}
	return zePlugin.OperatorErrorResponse(opererr.Wrap(err, "full-store recovery was not accepted",
		"no complete verified recovery source was accepted and the live store is unchanged", "ze doctor"))
}

func fullRestoreLifecycleUnavailable() *zePlugin.Response {
	return zePlugin.OperatorErrorResponse(opererr.New("full-store recovery lifecycle is not installed",
		"this daemon has no outer live-tree owner that can close its store before replacing it",
		"ze data restore <artifact> full"))
}
