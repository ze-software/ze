// Design: docs/architecture/api/architecture.md -- gNMI transport, panic containment per RPC
// Related: server.go -- Serve, which chains these interceptors outermost, and SetMetricsRegistry
// Related: errors.go -- sentinel errors

package gnmi

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/ze-software/ze/internal/core/taskguard"
)

// taskRPC is the task label of one gNMI RPC (Capabilities, Get, Set) or one
// Subscribe stream in ze_task_panics_total{component="gnmi"}.
const taskRPC = "rpc"

// rpcRestart is what follows a contained RPC panic, in the words the guard's
// log line gives the operator.
const rpcRestart = "only this RPC or Subscribe stream ended, with code Internal; the client " +
	"retries it, and the gNMI listener and every other RPC and stream keep running"

// rpcGuard is the panic boundary of every gNMI RPC and stream. A contained panic
// is logged, counted in ze_task_panics_total{component="gnmi",task="rpc"} and
// crash-recorded. Safe for concurrent use.
var rpcGuard = taskguard.New("gnmi", logger)

// rpcTask is the RPC task, declared once so its count exists before the first
// panic.
var rpcTask = rpcGuard.Task(taskRPC, rpcRestart)

// containUnaryPanic is the outermost unary interceptor: a Ze defect met while
// one RPC is served ends that RPC with codes.Internal, and grpc-go, which
// recovers nothing itself, never sees the panic.
func (s *Server) containUnaryPanic(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer rpcTask.Contain(
		func() taskguard.Context { return rpcContext(ctx, info.FullMethod) },
		func(p *taskguard.Panic) {
			resp = nil
			err = s.endRPCAfterPanic(info.FullMethod, p)
		},
	)
	return handler(ctx, req)
}

// containStreamPanic is the outermost stream interceptor, the Subscribe
// counterpart of containUnaryPanic. The stream handler runs every Subscribe
// mode on this goroutine, so the boundary covers the whole stream.
func (s *Server) containStreamPanic(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	defer rpcTask.Contain(
		func() taskguard.Context { return rpcContext(ss.Context(), info.FullMethod) },
		func(p *taskguard.Panic) { err = s.endRPCAfterPanic(info.FullMethod, p) },
	)
	return handler(srv, ss)
}

// endRPCAfterPanic counts the RPC's error and answers the client in plain words:
// what ended, that the rest of the server runs on, and what to do next.
func (s *Server) endRPCAfterPanic(fullMethod string, p *taskguard.Panic) error {
	method := rpcName(fullMethod)
	s.recordError(method, codes.Internal)

	next := "retry the request; if it fails again, run 'ze show crashes' on the router " +
		"and attach the newest record to a bug report"
	if p.Report != "" {
		next = "retry the request; if it fails again, run 'ze show crashes name " + p.Report +
			"' on the router and attach it to a bug report"
	}
	return status.Error(codes.Internal, "an internal Ze defect ended this "+method+
		" call; the gNMI server and every other call keep running: "+next)
}

// rpcContext names the client and the RPC for the guard's log line and crash
// record. It reads only the request context, so it never blocks.
func rpcContext(ctx context.Context, fullMethod string) taskguard.Context {
	unit := "client with no peer address"
	if client, known := peer.FromContext(ctx); known {
		if client.Addr != nil {
			unit = "client " + client.Addr.String()
		}
	}
	return taskguard.Context{Unit: unit, MessageType: rpcName(fullMethod)}
}

// rpcName is the RPC's short name ("Get") from its full gRPC method
// ("/gnmi.gNMI/Get"), the label the gNMI error counter already uses.
func rpcName(fullMethod string) string {
	return fullMethod[strings.LastIndexByte(fullMethod, '/')+1:]
}
