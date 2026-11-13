// Design: docs/architecture/api/process-protocol.md -- engine operation admission at Stage 2.

package server

import (
	"context"

	plugipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/taskguard"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// serveConfigureEngineOp consumes a nested Stage 2 request, or leaves the legal
// next-stage request to the shared startup driver. Runtime state and codecs
// retain their stage barrier; only explicitly declared ConfigureSafe handlers
// may run here, with the configured process identity and startup deadline.
func (s *Server) serveConfigureEngineOp(ctx context.Context, proc *process.Process, conn *plugipc.PluginConn, req *rpc.Request) (consumed bool, err error) {
	if req.Method == methodDeclareCapabilities {
		return false, nil
	}
	// A recovered handler panic still consumes its request: it must not be
	// mistaken for the next startup stage and replayed by ReadRequest.
	consumed = true
	defer engineOpPanics().request.Contain(
		func() taskguard.Context {
			return taskguard.Context{Unit: "configure request from plugin " + proc.Name(), MessageType: req.Method}
		},
		func(p *taskguard.Panic) {
			err = conn.SendError(ctx, req.ID, engineOpPanicMessage(req.Method, p))
		},
	)
	op := lookupEngineOp(req.Method)
	if op == nil {
		return true, s.refuseConfigureEngineOp(ctx, conn, req)
	}
	if op.startup != EngineOperationConfigureSafe {
		return true, s.refuseConfigureEngineOp(ctx, conn, req)
	}
	return true, s.serveEngineOpJSONContext(ctx, ctx, proc, conn, req, op)
}

func (s *Server) refuseConfigureEngineOp(ctx context.Context, conn *plugipc.PluginConn, req *rpc.Request) error {
	return conn.SendError(ctx, req.ID, "method "+req.Method+" is unavailable during configure; call it after ready")
}
