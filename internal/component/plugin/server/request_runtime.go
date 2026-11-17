// Design: docs/architecture/api/process-protocol.md -- runtime socket request lifetimes.
// Related: request_cancel.go -- synchronous admission and final settlement.
package server

import (
	"encoding/json"
	"errors"
	"sync"

	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
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
					logger().Debug("rpc runtime: close collided connection failed", "plugin", proc.Name(), "error", closeErr)
				}
				return
			}
			workers.Go(func() {
				defer admission.finish()
				if sendErr := conn.SendError(s.replyContext(), request.ID, rpcErrMessage(err)); sendErr != nil {
					logger().Debug("rpc runtime: send error failed", "plugin", proc.Name(), "error", sendErr)
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
