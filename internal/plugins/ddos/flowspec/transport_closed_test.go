// Design: docs/guide/ddos-mitigation.md -- what each end of the plugin does to a live FlowSpec rule
// Related: register.go (transportClosed, stopResponder)

package flowspec

import (
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
// producer wraps it. A write after the engine closed its end of an in-process
// net.Pipe answers io.ErrClosedPipe, which MuxConn.CallRPC wraps as "send
// request: %w" (pkg/plugin/rpc/mux.go); a read that met the engine's orderly
// close answers io.EOF, which closedErr wraps as "mux conn read error: %w". The
// dispatcher wraps either again as "update-route: %w".
// METHOD: one table, one row per arm, plus the errors that must not count: a
// socket reset or broken pipe, an RPC error answer, and nil.
// PREVENTS: an arm deleted in silence. Before this test only a race in another
// test reached the io.ErrClosedPipe arm, so deleting it left every test green.
//
// MUTATION: each arm removed from transportClosed reddens its own row only:
// the io.ErrClosedPipe arm the "send request" row, the io.EOF arm the "mux conn
// read error: EOF" row, the rpc.ErrMuxConnClosed arm the "mux conn closed" row,
// the rpc.ErrBridgeClosed arm the "bridge closed" row, and the net.ErrClosed
// arm the "use of closed network connection" row. The function written as
// `return err != nil` reddens every false row but nil.
func TestTransportClosedReadsEachClosedConnectionForm(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "send request on a closed pipe",
			err:  fmt.Errorf("update-route: %w", fmt.Errorf("send request: %w", io.ErrClosedPipe)),
			want: true,
		},
		{
			name: "mux conn read error: EOF",
			err:  fmt.Errorf("update-route: %w", fmt.Errorf("mux conn read error: %w", io.EOF)),
			want: true,
		},
		{
			name: "mux conn closed",
			err:  fmt.Errorf("update-route: %w", rpc.ErrMuxConnClosed),
			want: true,
		},
		{
			name: "bridge closed",
			err:  fmt.Errorf("update-route: %w", rpc.ErrBridgeClosed),
			want: true,
		},
		{
			name: "use of closed network connection",
			err: fmt.Errorf("update-route: %w", fmt.Errorf("mux conn read error: %w",
				&net.OpError{Op: "read", Net: "tcp", Err: net.ErrClosed})),
			want: true,
		},
		{
			name: "connection reset by peer",
			err: fmt.Errorf("update-route: %w", fmt.Errorf("send request: %w",
				&net.OpError{Op: "write", Net: "tcp", Err: os.NewSyscallError("write", syscall.ECONNRESET)})),
			want: false,
		},
		{
			name: "broken pipe",
			err: fmt.Errorf("update-route: %w", fmt.Errorf("send request: %w",
				&net.OpError{Op: "write", Net: "tcp", Err: os.NewSyscallError("write", syscall.EPIPE)})),
			want: false,
		},
		{
			name: "rpc error answer",
			err:  fmt.Errorf("update-route: %w", &rpc.RPCCallError{Code: "send-refused", Message: "no peer attaches this process"}),
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
