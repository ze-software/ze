package server

import (
	"net"
	"reflect"
	"sync/atomic"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// muxPluginConn wires the engine's end of a plugin connection the way
// production wires one (Process.attachConn): through a MuxConn, which is what
// routes an answer's lines by id. A test that wires the engine end directly
// meets the refusal SendExecuteCommandAnswer owes such a connection, and never
// reaches the behavior it was written to check.
func muxPluginConn(t *testing.T, engineSide net.Conn) *ipc.PluginConn {
	t.Helper()

	mux := rpc.NewMuxConn(rpc.NewConn(engineSide, engineSide))
	t.Cleanup(func() { _ = mux.Close() })
	return ipc.NewMuxPluginConn(mux)
}

func markProcessRunning(t *testing.T, proc *process.Process) {
	t.Helper()

	field := reflect.ValueOf(proc).Elem().FieldByName("running")
	require.True(t, field.IsValid(), "process.running field must exist")
	require.True(t, field.CanAddr(), "process.running field must be addressable")

	running := (*atomic.Bool)(unsafe.Pointer(field.UnsafeAddr()))
	running.Store(true)
}
