// Design: docs/architecture/api/process-protocol.md -- runtime socket request lifetimes.
// Related: request_cancel.go -- synchronous admission and final settlement.
package server

import (
	"encoding/json"
	"errors"
	"sync"

	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// serveRuntimeRequests owns this connection's dispatch lifetimes. It MUST
// cancel admitted opt-in contexts before joining handlers on every reader exit;
// the caller MUST defer process cleanup until this function has returned.
func (s *Server) serveRuntimeRequests(proc *process.Process, conn *plugipc.PluginConn) {
	requests := newRuntimeRequests(s.Context(), func(operation *engineOp, params json.RawMessage) (requestScope, error) {
		return operation.cancellationScope(s, proc, params)
	})
	var workers sync.WaitGroup
	defer func() { requests.stop(); workers.Wait(); requests.cancelAll() }()
	// This is the connection's long-lived reader, ending on EOF or server stop.
	for {
		request, err := conn.ReadRequest(s.Context())
		if err != nil {
			if s.Context().Err() == nil {
				logger().Debug("rpc runtime: read failed", "plugin", proc.Name(), "error", err)
			}
			return
		}
		admission, err := requests.admit(request)
		if err != nil {
			if _, collision := errors.AsType[*requestIDCollision](err); collision {
				// Never answer under an ID another handler still owns: that
				// would release its caller before the original writer joins.
				if closeErr := conn.AbortRuntime(requests.cancelAll); closeErr != nil {
					noteCollisionAbortFailure(proc, closeErr)
				}
				return
			}
			workers.Go(func() {
				defer admission.finish()
				if sendErr := conn.SendError(s.replyContext(), request.ID, rpcErrMessage(err)); sendErr != nil {
					s.noteReplyDeliveryFailure(proc, request.Method, replyKindError, sendErr)
				}
			})
			continue
		}
		workers.Go(func() {
			defer admission.finish()
			s.dispatchPluginRPCContext(admission.ctx, proc, conn, request)
		})
	}
}

// noteCollisionAbortFailure records a failure from conn.AbortRuntime after a
// duplicate in-flight request ID collision. Already-closed connection errors
// stay at Debug because cleanupProcess tears down the plugin; any other error
// warns under CodeDoctorPluginRequestRefused and raises on the report bus,
// since MuxConn.Close runs via sync.Once and teardown will not retry the close.
func noteCollisionAbortFailure(proc *process.Process, closeErr error) {
	pluginName := ""
	if proc != nil {
		pluginName = proc.Name()
	}
	displayName := pluginName
	if displayName == "" {
		displayName = "unknown-plugin"
	}
	if isClosedConnErr(closeErr) {
		// refuse-by-default: guards cleanupProcess -- connection already closed; cleanupProcess tears down the plugin
		logger().Debug("rpc runtime: collided connection already closed", "plugin", displayName, "error", closeErr)
		return
	}
	fix := pluginRestartFix(pluginName)
	var msg textbuf.Buffer
	msg.Str("daemon could not close connection for plugin ").Str(displayName).
		Str(" after a duplicate in-flight request ID collision: ").Err(closeErr).
		Str("; teardown will not retry the close, so the plugin session may remain stuck")
	logger().Warn("daemon could not close collided plugin connection; teardown will not retry the close",
		"code", diagnostic.CodeDoctorPluginRequestRefused,
		"plugin", displayName,
		"error", closeErr,
		"fix", fix)
	refusePluginRequestWithFix(fitSubject(displayName), msg.String(), busFix(fix, fixShowErrors))
}
