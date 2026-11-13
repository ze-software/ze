// Design: docs/guide/ddos-mitigation.md -- what each end of the plugin does to a live FlowSpec rule
// Related: register.go (transportClosed, stopResponder)

package flowspec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"syscall"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestTransportClosedReadsEachClosedConnectionForm proves that transportClosed
// answers true for each error a call meets on a connection to the engine that
// is gone, and false for every other error.
//
// VALIDATES: each arm of transportClosed on its own row, in the form the
// producer gives it. The write path, wrapped by MuxConn.CallRPC as "send
// request: %w" (pkg/plugin/rpc/mux.go): on a net.Pipe whose remote end is
// closed, Conn.writeFrame's SetWriteDeadline fails with io.ErrClosedPipe and
// is wrapped as "set write deadline: %w"; on an io.Pipe, which has no write
// deadline, the write itself fails with io.ErrClosedPipe and is wrapped as
// "write frame: %w" (pkg/plugin/rpc/conn.go). A socket whose peer closed fails
// the write with EPIPE or ECONNRESET, which do not count. The read path: the reader's terminal
// error, io.EOF for the engine's orderly close and io.ErrClosedPipe or
// net.ErrClosed for a closed stream, which CallRPC and closedErr wrap as "mux
// conn read error: %w". The sentinels: rpc.ErrMuxConnClosed from closedErr and
// rpc.ErrBridgeClosed from the bridge, unwrapped. sdkDispatcher.Dispatch and the
// SDK's callEngineRaw return the call's error as they got it, so no row adds a
// wrap of the dispatcher's own.
// METHOD: one table, one row per arm and per path that reaches it, plus the
// errors that must not count: a socket reset or broken pipe, a real timeout on
// either path, an RPC error answer, and nil.
// PREVENTS: an arm deleted in silence. Before this test only a race in another
// test reached the io.ErrClosedPipe arm, so deleting it left every test green.
//
// MUTATION: each arm removed from transportClosed reddens its own rows only:
// the io.ErrClosedPipe arm the two "send request on a closed" rows and the
// "mux conn read error: closed pipe" row, the io.EOF arm the "mux conn read error: EOF"
// row, the rpc.ErrMuxConnClosed arm the "mux conn closed" row, the
// rpc.ErrBridgeClosed arm the "bridge closed" row, and the net.ErrClosed arm the
// "use of closed network connection" row. The function written as
// `return err != nil` reddens every false row but nil.
func TestTransportClosedReadsEachClosedConnectionForm(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			// net.Pipe: SetWriteDeadline fails once the remote end is closed, so
			// writeFrame returns before it writes.
			name: "send request on a closed net.Pipe",
			err:  fmt.Errorf("send request: %w", fmt.Errorf("set write deadline: %w", io.ErrClosedPipe)),
			want: true,
		},
		{
			// io.Pipe has no write deadline, so the write runs and fails.
			name: "send request on a closed io.Pipe",
			err:  fmt.Errorf("send request: %w", fmt.Errorf("write frame: %w", io.ErrClosedPipe)),
			want: true,
		},
		{
			name: "mux conn read error: EOF",
			err:  fmt.Errorf("mux conn read error: %w", io.EOF),
			want: true,
		},
		{
			name: "mux conn read error: closed pipe",
			err:  fmt.Errorf("mux conn read error: %w", io.ErrClosedPipe),
			want: true,
		},
		{
			name: "mux conn closed",
			err:  rpc.ErrMuxConnClosed,
			want: true,
		},
		{
			name: "bridge closed",
			err:  rpc.ErrBridgeClosed,
			want: true,
		},
		{
			name: "use of closed network connection",
			err: fmt.Errorf("mux conn read error: %w",
				&net.OpError{Op: "read", Net: "tcp", Err: net.ErrClosed}),
			want: true,
		},
		{
			name: "connection reset by peer",
			err: fmt.Errorf("send request: %w", fmt.Errorf("write frame: %w",
				&net.OpError{Op: "write", Net: "tcp", Err: os.NewSyscallError("write", syscall.ECONNRESET)})),
			want: false,
		},
		{
			name: "broken pipe",
			err: fmt.Errorf("send request: %w", fmt.Errorf("write frame: %w",
				&net.OpError{Op: "write", Net: "tcp", Err: os.NewSyscallError("write", syscall.EPIPE)})),
			want: false,
		},
		{
			// A write that timed out: writeFrame answers context.DeadlineExceeded
			// unwrapped, and CallRPC wraps it as "send request: %w".
			name: "send request past its deadline",
			err:  fmt.Errorf("send request: %w", context.DeadlineExceeded),
			want: false,
		},
		{
			// A response that did not arrive in time: CallRPC answers ctx.Err().
			name: "response past its deadline",
			err:  context.DeadlineExceeded,
			want: false,
		},
		{
			name: "rpc error answer",
			err:  &rpc.RPCCallError{Code: "send-refused", Message: "no peer attaches this process"},
			want: false,
		},
		{
			name: "plain error",
			err:  errors.New("update-route: invalid prefix"),
			want: false,
		},
		{
			name: "nil",
			err:  nil,
			want: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := transportClosed(tc.err); got != tc.want {
				t.Errorf("transportClosed(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
